package proxyauth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"log/slog"
	"net/netip"
	"time"

	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/biz/access"
	"github.com/go-kratos/kratos/v3/errors"
)

type TenantAccess interface {
	RequireActor(string, string, access.Permission) (string, error)
}

type Service struct {
	repo   Repository
	access TenantAccess
}

func New(repo Repository, access TenantAccess) *Service {
	return &Service{repo: repo, access: access}
}

func (s *Service) CreateCredential(ctx context.Context, token, tenant string, expiry *time.Time, invalidExpiry bool, sources []string) (*IssuedCredential, error) {
	actor, err := s.access.RequireActor(token, tenant, access.ManageProxyCredentials)
	if err != nil {
		return nil, err
	}
	if invalidExpiry || (expiry != nil && !expiry.After(time.Now())) {
		return nil, errors.BadRequest("INVALID_EXPIRY", "credential expiry must be in the future")
	}
	if len(sources) > 100 {
		return nil, errors.BadRequest("INVALID_SOURCE_CIDRS", "maximum of 100 CIDRs")
	}
	cidrs := make([]string, 0, len(sources))
	for _, source := range sources {
		prefix, err := netip.ParsePrefix(source)
		if err != nil || prefix.Addr().Is4In6() {
			return nil, errors.BadRequest("INVALID_SOURCE_CIDRS", "invalid source CIDR")
		}
		cidrs = append(cidrs, prefix.Masked().String())
	}
	for range 3 {
		id, username, password := rand.Text(), "px_"+rand.Text(), rand.Text()
		hash := sha256.Sum256([]byte(password))
		inserted, err := s.repo.InsertCredential(ctx, id, tenant, username, hash[:], expiry, cidrs, actor)
		if err != nil {
			return nil, databaseError(err)
		}
		if inserted {
			return &IssuedCredential{ClientID: id, Username: username, Password: password}, nil
		}
	}
	return nil, errors.InternalServer("CREDENTIAL_GENERATION_FAILED", "credential creation failed")
}

func (s *Service) DisableCredential(ctx context.Context, token, tenant, clientID string) error {
	if _, err := s.access.RequireActor(token, tenant, access.ManageProxyCredentials); err != nil {
		return err
	}
	disabled, err := s.repo.DisableCredential(ctx, tenant, clientID)
	if err != nil {
		return databaseError(err)
	}
	if !disabled {
		return errors.NotFound("CREDENTIAL_NOT_FOUND", "credential not found")
	}
	return nil
}

func databaseError(err error) error {
	if typed := errors.FromError(err); typed.Code < 500 {
		return err
	}
	// Log only the error type. Driver errors can contain input data.
	slog.Error("proxy authentication database operation failed", "type", fmt.Sprintf("%T", err))
	return errors.ServiceUnavailable("AUTH_STATE_UNAVAILABLE", "authentication state unavailable")
}
