package access

import (
	"crypto/hmac"
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/conf"
	"github.com/go-kratos/kratos/v3/errors"
)

const (
	passwordIterations = 600_000
	sessionDuration    = 8 * time.Hour
)

type Permission uint8

const (
	ReadTenantUsage Permission = iota + 1
	ManageProxyCredentials
	ReadOperations
	ExtractProxy
)

type Tenant struct {
	ID, Name string
}

type passwordHash struct {
	salt []byte
	key  []byte
}

var tenantIDPattern = regexp.MustCompile("^[A-Za-z0-9_-]{1,64}$")

type userAccess struct {
	passwordHash
	memberships map[string]conf.TenantRole
	operator    bool
}

type Service struct {
	tenants     map[string]string
	users       map[string]userAccess
	signingKey  [32]byte
	verifySlots chan struct{}
}

func New(config *conf.Access) (*Service, error) {
	service := &Service{users: make(map[string]userAccess), tenants: make(map[string]string), verifySlots: make(chan struct{}, 4)}
	if _, err := rand.Read(service.signingKey[:]); err != nil {
		return nil, fmt.Errorf("generate access signing key: %w", err)
	}
	for _, tenant := range config.GetTenants() {
		id := tenant.GetId()
		if !tenantIDPattern.MatchString(id) || strings.TrimSpace(tenant.GetName()) == "" {
			return nil, fmt.Errorf("invalid access tenant %q", id)
		}
		if _, exists := service.tenants[id]; exists {
			return nil, fmt.Errorf("duplicate access tenant %q", id)
		}
		service.tenants[id] = tenant.GetName()
	}
	for _, user := range config.GetUsers() {
		name := user.GetUsername()
		if name == "" || strings.Contains(name, ":") || len(name) > 128 {
			return nil, fmt.Errorf("invalid access username")
		}
		if _, exists := service.users[name]; exists {
			return nil, fmt.Errorf("duplicate access username %q", name)
		}
		saltText, keyText, _ := strings.Cut(user.GetPasswordHash(), ":")
		salt, saltErr := hex.DecodeString(saltText)
		key, keyErr := hex.DecodeString(keyText)
		if saltErr != nil || keyErr != nil || len(salt) != 16 || len(key) != sha256.Size {
			return nil, fmt.Errorf("invalid password hash for access user %q", name)
		}
		memberships := make(map[string]conf.TenantRole)
		for _, membership := range user.GetMemberships() {
			id, role := membership.GetTenantId(), membership.GetRole()
			if _, exists := service.tenants[id]; !exists {
				return nil, fmt.Errorf("unknown tenant %q for access user %q", id, name)
			}
			if !activeMembership(role) {
				return nil, fmt.Errorf("invalid role for access user %q in tenant %q", name, id)
			}
			if _, exists := memberships[id]; exists {
				return nil, fmt.Errorf("duplicate membership for access user %q in tenant %q", name, id)
			}
			if membership.GetDisabled() {
				role = conf.TenantRole_TENANT_ROLE_UNSPECIFIED
			}
			memberships[id] = role
		}
		service.users[name] = userAccess{passwordHash: passwordHash{salt: salt, key: key}, memberships: memberships, operator: user.GetPlatformOperator()}
	}
	return service, nil
}

func (s *Service) Login(username, password string) (string, int32, error) {
	invalid := errors.Unauthorized("INVALID_LOGIN", "invalid username or password")
	if password == "" || len(password) > 1024 || len(username) > 128 {
		return "", 0, invalid
	}
	select {
	case s.verifySlots <- struct{}{}:
		defer func() { <-s.verifySlots }()
	default:
		return "", 0, errors.New(429, "LOGIN_BUSY", "too many concurrent login requests")
	}
	user, exists := s.users[username]
	hash := user.passwordHash
	if !exists {
		hash = passwordHash{salt: s.signingKey[:16], key: s.signingKey[:]}
	}
	key, err := pbkdf2.Key(sha256.New, password, hash.salt, passwordIterations, sha256.Size)
	if err != nil || !hmac.Equal(key, hash.key) || !exists {
		return "", 0, invalid
	}

	expires := time.Now().Add(sessionDuration).Unix()
	payload := []byte(strconv.FormatInt(expires, 10) + ":" + username)
	token := base64.RawURLEncoding.EncodeToString(payload) + "." + s.sign(payload)
	return token, int32(sessionDuration.Seconds()), nil
}

func (s *Service) sign(payload []byte) string {
	mac := hmac.New(sha256.New, s.signingKey[:])
	mac.Write(payload)
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func (s *Service) ListTenants(token string) ([]Tenant, error) {
	username, err := s.identity(token)
	if err != nil {
		return nil, err
	}
	var tenants []Tenant
	for id, role := range s.users[username].memberships {
		if activeMembership(role) {
			tenants = append(tenants, Tenant{ID: id, Name: s.tenants[id]})
		}
	}
	sort.Slice(tenants, func(i, j int) bool {
		return tenants[i].ID < tenants[j].ID
	})
	return tenants, nil
}

func (s *Service) GetTenantAccess(token, id string) (Tenant, error) {
	if err := s.Require(token, id, ReadTenantUsage); err != nil {
		return Tenant{}, err
	}
	return Tenant{ID: id, Name: s.tenants[id]}, nil
}

func (s *Service) identity(token string) (string, error) {
	invalid := errors.Unauthorized("AUTHENTICATION_REQUIRED", "valid bearer token required")
	encodedPayload, signature, _ := strings.Cut(token, ".")
	payload, err := base64.RawURLEncoding.DecodeString(encodedPayload)
	if err != nil || !hmac.Equal([]byte(signature), []byte(s.sign(payload))) {
		return "", invalid
	}
	expiryText, username, _ := strings.Cut(string(payload), ":")
	expiry, err := strconv.ParseInt(expiryText, 10, 64)
	if _, exists := s.users[username]; err != nil || time.Now().Unix() >= expiry || !exists {
		return "", invalid
	}
	return username, nil
}

// Require checks the authenticated user against the exact tenant targeted by the
// operation. Callers pass the operation's bound tenant ID, or an empty ID for
// platform-wide operations, and keep the permission fixed by the operation.
func (s *Service) Require(token, tenantID string, permission Permission) error {
	_, err := s.RequireActor(token, tenantID, permission)
	return err
}

// RequireActor checks the permission and returns the authenticated username.
func (s *Service) RequireActor(token, tenantID string, permission Permission) (string, error) {
	username, err := s.identity(token)
	if err != nil {
		return "", err
	}
	user := s.users[username]
	role := user.memberships[tenantID]
	switch permission {
	case ReadTenantUsage:
		if activeMembership(role) {
			return username, nil
		}
	case ManageProxyCredentials, ExtractProxy:
		if role == conf.TenantRole_TENANT_ROLE_ADMIN {
			return username, nil
		}
	case ReadOperations:
		if tenantID == "" && user.operator {
			return username, nil
		}
	}
	return "", errors.Forbidden("ACCESS_DENIED", "permission denied")
}

func activeMembership(role conf.TenantRole) bool {
	return role == conf.TenantRole_TENANT_ROLE_MEMBER || role == conf.TenantRole_TENANT_ROLE_ADMIN
}
