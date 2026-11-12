package repo

import (
	"context"
	"database/sql"

	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/biz/usage"
	"gorm.io/gorm"
)

func (r *UsageRepository) AuditGrants(ctx context.Context, tenant string, after int64, limit int) ([]usage.AuditedGrant, int64, error) {
	var grants []usage.AuditedGrant
	var balance struct{ OverdraftBytes int64 }
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Raw(`SELECT id, granted_bytes, used_bytes, overdraft_offset_bytes, starts_at, expires_at, status
			FROM traffic_grants WHERE tenant_id = ? AND id > ? ORDER BY id LIMIT ?`, tenant, after, limit).Scan(&grants).Error; err != nil {
			return err
		}
		return tx.Table("tenant_balances").Select("overdraft_bytes").Where("tenant_id = ?", tenant).Find(&balance).Error
	}, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	return grants, balance.OverdraftBytes, err
}

func (r *UsageRepository) AuditLedger(ctx context.Context, tenant string, after int64, limit int) ([]usage.LedgerEntry, error) {
	var entries []usage.LedgerEntry
	err := r.db.WithContext(ctx).Raw(`SELECT id, kind, COALESCE(grant_id, 0) AS grant_id, delta_bytes,
		available_before, available_after, COALESCE(external_ref, '') AS external_ref, reason, operator, created_at,
		COALESCE(correction_of, 0) AS correction_of
		FROM balance_ledger WHERE tenant_id = ? AND id > ? ORDER BY id LIMIT ?`, tenant, after, limit).Scan(&entries).Error
	return entries, err
}

func (r *UsageRepository) AuditRates(ctx context.Context, scopeType, scopeID string, id, after int64, limit int) ([]usage.RateVersion, error) {
	var rates []usage.RateVersion
	err := r.db.WithContext(ctx).Raw(`SELECT id, scope_type, scope_id, upload_ratio_milli, download_ratio_milli, effective_from
		FROM billing_rates WHERE id > ?
		AND ((? = 0 AND scope_type = ? AND scope_id = ?) OR id = ?) ORDER BY id LIMIT ?`,
		after, id, scopeType, scopeID, id, limit).Scan(&rates).Error
	return rates, err
}
