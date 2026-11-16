package repo

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"log/slog"
	"math"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/biz/wallet"
	"gorm.io/gorm"
)

type staticRenewalAllocation struct {
	SupplierItemID, SupplierOrderID, IP, OriginalOffer string
	ExpiresAt                                          time.Time
	Pending                                            bool
}

type staticRenewalItem struct {
	SupplierItemID, SupplierOrderID, OriginalIP, Outcome string
	OriginalExpiry, ExpectedExpiry                       time.Time
	Position                                             int32
}

func (r *WalletRepository) QuoteStaticRenewal(ctx context.Context, tenant string, p wallet.StaticRenewalRequest) (wallet.StaticRenewalQuote, error) {
	quote, _, err := r.renewalQuote(ctx, tenant, p)
	if err != nil {
		return wallet.StaticRenewalQuote{}, err
	}
	quote.ExpiresAt = time.Now().UTC().Add(5 * time.Minute)
	data, err := json.Marshal(quote)
	if err != nil {
		return wallet.StaticRenewalQuote{}, err
	}
	quote.ConfirmationToken = base64.RawURLEncoding.EncodeToString(r.staticCipher.Seal(nil, nil, data, []byte("static-renewal/"+tenant)))
	return quote, nil
}

func (r *WalletRepository) renewalQuote(ctx context.Context, tenant string, p wallet.StaticRenewalRequest) (wallet.StaticRenewalQuote, []staticRenewalAllocation, error) {
	if r.db == nil {
		return wallet.StaticRenewalQuote{}, nil, errWalletUnavailable
	}
	if r.static == nil || r.staticCipher == nil {
		return wallet.StaticRenewalQuote{}, nil, wallet.ErrStaticUnavailable
	}
	allocations, err := renewalAllocations(r.db.WithContext(ctx), tenant, p.IDs, false)
	if err != nil {
		return wallet.StaticRenewalQuote{}, nil, err
	}
	// IPCrown confirmed that the current purchase catalog also prices renewals.
	// New purchase availability and the new-purchase switch do not govern renewals.
	offers, err := r.static.Offers(ctx)
	if err != nil {
		return wallet.StaticRenewalQuote{}, nil, wallet.ErrStaticUnavailable
	}
	quote := wallet.StaticRenewalQuote{Term: p.Term}
	prices := make(map[int64]wallet.StaticOffer)
	for _, offer := range offers {
		if offer.Term == p.Term {
			prices[offer.NodeID] = offer
		}
	}
	for _, a := range allocations {
		node, _, _ := strings.Cut(a.OriginalOffer, "/")
		nodeID, err := strconv.ParseInt(node, 10, 64)
		offer := prices[nodeID]
		if err != nil || offer.UnitMinor <= 0 || offer.TermDays <= 0 {
			return wallet.StaticRenewalQuote{}, nil, wallet.ErrStaticRenewalPrice
		}
		quote.Lines = append(quote.Lines, wallet.StaticRenewalLine{ID: a.SupplierItemID, IP: a.IP,
			UnitMinor: offer.UnitMinor, TermDays: offer.TermDays, OriginalExpiry: a.ExpiresAt.UTC()})
	}
	return quote, allocations, nil
}

func renewalAllocations(tx *gorm.DB, tenant string, ids []string, lock bool) ([]staticRenewalAllocation, error) {
	query := `SELECT a.supplier_item_id, p.supplier_order_id, a.ip, a.expires_at, i.reference AS original_offer,
        EXISTS (SELECT 1 FROM static_renewal_items r WHERE r.supplier_item_id=a.supplier_item_id AND r.outcome='pending') AS pending
        FROM static_ip_allocations a
        JOIN static_purchases p ON p.tenant_id=a.tenant_id AND p.order_id=a.order_id
        JOIN order_items i ON i.tenant_id=a.tenant_id AND i.order_id=a.order_id AND i.position=1
        WHERE a.tenant_id=? AND a.supplier_item_id IN ? ORDER BY a.supplier_item_id`
	if lock {
		query += " FOR UPDATE OF a"
	}
	var rows []staticRenewalAllocation
	if err := tx.Raw(query, tenant, ids).Scan(&rows).Error; err != nil {
		return nil, err
	}
	if len(rows) != len(ids) {
		return nil, wallet.ErrMissing
	}
	now := time.Now()
	for _, row := range rows {
		if row.Pending {
			return nil, wallet.ErrStaticRenewalPending
		}
		if !now.Before(row.ExpiresAt) {
			return nil, wallet.ErrStaticRenewalExpired
		}
		if row.SupplierOrderID == "" {
			return nil, wallet.ErrState
		}
	}
	return rows, nil
}

