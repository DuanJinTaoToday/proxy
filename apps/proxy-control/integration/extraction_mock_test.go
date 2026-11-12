package integration

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"

	v1 "github.com/DuanJinTaoToday/proxy/api/proxycontrol/v1"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/integration/testfixture"
)

func TestExtractionMock(t *testing.T) {
	f := setupUsage(t)
	sample, err := os.ReadFile("../demo/mock_supplier_usage.sql")
	if err != nil {
		t.Fatal(err)
	}
	var date string
	if err := f.db.Raw("SELECT (CURRENT_TIMESTAMP AT TIME ZONE 'Asia/Shanghai')::date::text").Scan(&date).Error; err != nil {
		t.Fatal(err)
	}
	extract := func(tenant, token, body string) int {
		t.Helper()
		return testfixture.Request(f.http, http.MethodPost, "/v1/tenants/"+tenant+"/proxy-extractions", body, token).Code
	}
	if got := extract("alpha", f.alice, `{"count":1}`); got != http.StatusNotFound {
		t.Fatalf("unprovisioned extraction: %d", got)
	}
	beta := testfixture.Login(t, f.http, "beta-admin")
	for attempt := 0; attempt < 2; attempt++ {
		execSQL(t, f.db, string(sample))
		for _, tenant := range []struct {
			id, token string
			mb        uint64
		}{{"alpha", f.alice, 100}, {"beta", beta, 200}} {
			path := fmt.Sprintf("/v1/tenants/%s/usage/daily?start_date=%s&end_date=%s", tenant.id, date, date)
			var daily v1.DailyUsageResponse
			reply := testfixture.Request(f.http, http.MethodGet, path+"&source=api_extraction_mock", "", tenant.token)
			readProto(t, reply, &daily)
			if len(daily.Days) != 1 || daily.Days[0].GetAmount().GetTotalBytes() != tenant.mb*1048576 {
				t.Fatalf("attempt %d tenant %s: %v", attempt, tenant.id, daily.Days)
			}
			for _, field := range []string{"input_bytes", "output_bytes", "billable_bytes"} {
				if strings.Contains(reply.Body.String(), field) {
					t.Fatalf("unexpected field %s in %s", field, reply.Body)
				}
			}
			var tunnel v1.DailyUsageResponse
			readProto(t, testfixture.Request(f.http, http.MethodGet, path, "", tenant.token), &tunnel)
			if len(tunnel.Days) != 0 {
				t.Fatalf("mock usage appeared in tunnel usage: %v", tunnel.Days)
			}
			if tenant.id == "beta" {
				denied := testfixture.Request(f.http, http.MethodGet, path+"&source=api_extraction_mock", "", f.alice)
				if denied.Code != http.StatusForbidden {
					t.Fatalf("cross-tenant usage returned %d", denied.Code)
				}
			}
			response := testfixture.Request(f.http, http.MethodPost, "/v1/tenants/"+tenant.id+"/proxy-extractions", `{"count":2}`, tenant.token)
			var list v1.ExtractProxyResponse
			readProto(t, response, &list)
			if len(list.IpList) != 2 || list.IpList[0].Username != "mock-"+tenant.id || !strings.Contains(response.Body.String(), `"expiresAt"`) || !strings.Contains(response.Body.String(), `"ipList"`) {
				t.Fatalf("unexpected extraction for %s: %s", tenant.id, response.Body)
			}
		}
	}
	for _, item := range []struct {
		tenant, token, body string
		status              int
	}{
		{"alpha", "", `{"count":1}`, http.StatusUnauthorized},
		{"beta", f.alice, `{"tenantId":"alpha","count":1}`, http.StatusForbidden},
		{"beta", f.bob, `{"count":1}`, http.StatusForbidden},
		{"alpha", f.alice, `{}`, http.StatusBadRequest},
		{"alpha", f.alice, `{"count":101}`, http.StatusBadRequest},
		{"alpha", f.alice, `{"count":100}`, http.StatusOK},
	} {
		if got := extract(item.tenant, item.token, item.body); got != item.status {
			t.Errorf("extract %s %s: got %d, want %d", item.tenant, item.body, got, item.status)
		}
	}
}
