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
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	v1 "github.com/DuanJinTaoToday/proxy/api/proxycontrol/v1"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/conf"
	"github.com/go-kratos/kratos/v3/errors"
	"github.com/go-kratos/kratos/v3/transport"
)

const (
	passwordIterations = 600_000
	sessionDuration    = 8 * time.Hour
)

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
		env := user.GetPasswordHashEnv()
		saltText, keyText, _ := strings.Cut(os.Getenv(env), ":")
		salt, saltErr := hex.DecodeString(saltText)
		key, keyErr := hex.DecodeString(keyText)
		if saltErr != nil || keyErr != nil || len(salt) != 16 || len(key) != sha256.Size {
			return nil, fmt.Errorf("invalid password hash for access user %q in environment variable %q", name, env)
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

func (s *Service) Login(ctx context.Context, req *v1.LoginRequest) (*v1.LoginResponse, error) {
	if tr, ok := transport.FromServerContext(ctx); ok {
		tr.ReplyHeader().Set("Cache-Control", "no-store")
	}

	invalid := errors.Unauthorized("INVALID_LOGIN", "invalid username or password")
	if req.GetPassword() == "" || len(req.GetPassword()) > 1024 || len(req.GetUsername()) > 128 {
		return nil, invalid
	}
	select {
	case s.verifySlots <- struct{}{}:
		defer func() { <-s.verifySlots }()
	default:
		return nil, errors.New(429, "LOGIN_BUSY", "too many concurrent login requests")
	}
	user, exists := s.users[req.GetUsername()]
	hash := user.passwordHash
	if !exists {
		hash = passwordHash{salt: s.signingKey[:16], key: s.signingKey[:]}
	}
	key, err := pbkdf2.Key(sha256.New, req.GetPassword(), hash.salt, passwordIterations, sha256.Size)
	if err != nil || !hmac.Equal(key, hash.key) || !exists {
		return nil, invalid
	}

	expires := time.Now().Add(sessionDuration).Unix()
	payload := []byte(strconv.FormatInt(expires, 10) + ":" + req.GetUsername())
	token := base64.RawURLEncoding.EncodeToString(payload) + "." + s.sign(payload)
	return &v1.LoginResponse{AccessToken: token, ExpiresInSeconds: int32(sessionDuration.Seconds())}, nil
}

func (s *Service) sign(payload []byte) string {
	mac := hmac.New(sha256.New, s.signingKey[:])
	mac.Write(payload)
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func (s *Service) ListTenants(ctx context.Context, _ *v1.ListTenantsRequest) (*v1.ListTenantsResponse, error) {
	username, err := s.identity(ctx)
	if err != nil {
		return nil, err
	}
	reply := &v1.ListTenantsResponse{}
	for id, role := range s.users[username].memberships {
		if activeMembership(role) {
			reply.Tenants = append(reply.Tenants, &v1.TenantAccess{TenantId: id, Name: s.tenants[id]})
		}
	}
	sort.Slice(reply.Tenants, func(i, j int) bool {
		return reply.Tenants[i].GetTenantId() < reply.Tenants[j].GetTenantId()
	})
	return reply, nil
}

func (s *Service) GetTenantAccess(ctx context.Context, req *v1.GetTenantAccessRequest) (*v1.TenantAccess, error) {
	id := req.GetTenantId()
	if err := s.Require(ctx, id, v1.Permission_PERMISSION_READ_TENANT_USAGE); err != nil {
		return nil, err
	}
	return &v1.TenantAccess{TenantId: id, Name: s.tenants[id]}, nil
}

func (s *Service) identity(ctx context.Context) (string, error) {
	invalid := errors.Unauthorized("AUTHENTICATION_REQUIRED", "valid bearer token required")
	tr, ok := transport.FromServerContext(ctx)
	if !ok {
		return "", invalid
	}
	tr.ReplyHeader().Set("Cache-Control", "no-store")
	scheme, token, _ := strings.Cut(tr.RequestHeader().Get("Authorization"), " ")
	if !strings.EqualFold(scheme, "Bearer") || len(token) > 256 {
		return "", invalid
	}
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

func (s *Service) CheckPermission(ctx context.Context, req *v1.CheckPermissionRequest) (*v1.CheckPermissionResponse, error) {
	if err := s.Require(ctx, req.GetTenantId(), req.GetPermission()); err != nil {
		return nil, err
	}
	return &v1.CheckPermissionResponse{}, nil
}

// Require checks the authenticated user against the exact tenant targeted by the
// operation. Callers pass the operation's bound tenant ID, or an empty ID for
// platform-wide operations, and keep the permission fixed by the operation.
func (s *Service) Require(ctx context.Context, tenantID string, permission v1.Permission) error {
	username, err := s.identity(ctx)
	if err != nil {
		return err
	}
	user := s.users[username]
	role := user.memberships[tenantID]
	switch permission {
	case v1.Permission_PERMISSION_READ_TENANT_USAGE:
		if activeMembership(role) {
			return nil
		}
	case v1.Permission_PERMISSION_MANAGE_PROXY_CREDENTIALS:
		if role == conf.TenantRole_TENANT_ROLE_ADMIN {
			return nil
		}
	case v1.Permission_PERMISSION_READ_OPERATIONS:
		if tenantID == "" && user.operator {
			return nil
		}
	}
	return errors.Forbidden("ACCESS_DENIED", "permission denied")
}

func activeMembership(role conf.TenantRole) bool {
	return role == conf.TenantRole_TENANT_ROLE_MEMBER || role == conf.TenantRole_TENANT_ROLE_ADMIN
}
