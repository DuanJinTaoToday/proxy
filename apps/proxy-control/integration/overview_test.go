package integration

import (
	"encoding/json"
	"net/http"
	"testing"

	v1 "github.com/DuanJinTaoToday/proxy/api/proxycontrol/v1"

	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/integration/testfixture"
)

func TestOverviewWhenSubaccountsAreDisabled(t *testing.T) {
	f := setupUsageWithSubaccounts(t, false)
	_, token := usageTenant(t, f, "13800005003")
	var overview v1.Overview
	readProto(t, testfixture.Request(f.http, "GET", "/v1/tenant/overview", "", token), &overview)
	if overview.GetCanCreateSubaccount() || len(overview.GetProducts()) != 2 {
		t.Fatalf("unexpected overview with subaccounts disabled: %v", &overview)
	}
}

func TestTenantOverview(t *testing.T) {
	f := setupUsage(t)
	tenantID, token := usageTenant(t, f, "13800005001")
	_, other := usageTenant(t, f, "13800005002")
	const path = "/v1/tenant/overview"
	if reply := testfixture.Request(f.http, "GET", path, "", ""); reply.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous overview: %d %s", reply.Code, reply.Body)
	}
	check := func(token, traffic string, low bool) {
		t.Helper()
		reply := testfixture.Request(f.http, "GET", path, "", token)
		var overview struct {
			WalletMinor         string                  `json:"wallet_minor"`
			TrafficBytes        string                  `json:"traffic_bytes"`
			ActiveStaticIPCount *int32                  `json:"active_static_ip_count"`
			Notices             []struct{ Kind string } `json:"notices"`
			CanCreateSubaccount bool                    `json:"can_create_subaccount"`
			Products            []struct {
				CanPurchase bool `json:"can_purchase"`
			} `json:"products"`
		}
		if reply.Code != http.StatusOK || json.Unmarshal(reply.Body.Bytes(), &overview) != nil ||
			overview.WalletMinor != "0" || overview.TrafficBytes != traffic || overview.ActiveStaticIPCount == nil ||
			*overview.ActiveStaticIPCount != 0 || !overview.CanCreateSubaccount || len(overview.Products) != 2 ||
			overview.Products[0].CanPurchase || overview.Products[1].CanPurchase || overview.Notices == nil ||
			(low && (len(overview.Notices) != 1 || overview.Notices[0].Kind != "low_traffic_balance")) ||
			(!low && len(overview.Notices) != 0) || reply.Header().Get("Cache-Control") != "no-store" {
			t.Fatalf("unexpected overview: %d %s", reply.Code, reply.Body)
		}
	}
	check(token, "0", false)
	if reply := testfixture.Request(f.http, "PUT", "/v1/tenant/traffic-alert", `{"threshold_bytes":"100"}`, token); reply.Code != http.StatusOK {
		t.Fatalf("set threshold: %d %s", reply.Code, reply.Body)
	}
	check(token, "0", true)
	check(other, "0", false)
	execSQL(t, f.db, "UPDATE tenant_balances SET available_bytes=-55 WHERE tenant_id=?", tenantID)
	check(token, "-55", true)
	check(other, "0", false)
	if reply := testfixture.Request(f.http, "POST", "/v1/tenants/ignored/proxy-extractions", `{"count":1}`, token); reply.Code != http.StatusNotFound {
		t.Fatalf("legacy extraction route: %d %s", reply.Code, reply.Body)
	}
}
