package repo

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/biz/proxyauth"
	"gorm.io/gorm"
)

type ProxyAuthRepository struct{ db *gorm.DB }

func NewProxyAuthRepository(db *gorm.DB) *ProxyAuthRepository {
	return &ProxyAuthRepository{db: db}
}

func (s *ProxyAuthRepository) InsertCredential(ctx context.Context, id, tenant, username string, hash []byte, expiry *time.Time, cidrs []string, actor string) (bool, error) {
	result := s.db.WithContext(ctx).Exec(`INSERT INTO proxy_credentials
			(client_id, tenant_id, username, secret_hash, expires_at, allowed_source_cidrs, created_by)
			VALUES (?, ?, ?, ?, ?, string_to_array(?, ',')::cidr[], ?) ON CONFLICT DO NOTHING`,
		id, tenant, username, hash, expiry, strings.Join(cidrs, ","), actor)
	return result.RowsAffected != 0, result.Error
}

func (s *ProxyAuthRepository) DisableCredential(ctx context.Context, tenant, clientID string) (bool, error) {
	result := s.db.WithContext(ctx).Exec(`UPDATE proxy_credentials SET status = 'disabled',
		disabled_at = COALESCE(disabled_at, now()) WHERE tenant_id = ? AND client_id = ?`, tenant, clientID)
	return result.RowsAffected != 0, result.Error
}

func (s *ProxyAuthRepository) WithGateway(ctx context.Context, hash []byte, handle func(proxyauth.GatewayTx, proxyauth.GatewayIdentity) error) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row struct{ GatewayID, Status string }
		result := tx.Raw(`SELECT g.id AS gateway_id, g.status FROM gateway_tokens t JOIN gateways g ON g.id = t.gateway_id
			WHERE t.scope = 'auth' AND t.token_hash = ? AND t.revoked_at IS NULL FOR SHARE OF t, g`, hash).Scan(&row)
		if result.Error != nil {
			return result.Error
		}
		return handle(authTx{db: tx}, proxyauth.GatewayIdentity{
			ID: row.GatewayID, Status: row.Status, Found: result.RowsAffected != 0,
		})
	})
}

type authTx struct{ db *gorm.DB }

type credentialRow struct {
	Reference, Username, ClientID, TenantID string
	SecretHash                              []byte
	Status                                  string
	ExpiresAt                               *time.Time
	Cidrs                                   string
}

func (r credentialRow) credential() (proxyauth.Credential, error) {
	credential := proxyauth.Credential{Reference: r.Reference, Username: r.Username, ClientID: r.ClientID,
		TenantID: r.TenantID, SecretHash: r.SecretHash, Status: r.Status, ExpiresAt: r.ExpiresAt}
	err := json.Unmarshal([]byte(r.Cidrs), &credential.AllowedSourceCIDRs)
	return credential, err
}

const credentialColumns = `p.username, p.client_id, p.tenant_id, p.secret_hash, p.status, p.expires_at,
	array_to_json(p.allowed_source_cidrs)::text AS cidrs`

func (t authTx) FindCredential(username string) (proxyauth.Credential, bool, error) {
	var row credentialRow
	result := t.db.Raw("SELECT "+credentialColumns+" FROM proxy_credentials p WHERE p.username = ? FOR SHARE", username).Scan(&row)
	if result.Error != nil || result.RowsAffected == 0 {
		return proxyauth.Credential{}, false, result.Error
	}
	credential, err := row.credential()
	return credential, true, err
}

func (t authTx) Reference(gateway, clientID, newReference string) (string, error) {
	if err := t.db.Exec(`INSERT INTO gateway_auth_references (gateway_id, client_id, reference)
 VALUES (?, ?, ?) ON CONFLICT (gateway_id, client_id) DO NOTHING`, gateway, clientID, newReference).Error; err != nil {
		return "", err
	}
	var reference string
	err := t.db.Raw(`SELECT reference FROM gateway_auth_references WHERE gateway_id = ? AND client_id = ?`, gateway, clientID).Scan(&reference).Error
	return reference, err
}

func (t authTx) FindReferences(gateway string, refs []string) ([]proxyauth.Credential, error) {
	var rows []credentialRow
	err := t.db.Raw("SELECT r.reference, "+credentialColumns+` FROM gateway_auth_references r
 JOIN proxy_credentials p ON p.client_id = r.client_id WHERE r.gateway_id = ? AND r.reference IN ?`, gateway, refs).Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	credentials := make([]proxyauth.Credential, 0, len(rows))
	for _, row := range rows {
		credential, err := row.credential()
		if err != nil {
			return nil, err
		}
		credentials = append(credentials, credential)
	}
	return credentials, nil
}

func (t authTx) Quota(tenant string, now time.Time) (*proxyauth.Quota, error) {
	var row struct {
		AvailableBytes          int64
		AdmissionThresholdBytes uint64
		ValidUntil              *time.Time
	}
	err := t.db.Raw(`WITH current_group AS (
		SELECT group_id FROM tenant_group_members WHERE tenant_id = @tenant AND effective_from <= @now
		ORDER BY effective_from DESC LIMIT 1
	), grants AS (
		SELECT COALESCE(SUM(granted_bytes - used_bytes) FILTER (
			WHERE starts_at <= @now AND (expires_at IS NULL OR expires_at > @now) AND activated_at IS NOT NULL
		), 0) AS available_bytes,
		LEAST(MIN(starts_at) FILTER (WHERE starts_at > @now),
			MIN(expires_at) FILTER (WHERE starts_at <= @now AND expires_at > @now)) AS valid_until
		FROM traffic_grants WHERE tenant_id = @tenant AND status = 'active'
	)
	SELECT grants.available_bytes - COALESCE((SELECT overdraft_bytes FROM tenant_balances WHERE tenant_id = @tenant), 0)
		AS available_bytes, grants.valid_until,
		COALESCE((SELECT admission_threshold_bytes FROM billing_settings WHERE admission_threshold_bytes IS NOT NULL
			AND ((scope_type = 'tenant' AND scope_id = @tenant)
			OR (scope_type = 'group' AND scope_id = (SELECT group_id FROM current_group))
			OR (scope_type = 'global' AND scope_id = ''))
			ORDER BY CASE scope_type WHEN 'tenant' THEN 0 WHEN 'group' THEN 1 ELSE 2 END LIMIT 1), 10485760)
		AS admission_threshold_bytes
	FROM grants`, map[string]any{"tenant": tenant, "now": now}).Scan(&row).Error
	if err != nil {
		return nil, err
	}
	return &proxyauth.Quota{AvailableBytes: row.AvailableBytes, AdmissionThresholdBytes: row.AdmissionThresholdBytes, ValidUntil: row.ValidUntil}, nil
}
