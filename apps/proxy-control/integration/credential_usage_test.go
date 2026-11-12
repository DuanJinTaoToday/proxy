package integration

import (
	"net/http"
	"testing"
	"time"

	v1 "github.com/DuanJinTaoToday/proxy/api/proxycontrol/v1"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/integration/testfixture"
)

func TestIssuedCredentialAttributesRecorderUsageAfterDisable(t *testing.T) {
	f := setupUsage(t)
	const createPath = "/v1/tenants/alpha/proxy-credentials"
	const loadPath = "/v1/gateway/auth-state/load"
	const refreshPath = "/v1/gateway/auth-state/refresh"
	const granted = 104857600
	execSQL(t, f.db, `INSERT INTO traffic_grants (tenant_id, granted_bytes, starts_at, activated_at, source, created_by)
		VALUES ('alpha', ?, now()-interval '1 day', now()-interval '1 day', 'recharge', 'test')`, granted)

	issued := &v1.CreateCredentialResponse{}
	readProto(t, testfixture.Request(f.http, "POST", createPath, "{}", f.alice), issued)
	load := &v1.LoadAuthStateRequest{Username: issued.GetUsername(), Password: issued.GetPassword()}
	state := &v1.AuthSnapshot{}
	readProto(t, authStateRequest(t, f.http, loadPath, authToken, load), state)
	if state.GetClientId() == "" || state.GetClientId() != issued.GetClientId() ||
		state.GetTenantId() != "alpha" || !state.GetActive() || state.GetQuota().GetAvailableBytes() != granted {
		t.Fatal("issued credential did not resolve to the expected tenant and grant")
	}

	// Both records are sampled before disable, but the second reaches the
	// control plane after the disable has committed.
	recordedAt := time.Now().In(time.FixedZone("UTC+08", 8*60*60))
	date := recordedAt.Format(time.DateOnly)
	first := recorder(0, state.GetClientId(), recordedAt.Format(time.RFC3339Nano), 130, 214, 130, 214)
	second := recorder(first.GetEndOffset(), state.GetClientId(), recordedAt.Format(time.RFC3339Nano), 180, 294, 50, 80)
	if ack := submit(t, f, "gateway-a", reportToken, first); ack.GetBillable() != 1 || ack.GetQuarantined() != 0 {
		t.Fatalf("initial record not accepted as billable: %v", ack)
	}
	assertDays(t, usageOn(t, f, "alpha", f.alice, date), usageDay(date, 130, 214, 344))

	disablePath := createPath + "/" + issued.GetClientId() + ":disable"
	readProto(t, testfixture.Request(f.http, "POST", disablePath, "{}", f.alice), &v1.DisableCredentialResponse{})
	testfixture.AssertStatus(t, authStateRequest(t, f.http, loadPath, authToken, load), http.StatusNotFound, "CREDENTIAL_DISABLED")
	refresh := &v1.RefreshAuthStateResponse{}
	readProto(t, authStateRequest(t, f.http, refreshPath, authToken,
		&v1.RefreshAuthStateRequest{References: []string{state.GetReference()}}), refresh)
	if snapshots := refresh.GetSnapshots(); len(snapshots) != 1 || snapshots[0].GetActive() ||
		snapshots[0].GetClientId() != state.GetClientId() || snapshots[0].GetTenantId() != "alpha" {
		t.Fatal("disable did not preserve the historical client ID and tenant")
	}

	if ack := submit(t, f, "gateway-a", reportToken, first, second); ack.GetDuplicates() != 1 ||
		ack.GetBillable() != 1 || ack.GetQuarantined() != 0 {
		t.Fatalf("late report did not separate replay from new usage: %v", ack)
	}
	assertDays(t, usageOn(t, f, "alpha", f.alice, date), usageDay(date, 180, 294, 474))
	if other := usageOn(t, f, "beta", f.bob, date); len(other.GetDays()) != 0 || other.GetLastReceivedAt() != nil {
		t.Fatalf("another tenant received the late usage: %v", other)
	}
	balance := &v1.TenantBalance{}
	readProto(t, testfixture.Request(f.http, "GET", "/v1/tenants/alpha/balance", "", f.alice), balance)
	if balance.GetAvailableBytes() != granted-474 || balance.GetOverdraftBytes() != 0 ||
		len(balance.GetGrants()) != 1 || balance.GetGrants()[0].GetUsedBytes() != 474 {
		t.Fatalf("late report did not debit the original tenant: %v", balance)
	}
}

func usageOn(t *testing.T, f usageFixture, tenant, token, date string) *v1.DailyUsageResponse {
	t.Helper()
	reply := &v1.DailyUsageResponse{}
	path := "/v1/tenants/" + tenant + "/usage/daily?start_date=" + date + "&end_date=" + date
	readProto(t, testfixture.Request(f.http, "GET", path, "", token), reply)
	return reply
}
