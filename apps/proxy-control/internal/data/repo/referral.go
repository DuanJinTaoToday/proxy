package repo

import (
	"context"

	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/biz/access"
	"gorm.io/gorm"
)

func (r *AccountRepository) GetOrCreateInvite(ctx context.Context, tenantID string) (string, bool, error) {
	db := r.db.WithContext(ctx)
	if err := ensurePromotion(db, tenantID); err != nil {
		return "", false, err
	}
	var invite struct {
		InviteCode string
		Paused     bool
	}
	result := db.Raw(`SELECT replace(invite_code::text, '-', '') AS invite_code, paused
        FROM tenant_promotions WHERE tenant_id = ?`, tenantID).Scan(&invite)
	if result.Error == nil && result.RowsAffected == 0 {
		return "", false, access.ErrTenantNotFound
	}
	return invite.InviteCode, invite.Paused, result.Error
}

func ensurePromotion(db *gorm.DB, tenantID string) error {
	return db.Exec(`INSERT INTO tenant_promotions (tenant_id) SELECT id FROM tenants WHERE id = ?
        ON CONFLICT (tenant_id) DO NOTHING`, tenantID).Error
}

func (r *AccountRepository) PromotionPaused(ctx context.Context, tenantID string) (bool, error) {
	var paused bool
	result := r.db.WithContext(ctx).Raw(`SELECT COALESCE(p.paused, false)
        FROM tenants t LEFT JOIN tenant_promotions p ON p.tenant_id = t.id WHERE t.id = ?`, tenantID).Scan(&paused)
	if result.Error != nil {
		return false, result.Error
	}
	if result.RowsAffected == 0 {
		return false, access.ErrTenantNotFound
	}
	return paused, nil
}

func (r *AccountRepository) SetPromotionPaused(ctx context.Context, tenantID, actor string, paused bool) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := ensurePromotion(tx, tenantID); err != nil {
			return err
		}
		var previous bool
		result := tx.Raw("SELECT paused FROM tenant_promotions WHERE tenant_id = ? FOR UPDATE", tenantID).Scan(&previous)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return access.ErrTenantNotFound
		}
		if previous == paused {
			return nil
		}
		if err := tx.Exec("UPDATE tenant_promotions SET paused = ? WHERE tenant_id = ?", paused, tenantID).Error; err != nil {
			return err
		}
		return tx.Exec(`INSERT INTO tenant_promotion_audit (tenant_id, operator_id, previous_paused, paused)
            VALUES (?, ?, ?, ?)`, tenantID, actor, previous, paused).Error
	})
}

func directReferrer(tx *gorm.DB, tenant access.Tenant, invite access.RegistrationInvite) (string, error) {
	if !invite.Valid() {
		return "", access.ErrInvalidInviteCode
	}
	var referrer struct {
		TenantID, Email string
		Paused          bool
	}
	// Keep promotion changes behind this registration until the binding commits.
	result := tx.Raw(`SELECT p.tenant_id, p.paused, a.email
        FROM tenant_promotions p JOIN tenant_admins a USING (tenant_id)
        WHERE p.invite_code = CAST(? AS uuid) AND a.email IS NOT NULL FOR SHARE OF p`, invite.Code).Scan(&referrer)
	if result.Error != nil {
		return "", result.Error
	}
	if result.RowsAffected == 0 || referrer.TenantID == tenant.ID {
		return "", access.ErrInvalidInviteCode
	}
	if referrer.Email == tenant.Email {
		return "", access.ErrEmailOccupied
	}
	if referrer.Paused {
		return "", access.ErrPromotionPaused
	}
	return referrer.TenantID, nil
}
