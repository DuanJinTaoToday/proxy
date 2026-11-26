package integration

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	v1 "github.com/DuanJinTaoToday/proxy/api/proxycontrol/v1"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/integration/testfixture"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestReferralRegistrationRollbackAndImmutableAttribution(t *testing.T) {
	f := setupUsage(t)
	const promoterEmail = "rollback-promoter@example.test"
	promoter := registerReferralTenant(t, f, promoterEmail)
	token := loginReferralTenant(t, f, promoterEmail, referralPassword)
	invite := readPersonalInvite(t, testfixture.Request(f.http, "GET", tenantInvitePath, "", token))
	const invitedEmail = "rollback-invitee@example.test"
	code := issueRegistrationCode(t, f, invitedEmail)
	register := func() *httptest.ResponseRecorder {
		return registerWithInvite(f, invitedEmail, code, map[string]string{"link_invite_code": invite.Code})
	}
	execSQL(t, f.db, "ALTER TABLE tenant_balances ADD CONSTRAINT reject_registration CHECK (updated_at IS NULL) NOT VALID")
	testfixture.AssertStatus(t, register(), http.StatusServiceUnavailable, "ACCESS_UNAVAILABLE")
	var attributable int
	if err := f.db.Raw("SELECT count(*) FROM tenants WHERE referrer_tenant_id = ?", promoter.GetTenantId()).Scan(&attributable).Error; err != nil || attributable != 0 {
		t.Fatalf("failed registration left %d attributed tenants: %v", attributable, err)
	}
	execSQL(t, f.db, "ALTER TABLE tenant_balances DROP CONSTRAINT reject_registration")
	var child v1.TenantAccess
	readProto(t, register(), &child)

	assertRejected := func(code string, query string, args ...any) {
		t.Helper()
		var failure *pgconn.PgError
		if err := f.db.Exec(query, args...).Error; !errors.As(err, &failure) || failure.Code != code {
			t.Fatalf("expected PostgreSQL rejection %s, got %v", code, err)
		}
	}
	assertRejected("23514", "UPDATE tenants SET referrer_tenant_id = ?, referral_source = 'link' WHERE id = ?",
		child.GetTenantId(), promoter.GetTenantId())
	assertRejected("23514", "UPDATE tenants SET referrer_tenant_id = NULL, referral_source = NULL WHERE id = ?", child.GetTenantId())
	assertRejected("23503", `INSERT INTO tenants (id, name, primary_account_name, referrer_tenant_id, referral_source) VALUES
        ('self', 'Self', 'self', 'self', 'link')`)
	assertRejected("23503", `INSERT INTO tenants (id, name, primary_account_name, referrer_tenant_id, referral_source) VALUES
        ('cycle-a', 'A', 'cycle-a', 'cycle-b', 'link'), ('cycle-b', 'B', 'cycle-b', 'cycle-a', 'link')`)
	assertDirectReferral(t, f, promoter.GetTenantId(), "", "")
	assertDirectReferral(t, f, child.GetTenantId(), promoter.GetTenantId(), "link")
}

func TestOwnInviteCannotReopenRecoveredTenant(t *testing.T) {
	f := setupUsage(t)
	const original = "self-old@example.test"
	tenant := registerReferralTenant(t, f, original)
	originalBearer := loginReferralTenant(t, f, original, referralPassword)
	invite := readPersonalInvite(t, testfixture.Request(f.http, "GET", tenantInvitePath, "", originalBearer))

	const replacement = "self-new@example.test"
	otp := issueRegistrationCode(t, f, replacement)
	execSQL(t, f.db, "INSERT INTO operator_permissions (operator_id, permission) VALUES ('operator', 'manage_tenant_accounts')")
	support := testfixture.Login(t, f.http, "operator")
	start := "/v1/operations/tenants/" + tenant.GetTenantId() + ":recover-email"
	testfixture.AssertStatus(t, emailRequest(f, start, replacement, support), http.StatusOK, "")
	recovery := emailLinkToken(t, f.mailbox.receive(t, replacement, "RocProxy account recovery"), "/zh-CN/recover-email")
	result := testfixture.Request(f.http, "POST", "/v1/access:recover-email", accessJSON(map[string]string{
		"token": recovery, "password": referralPassword,
	}), "")
	testfixture.AssertStatus(t, result, http.StatusOK, "")

	testfixture.AssertStatus(t, registerWithInvite(f, " SELF-NEW@example.test ", otp, map[string]string{
		"invite_code": invite.Code,
	}), http.StatusConflict, "EMAIL_ALREADY_REGISTERED")
	recoveredBearer := loginReferralTenant(t, f, replacement, referralPassword)
	recoveredInvite := readPersonalInvite(t, testfixture.Request(f.http, "GET", tenantInvitePath, "", recoveredBearer))
	if recoveredInvite != invite {
		t.Fatal("email recovery changed the tenant's invite")
	}
	assertDirectReferral(t, f, tenant.GetTenantId(), "", "")
}

func TestReferralRegistrationWaitsForConcurrentPause(t *testing.T) {
	f := setupUsage(t)
	const promoterEmail = "concurrent-pause@example.test"
	promoter := registerReferralTenant(t, f, promoterEmail)
	bearer := loginReferralTenant(t, f, promoterEmail, referralPassword)
	invite := readPersonalInvite(t, testfixture.Request(f.http, "GET", tenantInvitePath, "", bearer))
	const invitedEmail = "pause-race@example.test"
	code := issueRegistrationCode(t, f, invitedEmail)

	pause := f.db.Begin()
	if pause.Error != nil {
		t.Fatal(pause.Error)
	}
	defer pause.Rollback()
	var blocker int
	if err := pause.Raw("SELECT pg_backend_pid()").Scan(&blocker).Error; err != nil {
		t.Fatal(err)
	}
	execSQL(t, pause, "UPDATE tenant_promotions SET paused = true WHERE tenant_id = ?", promoter.GetTenantId())
	response := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		response <- registerWithInvite(f, invitedEmail, code, map[string]string{"link_invite_code": invite.Code})
	}()
	deadline := time.Now().Add(2 * time.Second)
	for {
		var blocked bool
		query := `SELECT EXISTS (SELECT 1 FROM pg_stat_activity WHERE ? = ANY (pg_blocking_pids(pid)))`
		if err := f.db.Raw(query, blocker).Scan(&blocked).Error; err != nil {
			t.Fatal(err)
		}
		if blocked {
			break
		}
		select {
		case result := <-response:
			t.Fatalf("registration completed before promotion transaction: %d %s", result.Code, result.Body)
		default:
		}
		if time.Now().After(deadline) {
			t.Fatal("registration did not wait for the promotion transaction")
		}
		time.Sleep(10 * time.Millisecond)
	}
	if err := pause.Commit().Error; err != nil {
		t.Fatal(err)
	}
	select {
	case result := <-response:
		testfixture.AssertStatus(t, result, http.StatusBadRequest, "PROMOTION_PAUSED")
	case <-time.After(2 * time.Second):
		t.Fatal("registration did not finish after promotion transaction committed")
	}
	var tenant v1.TenantAccess
	readProto(t, registerWithInvite(f, invitedEmail, code, map[string]string{"invite_code": ""}), &tenant)
	assertDirectReferral(t, f, tenant.GetTenantId(), "", "")
}
