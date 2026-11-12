package proxyauth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"log/slog"
	"net/netip"
	"strings"
	"time"

	v1 "github.com/DuanJinTaoToday/proxy/api/proxycontrol/v1"
	"github.com/go-kratos/kratos/v3/errors"
	"github.com/go-kratos/kratos/v3/transport"
	"gorm.io/gorm"
)

type TenantAccess interface {
	RequireActor(context.Context, string, v1.Permission) (string, error)
}

type Service struct {
	db     *gorm.DB
	access TenantAccess
}

func New(db *gorm.DB, access TenantAccess) *Service {
	return &Service{db: db, access: access}
}

func noStore(ctx context.Context) {
	if tr, ok := transport.FromServerContext(ctx); ok {
		tr.ReplyHeader().Set("Cache-Control", "no-store")
	}
}

func (s *Service) CreateCredential(ctx context.Context, req *v1.CreateCredentialRequest) (*v1.CreateCredentialResponse, error) {
	noStore(ctx)
	tenant := req.GetTenantId()
	actor, err := s.access.RequireActor(ctx, tenant, v1.Permission_PERMISSION_MANAGE_PROXY_CREDENTIALS)
	if err != nil {
		return nil, err
	}
	var expiry *time.Time
	if value := req.GetExpiresAt(); value != nil {
		if !value.IsValid() || !value.AsTime().After(time.Now()) {
			return nil, errors.BadRequest("INVALID_EXPIRY", "credential expiry must be in the future")
		}
		t := value.AsTime()
		expiry = &t
	}
	if len(req.GetAllowedSourceCidrs()) > 100 {
		return nil, errors.BadRequest("INVALID_SOURCE_CIDRS", "maximum of 100 CIDRs")
	}
	cidrs := make([]string, 0, len(req.GetAllowedSourceCidrs()))
	for _, source := range req.GetAllowedSourceCidrs() {
		prefix, err := netip.ParsePrefix(source)
		if err != nil || prefix.Addr().Is4In6() {
			return nil, errors.BadRequest("INVALID_SOURCE_CIDRS", "invalid source CIDR")
		}
		cidrs = append(cidrs, prefix.Masked().String())
	}
	for range 3 {
		id, username, password := rand.Text(), "px_"+rand.Text(), rand.Text()
		hash := sha256.Sum256([]byte(password))
		result := s.db.WithContext(ctx).Exec(`INSERT INTO proxy_credentials
			(client_id, tenant_id, username, secret_hash, expires_at, allowed_source_cidrs, created_by)
			VALUES (?, ?, ?, ?, ?, string_to_array(?, ',')::cidr[], ?) ON CONFLICT DO NOTHING`,
			id, tenant, username, hash[:], expiry, strings.Join(cidrs, ","), actor)
		if result.Error != nil {
			return nil, databaseError(result.Error)
		}
		if result.RowsAffected != 0 {
			return &v1.CreateCredentialResponse{ClientId: id, Username: username, Password: password}, nil
		}
	}
	return nil, errors.InternalServer("CREDENTIAL_GENERATION_FAILED", "credential creation failed")
}

func (s *Service) DisableCredential(ctx context.Context, req *v1.DisableCredentialRequest) (*v1.DisableCredentialResponse, error) {
	noStore(ctx)
	tenant := req.GetTenantId()
	if _, err := s.access.RequireActor(ctx, tenant, v1.Permission_PERMISSION_MANAGE_PROXY_CREDENTIALS); err != nil {
		return nil, err
	}
	result := s.db.WithContext(ctx).Exec(`UPDATE proxy_credentials SET status = 'disabled',
		disabled_at = COALESCE(disabled_at, now()) WHERE tenant_id = ? AND client_id = ?`, tenant, req.GetClientId())
	if result.Error != nil {
		return nil, databaseError(result.Error)
	}
	if result.RowsAffected == 0 {
		return nil, errors.NotFound("CREDENTIAL_NOT_FOUND", "credential not found")
	}
	return &v1.DisableCredentialResponse{}, nil
}

func databaseError(err error) error {
	if typed := errors.FromError(err); typed.Code < 500 {
		return err
	}
	// Log only the error type. Driver errors can contain input data.
	slog.Error("proxy authentication database operation failed", "type", fmt.Sprintf("%T", err))
	return errors.ServiceUnavailable("AUTH_STATE_UNAVAILABLE", "authentication state unavailable")
}
