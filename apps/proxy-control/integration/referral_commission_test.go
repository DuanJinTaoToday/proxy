package integration

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	v1 "github.com/DuanJinTaoToday/proxy/api/proxycontrol/v1"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/integration/testfixture"
)

const commissionRatesPath = "/v1/operations/commission-rates"
const tenantCommissionsPath = "/v1/tenant/commissions"

func TestCommissionSettlementRollsBackWithWalletAndProduct(t *testing.T) {
	f := setupUsage(t)
	_, promoterToken := registerCommissionCustomer(t, f, "commission-promoter@example.test", "")
	invite := readPersonalInvite(t, testfixture.Request(f.http, "GET", tenantInvitePath, "", promoterToken))
	buyer, buyerToken := registerCommissionCustomer(t, f, "commission-buyer@example.test", invite.Code)
	req := func(method, path, body, token string) *httptest.ResponseRecorder {
		return testfixture.Request(f.http, method, path, body, token)
	}

	// Arrange an on-sale package and wallet balance independently of commission.
	execSQL(t, f.db, "UPDATE wallets SET available_minor=3000 WHERE tenant_id=?", buyer.GetTenantId())
	configureCommissionPackage(t, f)
	cashier := commissionOperator(t, f, "commission-cashier", "manage_billing")
	var rates v1.CommissionRates
	readProto(t, req("PUT", commissionRatesPath, `{"first_purchase_bps":1000,"repeat_purchase_bps":500}`, cashier), &rates)

	// Inject a write failure after the normal product fulfillment and commission balance update.
	execSQL(t, f.db, `CREATE FUNCTION reject_test_commission() RETURNS trigger LANGUAGE plpgsql AS $$
		BEGIN RAISE EXCEPTION 'injected commission write failure'; END $$;
		CREATE TRIGGER reject_test_commission BEFORE INSERT ON referral_commissions
		FOR EACH ROW EXECUTE FUNCTION reject_test_commission()`)
	const purchase = `{"package_id":"5gb","package_version":"1","request_key":"commission-rollback"}`
	testfixture.AssertStatus(t, req("POST", "/v1/tenant/orders/traffic-packages", purchase, buyerToken), http.StatusServiceUnavailable, "WALLET_UNAVAILABLE")
	var overview v1.Overview
	readProto(t, req("GET", "/v1/tenant/overview", "", buyerToken), &overview)
	if overview.GetWalletMinor() != 3000 || overview.GetTrafficBytes() != 0 {
		t.Fatalf("failed commission leaked wallet or delivery: %s", &overview)
	}
	var orders v1.PurchaseOrders
	readProto(t, req("GET", "/v1/tenant/orders", "", buyerToken), &orders)
	if len(orders.GetOrders()) != 0 {
		t.Fatalf("failed commission created an order: %s", &orders)
	}
	var ledger v1.WalletEntries
	readProto(t, req("GET", "/v1/tenant/wallet/ledger", "", buyerToken), &ledger)
	if len(ledger.GetEntries()) != 0 {
		t.Fatalf("failed commission leaked a wallet entry: %s", &ledger)
	}
	var page v1.CommissionPage
	readProto(t, req("GET", tenantCommissionsPath, "", promoterToken), &page)
	if page.GetAvailableMinor() != 0 || len(page.GetRecords()) != 0 {
		t.Fatalf("failed commission leaked an account credit: %s", &page)
	}

	execSQL(t, f.db, "DROP TRIGGER reject_test_commission ON referral_commissions")
	var order v1.PurchaseOrder
	readProto(t, req("POST", "/v1/tenant/orders/traffic-packages", purchase, buyerToken), &order)
	if order.GetStatus() != "succeeded" || order.GetSettledAt() == nil {
		t.Fatalf("retry did not settle: %s", &order)
	}

	response := req("GET", tenantCommissionsPath, "", promoterToken)
	readProto(t, response, &page)
	if page.GetAvailableMinor() != 150 || len(page.GetRecords()) != 1 {
		t.Fatalf("unexpected commission after retry: %s", &page)
	}
	row := page.GetRecords()[0]
	if row.GetCustomerKey() == "" || strings.Contains(response.Body.String(), buyer.GetTenantId()) ||
		row.GetClassification() != "first_purchase" || row.GetCommissionMinor() != 150 || row.GetNetDebitMinor() != 1500 ||
		row.GetRateVersion() != rates.GetVersion() || !row.GetSettledAt().AsTime().Equal(order.GetSettledAt().AsTime()) {
		t.Fatalf("unexpected commission row: %s", row)
	}
}

