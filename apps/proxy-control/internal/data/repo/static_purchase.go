package repo

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"net"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/biz/product"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/biz/wallet"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func NewWalletRepositoryWithStatic(db *gorm.DB, supplier wallet.StaticSupplier, passwordKey, exchangeProbeAddress string) (*WalletRepository, error) {
	repository := NewWalletRepository(db)
	if exchangeProbeAddress != "" {
		host, rawPort, err := net.SplitHostPort(exchangeProbeAddress)
		port, portErr := strconv.ParseUint(rawPort, 10, 16)
		if err != nil || host == "" || strings.ContainsAny(host, " /\\\t\r\n") || portErr != nil || port == 0 {
			return nil, fmt.Errorf("ipcrown.exchange_probe_address must be a host and port")
		}
		repository.staticExchangeProbeAddress = exchangeProbeAddress
	}
	if supplier == nil && passwordKey == "" {
		return repository, nil
	}
	key, err := base64.RawURLEncoding.DecodeString(passwordKey)
	if err != nil || len(key) != 32 {
		return nil, fmt.Errorf("ipcrown.password_key must be a base64url-encoded 32-byte key")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	repository.staticCipher, err = cipher.NewGCMWithRandomNonce(block)
	repository.static = supplier
	return repository, err
}

type storedStaticIP struct {
	wallet.StaticAllocation
	PasswordCiphertext    []byte
	PurchaseSpecification string
	OriginalOffer         string
	StaticRegion          string
	StaticIPType          string
}

func staticPasswordScope(tenant, item string) []byte {
	return []byte(tenant + ":" + item)
}

func (r *WalletRepository) requireStaticOfferSale(ctx context.Context) error {
	if r.db == nil {
		return errWalletUnavailable
	}
	if err := RequireProductSale(ctx, r.db, product.Static); err != nil {
		return err
	}
	if r.static == nil {
		return wallet.ErrStaticUnavailable
	}
	return nil
}

func (r *WalletRepository) liveStaticOffers(ctx context.Context, shareMode string, purposeID int32) ([]wallet.StaticOffer, error) {
	if err := r.requireStaticOfferSale(ctx); err != nil {
		return nil, err
	}

	offers, err := r.static.Offers(ctx, shareMode, purposeID)
	if err != nil {
		return nil, wallet.ErrStaticUnavailable
	}
	if err := RequireProductSale(ctx, r.db, product.Static); err != nil {
		return nil, err
	}
	return offers, nil
}

type staticQuotedLine struct {
	wallet.StaticPurchaseLine
	Available     int32
	TermDays      int32
	Specification string
}

type staticQuoteToken struct {
	Term      string
	ShareMode string
	PurposeID int32
	ExpiresAt time.Time
	Lines     []staticQuotedLine

	// Single-node tokens issued before multi-node checkout remain valid until expiry.
	NodeID, UnitMinor   int64 `json:",omitempty"`
	Quantity, Available int32 `json:",omitempty"`
}

func offerSpecification(offer wallet.StaticOffer) string {
	data, _ := json.Marshal([4]string{offer.Region, offer.IPType, offer.Continent, offer.Line})
	sum := sha256.Sum256(data)
	return base64.RawStdEncoding.EncodeToString(sum[:])
}

func (r *WalletRepository) QuoteStatic(ctx context.Context, tenant string, p wallet.StaticPurchaseRequest) (wallet.StaticQuote, error) {
	lines, err := r.currentStaticQuotes(ctx, p)
	if err != nil {
		return wallet.StaticQuote{}, err
	}
	payload := staticQuoteToken{Term: p.Term, ExpiresAt: time.Now().UTC().Add(5 * time.Minute),
		ShareMode: p.ShareMode, PurposeID: p.PurposeID}
	for _, line := range lines {
		payload.Lines = append(payload.Lines, staticQuotedLine{
			StaticPurchaseLine: wallet.StaticPurchaseLine{
				NodeID: line.Offer.NodeID, Quantity: line.Quantity, UnitMinor: line.Offer.UnitMinor,
			},
			Available: line.Offer.Available, TermDays: line.Offer.TermDays, Specification: offerSpecification(line.Offer),
		})
	}
	content, err := json.Marshal(payload)
	if err != nil {
		return wallet.StaticQuote{}, err
	}
	token := r.staticCipher.Seal(nil, nil, content, []byte("static-quote/"+tenant))
	return wallet.StaticQuote{Lines: lines, ExpiresAt: payload.ExpiresAt,
		ConfirmationToken: base64.RawURLEncoding.EncodeToString(token)}, nil
}

func (r *WalletRepository) confirmStaticQuote(tenant string, p wallet.StaticPurchaseRequest) (staticQuoteToken, error) {
	token, err := base64.RawURLEncoding.DecodeString(p.ConfirmationToken)
	if err != nil || r.staticCipher == nil {
		return staticQuoteToken{}, wallet.ErrStaticQuoteExpired
	}
	content, err := r.staticCipher.Open(nil, nil, token, []byte("static-quote/"+tenant))
	if err != nil {
		return staticQuoteToken{}, wallet.ErrStaticQuoteExpired
	}
	var quote staticQuoteToken
	if json.Unmarshal(content, &quote) != nil || !time.Now().Before(quote.ExpiresAt) ||
		quote.Term != p.Term || quote.ShareMode != p.ShareMode || quote.PurposeID != p.PurposeID {
		return staticQuoteToken{}, wallet.ErrStaticQuoteExpired
	}
	if len(quote.Lines) == 0 {
		quote.Lines = []staticQuotedLine{{
			StaticPurchaseLine: wallet.StaticPurchaseLine{NodeID: quote.NodeID, Quantity: quote.Quantity, UnitMinor: quote.UnitMinor},
			Available:          quote.Available,
		}}
	}
	if !slices.EqualFunc(quote.Lines, p.LineItems(), func(quoted staticQuotedLine, submitted wallet.StaticPurchaseLine) bool {
		return quoted.StaticPurchaseLine == submitted
	}) {
		return staticQuoteToken{}, wallet.ErrStaticQuoteExpired
	}
	return quote, nil
}

func (r *WalletRepository) currentStaticQuotes(ctx context.Context, p wallet.StaticPurchaseRequest) ([]wallet.StaticQuoteLine, error) {
	offers, err := r.liveStaticOffers(ctx, p.ShareMode, p.PurposeID)
	if err != nil {
		return nil, err
	}
	byNode := make(map[int64]wallet.StaticOffer)
	for _, offer := range offers {
		if offer.Term == p.Term {
			byNode[offer.NodeID] = offer
		}
	}
	var lines []wallet.StaticQuoteLine
	for _, selection := range p.LineItems() {
		offer, exists := byNode[selection.NodeID]
		if !exists {
			return nil, wallet.ErrStaticOfferMissing
		}
		if err := offer.CheckAvailability(selection.Quantity); err != nil {
			return nil, err
		}
		lines = append(lines, wallet.StaticQuoteLine{Offer: offer, Quantity: selection.Quantity})
	}
	return lines, nil
}

func (r *WalletRepository) staticPurchaseCommand(ctx context.Context, tenant string, p wallet.StaticPurchaseRequest,
	quote staticQuoteToken) (wallet.Command, error) {
	lines, err := r.currentStaticQuotes(ctx, p)
	if err != nil {
		return wallet.Command{}, err
	}
	command := wallet.Command{TenantID: tenant, Key: p.Key, Action: wallet.Hold, Kind: wallet.StaticPurchase}
	for i, line := range lines {
		offer := line.Offer
		confirmed := quote.Lines[i]
		if err := offer.CheckConfirmedPriceAndStock(confirmed.UnitMinor, confirmed.Available); err != nil {
			return wallet.Command{}, err
		}
		if (confirmed.TermDays != 0 && offer.TermDays != confirmed.TermDays) ||
			(confirmed.Specification != "" && offerSpecification(offer) != confirmed.Specification) {
			return wallet.Command{}, wallet.ErrStaticQuoteExpired
		}
		if offer.UnitMinor > (math.MaxInt64-command.Amount)/int64(line.Quantity) {
			return wallet.Command{}, wallet.ErrInvalid
		}
		command.Amount += offer.UnitMinor * int64(line.Quantity)
		command.Items = append(command.Items, wallet.OrderItem{
			Reference: p.OfferReference(offer.NodeID), Specification: offer.Region + " · " + offer.IPType,
			StaticRegion: offer.Region, StaticIPType: offer.IPType,
			TermDays: offer.TermDays, UnitMinor: offer.UnitMinor, Quantity: line.Quantity,
		})
	}
	return command, nil
}

func (r *WalletRepository) PurchaseStatic(ctx context.Context, tenant string, p wallet.StaticPurchaseRequest) (wallet.Order, error) {
	if r.db == nil {
		return wallet.Order{}, errWalletUnavailable
	}
	fingerprint, err := walletFingerprint(p)
	if err != nil {
		return wallet.Order{}, err
	}
	previous, exists, err := walletReplay(r.db.WithContext(ctx), tenant, p.Key, fingerprint)
	if err != nil {
		return wallet.Order{}, err
	}
	if exists {
		return r.Order(ctx, tenant, previous.OrderID)
	}

	var quote staticQuoteToken
	err = RequireProductSale(ctx, r.db, product.Static)
	if err == nil {
		quote, err = r.confirmStaticQuote(tenant, p)
	}
	var command wallet.Command
	if err == nil {
		command, err = r.staticPurchaseCommand(ctx, tenant, p, quote)
	}
	if err != nil {
		previous, exists, replayErr := walletReplay(r.db.WithContext(ctx), tenant, p.Key, fingerprint)
		if replayErr != nil {
			return wallet.Order{}, replayErr
		}
		if exists {
			return r.Order(ctx, tenant, previous.OrderID)
		}
		return wallet.Order{}, err
	}
	var accepted wallet.Order
	entry, err := r.apply(ctx, command, p, nil, func(tx *gorm.DB, _ *wallet.Command) error {
		if err := RequireProductSale(ctx, tx, product.Static); err != nil {
			return err
		}
		if !time.Now().Before(quote.ExpiresAt) {
			return wallet.ErrStaticQuoteExpired
		}
		return nil
	}, func(tx *gorm.DB, e wallet.Entry) error {
		if err := tx.Exec("INSERT INTO static_purchases (tenant_id, order_id) VALUES (?, ?)", tenant, e.OrderID).Error; err != nil {
			return err
		}
		var err error
		accepted, err = findPurchaseOrder(tx, tenant, e.OrderID, false)
		return err
	})
	if err != nil {
		return wallet.Order{}, err
	}
	if accepted.ID == 0 {
		return r.Order(ctx, tenant, entry.OrderID)
	}

	// After commit the request key can only read the order. An ambiguous
	// supplier response is reconciled by operations without resubmitting.
	process, cancel := context.WithTimeout(context.WithoutCancel(ctx), 25*time.Second)
	defer cancel()
	delivery, err := r.static.Place(process, p)
	if err != nil {
		slog.WarnContext(process, "static supplier purchase result is unknown", "order_id", accepted.ID)
		return accepted, nil
	}
	if err := r.recordStaticDelivery(process, tenant, entry.OrderID, delivery, "", "", delivery.Rejected, false); err != nil {
		slog.WarnContext(process, "static purchase delivery could not be recorded", "order_id", accepted.ID)
		return accepted, nil
	}
	order, err := r.Order(process, tenant, entry.OrderID)
	if err != nil {
		slog.WarnContext(process, "static purchase settlement could not be read", "order_id", accepted.ID)
		return accepted, nil
	}
	return order, nil
}

func (r *WalletRepository) StaticAllocations(ctx context.Context, tenant string, ids []string, onlyActive bool) ([]wallet.StaticAllocation, error) {
	if r.db == nil {
		return nil, errWalletUnavailable
	}
	var rows []storedStaticIP
	db := r.db.WithContext(ctx).Table("static_ip_allocations AS a").Where("a.tenant_id = ?", tenant)
	if len(ids) == 0 {
		db = db.Select(`a.allocation_id, a.order_id, a.ip, a.port, a.expires_at,
			i.specification AS purchase_specification, i.static_region, i.static_ip_type, i.reference AS original_offer,
			COALESCE(r.order_id, 0) AS pending_renewal_order_id,
			COALESCE(x.status, '') AS exchange_status, COALESCE(x.idempotency_key, '') AS exchange_idempotency_key`).
			Joins("JOIN order_items i ON i.tenant_id=a.tenant_id AND i.order_id=a.order_id AND i.position=a.order_item_position").
			Joins("LEFT JOIN static_renewal_items r ON r.tenant_id=a.tenant_id AND r.allocation_id=a.allocation_id AND r.outcome='pending'").
			Joins(`LEFT JOIN LATERAL (SELECT status, idempotency_key FROM static_ip_exchanges
				WHERE tenant_id=a.tenant_id AND allocation_id=a.allocation_id
				ORDER BY created_at DESC, idempotency_key DESC LIMIT 1) x ON true`)
	} else {
		db = db.Where("a.allocation_id IN ?", ids)
	}
	if onlyActive {
		db = db.Where("a.expires_at > clock_timestamp()")
	}
	if err := db.Order("a.expires_at DESC, a.allocation_id").Scan(&rows).Error; err != nil {
		return nil, err
	}
	allocations := make([]wallet.StaticAllocation, 0, len(rows))
	for _, row := range rows {
		if len(ids) == 0 {
			row.Region, row.IPType = row.StaticRegion, row.StaticIPType
			nodeID, shareMode, _, valid := wallet.ParseStaticPurchaseReference(row.OriginalOffer)
			if valid {
				row.NodeID = nodeID
				row.ShareMode = shareMode
				if row.ShareMode == "" {
					row.ShareMode = "exclusive"
				}
			}
			if row.Region == "" && row.IPType == "" {
				row.Region, row.IPType, _ = strings.Cut(row.PurchaseSpecification, " · ")
			}
			allocations = append(allocations, row.StaticAllocation)
			continue
		}
		if r.staticCipher == nil {
			return nil, errWalletUnavailable
		}
		password, err := r.staticCipher.Open(nil, nil, row.PasswordCiphertext, staticPasswordScope(tenant, row.SupplierItemID))
		if err != nil {
			return nil, err
		}
		row.Password = string(password)
		allocations = append(allocations, row.StaticAllocation)
	}
	return allocations, nil
}

func (r *WalletRepository) ResolveStatic(ctx context.Context, tenant string, id int64, operator, supplierID, evidence string, closeRemaining, confirmedUndeliverable bool) (wallet.Order, error) {
	if r.db == nil {
		return wallet.Order{}, errWalletUnavailable
	}
	var supplierOrderID string
	err := scanWalletRecord(r.db.WithContext(ctx).Raw(`
        SELECT COALESCE(s.supplier_order_id, '') FROM static_purchases s
        JOIN purchase_orders o ON o.id = s.order_id AND o.tenant_id = s.tenant_id
        WHERE s.tenant_id = ? AND s.order_id = ? AND o.status = 'pending'
        AND (? OR o.created_at <= clock_timestamp() - interval '24 hours')`, tenant, id, confirmedUndeliverable), &supplierOrderID)
	if err != nil {
		return wallet.Order{}, err
	}
	if supplierID == "" {
		supplierID = supplierOrderID
	}
	if supplierOrderID != "" && supplierOrderID != supplierID {
		return wallet.Order{}, wallet.ErrState
	}
	var delivery wallet.StaticDelivery
	if supplierID != "" {
		if r.static != nil {
			delivery, err = r.static.Detail(ctx, supplierID)
		} else {
			err = wallet.ErrStaticUnavailable
		}
		if err != nil && !closeRemaining {
			return wallet.Order{}, wallet.ErrStaticUnavailable
		}
	} else if !closeRemaining {
		return wallet.Order{}, wallet.ErrInvalid
	}
	if err := r.recordStaticDelivery(ctx, tenant, id, delivery, operator, evidence, closeRemaining, closeRemaining && !confirmedUndeliverable); err != nil {
		return wallet.Order{}, err
	}
	return r.Order(ctx, tenant, id)
}

func (r *WalletRepository) recordStaticDelivery(ctx context.Context, tenant string, id int64, delivery wallet.StaticDelivery, operator, evidence string, closeRemaining, requireDeadline bool) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		order, err := findPurchaseOrder(tx, tenant, id, true)
		if err != nil {
			return err
		}
		if order.Kind != wallet.StaticPurchase || order.Status != wallet.Pending || len(order.Items) == 0 {
			return wallet.ErrState
		}
		if requireDeadline && time.Since(order.CreatedAt) < 72*time.Hour {
			return wallet.ErrState
		}
		if delivery.SupplierOrderID != "" {
			result := tx.Exec(`UPDATE static_purchases SET supplier_order_id = ?
                WHERE tenant_id = ? AND order_id = ? AND (supplier_order_id IS NULL OR supplier_order_id = ?)`,
				delivery.SupplierOrderID, tenant, id, delivery.SupplierOrderID)
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected != 1 {
				return wallet.ErrState
			}
		}
		positions := make(map[int64]int32, len(order.Items))
		for _, item := range order.Items {
			nodeID, _, _, valid := wallet.ParseStaticPurchaseReference(item.Reference)
			if !valid || positions[nodeID] != 0 {
				return wallet.ErrState
			}
			positions[nodeID] = item.Position
		}
		if len(delivery.Allocations) > 0 {
			var locked []string
			if err := tx.Raw(`SELECT allocation_id FROM static_ip_allocations
				WHERE tenant_id=? AND order_id=? ORDER BY allocation_id FOR UPDATE`, tenant, id).Scan(&locked).Error; err != nil {
				return err
			}
		}
		for _, ip := range delivery.Allocations {
			position, ok := positions[ip.NodeID]
			if !ok {
				return wallet.ErrState
			}
			if r.staticCipher == nil {
				return errWalletUnavailable
			}
			password := r.staticCipher.Seal(nil, nil, []byte(ip.Password), staticPasswordScope(tenant, ip.SupplierItemID))
			result := tx.Exec(`WITH exchanged AS (
                    SELECT 1 FROM static_ip_exchanges x JOIN static_ip_allocations a
                        ON a.tenant_id=x.tenant_id AND a.allocation_id=x.allocation_id
                    WHERE a.tenant_id=? AND a.supplier_item_id=?
                        AND x.status IN ('pending', 'unknown', 'activating', 'ready', 'support_required') LIMIT 1
                )
                INSERT INTO static_ip_allocations AS a
                    (supplier_item_id, tenant_id, order_id, order_item_position, proxy_id, ip, port, username, password_ciphertext, expires_at)
                VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
                ON CONFLICT (supplier_item_id) DO UPDATE SET
                    expires_at=CASE WHEN EXISTS (SELECT 1 FROM exchanged) THEN a.expires_at
                        ELSE GREATEST(a.expires_at, EXCLUDED.expires_at) END,
                    proxy_id=CASE WHEN a.proxy_id='' AND NOT EXISTS (SELECT 1 FROM exchanged)
                        THEN EXCLUDED.proxy_id ELSE a.proxy_id END
                WHERE a.tenant_id=EXCLUDED.tenant_id AND a.order_id=EXCLUDED.order_id
                    AND a.order_item_position=EXCLUDED.order_item_position
                    AND (a.ip=EXCLUDED.ip OR EXISTS (SELECT 1 FROM exchanged))`,
				tenant, ip.SupplierItemID, ip.SupplierItemID, tenant, id, position, ip.ProxyID, ip.IP, ip.Port, ip.Username, password, ip.ExpiresAt)
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected != 1 {
				return wallet.ErrState
			}
		}
		var deliveries []struct{ Position, Count int32 }
		if err := tx.Raw(`SELECT order_item_position AS position, COUNT(*) AS count FROM static_ip_allocations
            WHERE tenant_id = ? AND order_id = ? GROUP BY order_item_position`, tenant, id).Scan(&deliveries).Error; err != nil {
			return err
		}
		byPosition := make(map[int32]int32, len(deliveries))
		for _, delivery := range deliveries {
			byPosition[delivery.Position] = delivery.Count
		}
		itemDeliveries := make([]int32, len(order.Items))
		var retainedMinor int64
		order.DeliveredCount = 0
		for index := range order.Items {
			item := &order.Items[index]
			count := byPosition[item.Position]
			if count > item.Quantity {
				return wallet.ErrState
			}
			item.DeliveredCount = count
			itemDeliveries[index] = count
			order.DeliveredCount += int64(count)
			retainedMinor += item.UnitMinor * int64(count)
		}
		if operator != "" {
			if err := tx.Exec(`INSERT INTO static_purchase_resolutions (tenant_id, order_id, operator_id, evidence)
                VALUES (?, ?, ?, ?)`, tenant, id, operator, evidence).Error; err != nil {
				return err
			}
		}
		if retainedMinor < order.LockedMinor && !closeRemaining {
			_, err := saveSettlementOrder(tx, tenant, order)
			return err
		}
		var before int64
		if err := scanWalletRecord(tx.Raw("SELECT available_minor FROM wallets WHERE tenant_id = ? FOR UPDATE", tenant), &before); err != nil {
			return err
		}
		command := wallet.Command{TenantID: tenant, OrderID: id, Key: uuid.NewString(), Action: wallet.Release,
			Amount: retainedMinor, ItemDeliveries: itemDeliveries}
		fingerprint, err := walletFingerprint(command)
		if err != nil {
			return err
		}
		_, err = applyWalletCommand(tx, command, order, before, fingerprint)
		return err
	})
}

