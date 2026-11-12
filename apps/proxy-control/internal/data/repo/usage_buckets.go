package repo

import (
	"fmt"

	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/biz/usage"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (t usageTx) WriteBuckets(minute, daily []usage.Bucket) error {
	dates := make(map[string]bool)
	for _, row := range minute {
		dates[row.UsageDate] = true
	}
	for date := range dates {
		if err := ensurePartition(t.db, date); err != nil {
			return err
		}
	}
	if len(minute) != 0 {
		if err := upsertBuckets(t.db, "usage_minutely", minute, true); err != nil {
			return err
		}
	}
	if len(daily) != 0 {
		return upsertBuckets(t.db, "usage_daily", daily, false)
	}
	return nil
}

func upsertBuckets(tx *gorm.DB, table string, rows []usage.Bucket, minute bool) error {
	keys := []clause.Column{{Name: "tenant_id"}, {Name: "gateway_id"}, {Name: "usage_date"}, {Name: "grant_id"}, {Name: "rate_id"}, {Name: "source"}}
	query := tx.Table(table)
	if minute {
		keys = append(keys, clause.Column{Name: "minute_start"})
	} else {
		query = query.Omit("MinuteStart")
	}
	updates := make(map[string]any)
	for _, col := range []string{"input_bytes", "output_bytes", "billable_bytes", "record_count"} {
		updates[col] = gorm.Expr(fmt.Sprintf("%s.%s + EXCLUDED.%s", table, col, col))
	}
	updates["last_received_at"] = gorm.Expr("GREATEST(" + table + ".last_received_at, EXCLUDED.last_received_at)")
	return query.Clauses(clause.OnConflict{Columns: keys, DoUpdates: clause.Assignments(updates)}).CreateInBatches(rows, 500).Error
}