func TestCommissionConcurrentFirstSettlementWhilePaused(t *testing.T) {
	f := setupUsage(t)
	promoter, promoterToken := registerCommissionCustomer(t, f, "comm-paused-promoter@example.test", "")
	invite := readPersonalInvite(t, testfixture.Request(f.http, "GET", tenantInvitePath, "", promoterToken))
	buyer, buyerToken := registerCommissionCustomer(t, f, "comm-paused-buyer@example.test", invite.Code)
	execSQL(t, f.db, "UPDATE wallets SET available_minor=6000 WHERE tenant_id=?", buyer.GetTenantId())
	configureCommissionPackage(t, f)

	var rates v1.CommissionRates
	cashier := commissionOperator(t, f, "comm-paused-cashier", "manage_billing")
	readProto(t, testfixture.Request(f.http, "PUT", commissionRatesPath, `{"first_purchase_bps":10000,"repeat_purchase_bps":2500}`, cashier), &rates)
	support := commissionOperator(t, f, "comm-paused-support", "manage_tenant_accounts")
	promotionPath := "/v1/operations/tenants/" + promoter.GetTenantId()
	testfixture.AssertStatus(t, testfixture.Request(f.http, "POST", promotionPath+":pause-promotion", "", support), http.StatusOK, "")

	purchase := func(key string) *httptest.ResponseRecorder {
		body := fmt.Sprintf(`{"package_id":"5gb","package_version":"1","request_key":%q}`, key)
		return testfixture.Request(f.http, "POST", "/v1/tenant/orders/traffic-packages", body, buyerToken)
	}
	var responses [2]*httptest.ResponseRecorder
	var wg sync.WaitGroup
	for i := range responses {
		wg.Go(func() { responses[i] = purchase(fmt.Sprintf("commission-race-%d", i)) })
	}
	wg.Wait()
	var firstOrder, secondOrder v1.PurchaseOrder
	readProto(t, responses[0], &firstOrder)
	readProto(t, responses[1], &secondOrder)
	if firstOrder.GetId() == secondOrder.GetId() {
		t.Fatal("independent purchases returned the same order")
	}

	var page v1.CommissionPage
	refresh := func() {
		t.Helper()
		readProto(t, testfixture.Request(f.http, "GET", tenantCommissionsPath, "", promoterToken), &page)
	}
	refresh()
	if page.GetAvailableMinor() != 0 || len(page.GetRecords()) != 2 {
		t.Fatalf("unexpected paused ledger: %s", &page)
	}
	first, second := page.Records[1], page.Records[0]
	if first.GetClassification() != "first_purchase" || second.GetClassification() != "repeat_purchase" ||
		first.GetSettlementSequence() != 1 || second.GetSettlementSequence() != 2 ||
		first.GetStatus() != "promotion_paused" || second.GetStatus() != "promotion_paused" ||
		first.GetRateBps() != 0 || second.GetRateBps() != 0 ||
		first.GetCustomerKey() != second.GetCustomerKey() {
		t.Fatalf("unexpected order or policy snapshot: %s", &page)
	}

	testfixture.AssertStatus(t, testfixture.Request(f.http, "POST", promotionPath+":resume-promotion", "", support), http.StatusOK, "")
	testfixture.AssertStatus(t, purchase("commission-after-resume"), http.StatusOK, "")
	refresh()
	if page.GetAvailableMinor() != 375 || len(page.GetRecords()) != 3 ||
		page.Records[0].GetSettlementSequence() != 3 || page.Records[0].GetCommissionMinor() != 375 ||
		page.Records[0].GetStatus() != "credited" || page.Records[2].GetStatus() != "promotion_paused" {
		t.Fatalf("resume affected the wrong settlement(s): %s", &page)
	}
}

func registerCommissionCustomer(t *testing.T, f usageFixture, email, inviteCode string) (*v1.TenantAccess, string) {
	t.Helper()
	var tenant v1.TenantAccess
	readProto(t, registerWithEmail(t, f, accessJSON(map[string]string{
		"email": email, "password": referralPassword, "invite_code": inviteCode,
	})), &tenant)
	return &tenant, loginReferralTenant(t, f, email, referralPassword)
}

func configureCommissionPackage(t *testing.T, f usageFixture) {
	t.Helper()
	execSQL(t, f.db, "UPDATE product_sales SET open=true, quote_ready=true, acceptance_evidence='test commission' WHERE kind='dynamic'")
	execSQL(t, f.db, "UPDATE traffic_packages SET price_minor=1500, listed=true, version=1 WHERE id='5gb'")
}

func commissionOperator(t *testing.T, f usageFixture, name, permission string) string {
	t.Helper()
	execSQL(t, f.db, "INSERT INTO operators (id, username, password_hash) VALUES (?, ?, ?)", name, name, testfixture.PasswordHash)
	execSQL(t, f.db, "INSERT INTO operator_permissions (operator_id, permission) VALUES (?, ?)", name, permission)
	return testfixture.Login(t, f.http, name)
}
