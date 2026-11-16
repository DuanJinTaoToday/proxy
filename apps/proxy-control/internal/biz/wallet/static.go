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
	ErrStaticOfferMissing = errors.Conflict("STATIC_OFFER_UNAVAILABLE", "the selected static IP offer is unavailable")
)

type StaticOffer struct {
	NodeID               int64
	Region, IPType, Term string
	TermDays, Available  int32
	UnitMinor            int64
}

type StaticPurchaseRequest struct {
	NodeID, UnitMinor int64
	Term, Key         string
	Quantity          int32
}

type StaticAllocation struct {
	SupplierItemID     string
	OrderID            int64
	IP                 string
	Port               int32
	Username, Password string
	ExpiresAt          time.Time
}

type StaticDelivery struct {
	SupplierOrderID string
	Allocations     []StaticAllocation
	Rejected        bool
}

type StaticSupplier interface {
	Offers(context.Context) ([]StaticOffer, error)
	Place(context.Context, StaticPurchaseRequest) (StaticDelivery, error)
	Detail(context.Context, string) (StaticDelivery, error)
}

type PendingStaticPurchase struct {
	OrderID, DeliveredCount   int64
	TenantID, SupplierOrderID string
	CreatedAt                 time.Time
	CanClose                  bool
}

type StaticRepository interface {
	PendingStaticPurchases(context.Context, int64) ([]PendingStaticPurchase, error)
	StaticOffers(context.Context) ([]StaticOffer, error)
	PurchaseStatic(context.Context, string, StaticPurchaseRequest) (Order, error)
	StaticAllocations(context.Context, string) ([]StaticAllocation, error)
	ResolveStatic(context.Context, string, int64, string, string, string, bool) (Order, error)
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

func (s *Service) StaticOffers(ctx context.Context) ([]StaticOffer, error) {
	return s.store.StaticOffers(ctx)
}

func (s *Service) PurchaseStatic(ctx context.Context, token string, request StaticPurchaseRequest) (Order, error) {
	tenant, err := s.scope(ctx, token, "", false)
	if err != nil {
		return Order{}, err
	}
	if request.NodeID <= 0 || request.Quantity <= 0 || request.UnitMinor <= 0 || request.Term == "" ||
		len(request.Term) > 16 || request.Key == "" || len(request.Key) > 128 || strings.ContainsRune(request.Term+request.Key, 0) {
		return Order{}, ErrInvalid
	}
	return s.store.PurchaseStatic(ctx, tenant, request)
}

func (s *Service) StaticAllocations(ctx context.Context, token string) ([]StaticAllocation, error) {
	tenant, err := s.scope(ctx, token, "", false)
	if err != nil {
		return nil, err
	}
	return s.store.StaticAllocations(ctx, tenant)
}

func (s *Service) ResolveStatic(ctx context.Context, token, tenant string, orderID int64, supplierOrderID, evidence string, closeRemaining bool) (Order, error) {
	operator, err := s.access.AuthorizeOperator(ctx, token, access.ManageBilling)
	if err != nil {
		return Order{}, err
	}
	if orderID <= 0 || tenant == "" || len(tenant) > 64 || len(supplierOrderID) > 128 ||
		len(evidence) > 500 || strings.TrimSpace(evidence) == "" || strings.ContainsRune(tenant+supplierOrderID+evidence, 0) {
		return Order{}, ErrInvalid
	}
	return s.store.ResolveStatic(ctx, tenant, orderID, operator, supplierOrderID, evidence, closeRemaining)
}

func (s *Service) PendingStaticPurchases(ctx context.Context, token string, afterID int64) ([]PendingStaticPurchase, error) {
	if _, err := s.access.AuthorizeOperator(ctx, token, access.ManageBilling); err != nil {
		return nil, err
	}
	if afterID < 0 {
		return nil, ErrInvalid
	}
	return s.store.PendingStaticPurchases(ctx, afterID)
}
