package access

import (
	"context"
	"crypto/hmac"
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/conf"
	"github.com/go-kratos/kratos/v3/errors"
)

type Principal struct {
	ID, Username, PasswordHash, EmailDeliveryAddress string
	Version                                          int64
	Tenant                                           Tenant
	Permissions                                      map[Permission]bool
}

type Repository interface {
	StoreCredential(context.Context, EmailCredential) error
	AllowEmailRequest(context.Context, string, string) (bool, error)
	CreateTenant(context.Context, Tenant, string, []byte, RegistrationInvite) error
	GetOrCreateInvite(context.Context, string) (string, bool, error)
	PromotionPaused(context.Context, string) (bool, error)
	SetPromotionPaused(context.Context, string, string, bool) error
	RedeemLink(context.Context, string, []byte, string) error
	CustomerByEmail(context.Context, string) (Principal, error)
	CustomerByTenant(context.Context, string) (Principal, error)
	OperatorByName(context.Context, string) (Principal, error)
	OperatorByID(context.Context, string) (Principal, error)
	ResetPassword(context.Context, string, string, string) (bool, error)
}

const ManageTenantAccounts Permission = 6

func NewPersistent(repo Repository, sender MailSender, settings *conf.Biz, logger *slog.Logger) (*Service, error) {
	delivery, err := newEmailDelivery(sender, settings.GetEmailAuth(), logger)
	if err != nil {
		return nil, err
	}
	s := &Service{repo: repo, verifySlots: make(chan struct{}, 4), delivery: delivery}
	if _, err := rand.Read(s.signingKey[:]); err != nil {
		return nil, fmt.Errorf("generate access signing key: %w", err)
	}
	return s, nil
}

func (s *Service) passwordDigest(password string) (string, error) {
	if !utf8.ValidString(password) || utf8.RuneCountInString(password) < 12 || len(password) > 1024 {
		return "", errors.BadRequest("INVALID_PASSWORD", "password must have at least 12 characters and at most 1024 bytes")
	}
	select {
	case s.verifySlots <- struct{}{}:
		defer func() { <-s.verifySlots }()
	default:
		return "", errors.New(429, "LOGIN_BUSY", "too many concurrent password requests")
	}
	var salt [16]byte
	if _, err := rand.Read(salt[:]); err != nil {
		return "", accessUnavailable()
	}
	key, err := pbkdf2.Key(sha256.New, password, salt[:], passwordIterations, sha256.Size)
	if err != nil {
		return "", accessUnavailable()
	}
	return hex.EncodeToString(salt[:]) + ":" + hex.EncodeToString(key), nil
}

func accessUnavailable() error {
	return errors.ServiceUnavailable("ACCESS_UNAVAILABLE", "account service unavailable")
}

func (s *Service) LoginCustomer(ctx context.Context, email, password string) (string, int32, error) {
	if s.repo == nil {
		return "", 0, accessUnavailable()
	}
	return s.loginPrincipal(ctx, "customer", NormalizeEmail(email), password, s.repo.CustomerByEmail)
}

func (s *Service) LoginOperator(ctx context.Context, username, password string) (string, int32, error) {
	if s.repo == nil {
		return "", 0, accessUnavailable()
	}
	username = strings.TrimSpace(username)
	if len(username) > 128 {
		username = ""
	}
	return s.loginPrincipal(ctx, "operator", username, password, s.repo.OperatorByName)
}

func (s *Service) loginPrincipal(ctx context.Context, kind, login, password string, lookup func(context.Context, string) (Principal, error)) (string, int32, error) {
	invalid := errors.Unauthorized("INVALID_LOGIN", "invalid account or password")
	if s.repo == nil {
		return "", 0, accessUnavailable()
	}
	if login == "" || len(login) > 254 || password == "" || len(password) > 1024 {
		return "", 0, invalid
	}
	select {
	case s.verifySlots <- struct{}{}:
		defer func() { <-s.verifySlots }()
	default:
		return "", 0, errors.New(429, "LOGIN_BUSY", "too many concurrent login requests")
	}
	user, err := lookup(ctx, login)
	if err != nil {
		return "", 0, accessUnavailable()
	}
	salt, key := s.signingKey[:16], s.signingKey[:]
	if user.ID != "" {
		saltText, keyText, _ := strings.Cut(user.PasswordHash, ":")
		salt, err = hex.DecodeString(saltText)
		if err != nil || len(salt) != 16 {
			return "", 0, accessUnavailable()
		}
		key, err = hex.DecodeString(keyText)
		if err != nil || len(key) != sha256.Size {
			return "", 0, accessUnavailable()
		}
	}
	computed, err := pbkdf2.Key(sha256.New, password, salt, passwordIterations, sha256.Size)
	if err != nil || !hmac.Equal(key, computed) || user.ID == "" {
		return "", 0, invalid
	}
	expiry := time.Now().Add(sessionDuration).Unix()
	payload := []byte(fmt.Sprintf("%d:%s:%s:%d", expiry, kind, user.ID, user.Version))
	return base64.RawURLEncoding.EncodeToString(payload) + "." + s.sign(payload), int32(sessionDuration.Seconds()), nil
}

