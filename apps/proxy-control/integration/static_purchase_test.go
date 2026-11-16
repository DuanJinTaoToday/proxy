package integration

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	v1 "github.com/DuanJinTaoToday/proxy/api/proxycontrol/v1"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/integration/testfixture"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/data/ipcrown"
)

func TestStaticPurchaseAndIndependentSales(t *testing.T) {
	const item = `{"id":23,"nid":3,"ip":"192.0.2.1","port":43000,"p_user":"customer","p_pass":"fixture","stoptime":"2099-01-01 12:00:00"}`
	calls := make(chan string, 4)
	var supplierMode atomic.Int32
	supplier := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/open/api/nodes-and-continents":
			_, _ = io.WriteString(w, `{"code":0,"data":{"nodes":[{"id":3,"title":"东京","isp":"独享","max":2,"prices":[{"price_type":"yp1","buytime":1,"price":10}]}]}}`)
		case "/open/api/order/place":
			body, _ := io.ReadAll(r.Body)
			if r.Header.Get("X-Partner-ID") != "test" || !strings.Contains(string(body), `"list":[{"nid":3,"number":2}]`) {
				t.Errorf("incorrect supplier request: %s", body)
			}
			calls <- r.URL.Path
			switch supplierMode.Load() {
			case 1:
				_, _ = io.WriteString(w, `{"code":-5,"msg":"库存不足"}`)
				return
			case 2:
				w.WriteHeader(http.StatusGatewayTimeout)
				return
			}
			_, _ = fmt.Fprintf(w, `{"code":0,"data":{"order_id":22,"list":[%s]}}`, item)
		case "/open/api/order/detail":
			_, _ = fmt.Fprintf(w, `{"code":0,"data":{"order_id":22,"proxy_list":[%s]}}`, item)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer supplier.Close()
	f := setupUsage(t, ipcrown.New(supplier.URL, "test", "fixture-secret"))
	execSQL(t, f.db, "INSERT INTO operators (id, username, password_hash) VALUES ('cashier','cashier',?)", testfixture.PasswordHash)
	execSQL(t, f.db, "INSERT INTO operator_permissions (operator_id, permission) VALUES ('cashier','manage_billing')")
	cashier := testfixture.Login(t, f.http, "cashier")
	const credentials = `{"phone":"13800004801","password":"static-test-password"}`
	var tenant v1.TenantAccess
	readProto(t, testfixture.Request(f.http, "POST", "/v1/access/register", credentials, ""), &tenant)
	var session v1.LoginResponse
	readProto(t, testfixture.Request(f.http, "POST", "/v1/access/login", credentials, ""), &session)
	fundCatalogWallet(t, f.db, tenant.GetTenantId())

	const catalog = "/v1/products/static/offers"
	const pendingPath = "/v1/operations/static-purchases/pending"
	const sales = "/v1/operations/product-sales/"
	const open = `{"open":true,"quote_ready":true,"acceptance_evidence":"controlled integration fixture"}`
	const buy = "/v1/tenant/orders/static-ips"
	const request = `{"node_id":"3","term":"yp1","quantity":2,"unit_minor":"1000","request_key":"static-purchase"}`
	req := func(method, path, body, token string) *httptest.ResponseRecorder {
		return testfixture.Request(f.http, method, path, body, token)
	}
	walletBalance := func(want int64) {
		t.Helper()
		var balance v1.WalletTenant
		readProto(t, req("GET", "/v1/tenant/wallet", "", session.GetAccessToken()), &balance)
		if balance.GetAvailableMinor() != want {
			t.Fatalf("wallet balance %d, want %d", balance.GetAvailableMinor(), want)
		}
	}

	var configured v1.TrafficPackage
	readProto(t, req("PATCH", "/v1/operations/traffic-packages/5gb",
		`{"version":"1","capacity_bytes":"5000000000","price_minor":"500","listed":true}`, cashier), &configured)
	var sale v1.Product
	readProto(t, req("PUT", sales+"dynamic", open, cashier), &sale)
	testfixture.AssertStatus(t, req("GET", catalog, "", ""), http.StatusConflict, "PRODUCT_SALE_CLOSED")
	testfixture.AssertStatus(t, req("POST", buy, request, session.GetAccessToken()), http.StatusConflict, "PRODUCT_SALE_CLOSED")
	walletBalance(4500)
	var orders v1.PurchaseOrders
	readProto(t, req("GET", "/v1/tenant/orders", "", session.GetAccessToken()), &orders)
	if len(orders.GetOrders()) != 0 || len(calls) != 0 {
		t.Fatal("closed static sale created an order or called supplier purchase")
	}

	readProto(t, req("PUT", sales+"static", open, cashier), &sale)
	readProto(t, req("PUT", sales+"dynamic", `{"open":false}`, cashier), &sale)
	var products v1.Products
	readProto(t, req("GET", "/public/products", "", ""), &products)
	if len(products.GetProducts()) != 2 || products.Products[0].GetCanPurchase() ||
		len(products.Products[0].GetPackages()) != 0 || !products.Products[1].GetCanPurchase() {
		t.Fatalf("unexpected public product state: %v", &products)
	}
	testfixture.AssertStatus(t, req("POST", "/v1/tenant/orders/traffic-packages",
		`{"package_id":"5gb","package_version":"2","request_key":"closed-dynamic"}`, session.GetAccessToken()),
		http.StatusConflict, "PRODUCT_SALE_CLOSED")
	walletBalance(4500)

	var offers v1.StaticOffers
	readProto(t, req("GET", catalog, "", ""), &offers)
	if len(offers.GetOffers()) != 1 || offers.Offers[0].GetUnitMinor() != 1000 {
		t.Fatalf("unexpected supplier offers: %v", &offers)
	}
	price := req("POST", buy, strings.Replace(request, `"1000"`, `"1200"`, 1), session.GetAccessToken())
	testfixture.AssertStatus(t, price, http.StatusConflict, "STATIC_PRICE_CHANGED")
	if !strings.Contains(price.Body.String(), `"unit_minor":"1000"`) {
		t.Fatal("price confirmation did not reveal the current quote")
	}
	stock := req("POST", buy, strings.Replace(request, `"quantity":2`, `"quantity":3`, 1), session.GetAccessToken())
	testfixture.AssertStatus(t, stock, http.StatusConflict, "STATIC_STOCK_INSUFFICIENT")
	if !strings.Contains(stock.Body.String(), `"available":"2"`) {
		t.Fatal("stock error did not reveal the current quantity")
	}
	walletBalance(4500)
	var ledger v1.WalletEntries
	readProto(t, req("GET", "/v1/tenant/wallet/ledger", "", session.GetAccessToken()), &ledger)
	if len(ledger.GetEntries()) != 1 || ledger.Entries[0].GetKind() != "recharge" {
		t.Fatalf("rejected requests changed wallet ledger: %v", &ledger)
	}
	readProto(t, req("GET", "/v1/tenant/orders", "", session.GetAccessToken()), &orders)
	if len(orders.GetOrders()) != 0 || len(calls) != 0 {
		t.Fatal("rejected requests created an order or called supplier purchase")
	}

	var order v1.PurchaseOrder
	readProto(t, req("POST", buy, request, session.GetAccessToken()), &order)
	if order.GetStatus() != "pending" || order.GetDeliveredCount() != 1 ||
		order.GetSettledMinor() != 1000 || order.GetUnsettledMinor() != 1000 {
		t.Fatalf("partial delivery settlement: %v", &order)
	}
	walletBalance(2500)
	readProto(t, req("PUT", sales+"static", `{"open":false}`, cashier), &sale)
	readProto(t, req("POST", buy, request, session.GetAccessToken()), &order)
	if len(calls) != 1 {
		t.Fatalf("supplier was called %d times", len(calls))
	}
	var ips v1.StaticIPs
	readProto(t, req("GET", "/v1/tenant/static-ips", "", session.GetAccessToken()), &ips)
	if len(ips.GetAllocations()) != 1 || ips.Allocations[0].GetIp() != "192.0.2.1" || ips.Allocations[0].GetPassword() != "fixture" ||
		!strings.HasPrefix(ips.Allocations[0].GetExpiresAt().AsTime().String(), "2099-01-01 04:00:00") {
		t.Fatalf("partial delivery was not immediately usable: %v", &ips)
	}
	resolve := fmt.Sprintf("/v1/operations/tenants/%s/orders/%d/static-resolution", tenant.GetTenantId(), order.GetId())
	const closeOrder = `{"evidence":"supplier confirms the remaining IP is unknown","close_remaining":true}`
	testfixture.AssertStatus(t, req("POST", resolve, closeOrder, cashier), http.StatusNotFound, "WALLET_RECORD_NOT_FOUND")
	var queue v1.PendingStaticPurchases
	readProto(t, req("GET", pendingPath, "", cashier), &queue)
	if len(queue.GetPurchases()) != 0 {
		t.Fatalf("order was handed off before 24 hours: %v", &queue)
	}
	execSQL(t, f.db, "UPDATE purchase_orders SET created_at=now()-interval '25 hours' WHERE id=?", order.GetId())
	readProto(t, req("GET", pendingPath, "", cashier), &queue)
	if len(queue.GetPurchases()) != 1 || queue.Purchases[0].GetCanClose() || queue.Purchases[0].GetSupplierOrderId() != "22" {
		t.Fatalf("order was not queued for review at 24 hours: %v", &queue)
	}
	testfixture.AssertStatus(t, req("POST", resolve, closeOrder, cashier), http.StatusConflict, "WALLET_ORDER_STATE")
	execSQL(t, f.db, "UPDATE purchase_orders SET created_at=now()-interval '73 hours' WHERE id=?", order.GetId())
	readProto(t, req("GET", pendingPath, "", cashier), &queue)
	if len(queue.GetPurchases()) != 1 || !queue.Purchases[0].GetCanClose() {
		t.Fatalf("order was not eligible for closing at 72 hours: %v", &queue)
	}
	testfixture.AssertStatus(t, req("GET", pendingPath, "", session.GetAccessToken()), http.StatusForbidden, "ACCESS_DENIED")
	readProto(t, req("POST", resolve, closeOrder, cashier), &order)
	if order.GetStatus() != "partial" || order.GetDeliveredCount() != 1 || order.GetReleasedMinor() != 1000 {
		t.Fatalf("remaining hold was not released: %v", &order)
	}
	walletBalance(3500)

	readProto(t, req("PUT", sales+"static", open, cashier), &sale)
	supplierMode.Store(1)
	rejected := strings.Replace(request, "static-purchase", "stock-race", 1)
	readProto(t, req("POST", buy, rejected, session.GetAccessToken()), &order)
	if order.GetStatus() != "failed" || order.GetReleasedMinor() != 2000 {
		t.Fatalf("explicit supplier rejection did not return the whole hold: %v", &order)
	}
	walletBalance(3500)
	supplierMode.Store(2)
	unknown := strings.Replace(request, "static-purchase", "unknown-result", 1)
	readProto(t, req("POST", buy, unknown, session.GetAccessToken()), &order)
	if order.GetStatus() != "pending" || order.GetUnsettledMinor() != 2000 {
		t.Fatalf("unknown supplier result did not preserve the hold: %v", &order)
	}
	walletBalance(1500)
	var replay v1.PurchaseOrder
	readProto(t, req("POST", buy, unknown, session.GetAccessToken()), &replay)
	if replay.GetId() != order.GetId() || len(calls) != 3 {
		t.Fatal("replaying the unknown result submitted another supplier purchase")
	}
}
