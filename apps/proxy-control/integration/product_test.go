package integration

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	v1 "github.com/DuanJinTaoToday/proxy/api/proxycontrol/v1"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/integration/testfixture"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/biz/product"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/biz/wallet"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/data/repo"
	"google.golang.org/protobuf/proto"
	"gorm.io/gorm"
)

func TestProductCatalogAndSales(t *testing.T) {
	f := setupUsage(t)
	execSQL(t, f.db, "INSERT INTO operators (id,username,password_hash) VALUES ('cashier','cashier',?)", testfixture.PasswordHash)
	execSQL(t, f.db, "INSERT INTO operator_permissions (operator_id,permission) VALUES ('cashier','manage_billing')")
	cashier := testfixture.Login(t, f.http, "cashier")
	reader := testfixture.Login(t, f.http, "operator")

	assert := func(method, path, body, token string, code int, reason string) {
		t.Helper()
		testfixture.AssertStatus(t, testfixture.Request(f.http, method, path, body, token), code, reason)
	}
	list := func() *v1.Products {
		t.Helper()
		var response v1.Products
		readProto(t, testfixture.Request(f.http, "GET", "/public/products", "", ""), &response)
		return &response
	}
	const packages = "/v1/operations/traffic-packages"
	const sales = "/v1/operations/product-sales/"
	const open = `{"open":true,"quote_ready":true,"acceptance_evidence":"controlled acceptance"}`
	ctx := context.Background()
	initial := list().GetProducts()
	if len(initial) != 2 || initial[0].GetOpen() || initial[1].GetOpen() || len(initial[0].GetPackages()) != 0 {
		t.Fatalf("unexpected public catalog: %v", initial)
	}
	assert("PATCH", packages+"/5gb", `{"version":"1","capacity_bytes":"5000000000","price_minor":"500","listed":true}`, reader, http.StatusForbidden, "ACCESS_DENIED")
	assert("PUT", sales+"dynamic", open, cashier, http.StatusConflict, "PRODUCT_QUOTE_NOT_READY")
	assert("PUT", sales+"static", `{"open":true,"quote_ready":true}`, cashier, http.StatusBadRequest, "INVALID_PRODUCT_SALE")
	if err := f.db.Transaction(func(tx *gorm.DB) error {
		return repo.RequireProductSale(ctx, tx, product.Static)
	}); err != product.ErrSaleClosed {
		t.Fatalf("closed static product gate: %v", err)
	}

	var configured v1.TrafficPackage
	const base = `{"version":"1","capacity_bytes":"5000000000","price_minor":"500","listed":true}`
	readProto(t, testfixture.Request(f.http, "PATCH", packages+"/5gb", base, cashier), &configured)
	assert("PATCH", packages+"/5gb", base, cashier, http.StatusConflict, "PACKAGE_CHANGED")
	var opened v1.Product
	readProto(t, testfixture.Request(f.http, "PUT", sales+"dynamic", open, cashier), &opened)
	products := list().GetProducts()
	if !products[0].GetCanPurchase() || products[1].GetOpen() {
		t.Fatalf("unexpected product switches: %v", products)
	}
	tiers := products[0].GetPackages()
	if len(tiers) != 3 || tiers[0].GetCapacityBytes() != 5_000_000_000 || tiers[1].GetCapacityBytes() != 50_000_000_000 ||
		tiers[2].GetCapacityBytes() != 200_000_000_000 || tiers[0].GetPriceMinor() != 500 ||
		tiers[1].GetPriceMinor() != 4500 || tiers[2].GetPriceMinor() != 16000 {
		t.Fatalf("unexpected default tiers: %v", tiers)
	}

	const customerCredentials = `{"phone":"13800004301","password":"product-test-password"}`
	var customer v1.TenantAccess
	readProto(t, testfixture.Request(f.http, "POST", "/v1/access/register", customerCredentials, ""), &customer)
	var session v1.LoginResponse
	readProto(t, testfixture.Request(f.http, "POST", "/v1/access/login", customerCredentials, ""), &session)
	const checkout = "/v1/tenant/orders/traffic-packages"
	const purchase = `{"package_id":"50gb","package_version":"2","request_key":"product-order"}`
	tryPurchase := func(body string) *httptest.ResponseRecorder {
		return testfixture.Request(f.http, "POST", checkout, body, session.AccessToken)
	}
	shortfall := tryPurchase(purchase)
	testfixture.AssertStatus(t, shortfall, http.StatusConflict, "WALLET_INSUFFICIENT_FUNDS")
	if !strings.Contains(shortfall.Body.String(), `"shortfall_minor":"4500"`) {
		t.Fatalf("missing exact shortfall: %s", shortfall.Body)
	}
	var orders v1.PurchaseOrders
	readProto(t, testfixture.Request(f.http, "GET", "/v1/tenant/orders", "", session.AccessToken), &orders)
	if len(orders.GetOrders()) != 0 {
		t.Fatal("insufficient funds created an order")
	}
	fundCatalogWallet(t, f.db, customer.TenantId)
	testfixture.AssertStatus(t, tryPurchase(`{"package_id":"50gb","package_version":"1","request_key":"product-order"}`),
		http.StatusConflict, "PACKAGE_CHANGED")
	// Force the grant write to fail after the wallet settlement has executed.
	execSQL(t, f.db, "ALTER TABLE traffic_grants ADD CONSTRAINT reject_test_grant CHECK (purchased_bytes <> 50000000000)")
	testfixture.AssertStatus(t, tryPurchase(purchase), http.StatusServiceUnavailable, "WALLET_UNAVAILABLE")
	var currentWallet v1.WalletTenant
	readProto(t, testfixture.Request(f.http, "GET", "/v1/tenant/wallet", "", session.AccessToken), &currentWallet)
	readProto(t, testfixture.Request(f.http, "GET", "/v1/tenant/orders", "", session.AccessToken), &orders)
	if currentWallet.GetAvailableMinor() != 4500 || currentWallet.GetAvailableTrafficBytes() != 0 || len(orders.GetOrders()) != 0 {
		t.Fatalf("failed grant left a partial settlement: %v %v", &currentWallet, &orders)
	}
	execSQL(t, f.db, "ALTER TABLE traffic_grants DROP CONSTRAINT reject_test_grant")
	execSQL(t, f.db, "UPDATE tenant_balances SET available_bytes=-2000000000 WHERE tenant_id=?", customer.TenantId)
	var proxyUser v1.Subaccount
	readProto(t, testfixture.Request(f.http, "POST", "/v1/tenant/subaccounts",
		`{"name":"test","password":"Secret456","active":true}`, session.AccessToken), &proxyUser)
	var initialAuth v1.AuthSnapshot
	authBody := fmt.Sprintf(`{"username":%q,"password":"Secret456"}`, customer.GetPrimaryAccountName()+".test")
	readProto(t, testfixture.Request(f.http, "POST", "/v1/gateway/auth-state/load", authBody, authToken), &initialAuth)
	if initialAuth.GetQuota().GetAvailableBytes() != -2_000_000_000 {
		t.Fatalf("expected debt before purchase: %v", &initialAuth)
	}

	var replies [2]*httptest.ResponseRecorder
	var group sync.WaitGroup
	for i := range replies {
		group.Go(func() { replies[i] = tryPurchase(purchase) })
	}
	group.Wait()
	var purchased v1.PurchaseOrder
	readProto(t, replies[0], &purchased)
	orderID := purchased.GetId()
	var concurrent v1.PurchaseOrder
	readProto(t, replies[1], &concurrent)
	items := purchased.GetItems()
	if orderID == 0 || !proto.Equal(&purchased, &concurrent) || purchased.GetStatus() != "succeeded" ||
		purchased.GetDebitedMinor() != 4500 || len(items) != 1 || items[0].GetQuantity() != 1 ||
		items[0].GetCapacityBytes() != 50_000_000_000 {
		t.Fatalf("concurrent submissions did not settle exactly one package: %v %v", &purchased, &concurrent)
	}
	testfixture.AssertStatus(t, tryPurchase(`{"package_id":"5gb","package_version":"2","request_key":"product-order"}`),
		http.StatusConflict, "WALLET_REQUEST_CONFLICT")
	var states v1.RefreshAuthStateResponse
	refreshBody := fmt.Sprintf(`{"references":[%q]}`, initialAuth.GetReference())
	readProto(t, testfixture.Request(f.http, "POST", "/v1/gateway/auth-state/refresh", refreshBody, authToken), &states)
	snapshots := states.GetSnapshots()
	if len(snapshots) != 1 || !snapshots[0].GetActive() || snapshots[0].GetQuota().GetAvailableBytes() != 48_000_000_000 ||
		snapshots[0].GetQuota().GetUsageVersion() <= initialAuth.GetQuota().GetUsageVersion() {
		t.Fatalf("gateway sync did not observe the newer positive balance: %v", &states)
	}
	var grants []struct{ PurchasedBytes, DebtOffsetBytes, RemainingBytes int64 }
	if err := f.db.Table("traffic_grants").Where("tenant_id = ?", customer.TenantId).Find(&grants).Error; err != nil ||
		len(grants) != 1 || grants[0].PurchasedBytes != 50_000_000_000 || grants[0].DebtOffsetBytes != 2_000_000_000 ||
		grants[0].RemainingBytes != 48_000_000_000 {
		t.Fatalf("incorrect permanent grant: %+v, %v", grants, err)
	}

	readProto(t, testfixture.Request(f.http, "PATCH", packages+"/50gb", `{"version":"2","capacity_bytes":"50000000000","price_minor":"4500","listed":false}`, cashier), &configured)
	if len(list().GetProducts()[0].GetPackages()) != 2 {
		t.Fatal("delisted package still visible")
	}
	testfixture.AssertStatus(t, tryPurchase(`{"package_id":"50gb","package_version":"2","request_key":"delisted"}`),
		http.StatusConflict, "PACKAGE_UNAVAILABLE")
	readProto(t, testfixture.Request(f.http, "PUT", sales+"static", open, cashier), &opened)
	if err := f.db.Transaction(func(tx *gorm.DB) error {
		if err := repo.RequireProductSale(ctx, tx, product.Static); err != nil {
			return err
		}
		_, err := repo.LockTrafficPackage(ctx, tx, "50gb", 2)
		return err
	}); err != product.ErrPackageUnavailable {
		t.Fatalf("purchase gate: %v", err)
	}

	readProto(t, testfixture.Request(f.http, "PUT", sales+"dynamic", `{"open":false}`, cashier), &opened)
	testfixture.AssertStatus(t, tryPurchase(`{"package_id":"5gb","package_version":"2","request_key":"closed-sale"}`),
		http.StatusConflict, "PRODUCT_SALE_CLOSED")
	products = list().GetProducts()
	if products[0].GetOpen() || len(products[0].GetPackages()) != 0 || !products[1].GetOpen() {
		t.Fatalf("unexpected product switches after closing dynamic: %v", products)
	}
	if err := f.db.Transaction(func(tx *gorm.DB) error {
		_, err := repo.LockTrafficPackage(ctx, tx, "5gb", 2)
		return err
	}); err != product.ErrSaleClosed {
		t.Fatalf("closed product purchase gate: %v", err)
	}
	var changes int64
	if err := f.db.Table("product_change_log").Where("operator_id = 'cashier' AND created_at IS NOT NULL AND before_value IS NOT NULL AND after_value IS NOT NULL").Count(&changes).Error; err != nil || changes != 7 {
		t.Fatalf("audit entries: %d, %v", changes, err)
	}
	var order v1.PurchaseOrder
	var replay v1.PurchaseOrder
	readProto(t, tryPurchase(purchase), &replay)
	if !proto.Equal(&purchased, &replay) {
		t.Fatalf("replay did not return the original order: %v", &replay)
	}
	readProto(t, testfixture.Request(f.http, "GET", fmt.Sprintf("/v1/tenant/orders/%d", orderID), "", session.AccessToken), &order)
	if order.GetLockedMinor() != 4500 || order.GetDeliveredBytes() != 50_000_000_000 {
		t.Fatalf("historical order changed after delisting: %v", &order)
	}
	var balance int64
	err := f.db.Table("tenant_balances").Select("available_bytes").Where("tenant_id = ?", customer.TenantId).Scan(&balance).Error
	if err != nil || balance != 48_000_000_000 {
		t.Fatalf("delisting changed delivered traffic: %d, %v", balance, err)
	}
}

func fundCatalogWallet(t *testing.T, db *gorm.DB, tenantID string) {
	t.Helper()
	ctx := context.Background()
	store := repo.NewWalletRepository(db)
	zero := int64(0)
	_, err := store.ApplyRecharge(ctx, wallet.Command{
		TenantID: tenantID, Key: "product-funding", Action: wallet.Recharge,
		Actor: "cashier", Reason: "product test", Amount: 4500, ExpectedBefore: &zero,
	})
	if err != nil {
		t.Fatal(err)
	}
}
