package integration

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	v1 "github.com/DuanJinTaoToday/proxy/api/proxycontrol/v1"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/integration/testfixture"
	"google.golang.org/protobuf/proto"
)

func TestBillingWritesAndAdmissionSnapshots(t *testing.T) {
	f := setupUsage(t)
	const (
		recharges   = "/v1/operations/tenants/alpha/recharges"
		adjustments = "/v1/operations/tenants/alpha/adjustments"
		recharge    = `{"external_ref":"order-1","granted_bytes":"104857600"}`
	)
	operator := testfixture.Login(t, f.http, "operator")
	for _, token := range []string{f.alice, f.bob, operator} {
		testfixture.AssertStatus(t, testfixture.Request(f.http, "POST", recharges, recharge, token), http.StatusForbidden, "ACCESS_DENIED")
	}
	billing := testfixture.Login(t, f.http, "billing")
	write := func(path, body string) *v1.BalanceLedgerEntry {
		t.Helper()
		reply := &v1.BalanceLedgerEntry{}
		readProto(t, testfixture.Request(f.http, "POST", path, body, billing), reply)
		return reply
	}
	grant := write(recharges, recharge)
	if grant.GetGrantId() == "" || grant.GetAvailableAfter() != 104857600 ||
		!proto.Equal(grant, write(recharges, recharge)) {
		t.Fatalf("recharge and replay differ: %v", grant)
	}
	issued := &v1.CreateCredentialResponse{}
	readProto(t, testfixture.Request(f.http, "POST", "/v1/tenants/alpha/proxy-credentials", "{}", f.alice), issued)
	state := &v1.AuthSnapshot{}
	readProto(t, authStateRequest(t, f.http, "/v1/gateway/auth-state/load", authToken,
		&v1.LoadAuthStateRequest{Username: issued.GetUsername(), Password: issued.GetPassword()}), state)
	if state.GetQuota().GetAvailableBytes() != 104857600 {
		t.Fatalf("initial admission quota: %v", state.GetQuota())
	}
	refresh := func(want int64) *v1.AuthQuota {
		t.Helper()
		reply := &v1.RefreshAuthStateResponse{}
		readProto(t, authStateRequest(t, f.http, "/v1/gateway/auth-state/refresh", authToken,
			&v1.RefreshAuthStateRequest{References: []string{state.GetReference()}}), reply)
		if len(reply.GetSnapshots()) != 1 {
			t.Fatalf("unexpected refresh result: %v", reply)
		}
		q := reply.GetSnapshots()[0].GetQuota()
		if q.GetAvailableBytes() != want || q.GetAdmissionThresholdBytes() != 10485760 {
			t.Fatalf("quota = %v, want %d available and 10485760 admission threshold", q, want)
		}
		return q
	}
	execSQL(t, f.db, `INSERT INTO billing_rates
		(scope_type, scope_id, upload_ratio_milli, download_ratio_milli, effective_from, created_by)
		VALUES ('tenant', 'alpha', 1200, 1000, now(), 'test')`)
	at := time.Now().Format(time.RFC3339Nano)
	first := recorder(0, issued.GetClientId(), at, 80<<20, 0, 80<<20, 0)
	if got := submit(t, f, "gateway-a", reportToken, first).GetBillable(); got != 1 {
		t.Fatalf("initial billable records = %d", got)
	}
	refresh(4 << 20)
	next := recorder(first.GetEndOffset(), issued.GetClientId(), at, 84<<20, 0, 4<<20, 0)
	if got := submit(t, f, "gateway-a", reportToken, next).GetBillable(); got != 1 {
		t.Fatalf("existing connection billable records = %d", got)
	}
	refresh(-838861)
	subtract := fmt.Sprintf(`{"external_ref":"adjust-1","mode":"SUBTRACT","bytes":"5000000","grant_id":%q}`, grant.GetGrantId())
	debit := write(adjustments, subtract)
	if debit.GetAvailableAfter() != -5838861 || debit.GetOverdraftAddedBytes() != 5000000 ||
		!proto.Equal(debit, write(adjustments, subtract)) {
		t.Fatalf("unexpected subtraction: %v", debit)
	}
	write(adjustments, `{"external_ref":"adjust-2","mode":"ADD","bytes":"1000000"}`)
	refresh(-4838861)
	starts := time.Now().Add(3 * time.Second).UTC().Truncate(time.Millisecond)
	future := fmt.Sprintf(`{"external_ref":"order-2","granted_bytes":"20000000","starts_at":%q}`, starts.Format(time.RFC3339Nano))
	credit := write(recharges, future)
	if credit.GetAvailableAfter() != -4838861 || !refresh(-4838861).GetValidUntil().AsTime().Equal(starts) {
		t.Fatalf("future grant changed admission before its start: %v", credit)
	}
	time.Sleep(time.Until(starts))
	refresh(15161139)
	refresh(15161139)
	audit := &v1.ListTrafficGrantsResponse{}
	readProto(t, testfixture.Request(f.http, "GET", "/v1/operations/tenants/alpha/grants", "", operator), audit)
	grants := audit.GetGrants()
	if audit.GetOverdraftBytes() != 0 || len(grants) != 3 || grants[2].GetGrantId() != credit.GetGrantId() ||
		grants[2].GetUsedBytes() != 4838861 || grants[2].GetOverdraftOffsetBytes() != 4838861 {
		t.Fatalf("debt offset is not reflected in the new grant: %v", audit)
	}
}
