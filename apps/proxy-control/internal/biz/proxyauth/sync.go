package proxyauth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"strings"
	"time"

	"github.com/go-kratos/kratos/v3/errors"
)

func (s *Service) LoadAuthState(ctx context.Context, token, username, password string) (*Snapshot, error) {
	var snapshot *Snapshot
	err := s.withGateway(ctx, token, func(tx GatewayTx, gateway string) error {
		if username == "" || password == "" || len(username) > 255 || len(password) > 255 || strings.ContainsRune(username, 0) {
			return deniedCredential()
		}
		row, found, err := tx.FindCredential(username)
		if err != nil {
			return err
		}
		now := time.Now().UTC()
		hash := sha256.Sum256([]byte(password))
		if !found || subtle.ConstantTimeCompare(hash[:], row.SecretHash) != 1 {
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
		row.Reference, err = tx.Reference(gateway, row.ClientID, rand.Text())
		if err != nil {
			return err
		}
		snapshot = row.snapshot(now)
		return nil
	})
	if err != nil {
		return nil, databaseError(err)
	}
	if err := s.attachQuotas(ctx, []*Snapshot{snapshot}); err != nil {
		return nil, databaseError(err)
	}
	return snapshot, nil
}

func deniedCredential() error {
	return errors.NotFound("INVALID_CREDENTIAL", "credential unavailable")
}

func (s *Service) RefreshAuthState(ctx context.Context, token string, refs []string) ([]*Snapshot, error) {
	var snapshots []*Snapshot
	err := s.withGateway(ctx, token, func(tx GatewayTx, gateway string) error {
		if len(refs) == 0 || len(refs) > 100 {
			return errors.BadRequest("INVALID_AUTH_REFERENCES", "expected 1 to 100 credential references")
		}
		for _, ref := range refs {
			if ref == "" || len(ref) > 128 || strings.ContainsRune(ref, 0) {
				return errors.BadRequest("INVALID_AUTH_REFERENCES", "invalid credential reference")
			}
		}
		rows, err := tx.FindReferences(gateway, refs)
		if err != nil {
			return err
		}
		if len(rows) != len(refs) {
			return errors.BadRequest("INVALID_AUTH_REFERENCES", "one or more references were not issued to this gateway")
		}
		now := time.Now().UTC()
		for _, row := range rows {
			snapshots = append(snapshots, row.snapshot(now))
		}
		return nil
	})
	if err != nil {
		return nil, databaseError(err)
	}
	if err := s.attachQuotas(ctx, snapshots); err != nil {
		return nil, databaseError(err)
	}
	return snapshots, nil
}

func (s *Service) withGateway(ctx context.Context, token string, handle func(GatewayTx, string) error) error {
	if decoded, err := base64.RawURLEncoding.DecodeString(token); err != nil || len(decoded) < 32 {
		return unauthenticated()
	}
	hash := sha256.Sum256([]byte(token))
	return s.repo.WithGateway(ctx, hash[:], func(tx GatewayTx, gateway GatewayIdentity) error {
		if !gateway.Found {
			return unauthenticated()
		}
		if gateway.Status != "active" {
			return errors.Forbidden("GATEWAY_FORBIDDEN", "gateway disabled")
		}
		return handle(tx, gateway.ID)
	})
}

func unauthenticated() error {
	return errors.Unauthorized("GATEWAY_UNAUTHENTICATED", "valid gateway auth token required")
}

func (c Credential) snapshot(now time.Time) *Snapshot {
	result := &Snapshot{Reference: c.Reference, Username: c.Username, ClientID: c.ClientID,
		TenantID: c.TenantID, Active: c.Status == "active" && (c.ExpiresAt == nil || now.Before(*c.ExpiresAt))}
	if c.ExpiresAt != nil {
		result.ExpiresAt = c.ExpiresAt
	}
	if !result.Active {
		return result
	}
	result.AllowedSourceCIDRs = c.AllowedSourceCIDRs
	result.PasswordHash = c.SecretHash
	return result
}

func (s *Service) attachQuotas(ctx context.Context, snapshots []*Snapshot) error {
	quotas := make(map[string]*Quota)
	for _, snapshot := range snapshots {
		if !snapshot.Active {
			continue
		}
		quota := quotas[snapshot.TenantID]
		if quota == nil {
			var err error
			quota, err = s.quota.AdmissionQuota(ctx, snapshot.TenantID)
			if err != nil {
				return err
			}
			quotas[snapshot.TenantID] = quota
		}
		snapshot.Quota = quota
	}
	return nil
}
