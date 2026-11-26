package wallet

import (
	"context"
	"regexp"
	"strings"
	"time"

	"github.com/go-kratos/kratos/v3/errors"
)

const (
	StaticExchangePending    = "pending"
	StaticExchangeUnknown    = "unknown"
	StaticExchangeFailed     = "failed"
	StaticExchangeActivating = "activating"
	StaticExchangeReady      = "ready"
	StaticExchangeSupport    = "support_required"
)

var (
	ErrStaticExchangeIneligible = errors.Conflict("STATIC_EXCHANGE_INELIGIBLE", "only active, delivered exclusive static IPs can be exchanged")
	ErrStaticExchangeRegion     = errors.Conflict("STATIC_EXCHANGE_REGION_MISMATCH", "the exchange must use the allocation's original node")
	ErrStaticExchangePending    = errors.Conflict("STATIC_EXCHANGE_PENDING", "a static IP exchange is still in progress")
	ErrStaticExchangeQuota      = errors.Conflict("STATIC_EXCHANGE_QUOTA_EXHAUSTED", "no static IP exchange quota remains")
)

type StaticExchangeRequest struct {
	AllocationID, IdempotencyKey string
	NodeID                       int64
}

type StaticExchange struct {
	StaticExchangeRequest
	Status               string
	CreatedAt, UpdatedAt time.Time
}

type StaticExchangeQuota struct {
	Remaining, Reserved, Used int32
}

type SupplierExchange struct {
	Allocation StaticAllocation
	Rejected   bool
}

type StaticExchangeSupplier interface {
	Exchange(context.Context, string, int64) (SupplierExchange, error)
}

type StaticExchangeRepository interface {
	StaticExchangeQuota(context.Context, string) (StaticExchangeQuota, error)
	StaticExchange(context.Context, string, string) (StaticExchange, error)
	ExchangeStaticIP(context.Context, string, StaticExchangeRequest) (StaticExchange, error)
	ProbeStaticExchanges(context.Context) error
	ReconcileStaticExchanges(context.Context) error
}

func (s *Service) StaticExchangeQuota(ctx context.Context, token string) (StaticExchangeQuota, error) {
	tenant, err := s.scope(ctx, token, "", false)
	if err != nil {
		return StaticExchangeQuota{}, err
	}
	return s.store.StaticExchangeQuota(ctx, tenant)
}

func (s *Service) StaticExchange(ctx context.Context, token, key string) (StaticExchange, error) {
	tenant, err := s.scope(ctx, token, "", false)
	if err != nil {
		return StaticExchange{}, err
	}
	if !staticExchangeKeyPattern.MatchString(key) {
		return StaticExchange{}, ErrInvalid
	}
	return s.store.StaticExchange(ctx, tenant, key)
}

func (s *Service) ExchangeStaticIP(ctx context.Context, token string, req StaticExchangeRequest) (StaticExchange, error) {
	tenant, err := s.scope(ctx, token, "", false)
	if err != nil {
		return StaticExchange{}, err
	}
	if !staticExchangeKeyPattern.MatchString(req.IdempotencyKey) || req.NodeID <= 0 || req.AllocationID == "" ||
		len(req.AllocationID) > 128 || strings.ContainsRune(req.AllocationID, 0) {
		return StaticExchange{}, ErrInvalid
	}
	return s.store.ExchangeStaticIP(ctx, tenant, req)
}

func (s *Service) ProbeStaticExchanges(ctx context.Context) error {
	return s.store.ProbeStaticExchanges(ctx)
}

func (s *Service) ReconcileStaticExchanges(ctx context.Context) error {
	return s.store.ReconcileStaticExchanges(ctx)
}

var staticExchangeKeyPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,128}$`)
