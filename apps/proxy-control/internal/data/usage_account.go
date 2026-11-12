package data

import (
	"time"

	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/biz/usage"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/data/model"
	"gorm.io/gorm/clause"
)

func (t usageTx) LoadAccount(id string, now time.Time) (usage.Account, error) {
	if err := t.db.Exec(`INSERT INTO tenant_balances (tenant_id, updated_at) VALUES (?, ?)
		ON CONFLICT DO NOTHING`, id, now).Error; err != nil {
		return usage.Account{}, err
	}
	var row struct {
		OverdraftBytes               int64
		WarningSince, LastReceivedAt *time.Time
	}
	if err := t.db.Table("tenant_balances").Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("tenant_id = ?", id).Take(&row).Error; err != nil {
		return usage.Account{}, err
	}
	a := usage.Account{ID: id, Overdraft: row.OverdraftBytes, WarningSince: row.WarningSince, LastReceivedAt: row.LastReceivedAt}
	var rows []model.TrafficGrant
	if err := t.db.Table("traffic_grants").Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("tenant_id = ? AND status = 'active'", id).Order("expires_at NULLS LAST, created_at, id").Find(&rows).Error; err != nil {
		return usage.Account{}, err
	}
	for _, g := range rows {
		a.Grants = append(a.Grants, usage.Grant{ID: g.ID, GrantedBytes: g.GrantedBytes, UsedBytes: g.UsedBytes,
			OverdraftOffsetBytes: g.OverdraftOffsetBytes, StartsAt: g.StartsAt, ExpiresAt: g.ExpiresAt,
			ActivatedAt: g.ActivatedAt, Source: g.Source})
	}
	return a, nil
}

const scopeFilter = `
	(scope_type = 'tenant' AND scope_id = sample.tenant_id) OR
	(scope_type = 'group' AND scope_id = (
		SELECT group_id FROM tenant_group_members WHERE tenant_id = sample.tenant_id AND effective_from <= sample.recorded_at
		ORDER BY effective_from DESC LIMIT 1
	)) OR (scope_type = 'global' AND scope_id = '')`

const scopeOrder = "CASE scope_type WHEN 'tenant' THEN 0 WHEN 'group' THEN 1 ELSE 2 END"

func (t usageTx) Thresholds(id string, now time.Time) ([]usage.Threshold, error) {
	var settings []usage.Threshold
	err := t.db.Raw("SELECT admission_threshold_bytes, warning_threshold_bytes FROM billing_settings, (SELECT ?::text AS tenant_id, ?::timestamptz AS recorded_at) sample WHERE "+scopeFilter+" ORDER BY "+scopeOrder,
		id, now).Scan(&settings).Error
	return settings, err
}

func (t usageTx) WriteLedger(tenant string, grantID int64, kind string, delta, before, after int64, correction *int64, now time.Time) error {
	return t.db.Exec(`INSERT INTO balance_ledger
		(tenant_id, grant_id, kind, delta_bytes, available_before, available_after, correction_of, operator, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, 'system', ?)`, tenant, grantID, kind, delta, before, after, correction, now).Error
}

func (t usageTx) SaveGrant(g usage.Grant) error {
	return t.db.Exec(`UPDATE traffic_grants SET used_bytes = ?, overdraft_offset_bytes = ?, activated_at = ? WHERE id = ?`,
		g.UsedBytes, g.OverdraftOffsetBytes, g.ActivatedAt, g.ID).Error
}

func (t usageTx) SaveAccount(a usage.Account, now time.Time) error {
	return t.db.Exec(`UPDATE tenant_balances SET overdraft_bytes = ?, warning_since = ?, last_received_at = ?,
		updated_at = ? WHERE tenant_id = ?`, a.Overdraft, a.WarningSince, a.LastReceivedAt, now, a.ID).Error
}

func (t usageTx) ExpiryEntry(grantID int64) (int64, bool, error) {
	var original struct{ ID int64 }
	q := t.db.Raw("SELECT id FROM balance_ledger WHERE grant_id = ? AND kind = 'grant_expire'", grantID).Scan(&original)
	return original.ID, q.RowsAffected != 0, q.Error
}
