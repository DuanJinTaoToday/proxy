package integration

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	v1 "github.com/DuanJinTaoToday/proxy/api/proxycontrol/v1"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/integration/testfixture"
)

func TestPersistentTenantRegistrationAndAccess(t *testing.T) {
	fixture := setupUsage(t)
	handler := fixture.http
	const password = "tenant-pass1"
	register := func(phone, pwd string) *httptest.ResponseRecorder {
		return testfixture.Request(handler, "POST", "/v1/access/register", fmt.Sprintf(`{"phone":%q,"password":%q}`, phone, pwd), "")
	}
	testfixture.AssertStatus(t, register("123", "password1234"), http.StatusBadRequest, "INVALID_PHONE")
	testfixture.AssertStatus(t, register("13800001234", "12345678901"), http.StatusBadRequest, "INVALID_PASSWORD")

	// Simulate the second balance insert failing after tenant, admin and wallet writes.
	execSQL(t, fixture.db, "ALTER TABLE tenant_balances ADD CONSTRAINT reject_registration CHECK (updated_at IS NULL) NOT VALID")
	testfixture.AssertStatus(t, register("13800001234", password), http.StatusServiceUnavailable, "ACCESS_UNAVAILABLE")
	execSQL(t, fixture.db, "ALTER TABLE tenant_balances DROP CONSTRAINT reject_registration")
	count := func(query string, want int64, args ...any) {
		t.Helper()
		var got int64
		if err := fixture.db.Raw(query, args...).Scan(&got).Error; err != nil || got != want {
			t.Fatalf("query %s returned %d, want %d; %v", query, got, want, err)
		}
	}
	count("SELECT count(*) FROM tenants", 0)
	count("SELECT count(*) FROM tenant_admins", 0)
	count("SELECT count(*) FROM wallets", 0)
	count("SELECT count(*) FROM tenant_balances", 0)

	var wg sync.WaitGroup
	replies := make([]*httptest.ResponseRecorder, 2)
	for i, phone := range []string{"13800001234", "+8613800001234"} {
		wg.Go(func() { replies[i] = register(phone, password) })
	}
	wg.Wait()
	var tenant v1.TenantAccess
	success, conflict := 0, 0
	for _, r := range replies {
		switch r.Code {
		case http.StatusOK:
			success++
			readProto(t, r, &tenant)
		case http.StatusConflict:
			conflict++
			testfixture.AssertStatus(t, r, http.StatusConflict, "PHONE_ALREADY_REGISTERED")
		default:
			t.Fatalf("unexpected register result: %d %s", r.Code, r.Body)
		}
	}
	if success != 1 || conflict != 1 || tenant.GetTenantId() == "" || tenant.GetPrimaryAccountName() == "" {
		t.Fatalf("registration outcomes %d successes, %d conflicts, tenant %q", success, conflict, tenant.GetTenantId())
	}
	count("SELECT count(*) FROM tenants", 1)
	count("SELECT count(*) FROM tenant_admins WHERE phone_e164 = '+8613800001234'", 1)
	count("SELECT count(*) FROM wallets WHERE tenant_id = ? AND available_minor = 0", 1, tenant.GetTenantId())
	count("SELECT count(*) FROM tenant_balances WHERE tenant_id = ? AND available_bytes = 0", 1, tenant.GetTenantId())

	var other v1.TenantAccess
	readProto(t, register("13900001234", password), &other)
	if other.GetPrimaryAccountName() == tenant.GetPrimaryAccountName() {
		t.Fatal("duplicate primary account name")
	}
	creds := func(path, key, name, pwd string) *httptest.ResponseRecorder {
		return testfixture.Request(handler, "POST", path, fmt.Sprintf(`{%q:%q,"password":%q}`, key, name, pwd), "")
	}
	token := func(r *httptest.ResponseRecorder) string {
		t.Helper()
		var result v1.LoginResponse
		readProto(t, r, &result)
		if result.GetAccessToken() == "" {
			t.Fatal("missing access token")
		}
		return result.GetAccessToken()
	}
	const loginPath = "/v1/access/login"
	a := token(creds(loginPath, "phone", "+8613800001234", password))
	b := token(creds(loginPath, "phone", "13800001234", password))
	otherToken := token(creds(loginPath, "phone", "13900001234", password))
	proxyUsername := tenant.GetPrimaryAccountName() + ".demo"
	testfixture.AssertStatus(t, creds(loginPath, "phone", proxyUsername, password), http.StatusUnauthorized, "INVALID_LOGIN")
	testfixture.AssertStatus(t, creds("/v1/operations/access/login", "username", proxyUsername, password), http.StatusUnauthorized, "INVALID_LOGIN")
	testfixture.AssertStatus(t, creds(loginPath, "phone", "13800001234", "wrong"), http.StatusUnauthorized, "INVALID_LOGIN")
	account := func(tkn string) *httptest.ResponseRecorder {
		return testfixture.Request(handler, "GET", "/v1/tenant/account", "", tkn)
	}
	var owned v1.TenantAccess
	readProto(t, account(a), &owned)
	if owned.GetTenantId() != tenant.GetTenantId() {
		t.Fatal("session resolved to another tenant")
	}
	var selected v1.TenantAccess
	readProto(t, testfixture.Request(handler, "GET", "/v1/tenant/account?tenant_id="+other.GetTenantId(), "", a), &selected)
	if selected.GetTenantId() != tenant.GetTenantId() {
		t.Fatalf("account query selected tenant %q instead of the session tenant", selected.GetTenantId())
	}

	execSQL(t, fixture.db, `INSERT INTO operators (id,username,password_hash) VALUES
 ('readonly','readonly',?), ('billing','billing',?), ('support','support',?)`,
		testfixture.PasswordHash, testfixture.PasswordHash, testfixture.PasswordHash)
	execSQL(t, fixture.db, `INSERT INTO operator_permissions (operator_id,permission) VALUES
 ('readonly','read_operations'),('billing','read_operations'),('billing','manage_billing'),('support','manage_tenant_accounts')`)
	operator := func(name string) string {
		return token(creds("/v1/operations/access/login", "username", name, "test-password"))
	}
	readOnly, billing, support := operator("readonly"), operator("billing"), operator("support")
	testfixture.AssertStatus(t, account(billing), http.StatusForbidden, "ACCESS_DENIED")
	testfixture.AssertStatus(t, creds(loginPath, "phone", "readonly", "test-password"), http.StatusUnauthorized, "INVALID_LOGIN")
	check := func(actor, tenantID string, perm v1.Permission) *httptest.ResponseRecorder {
		return testfixture.Request(handler, "POST", "/v1/access/check", fmt.Sprintf(`{"permission":%d,"tenant_id":%q}`, perm, tenantID), actor)
	}
	testfixture.AssertStatus(t, check(a, tenant.GetTenantId(), v1.Permission_PERMISSION_READ_TENANT_USAGE), http.StatusOK, "")
	testfixture.AssertStatus(t, check(a, other.GetTenantId(), v1.Permission_PERMISSION_READ_TENANT_USAGE), http.StatusForbidden, "ACCESS_DENIED")
	testfixture.AssertStatus(t, check(otherToken, tenant.GetTenantId(), v1.Permission_PERMISSION_READ_TENANT_USAGE), http.StatusForbidden, "ACCESS_DENIED")
	testfixture.AssertStatus(t, check(readOnly, "", v1.Permission_PERMISSION_READ_OPERATIONS), http.StatusOK, "")
	testfixture.AssertStatus(t, check(readOnly, other.GetTenantId(), v1.Permission_PERMISSION_MANAGE_BILLING), http.StatusForbidden, "ACCESS_DENIED")
	testfixture.AssertStatus(t, check(billing, other.GetTenantId(), v1.Permission_PERMISSION_MANAGE_BILLING), http.StatusOK, "")
	testfixture.AssertStatus(t, check(a, tenant.GetTenantId(), v1.Permission_PERMISSION_MANAGE_BILLING), http.StatusForbidden, "ACCESS_DENIED")

	reset := func(tkn, pwd string) *httptest.ResponseRecorder {
		return testfixture.Request(handler, "POST", "/v1/operations/tenants/"+tenant.GetTenantId()+":reset-password", fmt.Sprintf(`{"password":%q}`, pwd), tkn)
	}
	testfixture.AssertStatus(t, reset(a, password), http.StatusForbidden, "ACCESS_DENIED")
	testfixture.AssertStatus(t, reset(otherToken, password), http.StatusForbidden, "ACCESS_DENIED")
	testfixture.AssertStatus(t, reset(readOnly, password), http.StatusForbidden, "ACCESS_DENIED")
	testfixture.AssertStatus(t, reset(billing, password), http.StatusForbidden, "ACCESS_DENIED")
	testfixture.AssertStatus(t, reset(support, "12345678901"), http.StatusBadRequest, "INVALID_PASSWORD")
	const newPassword = "new-pass-123"
	testfixture.AssertStatus(t, reset(support, newPassword), http.StatusOK, "")
	count("SELECT count(*) FROM tenant_admins WHERE tenant_id = ? AND session_version = 2 AND password_reset_by = 'support' AND password_reset_at IS NOT NULL", 1, tenant.GetTenantId())
	testfixture.AssertStatus(t, account(a), http.StatusUnauthorized, "AUTHENTICATION_REQUIRED")
	testfixture.AssertStatus(t, account(b), http.StatusUnauthorized, "AUTHENTICATION_REQUIRED")
	testfixture.AssertStatus(t, account(otherToken), http.StatusOK, "")
	testfixture.AssertStatus(t, creds(loginPath, "phone", "13800001234", password), http.StatusUnauthorized, "INVALID_LOGIN")
	testfixture.AssertStatus(t, account(token(creds(loginPath, "phone", "13800001234", newPassword))), http.StatusOK, "")
}
