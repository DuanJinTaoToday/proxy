package repo

import (
	"time"

	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/biz/usage"
)

func (t usageTx) LockBillingConfig(exclusive bool) error {
	if exclusive {
		return t.db.Exec("SELECT pg_advisory_xact_lock(14014, 1)").Error
	}
	return t.db.Exec("SELECT pg_advisory_xact_lock_shared(14014, 1)").Error
}

func (t usageTx) BillingSettings(scope usage.Scope) (usage.Threshold, error) {
	var settings usage.Threshold
	err := t.db.Table("billing_settings").Select("admission_threshold_bytes", "warning_threshold_bytes").
		Where("scope_type = ? AND scope_id = ?", scope.Type, scope.ID).Scan(&settings).Error
	return settings, err
}

func (t usageTx) SaveBillingSettings(scope usage.Scope, settings usage.Threshold, actor string, now time.Time) error {
	return t.db.Exec(`INSERT INTO billing_settings
		(scope_type, scope_id, admission_threshold_bytes, warning_threshold_bytes, updated_by, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT (scope_type, scope_id) DO UPDATE SET
			admission_threshold_bytes = EXCLUDED.admission_threshold_bytes,
			warning_threshold_bytes = EXCLUDED.warning_threshold_bytes,
			updated_by = EXCLUDED.updated_by, updated_at = EXCLUDED.updated_at`,
		scope.Type, scope.ID, settings.AdmissionThresholdBytes, settings.WarningThresholdBytes, actor, now).Error
}

func (t usageTx) GroupExists(id string) (bool, error) {
	var exists bool
	err := t.db.Raw("SELECT EXISTS (SELECT 1 FROM tenant_groups WHERE id = ?)", id).Scan(&exists).Error
	return exists, err
}

func (t usageTx) InsertGroup(id, name string, now time.Time) (bool, error) {
	result := t.db.Exec("INSERT INTO tenant_groups (id, name, created_at) VALUES (?, ?, ?) ON CONFLICT (id) DO NOTHING", id, name, now)
	return result.RowsAffected == 1, result.Error
}

func (t usageTx) AppendGroupMembership(tenant, group, actor string, now time.Time) (time.Time, error) {
	var effective time.Time
	err := t.db.Raw(`INSERT INTO tenant_group_members (tenant_id, group_id, effective_from, updated_by, updated_at)
		SELECT ?, NULLIF(?, ''), GREATEST(?::timestamptz, MAX(effective_from) + interval '1 microsecond'), ?, ?
		FROM tenant_group_members WHERE tenant_id = ?
		RETURNING effective_from`, tenant, group, now, actor, now, tenant).Scan(&effective).Error
	return effective, err
}

func (t usageTx) InsertBillingRate(rate usage.RateVersion, actor string, now time.Time) (int64, error) {
	var id int64
	err := t.db.Raw(`INSERT INTO billing_rates
		(scope_type, scope_id, upload_ratio_milli, download_ratio_milli, effective_from, created_by, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (scope_type, scope_id, effective_from) DO NOTHING RETURNING id`,
		rate.ScopeType, rate.ScopeID, rate.UploadRatioMilli, rate.DownloadRatioMilli, rate.EffectiveFrom, actor, now).Scan(&id).Error
	return id, err
}
