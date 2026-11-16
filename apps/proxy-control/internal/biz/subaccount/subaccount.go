package subaccount

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"regexp"

	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/biz/access"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/conf"
	kerrors "github.com/go-kratos/kratos/v3/errors"
	"github.com/google/uuid"
)

const MaxAccounts = 10

type Account struct {
	ClientID, TenantID, Name, Remark string
	PasswordCiphertext               []byte
	Active                           bool
	LimitBytes                       *int64
	UsedBytes                        int64
}

type AuthState struct {
	Account
	Reference, Username string
	CredentialVersion   int64
	AvailableBytes      int64
	UsageVersion        int64
}

type Update struct {
	InvalidMask      bool
	Password, Remark *string
	Active           *bool
	UpdateLimit      bool
	LimitBytes       *int64
}

type Repository interface {
	List(context.Context, string) ([]Account, error)
	Find(context.Context, string, string) (Account, error)
	Create(context.Context, Account) error
	Update(context.Context, string, string, Update, []byte) (Account, error)
	Gateway(context.Context, []byte) (string, error)
	Load(context.Context, string) (AuthState, error)
	Bind(context.Context, string, string) (string, error)
	Refresh(context.Context, string, []string) ([]AuthState, error)
	Acquire(context.Context, string, SessionRequest) (StickySession, error)
	Commit(context.Context, string, SessionRequest) (StickySession, error)
	Release(context.Context, string, SessionRequest) error
}

var (
	ErrTooMany       = errors.New("maximum proxy subaccounts reached")
	ErrDuplicate     = errors.New("proxy subaccount name already exists")
	ErrLimitDecrease = errors.New("cumulative limit cannot decrease")
	nameFormat       = regexp.MustCompile(`^[a-z][a-z0-9]{3,7}$`)
	passwordFormat   = regexp.MustCompile(`^[A-Za-z0-9]{6,12}$`)
	letter           = regexp.MustCompile(`[A-Za-z]`)
	digit            = regexp.MustCompile(`[0-9]`)
)

type Service struct {
	repo            Repository
	access          *access.Service
	cipher          cipher.AEAD
	Host            string
	Port            uint32
	ExplicitSIDTime bool
}

