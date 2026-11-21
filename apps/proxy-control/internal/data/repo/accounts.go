package repo

import (
	"context"
	"errors"

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

func (r *AccountRepository) CreateTenant(ctx context.Context, t access.Tenant, phone, digest string) (bool, error) {
	created := false
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Roll back the tenant row on a concurrently claimed phone.
		if err := tx.Exec("INSERT INTO tenants (id,name,primary_account_name) VALUES (?,?,?)", t.ID, t.Name, t.PrimaryAccountName).Error; err != nil {
			return err
		}
		result := tx.Exec("INSERT INTO tenant_admins (tenant_id,phone_e164,password_hash) VALUES (?,?,?) ON CONFLICT (phone_e164) DO NOTHING", t.ID, phone, digest)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return errPhoneClaimed
		}
		if err := tx.Exec("INSERT INTO wallets (tenant_id) VALUES (?)", t.ID).Error; err != nil {
			return err
		}
		if err := tx.Exec("INSERT INTO tenant_balances (tenant_id,updated_at) VALUES (?,now())", t.ID).Error; err != nil {
			return err
		}
		created = true
		return nil
	})
	if err == errPhoneClaimed {
		return false, nil
	}
	return created, err
}

var errPhoneClaimed = errors.New("phone already claimed")

type accountRow struct {
	ID, Name, PrimaryAccountName, PasswordHash string
	Version                                    int64
}

func (r *AccountRepository) customer(ctx context.Context, field, value string) (access.Principal, error) {
	var row accountRow
	err := r.db.WithContext(ctx).Raw(`SELECT t.id,t.name,t.primary_account_name,a.password_hash,a.session_version AS version
 FROM tenant_admins a JOIN tenants t ON t.id=a.tenant_id WHERE `+field+" = ?", value).Scan(&row).Error
	return access.Principal{ID: row.ID, PasswordHash: row.PasswordHash, Version: row.Version, Tenant: access.Tenant{ID: row.ID, Name: row.Name, PrimaryAccountName: row.PrimaryAccountName}}, err
}

func (r *AccountRepository) CustomerByPhone(ctx context.Context, v string) (access.Principal, error) {
	return r.customer(ctx, "a.phone_e164", v)
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
