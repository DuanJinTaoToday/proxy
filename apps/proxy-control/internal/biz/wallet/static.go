package wallet

import (
	"context"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/biz/access"
	"github.com/go-kratos/kratos/v3/errors"
)

var (
	ErrStaticUnavailable  = errors.ServiceUnavailable("STATIC_SUPPLIER_UNAVAILABLE", "static IP supplier is unavailable")
	ErrStaticQuoteExpired = errors.Conflict("STATIC_QUOTE_EXPIRED", "confirm a current static IP quote")
	ErrStaticOfferMissing = errors.Conflict("STATIC_OFFER_UNAVAILABLE", "the selected static IP offer is unavailable")
)

type StaticOffer struct {
	NodeID               int64
	Region, IPType, Term string
	TermDays, Available  int32
	ShareMode            string
	PurposeID            int32
	UnitMinor            int64
}

type StaticPurchaseRequest struct {
	NodeID, UnitMinor            int64
	Term, Key, ConfirmationToken string
	Quantity                     int32
	ShareMode                    string               `json:",omitempty"`
	PurposeID                    int32                `json:",omitempty"`
	Lines                        []StaticPurchaseLine `json:",omitempty"`
}

type StaticPurchaseLine struct {
	NodeID, UnitMinor int64
	Quantity          int32
}

type StaticQuoteLine struct {
	Offer    StaticOffer
	Quantity int32
}

type StaticQuote struct {
	Lines             []StaticQuoteLine
	ConfirmationToken string
	ExpiresAt         time.Time
}

type StaticAllocation struct {
	NodeID                int64
	SupplierItemID        string
	OrderID               int64
	IP                    string
	Port                  int32
	Username, Password    string
	ExpiresAt             time.Time
	Region, IPType        string
	PendingRenewalOrderID int64
}

type StaticDelivery struct {
	SupplierOrderID string
	Allocations     []StaticAllocation
	Rejected        bool
}

type StaticSupplier interface {
	Renew(context.Context, []string, string, string) (SupplierRenewal, error)
	Offers(context.Context, string, int32) ([]StaticOffer, error)
	Place(context.Context, StaticPurchaseRequest) (StaticDelivery, error)
	Detail(context.Context, string) (StaticDelivery, error)
}

type PendingStaticPurchase struct {
	OrderID, DeliveredCount   int64
	TenantID, SupplierOrderID string
	PendingRenewalItems       string
	CreatedAt                 time.Time
	CanClose                  bool
}

type StaticSummary struct {
	Active, Expiring int32
}

func (s *Service) StaticSummary(ctx context.Context, token string) (StaticSummary, error) {
	tenant, err := s.scope(ctx, token, "", false)
	if err != nil {
		return StaticSummary{}, err
	}
	return s.store.StaticSummary(ctx, tenant)
}

type StaticRepository interface {
	StaticSummary(context.Context, string) (StaticSummary, error)
	ClaimStaticIPExpiryNotices(context.Context, string) (int32, error)
	QuoteStatic(context.Context, string, StaticPurchaseRequest) (StaticQuote, error)
	PendingStaticPurchases(context.Context, int64) ([]PendingStaticPurchase, error)
	StaticOffers(context.Context, string, int32) ([]StaticOffer, error)
	PurchaseStatic(context.Context, string, StaticPurchaseRequest) (Order, error)
	StaticAllocations(context.Context, string, []string, bool) ([]StaticAllocation, error)
	ResolveStatic(context.Context, string, int64, string, string, string, bool, bool) (Order, error)
}

func (offer StaticOffer) CheckAvailability(quantity int32) error {
	if offer.Available >= quantity {
		return nil
	}
	return offer.catalogConflict("STATIC_STOCK_INSUFFICIENT", "insufficient stock")
}

func (offer StaticOffer) CheckConfirmedPriceAndStock(unitMinor int64, available int32) error {
	if offer.UnitMinor != unitMinor {
		return offer.catalogConflict("STATIC_PRICE_CHANGED", "confirm the current price")
	}
	if available != 0 && offer.Available != available {
		return offer.catalogConflict("STATIC_STOCK_CHANGED", "confirm the current stock")
	}
	return nil
}

func (offer StaticOffer) catalogConflict(reason, message string) error {
	return errors.Conflict(reason, message).WithMetadata(map[string]string{
		"node_id":    strconv.FormatInt(offer.NodeID, 10),
		"unit_minor": strconv.FormatInt(offer.UnitMinor, 10),
		"available":  strconv.FormatInt(int64(offer.Available), 10),
	})
}

func (s *Service) StaticOffers(ctx context.Context, shareMode string, purposeID int32) ([]StaticOffer, error) {
	if !validStaticCatalog(shareMode, purposeID) {
		return nil, ErrInvalid
	}
	return s.store.StaticOffers(ctx, shareMode, purposeID)
}

