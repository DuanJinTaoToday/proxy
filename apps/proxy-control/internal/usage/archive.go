package usage

import (
	"context"
	"fmt"
	"time"

	"gorm.io/gorm"
)

// Ingestion and partition maintenance acquire this lock before touching any
// usage partition. CREATE/DROP PARTITION locks the parent table.
func lockArchiveState(tx *gorm.DB) (string, error) {
	var state struct{ BeforeDate string }
	err := tx.Raw(`SELECT to_char(before_date, 'YYYY-MM-DD') AS before_date
		FROM usage_archive_state WHERE singleton FOR UPDATE`).Take(&state).Error
	return state.BeforeDate, err
}

func partitionName(date string) (string, time.Time, error) {
	day, err := time.Parse(time.DateOnly, date)
	if err != nil {
		return "", time.Time{}, err
	}
	return "usage_minutely_" + day.Format("20060102"), day, nil
}

func ensurePartition(tx *gorm.DB, date string) error {
	name, day, err := partitionName(date)
	if err != nil {
		return err
	}
	var exists bool
	if err := tx.Raw("SELECT to_regclass(?) IS NOT NULL", name).Scan(&exists).Error; err != nil || exists {
		return err
	}
	statement := fmt.Sprintf("CREATE TABLE %s PARTITION OF usage_minutely FOR VALUES FROM ('%s') TO ('%s')",
		name, day.Format(time.DateOnly), day.AddDate(0, 0, 1).Format(time.DateOnly))
	return tx.Exec(statement).Error
}

func (s *Service) Archive(ctx context.Context, now time.Time) error {
	cutoff := now.In(shanghai).AddDate(0, 0, -60).Format(time.DateOnly)
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		before, err := lockArchiveState(tx)
		if err != nil || cutoff <= before {
			return err
		}
		var dates []string
		if err := tx.Raw(`SELECT DISTINCT to_char(usage_date, 'YYYY-MM-DD')
			FROM usage_minutely WHERE usage_date < ? ORDER BY 1`, cutoff).Scan(&dates).Error; err != nil {
			return err
		}
		for _, date := range dates {
			if err := archiveDay(tx, date); err != nil {
				return err
			}
		}
		return tx.Exec("UPDATE usage_archive_state SET before_date = ? WHERE singleton", cutoff).Error
	})
}

func archiveDay(tx *gorm.DB, date string) error {
	if err := tx.Exec(`INSERT INTO usage_daily
		(tenant_id, gateway_id, usage_date, grant_id, rate_id, source, input_bytes, output_bytes, billable_bytes, record_count, last_received_at)
		SELECT tenant_id, gateway_id, usage_date, grant_id, rate_id, source,
			SUM(input_bytes), SUM(output_bytes), SUM(billable_bytes), SUM(record_count), MAX(last_received_at)
		FROM usage_minutely WHERE usage_date = ?
		GROUP BY tenant_id, gateway_id, usage_date, grant_id, rate_id, source
		ON CONFLICT (tenant_id, gateway_id, usage_date, grant_id, rate_id, source) DO UPDATE SET
			input_bytes = usage_daily.input_bytes + EXCLUDED.input_bytes,
			output_bytes = usage_daily.output_bytes + EXCLUDED.output_bytes,
			billable_bytes = usage_daily.billable_bytes + EXCLUDED.billable_bytes,
			record_count = usage_daily.record_count + EXCLUDED.record_count,
			last_received_at = GREATEST(usage_daily.last_received_at, EXCLUDED.last_received_at)`, date).Error; err != nil {
		return err
	}
	name, _, err := partitionName(date)
	if err != nil {
		return err
	}
	return tx.Exec("DROP TABLE " + name).Error
}

func (s *Service) SettleGrants(ctx context.Context, now time.Time) error {
	var ids []string
	if err := s.db.WithContext(ctx).Raw(`SELECT DISTINCT tenant_id FROM traffic_grants g
		WHERE status = 'active' AND starts_at <= @now AND
		(activated_at IS NULL OR expires_at <= @now AND NOT EXISTS (
			SELECT 1 FROM balance_ledger WHERE grant_id = g.id AND kind = 'grant_expire'
		)) ORDER BY tenant_id`, map[string]any{"now": now}).Scan(&ids).Error; err != nil {
		return err
	}
	for _, id := range ids {
		err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			a, err := loadAccount(tx, id, now)
			if err != nil {
				return err
			}
			available, err := a.available(now)
			if err != nil {
				return err
			}
			for _, g := range a.grants {
				if g.ExpiresAt == nil || g.ExpiresAt.After(now) {
					continue
				}
				if err := tx.Exec(`INSERT INTO balance_ledger
					(tenant_id, grant_id, kind, delta_bytes, available_before, available_after, operator, created_at)
					VALUES (?, ?, 'grant_expire', ?, ?, ?, 'system', ?)
					ON CONFLICT (grant_id) WHERE kind = 'grant_expire' DO NOTHING`,
					id, g.ID, g.UsedBytes-g.GrantedBytes, available, available, now).Error; err != nil {
					return err
				}
			}
			return a.save(tx, now)
		})
		if err != nil {
			return err
		}
		s.refreshBalances(ctx, []string{id})
	}
	return nil
}
