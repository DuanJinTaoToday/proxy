package proxyauth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"strings"
	"time"

	v1 "github.com/DuanJinTaoToday/proxy/api/proxycontrol/v1"
	"github.com/go-kratos/kratos/v3/errors"
	"github.com/go-kratos/kratos/v3/transport"
	"google.golang.org/protobuf/types/known/timestamppb"
	"gorm.io/gorm"
)

type storedCredential struct {
	Reference  string
	Username   string
	ClientID   string
	TenantID   string
	SecretHash []byte
	Status     string
	ExpiresAt  *time.Time
	Cidrs      string
}

const credentialColumns = `p.username, p.client_id, p.tenant_id, p.secret_hash, p.status, p.expires_at,
	array_to_json(p.allowed_source_cidrs)::text AS cidrs`

func (s *Service) LoadAuthState(ctx context.Context, req *v1.LoadAuthStateRequest) (*v1.AuthSnapshot, error) {
	noStore(ctx)
	var snapshot *v1.AuthSnapshot
	err := s.withGateway(ctx, func(tx *gorm.DB, gateway string) error {
		username, password := req.GetUsername(), req.GetPassword()
		if username == "" || password == "" || len(username) > 255 || len(password) > 255 || strings.ContainsRune(username, 0) {
			return deniedCredential()
		}
		var row storedCredential
		result := tx.Raw("SELECT "+credentialColumns+" FROM proxy_credentials p WHERE p.username = ? FOR SHARE", username).Scan(&row)
		if result.Error != nil {
			return result.Error
		}
		now := time.Now().UTC()
		hash := sha256.Sum256([]byte(password))
		if result.RowsAffected == 0 || subtle.ConstantTimeCompare(hash[:], row.SecretHash) != 1 {
			return errors.NotFound("INVALID_CREDENTIAL", "credential unavailable")
		}
		if row.Status != "active" {
			return errors.NotFound("CREDENTIAL_DISABLED", "credential unavailable")
		}
		if row.ExpiresAt != nil && !now.Before(*row.ExpiresAt) {
			return errors.NotFound("CREDENTIAL_EXPIRED", "credential unavailable")
		}
		// Durable per-gateway references survive control plane restarts and
		// cannot be used with another gateway's auth token.
		if err := tx.Exec(`INSERT INTO gateway_auth_references (gateway_id, client_id, reference)
			VALUES (?, ?, ?) ON CONFLICT (gateway_id, client_id) DO NOTHING`, gateway, row.ClientID, rand.Text()).Error; err != nil {
			return err
		}
		if err := tx.Raw(`SELECT reference FROM gateway_auth_references WHERE gateway_id = ? AND client_id = ?`,
			gateway, row.ClientID).Scan(&row.Reference).Error; err != nil {
			return err
		}
		snapshot, result.Error = row.snapshot(tx, now, make(map[string]*v1.AuthQuota))
		return result.Error
	})
	if err != nil {
		return nil, databaseError(err)
	}
	return snapshot, nil
}

func deniedCredential() error {
	return errors.NotFound("INVALID_CREDENTIAL", "credential unavailable")
}