func (r *WalletRepository) PendingStaticPurchases(ctx context.Context, afterID int64) ([]wallet.PendingStaticPurchase, error) {
	if r.db == nil {
		return nil, errWalletUnavailable
	}
	var purchases []wallet.PendingStaticPurchase
	err := r.db.WithContext(ctx).Raw(`
        SELECT o.id AS order_id, o.tenant_id, COALESCE(s.supplier_order_id, '') AS supplier_order_id,
            o.created_at, o.delivered_count, o.created_at <= clock_timestamp() - interval '72 hours' AS can_close,
            COALESCE((SELECT string_agg(r.allocation_id, ',' ORDER BY r.position)
                FROM static_renewal_items r WHERE r.order_id=o.id AND r.outcome='pending'), '') AS pending_renewal_items
        FROM purchase_orders o LEFT JOIN static_purchases s ON s.tenant_id=o.tenant_id AND s.order_id=o.id
        WHERE (s.order_id IS NOT NULL OR o.kind='static_renewal') AND o.status='pending'
            AND o.created_at <= clock_timestamp() - interval '24 hours' AND o.id > ?
        ORDER BY o.id LIMIT ?`, afterID, wallet.PageSize+1).Scan(&purchases).Error
	return purchases, err
}

const expiringStaticIP = "expires_at > statement_timestamp() AND expires_at <= statement_timestamp() + interval '120 hours'"

func (r *WalletRepository) StaticSummary(ctx context.Context, tenant string) (wallet.StaticSummary, error) {
	if r.db == nil {
		return wallet.StaticSummary{}, errWalletUnavailable
	}
	var summary wallet.StaticSummary
	err := r.db.WithContext(ctx).Raw(`
		SELECT COUNT(*) AS active,
			COUNT(*) FILTER (WHERE `+expiringStaticIP+`) AS expiring
		FROM static_ip_allocations
		WHERE tenant_id = ? AND expires_at > statement_timestamp()`, tenant).Scan(&summary).Error
	return summary, err
}

func (r *WalletRepository) ClaimStaticIPExpiryNotices(ctx context.Context, tenant string) (int32, error) {
	if r.db == nil {
		return 0, errWalletUnavailable
	}
	var count int32
	err := scanWalletRecord(r.db.WithContext(ctx).Raw(`
        WITH received AS (
            UPDATE static_ip_allocations SET reminded_expiry = expires_at
            WHERE tenant_id = ? AND `+expiringStaticIP+`
            AND reminded_expiry IS DISTINCT FROM expires_at
            RETURNING 1
        ) SELECT COUNT(*) FROM received`, tenant), &count)
	return count, err
}
