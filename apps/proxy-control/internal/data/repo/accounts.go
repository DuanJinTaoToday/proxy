package repo

import (
	"context"

	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/biz/access"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/data"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type AccountRepository struct{ db *gorm.DB }

func ProvideAccounts(resources *data.Data) access.Repository {
	db := resources.UsageDB()
	if db == nil {
		db = resources.ProxyAuthDB()
	}
	if db == nil {
		return nil
	}
	return NewAccountRepository(db)
}

func NewAccountRepository(db *gorm.DB) *AccountRepository {
	return &AccountRepository{db: db.Session(&gorm.Session{Logger: logger.Discard})}
}

type accountRow struct {
	ID, Name, PrimaryAccountName, Email, EmailDeliveryAddress, PasswordHash string
	Version                                                                 int64
}

func (r *AccountRepository) customer(ctx context.Context, field, value string) (access.Principal, error) {
	var row accountRow
	err := r.db.WithContext(ctx).Raw(`SELECT t.id,t.name,t.primary_account_name,COALESCE(a.email, '') AS email,COALESCE(a.email_delivery_address, '') AS email_delivery_address,a.password_hash,a.session_version AS version
 FROM tenant_admins a JOIN tenants t ON t.id=a.tenant_id WHERE `+field+" = ?", value).Scan(&row).Error
	return access.Principal{ID: row.ID, EmailDeliveryAddress: row.EmailDeliveryAddress, PasswordHash: row.PasswordHash, Version: row.Version, Tenant: access.Tenant{ID: row.ID, Name: row.Name, PrimaryAccountName: row.PrimaryAccountName, Email: row.Email}}, err
}

func (r *AccountRepository) CustomerByEmail(ctx context.Context, v string) (access.Principal, error) {
	return r.customer(ctx, "a.email", v)
}

func (r *AccountRepository) CustomerByTenant(ctx context.Context, v string) (access.Principal, error) {
	return r.customer(ctx, "a.tenant_id", v)
}

func (r *AccountRepository) operator(ctx context.Context, field, value string) (access.Principal, error) {
	var rows []struct {
		ID, Username, PasswordHash, Permission string
		Version                                int64
	}
	err := r.db.WithContext(ctx).Raw(`SELECT o.id,o.username,o.password_hash,o.session_version AS version,p.permission
 FROM operators o LEFT JOIN operator_permissions p ON p.operator_id=o.id WHERE `+field+" = ?", value).Scan(&rows).Error
	if err != nil || len(rows) == 0 {
		return access.Principal{}, err
	}
	first := rows[0]
	result := access.Principal{ID: first.ID, Username: first.Username, PasswordHash: first.PasswordHash, Version: first.Version, Permissions: make(map[access.Permission]bool)}
	for _, row := range rows {
		switch row.Permission {
		case "read_operations":
			result.Permissions[access.ReadOperations] = true
		case "manage_billing":
			result.Permissions[access.ManageBilling] = true
		case "manage_tenant_accounts":
			result.Permissions[access.ManageTenantAccounts] = true
		}
	}
	return result, nil
}

func (r *AccountRepository) OperatorByName(ctx context.Context, v string) (access.Principal, error) {
	return r.operator(ctx, "o.username", v)
}

func (r *AccountRepository) OperatorByID(ctx context.Context, v string) (access.Principal, error) {
	return r.operator(ctx, "o.id", v)
}

func (r *AccountRepository) ResetPassword(ctx context.Context, tenant, digest, actor string) (bool, error) {
	q := r.db.WithContext(ctx).Exec(`UPDATE tenant_admins SET password_hash=?,session_version=session_version+1,
 password_reset_at=now(),password_reset_by=? WHERE tenant_id=?`, digest, actor, tenant)
	return q.RowsAffected != 0, q.Error
}
