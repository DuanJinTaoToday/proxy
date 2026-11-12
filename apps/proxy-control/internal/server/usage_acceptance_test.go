package server_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	v1 "github.com/DuanJinTaoToday/proxy/api/proxycontrol/v1"
	"google.golang.org/protobuf/proto"
)

const (
	beforeMidnight = "2020-11-10T23:59:00+08:00"
	afterMidnight  = "2020-11-11T00:01:00+08:00"
	dailyPath      = "/v1/tenants/%s/usage/daily?start_date=2020-11-10&end_date=2020-11-11"
)

func TestUsageArchiveAndLateReplay(t *testing.T) {
	f := setupUsage(t)
	first := recorder(0, "client-a", beforeMidnight, 130, 214, 130, 214)
	second := recorder(first.GetEndOffset(), "client-a", afterMidnight, 180, 294, 50, 80)
	submit(t, f, "gateway-a", reportToken, first)
	before := daily(t, f, "alpha", f.alice)
	archiveAt := time.Date(2021, 2, 1, 0, 0, 0, 0, time.UTC)
	for range 2 {
		if err := f.service.Archive(t.Context(), archiveAt); err != nil {
			t.Fatal(err)
		}
		if after := daily(t, f, "alpha", f.alice); !proto.Equal(before, after) {
			t.Fatalf("archive changed daily response: %v -> %v", before, after)
		}
	}
	got := submit(t, f, "gateway-a", reportToken, first, second)
	if got.GetDuplicates() != 1 || got.GetBillable() != 1 {
		t.Fatalf("late replay disposition = %v", got)
	}
	want := daily(t, f, "alpha", f.alice)
	assertDays(t, want, usageDay("2020-11-10", 130, 214, 344), usageDay("2020-11-11", 50, 80, 130))
	got = submit(t, f, "gateway-a", reportToken, first, second)
	if got.GetDuplicates() != 2 || !proto.Equal(want, daily(t, f, "alpha", f.alice)) {
		t.Fatalf("replay changed confirmed usage: %v", got)
	}
}

func TestUsageGrantExhaustionAndTenantIsolation(t *testing.T) {
	f := setupUsage(t)
	execSQL(t, f.db, `INSERT INTO billing_rates (scope_type, scope_id, upload_ratio_milli, download_ratio_milli, effective_from, created_by)
		VALUES ('tenant', 'alpha', 1200, 1000, '2020-11-11T00:00:00+08:00', 'test');
		INSERT INTO traffic_grants (tenant_id, granted_bytes, starts_at, source, created_by)
		VALUES ('alpha', 400, '2020-11-10T00:00:00+08:00', 'recharge', 'test');`)
	first := recorder(0, "client-a", beforeMidnight, 130, 214, 130, 214)
	second := recorder(first.GetEndOffset(), "client-a", afterMidnight, 180, 294, 50, 80)
	third := recorder(second.GetEndOffset(), "client-b", afterMidnight, 7, 13, 7, 13)
	submit(t, f, "gateway-a", reportToken, first, second, third)

	alpha := daily(t, f, "alpha", f.alice)
	assertDays(t, alpha, usageDay("2020-11-10", 130, 214, 344), usageDay("2020-11-11", 50, 80, 140))
	assertDays(t, daily(t, f, "beta", f.bob), usageDay("2020-11-11", 7, 13, 20))

	var balance v1.TenantBalance
	readProto(t, request(f.http, "GET", "/v1/tenants/alpha/balance", "", f.alice), &balance)
	if balance.AvailableBytes != -84 || balance.OverdraftBytes != 84 || len(balance.Grants) != 1 || balance.Grants[0].UsedBytes != 400 {
		t.Fatalf("balance = %v, want grant used 400 and overdraft 84", &balance)
	}
	if available, err := f.service.AvailableBytes(t.Context(), "alpha"); err != nil || available != -84 {
		t.Fatalf("cached available = %d, %v", available, err)
	}
	if got := balance.Grants[0].UsedBytes + balance.OverdraftBytes; got != alpha.Days[0].Amount.BillableBytes+alpha.Days[1].Amount.BillableBytes {
		t.Fatalf("usage does not reconcile to grants and overdraft")
	}
	for _, tenant := range []struct{ id, wrongToken string }{{"alpha", f.bob}, {"beta", f.alice}} {
		assertStatus(t, request(f.http, "GET", fmt.Sprintf(dailyPath, tenant.id), "", tenant.wrongToken), http.StatusForbidden, "ACCESS_DENIED")
		assertStatus(t, request(f.http, "GET", "/v1/tenants/"+tenant.id+"/balance", "", tenant.wrongToken), http.StatusForbidden, "ACCESS_DENIED")
	}
}