func NewService(repo Repository, accessService *access.Service, cfg *conf.Biz) (*Service, error) {
	settings := cfg.GetSubaccounts()
	s := &Service{repo: repo, access: accessService, Host: settings.GetGatewayHost(), Port: settings.GetGatewayPort(), ExplicitSIDTime: settings.GetExplicitSidtime()}
	if settings == nil {
		return s, nil
	}
	key, err := base64.RawURLEncoding.DecodeString(settings.GetPasswordKey())
	if err != nil || len(key) != 32 || s.Host == "" || s.Port == 0 || s.Port > 65535 {
		return nil, errors.New("configure subaccounts with a 32-byte base64url key and valid public gateway host and port")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	s.cipher, err = cipher.NewGCMWithRandomNonce(block)
	return s, err
}

func unavailable() error {
	return kerrors.ServiceUnavailable("SUBACCOUNTS_UNAVAILABLE", "proxy subaccount service unavailable")
}

func missing() error {
	return kerrors.NotFound("SUBACCOUNT_NOT_FOUND", "proxy subaccount not found")
}

func (s *Service) encrypt(client, password string) ([]byte, error) {
	if !passwordFormat.MatchString(password) || !letter.MatchString(password) || !digit.MatchString(password) {
		return nil, kerrors.BadRequest("INVALID_PROXY_PASSWORD", "password must contain 6–12 ASCII letters and digits, including both")
	}
	return s.cipher.Seal(nil, nil, []byte(password), []byte(client)), nil
}

func (s *Service) decrypt(a Account) ([]byte, error) {
	plain, err := s.cipher.Open(nil, nil, a.PasswordCiphertext, []byte(a.ClientID))
	if err != nil {
		return nil, unavailable()
	}
	return plain, nil
}

func (s *Service) customer(ctx context.Context, token string) (access.Tenant, error) {
	tenant, err := s.access.Account(ctx, token)
	if err != nil {
		return access.Tenant{}, err
	}
	if s.cipher == nil || s.repo == nil {
		return access.Tenant{}, unavailable()
	}
	return tenant, nil
}

func (s *Service) List(ctx context.Context, token string) ([]Account, error) {
	tenant, err := s.customer(ctx, token)
	if err != nil {
		return nil, err
	}
	rows, err := s.repo.List(ctx, tenant.ID)
	if err != nil {
		return nil, unavailable()
	}
	return rows, nil
}

func (s *Service) CanCreate(ctx context.Context, token string) (bool, error) {
	tenant, err := s.access.Account(ctx, token)
	if err != nil {
		return false, err
	}
	if s.cipher == nil || s.repo == nil {
		return false, nil
	}
	rows, err := s.repo.List(ctx, tenant.ID)
	if err != nil {
		return false, unavailable()
	}
	return len(rows) < MaxAccounts, nil
}

func (s *Service) Create(ctx context.Context, token, name, password, remark string, active bool, limit *int64, validate func() error) (Account, error) {
	tenant, err := s.customer(ctx, token)
	if err != nil {
		return Account{}, err
	}
	if !nameFormat.MatchString(name) || len(remark) > 512 || limit != nil && *limit < 0 {
		return Account{}, kerrors.BadRequest("INVALID_SUBACCOUNT", "invalid name, remark or cumulative limit")
	}
	id, err := uuid.NewRandom()
	if err != nil {
		return Account{}, unavailable()
	}
	a := Account{ClientID: id.String(), TenantID: tenant.ID, Name: name, Remark: remark, Active: active, LimitBytes: limit}
	a.PasswordCiphertext, err = s.encrypt(a.ClientID, password)
	if err != nil {
		return Account{}, err
	}
	if validate() != nil {
		return Account{}, kerrors.BadRequest("INVALID_SUBACCOUNT", "invalid subaccount request")
	}
	switch err = s.repo.Create(ctx, a); {
	case errors.Is(err, ErrTooMany):
		return Account{}, kerrors.Conflict("SUBACCOUNT_LIMIT_REACHED", "each tenant may have at most 10 subaccounts")
	case errors.Is(err, ErrDuplicate):
		return Account{}, kerrors.Conflict("SUBACCOUNT_NAME_EXISTS", "subaccount name already exists")
	case err != nil:
		return Account{}, unavailable()
	}
	return a, nil
}

func (s *Service) Update(ctx context.Context, token, id string, update Update, validate func() error) (Account, error) {
	tenant, err := s.customer(ctx, token)
	if err != nil {
		return Account{}, err
	}
	if update.InvalidMask {
		return Account{}, kerrors.BadRequest("INVALID_UPDATE_MASK", "provide supported subaccount fields to update")
	}
	if update.Remark != nil && len(*update.Remark) > 512 || update.UpdateLimit && update.LimitBytes != nil && *update.LimitBytes < 0 {
		return Account{}, kerrors.BadRequest("INVALID_SUBACCOUNT", "invalid remark or cumulative limit")
	}
	var encrypted []byte
	if update.Password != nil {
		encrypted, err = s.encrypt(id, *update.Password)
		if err != nil {
			return Account{}, err
		}
	}
	if validate() != nil {
		return Account{}, kerrors.BadRequest("INVALID_SUBACCOUNT", "invalid subaccount request")
	}
	a, err := s.repo.Update(ctx, tenant.ID, id, update, encrypted)
	if errors.Is(err, ErrLimitDecrease) {
		return Account{}, kerrors.BadRequest("INVALID_SUBACCOUNT_LIMIT", "cumulative limit cannot decrease")
	}
	if err != nil {
		return Account{}, unavailable()
	}
	if a.ClientID == "" {
		return Account{}, missing()
	}
	return a, nil
}

func (s *Service) Credentials(ctx context.Context, token, id string, requireActive bool, validate func() error) (string, string, error) {
	tenant, err := s.customer(ctx, token)
	if err != nil {
		return "", "", err
	}
	if validate() != nil {
		return "", "", kerrors.BadRequest("INVALID_SUBACCOUNT", "invalid subaccount identifier")
	}
	a, err := s.repo.Find(ctx, tenant.ID, id)
	if err != nil {
		return "", "", unavailable()
	}
	if a.ClientID == "" || requireActive && !a.Active {
		return "", "", missing()
	}
	password, err := s.decrypt(a)
	return tenant.PrimaryAccountName + "." + a.Name, string(password), err
}

func (s *Service) gateway(ctx context.Context, token string) (string, error) {
	if s.cipher == nil || s.repo == nil {
		return "", unavailable()
	}
	if token == "" {
		return "", kerrors.Forbidden("GATEWAY_DENIED", "gateway authentication failed")
	}
	digest := sha256.Sum256([]byte(token))
	id, err := s.repo.Gateway(ctx, digest[:])
	if err != nil {
		return "", unavailable()
	}
	if id == "" {
		return "", kerrors.Forbidden("GATEWAY_DENIED", "gateway authentication failed")
	}
	return id, nil
}

func (s *Service) Load(ctx context.Context, token, username, password string, validate func() error) (AuthState, []byte, error) {
	gateway, err := s.gateway(ctx, token)
	if err != nil {
		return AuthState{}, nil, err
	}
	if validate() != nil {
		return AuthState{}, nil, kerrors.BadRequest("INVALID_AUTH_REQUEST", "invalid authentication request")
	}
	state, err := s.repo.Load(ctx, username)
	if err != nil {
		return AuthState{}, nil, unavailable()
	}
	denied := kerrors.NotFound("CREDENTIAL_DENIED", "proxy authentication failed")
	if state.ClientID == "" {
		return AuthState{}, nil, denied
	}
	hash, err := s.verifier(state)
	if err != nil {
		return AuthState{}, nil, err
	}
	supplied := sha256.Sum256([]byte(password))
	if subtle.ConstantTimeCompare(hash, supplied[:]) != 1 {
		return AuthState{}, nil, denied
	}
	if !state.Active {
		return AuthState{}, nil, kerrors.NotFound("CREDENTIAL_DISABLED", "proxy authentication failed")
	}
	state.Reference, err = s.repo.Bind(ctx, gateway, state.ClientID)
	if err != nil {
		return AuthState{}, nil, unavailable()
	}
	return state, hash, nil
}

func (s *Service) Refresh(ctx context.Context, token string, refs []string, validate func() error) ([]AuthState, error) {
	gateway, err := s.gateway(ctx, token)
	if err != nil {
		return nil, err
	}
	if validate() != nil {
		return nil, kerrors.BadRequest("INVALID_AUTH_REQUEST", "invalid authentication request")
	}
	rows, err := s.repo.Refresh(ctx, gateway, refs)
	if err != nil {
		return nil, unavailable()
	}
	return rows, nil
}

func (s *Service) Verifier(state AuthState) ([]byte, error) {
	if !state.Active {
		return nil, nil
	}
	return s.verifier(state)
}

func (s *Service) verifier(state AuthState) ([]byte, error) {
	plain, err := s.decrypt(state.Account)
	if err != nil {
		return nil, err
	}
	digest := sha256.Sum256(plain)
	return digest[:], nil
}
