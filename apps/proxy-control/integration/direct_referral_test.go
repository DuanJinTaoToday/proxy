package integration

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"testing"

	v1 "github.com/DuanJinTaoToday/proxy/api/proxycontrol/v1"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/integration/testfixture"
)

const (
	referralPassword = "referral-pass-1"
	tenantInvitePath = "/v1/tenant/invite"
)

type personalInvite struct {
	Code   string `json:"invite_code"`
	URL    string `json:"invite_url"`
	Status string `json:"promotion_status"`
}

func readPersonalInvite(t *testing.T, response *httptest.ResponseRecorder) personalInvite {
	t.Helper()
	testfixture.AssertStatus(t, response, http.StatusOK, "")
	var invite personalInvite
	if err := json.Unmarshal(response.Body.Bytes(), &invite); err != nil {
		t.Fatal(err)
	}
	return invite
}

func registerReferralTenant(t *testing.T, f usageFixture, email string) *v1.TenantAccess {
	t.Helper()
	var tenant v1.TenantAccess
	readProto(t, registerWithEmail(t, f, accessJSON(map[string]string{
		"email": email, "password": referralPassword,
	})), &tenant)
	return &tenant
}

func loginReferralTenant(t *testing.T, f usageFixture, email, password string) string {
	t.Helper()
	login := testfixture.Request(f.http, "POST", "/v1/access/login", accessJSON(map[string]string{
		"email": email, "password": password,
	}), "")
	var session v1.LoginResponse
	readProto(t, login, &session)
	return session.GetAccessToken()
}

func TestVerifiedTenantGetsStablePersonalInvite(t *testing.T) {
	f := setupUsage(t)
	const promoterEmail = "promoter@example.test"
	registerReferralTenant(t, f, promoterEmail)
	session := loginReferralTenant(t, f, promoterEmail, referralPassword)
	getInvite := func(token string) *httptest.ResponseRecorder {
		req := httptest.NewRequest("GET", tenantInvitePath, nil)
		req.Host = "untrusted.example.test"
		req.Header.Set("Authorization", "Bearer "+token)
		res := httptest.NewRecorder()
		f.http.ServeHTTP(res, req)
		return res
	}

	testfixture.AssertStatus(t, getInvite(""), http.StatusUnauthorized, "AUTHENTICATION_REQUIRED")
	testfixture.AssertStatus(t, getInvite(testfixture.Login(t, f.http, "operator")), http.StatusForbidden, "ACCESS_DENIED")
	var responses [2]*httptest.ResponseRecorder
	var wg sync.WaitGroup
	for i := range responses {
		wg.Go(func() { responses[i] = getInvite(session) })
	}
	wg.Wait()
	response := responses[0]
	invite := readPersonalInvite(t, response)
	link, err := url.Parse(invite.URL)
	if err != nil || link.Scheme != "https" || link.Host != "console.example.test" ||
		link.Path != "/zh-CN/register" || link.Query().Get("invite") != invite.Code ||
		!regexp.MustCompile(`^[0-9a-f]{32}$`).MatchString(invite.Code) || invite.Status != "active" {
		t.Fatalf("unexpected personal invite %v", invite)
	}
	if response.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("personal invite response is cacheable")
	}
	if concurrent := readPersonalInvite(t, responses[1]); concurrent != invite {
		t.Fatalf("concurrent first reads returned different invites: %+v and %+v", invite, concurrent)
	}
	freshSession := loginReferralTenant(t, f, promoterEmail, referralPassword)
	if repeated := readPersonalInvite(t, getInvite(freshSession)); repeated != invite {
		t.Fatalf("personal invite changed: %+v to %+v", invite, repeated)
	}
}

