package wallet

import (
	"context"
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
	ShareMode                    string `json:",omitempty"`
	PurposeID                    int32  `json:",omitempty"`
}

type StaticQuote struct {
	Offer             StaticOffer
	Quantity          int32
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
	Renew(context.Context, []string, string) (SupplierRenewal, error)
	Offers(context.Context) ([]StaticOffer, error)
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

func (p StaticPurchaseRequest) CheckOffer(offer StaticOffer) error {
	if offer.Available < p.Quantity {
		return errors.Conflict("STATIC_STOCK_INSUFFICIENT", "insufficient stock").WithMetadata(map[string]string{
			"available": strconv.FormatInt(int64(offer.Available), 10),
		})
	}
	if offer.UnitMinor != p.UnitMinor {
		return errors.Conflict("STATIC_PRICE_CHANGED", "confirm the current price").WithMetadata(map[string]string{
			"unit_minor": strconv.FormatInt(offer.UnitMinor, 10),
		})
	}
	return nil
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
	if !request.validSpec() {
		return StaticQuote{}, ErrInvalid
	}
	return s.store.QuoteStatic(ctx, tenant, request)
}

func (p StaticPurchaseRequest) validSpec() bool {
	return p.NodeID > 0 && p.Quantity > 0 && p.Term != "" && len(p.Term) <= 16 && !strings.ContainsRune(p.Term, 0) &&
		validStaticCatalog(p.ShareMode, p.PurposeID)
}

func (s *Service) PurchaseStatic(ctx context.Context, token string, request StaticPurchaseRequest) (Order, error) {
	tenant, err := s.scope(ctx, token, "", false)
	if err != nil {
		return Order{}, err
	}
	if request.ShareMode == "exclusive" {
		request.ShareMode = ""
	}
	if !request.validSpec() || request.UnitMinor <= 0 || request.Key == "" || len(request.Key) > 128 ||
		len(request.ConfirmationToken) > 1024 || strings.ContainsRune(request.Key, 0) {
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