func TestReportingRejectsImpersonationAndRevokedTokens(t *testing.T) {
	f := setupUsage(t)
	line := recorder(0, "client-a", beforeMidnight, 130, 214, 130, 214)
	assertStatus(t, submitResponse(t, f, "gateway-a", f.alice, line), 401, "GATEWAY_UNAUTHENTICATED")
	assertStatus(t, submitResponse(t, f, "gateway-a", authToken, line), 401, "GATEWAY_UNAUTHENTICATED")
	assertStatus(t, submitResponse(t, f, "gateway-b", reportToken, line), 403, "GATEWAY_FORBIDDEN")
	execSQL(t, f.db, "UPDATE gateways SET status = 'disabled' WHERE id = 'gateway-a'")
	assertStatus(t, submitResponse(t, f, "gateway-a", reportToken, line), 403, "GATEWAY_FORBIDDEN")
	execSQL(t, f.db, "UPDATE gateway_tokens SET revoked_at = now() WHERE scope = 'report'")
	assertStatus(t, submitResponse(t, f, "gateway-a", reportToken, line), 401, "GATEWAY_UNAUTHENTICATED")
}

func TestUsageBatchRollbackAndQuarantine(t *testing.T) {
	f := setupUsage(t)
	first := recorder(0, "client-a", beforeMidnight, 130, 214, 130, 214)
	gap := recorder(first.GetEndOffset()+1, "client-a", afterMidnight, 180, 294, 50, 80)
	assertStatus(t, submitResponse(t, f, "gateway-a", reportToken, first, gap), 400, "INVALID_BATCH")
	if reply := daily(t, f, "alpha", f.alice); len(reply.Days) != 0 || reply.LastReceivedAt != nil {
		t.Fatalf("invalid batch changed usage: %v", reply)
	}

	bad := &v1.RecorderLine{FileId: "sessions", FileOffset: first.GetEndOffset(), EndOffset: first.GetEndOffset() + 3, Line: "{"}
	unknown := recorder(bad.GetEndOffset(), "unknown", afterMidnight, 1, 1, 1, 1)
	invalidID := recorder(unknown.GetEndOffset(), "client\x00a", afterMidnight, 1, 1, 1, 1)
	invalidID.Line = strings.ReplaceAll(invalidID.Line, `\x00`, `\u0000`)
	invalidID.EndOffset += 2
	counters := recorder(invalidID.GetEndOffset(), "client-a", afterMidnight, 1, 1, 2, 1)
	rejected := recorder(counters.GetEndOffset(), "client-a", afterMidnight, 1, 1, 1, 1)
	failure := `,"err":"upstream connection refused"}`
	rejected.Line = strings.TrimSuffix(rejected.Line, "}") + failure
	rejected.EndOffset += uint64(len(failure) - 1)
	last := recorder(rejected.GetEndOffset(), "client-a", afterMidnight, 180, 294, 50, 80)
	reply := submit(t, f, "gateway-a", reportToken, first, bad, unknown, invalidID, counters, rejected, last)
	if !proto.Equal(reply, &v1.SubmitBatchResponse{Billable: 2, Quarantined: 4, NotBillable: 1}) {
		t.Fatalf("unexpected dispositions: %v", reply)
	}
	assertDays(t, daily(t, f, "alpha", f.alice), usageDay("2020-11-10", 130, 214, 344), usageDay("2020-11-11", 50, 80, 130))

	var kinds []string
	if err := f.db.Table("record_anomalies").Order("file_offset").Pluck("kind", &kinds).Error; err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(kinds, []string{"invalid_record", "unknown_client_id", "unknown_client_id", "invalid_counters"}) {
		t.Fatalf("anomaly kinds = %v", kinds)
	}
}

func assertStatus(t *testing.T, reply *httptest.ResponseRecorder, status int, reason string) {
	t.Helper()
	if reply.Code != status || !strings.Contains(reply.Body.String(), reason) {
		t.Fatalf("status = %d %s, want %d %s", reply.Code, reply.Body, status, reason)
	}
}
