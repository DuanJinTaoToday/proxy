package integration

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	v1 "github.com/DuanJinTaoToday/proxy/api/proxycontrol/v1"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/integration/testfixture"
	"google.golang.org/protobuf/proto"
)

const commissionTransferPath = "/v1/tenant/commission-transfers"

func TestCommissionTransferRollsBackUntilWalletEntryIsRecorded(t *testing.T) {
	f := setupUsage(t)
	promoter, token := registerCommissionCustomer(t, f, "commission-transfer@example.test", "")
	execSQL(t, f.db, "INSERT INTO commission_accounts (tenant_id, available_minor) VALUES (?, 500)", promoter.GetTenantId())
	execSQL(t, f.db, `CREATE FUNCTION reject_test_wallet_entry() RETURNS trigger LANGUAGE plpgsql AS $$
		BEGIN RAISE EXCEPTION 'injected wallet entry failure'; END $$;
		CREATE TRIGGER reject_test_wallet_entry BEFORE INSERT ON wallet_entries
		FOR EACH ROW EXECUTE FUNCTION reject_test_wallet_entry()`)

	const transferBody = `{"request_key":"commission-transfer-rollback","amount_minor":"500"}`
	const ledgerPath = "/v1/tenant/wallet/ledger?kind=commission_transfer"
	testfixture.AssertStatus(t, testfixture.Request(f.http, "POST", commissionTransferPath, transferBody, token),
		http.StatusServiceUnavailable, "WALLET_UNAVAILABLE")
	var commission v1.CommissionPage
	readProto(t, testfixture.Request(f.http, "GET", tenantCommissionsPath, "", token), &commission)
	var overview v1.Overview
	readProto(t, testfixture.Request(f.http, "GET", "/v1/tenant/overview", "", token), &overview)
	var ledger v1.WalletEntries
	readProto(t, testfixture.Request(f.http, "GET", ledgerPath, "", token), &ledger)
	if commission.GetAvailableMinor() != 500 || overview.GetWalletMinor() != 0 || len(ledger.GetEntries()) != 0 {
		t.Fatalf("failed transfer leaked a balance or entry: commission=%s overview=%s ledger=%s", &commission, &overview, &ledger)
	}

	execSQL(t, f.db, "DROP TRIGGER reject_test_wallet_entry ON wallet_entries")
	var posted, replay v1.WalletEntry
	readProto(t, testfixture.Request(f.http, "POST", commissionTransferPath, transferBody, token), &posted)
	readProto(t, testfixture.Request(f.http, "POST", commissionTransferPath, transferBody, token), &replay)
	if !proto.Equal(&posted, &replay) || posted.GetId() <= 0 || posted.GetKind() != "commission_transfer" ||
		posted.GetBeforeMinor() != 0 || posted.GetAfterMinor() != 500 || posted.GetOrderId() != 0 || posted.GetCreatedAt() == nil {
		t.Fatalf("unexpected transfer or replay: posted=%s replay=%s", &posted, &replay)
	}
	readProto(t, testfixture.Request(f.http, "GET", ledgerPath, "", token), &ledger)
	readProto(t, testfixture.Request(f.http, "GET", tenantCommissionsPath, "", token), &commission)
	readProto(t, testfixture.Request(f.http, "GET", "/v1/tenant/overview", "", token), &overview)
	if commission.GetAvailableMinor() != 0 || overview.GetWalletMinor() != 500 || len(ledger.GetEntries()) != 1 ||
		!proto.Equal(ledger.Entries[0], &posted) {
		t.Fatalf("retry created the wrong balance or entry: commission=%s overview=%s ledger=%s", &commission, &overview, &ledger)
	}
}

func TestCommissionTransferWithConcurrentReferredPurchase(t *testing.T) {
	f := setupUsage(t)
	_, promoterToken := registerCommissionCustomer(t, f, "transfer-concurrent-promoter@example.test", "")
	invite := readPersonalInvite(t, testfixture.Request(f.http, "GET", tenantInvitePath, "", promoterToken))
	buyer, buyerToken := registerCommissionCustomer(t, f, "transfer-concurrent-buyer@example.test", invite.Code)
	execSQL(t, f.db, "UPDATE wallets SET available_minor=3000 WHERE tenant_id=?", buyer.GetTenantId())
	configureCommissionPackage(t, f)
	cashier := commissionOperator(t, f, "transfer-concurrent-cashier", "manage_billing")
	testfixture.AssertStatus(t, testfixture.Request(f.http, "PUT", commissionRatesPath,
		`{"first_purchase_bps":1000,"repeat_purchase_bps":500}`, cashier), http.StatusOK, "")
	const purchasePath = "/v1/tenant/orders/traffic-packages"
	const firstPurchase = `{"package_id":"5gb","package_version":"1","request_key":"first-purchase"}`
	testfixture.AssertStatus(t, testfixture.Request(f.http, "POST", purchasePath, firstPurchase, buyerToken), http.StatusOK, "")

	start := make(chan struct{})
	var wg sync.WaitGroup
	var transfer, purchase *httptest.ResponseRecorder
	wg.Go(func() {
		<-start
		const body = `{"request_key":"concurrent-transfer","amount_minor":"150"}`
		transfer = testfixture.Request(f.http, "POST", commissionTransferPath, body, promoterToken)
	})
	wg.Go(func() {
		<-start
		const body = `{"package_id":"5gb","package_version":"1","request_key":"concurrent-purchase"}`
		purchase = testfixture.Request(f.http, "POST", purchasePath, body, buyerToken)
	})
	close(start)
	wg.Wait()

	var receipt v1.WalletEntry
	readProto(t, transfer, &receipt)
	var order v1.PurchaseOrder
	readProto(t, purchase, &order)
	var page v1.CommissionPage
	readProto(t, testfixture.Request(f.http, "GET", tenantCommissionsPath, "", promoterToken), &page)
	if receipt.GetAfterMinor() != 150 || order.GetStatus() != "succeeded" || order.GetSettledMinor() != 1500 ||
		page.GetAvailableMinor() != 75 || len(page.GetRecords()) != 2 || page.Records[0].GetCommissionMinor() != 75 {
		t.Fatalf("unexpected concurrent settlement: entry=%s order=%s commissions=%s", &receipt, &order, &page)
	}
}
