package integration

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"sync/atomic"
	"testing"
	"time"

	v1 "github.com/DuanJinTaoToday/proxy/api/proxycontrol/v1"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/integration/testfixture"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/data/acl/ipcrown"
)

func TestStaticRenewalPriceRecheckAndMixedSettlement(t *testing.T) {
	var unit atomic.Int32
	unit.Store(10)
	var renewals atomic.Int32
	var staleDetail atomic.Bool
	var lateDetail atomic.Bool
	item := func(id int, stop string) string {
		return fmt.Sprintf(`{"id":%d,"nid":3,"ip":"192.0.2.%d","port":43000,"p_user":"fixture","p_pass":"fixture","stoptime":%q}`, id, id, stop)
	}
	const original = "2099-01-01 12:00:00"
	const renewed = "2099-01-31 12:00:00"
	var firstRenewedExpiry atomic.Value
	firstRenewedExpiry.Store(renewed)
	supplier := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		switch req.URL.Path {
		case "/open/api/nodes-and-continents":
			_, _ = fmt.Fprintf(w, `{"code":0,"data":{"nodes":[{"id":3,"title":"东京","max":2,"prices":[{"price_type":"yp1","buytime":1,"price":%d}]}]}}`, unit.Load())
		case "/open/api/order/place":
			_, _ = fmt.Fprintf(w, `{"code":0,"data":{"order_id":82,"list":[%s,%s]}}`, item(83, original), item(84, original))
		case "/open/api/order/detail":
			expiry := original
			if renewals.Load() != 0 && !staleDetail.Load() {
				expiry = firstRenewedExpiry.Load().(string)
			}
			second := original
			if lateDetail.Load() {
				second = renewed
			}
			_, _ = fmt.Fprintf(w, `{"code":0,"data":{"order_id":82,"proxy_list":[%s,%s]}}`, item(83, expiry), item(84, second))
		case "/open/api/order/renew":
			var body struct {
				IDs  []int64
				Time string
			}
			if err := json.NewDecoder(req.Body).Decode(&body); err != nil ||
				len(body.IDs) != 2 || body.IDs[0] != 83 || body.IDs[1] != 84 || body.Time != "yp1" {
				t.Error("unexpected batch at supplier")
			}
			renewals.Add(1)
			_, _ = io.WriteString(w, `{"code":0,"data":[{"id":83,"stoptime":"2099-01-31 12:00:00"}]}`)
		default:
			w.WriteHeader(404)
		}
	}))
	defer supplier.Close()
	f := setupUsage(t, ipcrown.New(supplier.URL, "fixture", "fixture"))
	execSQL(t, f.db, "INSERT INTO operators (id, username, password_hash) VALUES ('cashier','cashier',?)", testfixture.PasswordHash)
	execSQL(t, f.db, "INSERT INTO operator_permissions (operator_id, permission) VALUES ('cashier','manage_billing')")
	cashier := testfixture.Login(t, f.http, "cashier")
	req := func(method, path, data, token string) *httptest.ResponseRecorder {
		return testfixture.Request(f.http, method, path, data, token)
	}
	const creds = `{"phone":"13800004901","password":"static-renewal-test"}`
	var tenant v1.TenantAccess
	var session v1.LoginResponse
	readProto(t, req("POST", "/v1/access/register", creds, ""), &tenant)
	readProto(t, req("POST", "/v1/access/login", creds, ""), &session)
	token := session.GetAccessToken()
	fundCatalogWallet(t, f.db, tenant.GetTenantId())
	var sale v1.Product
	const salePath = "/v1/operations/product-sales/static"
	readProto(t, req("PUT", salePath, `{"open":true,"quote_ready":true,"acceptance_evidence":"fixture"}`, cashier), &sale)
	var buyQuote v1.StaticPurchaseQuote
	const buySpec = `{"node_id":"3","term":"yp1","quantity":2}`
	readProto(t, req("POST", "/v1/tenant/static-purchase-quotes", buySpec, token), &buyQuote)
	var bought v1.PurchaseOrder
	readProto(t, req("POST", "/v1/tenant/static-orders", fmt.Sprintf(
		`{"node_id":"3","term":"yp1","quantity":2,"request_key":"buy","unit_minor":"1000","confirmation_token":%q}`, buyQuote.GetConfirmationToken()), token), &bought)
	if bought.GetStatus() != "succeeded" {
		t.Fatalf("fixture purchase status %s", bought.GetStatus())
	}
	readProto(t, req("PUT", salePath, `{"open":false}`, cashier), &sale)
	const quotePath = "/v1/tenant/static-renewal-quotes"
	const renewPath = "/v1/tenant/static-renewal-orders"
	const spec = `{"supplier_item_ids":["84","83"],"term":"yp1"}`
	var quote v1.StaticRenewalQuote
	readProto(t, req("POST", quotePath, spec, token), &quote)
	request := fmt.Sprintf(`{"supplier_item_ids":["84","83"],"term":"yp1","request_key":"renew","confirmation_token":%q}`, quote.GetConfirmationToken())
	unit.Store(12)
	testfixture.AssertStatus(t, req("POST", renewPath, request, token), http.StatusConflict, "STATIC_QUOTE_EXPIRED")
	unit.Store(10)
	if renewals.Load() != 0 {
		t.Fatal("the stale quote issued a supplier renewal")
	}
	staleDetail.Store(true)
	var order v1.PurchaseOrder
	readProto(t, req("POST", renewPath, request, token), &order)
	if order.GetStatus() != "pending" || order.GetUnsettledMinor() != 2000 {
		t.Fatalf("order did not wait for supplier detail: %v", &order)
	}
	testfixture.AssertStatus(t, req("POST", quotePath, spec, token), http.StatusConflict, "STATIC_RENEWAL_PENDING")
	// Move the first pending allocation past its original expiry, preserving the 30-day extension.
	expiredAt := time.Now().UTC().Truncate(time.Second).Add(-time.Hour)
	extendedAt := expiredAt.AddDate(0, 0, 30)
	firstRenewedExpiry.Store(extendedAt.In(time.FixedZone("CST", 8*60*60)).Format(time.DateTime))
	execSQL(t, f.db, "UPDATE static_ip_allocations SET expires_at=? WHERE supplier_item_id='83'", expiredAt)
	execSQL(t, f.db, "UPDATE static_renewal_items SET original_expiry=?, expected_expiry=? WHERE order_id=? AND supplier_item_id='83'", expiredAt, extendedAt, order.GetId())
	testfixture.AssertStatus(t, req("GET", "/v1/tenant/static-ips/83/proxy-config", "", token), http.StatusNotFound, "WALLET_RECORD_NOT_FOUND")
	staleDetail.Store(false)
	path := fmt.Sprintf("%s/%d/recheck", renewPath, order.GetId())
	readProto(t, req("POST", path, `{}`, token), &order)
	if order.GetStatus() != "pending" || order.GetDeliveredCount() != 1 || order.GetSettledMinor() != 1000 || order.GetUnsettledMinor() != 1000 {
		t.Fatalf("partial renewal settlement: %v", &order)
	}
	var ips v1.StaticIPs
	readProto(t, req("GET", "/v1/tenant/static-ips", "", token), &ips)
	if len(ips.GetAllocations()) != 2 || !slices.ContainsFunc(ips.GetAllocations(), func(ip *v1.StaticIP) bool {
		return ip.GetSupplierItemId() == "83" && ip.GetIp() == "192.0.2.83" && ip.GetExpiresAt().AsTime().Equal(extendedAt)
	}) {
		t.Fatalf("renewal did not update the original allocation: %v", &ips)
	}
	var restored v1.StaticIP
	readProto(t, req("GET", "/v1/tenant/static-ips/83/proxy-config", "", token), &restored)
	if restored.GetSupplierItemId() != "83" || !restored.GetExpiresAt().AsTime().Equal(extendedAt) {
		t.Fatalf("confirmed renewal did not restore the expired allocation")
	}
	var replay v1.PurchaseOrder
	readProto(t, req("POST", renewPath, request, token), &replay)
	if replay.GetId() != order.GetId() || renewals.Load() != 1 {
		t.Fatal("replay created another supplier renewal")
	}
	resolve := fmt.Sprintf("/v1/operations/tenants/%s/orders/%d/renewal-resolution", tenant.GetTenantId(), order.GetId())
	const close = `{"supplier_item_id":"84","evidence":"fixture unknown","close_unknown":true}`
	testfixture.AssertStatus(t, req("POST", resolve, close, cashier), http.StatusConflict, "WALLET_ORDER_STATE")
	execSQL(t, f.db, "UPDATE purchase_orders SET created_at=now()-interval '73 hours' WHERE id=?", order.GetId())
	var pending v1.PendingStaticPurchases
	readProto(t, req("GET", "/v1/operations/orders/static-pending", "", cashier), &pending)
	if len(pending.GetPurchases()) != 1 || !pending.Purchases[0].GetCanClose() ||
		len(pending.Purchases[0].GetPendingRenewalItems()) != 1 || pending.Purchases[0].PendingRenewalItems[0] != "84" {
		t.Fatalf("pending renewal item not exposed to operations: %v", &pending)
	}
	readProto(t, req("POST", resolve, close, cashier), &order)
	lateDetail.Store(true)
	readProto(t, req("POST", path, `{}`, token), &replay)
	if order.GetStatus() != "partial" || replay.GetReleasedMinor() != 1000 || replay.GetSettledMinor() != 1000 || renewals.Load() != 1 {
		t.Fatalf("close and replay did not preserve settlement: %v", &replay)
	}
	var wallet v1.WalletTenant
	readProto(t, req("GET", "/v1/tenant/wallet", "", token), &wallet)
	if wallet.GetAvailableMinor() != 1500 {
		t.Fatalf("unexpected wallet balance: %d", wallet.GetAvailableMinor())
	}
}
