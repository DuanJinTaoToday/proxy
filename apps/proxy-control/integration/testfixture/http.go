package testfixture

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/biz/access"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/conf"
)

func Request(handler http.Handler, method, path, body, token string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	reply := httptest.NewRecorder()
	handler.ServeHTTP(reply, req)
	return reply
}

const PasswordHash = "000102030405060708090a0b0c0d0e0f:cae9c801374596f17de48ba4ed7061692b5d0ab433932a7d3cdf698dfd8bb3e8"

func Login(t *testing.T, handler http.Handler, username string) string {
	t.Helper()
	body, err := json.Marshal(map[string]string{"username": username, "password": "test-password"})
	if err != nil {
		t.Fatal(err)
	}
	reply := Request(handler, "POST", "/v1/access/login", string(body), "")
	if reply.Code != http.StatusOK {
		t.Fatalf("login: %d %s", reply.Code, reply.Body)
	}
	var session struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(reply.Body.Bytes(), &session); err != nil || session.AccessToken == "" {
		t.Fatalf("invalid login response: %v", err)
	}
	return session.AccessToken
}

func AuthAccess(t *testing.T) *access.Service {
	t.Helper()
	users := []*conf.User{
		{Username: "alice", PasswordHash: PasswordHash, Memberships: []*conf.Membership{{TenantId: "alpha", Role: conf.TenantRole_TENANT_ROLE_ADMIN}}},
		{Username: "bob", PasswordHash: PasswordHash, Memberships: []*conf.Membership{{TenantId: "alpha", Role: conf.TenantRole_TENANT_ROLE_MEMBER}}},
	}
	accessService, err := access.New(&conf.Access{Tenants: []*conf.Tenant{{Id: "alpha", Name: "A"}, {Id: "beta", Name: "B"}}, Users: users})
	if err != nil {
		t.Fatal(err)
	}
	return accessService
}

func AssertStatus(t *testing.T, reply *httptest.ResponseRecorder, status int, reason string) {
	t.Helper()
	if reply.Code != status || !strings.Contains(reply.Body.String(), reason) {
		t.Fatalf("status = %d %s, want %d %s", reply.Code, reply.Body, status, reason)
	}
}
