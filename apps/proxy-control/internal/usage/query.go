package usage

import (
	"context"
	"database/sql"
	"strconv"
	"time"

	v1 "github.com/DuanJinTaoToday/proxy/api/proxycontrol/v1"
	"github.com/go-kratos/kratos/v3/errors"
	"google.golang.org/protobuf/types/known/timestamppb"
	"gorm.io/gorm"
)

func (s *Service) GetTenantDailyUsage(ctx context.Context, req *v1.DailyUsageRequest) (*v1.DailyUsageResponse, error) {
	id := req.GetTenantId()
	if err := s.access.Require(ctx, id, v1.Permission_PERMISSION_READ_TENANT_USAGE); err != nil {
		return nil, err
	}
	start, startErr := time.ParseInLocation(time.DateOnly, req.GetStartDate(), shanghai)
	end, endErr := time.ParseInLocation(time.DateOnly, req.GetEndDate(), shanghai)
	if startErr != nil || endErr != nil || start.Year() < 1 || end.Before(start) || !end.Before(start.AddDate(0, 0, 93)) {
		return nil, errors.BadRequest("INVALID_TIME_RANGE", "date range must contain 1 to 93 Shanghai dates")
	}
	response := &v1.DailyUsageResponse{}
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var rows []struct {
			Date                                   string
			InputBytes, OutputBytes, BillableBytes uint64
		}
		if err := tx.Raw(`SELECT to_char(usage_date, 'YYYY-MM-DD') AS date,
			SUM(input_bytes) AS input_bytes, SUM(output_bytes) AS output_bytes, SUM(billable_bytes) AS billable_bytes
			FROM (
				SELECT usage_date, input_bytes, output_bytes, billable_bytes FROM usage_minutely
				WHERE tenant_id = @tenant AND source = 'tunnel' AND usage_date BETWEEN @start AND @end
				UNION ALL
				SELECT usage_date, input_bytes, output_bytes, billable_bytes FROM usage_daily
				WHERE tenant_id = @tenant AND source = 'tunnel' AND usage_date BETWEEN @start AND @end
			) usage GROUP BY usage_date
			HAVING SUM(input_bytes) > 0 OR SUM(output_bytes) > 0 OR SUM(billable_bytes) > 0
			ORDER BY usage_date`,
			sql.Named("tenant", id), sql.Named("start", req.GetStartDate()), sql.Named("end", req.GetEndDate())).Scan(&rows).Error; err != nil {
			return err
		}
		for _, row := range rows {
			total := row.InputBytes + row.OutputBytes
			if total < row.InputBytes {
				return errByteOverflow
			}
			response.Days = append(response.Days, &v1.DailyUsage{Date: row.Date, Amount: &v1.UsageAmount{
				InputBytes: row.InputBytes, OutputBytes: row.OutputBytes, TotalBytes: total, BillableBytes: row.BillableBytes,
			}})
		}
		var balance struct{ LastReceivedAt *time.Time }
		if err := tx.Table("tenant_balances").Select("last_received_at").Where("tenant_id = ?", id).Find(&balance).Error; err != nil {
			return err
		}
		response.LastReceivedAt = timestamp(balance.LastReceivedAt)
		return nil
	}, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return nil, err
	}
	return response, nil
}

func timestamp(at *time.Time) *timestamppb.Timestamp {
	if at == nil {
		return nil
	}
	return timestamppb.New(*at)
}

func (s *Service) GetTenantBalance(ctx context.Context, req *v1.TenantBalanceRequest) (*v1.TenantBalance, error) {
	id := req.GetTenantId()
	if err := s.access.Require(ctx, id, v1.Permission_PERMISSION_READ_TENANT_USAGE); err != nil {
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
	return balance.GetAvailableBytes(), err
}

func (s *Service) readBalance(ctx context.Context, id string) (*v1.TenantBalance, error) {
	s.cacheMu.Lock()
	defer s.cacheMu.Unlock()
	return s.loadBalance(ctx, id)
}

// Holding cacheMu through the read prevents a slow refresh from replacing a newer value.
func (s *Service) loadBalance(ctx context.Context, id string) (*v1.TenantBalance, error) {
	var reply *v1.TenantBalance
	var expires time.Time
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
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
		reply = &v1.TenantBalance{TenantId: id, AvailableBytes: available, OverdraftBytes: uint64(a.overdraft),
			AdmissionThresholdBytes: uint64(a.admission), WarningThresholdBytes: uint64(a.warning), WarningSince: timestamp(a.warningSince)}
		for _, g := range a.grants {
			if g.StartsAt.After(now) && g.StartsAt.Before(expires) {
				expires = g.StartsAt
			}
			if !g.activeAt(now) {
				continue
			}
			if g.ExpiresAt != nil && g.ExpiresAt.Before(expires) {
				expires = *g.ExpiresAt
			}
			reply.Grants = append(reply.Grants, &v1.TrafficGrant{GrantId: strconv.FormatInt(g.ID, 10),
				GrantedBytes: uint64(g.GrantedBytes), UsedBytes: uint64(g.UsedBytes),
				StartsAt: timestamppb.New(g.StartsAt), ExpiresAt: timestamp(g.ExpiresAt), Source: g.Source})
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