func TestRegistrationBindsOnlySelectedDirectReferrer(t *testing.T) {
	f := setupUsage(t)
	const linkEmail = "link-promoter@example.test"
	const manualEmail = "manual-promoter@example.test"
	linkTenant := registerReferralTenant(t, f, linkEmail)
	manualTenant := registerReferralTenant(t, f, manualEmail)
	inviteFor := func(email string) personalInvite {
		t.Helper()
		token := loginReferralTenant(t, f, email, referralPassword)
		return readPersonalInvite(t, testfixture.Request(f.http, "GET", tenantInvitePath, "", token))
	}
	link, manual := inviteFor(linkEmail), inviteFor(manualEmail)
	if link.Code == manual.Code {
		t.Fatal("distinct tenants share an invite code")
	}

	const referred = "new.member+alias@example.test"
	code := issueRegistrationCode(t, f, referred)
	unknown := strings.Repeat("0", 32)
	wrongOTP := "111111"
	if code == wrongOTP {
		wrongOTP = "222222"
	}
	request := func(email, otp, manualCode string) *httptest.ResponseRecorder {
		return registerWithInvite(f, email, otp, map[string]string{"link_invite_code": link.Code, "invite_code": manualCode})
	}
	testfixture.AssertStatus(t, request(referred, wrongOTP, unknown), http.StatusBadRequest, "INVALID_VERIFICATION_CODE")
	testfixture.AssertStatus(t, request(referred, code, unknown), http.StatusBadRequest, "INVALID_INVITE_CODE")
	testfixture.AssertStatus(t, request(referred, code, "contains-invalid-characters"), http.StatusBadRequest, "INVALID_INVITE_CODE")

	var replies [2]*httptest.ResponseRecorder
	var wg sync.WaitGroup
	for i, email := range []string{referred, " New.Member+Alias@Example.test "} {
		wg.Go(func() { replies[i] = request(email, code, " "+strings.ToUpper(manual.Code)+" ") })
	}
	wg.Wait()
	var newest v1.TenantAccess
	success := 0
	for _, response := range replies {
		if response.Code == http.StatusOK {
			success++
			readProto(t, response, &newest)
			if strings.Contains(response.Body.String(), manualTenant.GetTenantId()) || strings.Contains(response.Body.String(), manualEmail) {
				t.Fatal("registration response disclosed the referrer")
			}
		} else {
			testfixture.AssertStatus(t, response, http.StatusBadRequest, "INVALID_VERIFICATION_CODE")
		}
	}
	if success != 1 || newest.GetEmail() != referred {
		t.Fatalf("registration successes = %d, email = %q", success, newest.GetEmail())
	}
	assertDirectReferral(t, f, newest.GetTenantId(), manualTenant.GetTenantId(), "manual")
	var registrations int
	if err := f.db.Raw(`SELECT count(*) FROM tenant_admins a JOIN wallets w USING (tenant_id)
        JOIN tenant_balances b USING (tenant_id) WHERE a.email = ? AND w.available_minor = 0 AND b.available_bytes = 0`,
		referred).Scan(&registrations).Error; err != nil || registrations != 1 {
		t.Fatalf("found %d complete registrations: %v", registrations, err)
	}
	testfixture.AssertStatus(t, request(referred, code, link.Code), http.StatusBadRequest, "INVALID_VERIFICATION_CODE")
	assertDirectReferral(t, f, newest.GetTenantId(), manualTenant.GetTenantId(), "manual")

	const linkOnly = "link-only@example.test"
	var linked v1.TenantAccess
	readProto(t, registerWithInvite(f, linkOnly, issueRegistrationCode(t, f, linkOnly), map[string]string{
		"link_invite_code": link.Code,
	}), &linked)
	assertDirectReferral(t, f, linked.GetTenantId(), linkTenant.GetTenantId(), "link")

	const clearEmail = "cleared-invite@example.test"
	clearCode := issueRegistrationCode(t, f, clearEmail)
	testfixture.AssertStatus(t, request(clearEmail, clearCode, unknown), http.StatusBadRequest, "INVALID_INVITE_CODE")
	var cleared v1.TenantAccess
	readProto(t, request(clearEmail, clearCode, ""), &cleared)
	assertDirectReferral(t, f, cleared.GetTenantId(), "", "")
	assertDirectReferral(t, f, linkTenant.GetTenantId(), "", "")
}

