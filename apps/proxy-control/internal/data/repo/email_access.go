package repo

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"

	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/biz/access"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
)

func (r *AccountRepository) StoreCredential(ctx context.Context, credential access.EmailCredential) error {
	var tenant, operator sql.NullString
	var version sql.NullInt64
	if credential.TenantID != "" {
		tenant = sql.NullString{String: credential.TenantID, Valid: true}
		version = sql.NullInt64{Int64: credential.SessionVersion, Valid: true}
	}
	if credential.OperatorID != "" {
		operator = sql.NullString{String: credential.OperatorID, Valid: true}
	}
	result := r.db.WithContext(ctx).Exec(`
        INSERT INTO email_access_credentials
            (purpose, subject, email, delivery_address, tenant_id, session_version, operator_id, secret_hash, expires_at)
        SELECT ?, ?, ?, ?, ?, ?, ?, decode(?, 'hex'), now() + interval '1 second' * CAST(? AS integer)
        WHERE ? <> 'recover' OR NOT EXISTS (SELECT 1 FROM tenant_admins WHERE email = ?)
        ON CONFLICT (purpose, subject) DO UPDATE SET
            email = excluded.email, delivery_address = excluded.delivery_address,
            tenant_id = excluded.tenant_id, session_version = excluded.session_version,
            operator_id = excluded.operator_id, secret_hash = excluded.secret_hash,
            expires_at = excluded.expires_at, attempts = 0, consumed_at = NULL`,
		credential.Purpose, credential.Subject, credential.Email, credential.DeliveryAddress, tenant, version, operator,
		hex.EncodeToString(credential.SecretHash), int(credential.Lifetime.Seconds()), credential.Purpose, credential.Email)
	if result.Error == nil && result.RowsAffected != 1 {
		return access.ErrEmailOccupied
	}
	return result.Error
}

var errEmailRequestLimited = errors.New("email request rate limit reached")

func (r *AccountRepository) AllowEmailRequest(ctx context.Context, email, source string) (bool, error) {
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, limit := range []struct {
			key              string
			seconds, maximum int
		}{{"source:" + source, 3600, 60}, {"email:" + email, 900, 5}} {
			hash := sha256.Sum256([]byte(limit.key))
			result := tx.Exec(`
                INSERT INTO email_access_rate_limits (key, expires_at, count)
                VALUES (decode(?, 'hex'), now() + interval '1 second' * CAST(? AS integer), 1)
                ON CONFLICT (key) DO UPDATE SET
                    count = CASE WHEN email_access_rate_limits.expires_at <= now() THEN 1
                        ELSE email_access_rate_limits.count + 1 END,
                    expires_at = CASE WHEN email_access_rate_limits.expires_at <= now() THEN excluded.expires_at
                        ELSE email_access_rate_limits.expires_at END
                WHERE email_access_rate_limits.expires_at <= now() OR email_access_rate_limits.count < ?`, hex.EncodeToString(hash[:]), limit.seconds, limit.maximum)
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected == 0 {
				return errEmailRequestLimited
			}
		}
		return tx.Exec(`
            DELETE FROM email_access_rate_limits WHERE key IN (
                SELECT key FROM email_access_rate_limits WHERE expires_at <= now()
                ORDER BY expires_at LIMIT 12 FOR UPDATE SKIP LOCKED)`).Error
	})
	if errors.Is(err, errEmailRequestLimited) {
		return false, nil
	}
	return err == nil, err
}

type emailCredentialRow struct {
	Subject, Email, DeliveryAddress string
	TenantID, OperatorID            string
	SessionVersion                  int64
	SecretHash                      []byte
}

func activeCredential(tx *gorm.DB, purpose, subject string) error {
	var active bool
	if err := tx.Raw(`SELECT consumed_at IS NULL AND expires_at > clock_timestamp() AND attempts < 5
        FROM email_access_credentials WHERE purpose = ? AND subject = ?`, purpose, subject).Scan(&active).Error; err != nil {
		return err
	}
	if !active {
		return access.ErrInvalidEmailCredential
	}
	return nil
}

func consumeCredential(tx *gorm.DB, purpose, subject string) error {
	return tx.Exec("UPDATE email_access_credentials SET consumed_at = now() WHERE purpose = ? AND subject = ?", purpose, subject).Error
}

