package server_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/biz/access"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/conf"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/server"
	httpservice "github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/service"
)

func request(handler http.Handler, method, path, body, token string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	reply := httptest.NewRecorder()
	handler.ServeHTTP(reply, req)
	return reply
}

func TestLogin(t *testing.T) {
	handler := testServer(t)
	reply := request(handler, "POST", "/v1/access/login", `{"username":"alice","password":"test-password"}`, "")
	if reply.Code != http.StatusOK {
		t.Fatalf("login: %d %s", reply.Code, reply.Body)
	}
}

const fixtureHash = "000102030405060708090a0b0c0d0e0f:cae9c801374596f17de48ba4ed7061692b5d0ab433932a7d3cdf698dfd8bb3e8"

func testServer(t *testing.T) http.Handler {
	t.Helper()
	// PBKDF2-SHA256 of test-password with the fixture salt and 600,000 iterations.
	t.Setenv("TEST_ALICE_PASSWORD_HASH", fixtureHash)
	service, err := access.New(&conf.Access{
		Tenants: []*conf.Tenant{{Id: "alpha", Name: "Alpha"}, {Id: "beta", Name: "Beta"}},
		Users: []*conf.User{
			{Username: "alice", PasswordHashEnv: "TEST_ALICE_PASSWORD_HASH", Memberships: []*conf.Membership{
				{TenantId: "alpha", Role: conf.TenantRole_TENANT_ROLE_ADMIN},
				{TenantId: "beta", Role: conf.TenantRole_TENANT_ROLE_MEMBER},
			}},
			{Username: "bob", PasswordHashEnv: "TEST_ALICE_PASSWORD_HASH", Memberships: []*conf.Membership{
				{TenantId: "alpha", Role: conf.TenantRole_TENANT_ROLE_MEMBER},
				{TenantId: "beta", Role: conf.TenantRole_TENANT_ROLE_ADMIN, Disabled: true},
			}},
			{Username: "operator", PasswordHashEnv: "TEST_ALICE_PASSWORD_HASH", PlatformOperator: true},
			{Username: "carol", PasswordHashEnv: "TEST_ALICE_PASSWORD_HASH", Memberships: []*conf.Membership{
				{TenantId: "alpha", Role: conf.TenantRole_TENANT_ROLE_MEMBER},
			}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return server.NewHTTPServer("127.0.0.1:0", httpservice.NewAccess(service))
}

func TestLoginRejectsInvalidCredentials(t *testing.T) {
	handler := testServer(t)
	for _, body := range []string{
		`{"username":"alice","password":"wrong"}`,
		`{"username":"unknown","password":"test-password"}`,
	} {
		reply := request(handler, "POST", "/v1/access/login", body, "")
		if reply.Code != http.StatusUnauthorized {
			t.Fatalf("invalid login: %d %s", reply.Code, reply.Body)
		}
	}
}

func TestTenantListRequiresLogin(t *testing.T) {
	handler := testServer(t)
	for _, token := range []string{"", "invalid-token"} {
		reply := request(handler, "GET", "/v1/access/tenants", "", token)
		if reply.Code != http.StatusUnauthorized {
			t.Fatalf("unauthenticated list: %d %s", reply.Code, reply.Body)
		}
	}
}

func login(t *testing.T, handler http.Handler, username string) string {
	t.Helper()
	body, err := json.Marshal(map[string]string{"username": username, "password": "test-password"})
	if err != nil {
		t.Fatal(err)
	}
	reply := request(handler, "POST", "/v1/access/login", string(body), "")
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

func TestTenantSelectionUsesActiveMemberships(t *testing.T) {
	handler := testServer(t)
	for _, tc := range []struct {
		user    string
		list    []string
		statusB int
	}{
		{"alice", []string{"alpha", "beta"}, http.StatusOK},
		{"bob", []string{"alpha"}, http.StatusForbidden},
		{"carol", []string{"alpha"}, http.StatusForbidden},
	} {
		t.Run(tc.user, func(t *testing.T) {
			token := login(t, handler, tc.user)
			reply := request(handler, "GET", "/v1/access/tenants", "", token)
			var list struct {
				Tenants []struct {
					ID string `json:"tenant_id"`
				} `json:"tenants"`
			}
			if err := json.Unmarshal(reply.Body.Bytes(), &list); err != nil || reply.Code != http.StatusOK || len(list.Tenants) != len(tc.list) {
				t.Fatalf("tenant list: %d %s", reply.Code, reply.Body)
			}
			for i, tenant := range list.Tenants {
				if tenant.ID != tc.list[i] {
					t.Fatalf("tenant list: %s", reply.Body)
				}
			}
			for path, want := range map[string]int{
				"/v1/access/tenants/alpha":                http.StatusOK,
				"/v1/access/tenants/beta?tenant_id=alpha": tc.statusB,
				"/v1/access/tenants/alpha?tenantId=beta":  http.StatusOK,
			} {
				reply = request(handler, "GET", path, "", token)
				if reply.Code != want {
					t.Errorf("%s: got %d, want %d (%s)", path, reply.Code, want, reply.Body)
				}
				if want == http.StatusOK {
					var selected struct {
						ID string `json:"tenant_id"`
					}
					if err := json.Unmarshal(reply.Body.Bytes(), &selected); err != nil || selected.ID != strings.SplitN(strings.TrimPrefix(path, "/v1/access/tenants/"), "?", 2)[0] {
						t.Errorf("invalid selected tenant: %s", reply.Body)
					}
				}
			}
		})
	}
}

func TestPermissionsAreScopedToTenantAndRole(t *testing.T) {
	handler := testServer(t)
	alice := login(t, handler, "alice")
	bob := login(t, handler, "bob")
	operator := login(t, handler, "operator")
	for _, tc := range []struct {
		name, token, query, body string
		want                     int
	}{
		{"member reads usage", bob, "", `{"tenant_id":"alpha","permission":1}`, http.StatusOK},
		{"admin reads usage", alice, "", `{"tenant_id":"alpha","permission":1}`, http.StatusOK},
		{"member cannot manage credentials", bob, "", `{"tenant_id":"alpha","permission":2}`, http.StatusForbidden},
		{"admin manages credentials in own tenant", alice, "", `{"tenant_id":"alpha","permission":2}`, http.StatusOK},
		{"admin role does not carry across tenants", alice, "", `{"tenant_id":"beta","permission":2}`, http.StatusForbidden},
		{"disabled admin cannot read usage", bob, "", `{"tenant_id":"beta","permission":1}`, http.StatusForbidden},
		{"query cannot replace body scope", bob, "?tenant_id=alpha", `{"tenant_id":"beta","permission":1}`, http.StatusForbidden},
		{"query alone cannot grant tenant access", bob, "?tenant_id=alpha", `{"permission":1}`, http.StatusForbidden},
		{"request body cannot impersonate a member", bob, "", `{"tenant_id":"beta","permission":1,"username":"alice"}`, http.StatusForbidden},
		{"tenant admin cannot read operations", alice, "", `{"permission":3}`, http.StatusForbidden},
		{"operator reads operations", operator, "", `{"permission":3}`, http.StatusOK},
		{"operator is not automatically a tenant member", operator, "", `{"tenant_id":"alpha","permission":1}`, http.StatusForbidden},
		{"unspecified permission fails closed", alice, "", `{"tenant_id":"alpha"}`, http.StatusForbidden},
		{"anonymous cannot read usage", "", "", `{"tenant_id":"alpha","permission":1}`, http.StatusUnauthorized},
	} {
		t.Run(tc.name, func(t *testing.T) {
			reply := request(handler, "POST", "/v1/access/check"+tc.query, tc.body, tc.token)
			if reply.Code != tc.want {
				t.Fatalf("got %d, want %d (%s)", reply.Code, tc.want, reply.Body)
			}
		})
	}
}
