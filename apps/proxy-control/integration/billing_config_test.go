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

func TestBillingConfiguration(t *testing.T) {
	f := setupUsage(t)
	billing := testfixture.Login(t, f.http, "billing")
	operator := testfixture.Login(t, f.http, "operator")
	const base = "/v1/operations/billing/"
	const global = base + "settings/global/_"
	const group = base + "settings/group/a"
	const tenant = base + "settings/tenant/alpha"
	const membership = "/v1/operations/tenants/alpha/group"
	const groupBody = `{"group_id":"a","name":"Group A"}`
	const globalBody = `{"admission_threshold_bytes":"100","warning_threshold_bytes":"1000"}`
	const rateBody = `{"scope_type":"global","upload_ratio_milli":"1100","download_ratio_milli":"1000"}`
	call := func(method, path, body, token string, reply proto.Message) {
		t.Helper()
		readProto(t, testfixture.Request(f.http, method, path, body, token), reply)
	}
	for _, token := range []string{operator, f.alice, f.bob} {
		for _, req := range []struct{ method, path, body string }{
			{"PUT", global, globalBody}, {"POST", base + "groups", groupBody},
			{"POST", base + "rates", rateBody}, {"PUT", membership, `{"group_id":"a"}`},
		} {
			testfixture.AssertStatus(t, testfixture.Request(f.http, req.method, req.path, req.body, token), http.StatusForbidden, "ACCESS_DENIED")
		}
	}
	for _, token := range []string{f.alice, f.bob} {
		testfixture.AssertStatus(t, testfixture.Request(f.http, "GET", global, "", token), http.StatusForbidden, "ACCESS_DENIED")
	}
	settings := func(path string, admission, warning uint64) {
		t.Helper()
		var reply v1.BillingSettingsResponse
		call("GET", path, "", operator, &reply)
		if got := reply.GetEffective(); got == nil || got.GetAdmissionThresholdBytes() != admission || got.GetWarningThresholdBytes() != warning {
			t.Fatalf("effective settings for %s = %v, want %d/%d", path, got, admission, warning)
		}
		quota, err := f.service.AdmissionQuota(t.Context(), "alpha")
		if err != nil || quota.AdmissionThresholdBytes != admission {
			t.Fatalf("cached admission quota = %v, err %v, want %d", quota, err, admission)
		}
	}
	settings(global+"?tenant_id=alpha", 10485760, 1048576000)
	call("PUT", global, globalBody, billing, &v1.BillingSettingsResponse{})
	settings(tenant, 100, 1000)
	call("POST", base+"groups", groupBody, billing, &v1.TenantGroup{})
	call("PUT", group, `{"admission_threshold_bytes":"200"}`, billing, &v1.BillingSettingsResponse{})
	call("PUT", membership, `{"group_id":"a"}`, billing, &v1.TenantGroupMembership{})
	settings(tenant, 200, 1000)
	call("PUT", tenant, `{"warning_threshold_bytes":"300"}`, billing, &v1.BillingSettingsResponse{})
	settings(tenant, 200, 300)
	testfixture.AssertStatus(t, testfixture.Request(f.http, "PUT", group, `{"admission_threshold_bytes":"300"}`, billing),
		http.StatusBadRequest, "INVALID_THRESHOLDS")
	testfixture.AssertStatus(t, testfixture.Request(f.http, "PUT", global,
		`{"admission_threshold_bytes":"500","warning_threshold_bytes":"1000"}`, billing), http.StatusOK, "")
	testfixture.AssertStatus(t, testfixture.Request(f.http, "PUT", membership, `{}`, billing), http.StatusBadRequest, "INVALID_THRESHOLDS")
	settings(tenant, 200, 300)
	call("PUT", tenant, `{"admission_threshold_bytes":"0"}`, billing, &v1.BillingSettingsResponse{})
	settings(tenant, 0, 1000)
	call("PUT", tenant, `{}`, billing, &v1.BillingSettingsResponse{})
	settings(tenant, 200, 1000)

	createRate := func(body string) *v1.BillingRateVersion {
		t.Helper()
		reply := &v1.BillingRateVersion{}
		call("POST", base+"rates", body, billing, reply)
		return reply
	}
	globalRate := createRate(rateBody)
	groupRate := createRate(`{"scope_type":"group","scope_id":"a","upload_ratio_milli":"1200","download_ratio_milli":"1000"}`)
	beforeClear := time.Now().UTC().Truncate(time.Microsecond)
	var cleared v1.TenantGroupMembership
	call("PUT", membership, `{}`, billing, &cleared)
	if !cleared.GetEffectiveFrom().AsTime().After(beforeClear) || cleared.GetGroupId() != "" {
		t.Fatalf("unexpected clear version: %v", &cleared)
	}
	settings(tenant, 500, 1000)
	tenantRate := createRate(`{"scope_type":"tenant","scope_id":"alpha","upload_ratio_milli":"1300","download_ratio_milli":"1000"}`)
	futureRate := createRate(fmt.Sprintf(`{"scope_type":"tenant","scope_id":"alpha","upload_ratio_milli":"1400","download_ratio_milli":"1000","effective_from":%q}`,
		time.Now().Add(time.Hour).UTC().Format(time.RFC3339Nano)))
	testfixture.AssertStatus(t, testfixture.Request(f.http, "POST", base+"rates",
		`{"scope_type":"global","upload_ratio_milli":"1000","download_ratio_milli":"1000","effective_from":"2020-01-01T00:00:00Z"}`, billing),
		http.StatusBadRequest, "INVALID_EFFECTIVE_FROM")
	var lines []*v1.RecorderLine
	var offset uint64
	for _, at := range []time.Time{beforeClear, cleared.GetEffectiveFrom().AsTime(), tenantRate.GetEffectiveFrom().AsTime(), futureRate.GetEffectiveFrom().AsTime()} {
		line := recorder(offset, "client-a", at.Format(time.RFC3339Nano), 100, 0, 100, 0)
		lines = append(lines, line)
		offset = line.GetEndOffset()
	}
	if got := submit(t, f, "gateway-a", reportToken, lines...).GetBillable(); got != 4 {
		t.Fatalf("billable records = %d, want 4", got)
	}
	for _, expected := range []struct {
		rate  *v1.BillingRateVersion
		bytes int64
	}{{globalRate, 110}, {groupRate, 120}, {tenantRate, 130}, {futureRate, 140}} {
		var billed int64
		if err := f.db.Raw("SELECT COALESCE(SUM(billable_bytes), 0) FROM usage_minutely WHERE rate_id = ?", expected.rate.GetRateId()).Scan(&billed).Error; err != nil || billed != expected.bytes {
			t.Fatalf("rate %s billed %d, want %d, err %v", expected.rate.GetRateId(), billed, expected.bytes, err)
		}
	}
}
