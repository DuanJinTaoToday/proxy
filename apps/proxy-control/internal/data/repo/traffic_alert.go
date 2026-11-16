package repo

import (
	"context"

	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/biz/usage"
	"gorm.io/gorm"
)

const activeTrafficAlert = `kind = 'low_traffic_balance' AND subject_key = '' AND resolved_at IS NULL`

func (r *UsageRepository) TrafficAlert(ctx context.Context, tenant string) (usage.TrafficAlert, error) {
	return readTrafficAlert(r.db.WithContext(ctx), tenant)
}

func (r *UsageRepository) SetTrafficAlert(ctx context.Context, tenant string, threshold *int64) (usage.TrafficAlert, error) {
	var state usage.TrafficAlert
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec(`UPDATE tenant_balances SET low_balance_threshold_bytes = ? WHERE tenant_id = ?`, threshold, tenant).Error; err != nil {
			return err
		}
		if err := reconcileTrafficAlert(tx, tenant); err != nil {
			return err
		}
		var err error
		state, err = readTrafficAlert(tx, tenant)
		return err
	})
	return state, err
}

func readTrafficAlert(db *gorm.DB, tenant string) (usage.TrafficAlert, error) {
	var state usage.TrafficAlert
	err := db.Raw(`SELECT b.available_bytes, b.low_balance_threshold_bytes AS threshold_bytes,
		COALESCE(n.occurrence, 0) AS occurrence, n.created_at AS triggered_at
		FROM tenant_balances AS b
		LEFT JOIN tenant_notifications AS n ON n.tenant_id = b.tenant_id AND `+activeTrafficAlert+`
		WHERE b.tenant_id = ?`, tenant).Take(&state).Error
	return state, err
}

// Call after locking and updating the tenant balance row in the same transaction.
func reconcileTrafficAlert(tx *gorm.DB, tenant string) error {
	if err := tx.Exec(`UPDATE tenant_notifications SET resolved_at = clock_timestamp()
		WHERE tenant_id = ? AND `+activeTrafficAlert+` AND EXISTS (
			SELECT 1 FROM tenant_balances WHERE tenant_id = ?
			AND (low_balance_threshold_bytes IS NULL OR available_bytes > low_balance_threshold_bytes)
		)`, tenant, tenant).Error; err != nil {
		return err
	}
	return tx.Exec(`WITH new_cycle AS (
		UPDATE tenant_balances SET low_balance_cycle = low_balance_cycle + 1
		WHERE tenant_id = ? AND available_bytes <= low_balance_threshold_bytes
		AND NOT EXISTS (SELECT 1 FROM tenant_notifications WHERE tenant_id = ? AND `+activeTrafficAlert+`)
		RETURNING tenant_id, low_balance_cycle
	)
	INSERT INTO tenant_notifications (tenant_id, kind, subject_key, occurrence, created_at)
	SELECT tenant_id, 'low_traffic_balance', '', low_balance_cycle, clock_timestamp() FROM new_cycle`, tenant, tenant).Error
}
