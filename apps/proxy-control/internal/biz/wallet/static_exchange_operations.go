package wallet

import (
	"context"
	"net/netip"
	"strings"
	"time"

	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/biz/access"
	"github.com/go-kratos/kratos/v3/errors"
)

const (
	StaticExchangeExecuted    = "executed"
	StaticExchangeNotExecuted = "not_executed"
)

var (
	ErrExchangeQuotaAdjustment = errors.Conflict("STATIC_EXCHANGE_QUOTA_ADJUSTMENT", "the adjustment would exceed the quota limits or take away reserved or used quota")
	ErrExchangeEvidence        = errors.Conflict("STATIC_EXCHANGE_EVIDENCE_REQUIRED", "the evidence does not support the requested exchange outcome")
	ErrExchangeAlreadyResolved = errors.Conflict("STATIC_EXCHANGE_ALREADY_RESOLVED", "the operation already has a different result")
)

type StaticExchangeHistoryRecord struct {
	ID, SupplierOrderID, SupplierItemID string
	OldIP, IP                           string
	OldNodeID, NodeID                   int64
	CreatedAt                           time.Time
}

type StaticExchangeHistorySupplier interface {
	ExchangeHistory(context.Context, string, time.Time) ([]StaticExchangeHistoryRecord, error)
}

type ExchangeOperationsQuota struct {
	Total          int32 `gorm:"column:quota"`
	Reserved, Used int32
}

func (q ExchangeOperationsQuota) Remaining() int32 {
	return int32(max(0, int64(q.Total)-int64(q.Used)-int64(q.Reserved)))
}

type ExchangeOperationsRecord struct {
	StaticExchange
	TenantID, SupplierOrderID, SupplierItemID, OldIP, NewIP, Resolution string
	UnknownAt                                                           *time.Time
	InQueue, ResolutionConflict, Corrected                              bool
}

type ExchangeOperationsAudit struct {
	ID                                                                                         int64
	Kind, IdempotencyKey, RequestKey, ActorID, Evidence, Outcome, SupplierRecordID, ObservedIP string
	QuotaBefore, ReservedBefore, UsedBefore, QuotaAfter, ReservedAfter, UsedAfter              int32
	CreatedAt                                                                                  time.Time
}

type ExchangeOperationsPage struct {
	Records []ExchangeOperationsRecord
}

type ExchangeOperationsFilter struct {
	TenantID, BeforeTenant, BeforeKey string
	BeforeTime                        time.Time
}

type ExchangeQuotaAdjustment struct {
	TenantID, RequestKey, Evidence string
	Delta                          int32
}

type ExchangeOperationsResolution struct {
	TenantID, IdempotencyKey, RequestKey, Outcome, Evidence, ExpectedIP string
	Correction                                                          bool
}

type ExchangeOperationsRepository interface {
	OperatorExchangeQuota(context.Context, string) (ExchangeOperationsQuota, error)
	OperatorExchangeHistory(context.Context, ExchangeOperationsFilter, bool) (ExchangeOperationsPage, error)
	OperatorExchangeAudit(context.Context, string, int64) ([]ExchangeOperationsAudit, error)
	AdjustExchangeQuota(context.Context, string, ExchangeQuotaAdjustment) (ExchangeOperationsAudit, error)
	ResolveExchange(context.Context, string, ExchangeOperationsResolution) (ExchangeOperationsRecord, error)
}

func (s *Service) OperatorExchangeQuota(ctx context.Context, token, tenant string) (ExchangeOperationsQuota, error) {
	id, err := s.scope(ctx, token, tenant, true)
	if err != nil {
		return ExchangeOperationsQuota{}, err
	}
	return s.store.OperatorExchangeQuota(ctx, id)
}

func (s *Service) OperatorExchangeHistory(ctx context.Context, token string, filter ExchangeOperationsFilter, queue bool) (ExchangeOperationsPage, error) {
	if queue && filter.TenantID == "" {
		if _, err := s.access.AuthorizeOperator(ctx, token, access.ReadOperations); err != nil {
			return ExchangeOperationsPage{}, err
		}
	} else if _, err := s.scope(ctx, token, filter.TenantID, true); err != nil {
		return ExchangeOperationsPage{}, err
	}
	if (!filter.BeforeTime.IsZero() || filter.BeforeKey != "") &&
		(filter.BeforeTime.IsZero() || !staticExchangeKeyPattern.MatchString(filter.BeforeKey) ||
			len(filter.BeforeTenant) == 0 || len(filter.BeforeTenant) > 64 || strings.ContainsRune(filter.BeforeTenant, 0)) {
		return ExchangeOperationsPage{}, ErrInvalid
	}
	return s.store.OperatorExchangeHistory(ctx, filter, queue)
}

func (s *Service) OperatorExchangeAudit(ctx context.Context, token, tenant string, before int64) ([]ExchangeOperationsAudit, error) {
	id, err := s.scope(ctx, token, tenant, true)
	if err != nil {
		return nil, err
	}
	if before < 0 {
		return nil, ErrInvalid
	}
	return s.store.OperatorExchangeAudit(ctx, id, before)
}

func (s *Service) AdjustExchangeQuota(ctx context.Context, token string, adjustment ExchangeQuotaAdjustment) (ExchangeOperationsAudit, error) {
	actor, err := s.access.AuthorizeOperator(ctx, token, access.ManageBilling)
	if err != nil {
		return ExchangeOperationsAudit{}, err
	}
	adjustment.Evidence = strings.TrimSpace(adjustment.Evidence)
	if adjustment.Delta == 0 || !validExchangeOperatorCommand(adjustment.TenantID, adjustment.RequestKey, adjustment.Evidence) {
		return ExchangeOperationsAudit{}, ErrInvalid
	}
	return s.store.AdjustExchangeQuota(ctx, actor, adjustment)
}

func (s *Service) ResolveExchange(ctx context.Context, token string, resolution ExchangeOperationsResolution) (ExchangeOperationsRecord, error) {
	actor, err := s.access.AuthorizeOperator(ctx, token, access.ManageBilling)
	if err != nil {
		return ExchangeOperationsRecord{}, err
	}
	resolution.Evidence = strings.TrimSpace(resolution.Evidence)
	if !staticExchangeKeyPattern.MatchString(resolution.IdempotencyKey) ||
		!validExchangeOperatorCommand(resolution.TenantID, resolution.RequestKey, resolution.Evidence) ||
		(resolution.Outcome != StaticExchangeExecuted && resolution.Outcome != StaticExchangeNotExecuted) ||
		(resolution.Outcome == StaticExchangeNotExecuted && resolution.ExpectedIP != "") {
		return ExchangeOperationsRecord{}, ErrInvalid
	}
	if resolution.Outcome == StaticExchangeExecuted {
		if ip, err := netip.ParseAddr(resolution.ExpectedIP); err != nil || ip.Zone() != "" {
			return ExchangeOperationsRecord{}, ErrInvalid
		}
	}
	if strings.ContainsAny(resolution.ExpectedIP, "\r\n\x00") {
		return ExchangeOperationsRecord{}, ErrInvalid
	}
	return s.store.ResolveExchange(ctx, actor, resolution)
}

func validExchangeOperatorCommand(tenant, key, evidence string) bool {
	return tenant != "" && len(tenant) <= 64 && staticExchangeKeyPattern.MatchString(key) &&
		evidence != "" && len(evidence) <= 500 && !strings.ContainsAny(tenant+evidence, "\r\n\x00")
}
