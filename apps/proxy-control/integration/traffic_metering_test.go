package integration

import (
	"fmt"
	"net/url"
	"testing"
	"time"

	v1 "github.com/DuanJinTaoToday/proxy/api/proxycontrol/v1"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/integration/testfixture"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/biz/usage"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/data/repo"
)

func TestMeteringCommitsDebitsAndArchivedBucketsWithWatermark(t *testing.T) {
	f := setupUsage(t)
	const loginBody = `{"phone":"13800000045","password":"tenant-password45"}`
	var tenant v1.TenantAccess
	var session v1.LoginResponse
	readProto(t, testfixture.Request(f.http, "POST", "/v1/access/register", loginBody, ""), &tenant)
	readProto(t, testfixture.Request(f.http, "POST", "/v1/access/login", loginBody, ""), &session)
	tenantID := tenant.GetTenantId()
	create := func(name string) string {
		t.Helper()
		var account v1.Subaccount
		body := fmt.Sprintf(`{"name":%q,"password":"Proxy45","active":true,"limit_bytes":"7"}`, name)
		readProto(t, testfixture.Request(f.http, "POST", "/v1/tenant/subaccounts", body, session.GetAccessToken()), &account)
		return account.GetClientId()
	}
	first, second := create("acct"), create("other")
	const firstPath = "/v1/tenant/subaccounts/"
	auth := func() *v1.AuthSnapshot {
		t.Helper()
		var state v1.AuthSnapshot
		readProto(t, authStateRequest(t, f.http, "/v1/gateway/auth-state/load", authToken, &v1.LoadAuthStateRequest{
			Username: tenant.GetPrimaryAccountName() + ".acct", Password: "Proxy45",
		}), &state)
		return &state
	}

	execSQL(t, f.db, "UPDATE tenant_balances SET available_bytes=10 WHERE tenant_id=?", tenantID)
	var order int64
	if err := f.db.Raw(`INSERT INTO purchase_orders
        (tenant_id,kind,status,locked_minor,debited_minor,released_minor,refunded_minor,delivered_bytes,delivered_count)
        VALUES (?,'traffic_package','succeeded',1,1,0,0,10,0) RETURNING id`, tenantID).Scan(&order).Error; err != nil {
		t.Fatal(err)
	}
	for i, amount := range []int64{6, 4} {
		execSQL(t, f.db, `INSERT INTO order_items
            (tenant_id,order_id,position,specification,reference,capacity_bytes,term_days,unit_minor,quantity,delivered_count)
            VALUES (?,?,?,'test','test',?,0,1,1,1)`, tenantID, order, i+1, amount)
		execSQL(t, f.db, `INSERT INTO traffic_grants(tenant_id,order_id,order_position,purchased_bytes,debt_offset_bytes,remaining_bytes,granted_at)
            VALUES (?,?,?,?,0,?,now())`, tenantID, order, i+1, amount, amount)
	}

	initial := auth()
	firstSample := recorder(0, first, beforeMidnight, 400, 900, 3, 4)
	secondSample := recorder(firstSample.GetEndOffset(), second, "2020-11-11T00:00:10+08:00", 40, 62, 1, 1)
	execSQL(t, f.db, "ALTER TABLE usage_minutely ADD CONSTRAINT reject_fixture CHECK (billable_bytes<5)")
	if res := submitResponse(t, f, "gateway-a", reportToken, firstSample, secondSample); res.Code < 500 {
		t.Fatalf("injected write failure should leave the batch retryable, got HTTP %d", res.Code)
	}
	rolledBack := auth()
	if rolledBack.GetQuota().GetAvailableBytes() != 10 || rolledBack.GetUsedBytes() != 0 ||
		rolledBack.GetQuota().GetUsageVersion() != initial.GetQuota().GetUsageVersion() {
		t.Fatal("failed batch changed admission state")
	}
	execSQL(t, f.db, "ALTER TABLE usage_minutely DROP CONSTRAINT reject_fixture")
	if ack := submit(t, f, "gateway-a", reportToken, firstSample, secondSample); ack.GetBillable() != 2 || ack.GetDuplicates() != 0 {
		t.Fatalf("rolled back cursor did not accept retry: %v", ack)
	}
	atLimit := auth()
	if atLimit.GetQuota().GetAvailableBytes() != 1 || atLimit.GetUsedBytes() != 7 || atLimit.GetLimitBytes() != 7 ||
		atLimit.GetQuota().GetUsageVersion() <= initial.GetQuota().GetUsageVersion() {
		t.Fatalf("bad post-debit snapshot: balance=%d usage=%d limit=%d version=%d",
			atLimit.GetQuota().GetAvailableBytes(), atLimit.GetUsedBytes(), atLimit.GetLimitBytes(), atLimit.GetQuota().GetUsageVersion())
	}
	var remaining []int64
	if err := f.db.Raw("SELECT remaining_bytes FROM traffic_grants ORDER BY granted_at,id").Scan(&remaining).Error; err != nil ||
		len(remaining) != 2 || remaining[0] != 0 || remaining[1] != 1 {
		t.Fatalf("grant consumption was not FIFO: %v, %v", remaining, err)
	}
	var updated v1.Subaccount
	readProto(t, testfixture.Request(f.http, "PATCH", firstPath+first, `{"limit_bytes":"10","update_mask":"limitBytes"}`, session.GetAccessToken()), &updated)
	raised := auth()
	if raised.GetUsedBytes() != 7 || raised.GetLimitBytes() != 10 || raised.GetQuota().GetUsageVersion() <= atLimit.GetQuota().GetUsageVersion() {
		t.Fatal("raising a limit reset usage or failed to advance its version")
	}

	operator := testfixture.Login(t, f.http, "operator")
	query := func(client string, want ...uint64) {
		t.Helper()
		q := url.Values{"tenant_id": {tenantID}, "start_time": {"2020-11-10T00:00:00+08:00"}, "end_time": {"2020-11-12T00:00:00+08:00"}, "granularity": {"3"}}
		if client != "" {
			q.Set("client_id", client)
		}
		var reply v1.QueryUsageResponse
		readProto(t, testfixture.Request(f.http, "GET", "/v1/operations/usage?"+q.Encode(), "", operator), &reply)
		if len(reply.GetBuckets()) != len(want) {
			t.Fatalf("unexpected bucket count: %s", reply.String())
		}
		for i, bytes := range want {
			if amount := reply.GetBuckets()[i].GetAmount(); amount.GetBillableBytes() != bytes || amount.GetTotalBytes() != bytes {
				t.Fatalf("bucket %d should have %d bytes: %s", i, bytes, amount.String())
			}
		}
	}
	query("", 7, 2)
	query(first, 7)
	archiver := usage.New(repo.NewUsageRepository(f.db), nil)
	if err := archiver.Archive(t.Context(), time.Date(2021, 2, 1, 0, 0, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	query("", 7, 2)
	late := recorder(secondSample.GetEndOffset(), first, "2020-11-10T21:24:00+08:00", 99, 100, 2, 3)
	if ack := submit(t, f, "gateway-a", reportToken, secondSample, late); ack.GetBillable() != 1 || ack.GetDuplicates() != 1 {
		t.Fatalf("late batch disposition: %v", ack)
	}
	if ack := submit(t, f, "gateway-a", reportToken, firstSample, secondSample, late); ack.GetDuplicates() != 3 || ack.GetBillable() != 0 {
		t.Fatalf("late replay charged twice: %v", ack)
	}
	query("", 12, 2)
	stillOwed := auth()
	if stillOwed.GetUsedBytes() != 12 || stillOwed.GetQuota().GetAvailableBytes() != -4 {
		t.Fatal("late usage was not charged against the current tenant balance")
	}
}
func TestMeteringSeparatesTenantsInMixedRecorderBatch(t *testing.T) {
	f := setupUsage(t)
	for _, tenant := range []string{"alpha", "beta"} {
		execSQL(t, f.db, "INSERT INTO tenants(id,name,primary_account_name) VALUES (?,?,?)", tenant, tenant, tenant)
		execSQL(t, f.db, "INSERT INTO tenant_balances(tenant_id,available_bytes,updated_at) VALUES (?,5000000000,now())", tenant)
	}
	const first = "00000000-0000-4000-8000-000000000001"
	const second = "00000000-0000-4000-8000-000000000002"
	for tenant, client := range map[string]string{"alpha": first, "beta": second} {
		execSQL(t, f.db, "INSERT INTO proxy_subaccounts(client_id,tenant_id,name,password_ciphertext,active) VALUES (?,?,'acct',decode('00','hex'),true)", client, tenant)
	}
	a := recorder(0, first, beforeMidnight, 3000000000, 0, 3000000000, 0)
	b := recorder(a.GetEndOffset(), second, beforeMidnight, 4, 9, 4, 9)
	unknown := recorder(b.GetEndOffset(), "unknown", beforeMidnight, 1, 1, 1, 1)
	ack := submit(t, f, "gateway-a", reportToken, a, b, unknown)
	if ack.GetBillable() != 2 || ack.GetQuarantined() != 1 || len(ack.GetWatermarks()) != 1 ||
		ack.GetWatermarks()[0].GetConfirmedOffset() != unknown.GetEndOffset() {
		t.Fatalf("unexpected dispositions or watermark: %v", ack)
	}
	duplicate := submit(t, f, "gateway-a", reportToken, a)
	if duplicate.GetDuplicates() != 1 || len(duplicate.GetWatermarks()) != 1 ||
		duplicate.GetWatermarks()[0].GetConfirmedOffset() != unknown.GetEndOffset() {
		t.Fatalf("earlier duplicate did not return unchanged watermark: %v", duplicate)
	}
	var balances []int64
	if err := f.db.Raw("SELECT available_bytes FROM tenant_balances ORDER BY tenant_id").Scan(&balances).Error; err != nil ||
		len(balances) != 2 || balances[0] != 2000000000 || balances[1] != 4999999987 {
		t.Fatalf("incorrect attribution of balances: %v, %v", balances, err)
	}
	q := url.Values{"tenant_id": {"alpha"}, "start_time": {"2020-11-10T00:00:00+08:00"}, "end_time": {"2020-11-11T00:00:00+08:00"},
		"granularity": {"3"}, "client_id": {second}}
	var reply v1.QueryUsageResponse
	operator := testfixture.Login(t, f.http, "operator")
	readProto(t, testfixture.Request(f.http, "GET", "/v1/operations/usage?"+q.Encode(), "", operator), &reply)
	if len(reply.GetBuckets()) != 0 {
		t.Fatal("tenant filter returned another tenant's usage")
	}
	q.Set("client_id", "malformed")
	testfixture.AssertStatus(t, testfixture.Request(f.http, "GET", "/v1/operations/usage?"+q.Encode(), "", operator), 400, "INVALID_CLIENT_ID")
}