func (s *Service) principal(ctx context.Context, token string) (Principal, error) {
	invalid := errors.Unauthorized("AUTHENTICATION_REQUIRED", "valid bearer token required")
	encoded, signature, _ := strings.Cut(token, ".")
	payload, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil || !hmac.Equal([]byte(signature), []byte(s.sign(payload))) {
		return Principal{}, invalid
	}
	fields := strings.Split(string(payload), ":")
	if len(fields) != 4 {
		return Principal{}, invalid
	}
	expiry, err := strconv.ParseInt(fields[0], 10, 64)
	if err != nil || expiry <= time.Now().Unix() {
		return Principal{}, invalid
	}
	version, err := strconv.ParseInt(fields[3], 10, 64)
	if err != nil {
		return Principal{}, invalid
	}
	var user Principal
	switch fields[1] {
	case "customer":
		user, err = s.repo.CustomerByTenant(ctx, fields[2])
	case "operator":
		user, err = s.repo.OperatorByID(ctx, fields[2])
	default:
		return Principal{}, invalid
	}
	if err != nil {
		return Principal{}, accessUnavailable()
	}
	if user.ID == "" || user.Version != version {
		return Principal{}, invalid
	}
	return user, nil
}

func (s *Service) Account(ctx context.Context, token string) (Tenant, error) {
	if s.repo == nil {
		return Tenant{}, accessUnavailable()
	}
	user, err := s.principal(ctx, token)
	if err != nil {
		return Tenant{}, err
	}
	if user.Tenant.ID == "" {
		return Tenant{}, errors.Forbidden("ACCESS_DENIED", "permission denied")
	}
	return user.Tenant, nil
}

func (s *Service) OperatorAccount(ctx context.Context, token string) (string, []string, error) {
	if s.repo == nil {
		return "", nil, accessUnavailable()
	}
	user, err := s.principal(ctx, token)
	if err != nil {
		return "", nil, err
	}
	if user.Tenant.ID != "" {
		return "", nil, errors.Forbidden("ACCESS_DENIED", "permission denied")
	}
	var permissions []string
	for _, pair := range []struct {
		permission Permission
		name       string
	}{
		{ReadOperations, "read_operations"},
		{ManageBilling, "manage_billing"},
		{ManageTenantAccounts, "manage_tenant_accounts"},
	} {
		if user.Permissions[pair.permission] {
			permissions = append(permissions, pair.name)
		}
	}
	return user.Username, permissions, nil
}

func (s *Service) ResetTenantPassword(ctx context.Context, token, tenant, password string) error {
	if s.repo == nil {
		return accessUnavailable()
	}
	user, err := s.principal(ctx, token)
	if err != nil {
		return err
	}
	if !user.Permissions[ManageTenantAccounts] {
		return errors.Forbidden("ACCESS_DENIED", "permission denied")
	}
	digest, err := s.passwordDigest(password)
	if err != nil {
		return err
	}
	updated, err := s.repo.ResetPassword(ctx, tenant, digest, user.ID)
	if err != nil {
		return accessUnavailable()
	}
	if !updated {
		return errors.NotFound("TENANT_NOT_FOUND", "tenant not found")
	}
	return nil
}

func (s *Service) requirePersistent(token, tenant string, permission Permission) (string, error) {
	if s.repo == nil {
		return "", errors.Unauthorized("AUTHENTICATION_REQUIRED", "valid bearer token required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	user, err := s.principal(ctx, token)
	if err != nil {
		return "", err
	}
	switch permission {
	case ReadTenantUsage:
		if user.Tenant.ID != "" && user.Tenant.ID == tenant {
			return user.ID, nil
		}
	case ReadOperations:
		if tenant == "" && user.Permissions[permission] {
			return user.ID, nil
		}
	case ManageBilling, ManageTenantAccounts:
		if user.Permissions[permission] {
			if tenant == "" {
				return user.ID, nil
			}
			target, err := s.repo.CustomerByTenant(ctx, tenant)
			if err != nil {
				return "", accessUnavailable()
			}
			if target.ID != "" {
				return user.ID, nil
			}
		}
	}
	return "", errors.Forbidden("ACCESS_DENIED", "permission denied")
}

// AuthorizeOperator returns the authenticated actor when at least one requested permission is granted.
func (s *Service) AuthorizeOperator(ctx context.Context, token string, permissions ...Permission) (string, error) {
	if s.repo == nil {
		return "", accessUnavailable()
	}
	user, err := s.principal(ctx, token)
	if err != nil {
		return "", err
	}
	for _, permission := range permissions {
		if user.Permissions[permission] {
			return user.ID, nil
		}
	}
	return "", errors.Forbidden("ACCESS_DENIED", "permission denied")
}