func (s *Service) RefreshAuthState(ctx context.Context, req *v1.RefreshAuthStateRequest) (*v1.RefreshAuthStateResponse, error) {
	noStore(ctx)
	reply := &v1.RefreshAuthStateResponse{}
	err := s.withGateway(ctx, func(tx *gorm.DB, gateway string) error {
		refs := req.GetReferences()
		if len(refs) == 0 || len(refs) > v1.GatewayAuthRefreshLimit {
			return errors.BadRequest("INVALID_AUTH_REFERENCES", "expected 1 to 100 credential references")
		}
		for _, ref := range refs {
			if ref == "" || len(ref) > 128 || strings.ContainsRune(ref, 0) {
				return errors.BadRequest("INVALID_AUTH_REFERENCES", "invalid credential reference")
			}
		}
		var rows []storedCredential
		err := tx.Raw("SELECT r.reference, "+credentialColumns+` FROM gateway_auth_references r
			JOIN proxy_credentials p ON p.client_id = r.client_id WHERE r.gateway_id = ? AND r.reference IN ?`,
			gateway, refs).Scan(&rows).Error
		if err != nil {
			return err
		}
		if len(rows) != len(refs) {
			return errors.BadRequest("INVALID_AUTH_REFERENCES", "one or more references were not issued to this gateway")
		}
		quotas := make(map[string]*v1.AuthQuota)
		now := time.Now().UTC()
		for _, row := range rows {
			snapshot, err := row.snapshot(tx, now, quotas)
			if err != nil {
				return err
			}
			reply.Snapshots = append(reply.Snapshots, snapshot)
		}
		return nil
	})
	if err != nil {
		return nil, databaseError(err)
	}
	return reply, nil
}

func (s *Service) withGateway(ctx context.Context, handle func(*gorm.DB, string) error) error {
	tr, ok := transport.FromServerContext(ctx)
	if !ok {
		return unauthenticated()
	}
	scheme, token, _ := strings.Cut(tr.RequestHeader().Get("Authorization"), " ")
	if !strings.EqualFold(scheme, "Bearer") || len(token) > 256 {
		return unauthenticated()
	}
	if decoded, err := base64.RawURLEncoding.DecodeString(token); err != nil || len(decoded) < 32 {
		return unauthenticated()
	}
	hash := sha256.Sum256([]byte(token))
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row struct{ GatewayID, Status string }
		result := tx.Raw(`SELECT g.id AS gateway_id, g.status FROM gateway_tokens t JOIN gateways g ON g.id = t.gateway_id
			WHERE t.scope = 'auth' AND t.token_hash = ? AND t.revoked_at IS NULL FOR SHARE OF t, g`, hash[:]).Scan(&row)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return unauthenticated()
		}
		if row.Status != "active" {
			return errors.Forbidden("GATEWAY_FORBIDDEN", "gateway disabled")
		}
		return handle(tx, row.GatewayID)
	})
}

func unauthenticated() error {
	return errors.Unauthorized("GATEWAY_UNAUTHENTICATED", "valid gateway auth token required")
}

func (c storedCredential) snapshot(tx *gorm.DB, now time.Time, quotas map[string]*v1.AuthQuota) (*v1.AuthSnapshot, error) {
	result := &v1.AuthSnapshot{Reference: c.Reference, Username: c.Username, ClientId: c.ClientID,
		TenantId: c.TenantID, Active: c.Status == "active" && (c.ExpiresAt == nil || now.Before(*c.ExpiresAt))}
	if c.ExpiresAt != nil {
		result.ExpiresAt = timestamppb.New(*c.ExpiresAt)
	}
	if !result.Active {
		return result, nil
	}
	if err := json.Unmarshal([]byte(c.Cidrs), &result.AllowedSourceCidrs); err != nil {
		return nil, err
	}
	result.PasswordHash = c.SecretHash
	q := quotas[c.TenantID]
	if q == nil {
		var err error
		q, err = loadQuota(tx, c.TenantID, now)
		if err != nil {
			return nil, err
		}
		quotas[c.TenantID] = q
	}
	result.Quota = q
	return result, nil
}

func loadQuota(tx *gorm.DB, tenant string, now time.Time) (*v1.AuthQuota, error) {
	var row struct {
		AvailableBytes          int64
		AdmissionThresholdBytes uint64
		ValidUntil              *time.Time
	}
	err := tx.Raw(`WITH current_group AS (
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
	q := &v1.AuthQuota{AvailableBytes: row.AvailableBytes, AdmissionThresholdBytes: row.AdmissionThresholdBytes}
	if row.ValidUntil != nil {
		q.ValidUntil = timestamppb.New(*row.ValidUntil)
	}
	return q, nil
}