func (r *AccountRepository) CreateTenant(ctx context.Context, tenant access.Tenant, digest string, codeHash []byte, invite access.RegistrationInvite) error {
	invalidCode := false
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var credential emailCredentialRow
		result := tx.Raw(`SELECT secret_hash, delivery_address FROM email_access_credentials
            WHERE purpose = 'register' AND subject = ? FOR UPDATE`, tenant.Email).Scan(&credential)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return access.ErrInvalidEmailCredential
		}
		if err := activeCredential(tx, access.RegistrationPurpose, tenant.Email); err != nil {
			return err
		}
		if !hmac.Equal(credential.SecretHash, codeHash) {
			invalidCode = true
			return tx.Exec(`UPDATE email_access_credentials SET attempts = attempts + 1
                WHERE purpose = 'register' AND subject = ?`, tenant.Email).Error
		}
		var referrerID, source sql.NullString
		if invite.Code != "" {
			id, err := directReferrer(tx, tenant, invite)
			if err != nil {
				return err
			}
			referrerID = sql.NullString{String: id, Valid: true}
			source = sql.NullString{String: invite.Source, Valid: true}
		}
		if err := tx.Exec(`INSERT INTO tenants (id, name, primary_account_name, referrer_tenant_id, referral_source)
            VALUES (?, ?, ?, ?, ?)`, tenant.ID, tenant.Name, tenant.PrimaryAccountName, referrerID, source).Error; err != nil {
			return err
		}
		result = tx.Exec(`INSERT INTO tenant_admins (tenant_id, email, email_delivery_address, password_hash)
            VALUES (?, ?, ?, ?) ON CONFLICT (email) DO NOTHING`, tenant.ID, tenant.Email, credential.DeliveryAddress, digest)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return access.ErrEmailOccupied
		}
		if err := tx.Exec("INSERT INTO wallets (tenant_id) VALUES (?)", tenant.ID).Error; err != nil {
			return err
		}
		if err := tx.Exec("INSERT INTO tenant_balances (tenant_id, updated_at) VALUES (?, now())", tenant.ID).Error; err != nil {
			return err
		}
		return consumeCredential(tx, access.RegistrationPurpose, tenant.Email)
	})
	if err == nil && invalidCode {
		return access.ErrInvalidEmailCredential
	}
	return err
}

func (r *AccountRepository) RedeemLink(ctx context.Context, purpose string, secretHash []byte, digest string) error {
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var credential emailCredentialRow
		result := tx.Raw(`SELECT subject, email, delivery_address, tenant_id, session_version, operator_id
            FROM email_access_credentials WHERE purpose = ? AND secret_hash = decode(?, 'hex') AND purpose <> 'register'
            FOR UPDATE`, purpose, hex.EncodeToString(secretHash)).Scan(&credential)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return access.ErrInvalidEmailCredential
		}
		if err := activeCredential(tx, purpose, credential.Subject); err != nil {
			return err
		}
		var resultUpdate *gorm.DB
		switch purpose {
		case access.PasswordResetPurpose:
			resultUpdate = tx.Exec(`UPDATE tenant_admins SET password_hash = ?, session_version = session_version + 1
                 WHERE tenant_id = ? AND email = ? AND session_version = ?`,
				digest, credential.TenantID, credential.Email, credential.SessionVersion)
		case access.RecoveryPurpose:
			var previous sql.NullString
			account := tx.Raw(`SELECT email FROM tenant_admins WHERE tenant_id = ? AND session_version = ? FOR UPDATE`,
				credential.TenantID, credential.SessionVersion).Scan(&previous)
			if account.Error != nil {
				return account.Error
			}
			if account.RowsAffected == 0 {
				return access.ErrInvalidEmailCredential
			}
			resultUpdate = tx.Exec(`UPDATE tenant_admins SET email = ?, email_delivery_address = ?, password_hash = ?,
                 session_version = session_version + 1 WHERE tenant_id = ?`, credential.Email,
				credential.DeliveryAddress, digest, credential.TenantID)
			if resultUpdate.Error == nil {
				if err := tx.Exec(`INSERT INTO email_recovery_audit (tenant_id, operator_id, previous_email, verified_email)
                    VALUES (?, ?, ?, ?)`, credential.TenantID, credential.OperatorID, previous, credential.Email).Error; err != nil {
					return err
				}
			}
		default:
			return access.ErrInvalidEmailCredential
		}
		if resultUpdate.Error != nil {
			return resultUpdate.Error
		}
		if resultUpdate.RowsAffected == 0 {
			return access.ErrInvalidEmailCredential
		}
		return consumeCredential(tx, purpose, credential.Subject)
	})
	var duplicate *pgconn.PgError
	if errors.As(err, &duplicate) && duplicate.Code == "23505" && duplicate.ConstraintName == "tenant_admins_email_key" {
		return access.ErrEmailOccupied
	}
	return err
}
