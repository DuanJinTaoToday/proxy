package repo

import (
	"context"

	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/biz/subaccount"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/data"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type SubaccountRepository struct{ db *gorm.DB }

func ProvideSubaccounts(resources *data.Data) subaccount.Repository {
	db := resources.UsageDB()
	if db == nil {
		db = resources.ProxyAuthDB()
	}
	if db == nil {
		return nil
	}
	return NewSubaccountRepository(db)
}

func NewSubaccountRepository(db *gorm.DB) *SubaccountRepository {
	return &SubaccountRepository{db: db.Session(&gorm.Session{Logger: logger.Discard})}
}

func (r *SubaccountRepository) List(ctx context.Context, tenant string) ([]subaccount.Account, error) {
	var rows []subaccount.Account
	err := r.db.WithContext(ctx).Raw(`SELECT client_id,name,remark,active,limit_bytes,used_bytes
        FROM proxy_subaccounts WHERE tenant_id=? ORDER BY name`, tenant).Scan(&rows).Error
	return rows, err
}

func (r *SubaccountRepository) Find(ctx context.Context, tenant, id string) (subaccount.Account, error) {
	var row subaccount.Account
	err := r.db.WithContext(ctx).Raw(`SELECT * FROM proxy_subaccounts WHERE tenant_id=? AND client_id=?`, tenant, id).Scan(&row).Error
	return row, err
}

func (r *SubaccountRepository) Create(ctx context.Context, a subaccount.Account) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var tenant string
		if err := tx.Raw(`SELECT id FROM tenants WHERE id=? FOR UPDATE`, a.TenantID).Scan(&tenant).Error; err != nil {
			return err
		}
		var count int64
		if err := tx.Raw(`SELECT count(*) FROM proxy_subaccounts WHERE tenant_id=?`, a.TenantID).Scan(&count).Error; err != nil {
			return err
		}
		if count >= 10 {
			return subaccount.ErrTooMany
		}
		result := tx.Exec(`INSERT INTO proxy_subaccounts(client_id,tenant_id,name,password_ciphertext,remark,active,limit_bytes)
            VALUES (?,?,?,?,?,?,?) ON CONFLICT(tenant_id,name) DO NOTHING`,
			a.ClientID, a.TenantID, a.Name, a.PasswordCiphertext, a.Remark, a.Active, a.LimitBytes)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return subaccount.ErrDuplicate
		}
		return nil
	})
}

func (r *SubaccountRepository) Update(ctx context.Context, tenant, id string, u subaccount.Update, encrypted []byte) (subaccount.Account, error) {
	var row subaccount.Account
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Raw(`SELECT client_id,name,remark,active,limit_bytes,used_bytes FROM proxy_subaccounts
            WHERE tenant_id=? AND client_id=? FOR UPDATE`, tenant, id).Scan(&row).Error; err != nil || row.ClientID == "" {
			return err
		}
		if u.UpdateLimit && u.LimitBytes != nil && row.LimitBytes != nil && *u.LimitBytes < *row.LimitBytes {
			return subaccount.ErrLimitDecrease
		}
		return tx.Raw(`UPDATE proxy_subaccounts SET password_ciphertext=CASE WHEN ? THEN ?::bytea ELSE password_ciphertext END,
            credential_version=credential_version+1,remark=COALESCE(?,remark),active=COALESCE(?,active),limit_bytes=CASE WHEN ? THEN CAST(? AS bigint) ELSE limit_bytes END
            WHERE tenant_id=? AND client_id=? RETURNING client_id,name,remark,active,limit_bytes,used_bytes`,
			encrypted != nil, encrypted, u.Remark, u.Active, u.UpdateLimit, u.LimitBytes, tenant, id).Scan(&row).Error
	})
	return row, err
}

func (r *SubaccountRepository) Gateway(ctx context.Context, hash []byte) (string, error) {
	var id string
	err := r.db.WithContext(ctx).Raw(`SELECT g.id FROM gateway_tokens t JOIN gateways g ON g.id=t.gateway_id
        WHERE t.token_hash=? AND t.scope='auth' AND t.revoked_at IS NULL AND g.status='active'`, hash).Scan(&id).Error
	return id, err
}

const authProjection = `SELECT s.*,t.primary_account_name || '.' || s.name AS username,b.available_bytes`
const authJoins = ` FROM proxy_subaccounts s JOIN tenants t ON t.id=s.tenant_id JOIN tenant_balances b ON b.tenant_id=s.tenant_id`

func (r *SubaccountRepository) Load(ctx context.Context, username string) (subaccount.AuthState, error) {
	var row subaccount.AuthState
	err := r.db.WithContext(ctx).Raw(authProjection+authJoins+
		` WHERE t.primary_account_name=split_part(?,'.',1) AND s.name=split_part(?,'.',2)
           AND t.primary_account_name || '.' || s.name=?`, username, username, username).Scan(&row).Error
	return row, err
}

func (r *SubaccountRepository) Bind(ctx context.Context, gateway, client string) (string, error) {
	var ref string
	err := r.db.WithContext(ctx).Raw(`INSERT INTO gateway_auth_references AS refs (gateway_id,client_id,reference)
        VALUES (?,?,?) ON CONFLICT(gateway_id,client_id) DO UPDATE SET reference=refs.reference RETURNING reference`,
		gateway, client, uuid.NewString()).Scan(&ref).Error
	return ref, err
}

func (r *SubaccountRepository) Refresh(ctx context.Context, gateway string, refs []string) ([]subaccount.AuthState, error) {
	var rows []subaccount.AuthState
	err := r.db.WithContext(ctx).Raw(authProjection+`,r.reference`+authJoins+
		` JOIN gateway_auth_references r ON r.client_id=s.client_id WHERE r.gateway_id=? AND r.reference IN ?`, gateway, refs).Scan(&rows).Error
	return rows, err
}
