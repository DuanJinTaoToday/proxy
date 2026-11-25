package access

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"math/big"
	"net/mail"
	"net/url"
	"strings"
	"time"

	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/conf"
	apierrors "github.com/go-kratos/kratos/v3/errors"
)

const (
	RegistrationPurpose  = "register"
	PasswordResetPurpose = "reset"
	RecoveryPurpose      = "recover"
)

var (
	ErrInvalidEmailCredential = errors.New("invalid email credential")
	ErrEmailOccupied          = errors.New("email already claimed")
)

type MailSender interface {
	Deliver(context.Context, string, string, string) error
}

type EmailCredential struct {
	Purpose, Subject, Email, DeliveryAddress string
	TenantID, OperatorID                     string
	SessionVersion                           int64
	SecretHash                               []byte
	Lifetime                                 time.Duration
}

type emailDelivery struct {
	sender  MailSender
	origin  *url.URL
	codeKey []byte
	slots   chan struct{}
	logger  *slog.Logger
}

func newEmailDelivery(sender MailSender, config *conf.EmailAuth, logger *slog.Logger) (*emailDelivery, error) {
	if config == nil {
		return nil, nil
	}
	if sender == nil {
		return nil, fmt.Errorf("email auth requires an email sender")
	}
	address, err := url.Parse(config.GetConsoleOrigin())
	if err != nil || address.Scheme != "https" || address.Hostname() == "" ||
		address.User != nil || address.RawQuery != "" || address.ForceQuery || address.Fragment != "" ||
		(address.Path != "" && address.Path != "/") {
		return nil, fmt.Errorf("configure biz.email_auth.console_origin as a bare HTTPS origin")
	}
	key, err := base64.RawURLEncoding.DecodeString(config.GetCodeKey())
	if err != nil || len(key) != 32 {
		return nil, fmt.Errorf("configure biz.email_auth.code_key as 32 bytes encoded in raw base64url")
	}
	return &emailDelivery{sender: sender, origin: address, codeKey: key, slots: make(chan struct{}, 32), logger: logger}, nil
}

func NormalizeEmail(value string) string {
	address := strings.TrimSpace(value)
	if len(address) < 3 || len(address) > 254 || strings.ContainsAny(address, "\r\n\t \x00") {
		return ""
	}
	parsed, err := mail.ParseAddress(address)
	local, domain, found := strings.Cut(address, "@")
	if err != nil || parsed.Address != address || !found || len(local) > 64 || domain == "" {
		return ""
	}
	normalized := strings.ToLower(address)
	if len(normalized) > 254 {
		return ""
	}
	return normalized
}

func invalidEmail() error {
	return apierrors.BadRequest("INVALID_EMAIL", "valid email address required")
}

func occupiedEmail() error {
	return apierrors.Conflict("EMAIL_ALREADY_REGISTERED", "email is already registered")
}

func (s *Service) codeHash(email, code string) []byte {
	mac := hmac.New(sha256.New, s.delivery.codeKey)
	mac.Write([]byte(email + "\x00" + code))
	return mac.Sum(nil)
}

func (s *Service) Register(ctx context.Context, email, password, code string) (Tenant, error) {
	email = NormalizeEmail(email)
	if email == "" {
		return Tenant{}, invalidEmail()
	}
	if s.repo == nil || s.delivery == nil {
		return Tenant{}, accessUnavailable()
	}
	digest, err := s.passwordDigest(password)
	if err != nil {
		return Tenant{}, err
	}
	var id [16]byte
	if _, err := rand.Read(id[:]); err != nil {
		return Tenant{}, accessUnavailable()
	}
	name := "p" + hex.EncodeToString(id[:])
	tenant := Tenant{ID: hex.EncodeToString(id[:]), Name: name, PrimaryAccountName: name, Email: email}
	err = s.repo.CreateTenant(ctx, tenant, digest, s.codeHash(email, strings.TrimSpace(code)))
	switch {
	case err == nil:
		return tenant, nil
	case errors.Is(err, ErrInvalidEmailCredential):
		return Tenant{}, apierrors.BadRequest("INVALID_VERIFICATION_CODE", "verification code is invalid or expired")
	case errors.Is(err, ErrEmailOccupied):
		return Tenant{}, occupiedEmail()
	default:
		s.delivery.logger.Warn("register account failed")
		return Tenant{}, accessUnavailable()
	}
}

