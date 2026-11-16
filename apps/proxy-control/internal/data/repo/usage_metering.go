package repo

import (
	"fmt"
	"time"

	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/biz/usage"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (t usageTx) ClientTenant(clientID string) (string, error) {
	var tenant string
	err := t.db.Raw("SELECT tenant_id FROM proxy_subaccounts WHERE client_id=?", clientID).Scan(&tenant).Error
	return tenant, err
}

func (t usageTx) DebitTenant(tenant string, bytes int64, now time.Time) error {
	// The balance row is the common serialization point for all grant writes.
	result := t.db.Exec(`UPDATE tenant_balances SET available_bytes=available_bytes-?,
        updated_at=?,usage_version=usage_version+CASE WHEN ?::bigint>0 THEN 1 ELSE 0 END
        WHERE tenant_id=?`, bytes, now, bytes, tenant)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return fmt.Errorf("missing balance for metered tenant %q", tenant)
	}
	if bytes == 0 {
		return nil
	}
	if err := reconcileTrafficAlert(t.db, tenant); err != nil {
		return err
	}
	return t.db.Exec(`WITH fifo AS (
        SELECT id,COALESCE(SUM(remaining_bytes) OVER (
            ORDER BY granted_at,id ROWS BETWEEN UNBOUNDED PRECEDING AND 1 PRECEDING
        ),0) AS preceding
        FROM traffic_grants WHERE tenant_id=? AND remaining_bytes>0
    ) UPDATE traffic_grants AS g
        SET remaining_bytes=g.remaining_bytes-LEAST(g.remaining_bytes,CAST(? AS bigint)-fifo.preceding)
        FROM fifo WHERE g.id=fifo.id AND fifo.preceding<?`, tenant, bytes, bytes).Error
}

func (t usageTx) AddClientUsage(client string, bytes int64) error {
	return t.db.Exec(`UPDATE proxy_subaccounts SET used_bytes=used_bytes+? WHERE client_id=?`, bytes, client).Error
}

type usageBucketRow struct {
	TenantID, ClientID, GatewayID, UsageDate            string
	MinuteStart                                         time.Time
	InputBytes, OutputBytes, BillableBytes, RecordCount int64
	LastReceivedAt                                      time.Time
}

func (t usageTx) WriteBuckets(gateway string, buckets []usage.MeteredBucket, beforeDate string, now time.Time) error {
	dates := make(map[string]bool)
	var minute []usageBucketRow
	daily := make(map[[2]string]usageBucketRow)
	for _, bucket := range buckets {
		row := usageBucketRow{
			TenantID: bucket.TenantID, ClientID: bucket.ClientID, GatewayID: gateway, UsageDate: bucket.UsageDate, MinuteStart: bucket.MinuteStart,
			InputBytes: bucket.InputBytes, OutputBytes: bucket.OutputBytes, BillableBytes: bucket.BillableBytes,
			RecordCount: bucket.RecordCount, LastReceivedAt: now,
		}
		if bucket.UsageDate >= beforeDate {
			dates[bucket.UsageDate] = true
			minute = append(minute, row)
			continue
		}
		key := [2]string{row.ClientID, row.UsageDate}
		prior := daily[key]
		row.InputBytes += prior.InputBytes
		row.OutputBytes += prior.OutputBytes
		row.BillableBytes += prior.BillableBytes
		row.RecordCount += prior.RecordCount
		daily[key] = row
	}
	for date := range dates {
		if err := ensureUsagePartition(t.db, date); err != nil {
			return err
		}
	}
	if err := writeMeteredBuckets(t.db, minute, true); err != nil {
		return err
	}
	rows := make([]usageBucketRow, 0, len(daily))
	for _, row := range daily {
		rows = append(rows, row)
	}
	return writeMeteredBuckets(t.db, rows, false)
}

func ensureUsagePartition(tx *gorm.DB, date string) error {
	name, err := partitionName(date)
	if err != nil {
		return err
	}
	day, err := time.Parse(time.DateOnly, date)
	if err != nil {
		return err
	}
	// Existing partitions need no DDL or parent-table lock upgrade.
	var exists bool
	if err := tx.Raw("SELECT to_regclass(?) IS NOT NULL", name).Scan(&exists).Error; err != nil || exists {
		return err
	}
	// The archive lock serializes changes to the partition tree.
	return tx.Exec(fmt.Sprintf("CREATE TABLE %s PARTITION OF usage_minutely FOR VALUES FROM ('%s') TO ('%s')",
		name, day.Format(time.DateOnly), day.AddDate(0, 0, 1).Format(time.DateOnly))).Error
}

func writeMeteredBuckets(tx *gorm.DB, rows []usageBucketRow, minute bool) error {
	if len(rows) == 0 {
		return nil
	}
	table := "usage_daily"
	keys := []clause.Column{{Name: "tenant_id"}, {Name: "gateway_id"}, {Name: "usage_date"}, {Name: "client_id"}}
	if minute {
		table = "usage_minutely"
		keys = append(keys, clause.Column{Name: "minute_start"})
	} else {
		tx = tx.Omit("MinuteStart")
	}
	updates := make(map[string]any)
	for _, column := range []string{"input_bytes", "output_bytes", "billable_bytes", "record_count"} {
		updates[column] = gorm.Expr(table + "." + column + " + EXCLUDED." + column)
	}
	updates["last_received_at"] = gorm.Expr("GREATEST(" + table + ".last_received_at, EXCLUDED.last_received_at)")
	return tx.Table(table).Clauses(clause.OnConflict{Columns: keys, DoUpdates: clause.Assignments(updates)}).CreateInBatches(rows, 500).Error
}
