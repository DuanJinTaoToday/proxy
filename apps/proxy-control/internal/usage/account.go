package usage

import (
	"math"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type grant struct {
	ID                   int64
	GrantedBytes         int64
	UsedBytes            int64
	OverdraftOffsetBytes int64
	StartsAt             time.Time
	ExpiresAt            *time.Time
	ActivatedAt          *time.Time
	Source               string
	dirty                bool
}

func (g *grant) activeAt(at time.Time) bool {
	return !g.StartsAt.After(at) && (g.ExpiresAt == nil || g.ExpiresAt.After(at))
}

type account struct {
	id             string
	overdraft      int64
	warningSince   *time.Time
	lastReceivedAt *time.Time
	admission      int64
	warning        int64
	grants         []grant
}

func loadAccount(tx *gorm.DB, id string, now time.Time) (*account, error) {
	if err := tx.Exec(`INSERT INTO tenant_balances (tenant_id, updated_at) VALUES (?, ?)
		ON CONFLICT DO NOTHING`, id, now).Error; err != nil {
		return nil, err
	}
	var row struct {
		OverdraftBytes               int64
		WarningSince, LastReceivedAt *time.Time
	}
	if err := tx.Table("tenant_balances").Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("tenant_id = ?", id).Take(&row).Error; err != nil {
		return nil, err
	}
	a := &account{id: id, overdraft: row.OverdraftBytes, warningSince: row.WarningSince, lastReceivedAt: row.LastReceivedAt}
	if err := tx.Table("traffic_grants").Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("tenant_id = ? AND status = 'active'", id).Order("expires_at NULLS LAST, created_at, id").Find(&a.grants).Error; err != nil {
		return nil, err
	}
	if err := loadThresholds(tx, a, now); err != nil {
		return nil, err
	}
	return a, a.activate(tx, now)
}

const scopeFilter = `
	(scope_type = 'tenant' AND scope_id = sample.tenant_id) OR
	(scope_type = 'group' AND scope_id = (
		SELECT group_id FROM tenant_group_members WHERE tenant_id = sample.tenant_id AND effective_from <= sample.recorded_at
		ORDER BY effective_from DESC LIMIT 1
	)) OR (scope_type = 'global' AND scope_id = '')`

const scopeOrder = "CASE scope_type WHEN 'tenant' THEN 0 WHEN 'group' THEN 1 ELSE 2 END"

func loadThresholds(tx *gorm.DB, a *account, now time.Time) error {
	var settings []struct{ AdmissionThresholdBytes, WarningThresholdBytes *int64 }
	if err := tx.Raw("SELECT admission_threshold_bytes, warning_threshold_bytes FROM billing_settings, (SELECT ?::text AS tenant_id, ?::timestamptz AS recorded_at) sample WHERE "+scopeFilter+" ORDER BY "+scopeOrder,
		a.id, now).Scan(&settings).Error; err != nil {
		return err
	}
	a.admission, a.warning = 10*1024*1024, 1000*1024*1024
	var admissionSet, warningSet bool
	for _, setting := range settings {
		if !admissionSet && setting.AdmissionThresholdBytes != nil {
			a.admission, admissionSet = *setting.AdmissionThresholdBytes, true
		}
		if !warningSet && setting.WarningThresholdBytes != nil {
			a.warning, warningSet = *setting.WarningThresholdBytes, true
		}
	}
	return nil
}

func (a *account) available(now time.Time) (int64, error) {
	total := -a.overdraft
	for _, g := range a.grants {
		if g.activeAt(now) && g.ActivatedAt != nil {
			remaining := g.GrantedBytes - g.UsedBytes
			if total > math.MaxInt64-remaining {
				return 0, errByteOverflow
			}
			total += remaining
		}
	}
	return total, nil
}

func (a *account) activate(tx *gorm.DB, now time.Time) error {
	// Activation is chronological, independently of the expiry order used for allocation.
	var due []*grant
	for i := range a.grants {
		g := &a.grants[i]
		if !g.StartsAt.After(now) && g.ActivatedAt == nil {
			due = append(due, g)
		}
	}
	sortActivations(due)
	for _, g := range due {
		if !g.activeAt(now) {
			// Keep the grant available to historical samples without offsetting
			// current debt after the whole activation window was missed.
			g.ActivatedAt = &now
			g.dirty = true
			continue
		}
		before, err := a.available(now)
		if err != nil {
			return err
		}
		offset := min(a.overdraft, g.GrantedBytes-g.UsedBytes)
		a.overdraft -= offset
		g.UsedBytes += offset
		g.OverdraftOffsetBytes += offset
		g.ActivatedAt = &now
		g.dirty = true
		if offset > 0 {
			after, err := a.available(now)
			if err != nil {
				return err
			}
			if err := writeLedger(tx, a.id, g.ID, "overdraft_offset", -offset, before, after, nil, now); err != nil {
				return err
			}
		}
	}
	return nil
}

func writeLedger(tx *gorm.DB, tenant string, grantID int64, kind string, delta, before, after int64, correction *int64, now time.Time) error {
	return tx.Exec(`INSERT INTO balance_ledger
		(tenant_id, grant_id, kind, delta_bytes, available_before, available_after, correction_of, operator, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, 'system', ?)`, tenant, grantID, kind, delta, before, after, correction, now).Error
}

func (a *account) save(tx *gorm.DB, now time.Time) error {
	available, err := a.available(now)
	if err != nil {
		return err
	}
	if available >= a.warning {
		a.warningSince = nil
	} else if a.warningSince == nil {
		a.warningSince = &now
	}
	for _, g := range a.grants {
		if !g.dirty {
			continue
		}
		if err := tx.Exec(`UPDATE traffic_grants SET used_bytes = ?, overdraft_offset_bytes = ?, activated_at = ? WHERE id = ?`,
			g.UsedBytes, g.OverdraftOffsetBytes, g.ActivatedAt, g.ID).Error; err != nil {
			return err
		}
	}
	return tx.Exec(`UPDATE tenant_balances SET overdraft_bytes = ?, warning_since = ?, last_received_at = ?,
		updated_at = ? WHERE tenant_id = ?`, a.overdraft, a.warningSince, a.lastReceivedAt, now, a.id).Error
}

func (a *account) correctExpiry(tx *gorm.DB, g *grant, debit int64, now time.Time) error {
	if g.ExpiresAt == nil || g.ExpiresAt.After(now) {
		return nil
	}
	var original struct{ ID int64 }
	q := tx.Raw("SELECT id FROM balance_ledger WHERE grant_id = ? AND kind = 'grant_expire'", g.ID).Scan(&original)
	if q.Error != nil || q.RowsAffected == 0 {
		return q.Error
	}
	available, err := a.available(now)
	if err != nil {
		return err
	}
	return writeLedger(tx, a.id, g.ID, "grant_expire_correction", debit, available, available, &original.ID, now)
}
