package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"time"

	v1 "github.com/DuanJinTaoToday/proxy/api/proxycontrol/v1"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/integration/testfixture"
	"gorm.io/gorm"
)

type testMessage struct{ to, subject, body string }
type testMailbox chan testMessage

func (mailbox testMailbox) Deliver(ctx context.Context, to, subject, body string) error {
	select {
	case mailbox <- testMessage{to: to, subject: subject, body: body}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (mailbox testMailbox) receive(t *testing.T, address, subject string) testMessage {
	t.Helper()
	select {
	case message := <-mailbox:
		if message.to != address || message.subject != subject {
			t.Fatalf("unexpected message recipient or subject: %q %q", message.to, message.subject)
		}
		return message
	case <-time.After(3 * time.Second):
		t.Fatalf("no %s message delivered to %s", subject, address)
		return testMessage{}
	}
}

func (mailbox testMailbox) assertEmpty(t *testing.T) {
	t.Helper()
	select {
	case message := <-mailbox:
		t.Fatalf("unexpected email delivery: %s", message.subject)
	case <-time.After(80 * time.Millisecond):
	}
}

func accessJSON(values map[string]string) string {
	encoded, err := json.Marshal(values)
	if err != nil {
		panic(err)
	}
	return string(encoded)
}

func emailRequest(f usageFixture, path, email, operator string) *httptest.ResponseRecorder {
	return testfixture.Request(f.http, "POST", path, accessJSON(map[string]string{"email": email}), operator)
}

var codePattern = regexp.MustCompile(`code is ([0-9]{6})\.`)
var linkPattern = regexp.MustCompile(`https://[^\s]+`)

func issueRegistrationCode(t *testing.T, f usageFixture, email string) string {
	t.Helper()
	testfixture.AssertStatus(t, emailRequest(f, "/v1/access/register-code", email, ""), http.StatusOK, "")
	message := f.mailbox.receive(t, email, "RocProxy verification code")
	parts := codePattern.FindStringSubmatch(message.body)
	if len(parts) != 2 {
		t.Fatal("registration email missing verification code")
	}
	return parts[1]
}

func emailLinkToken(t *testing.T, message testMessage, path string) string {
	t.Helper()
	link, err := url.Parse(linkPattern.FindString(message.body))
	if err != nil || link.Host != "console.example.test" || link.Path != path || link.Query().Get("token") == "" {
		t.Fatalf("email missing expected console link %s", path)
	}
	return link.Query().Get("token")
}

func registerWithEmail(t *testing.T, f usageFixture, credentials string) *httptest.ResponseRecorder {
	t.Helper()
	var values map[string]string
	if err := json.Unmarshal([]byte(credentials), &values); err != nil {
		t.Fatal(err)
	}
	values["verification_code"] = issueRegistrationCode(t, f, values["email"])
	return testfixture.Request(f.http, "POST", "/v1/access/register", accessJSON(values), "")
}

func TestEmailAccessRejectsLegacyPayloads(t *testing.T) {
	f := setupUsage(t)
	for _, path := range []string{"/v1/access/login", "/v1/access/register"} {
		response := testfixture.Request(f.http, "POST", path, `{"phone":"13800000001","email":"alpha@example.test","password":"test-password"}`, "")
		testfixture.AssertStatus(t, response, http.StatusBadRequest, "INVALID_ACCESS_REQUEST")
	}
}
func TestVerificationExpiryAttemptsAndUnifiedReceipt(t *testing.T) {
	f := setupUsage(t)
	const email = "first.last+alias@example.test"
	const password = "verified-pass-1"
	register := func(email, code string) *httptest.ResponseRecorder {
		return testfixture.Request(f.http, "POST", "/v1/access/register", accessJSON(map[string]string{
			"email": email, "password": password, "verification_code": code,
		}), "")
	}
	code := issueRegistrationCode(t, f, email)
	execSQL(t, f.db, "UPDATE email_access_credentials SET expires_at = now() - interval '1 second' WHERE subject = ?", email)
	testfixture.AssertStatus(t, register(email, code), http.StatusBadRequest, "INVALID_VERIFICATION_CODE")
	code = issueRegistrationCode(t, f, email)
	wrongCode := "111111"
	if code == wrongCode {
		wrongCode = "222222"
	}
	for range 5 {
		testfixture.AssertStatus(t, register(email, wrongCode), http.StatusBadRequest, "INVALID_VERIFICATION_CODE")
	}
	testfixture.AssertStatus(t, register(email, code), http.StatusBadRequest, "INVALID_VERIFICATION_CODE")
	available := emailRequest(f, "/v1/access/register-code", email, "")
	code = codePattern.FindStringSubmatch(f.mailbox.receive(t, email, "RocProxy verification code").body)[1]
	testfixture.AssertStatus(t, register(email, code), http.StatusOK, "")
	testfixture.AssertStatus(t, register(email, code), http.StatusBadRequest, "INVALID_VERIFICATION_CODE")
	occupied := emailRequest(f, "/v1/access/register-code", " FIRST.LAST+ALIAS@EXAMPLE.TEST ", "")
	testfixture.AssertStatus(t, occupied, http.StatusOK, "")
	if occupied.Body.String() != available.Body.String() {
		t.Fatal("registration response discloses whether email is occupied")
	}
	f.mailbox.assertEmpty(t)
	testfixture.AssertStatus(t, emailRequest(f, "/v1/access/register-code", "prefix <member@example.test>", ""), http.StatusBadRequest, "INVALID_EMAIL")
	testfixture.AssertStatus(t, emailRequest(f, "/v1/access/register-code", strings.Repeat("e", 244)+"@example.test", ""), http.StatusBadRequest, "INVALID_EMAIL")

	const rateEmail = "ratelimit@example.test"
	for range 5 {
		issueRegistrationCode(t, f, rateEmail)
	}
	limited := emailRequest(f, "/v1/access/register-code", " RATELIMIT@Example.Test ", "")
	testfixture.AssertStatus(t, limited, http.StatusOK, "")
	if limited.Body.String() != available.Body.String() {
		t.Fatal("rate-limited request response discloses limiter state")
	}
	f.mailbox.assertEmpty(t)
}

func TestEmailRecoveryAndPasswordResetAtPublicRoutes(t *testing.T) {
	f := setupUsage(t)
	const original = "Original+Alias@Example.test"
	const normalized = "original+alias@example.test"
	const originalPassword = "email-password-1"
	create := func(email string) *v1.TenantAccess {
		t.Helper()
		var tenant v1.TenantAccess
		readProto(t, registerWithEmail(t, f, accessJSON(map[string]string{"email": email, "password": originalPassword})), &tenant)
		return &tenant
	}
	login := func(email, password string, status int) string {
		t.Helper()
		result := testfixture.Request(f.http, "POST", "/v1/access/login", accessJSON(map[string]string{"email": email, "password": password}), "")
		if status != http.StatusOK {
			testfixture.AssertStatus(t, result, status, "INVALID_LOGIN")
			return ""
		}
		var session v1.LoginResponse
		readProto(t, result, &session)
		return session.GetAccessToken()
	}
	account := func(token string) *httptest.ResponseRecorder {
		return testfixture.Request(f.http, "GET", "/v1/tenant/account", "", token)
	}
	redeem := func(path, token, password string) *httptest.ResponseRecorder {
		return testfixture.Request(f.http, "POST", path, accessJSON(map[string]string{"token": token, "password": password}), "")
	}
	tenant := create(original)
	occupied := create("taken@example.test")
	auth := login(normalized, originalPassword, http.StatusOK)
	unknown := emailRequest(f, "/v1/access/forgot-password", "unknown@example.test", "")
	known := emailRequest(f, "/v1/access/forgot-password", normalized, "")
	testfixture.AssertStatus(t, unknown, http.StatusOK, "")
	testfixture.AssertStatus(t, known, http.StatusOK, "")
	if known.Body.String() != unknown.Body.String() {
		t.Fatal("password reset response discloses account existence")
	}
	oldToken := emailLinkToken(t, f.mailbox.receive(t, original, "RocProxy password reset"), "/zh-CN/reset-password")
	testfixture.AssertStatus(t, emailRequest(f, "/v1/access/forgot-password", normalized, ""), http.StatusOK, "")
	token := emailLinkToken(t, f.mailbox.receive(t, original, "RocProxy password reset"), "/zh-CN/reset-password")
	const resetPath = "/v1/access:reset-password"
	const completeRecovery = "/v1/access:recover-email"
	testfixture.AssertStatus(t, redeem(resetPath, oldToken, "reset-password-2"), http.StatusBadRequest, "INVALID_ACCESS_TOKEN")
	testfixture.AssertStatus(t, redeem(completeRecovery, token, "reset-password-2"), http.StatusBadRequest, "INVALID_ACCESS_TOKEN")
	testfixture.AssertStatus(t, redeem(resetPath, token, "short"), http.StatusBadRequest, "INVALID_PASSWORD")
	testfixture.AssertStatus(t, redeem(resetPath, token, "reset-password-2"), http.StatusOK, "")
	testfixture.AssertStatus(t, redeem(resetPath, token, "another-password-3"), http.StatusBadRequest, "INVALID_ACCESS_TOKEN")
	testfixture.AssertStatus(t, account(auth), http.StatusUnauthorized, "AUTHENTICATION_REQUIRED")
	login(normalized, originalPassword, http.StatusUnauthorized)
	auth = login(normalized, "reset-password-2", http.StatusOK)

	execSQL(t, f.db, "INSERT INTO operators (id, username, password_hash) VALUES ('support', 'support', ?), ('cashier', 'cashier', ?)", testfixture.PasswordHash, testfixture.PasswordHash)
	execSQL(t, f.db, "INSERT INTO operator_permissions (operator_id, permission) VALUES ('support', 'manage_tenant_accounts'), ('cashier', 'manage_billing')")
	support := testfixture.Login(t, f.http, "support")
	cashier := testfixture.Login(t, f.http, "cashier")
	readonly := testfixture.Login(t, f.http, "operator")
	initiate := "/v1/operations/tenants/" + tenant.GetTenantId() + ":recover-email"
	for _, denied := range []string{auth, readonly, cashier} {
		testfixture.AssertStatus(t, emailRequest(f, initiate, "next@example.test", denied), http.StatusForbidden, "ACCESS_DENIED")
	}
	testfixture.AssertStatus(t, emailRequest(f, initiate, "TAKEN@EXAMPLE.TEST", support), http.StatusConflict, "EMAIL_ALREADY_REGISTERED")
	f.mailbox.assertEmpty(t)

	startRecovery := func(address string) string {
		t.Helper()
		testfixture.AssertStatus(t, emailRequest(f, initiate, address, support), http.StatusOK, "")
		return emailLinkToken(t, f.mailbox.receive(t, address, "RocProxy account recovery"), "/zh-CN/recover-email")
	}
	superseded := startRecovery("superseded@example.test")
	raceTarget := "claimed-after@example.test"
	collided := startRecovery(raceTarget)
	create(raceTarget)
	testfixture.AssertStatus(t, redeem(completeRecovery, superseded, "recovery-password-4"), http.StatusBadRequest, "INVALID_ACCESS_TOKEN")
	testfixture.AssertStatus(t, redeem(completeRecovery, collided, "recovery-password-4"), http.StatusConflict, "EMAIL_ALREADY_REGISTERED")
	testfixture.AssertStatus(t, account(auth), http.StatusOK, "")

	execSQL(t, f.db, "UPDATE wallets SET available_minor = 1500 WHERE tenant_id = ?", tenant.GetTenantId())
	execSQL(t, f.db, "UPDATE tenant_balances SET available_bytes = 2500 WHERE tenant_id = ?", tenant.GetTenantId())
	orderID := seedAccountOrder(t, f.db, tenant.GetTenantId())
	const replacement = "New.Case+Alias@example.test"
	newest := startRecovery(replacement)
	testfixture.AssertStatus(t, emailRequest(f, initiate, occupied.GetEmail(), support), http.StatusConflict, "EMAIL_ALREADY_REGISTERED")
	f.mailbox.assertEmpty(t)
	testfixture.AssertStatus(t, redeem(completeRecovery, newest, "recovery-password-4"), http.StatusOK, "")
	testfixture.AssertStatus(t, redeem(completeRecovery, newest, "recovery-password-5"), http.StatusBadRequest, "INVALID_ACCESS_TOKEN")
	testfixture.AssertStatus(t, account(auth), http.StatusUnauthorized, "AUTHENTICATION_REQUIRED")
	login(normalized, "reset-password-2", http.StatusUnauthorized)
	fresh := login(" NEW.CASE+ALIAS@EXAMPLE.TEST ", "recovery-password-4", http.StatusOK)
	var after v1.TenantAccess
	readProto(t, account(fresh), &after)
	if after.GetTenantId() != tenant.GetTenantId() || after.GetPrimaryAccountName() != tenant.GetPrimaryAccountName() ||
		after.GetEmail() != "new.case+alias@example.test" {
		t.Fatalf("recovery did not preserve identity: %+v", &after)
	}
	var wallet v1.WalletTenant
	readProto(t, testfixture.Request(f.http, "GET", "/v1/operations/tenants?query="+url.QueryEscape(" NEW.CASE+ALIAS@EXAMPLE.TEST "), "", readonly), &wallet)
	if wallet.GetAvailableMinor() != 1500 || wallet.GetAvailableTrafficBytes() != 2500 || wallet.GetTenantId() != after.GetTenantId() {
		t.Fatalf("recovery did not preserve wallet and traffic: %+v", &wallet)
	}
	verifyAccountOrder(t, f, fresh, orderID)
	var audits int64
	if err := f.db.Raw(`SELECT count(*) FROM email_recovery_audit WHERE tenant_id = ? AND operator_id = 'support'
        AND previous_email = ? AND verified_email = ?`, tenant.GetTenantId(), normalized, after.GetEmail()).Scan(&audits).Error; err != nil || audits != 1 {
		t.Fatalf("recovery audit count = %d, want 1: %v", audits, err)
	}
}

func seedAccountOrder(t *testing.T, db *gorm.DB, tenantID string) int64 {
	t.Helper()
	var orderID int64
	err := db.Raw(`INSERT INTO purchase_orders
        (tenant_id, kind, status, locked_minor, debited_minor, released_minor, refunded_minor, delivered_bytes, delivered_count)
        VALUES (?, 'traffic_package', 'succeeded', 150, 150, 0, 0, 2500, 0) RETURNING id`, tenantID).Scan(&orderID).Error
	if err != nil || orderID == 0 {
		t.Fatalf("could not seed order: %v", err)
	}
	return orderID
}

func verifyAccountOrder(t *testing.T, f usageFixture, token string, orderID int64) {
	t.Helper()
	var orders v1.PurchaseOrders
	response := testfixture.Request(f.http, "GET", "/v1/tenant/orders?kind=traffic_package", "", token)
	readProto(t, response, &orders)
	if len(orders.GetOrders()) != 1 || orders.GetOrders()[0].GetId() != orderID || orders.GetOrders()[0].GetDeliveredBytes() != 2500 {
		t.Fatalf("tenant order identity or purchase details changed: %s", &orders)
	}
}

func TestEmailMigrationPreservesSeedAccountIdentity(t *testing.T) {
	var orderID int64
	f := setupUsageWithLegacyAccounts(t, true, func(db *gorm.DB) {
		execSQL(t, db, "INSERT INTO tenants (id, name, primary_account_name) VALUES ('alpha', 'Alpha', 'localalpha')")
		execSQL(t, db, "INSERT INTO tenant_admins (tenant_id, phone_e164, password_hash) VALUES ('alpha', '+8613800000001', ?)", testfixture.PasswordHash)
		execSQL(t, db, "INSERT INTO wallets (tenant_id, available_minor) VALUES ('alpha', 1800)")
		execSQL(t, db, "INSERT INTO tenant_balances (tenant_id, available_bytes, updated_at) VALUES ('alpha', 2500, now())")
		orderID = seedAccountOrder(t, db, "alpha")
	})
	var session v1.LoginResponse
	readProto(t, testfixture.Request(f.http, "POST", "/v1/access/login",
		`{"email":"alpha@example.test","password":"test-password"}`, ""), &session)
	var account v1.TenantAccess
	readProto(t, testfixture.Request(f.http, "GET", "/v1/tenant/account", "", session.GetAccessToken()), &account)
	if account.GetTenantId() != "alpha" || account.GetPrimaryAccountName() != "localalpha" || account.GetEmail() != "alpha@example.test" {
		t.Fatalf("legacy account migrated with different identity: %s", &account)
	}
	operator := testfixture.Login(t, f.http, "operator")
	var summary v1.WalletTenant
	readProto(t, testfixture.Request(f.http, "GET", "/v1/operations/tenants?query=alpha%40example.test", "", operator), &summary)
	if summary.GetTenantId() != "alpha" || summary.GetAvailableMinor() != 1800 || summary.GetAvailableTrafficBytes() != 2500 {
		t.Fatalf("legacy account assets changed: %s", &summary)
	}
	verifyAccountOrder(t, f, session.GetAccessToken(), orderID)
}

func TestEmailRequestRateLimitBySource(t *testing.T) {
	f := setupUsage(t)
	for i := range 60 {
		result := emailRequest(f, "/v1/access/forgot-password", fmt.Sprintf("unknown-%d@example.test", i), "")
		testfixture.AssertStatus(t, result, http.StatusOK, "")
	}
	const email = "source-limited@example.test"
	limited := emailRequest(f, "/v1/access/register-code", email, "")
	testfixture.AssertStatus(t, limited, http.StatusOK, "")
	f.mailbox.assertEmpty(t)

	req := httptest.NewRequest("POST", "/v1/access/register-code", strings.NewReader(`{"email":"source-limited@example.test"}`))
	req.RemoteAddr = "192.0.2.55:2345"
	result := httptest.NewRecorder()
	f.http.ServeHTTP(result, req)
	testfixture.AssertStatus(t, result, http.StatusOK, "")
	if limited.Body.String() != result.Body.String() {
		t.Fatal("response discloses source limiter state")
	}
	f.mailbox.receive(t, email, "RocProxy verification code")
}
