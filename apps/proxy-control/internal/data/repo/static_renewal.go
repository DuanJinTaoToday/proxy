package repo

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"log/slog"
	"maps"
	"math"
	"slices"
	"time"

	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/biz/wallet"
	"gorm.io/gorm"
)

type staticRenewalAllocation struct {
	AllocationID, SupplierItemID, SupplierOrderID, IP, OriginalOffer string
	ExpiresAt                                                        time.Time
	Pending                                                          bool
}

type staticRenewalProduct struct {
	shareMode string
	purposeID int32
}

type staticRenewalNode struct {
	product staticRenewalProduct
	nodeID  int64
}

type staticRenewalItem struct {
	AllocationID, SupplierItemID, SupplierOrderID, OriginalIP, Outcome string
	OriginalExpiry, ExpectedExpiry                                     time.Time
	Position                                                           int32
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
	allocations, prices, err := r.renewalPrices(ctx, tenant, p.IDs)
	if err != nil {
		return wallet.StaticRenewalQuote{}, nil, err
	}
	quote := wallet.StaticRenewalQuote{Term: p.Term}
	for i, a := range allocations {
		days, found := prices[i][p.Term]
		if !found || (len(quote.Lines) > 0 && quote.Lines[0].TermDays != days.TermDays) {
			return wallet.StaticRenewalQuote{}, nil, wallet.ErrStaticRenewalPrice
		}
		quote.Lines = append(quote.Lines, wallet.StaticRenewalLine{ID: a.AllocationID, IP: a.IP,
			UnitMinor: days.UnitMinor, TermDays: days.TermDays, OriginalExpiry: a.ExpiresAt.UTC()})
	}
	return quote, allocations, nil
}

func (r *WalletRepository) ListStaticRenewalTerms(ctx context.Context, tenant string, ids []string) ([]wallet.StaticRenewalTerm, error) {
	_, prices, err := r.renewalPrices(ctx, tenant, ids)
	if err != nil {
		return nil, err
	}
	var terms []wallet.StaticRenewalTerm
	for term, offer := range prices[0] {
		if slices.ContainsFunc(prices[1:], func(offers map[string]wallet.StaticOffer) bool {
			return offers[term].TermDays != offer.TermDays
		}) {
			continue
		}
		terms = append(terms, wallet.StaticRenewalTerm{Term: term, TermDays: offer.TermDays})
	}
	slices.SortFunc(terms, func(a, b wallet.StaticRenewalTerm) int {
		if a.TermDays != b.TermDays {
			return int(a.TermDays - b.TermDays)
		}
		if a.Term < b.Term {
			return -1
		}
		if a.Term > b.Term {
			return 1
		}
		return 0
	})
	return terms, nil
}

func (r *WalletRepository) renewalPrices(ctx context.Context, tenant string, ids []string) ([]staticRenewalAllocation, []map[string]wallet.StaticOffer, error) {
	if r.db == nil {
		return nil, nil, errWalletUnavailable
	}
	if r.static == nil || r.staticCipher == nil {
		return nil, nil, wallet.ErrStaticUnavailable
	}
	allocations, err := renewalAllocations(r.db.WithContext(ctx), tenant, ids, false)
	if err != nil {
		return nil, nil, err
	}
	// IPCrown confirmed that the current purchase catalog also prices renewals.
	// New purchase availability and the new-purchase switch do not govern renewals.
	catalogs := make(map[staticRenewalProduct][]wallet.StaticOffer)
	nodes := make(map[staticRenewalNode]map[string]wallet.StaticOffer)
	prices := make([]map[string]wallet.StaticOffer, 0, len(allocations))
	for _, allocation := range allocations {
		nodeID, shareMode, purposeID, ok := wallet.ParseStaticPurchaseReference(allocation.OriginalOffer)
		if !ok {
			return nil, nil, wallet.ErrStaticRenewalPrice
		}
		product := staticRenewalProduct{shareMode: shareMode, purposeID: purposeID}
		node := staticRenewalNode{product: product, nodeID: nodeID}
		if _, found := nodes[node]; !found {
			offers, found := catalogs[product]
			if !found {
				offers, err = r.static.Offers(ctx, shareMode, purposeID)
				if err != nil {
					return nil, nil, wallet.ErrStaticUnavailable
				}
				catalogs[product] = offers
			}
			byTerm := make(map[string]wallet.StaticOffer)
			for _, offer := range offers {
				if offer.NodeID == nodeID && offer.UnitMinor > 0 && offer.TermDays > 0 && offer.Term != "" {
					byTerm[offer.Term] = offer
				}
			}
			nodes[node] = byTerm
		}
		prices = append(prices, nodes[node])
	}
	return allocations, prices, nil
}