func TestOperationsPromotionPauseAndResume(t *testing.T) {
	f := setupUsage(t)
	const promoterEmail = "managed-promoter@example.test"
	promoter := registerReferralTenant(t, f, promoterEmail)
	tenantToken := loginReferralTenant(t, f, promoterEmail, referralPassword)
	personal := func() personalInvite {
		return readPersonalInvite(t, testfixture.Request(f.http, "GET", tenantInvitePath, "", tenantToken))
	}

	execSQL(t, f.db, `INSERT INTO operators (id, username, password_hash)
        VALUES ('support', 'support', ?), ('billing', 'billing', ?)`, testfixture.PasswordHash, testfixture.PasswordHash)
	execSQL(t, f.db, `INSERT INTO operator_permissions (operator_id, permission)
        VALUES ('support', 'manage_tenant_accounts'), ('billing', 'manage_billing')`)
	support := testfixture.Login(t, f.http, "support")
	reader := testfixture.Login(t, f.http, "operator")
	billing := testfixture.Login(t, f.http, "billing")
	target := "/v1/operations/tenants/" + promoter.GetTenantId()
	read := func(token string) *httptest.ResponseRecorder {
		return testfixture.Request(f.http, "GET", target+"/promotion", "", token)
	}
	change := func(path, token string) *httptest.ResponseRecorder {
		return testfixture.Request(f.http, "POST", path, "{}", token)
	}
	assertStatus := func(response *httptest.ResponseRecorder, status string) {
		t.Helper()
		testfixture.AssertStatus(t, response, http.StatusOK, "")
		var values map[string]string
		if err := json.Unmarshal(response.Body.Bytes(), &values); err != nil || len(values) != 1 || values["promotion_status"] != status {
			t.Fatalf("unexpected promotion status %s: %v", response.Body, err)
		}
	}
	pause, resume := target+":pause-promotion", target+":resume-promotion"
	testfixture.AssertStatus(t, read(""), http.StatusUnauthorized, "AUTHENTICATION_REQUIRED")
	testfixture.AssertStatus(t, read(support), http.StatusForbidden, "ACCESS_DENIED")
	testfixture.AssertStatus(t, read(tenantToken), http.StatusForbidden, "ACCESS_DENIED")
	assertStatus(read(reader), "active")
	testfixture.AssertStatus(t, change(pause, ""), http.StatusUnauthorized, "AUTHENTICATION_REQUIRED")
	for _, denied := range []string{reader, billing, tenantToken} {
		testfixture.AssertStatus(t, change(pause, denied), http.StatusForbidden, "ACCESS_DENIED")
		testfixture.AssertStatus(t, change(resume, denied), http.StatusForbidden, "ACCESS_DENIED")
	}
	testfixture.AssertStatus(t, change("/v1/operations/tenants/missing:pause-promotion", support), http.StatusNotFound, "TENANT_NOT_FOUND")
	assertStatus(change(resume, support), "active")
	original := personal()
	assertStatus(change(pause, support), "paused")
	assertStatus(change(pause, support), "paused")
	assertStatus(read(reader), "paused")
	if paused := personal(); paused.Status != "paused" || paused.Code != original.Code || paused.URL != original.URL {
		t.Fatalf("unexpected paused invite: %+v", paused)
	}

	const newEmail = "after-pause@example.test"
	code := issueRegistrationCode(t, f, newEmail)
	register := func() *httptest.ResponseRecorder {
		return registerWithInvite(f, newEmail, code, map[string]string{"link_invite_code": original.Code})
	}
	testfixture.AssertStatus(t, register(), http.StatusBadRequest, "PROMOTION_PAUSED")
	assertStatus(change(resume, support), "active")
	assertStatus(change(resume, support), "active")
	assertStatus(read(reader), "active")
	if after := personal(); after != original {
		t.Fatalf("resume changed the original invite: %+v", after)
	}
	var tenant v1.TenantAccess
	readProto(t, register(), &tenant)
	assertDirectReferral(t, f, tenant.GetTenantId(), promoter.GetTenantId(), "link")

	var audit []struct {
		OperatorID, TenantID   string
		PreviousPaused, Paused bool
	}
	err := f.db.Raw(`SELECT operator_id, tenant_id, previous_paused, paused FROM tenant_promotion_audit ORDER BY id`).Scan(&audit).Error
	if err != nil || len(audit) != 2 {
		t.Fatalf("found %d audited changes, want 2: %v", len(audit), err)
	}
	for i, change := range audit {
		if change.OperatorID != "support" || change.TenantID != promoter.GetTenantId() ||
			change.PreviousPaused != (i == 1) || change.Paused != (i == 0) {
			t.Fatalf("unexpected promotion audit event: %+v", change)
		}
	}
}

func registerWithInvite(f usageFixture, email, otp string, attribution map[string]string) *httptest.ResponseRecorder {
	payload := map[string]string{"email": email, "password": referralPassword, "verification_code": otp}
	for field, value := range attribution {
		payload[field] = value
	}
	return testfixture.Request(f.http, "POST", "/v1/access/register", accessJSON(payload), "")
}

func assertDirectReferral(t *testing.T, f usageFixture, tenantID, referrerID, source string) {
	t.Helper()
	var stored struct {
		ReferrerTenantID *string
		ReferralSource   *string
	}
	row := f.db.Raw("SELECT referrer_tenant_id, referral_source FROM tenants WHERE id = ?", tenantID).Scan(&stored)
	if row.Error != nil || row.RowsAffected != 1 {
		t.Fatalf("read fixed direct attribution: %v", row.Error)
	}
	if referrerID == "" && source == "" {
		if stored.ReferrerTenantID != nil || stored.ReferralSource != nil {
			t.Fatal("tenant should have no direct referrer")
		}
		return
	}
	if stored.ReferrerTenantID == nil || *stored.ReferrerTenantID != referrerID ||
		stored.ReferralSource == nil || *stored.ReferralSource != source {
		t.Fatalf("tenant %q should have direct referrer %q from %q", tenantID, referrerID, source)
	}
}
