package access

import (
	"context"
	"encoding/hex"
	stderrors "errors"
	"net/url"
	"strings"

	"github.com/go-kratos/kratos/v3/errors"
)

var (
	ErrInvalidInviteCode = stderrors.New("invalid invite code")
	ErrPromotionPaused   = stderrors.New("promotion paused")
	ErrTenantNotFound    = stderrors.New("tenant not found")
)

type RegistrationInvite struct {
	Code, Source string
}

func SelectRegistrationInvite(manual *string, linked string) RegistrationInvite {
	if manual != nil {
		return RegistrationInvite{Code: strings.ToLower(strings.TrimSpace(*manual)), Source: "manual"}
	}
	return RegistrationInvite{Code: strings.ToLower(strings.TrimSpace(linked)), Source: "link"}
}

func (i RegistrationInvite) Valid() bool {
	if len(i.Code) != 32 {
		return false
	}
	_, err := hex.DecodeString(i.Code)
	return err == nil
}

type TenantInvite struct {
	Code, URL string
	Paused    bool
}

func (s *Service) Invite(ctx context.Context, token string) (TenantInvite, error) {
	tenant, err := s.Account(ctx, token)
	if err != nil {
		return TenantInvite{}, err
	}
	if tenant.Email == "" {
		return TenantInvite{}, errors.Forbidden("EMAIL_NOT_VERIFIED", "verified email required")
	}
	if s.delivery == nil {
		return TenantInvite{}, accessUnavailable()
	}
	code, paused, err := s.repo.GetOrCreateInvite(ctx, tenant.ID)
	if err != nil {
		return TenantInvite{}, accessUnavailable()
	}
	link := *s.delivery.origin
	link.Path = "/zh-CN/register"
	link.RawQuery = url.Values{"invite": {code}}.Encode()
	return TenantInvite{Code: code, URL: link.String(), Paused: paused}, nil
}

func (s *Service) PromotionPaused(ctx context.Context, token, tenantID string) (bool, error) {
	if _, err := s.AuthorizeOperator(ctx, token, ReadOperations); err != nil {
		return false, err
	}
	if err := validatePromotionTenantID(tenantID); err != nil {
		return false, err
	}
	paused, err := s.repo.PromotionPaused(ctx, tenantID)
	return paused, promotionError(err)
}

func (s *Service) SetPromotionPaused(ctx context.Context, token, tenantID string, paused bool) error {
	actor, err := s.AuthorizeOperator(ctx, token, ManageTenantAccounts)
	if err != nil {
		return err
	}
	if err := validatePromotionTenantID(tenantID); err != nil {
		return err
	}
	return promotionError(s.repo.SetPromotionPaused(ctx, tenantID, actor, paused))
}

func validatePromotionTenantID(tenantID string) error {
	if len(tenantID) == 0 || len(tenantID) > 64 || strings.IndexByte(tenantID, 0) >= 0 {
		return errors.BadRequest("INVALID_TENANT_ID", "valid tenant id required")
	}
	return nil
}

func promotionError(err error) error {
	switch {
	case err == nil:
		return nil
	case stderrors.Is(err, ErrTenantNotFound):
		return errors.NotFound("TENANT_NOT_FOUND", "tenant not found")
	default:
		return accessUnavailable()
	}
}
