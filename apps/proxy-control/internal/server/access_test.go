package server_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/integration/testfixture"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/biz/access"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/conf"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/server"
	httpservice "github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/service"
)

func TestLogin(t *testing.T) {
	handler := testServer(t)
	reply := testfixture.Request(handler, "POST", "/v1/access/login", `{"username":"alice","password":"test-password"}`, "")
	if reply.Code != http.StatusOK {
		t.Fatalf("login: %d %s", reply.Code, reply.Body)
	}
}

func testServer(t *testing.T) http.Handler {
	t.Helper()
	// PBKDF2-SHA256 of test-password with the fixture salt and 600,000 iterations.
	service, err := access.New(&conf.Access{
		Tenants: []*conf.Tenant{{Id: "alpha", Name: "Alpha"}, {Id: "beta", Name: "Beta"}},
		Users: []*conf.User{
			{Username: "alice", PasswordHash: testfixture.PasswordHash, Memberships: []*conf.Membership{
				{TenantId: "alpha", Role: conf.TenantRole_TENANT_ROLE_ADMIN},
				{TenantId: "beta", Role: conf.TenantRole_TENANT_ROLE_MEMBER},
			}},
			{Username: "bob", PasswordHash: testfixture.PasswordHash, Memberships: []*conf.Membership{
				{TenantId: "alpha", Role: conf.TenantRole_TENANT_ROLE_MEMBER},
				{TenantId: "beta", Role: conf.TenantRole_TENANT_ROLE_ADMIN, Disabled: true},
			}},
			{Username: "operator", PasswordHash: testfixture.PasswordHash, PlatformOperator: true},
			{Username: "carol", PasswordHash: testfixture.PasswordHash, Memberships: []*conf.Membership{
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
		reply := testfixture.Request(handler, "POST", "/v1/access/login", body, "")
		if reply.Code != http.StatusUnauthorized {
			t.Fatalf("invalid login: %d %s", reply.Code, reply.Body)
		}
	}
}

func TestTenantListRequiresLogin(t *testing.T) {
	handler := testServer(t)
	for _, token := range []string{"", "invalid-token"} {
		reply := testfixture.Request(handler, "GET", "/v1/access/tenants", "", token)
		if reply.Code != http.StatusUnauthorized {
			t.Fatalf("unauthenticated list: %d %s", reply.Code, reply.Body)
		}
	}
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
			token := testfixture.Login(t, handler, tc.user)
			reply := testfixture.Request(handler, "GET", "/v1/access/tenants", "", token)
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
				reply = testfixture.Request(handler, "GET", path, "", token)
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
	alice := testfixture.Login(t, handler, "alice")
	bob := testfixture.Login(t, handler, "bob")
	operator := testfixture.Login(t, handler, "operator")
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
			reply := testfixture.Request(handler, "POST", "/v1/access/check"+tc.query, tc.body, tc.token)
			if reply.Code != tc.want {
				t.Fatalf("got %d, want %d (%s)", reply.Code, tc.want, reply.Body)
			}
		})
	}
}