func renewalAllocations(tx *gorm.DB, tenant string, ids []string, lock bool) ([]staticRenewalAllocation, error) {
	query := `SELECT a.allocation_id, a.supplier_item_id, p.supplier_order_id, a.ip, a.expires_at, i.reference AS original_offer,
        EXISTS (SELECT 1 FROM static_renewal_items r WHERE r.tenant_id=a.tenant_id AND r.allocation_id=a.allocation_id AND r.outcome='pending') AS pending
        FROM static_ip_allocations a
        JOIN static_purchases p ON p.tenant_id=a.tenant_id AND p.order_id=a.order_id
        JOIN order_items i ON i.tenant_id=a.tenant_id AND i.order_id=a.order_id AND i.position=a.order_item_position
        WHERE a.tenant_id=? AND a.allocation_id IN ? ORDER BY a.allocation_id`
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
			if err := tx.Exec(`INSERT INTO static_renewal_items (tenant_id, order_id, position, allocation_id,
                supplier_order_id, original_ip, original_expiry, expected_expiry) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
				tenant, receipt.OrderID, i+1, allocation.AllocationID, allocation.SupplierOrderID, allocation.IP,
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
	groups := make(map[string][]string)
	for _, allocation := range allocations {
		_, shareMode, _, _ := wallet.ParseStaticPurchaseReference(allocation.OriginalOffer)
		groups[shareMode] = append(groups[shareMode], allocation.SupplierItemID)
	}
	responses := make(map[string]wallet.SupplierRenewal, len(allocations))
	for _, shareMode := range slices.Sorted(maps.Keys(groups)) {
		ids := groups[shareMode]
		slices.Sort(ids)
		response, err := r.static.Renew(process, ids, p.Term, shareMode)
		if err != nil {
			slog.WarnContext(process, "static renewal response requires reconciliation", "order_id", accepted.ID)
			continue
		}
		for _, id := range ids {
			responses[id] = response
		}
	}
	order, err := r.reconcileStaticRenewal(process, tenant, accepted.ID, responses, "", wallet.StaticRenewalResolution{})
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
	return r.reconcileStaticRenewal(ctx, tenant, id, nil, "", wallet.StaticRenewalResolution{})
}

func (r *WalletRepository) ResolveStaticRenewal(ctx context.Context, operator string, resolution wallet.StaticRenewalResolution) (wallet.Order, error) {
	return r.reconcileStaticRenewal(ctx, resolution.TenantID, resolution.OrderID, nil, operator, resolution)
}

func (r *WalletRepository) reconcileStaticRenewal(ctx context.Context, tenant string, id int64, responses map[string]wallet.SupplierRenewal, actor string, resolution wallet.StaticRenewalResolution) (wallet.Order, error) {
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
	if err := r.db.WithContext(ctx).Table("static_renewal_items AS r").
		Select("r.*, a.supplier_item_id").
		Joins("JOIN static_ip_allocations a ON a.tenant_id=r.tenant_id AND a.allocation_id=r.allocation_id").
		Where("r.tenant_id=? AND r.order_id=?", tenant, id).
		Order("r.position").Scan(&rows).Error; err != nil {
		return wallet.Order{}, err
	}
	if len(rows) != len(existing.Items) || len(rows) == 0 {
		return wallet.Order{}, wallet.ErrState
	}
	if actor != "" && !slices.ContainsFunc(rows, func(item staticRenewalItem) bool { return item.AllocationID == resolution.ItemID }) {
		return wallet.Order{}, wallet.ErrMissing
	}
	results, detailUnavailable := r.renewalOutcomes(ctx, rows, responses)
	if (resolution.CloseUnknown || resolution.ConfirmedFailure) && results[resolution.ItemID] != wallet.Succeeded {
		results[resolution.ItemID] = wallet.Failed
	}
	if len(results) == 0 {
		if detailUnavailable {
			return wallet.Order{}, wallet.ErrStaticUnavailable
		}
		return existing, nil
	}
	updated, err := r.saveStaticRenewalOutcomes(ctx, tenant, id, rows, results, actor, resolution)
	if err == nil && detailUnavailable && responses == nil && actor == "" {
		return wallet.Order{}, wallet.ErrStaticUnavailable
	}
	return updated, err
}

func (r *WalletRepository) renewalOutcomes(ctx context.Context, rows []staticRenewalItem, responses map[string]wallet.SupplierRenewal) (map[string]string, bool) {
	details := make(map[string]wallet.StaticDelivery)
	results := make(map[string]string)
	detailUnavailable := false
	for _, row := range rows {
		if row.Outcome != wallet.Pending {
			continue
		}
		response := responses[row.SupplierItemID]
		if response.Rejected {
			results[row.AllocationID] = wallet.Failed
			continue
		}
		if r.static == nil {
			detailUnavailable = true
			continue
		}
		if expiry, found := response.Expiries[row.SupplierItemID]; found && !expiry.Equal(row.ExpectedExpiry) {
			continue
		}
		detail, ok := details[row.SupplierOrderID]
		if !ok {
			var err error
			detail, err = r.static.Detail(ctx, row.SupplierOrderID)
			if err != nil {
				detailUnavailable = true
				detail = wallet.StaticDelivery{}
			}
			details[row.SupplierOrderID] = detail
		}
		for _, a := range detail.Allocations {
			if a.SupplierItemID == row.SupplierItemID && a.IP == row.OriginalIP && a.ExpiresAt.Equal(row.ExpectedExpiry) {
				results[row.AllocationID] = wallet.Succeeded
			}
		}
	}
	return results, detailUnavailable
}