func (r *WalletRepository) confirmRenewalQuote(tenant string, p wallet.StaticRenewalRequest) (wallet.StaticRenewalQuote, error) {
	var quote wallet.StaticRenewalQuote
	raw, err := base64.RawURLEncoding.DecodeString(p.ConfirmationToken)
	if err != nil || r.staticCipher == nil {
		return quote, wallet.ErrStaticQuoteExpired
	}
	data, err := r.staticCipher.Open(nil, nil, raw, []byte("static-renewal/"+tenant))
	if err != nil || json.Unmarshal(data, &quote) != nil || !time.Now().Before(quote.ExpiresAt) || quote.Term != p.Term || len(quote.Lines) != len(p.IDs) {
		return quote, wallet.ErrStaticQuoteExpired
	}
	for i, line := range quote.Lines {
		if line.ID != p.IDs[i] {
			return quote, wallet.ErrStaticQuoteExpired
		}
	}
	return quote, nil
}

func (r *WalletRepository) RenewStatic(ctx context.Context, tenant string, p wallet.StaticRenewalRequest) (wallet.Order, error) {
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
	confirmed, err := r.confirmRenewalQuote(tenant, p)
	if err != nil {
		return wallet.Order{}, err
	}
	quote, allocations, err := r.renewalQuote(ctx, tenant, p)
	if err == nil {
		if slices.Equal(quote.Lines, confirmed.Lines) {
			err = r.verifyRenewalAllocations(ctx, allocations)
		} else {
			err = wallet.ErrStaticQuoteExpired
		}
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
	command := wallet.Command{TenantID: tenant, Key: p.Key, Action: wallet.Hold, Kind: wallet.StaticRenewal}
	for _, line := range quote.Lines {
		if line.UnitMinor > math.MaxInt64-command.Amount {
			return wallet.Order{}, wallet.ErrInvalid
		}
		command.Amount += line.UnitMinor
		command.Items = append(command.Items, wallet.OrderItem{Reference: line.ID, Specification: line.IP,
			TermDays: line.TermDays, UnitMinor: line.UnitMinor, Quantity: 1})
	}
	var accepted wallet.Order
	entry, err := r.apply(ctx, command, p, func(tx *gorm.DB, _ *wallet.Command) error {
		current, err := renewalAllocations(tx, tenant, p.IDs, true)
		if err != nil {
			return err
		}
		if !time.Now().Before(confirmed.ExpiresAt) || !slices.Equal(current, allocations) {
			return wallet.ErrStaticQuoteExpired
		}
		return nil
	}, func(tx *gorm.DB, receipt wallet.Entry) error {
		for i, allocation := range allocations {
			expected := allocation.ExpiresAt.Add(time.Duration(quote.Lines[i].TermDays) * 24 * time.Hour)
			if err := tx.Exec(`INSERT INTO static_renewal_items (tenant_id, order_id, position, supplier_item_id,
                supplier_order_id, original_ip, original_expiry, expected_expiry) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
				tenant, receipt.OrderID, i+1, allocation.SupplierItemID, allocation.SupplierOrderID, allocation.IP,
				allocation.ExpiresAt, expected).Error; err != nil {
				return err
			}
		}
		var err error
		accepted, err = findPurchaseOrder(tx, tenant, receipt.OrderID, false)
		return err
	})
	if err != nil {
		return wallet.Order{}, err
	}
	if accepted.ID == 0 {
		return r.Order(ctx, tenant, entry.OrderID)
	}
	// Only the transaction that created the hold submits to IPCrown.
	process, cancel := context.WithTimeout(context.WithoutCancel(ctx), 35*time.Second)
	defer cancel()
	response, err := r.static.Renew(process, p.IDs, p.Term)
	if err != nil {
		slog.WarnContext(process, "static renewal response requires reconciliation", "order_id", accepted.ID)
	}
	order, err := r.reconcileStaticRenewal(process, tenant, accepted.ID, response, "", wallet.StaticRenewalResolution{})
	if err != nil {
		slog.WarnContext(process, "accepted static renewal requires reconciliation", "order_id", accepted.ID)
		return accepted, nil
	}
	return order, nil
}

func (r *WalletRepository) verifyRenewalAllocations(ctx context.Context, allocations []staticRenewalAllocation) error {
	details := make(map[string]wallet.StaticDelivery)
	for _, allocation := range allocations {
		detail, found := details[allocation.SupplierOrderID]
		if !found {
			var err error
			detail, err = r.static.Detail(ctx, allocation.SupplierOrderID)
			if err != nil {
				return wallet.ErrStaticUnavailable
			}
			details[allocation.SupplierOrderID] = detail
		}
		if !slices.ContainsFunc(detail.Allocations, func(item wallet.StaticAllocation) bool {
			return item.SupplierItemID == allocation.SupplierItemID && item.IP == allocation.IP && item.ExpiresAt.Equal(allocation.ExpiresAt)
		}) {
			return wallet.ErrStaticQuoteExpired
		}
	}
	return nil
}

func (r *WalletRepository) RecheckStaticRenewal(ctx context.Context, tenant string, id int64) (wallet.Order, error) {
	return r.reconcileStaticRenewal(ctx, tenant, id, wallet.SupplierRenewal{}, "", wallet.StaticRenewalResolution{})
}

func (r *WalletRepository) ResolveStaticRenewal(ctx context.Context, operator string, resolution wallet.StaticRenewalResolution) (wallet.Order, error) {
	return r.reconcileStaticRenewal(ctx, resolution.TenantID, resolution.OrderID, wallet.SupplierRenewal{}, operator, resolution)
}

func (r *WalletRepository) reconcileStaticRenewal(ctx context.Context, tenant string, id int64, response wallet.SupplierRenewal, actor string, resolution wallet.StaticRenewalResolution) (wallet.Order, error) {
	existing, err := r.Order(ctx, tenant, id)
	if err != nil {
		return wallet.Order{}, err
	}
	if existing.Kind != wallet.StaticRenewal {
		return wallet.Order{}, wallet.ErrMissing
	}
	if existing.Status != wallet.Pending {
		return existing, nil
	}
	if resolution.CloseUnknown && time.Since(existing.CreatedAt) < 72*time.Hour {
		return wallet.Order{}, wallet.ErrState
	}
	var rows []staticRenewalItem
	if err := r.db.WithContext(ctx).Table("static_renewal_items").Where("tenant_id=? AND order_id=?", tenant, id).
		Order("position").Scan(&rows).Error; err != nil {
		return wallet.Order{}, err
	}
	if len(rows) != len(existing.Items) || len(rows) == 0 {
		return wallet.Order{}, wallet.ErrState
	}
	if actor != "" && !slices.ContainsFunc(rows, func(item staticRenewalItem) bool { return item.SupplierItemID == resolution.ItemID }) {
		return wallet.Order{}, wallet.ErrMissing
	}
	results := r.renewalOutcomes(ctx, rows, response)
	if (resolution.CloseUnknown || resolution.ConfirmedFailure) && results[resolution.ItemID] != wallet.Succeeded {
		results[resolution.ItemID] = wallet.Failed
	}
	if len(results) == 0 {
		return existing, nil
	}
	return r.saveStaticRenewalOutcomes(ctx, tenant, id, rows, results, actor, resolution)
}

func (r *WalletRepository) renewalOutcomes(ctx context.Context, rows []staticRenewalItem, response wallet.SupplierRenewal) map[string]string {
	details := make(map[string]wallet.StaticDelivery)
	results := make(map[string]string)
	for _, row := range rows {
		if row.Outcome != wallet.Pending {
			continue
		}
		if response.Rejected {
			results[row.SupplierItemID] = wallet.Failed
			continue
		}
		if r.static == nil {
			continue
		}
		if expiry, found := response.Expiries[row.SupplierItemID]; found && !expiry.Equal(row.ExpectedExpiry) {
			continue
		}
		detail, ok := details[row.SupplierOrderID]
		if !ok {
			detail, _ = r.static.Detail(ctx, row.SupplierOrderID)
			details[row.SupplierOrderID] = detail
		}
		for _, a := range detail.Allocations {
			if a.SupplierItemID == row.SupplierItemID && a.IP == row.OriginalIP && a.ExpiresAt.Equal(row.ExpectedExpiry) {
				results[row.SupplierItemID] = wallet.Succeeded
			}
		}
	}
	return results
}
