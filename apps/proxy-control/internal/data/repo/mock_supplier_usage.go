package repo

import (
	"context"

	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/biz/usage"
)

func (r *UsageRepository) MockEnabled(ctx context.Context, tenantID string) (bool, error) {
	var enabled bool
	err := r.db.WithContext(ctx).Raw(
		"SELECT EXISTS (SELECT 1 FROM mock_supplier_usage WHERE tenant_id = ?)", tenantID,
	).Scan(&enabled).Error
	return enabled, err
}

func (r *UsageRepository) MockDailyUsage(ctx context.Context, tenantID, start, end string) ([]usage.Day, error) {
	var rows []struct {
		Date       string
		TotalBytes uint64
	}
	err := r.db.WithContext(ctx).Raw(`
		SELECT to_char(usage_date, 'YYYY-MM-DD') AS date, SUM(total_bytes) AS total_bytes
		FROM mock_supplier_usage WHERE tenant_id = ? AND usage_date BETWEEN ? AND ?
		GROUP BY usage_date ORDER BY usage_date`, tenantID, start, end).Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	days := make([]usage.Day, 0, len(rows))
	for _, row := range rows {
		days = append(days, usage.Day{DailyTotals: usage.DailyTotals{Date: row.Date}, TotalBytes: row.TotalBytes})
	}
	return days, nil
}
