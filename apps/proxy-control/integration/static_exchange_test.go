package integration

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	v1 "github.com/DuanJinTaoToday/proxy/api/proxycontrol/v1"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/integration/testfixture"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/data/acl/ipcrown"
)

func TestStaticExchangeInitialQuota(t *testing.T) {
	f := setupUsage(t)
	const credentials = `{"email":"exchange-initial@example.test","password":"static-exchange-password"}`
	registerWithEmail(t, f, credentials)
	var session v1.LoginResponse
	readProto(t, testfixture.Request(f.http, "POST", "/v1/access/login", credentials, ""), &session)
	const route = "/v1/tenant/static-ip-exchange-quota"
	quota := testfixture.Request(f.http, "GET", route, "", session.GetAccessToken())
	var counts map[string]int32
	if quota.Code != http.StatusOK || json.Unmarshal(quota.Body.Bytes(), &counts) != nil ||
		len(counts) != 3 || counts["remaining"] != 10 || counts["reserved"] != 0 || counts["used"] != 0 {
		t.Fatalf("initial tenant quota = %d %s", quota.Code, quota.Body.String())
	}
}

func TestStaticExchangeTenantQuotaAndRenewalConflicts(t *testing.T) {
	const (
		tenantCreds = `{"email":"exchange-race@example.test","password":"static-exchange-password"}`
		exchange    = "/v1/tenant/static-ip-exchanges"
		quota       = "/v1/tenant/static-ip-exchange-quota"
		renewQuote  = "/v1/tenant/static-renewal-quotes"
		renew       = "/v1/tenant/static-renewal-orders"
	)
	item := func(id int) string {
		return fmt.Sprintf(`{"id":%d,"nid":3,"proxyid":22,"ip":"192.0.2.%d","port":43000,"p_user":"client","p_pass":"fixture","stoptime":"2099-01-01 12:00:00"}`, id, id)
	}
	var purchases, exchanges atomic.Int32
	renewing := make(chan struct{})
	exchanging := make(chan int64, 2)
	releaseRenewal := make(chan struct{})
	releaseExchange := make(chan struct{})
	supplier := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		switch req.URL.Path {
		case "/open/api/nodes-and-continents":
			_, _ = io.WriteString(w, `{"code":0,"data":{"nodes":[{"id":3,"title":"Fixture","max":10,"prices":[{"price_type":"yp1","buytime":1,"price":10}]}]}}`)
		case "/open/api/order/place":
			id := 20 + int(purchases.Add(1))
			_, _ = fmt.Fprintf(w, `{"code":0,"data":{"order_id":%d,"list":[%s]}}`, id+100, item(id))
		case "/open/api/order/detail":
			var body struct {
				OrderID string `json:"order_id"`
			}
			if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
				t.Error("invalid supplier detail request")
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			id, _ := strconv.Atoi(body.OrderID)
			_, _ = fmt.Fprintf(w, `{"code":0,"data":{"order_id":%d,"proxy_list":[%s]}}`, id, item(id-100))
		case "/open/api/order/renew":
			close(renewing)
			select {
			case <-releaseRenewal:
				_, _ = io.WriteString(w, `{"code":-4}`)
			case <-req.Context().Done():
			}
		case "/open/api/order/exchange":
			var body struct {
				Type   string
				IDs    []int64
				Isrand int
				List   []struct {
					Node   int `json:"nid"`
					Number int
				}
			}
			if err := json.NewDecoder(req.Body).Decode(&body); err != nil || body.Type != "same" || len(body.IDs) != 1 ||
				len(body.List) != 1 || body.List[0].Node != 3 || body.List[0].Number != 1 || body.Isrand != 4 ||
				(body.IDs[0] != 21 && body.IDs[0] != 22) {
				t.Error("unexpected supplier exchange request")
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			if exchanges.Add(1) == 1 {
				_, _ = io.WriteString(w, `{"code":-5}`)
				return
			}
			exchanging <- body.IDs[0]
			select {
			case <-releaseExchange:
				w.WriteHeader(http.StatusGatewayTimeout)
				_, _ = io.WriteString(w, `{"code":-5}`)
			case <-req.Context().Done():
			}
		default:
			t.Errorf("unexpected supplier endpoint: %s", req.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer supplier.Close()
	f := setupUsageFixture(t, true, nil, "probe.example.test:80", ipcrown.New(supplier.URL, "fixture", "fixture"))
	execSQL(t, f.db, "INSERT INTO operators (id, username, password_hash) VALUES ('cashier','cashier',?)", testfixture.PasswordHash)
	execSQL(t, f.db, "INSERT INTO operator_permissions (operator_id, permission) VALUES ('cashier','manage_billing')")
	cashier := testfixture.Login(t, f.http, "cashier")
	var tenant v1.TenantAccess
	readProto(t, registerWithEmail(t, f, tenantCreds), &tenant)
	var login v1.LoginResponse
	readProto(t, testfixture.Request(f.http, "POST", "/v1/access/login", tenantCreds, ""), &login)
	token := login.GetAccessToken()
	fundCatalogWallet(t, f.db, tenant.GetTenantId())
	testfixture.AssertStatus(t, testfixture.Request(f.http, "PUT", "/v1/operations/product-sales/static",
		`{"open":true,"quote_ready":true,"acceptance_evidence":"local fixture"}`, cashier), http.StatusOK, "")
	req := func(method, path, body string) *httptest.ResponseRecorder {
		return testfixture.Request(f.http, method, path, body, token)
	}
	for _, key := range []string{"purchase-1", "purchase-2"} {
		var quote v1.StaticPurchaseQuote
		readProto(t, req("POST", "/v1/tenant/static-purchase-quotes", `{"node_id":"3","term":"yp1","quantity":1}`), &quote)
		var order v1.PurchaseOrder
		readProto(t, req("POST", "/v1/tenant/static-orders", fmt.Sprintf(
			`{"node_id":"3","term":"yp1","quantity":1,"unit_minor":"1000","request_key":%q,"confirmation_token":%q}`,
			key, quote.GetConfirmationToken())), &order)
		if order.GetStatus() != "succeeded" {
			t.Fatalf("fixture purchase = %s", order.GetStatus())
		}
	}
	var allocations v1.StaticIPs
	readProto(t, req("GET", "/v1/tenant/static-ips", ""), &allocations)
	ids := make(map[int64]string)
	for _, allocation := range allocations.GetAllocations() {
		switch allocation.GetIp() {
		case "192.0.2.21":
			ids[21] = allocation.GetAllocationId()
		case "192.0.2.22":
			ids[22] = allocation.GetAllocationId()
		}
	}
	if len(ids) != 2 || allocations.GetAllocations()[0].GetOrderId() == allocations.GetAllocations()[1].GetOrderId() {
		t.Fatal("exchange fixtures must belong to different orders")
	}
	execSQL(t, f.db, "INSERT INTO static_ip_exchange_quotas (tenant_id, quota) VALUES (?, 1)", tenant.GetTenantId())
	body := func(id int64, key string) string {
		return fmt.Sprintf(`{"node_id":"3","allocation_id":%q,"idempotency_key":%q}`, ids[id], key)
	}
	assertQuota := func(remaining, reserved int32) {
		t.Helper()
		var counts v1.StaticIPExchangeQuota
		readProto(t, req("GET", quota, ""), &counts)
		if counts.GetRemaining() != remaining || counts.GetReserved() != reserved || counts.GetUsed() != 0 {
			t.Fatalf("quota = %v, want ([%d remaining], [%d reserved], 0 used)", &counts, remaining, reserved)
		}
	}
	var denied v1.StaticIPExchange
	readProto(t, req("POST", exchange, body(21, "denied")), &denied)
	if denied.GetStatus() != "failed" || exchanges.Load() != 1 {
		t.Fatalf("explicit rejection = %v, %d supplier calls", &denied, exchanges.Load())
	}
	assertQuota(1, 0)

	var quote v1.StaticRenewalQuote
	readProto(t, req("POST", renewQuote, fmt.Sprintf(`{"allocation_ids":[%q],"term":"yp1"}`, ids[21])), &quote)
	renewReply := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		renewReply <- req("POST", renew, fmt.Sprintf(`{"allocation_ids":[%q],"term":"yp1","request_key":"renew-first","confirmation_token":%q}`,
			ids[21], quote.GetConfirmationToken()))
	}()
	select {
	case <-renewing:
	case <-time.After(5 * time.Second):
		t.Fatal("renewal never reached supplier")
	}
	testfixture.AssertStatus(t, req("POST", exchange, body(21, "while-renewing")), http.StatusConflict, "STATIC_RENEWAL_PENDING")
	close(releaseRenewal)
	var renewal v1.PurchaseOrder
	readProto(t, <-renewReply, &renewal)
	if renewal.GetStatus() != "failed" {
		t.Fatalf("supplier fixture did not settle rejected renewal: %s", renewal.GetStatus())
	}
	var balanceBefore v1.WalletTenant
	readProto(t, req("GET", "/v1/tenant/wallet", ""), &balanceBefore)

	start := make(chan struct{})
	replies := make(chan *httptest.ResponseRecorder, 2)
	for _, id := range []int64{21, 22} {
		go func() {
			<-start
			replies <- req("POST", exchange, body(id, fmt.Sprintf("competing-%d", id)))
		}()
	}
	close(start)
	var winner int64
	select {
	case winner = <-exchanging:
	case <-time.After(5 * time.Second):
		t.Fatal("no exchange reached supplier")
	}
	select {
	case reply := <-replies:
		testfixture.AssertStatus(t, reply, http.StatusConflict, "STATIC_EXCHANGE_QUOTA_EXHAUSTED")
	case <-time.After(5 * time.Second):
		t.Fatal("the competing order never received a quota conflict")
	}
	key := fmt.Sprintf("competing-%d", winner)
	var pending v1.StaticIPExchange
	readProto(t, req("POST", exchange, body(winner, key)), &pending)
	if pending.GetStatus() != "pending" {
		t.Fatalf("in-flight original request = %v", &pending)
	}
	assertQuota(0, 1)
	testfixture.AssertStatus(t, req("POST", exchange, body(43-winner, key)), http.StatusConflict, "WALLET_REQUEST_CONFLICT")
	renewBody := fmt.Sprintf(`{"allocation_ids":[%q],"term":"yp1"}`, ids[winner])
	testfixture.AssertStatus(t, req("POST", renewQuote, renewBody), http.StatusConflict, "STATIC_EXCHANGE_PENDING")

	close(releaseExchange)
	var lost, queried, retried v1.StaticIPExchange
	readProto(t, <-replies, &lost)
	readProto(t, req("GET", exchange+"/"+key, ""), &queried)
	readProto(t, req("POST", exchange, body(winner, key)), &retried)
	for _, record := range []*v1.StaticIPExchange{&lost, &queried, &retried} {
		if record.GetStatus() != "unknown" || record.GetIdempotencyKey() != key || record.GetAllocationId() != ids[winner] ||
			!record.GetCreatedAt().AsTime().Equal(pending.GetCreatedAt().AsTime()) {
			t.Fatalf("unexpected original request after lost response: %v", record)
		}
	}
	assertQuota(0, 1)
	var balanceAfter v1.WalletTenant
	readProto(t, req("GET", "/v1/tenant/wallet", ""), &balanceAfter)
	if exchanges.Load() != 2 || balanceBefore.GetAvailableMinor() != balanceAfter.GetAvailableMinor() {
		t.Fatalf("unexpected supplier attempts or wallet change: %d, %d to %d", exchanges.Load(), balanceBefore.GetAvailableMinor(), balanceAfter.GetAvailableMinor())
	}
}

func TestStaticExchangeActivatingConfigRenewalAndPartialSettlement(t *testing.T) {
	const (
		credentials = `{"email":"exchange-partial@example.test","password":"static-exchange-password"}`
		originalIP  = "192.0.2.41"
		replacedIP  = "192.0.2.81"
		expires     = "2099-01-01 12:00:00"
		extended    = "2099-01-31 12:00:00"
	)
	original := `{"id":41,"nid":3,"proxyid":500,"ip":"192.0.2.41","port":43000,"p_user":"old-user","p_pass":"old-password","stoptime":"2099-01-01 12:00:00"}`
	replacement := func(expiry string) string {
		return fmt.Sprintf(`{"id":41,"nid":3,"proxyid":501,"ip":%q,"port":43001,"p_user":"new-user","p_pass":"new-password","stoptime":%q}`, replacedIP, expiry)
	}
	var renewed, stalePurchaseDetail atomic.Bool
	var exchanges atomic.Int32
	supplier := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		switch req.URL.Path {
		case "/open/api/nodes-and-continents":
			_, _ = io.WriteString(w, `{"code":0,"data":{"nodes":[{"id":3,"title":"Fixture","max":10,"prices":[{"price_type":"yp1","buytime":1,"price":10}]}]}}`)
		case "/open/api/order/place":
			_, _ = fmt.Fprintf(w, `{"code":0,"data":{"order_id":141,"list":[%s]}}`, original)
		case "/open/api/order/detail":
			switch {
			case stalePurchaseDetail.Load():
				const additional = `{"id":42,"nid":3,"proxyid":502,"ip":"192.0.2.42","port":43000,"p_user":"fixture","p_pass":"fixture","stoptime":"2099-01-01 12:00:00"}`
				_, _ = fmt.Fprintf(w, `{"code":0,"data":{"order_id":141,"proxy_list":[%s,%s]}}`, original, additional)
			case renewed.Load():
				_, _ = fmt.Fprintf(w, `{"code":0,"data":{"order_id":141,"proxy_list":[%s]}}`, replacement(extended))
			default:
				_, _ = fmt.Fprintf(w, `{"code":0,"data":{"order_id":141,"proxy_list":[%s]}}`, replacement(expires))
			}
		case "/open/api/order/exchange":
			exchanges.Add(1)
			_, _ = fmt.Fprintf(w, `{"code":0,"data":[%s]}`, replacement(expires))
		case "/open/api/order/renew":
			var body struct {
				IDs  []int64
				Time string
			}
			if err := json.NewDecoder(req.Body).Decode(&body); err != nil || len(body.IDs) != 1 || body.IDs[0] != 41 || body.Time != "yp1" {
				t.Error("renewal did not use the original supplier item")
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			renewed.Store(true)
			_, _ = fmt.Fprintf(w, `{"code":0,"data":[{"id":41,"stoptime":%q}]}`, extended)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer supplier.Close()
	f := setupUsageFixture(t, true, nil, "probe.example.test:80", ipcrown.New(supplier.URL, "fixture", "fixture"))
	execSQL(t, f.db, "INSERT INTO operators (id, username, password_hash) VALUES ('cashier','cashier',?)", testfixture.PasswordHash)
	execSQL(t, f.db, "INSERT INTO operator_permissions (operator_id, permission) VALUES ('cashier','manage_billing')")
	cashier := testfixture.Login(t, f.http, "cashier")
	var tenant v1.TenantAccess
	readProto(t, registerWithEmail(t, f, credentials), &tenant)
	var login v1.LoginResponse
	readProto(t, testfixture.Request(f.http, "POST", "/v1/access/login", credentials, ""), &login)
	token := login.GetAccessToken()
	fundCatalogWallet(t, f.db, tenant.GetTenantId())
	testfixture.AssertStatus(t, testfixture.Request(f.http, "PUT", "/v1/operations/product-sales/static",
		`{"open":true,"quote_ready":true,"acceptance_evidence":"controlled fixture"}`, cashier), http.StatusOK, "")
	req := func(method, path, body string) *httptest.ResponseRecorder {
		return testfixture.Request(f.http, method, path, body, token)
	}

	var purchaseQuote v1.StaticPurchaseQuote
	readProto(t, req("POST", "/v1/tenant/static-purchase-quotes", `{"node_id":"3","term":"yp1","quantity":2}`), &purchaseQuote)
	var purchase v1.PurchaseOrder
	readProto(t, req("POST", "/v1/tenant/static-orders", fmt.Sprintf(
		`{"node_id":"3","term":"yp1","quantity":2,"unit_minor":"1000","request_key":"partial-purchase","confirmation_token":%q}`,
		purchaseQuote.GetConfirmationToken())), &purchase)
	if purchase.GetStatus() != "pending" || purchase.GetDeliveredCount() != 1 {
		t.Fatalf("expected a partial original purchase: %v", &purchase)
	}
	var allocations v1.StaticIPs
	readProto(t, req("GET", "/v1/tenant/static-ips", ""), &allocations)
	if len(allocations.GetAllocations()) != 1 {
		t.Fatalf("partial purchase allocations = %v", &allocations)
	}
	before := allocations.GetAllocations()[0]
	id := before.GetAllocationId()
	if before.GetIp() != originalIP || before.GetNodeId() != 3 || before.GetShareMode() != "exclusive" {
		t.Fatalf("unexpected initial platform allocation: %v", before)
	}
	var balanceBefore, balanceAfter v1.WalletTenant
	readProto(t, req("GET", "/v1/tenant/wallet", ""), &balanceBefore)

	const exchange = "/v1/tenant/static-ip-exchanges"
	exchangeBody := func(key string) string {
		return fmt.Sprintf(`{"allocation_id":%q,"node_id":"3","idempotency_key":%q}`, id, key)
	}
	var accepted v1.StaticIPExchange
	readProto(t, req("POST", exchange, exchangeBody("exchange-activating")), &accepted)
	if accepted.GetStatus() != "activating" {
		t.Fatalf("confirmed exchange should await the asynchronous probe: %v", &accepted)
	}
	testfixture.AssertStatus(t, req("POST", exchange, exchangeBody("second-while-activating")), http.StatusConflict, "STATIC_EXCHANGE_PENDING")
	configPath := "/v1/tenant/static-ips/" + id + "/proxy-config"
	var copied v1.StaticIP
	readProto(t, req("GET", configPath, ""), &copied)
	if copied.GetIp() != replacedIP || copied.GetPort() != 43001 || copied.GetUsername() != "new-user" ||
		copied.GetPassword() != "new-password" || copied.GetOrderId() != purchase.GetId() ||
		!copied.GetExpiresAt().AsTime().Equal(before.GetExpiresAt().AsTime()) {
		t.Fatalf("changed config was unavailable while activating: %v", &copied)
	}
	export := req("POST", "/v1/tenant/static-ips:export", fmt.Sprintf(`{"allocation_ids":[%q]}`, id))
	if export.Code != http.StatusOK || export.Body.String() != replacedIP+":43001:new-user:new-password\n" {
		t.Fatalf("export during activation = %d %q", export.Code, export.Body.String())
	}
	readProto(t, req("GET", "/v1/tenant/wallet", ""), &balanceAfter)
	if balanceBefore.GetAvailableMinor() != balanceAfter.GetAvailableMinor() || exchanges.Load() != 1 {
		t.Fatal("exchange charged the wallet or issued more than one supplier request")
	}

	var renewalQuote v1.StaticRenewalQuote
	readProto(t, req("POST", "/v1/tenant/static-renewal-quotes", fmt.Sprintf(`{"allocation_ids":[%q],"term":"yp1"}`, id)), &renewalQuote)
	var renewal v1.PurchaseOrder
	readProto(t, req("POST", "/v1/tenant/static-renewal-orders", fmt.Sprintf(
		`{"allocation_ids":[%q],"term":"yp1","request_key":"renew-activating","confirmation_token":%q}`,
		id, renewalQuote.GetConfirmationToken())), &renewal)
	if renewal.GetStatus() != "succeeded" || !renewed.Load() {
		t.Fatalf("renewal during activation did not settle: %v", &renewal)
	}

	// Make the already pending purchase eligible for operator reconciliation.
	execSQL(t, f.db, "UPDATE purchase_orders SET created_at=clock_timestamp()-interval '25 hours' WHERE id=?", purchase.GetId())
	stalePurchaseDetail.Store(true)
	var reconciled v1.PurchaseOrder
	resolve := fmt.Sprintf("/v1/operations/tenants/%s/orders/%d/static-resolution", tenant.GetTenantId(), purchase.GetId())
	readProto(t, testfixture.Request(f.http, "POST", resolve, `{"evidence":"fixture supplied the remaining item"}`, cashier), &reconciled)
	if reconciled.GetDeliveredCount() != 2 || reconciled.GetStatus() != "succeeded" || reconciled.GetUnsettledMinor() != 0 {
		t.Fatalf("remaining purchase did not settle: %v", &reconciled)
	}
	var after v1.StaticIP
	readProto(t, req("GET", configPath, ""), &after)
	if after.GetAllocationId() != id || after.GetIp() != replacedIP || after.GetPort() != 43001 ||
		after.GetUsername() != "new-user" || after.GetPassword() != "new-password" ||
		!after.GetExpiresAt().AsTime().Equal(before.GetExpiresAt().AsTime().Add(30*24*time.Hour)) {
		t.Fatalf("stale purchase receipt changed the replacement or its renewed expiry: %v", &after)
	}
	var quota v1.StaticIPExchangeQuota
	readProto(t, req("GET", "/v1/tenant/static-ip-exchange-quota", ""), &quota)
	if quota.GetUsed() != 1 || quota.GetReserved() != 0 || quota.GetRemaining() != 9 {
		t.Fatalf("renewal and purchase reconciliation affected exchange quota: %v", &quota)
	}
}