func (s *Service) RequestEmail(ctx context.Context, address, source, purpose string) error {
	email := NormalizeEmail(address)
	if email == "" {
		return invalidEmail()
	}
	if s.repo == nil || s.delivery == nil {
		return nil
	}
	allowed, err := s.repo.AllowEmailRequest(ctx, email, source)
	if err != nil {
		s.delivery.logger.Warn("check public email request limit failed")
	}
	if err != nil || !allowed {
		return nil
	}
	customer, err := s.repo.CustomerByEmail(ctx, email)
	if err != nil {
		s.delivery.logger.Warn("look up public email request failed")
		return nil
	}
	switch purpose {
	case RegistrationPurpose:
		if customer.ID != "" {
			return nil
		}
		raw, err := rand.Int(rand.Reader, big.NewInt(1_000_000))
		if err != nil {
			s.delivery.logger.Warn("generate registration code failed")
			return nil
		}
		code := fmt.Sprintf("%06d", raw.Int64())
		address = strings.TrimSpace(address)
		credential := EmailCredential{Purpose: purpose, Subject: email, Email: email, DeliveryAddress: address,
			SecretHash: s.codeHash(email, code), Lifetime: 10 * time.Minute}
		if err := s.repo.StoreCredential(ctx, credential); err != nil {
			s.delivery.logger.Warn("store registration code failed")
			return nil
		}
		s.delivery.sendPublic(address, "RocProxy verification code", "Your RocProxy verification code is "+code+". It expires in 10 minutes.")
	case PasswordResetPurpose:
		if customer.ID == "" {
			return nil
		}
		credential := EmailCredential{Purpose: purpose, Subject: email, Email: email, DeliveryAddress: customer.EmailDeliveryAddress,
			TenantID: customer.ID, SessionVersion: customer.Version}
		link, err := s.issueLink(ctx, credential, "/zh-CN/reset-password")
		if err != nil {
			s.delivery.logger.Warn("store password reset link failed")
			return nil
		}
		s.delivery.sendPublic(credential.DeliveryAddress, "RocProxy password reset", linkMessage("Reset your RocProxy password", link))
	}
	return nil
}

func (d *emailDelivery) sendPublic(address, subject, body string) {
	select {
	case d.slots <- struct{}{}:
		go func() {
			defer func() { <-d.slots }()
			if err := d.send(context.Background(), address, subject, body); err != nil {
				d.logger.Warn("deliver public account email failed")
			}
		}()
	default:
		d.logger.Warn("public account email delivery capacity reached")
	}
}

func (d *emailDelivery) send(ctx context.Context, address, subject, body string) error {
	sendContext, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	return d.sender.Deliver(sendContext, address, subject, body)
}

func linkMessage(action, link string) string {
	return action + " using this one-time link. It expires in 30 minutes.\r\n\r\n" + link
}

func (s *Service) issueLink(ctx context.Context, credential EmailCredential, page string) (string, error) {
	var secret [32]byte
	if _, err := rand.Read(secret[:]); err != nil {
		return "", err
	}
	hash := sha256.Sum256(secret[:])
	credential.SecretHash = hash[:]
	credential.Lifetime = 30 * time.Minute
	if err := s.repo.StoreCredential(ctx, credential); err != nil {
		return "", err
	}
	link := *s.delivery.origin
	link.Path = page
	link.RawQuery = url.Values{"token": {base64.RawURLEncoding.EncodeToString(secret[:])}}.Encode()
	return link.String(), nil
}

func (s *Service) StartEmailRecovery(ctx context.Context, token, tenantID, address string) error {
	actor, err := s.AuthorizeOperator(ctx, token, ManageTenantAccounts)
	if err != nil {
		return err
	}
	email := NormalizeEmail(address)
	if email == "" {
		return invalidEmail()
	}
	customer, err := s.repo.CustomerByTenant(ctx, tenantID)
	if err != nil {
		return accessUnavailable()
	}
	if customer.ID == "" {
		return apierrors.NotFound("TENANT_NOT_FOUND", "tenant not found")
	}
	owner, err := s.repo.CustomerByEmail(ctx, email)
	if err != nil {
		return accessUnavailable()
	}
	if owner.ID != "" {
		return occupiedEmail()
	}
	if s.delivery == nil {
		return accessUnavailable()
	}
	address = strings.TrimSpace(address)
	credential := EmailCredential{Purpose: RecoveryPurpose, Subject: tenantID, Email: email, DeliveryAddress: address,
		TenantID: tenantID, SessionVersion: customer.Version, OperatorID: actor}
	link, err := s.issueLink(ctx, credential, "/zh-CN/recover-email")
	if errors.Is(err, ErrEmailOccupied) {
		return occupiedEmail()
	}
	if err != nil {
		return accessUnavailable()
	}
	if err := s.delivery.send(ctx, address, "RocProxy account recovery", linkMessage("Recover your RocProxy account", link)); err != nil {
		s.delivery.logger.Warn("deliver account recovery email failed")
		return accessUnavailable()
	}
	return nil
}

func (s *Service) RedeemEmailLink(ctx context.Context, purpose, token, password string) error {
	if s.repo == nil {
		return accessUnavailable()
	}
	invalid := apierrors.BadRequest("INVALID_ACCESS_TOKEN", "email link is invalid or expired")
	secret, err := base64.RawURLEncoding.Strict().DecodeString(token)
	if err != nil || len(secret) != 32 {
		return invalid
	}
	digest, err := s.passwordDigest(password)
	if err != nil {
		return err
	}
	hash := sha256.Sum256(secret)
	err = s.repo.RedeemLink(ctx, purpose, hash[:], digest)
	switch {
	case err == nil:
		return nil
	case errors.Is(err, ErrInvalidEmailCredential):
		return invalid
	case errors.Is(err, ErrEmailOccupied):
		return occupiedEmail()
	default:
		slog.Warn("redeem email link failed")
		return accessUnavailable()
	}
}
