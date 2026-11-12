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
	return s.dailyUsage(ctx, id, startDate, endDate)
}

func (s *Service) QueryUsage(ctx context.Context, token, id string, start, end *time.Time, dailyTotal bool) (*DailyUsage, error) {
	if err := s.access.Require(token, "", access.ReadOperations); err != nil {
		return nil, err
	}
	if id == "" {
		return nil, errors.BadRequest("TENANT_REQUIRED", "tenant_id is required")
	}
	if !dailyTotal {
		return nil, errors.BadRequest("UNSUPPORTED_USAGE_QUERY", "only unfiltered daily totals are currently supported")
	}
	if start == nil || end == nil {
		return nil, errors.BadRequest("INVALID_TIME_RANGE", "start_time and end_time are required")
	}
	from, to := start.In(shanghai), end.In(shanghai)
	if from.Year() < 1 || to.Year() > 9999 ||
		!from.Equal(shanghaiDayStart(from)) || !to.Equal(shanghaiDayStart(to)) ||
		!to.After(from) || to.After(from.AddDate(0, 0, 366)) {
		return nil, errors.BadRequest("INVALID_TIME_RANGE", "use Shanghai midnight boundaries spanning 1 to 366 days")
	}
	return s.dailyUsage(ctx, id, from.Format(time.DateOnly), to.AddDate(0, 0, -1).Format(time.DateOnly))
}

func shanghaiDayStart(at time.Time) time.Time {
	return time.Date(at.Year(), at.Month(), at.Day(), 0, 0, 0, 0, shanghai)
}

func (s *Service) dailyUsage(ctx context.Context, id, startDate, endDate string) (*DailyUsage, error) {
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
		at, err := time.ParseInLocation(time.DateOnly, row.Date, shanghai)
		if err != nil {
			return nil, err
		}
		response.Days = append(response.Days, Day{DailyTotals: row, TotalBytes: total, StartTime: at})
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