func (s *Service) QuoteStatic(ctx context.Context, token string, request StaticPurchaseRequest) (StaticQuote, error) {
	tenant, err := s.scope(ctx, token, "", false)
	if err != nil {
		return StaticQuote{}, err
	}
	if request.ShareMode == "exclusive" {
		request.ShareMode = ""
	}
	if !request.validSpec(false) {
		return StaticQuote{}, ErrInvalid
	}
	return s.store.QuoteStatic(ctx, tenant, request)
}

func (p StaticPurchaseRequest) LineItems() []StaticPurchaseLine {
	if len(p.Lines) > 0 {
		return p.Lines
	}
	return []StaticPurchaseLine{{NodeID: p.NodeID, Quantity: p.Quantity, UnitMinor: p.UnitMinor}}
}

func (p StaticPurchaseRequest) validSpec(requirePrice bool) bool {
	if p.Term == "" || len(p.Term) > 16 || strings.ContainsAny(p.Term, "/\x00") ||
		!validStaticCatalog(p.ShareMode, p.PurposeID) || len(p.Lines) > 25 {
		return false
	}
	if len(p.Lines) > 0 && (p.NodeID != 0 || p.Quantity != 0 || p.UnitMinor != 0) {
		return false
	}
	seen := make(map[int64]bool, len(p.Lines))
	var quantity int64
	for _, line := range p.LineItems() {
		if line.NodeID <= 0 || line.Quantity <= 0 || seen[line.NodeID] || (requirePrice && line.UnitMinor <= 0) {
			return false
		}
		seen[line.NodeID] = true
		quantity += int64(line.Quantity)
	}
	return quantity <= math.MaxInt32
}

func (s *Service) PurchaseStatic(ctx context.Context, token string, request StaticPurchaseRequest) (Order, error) {
	tenant, err := s.scope(ctx, token, "", false)
	if err != nil {
		return Order{}, err
	}
	if request.ShareMode == "exclusive" {
		request.ShareMode = ""
	}
	if !request.validSpec(true) || request.Key == "" || len(request.Key) > 128 ||
		len(request.ConfirmationToken) > 8192 || strings.ContainsRune(request.Key, 0) {
		return Order{}, ErrInvalid
	}
	return s.store.PurchaseStatic(ctx, tenant, request)
}

func (s *Service) StaticAllocations(ctx context.Context, token string) ([]StaticAllocation, error) {
	tenant, err := s.scope(ctx, token, "", false)
	if err != nil {
		return nil, err
	}
	return s.store.StaticAllocations(ctx, tenant, nil, false)
}

func (s *Service) StaticCredentials(ctx context.Context, token string, ids []string, onlyActive bool) ([]StaticAllocation, error) {
	tenant, err := s.scope(ctx, token, "", false)
	if err != nil {
		return nil, err
	}
	if len(ids) == 0 || len(ids) > 1000 {
		return nil, ErrInvalid
	}
	seen := make(map[string]bool, len(ids))
	for _, id := range ids {
		if id == "" || len(id) > 128 || strings.ContainsRune(id, 0) || seen[id] {
			return nil, ErrInvalid
		}
		seen[id] = true
	}
	allocations, err := s.store.StaticAllocations(ctx, tenant, ids, onlyActive)
	if err != nil {
		return nil, err
	}
	if len(allocations) != len(ids) {
		return nil, ErrMissing
	}
	return allocations, nil
}

func (s *Service) ResolveStatic(ctx context.Context, token, tenant string, orderID int64, supplierOrderID, evidence string, closeRemaining, confirmedUndeliverable bool) (Order, error) {
	operator, err := s.access.AuthorizeOperator(ctx, token, access.ManageBilling)
	if err != nil {
		return Order{}, err
	}
	if confirmedUndeliverable && !closeRemaining || orderID <= 0 || tenant == "" || len(tenant) > 64 || len(supplierOrderID) > 128 ||
		len(evidence) > 500 || strings.TrimSpace(evidence) == "" || strings.ContainsRune(tenant+supplierOrderID+evidence, 0) {
		return Order{}, ErrInvalid
	}
	return s.store.ResolveStatic(ctx, tenant, orderID, operator, supplierOrderID, evidence, closeRemaining, confirmedUndeliverable)
}

func (s *Service) PendingStaticPurchases(ctx context.Context, token string, afterID int64) ([]PendingStaticPurchase, error) {
	if _, err := s.access.AuthorizeOperator(ctx, token, access.ReadOperations, access.ManageBilling); err != nil {
		return nil, err
	}
	if afterID < 0 {
		return nil, ErrInvalid
	}
	return s.store.PendingStaticPurchases(ctx, afterID)
}

func (s *Service) ClaimStaticIPExpiryNotices(ctx context.Context, token string) (int32, error) {
	tenant, err := s.scope(ctx, token, "", false)
	if err != nil {
		return 0, err
	}
	return s.store.ClaimStaticIPExpiryNotices(ctx, tenant)
}
