package integration

import (
	"encoding/csv"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	v1 "github.com/DuanJinTaoToday/proxy/api/proxycontrol/v1"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/integration/testfixture"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/biz/usage"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/data/repo"
)

func usageTenant(t *testing.T, f usageFixture, phone string) (string, string) {
	t.Helper()
	body := `{"phone":"` + phone + `","password":"tenant-password47"}`
	var tenant v1.TenantAccess
	var login v1.LoginResponse
	readProto(t, testfixture.Request(f.http, "POST", "/v1/access/register", body, ""), &tenant)
	readProto(t, testfixture.Request(f.http, "POST", "/v1/access/login", body, ""), &login)
	return tenant.GetTenantId(), login.GetAccessToken()
}

func TestTenantDailyUsageAndCSV(t *testing.T) {
	f := setupUsage(t)
	_, token := usageTenant(t, f, "13800000047")
	_, other := usageTenant(t, f, "13800000048")
	create := func(name string) string {
		t.Helper()
		var account v1.Subaccount
		readProto(t, testfixture.Request(f.http, "POST", "/v1/tenant/subaccounts",
			`{"name":"`+name+`","password":"Proxy47","active":true}`, token), &account)
		return account.GetClientId()
	}
	first, second := create("first"), create("second")
	day := time.Now().In(time.FixedZone("Shanghai", 8*3600)).Format(time.DateOnly)
	a := recorder(0, first, day+"T00:00:00+08:00", 10, 20, 10, 20)
	b := recorder(a.GetEndOffset(), second, day+"T00:00:00+08:00", 5, 7, 5, 7)
	c := recorder(b.GetEndOffset(), first, "2020-11-10T23:59:59+08:00", 1, 2, 1, 2)
	if ack := submit(t, f, "gateway-a", reportToken, a, b, c); ack.GetBillable() != 3 {
		t.Fatalf("unexpected metering result: %v", ack)
	}
	var disabled v1.Subaccount
	readProto(t, testfixture.Request(f.http, "PATCH", "/v1/tenant/subaccounts/"+first,
		`{"active":false,"update_mask":"active"}`, token), &disabled)
	const base = "/v1/tenant/usage/daily"
	list := func(owner, query string) []map[string]string {
		t.Helper()
		response := testfixture.Request(f.http, "GET", base+query, "", owner)
		var result struct{ Days []map[string]string }
		if response.Code != http.StatusOK || json.Unmarshal(response.Body.Bytes(), &result) != nil {
			t.Fatalf("daily usage: HTTP %d %s", response.Code, response.Body)
		}
		return result.Days
	}
	if rows := list(token, ""); len(rows) != 1 || rows[0]["date"] != day || rows[0]["billable_bytes"] != "42" || rows[0]["updated_at"] == "" {
		t.Fatalf("default Shanghai usage: %v", rows)
	}
	archiver := usage.New(repo.NewUsageRepository(f.db), nil)
	if err := archiver.Archive(t.Context(), time.Date(2021, 2, 1, 0, 0, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	wideRange := "?start_date=2020-11-10&end_date=" + day
	if rows := list(token, wideRange); len(rows) != 2 || rows[0]["billable_bytes"] != "3" || rows[1]["billable_bytes"] != "42" {
		t.Fatalf("historical range must include both endpoints: %v", rows)
	}
	wideExport := testfixture.Request(f.http, "GET", base+".csv"+wideRange, "", token)
	if wideExport.Code != http.StatusOK || strings.Count(wideExport.Body.String(), "\n") != 3 {
		t.Fatalf("wide CSV must include the complete range: HTTP %d %q", wideExport.Code, wideExport.Body)
	}
	filter := "?start_date=2020-11-10&end_date=2020-11-10&client_id=" + first
	rows := list(token, filter)
	if len(rows) != 1 || rows[0]["billable_bytes"] != "3" || rows[0]["updated_at"] == "" {
		t.Fatalf("disabled account history: %v", rows)
	}
	response := testfixture.Request(f.http, "GET", base+".csv"+filter, "", token)
	csvRows, err := csv.NewReader(strings.NewReader(response.Body.String())).ReadAll()
	if response.Code != http.StatusOK || !strings.HasPrefix(response.Header().Get("Content-Type"), "text/csv") ||
		err != nil || len(csvRows) != 2 || len(csvRows[1]) != 3 ||
		csvRows[1][0] != rows[0]["date"] || csvRows[1][1] != "0.000000003" {
		t.Fatalf("CSV should match the daily query: HTTP %d %q (%v)", response.Code, response.Body, err)
	}
	csvTime, csvErr := time.Parse(time.RFC3339Nano, csvRows[1][2])
	jsonTime, jsonErr := time.Parse(time.RFC3339Nano, rows[0]["updated_at"])
	if csvErr != nil || jsonErr != nil || !csvTime.Equal(jsonTime) {
		t.Fatalf("CSV and JSON timestamps differ: %q vs %q", csvRows[1][2], rows[0]["updated_at"])
	}
	if rows := list(other, filter); len(rows) != 0 {
		t.Fatalf("found another tenant's usage: %v", rows)
	}
	empty := testfixture.Request(f.http, "GET", base+".csv"+filter, "", other)
	if empty.Code != http.StatusOK || empty.Body.String() != "date,billable_gb,updated_at\n" {
		t.Fatalf("empty CSV should contain only headers: HTTP %d %q", empty.Code, empty.Body)
	}
	for _, path := range []string{base, base + ".csv"} {
		testfixture.AssertStatus(t, testfixture.Request(f.http, "GET", path+"?start_date=2025-02-29", "", token),
			http.StatusBadRequest, "INVALID_DATE_RANGE")
		testfixture.AssertStatus(t, testfixture.Request(f.http, "GET", path, "", ""),
			http.StatusUnauthorized, "AUTHENTICATION_REQUIRED")
	}
}

func TestTenantTrafficAlertCycles(t *testing.T) {
	f := setupUsage(t)
	tenant, token := usageTenant(t, f, "13800000047")
	const path = "/v1/tenant/traffic-alert"
	type state struct {
		Threshold *string `json:"threshold_bytes"`
		Balance   string  `json:"available_bytes"`
		Cycle     string  `json:"occurrence"`
		Since     string  `json:"triggered_at"`
	}
	alert := func(method, body string) state {
		t.Helper()
		result := testfixture.Request(f.http, method, path, body, token)
		var reply state
		if result.Code != http.StatusOK || json.Unmarshal(result.Body.Bytes(), &reply) != nil {
			t.Fatalf("traffic alert: HTTP %d %s", result.Code, result.Body)
		}
		return reply
	}
	if initial := alert("GET", ""); initial.Threshold != nil || initial.Cycle != "" {
		t.Fatalf("unexpected initial alert: %+v", initial)
	}
	first := alert("PUT", `{"threshold_bytes":"0"}`)
	if first.Cycle != "1" || first.Since == "" || first.Threshold == nil || *first.Threshold != "0" {
		t.Fatalf("setting the threshold at equality must trigger: %+v", first)
	}
	changed := alert("PUT", `{"threshold_bytes":"1"}`)
	if changed.Cycle != first.Cycle || changed.Since != first.Since {
		t.Fatalf("editing below threshold started another alert: %+v", changed)
	}

	var account v1.Subaccount
	readProto(t, testfixture.Request(f.http, "POST", "/v1/tenant/subaccounts",
		`{"name":"first","password":"Proxy47","active":true}`, token), &account)
	execSQL(t, f.db, "INSERT INTO operators (id,username,password_hash) VALUES ('cashier','cashier',?)", testfixture.PasswordHash)
	execSQL(t, f.db, "INSERT INTO operator_permissions (operator_id,permission) VALUES ('cashier','manage_billing')")
	fundCatalogWallet(t, f.db, tenant)
	cashier := testfixture.Login(t, f.http, "cashier")
	var pkg v1.TrafficPackage
	readProto(t, testfixture.Request(f.http, "PATCH", "/v1/operations/traffic-packages/5gb",
		`{"version":"1","capacity_bytes":"5000000000","price_minor":"500","listed":true}`, cashier), &pkg)
	var product v1.Product
	readProto(t, testfixture.Request(f.http, "PUT", "/v1/operations/product-sales/dynamic",
		`{"open":true,"quote_ready":true,"acceptance_evidence":"local acceptance"}`, cashier), &product)
	var order v1.PurchaseOrder
	readProto(t, testfixture.Request(f.http, "POST", "/v1/tenant/orders/traffic-packages",
		`{"package_id":"5gb","package_version":"2","request_key":"alert-recovery"}`, token), &order)
	recovered := alert("GET", "")
	if recovered.Cycle != "" || recovered.Balance != "5000000000" {
		t.Fatalf("purchase must reset alert: %+v", recovered)
	}
	record := recorder(0, account.GetClientId(), beforeMidnight, 2_000_000_000, 2_999_999_999, 2_000_000_000, 2_999_999_999)
	execSQL(t, f.db, "ALTER TABLE usage_minutely ADD CONSTRAINT reject_alert_fixture CHECK (billable_bytes < 5)")
	if failed := submitResponse(t, f, "gateway-a", reportToken, record); failed.Code < 500 {
		t.Fatalf("injected write failure returned HTTP %d", failed.Code)
	}
	if afterFailure := alert("GET", ""); afterFailure.Cycle != "" || afterFailure.Balance != recovered.Balance {
		t.Fatalf("failed Recorder write left a partial alert transition: %+v", afterFailure)
	}
	execSQL(t, f.db, "ALTER TABLE usage_minutely DROP CONSTRAINT reject_alert_fixture")
	if ack := submit(t, f, "gateway-a", reportToken, record); ack.GetBillable() != 1 {
		t.Fatalf("unexpected metering result: %v", ack)
	}
	second := alert("GET", "")
	if second.Cycle != "2" || second.Since == "" || second.Balance != "1" {
		t.Fatalf("debit to threshold must start a new cycle: %+v", second)
	}
	if ack := submit(t, f, "gateway-a", reportToken, record); ack.GetDuplicates() != 1 {
		t.Fatalf("unexpected replay result: %v", ack)
	}
	if repeated := alert("GET", ""); repeated.Cycle != second.Cycle || repeated.Since != second.Since || repeated.Balance != second.Balance {
		t.Fatalf("replay or query changed the active alert: %+v", repeated)
	}
	if cleared := alert("PUT", `{"threshold_bytes":null}`); cleared.Threshold != nil || cleared.Cycle != "" {
		t.Fatalf("clearing the threshold did not hide the alert: %+v", cleared)
	}
}
