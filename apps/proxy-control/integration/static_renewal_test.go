package integration

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"strings"
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
	var purchases atomic.Int32
	var otherRenewed atomic.Bool
	var staleDetail atomic.Bool
	var lateDetail atomic.Bool
	var rejectSingle atomic.Bool
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
			if purchases.Add(1) == 2 {
				_, _ = fmt.Fprintf(w, `{"code":0,"data":{"order_id":91,"list":[%s]}}`, item(92, original))
				return
			}
			_, _ = fmt.Fprintf(w, `{"code":0,"data":{"order_id":82,"list":[%s,%s]}}`, item(83, original), item(84, original))
		case "/open/api/order/detail":
			var body struct {
				OrderID string `json:"order_id"`
			}
			if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
				t.Errorf("could not decode supplier detail request: %v", err)
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			if body.OrderID == "91" {
				expiry := original
				if otherRenewed.Load() {
					expiry = renewed
				}
				_, _ = fmt.Fprintf(w, `{"code":0,"data":{"order_id":91,"proxy_list":[%s]}}`, item(92, expiry))
				return
			}
			if body.OrderID != "82" {
				t.Errorf("unexpected supplier order detail request: order ID %q", body.OrderID)
				w.WriteHeader(http.StatusBadRequest)
				return
			}
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
			if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
				t.Error("invalid supplier renewal request")
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			if slices.Equal(body.IDs, []int64{92}) && body.Time == "yp1" {
				otherRenewed.Store(true)
				_, _ = fmt.Fprintf(w, `{"code":0,"data":[{"id":92,"stoptime":%q}]}`, renewed)
				return
			}
			if rejectSingle.Load() {
				if !slices.Equal(body.IDs, []int64{84}) || body.Time != "yp1" {
					t.Error("unexpected single request at supplier")
				}
				_, _ = io.WriteString(w, `{"code":-4,"data":[]}`)
				return
			}
			if !slices.Equal(body.IDs, []int64{83, 84}) || body.Time != "yp1" {
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
	const creds = `{"email":"tenant-13800004901@example.test","password":"static-renewal-test"}`
	var tenant v1.TenantAccess
	var session v1.LoginResponse
	readProto(t, registerWithEmail(t, f, creds), &tenant)
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
	const otherCreds = `{"email":"tenant-13800004902@example.test","password":"other-renewal-test"}`
	var other v1.TenantAccess
	var otherSession v1.LoginResponse
	readProto(t, registerWithEmail(t, f, otherCreds), &other)
	readProto(t, req("POST", "/v1/access/login", otherCreds, ""), &otherSession)
	otherToken := otherSession.GetAccessToken()
	fundCatalogWallet(t, f.db, other.GetTenantId())
	const otherBuySpec = `{"node_id":"3","term":"yp1","quantity":1}`
	readProto(t, req("POST", "/v1/tenant/static-purchase-quotes", otherBuySpec, otherToken), &buyQuote)
	var otherBought v1.PurchaseOrder
	readProto(t, req("POST", "/v1/tenant/static-orders", fmt.Sprintf(
		`{"node_id":"3","term":"yp1","quantity":1,"request_key":"buy","unit_minor":"1000","confirmation_token":%q}`, buyQuote.GetConfirmationToken()), otherToken), &otherBought)
	if otherBought.GetStatus() != "succeeded" {
		t.Fatalf("second fixture purchase status %s", otherBought.GetStatus())
	}

	migration, err := os.ReadFile("../migrations/00019_static_allocation_ids.sql")
	if err != nil {
		t.Fatal(err)
	}
	up, down, ok := strings.Cut(string(migration), "-- +goose Down")
	if !ok {
		t.Fatal("cutover migration is missing its rollback")
	}
	// Drop the new schema after delivery to recreate the v18 data at the cutover.
	execSQL(t, f.db, down)
	execSQL(t, f.db, up)

	readProto(t, req("PUT", salePath, `{"open":false}`, cashier), &sale)
	const termsPath = "/v1/tenant/static-renewal-terms"
	const quotePath = "/v1/tenant/static-renewal-quotes"
	const renewPath = "/v1/tenant/static-renewal-orders"
	var before v1.StaticIPs
	readProto(t, req("GET", "/v1/tenant/static-ips", "", token), &before)
	allocationIDs := make(map[string]string)
	for _, ip := range before.GetAllocations() {
		if ip.GetOrderId() != bought.GetId() || ip.GetAllocationId() == "" {
			t.Fatalf("unexpected purchased allocation: %v", ip)
		}
		allocationIDs[ip.GetIp()] = ip.GetAllocationId()
	}
	first, second := allocationIDs["192.0.2.83"], allocationIDs["192.0.2.84"]
	if first == "" || second == "" || first == second {
		t.Fatalf("missing distinct platform IDs for batch: %v", allocationIDs)
	}
	var otherAllocations v1.StaticIPs
	readProto(t, req("GET", "/v1/tenant/static-ips", "", otherToken), &otherAllocations)
	if len(otherAllocations.GetAllocations()) != 1 || otherAllocations.Allocations[0].GetOrderId() != otherBought.GetId() ||
		otherAllocations.Allocations[0].GetIp() != "192.0.2.92" || !strings.HasPrefix(otherAllocations.Allocations[0].GetAllocationId(), "sa_") {
		t.Fatalf("the cutover did not preserve the second tenant allocation: %v", &otherAllocations)
	}
	otherID := otherAllocations.Allocations[0].GetAllocationId()
	var migratedConfig v1.StaticIP
	readProto(t, req("GET", "/v1/tenant/static-ips/"+otherID+"/proxy-config", "", otherToken), &migratedConfig)
	if migratedConfig.GetAllocationId() != otherID || migratedConfig.GetUsername() != "fixture" || migratedConfig.GetPassword() != "fixture" {
		t.Fatal("the cutover did not preserve the encrypted credentials")
	}
	testfixture.AssertStatus(t, req("GET", "/v1/tenant/static-ips/"+otherID+"/proxy-config", "", token), http.StatusNotFound, "WALLET_RECORD_NOT_FOUND")
	testfixture.AssertStatus(t, req("POST", "/v1/tenant/static-ips:export", fmt.Sprintf(`{"allocation_ids":[%q,%q]}`, first, otherID), token), http.StatusNotFound, "WALLET_RECORD_NOT_FOUND")
	testfixture.AssertStatus(t, req("POST", quotePath, fmt.Sprintf(`{"allocation_ids":[%q,%q],"term":"yp1"}`, first, otherID), token), http.StatusNotFound, "WALLET_RECORD_NOT_FOUND")

	spec := fmt.Sprintf(`{"allocation_ids":[%q,%q],"term":"yp1"}`, second, first)
	testfixture.AssertStatus(t, req("POST", quotePath, `{"supplier_item_ids":["84"],"term":"yp1"}`, token), http.StatusBadRequest, "INVALID_STATIC_RENEWAL")
	testfixture.AssertStatus(t, req("POST", quotePath, `{"allocation_ids":["84"],"term":"yp1"}`, token), http.StatusNotFound, "WALLET_RECORD_NOT_FOUND")
	testfixture.AssertStatus(t, req("POST", termsPath, `{"supplier_item_ids":["84"]}`, token), http.StatusBadRequest, "INVALID_STATIC_RENEWAL")
	testfixture.AssertStatus(t, req("POST", termsPath, fmt.Sprintf(`{"allocation_ids":[%q]}`, first), otherToken), http.StatusNotFound, "WALLET_RECORD_NOT_FOUND")
	var terms v1.StaticRenewalTerms
	readProto(t, req("POST", termsPath, fmt.Sprintf(`{"allocation_ids":[%q,%q]}`, first, second), token), &terms)
	if len(terms.GetTerms()) != 1 || terms.Terms[0].GetTerm() != "yp1" {
		t.Fatalf("batch allocations did not receive their available renewal terms: %v", &terms)
	}
	var quote v1.StaticRenewalQuote
	readProto(t, req("POST", quotePath, spec, token), &quote)
	request := fmt.Sprintf(`{"allocation_ids":[%q,%q],"term":"yp1","request_key":"renew","confirmation_token":%q}`, second, first, quote.GetConfirmationToken())
	if len(quote.GetLines()) != 2 || !slices.ContainsFunc(quote.GetLines(), func(line *v1.StaticRenewalQuoteLine) bool {
		return line.GetAllocationId() == first && line.GetIp() == "192.0.2.83"
	}) || !slices.ContainsFunc(quote.GetLines(), func(line *v1.StaticRenewalQuoteLine) bool {
		return line.GetAllocationId() == second && line.GetIp() == "192.0.2.84"
	}) {
		t.Fatalf("renewal quote lost allocation identity: %v", &quote)
	}
	testfixture.AssertStatus(t, req("POST", renewPath, request, otherToken), http.StatusConflict, "STATIC_QUOTE_EXPIRED")
	legacyRequest := fmt.Sprintf(`{"supplier_item_ids":["83","84"],"term":"yp1","request_key":"legacy","confirmation_token":%q}`, quote.GetConfirmationToken())
	testfixture.AssertStatus(t, req("POST", renewPath, legacyRequest, token), http.StatusBadRequest, "INVALID_STATIC_RENEWAL")
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
	execSQL(t, f.db, "UPDATE static_renewal_items SET original_expiry=?, expected_expiry=? WHERE order_id=? AND allocation_id=?", expiredAt, extendedAt, order.GetId(), first)
	testfixture.AssertStatus(t, req("GET", "/v1/tenant/static-ips/"+first+"/proxy-config", "", token), http.StatusNotFound, "WALLET_RECORD_NOT_FOUND")
	staleDetail.Store(false)
	path := fmt.Sprintf("%s/%d/recheck", renewPath, order.GetId())
	readProto(t, req("POST", path, `{}`, token), &order)
	if order.GetStatus() != "pending" || order.GetDeliveredCount() != 1 || order.GetSettledMinor() != 1000 || order.GetUnsettledMinor() != 1000 {
		t.Fatalf("partial renewal settlement: %v", &order)
	}
	var ips v1.StaticIPs
	readProto(t, req("GET", "/v1/tenant/static-ips", "", token), &ips)
	if len(ips.GetAllocations()) != 2 || !slices.ContainsFunc(ips.GetAllocations(), func(ip *v1.StaticIP) bool {
		return ip.GetAllocationId() == first && ip.GetIp() == "192.0.2.83" && ip.GetExpiresAt().AsTime().Equal(extendedAt)
	}) {
		t.Fatalf("renewal did not update the original allocation: %v", &ips)
	}
	var restored v1.StaticIP
	readProto(t, req("GET", "/v1/tenant/static-ips/"+first+"/proxy-config", "", token), &restored)
	if restored.GetAllocationId() != first || !restored.GetExpiresAt().AsTime().Equal(extendedAt) {
		t.Fatalf("confirmed renewal did not restore the expired allocation")
	}
	var replay v1.PurchaseOrder
	readProto(t, req("POST", renewPath, request, token), &replay)
	if replay.GetId() != order.GetId() || renewals.Load() != 1 {
		t.Fatal("replay created another supplier renewal")
	}
	resolve := fmt.Sprintf("/v1/operations/tenants/%s/orders/%d/renewal-resolution", tenant.GetTenantId(), order.GetId())
	close := fmt.Sprintf(`{"allocation_id":%q,"evidence":"fixture unknown","close_unknown":true}`, second)
	testfixture.AssertStatus(t, req("POST", resolve, close, cashier), http.StatusConflict, "WALLET_ORDER_STATE")
	execSQL(t, f.db, "UPDATE purchase_orders SET created_at=now()-interval '73 hours' WHERE id=?", order.GetId())
	var pending v1.PendingStaticPurchases
	readProto(t, req("GET", "/v1/operations/orders/static-pending", "", cashier), &pending)
	if len(pending.GetPurchases()) != 1 || !pending.Purchases[0].GetCanClose() ||
		len(pending.Purchases[0].GetPendingRenewalItems()) != 1 || pending.Purchases[0].PendingRenewalItems[0] != second {
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

	lateDetail.Store(false)
	rejectSingle.Store(true)
	singleSpec := fmt.Sprintf(`{"allocation_ids":[%q],"term":"yp1"}`, second)
	readProto(t, req("POST", quotePath, singleSpec, token), &quote)
	rejectedRequest := fmt.Sprintf(`{"allocation_ids":[%q],"term":"yp1","request_key":"renew-rejected","confirmation_token":%q}`, second, quote.GetConfirmationToken())
	var rejected v1.PurchaseOrder
	readProto(t, req("POST", renewPath, rejectedRequest, token), &rejected)
	if rejected.GetStatus() != "failed" || rejected.GetReleasedMinor() != 1000 || rejected.GetRefundedMinor() != 1000 || rejected.GetSettledMinor() != 0 {
		t.Fatalf("explicit supplier rejection did not release the hold: %v", &rejected)
	}
	readProto(t, req("GET", "/v1/tenant/wallet", "", token), &wallet)
	if wallet.GetAvailableMinor() != 1500 {
		t.Fatalf("explicit supplier rejection changed wallet balance: %d", wallet.GetAvailableMinor())
	}

	otherSpec := fmt.Sprintf(`{"allocation_ids":[%q],"term":"yp1"}`, otherID)
	readProto(t, req("POST", quotePath, otherSpec, otherToken), &quote)
	otherRequest := fmt.Sprintf(`{"allocation_ids":[%q],"term":"yp1","request_key":"single-renew","confirmation_token":%q}`, otherID, quote.GetConfirmationToken())
	var otherOrder v1.PurchaseOrder
	readProto(t, req("POST", renewPath, otherRequest, otherToken), &otherOrder)
	if otherOrder.GetStatus() != "succeeded" || len(otherOrder.GetItems()) != 1 || otherOrder.Items[0].GetAllocationId() != otherID || !otherRenewed.Load() {
		t.Fatalf("second tenant single renewal did not settle: %v", &otherOrder)
	}
	var renewedConfig v1.StaticIP
	readProto(t, req("GET", "/v1/tenant/static-ips/"+otherID+"/proxy-config", "", otherToken), &renewedConfig)
	if renewedConfig.GetAllocationId() != otherID || renewedConfig.GetIp() != migratedConfig.GetIp() ||
		renewedConfig.GetPort() != migratedConfig.GetPort() || renewedConfig.GetUsername() != migratedConfig.GetUsername() ||
		renewedConfig.GetPassword() != migratedConfig.GetPassword() ||
		!renewedConfig.GetExpiresAt().AsTime().Equal(migratedConfig.GetExpiresAt().AsTime().AddDate(0, 0, 30)) {
		t.Fatalf("single renewal did not preserve the original allocation and connection details: %v", &renewedConfig)
	}
}
