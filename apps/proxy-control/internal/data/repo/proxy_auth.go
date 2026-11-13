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
