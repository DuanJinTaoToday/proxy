package repo

import (
	"time"

	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/biz/usage"
)

func (t usageTx) CreateGrant(tenant, actor string, grant usage.Grant, now time.Time) (int64, error) {
	var id int64
	err := t.db.Raw(`INSERT INTO traffic_grants
		(tenant_id, granted_bytes, used_bytes, overdraft_offset_bytes, starts_at, expires_at, activated_at, source, created_by, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?) RETURNING id`, tenant, grant.GrantedBytes, grant.UsedBytes,
		grant.OverdraftOffsetBytes, grant.StartsAt, grant.ExpiresAt, grant.ActivatedAt, grant.Source, actor, now).Scan(&id).Error
	return id, err
}

func (t usageTx) SaveGrantAmount(grant usage.Grant) error {
	return t.db.Exec("UPDATE traffic_grants SET granted_bytes = ? WHERE id = ?", grant.GrantedBytes, grant.ID).Error
}

func (t usageTx) FindBalanceChange(tenant, externalRef string) (usage.LedgerEntry, error) {
	var entry usage.LedgerEntry
	err := t.db.Table("balance_ledger").Where("tenant_id = ? AND external_ref = ?", tenant, externalRef).Scan(&entry).Error
	return entry, err
}

func (t usageTx) WriteBalanceChange(tenant string, entry usage.LedgerEntry) (usage.LedgerEntry, error) {
	err := t.db.Raw(`INSERT INTO balance_ledger
		(tenant_id, grant_id, kind, delta_bytes, available_before, available_after, external_ref, reason, operator, created_at, overdraft_added_bytes)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?) RETURNING id`, tenant, entry.GrantID, entry.Kind, entry.DeltaBytes,
		entry.AvailableBefore, entry.AvailableAfter, entry.ExternalRef, entry.Reason, entry.Operator, entry.CreatedAt,
		entry.OverdraftAddedBytes).Scan(&entry.ID).Error
	return entry, err
}
