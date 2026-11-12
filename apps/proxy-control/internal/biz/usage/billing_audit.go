package usage

import (
	"context"
	"strconv"
	"time"

	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/biz/access"
	"github.com/go-kratos/kratos/v3/errors"
)

type AuditedGrant struct {
	ID, GrantedBytes, UsedBytes, OverdraftOffsetBytes int64
	StartsAt                                          time.Time
	ExpiresAt                                         *time.Time
	Status                                            string
}

type LedgerEntry struct {
	ID, GrantID, DeltaBytes, AvailableBefore, AvailableAfter, CorrectionOf int64
	Kind, ExternalRef, Reason, Operator                                    string
	CreatedAt                                                              time.Time
}

type RateVersion struct {
	Rate
	ScopeType, ScopeID string
	EffectiveFrom      time.Time
}

type AuditPage[T any] struct {
	Items     []T
	NextToken string
}

type GrantAuditPage struct {
	AuditPage[AuditedGrant]
	OverdraftBytes int64
}

func (s *Service) ListTrafficGrants(ctx context.Context, token, tenant string, size int, pageToken string) (GrantAuditPage, error) {
	var result GrantAuditPage
	if err := s.requireAuditTenant(token, tenant); err != nil {
		return result, err
	}
	page, err := readAuditPage([2]string{"grants", tenant}, size, pageToken, func(after int64, limit int) ([]AuditedGrant, error) {
		grants, overdraft, err := s.repo.AuditGrants(ctx, tenant, after, limit)
		result.OverdraftBytes = overdraft
		return grants, err
	}, func(grant AuditedGrant) int64 { return grant.ID })
	result.AuditPage = page
	return result, err
}

func (s *Service) ListBalanceLedger(ctx context.Context, token, tenant string, size int, pageToken string) (AuditPage[LedgerEntry], error) {
	if err := s.requireAuditTenant(token, tenant); err != nil {
		return AuditPage[LedgerEntry]{}, err
	}
	return readAuditPage([2]string{"ledger", tenant}, size, pageToken, func(after int64, limit int) ([]LedgerEntry, error) {
		return s.repo.AuditLedger(ctx, tenant, after, limit)
	}, func(entry LedgerEntry) int64 { return entry.ID })
}

func (s *Service) ListBillingRates(ctx context.Context, token, scopeType, scopeID, rateID string, size int, pageToken string) (AuditPage[RateVersion], error) {
	if err := s.access.Require(token, "", access.ReadOperations); err != nil {
		return AuditPage[RateVersion]{}, err
	}
	if scopeType == "global" && scopeID == "_" {
		scopeID = ""
	}
	var id int64
	if rateID != "" {
		var err error
		id, err = strconv.ParseInt(rateID, 10, 64)
		if err != nil || id <= 0 || scopeType != "" || scopeID != "" {
			return AuditPage[RateVersion]{}, errors.BadRequest("INVALID_RATE_ID", "provide a positive rate_id without a scope")
		}
	}
	globalScope := scopeType == "global" && scopeID == ""
	namedScope := (scopeType == "group" || scopeType == "tenant") && scopeID != ""
	if id == 0 && !globalScope && !namedScope {
		return AuditPage[RateVersion]{}, errors.BadRequest("INVALID_SCOPE", "provide a global, group or tenant rate scope")
	}
	return readAuditPage([4]string{"rates", scopeType, scopeID, rateID}, size, pageToken, func(after int64, limit int) ([]RateVersion, error) {
		return s.repo.AuditRates(ctx, scopeType, scopeID, id, after, limit)
	}, func(rate RateVersion) int64 { return rate.ID })
}

func (s *Service) requireAuditTenant(token, tenant string) error {
	if err := s.access.Require(token, "", access.ReadOperations); err != nil {
		return err
	}
	if tenant == "" {
		return errors.BadRequest("TENANT_REQUIRED", "tenant_id is required")
	}
	return nil
}

func readAuditPage[T any](scope any, size int, token string, load func(int64, int) ([]T, error), key func(T) int64) (AuditPage[T], error) {
	var page AuditPage[T]
	cursor, after, size, err := readPage[int64](scope, size, token)
	if err != nil {
		return page, err
	}
	var id int64
	if after != nil {
		if *after <= 0 {
			return page, errInvalidPageToken
		}
		id = *after
	}
	rows, err := load(id, size+1)
	if err != nil {
		return page, err
	}
	page.Items, page.NextToken, err = finishPage(rows, size, cursor, key)
	return page, err
}
