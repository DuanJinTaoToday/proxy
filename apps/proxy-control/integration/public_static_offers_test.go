package integration

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	v1 "github.com/DuanJinTaoToday/proxy/api/proxycontrol/v1"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/integration/testfixture"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/data/acl/ipcrown"
)

func TestPublicStaticOfferPeakPreservesLiveCheckoutAndRenewal(t *testing.T) {
	var exclusiveCalls, sharedCalls, failedCalls, price atomic.Int32
	var renewed atomic.Bool
	price.Store(10)
	started, release := make(chan struct{}), make(chan struct{})
	supplier := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		switch req.URL.Path {
		case "/open/api/nodes-and-continents":
			var spec struct {
				CID int
				PID int
			}
			if err := json.NewDecoder(req.Body).Decode(&spec); err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			if spec.PID == 10 {
				failedCalls.Add(1)
				w.WriteHeader(http.StatusBadGateway)
				return
			}
			if spec.CID == 28 {
				if sharedCalls.Add(1) == 1 {
					close(started)
				}
				<-release
				_, _ = io.WriteString(w, `{"code":0,"data":{"nodes":[{"id":4,"title":"Singapore","isp":"共享","max":0,"prices":[{"price_type":"zp","buytime":0,"price":3}]}]}}`)
				return
			}
			exclusiveCalls.Add(1)
			_, _ = fmt.Fprintf(w, `{"code":0,"data":{"nodes":[{"id":3,"title":"Tokyo","isp":"独享","max":2,"prices":[{"price_type":"yp1","buytime":1,"price":%d}]}]}}`, price.Load())
		case "/open/api/order/place":
			_, _ = io.WriteString(w, `{"code":0,"data":{"order_id":32,"list":[{"id":33,"nid":3,"ip":"192.0.2.1","port":43000,"p_user":"customer","p_pass":"fixture","stoptime":"2099-01-01 12:00:00"}]}}`)
		case "/open/api/order/detail":
			expiry := "2099-01-01 12:00:00"
			if renewed.Load() {
				expiry = "2099-01-31 12:00:00"
			}
			_, _ = fmt.Fprintf(w, `{"code":0,"data":{"order_id":32,"proxy_list":[{"id":33,"nid":3,"ip":"192.0.2.1","port":43000,"p_user":"customer","p_pass":"fixture","stoptime":%q}]}}`, expiry)
		case "/open/api/order/renew":
			var body struct {
				IDs  []int64
				Time string
			}
			if err := json.NewDecoder(req.Body).Decode(&body); err != nil || len(body.IDs) != 1 || body.IDs[0] != 33 || body.Time != "yp1" {
				t.Errorf("unexpected supplier renewal request: %+v, %v", body, err)
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			renewed.Store(true)
			_, _ = io.WriteString(w, `{"code":0,"data":[{"id":33,"stoptime":"2099-01-31 12:00:00"}]}`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer supplier.Close()
	f := setupUsage(t, ipcrown.New(supplier.URL, "test", "fixture"))
	execSQL(t, f.db, "INSERT INTO operators (id, username, password_hash) VALUES ('cashier','cashier',?)", testfixture.PasswordHash)
	execSQL(t, f.db, "INSERT INTO operator_permissions (operator_id, permission) VALUES ('cashier','manage_billing')")
	cashier := testfixture.Login(t, f.http, "cashier")
	req := func(method, path, body, token string) *httptest.ResponseRecorder {
		return testfixture.Request(f.http, method, path, body, token)
	}
	var sale v1.Product
	readProto(t, req("PUT", "/v1/operations/product-sales/static",
		`{"open":true,"quote_ready":true,"acceptance_evidence":"fixture"}`, cashier), &sale)

	const catalog = "/v1/public/static-offers"
	var public v1.StaticOffers
	readProto(t, req("GET", catalog, "", ""), &public)
	if !public.GetOpen() || len(public.GetOffers()) != 1 || public.GetOffers()[0].GetUnitMinor() != 1000 {
		t.Fatalf("unexpected public snapshot: %v", &public)
	}

	const guestCount = 30
	shared := make(chan *httptest.ResponseRecorder, guestCount)
	for range guestCount {
		go func() {
			shared <- req("GET", catalog+"?share_mode=shared&purpose_id=8", "", "")
		}()
	}
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		close(release)
		t.Fatal("supplier did not receive shared catalog refresh")
	}
	// Give concurrent HTTP requests time to join the blocked supplier refresh.
	time.Sleep(100 * time.Millisecond)
	close(release)

	var snapshotAt time.Time
	for range guestCount {
		var offers v1.StaticOffers
		readProto(t, <-shared, &offers)
		if len(offers.GetOffers()) != 1 || offers.GetOffers()[0].GetAvailable() != 0 ||
			offers.GetOffers()[0].GetPurposeId() != 8 || offers.GetOffers()[0].GetTermDays() != 7 {
			t.Fatalf("shared catalog must preserve specs with zero stock: %v", &offers)
		}
		if snapshotAt.IsZero() {
			snapshotAt = offers.GetQuotedAt().AsTime()
		}
		if !snapshotAt.Equal(offers.GetQuotedAt().AsTime()) {
			t.Fatalf("concurrent guest did not receive the shared snapshot timestamp: %v", &offers)
		}
	}
	if sharedCalls.Load() != 1 {
		t.Fatalf("concurrent users issued %d requests for one specification", sharedCalls.Load())
	}

	const unavailable = "?share_mode=shared&purpose_id=10"
	for range 22 {
		testfixture.AssertStatus(t, req("GET", catalog+unavailable, "", ""), http.StatusServiceUnavailable, "STATIC_SUPPLIER_UNAVAILABLE")
	}
	if failedCalls.Load() != 18 {
		t.Fatalf("expected 18 failed refreshes and four local limit rejections, got %d supplier calls", failedCalls.Load())
	}
	testfixture.AssertStatus(t, req("GET", catalog+"?share_mode=shared&purpose_id=9", "", ""), http.StatusServiceUnavailable, "STATIC_SUPPLIER_UNAVAILABLE")
	if sharedCalls.Load() != 1 {
		t.Fatal("budget exhaustion reached supplier via another specification")
	}

	const credentials = `{"email":"tenant-13800004908@example.test","password":"catalog-test-password"}`
	var tenant v1.TenantAccess
	readProto(t, registerWithEmail(t, f, credentials), &tenant)
	var session v1.LoginResponse
	readProto(t, req("POST", "/v1/access/login", credentials, ""), &session)
	fundCatalogWallet(t, f.db, tenant.GetTenantId())
	token := session.GetAccessToken()
	const buySpec = `{"node_id":"3","term":"yp1","quantity":1}`
	price.Store(12)
	var buy v1.StaticPurchaseQuote
	readProto(t, req("POST", "/v1/tenant/static-purchase-quotes", buySpec, token), &buy)
	if buy.GetOffer().GetUnitMinor() != 1200 {
		t.Fatalf("purchase quote did not use live price: %v", buy.GetOffer())
	}
	place := fmt.Sprintf(`{"node_id":"3","term":"yp1","quantity":1,"unit_minor":"1200","request_key":"verify-catalog","confirmation_token":%q}`, buy.GetConfirmationToken())
	var order v1.PurchaseOrder
	readProto(t, req("POST", "/v1/tenant/static-orders", place, token), &order)
	if order.GetStatus() != "succeeded" || order.GetSettledMinor() != 1200 {
		t.Fatalf("purchase under public peak did not settle at live prices: %v", &order)
	}
	var resources v1.StaticIPs
	readProto(t, req("GET", "/v1/tenant/static-ips", "", token), &resources)
	allocations := resources.GetAllocations()
	if len(allocations) != 1 || allocations[0].GetOrderId() != order.GetId() || allocations[0].GetAllocationId() == "" {
		t.Fatalf("purchase did not return a platform allocation: %v", &resources)
	}
	allocationID := allocations[0].GetAllocationId()
	price.Store(14)
	var renew v1.StaticRenewalQuote
	const renewalQuote = "/v1/tenant/static-renewal-quotes"
	const renewalOrder = "/v1/tenant/static-renewal-orders"
	renewalSpec := fmt.Sprintf(`{"allocation_ids":[%q],"term":"yp1"}`, allocationID)
	renewalRequest := func(confirmation string) string {
		return fmt.Sprintf(`{"allocation_ids":[%q],"term":"yp1","request_key":"verify-catalog-renew","confirmation_token":%q}`, allocationID, confirmation)
	}
	readProto(t, req("POST", renewalQuote, renewalSpec, token), &renew)
	if len(renew.GetLines()) != 1 || renew.GetLines()[0].GetUnitMinor() != 1400 || exclusiveCalls.Load() != 4 {
		t.Fatalf("renewal quote did not use live price: %v, calls=%d", &renew, exclusiveCalls.Load())
	}
	price.Store(15)
	testfixture.AssertStatus(t, req("POST", renewalOrder, renewalRequest(renew.GetConfirmationToken()), token), http.StatusConflict, "STATIC_QUOTE_EXPIRED")
	if renewed.Load() {
		t.Fatal("stale renewal quote reached supplier")
	}
	readProto(t, req("POST", renewalQuote, renewalSpec, token), &renew)
	readProto(t, req("POST", renewalOrder, renewalRequest(renew.GetConfirmationToken()), token), &order)
	if order.GetStatus() != "succeeded" || order.GetSettledMinor() != 1500 || exclusiveCalls.Load() != 7 {
		t.Fatalf("renewal under public peak did not settle at live price: %v, calls=%d", &order, exclusiveCalls.Load())
	}
	var publicAfter v1.StaticOffers
	readProto(t, req("GET", catalog, "", ""), &publicAfter)
	if publicAfter.GetOffers()[0].GetUnitMinor() != 1000 || !publicAfter.GetQuotedAt().AsTime().Equal(public.GetQuotedAt().AsTime()) {
		t.Fatalf("private quote replaced guest snapshot: %v", &publicAfter)
	}
}
