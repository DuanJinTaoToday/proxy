package usage

import (
	"context"
	"time"

	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/biz/access"
	"github.com/go-kratos/kratos/v3/errors"
)

func (s *Service) GetTenantDailyUsage(ctx context.Context, token, id, startDate, endDate string) (*DailyUsage, error) {
	if err := s.access.Require(token, id, access.ReadTenantUsage); err != nil {
		return nil, err
	}
	start, startErr := time.ParseInLocation(time.DateOnly, startDate, shanghai)
	end, endErr := time.ParseInLocation(time.DateOnly, endDate, shanghai)
	if startErr != nil || endErr != nil || start.Year() < 1 || end.Before(start) || !end.Before(start.AddDate(0, 0, 93)) {
		return nil, errors.BadRequest("INVALID_TIME_RANGE", "date range must contain 1 to 93 Shanghai dates")
	}
	rows, lastReceivedAt, err := s.repo.DailyUsage(ctx, id, startDate, endDate)
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
	available int64
	expires   time.Time
}

// AvailableBytes is the cached quota view for gateway admission.
func (s *Service) AvailableBytes(ctx context.Context, id string) (int64, error) {
	s.cacheMu.Lock()
	defer s.cacheMu.Unlock()
	if entry, ok := s.cache[id]; ok && time.Now().Before(entry.expires) {
		return entry.available, nil
	}
	balance, err := s.loadBalance(ctx, id)
	if err != nil {
		return 0, err
	}
	return balance.AvailableBytes, nil
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
			if g.StartsAt.After(now) && g.StartsAt.Before(expires) {
				expires = g.StartsAt
			}
			if !g.activeAt(now) {
				continue
			}
			if g.ExpiresAt != nil && g.ExpiresAt.Before(expires) {
				expires = *g.ExpiresAt
			}
			reply.Grants = append(reply.Grants, g)
		}
		return nil
	})
	if err != nil {
		delete(s.cache, id)
		return nil, err
	}
	s.cache[id] = cachedBalance{available: reply.AvailableBytes, expires: expires}
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
