package data

import (
	"context"
	"database/sql"
	"time"

	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/biz/usage"
	"gorm.io/gorm"
)

func (r *UsageRepository) DailyUsage(ctx context.Context, id, start, end string) ([]usage.DailyTotals, *time.Time, error) {
	var rows []usage.DailyTotals
	var balance struct{ LastReceivedAt *time.Time }
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
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
			sql.Named("tenant", id), sql.Named("start", start), sql.Named("end", end)).Scan(&rows).Error; err != nil {
			return err
		}
		return tx.Table("tenant_balances").Select("last_received_at").Where("tenant_id = ?", id).Find(&balance).Error
	}, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	return rows, balance.LastReceivedAt, err
}
