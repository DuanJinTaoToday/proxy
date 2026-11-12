package usage

import (
	"context"
	"time"

	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/biz/access"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/biz/proxyauth"
	"github.com/go-kratos/kratos/v3/errors"
)

func (s *Service) GetTenantDailyUsage(ctx context.Context, token, id, startDate, endDate, source string) (*DailyUsage, error) {
	if err := s.access.Require(token, id, access.ReadTenantUsage); err != nil {
		return nil, err
	}
	start, startErr := time.ParseInLocation(time.DateOnly, startDate, shanghai)
	end, endErr := time.ParseInLocation(time.DateOnly, endDate, shanghai)
	if startErr != nil || endErr != nil || start.Year() < 1 || end.Before(start) || !end.Before(start.AddDate(0, 0, 93)) {
		return nil, errors.BadRequest("INVALID_TIME_RANGE", "date range must contain 1 to 93 Shanghai dates")
	}
	switch source {
	case "", "tunnel":
	case MockExtractionSource:
		if err := s.requireMockTenant(ctx, id); err != nil {
			return nil, err
		}
		days, err := s.repo.MockDailyUsage(ctx, id, startDate, endDate)
		if err != nil {
			return nil, err
		}
		return &DailyUsage{Days: days}, nil
	default:
		return nil, errors.BadRequest("INVALID_USAGE_SOURCE", "unsupported usage source")
	}
	rows, lastReceivedAt, err := s.repo.DailyUsage(ctx, id, start, end.AddDate(0, 0, 1))
	if err != nil {
		return nil, err
	}
	response := &DailyUsage{LastReceivedAt: lastReceivedAt}
	for _, row := range rows {
		total := row.InputBytes + row.OutputBytes
		if total < row.InputBytes {
			return nil, errByteOverflow
		}
		response.Days = append(response.Days, Day{DailyTotals: row, TotalBytes: total})
	}
	return response, nil
}

func (s *Service) GetTenantBalance(ctx context.Context, token, id string) (*Balance, error) {
	if err := s.access.Require(token, id, access.ReadTenantUsage); err != nil {
		return nil, err
	}
	return s.readBalance(ctx, id)
}

type cachedBalance struct {
	quota   proxyauth.Quota
	expires time.Time
}

func (s *Service) AvailableBytes(ctx context.Context, id string) (int64, error) {
	quota, err := s.AdmissionQuota(ctx, id)
	if err != nil {
		return 0, err
	}
	return quota.AvailableBytes, nil
}

func (s *Service) AdmissionQuota(ctx context.Context, id string) (*proxyauth.Quota, error) {
	s.cacheMu.Lock()
	defer s.cacheMu.Unlock()
	entry, ok := s.cache[id]
	if !ok || !time.Now().Before(entry.expires) {
		if _, err := s.loadBalance(ctx, id); err != nil {
			return nil, err
		}
		entry = s.cache[id]
	}
	return &entry.quota, nil
}

func (s *Service) readBalance(ctx context.Context, id string) (*Balance, error) {
	s.cacheMu.Lock()
	defer s.cacheMu.Unlock()
	return s.loadBalance(ctx, id)
}

// Holding cacheMu through the read prevents a slow refresh from replacing a newer value.
func (s *Service) loadBalance(ctx context.Context, id string) (*Balance, error) {
	var reply *Balance
	var expires time.Time
	var boundary *time.Time
	err := s.repo.Transact(ctx, func(tx Transaction) error {
		now := time.Now().UTC()
		a, err := loadAccount(tx, id, now)
		if err != nil {
			return err
		}
		available, err := a.available(now)
		if err != nil {
			return err
		}
		if err := a.save(tx, now); err != nil {
			return err
		}
		expires = now.Add(5 * time.Second)
		reply = &Balance{TenantID: id, AvailableBytes: available, OverdraftBytes: uint64(a.Overdraft),
			AdmissionThresholdBytes: uint64(a.Admission), WarningThresholdBytes: uint64(a.Warning), WarningSince: a.WarningSince}
		for _, g := range a.Grants {
			if g.StartsAt.After(now) && (boundary == nil || g.StartsAt.Before(*boundary)) {
				boundary = &g.StartsAt
			}
			if !g.activeAt(now) {
				continue
			}
			if g.ExpiresAt != nil && (boundary == nil || g.ExpiresAt.Before(*boundary)) {
				boundary = g.ExpiresAt
			}
			reply.Grants = append(reply.Grants, g)
		}
		return nil
	})
	if err != nil {
		delete(s.cache, id)
		return nil, err
	}
	if boundary != nil && boundary.Before(expires) {
		expires = *boundary
	}
	s.cache[id] = cachedBalance{quota: proxyauth.Quota{AvailableBytes: reply.AvailableBytes,
		AdmissionThresholdBytes: reply.AdmissionThresholdBytes, ValidUntil: boundary}, expires: expires}
	return reply, nil
}

func (s *Service) refreshBalances(ctx context.Context, ids []string) {
	s.cacheMu.Lock()
	defer s.cacheMu.Unlock()
	for _, id := range ids {
		delete(s.cache, id)
	}
	// A committed batch remains acknowledged even when a cache refill fails.
	for _, id := range ids {
		if ctx.Err() != nil {
			break
		}
		_, _ = s.loadBalance(ctx, id)
	}
}
