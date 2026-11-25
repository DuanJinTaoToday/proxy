package integration

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"sync"
	"testing"

	v1 "github.com/DuanJinTaoToday/proxy/api/proxycontrol/v1"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/integration/testfixture"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/biz/wallet"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/data/repo"
	kerrors "github.com/go-kratos/kratos/v3/errors"
	"google.golang.org/protobuf/proto"
	"gorm.io/gorm"
)

func TestWalletRechargeIsIdempotent(t *testing.T) {
	f := setupUsage(t)
	execSQL(t, f.db, "INSERT INTO tenants (id,name,primary_account_name) VALUES ('wallet-test','Wallet test','wallet-test')")
	execSQL(t, f.db, "INSERT INTO wallets (tenant_id) VALUES ('wallet-test')")
	store := repo.NewWalletRepository(f.db)
	ctx := context.Background()
	zero := int64(0)
	command := wallet.Command{TenantID: "wallet-test", Key: "recharge-1", Action: wallet.Recharge, Actor: "operator", Reason: "offline payment", Amount: 1000, ExpectedBefore: &zero}
	first, err := store.Apply(ctx, command, nil)
	if err != nil || first.BeforeMinor != 0 || first.AfterMinor != 1000 {
		t.Fatalf("recharge: %+v, %v", first, err)
	}
	retry, err := store.Apply(ctx, command, nil)
	if err != nil || retry.ID != first.ID {
		t.Fatalf("retry: %+v, %v", retry, err)
	}
}

func TestWalletSettlementAndTenantIsolation(t *testing.T) {
	f := setupUsage(t)
	store := repo.NewWalletRepository(f.db)
	ctx := context.Background()
	handler := f.http
	const customerPassword = "wallet-test-password"

	register := func(email string) (*v1.TenantAccess, string) {
		t.Helper()
		body := fmt.Sprintf(`{"email":%q,"password":%q}`, email, customerPassword)
		var customer v1.TenantAccess
		var login v1.LoginResponse
		readProto(t, registerWithEmail(t, f, body), &customer)
		readProto(t, testfixture.Request(handler, "POST", "/v1/access/login", body, ""), &login)
		return &customer, login.AccessToken
	}
	customer, token := register("tenant-13800002001@example.test")
	other, otherToken := register("tenant-13800002002@example.test")
	execSQL(t, f.db, "INSERT INTO operators (id,username,password_hash) VALUES ('cashier','cashier',?)", testfixture.PasswordHash)
	execSQL(t, f.db, "INSERT INTO operator_permissions (operator_id,permission) VALUES ('cashier','manage_billing')")
	operator := testfixture.Login(t, handler, "cashier")
	readOnly := testfixture.Login(t, handler, "operator")
	request := func(method, path, body, session string) *httptest.ResponseRecorder {
		return testfixture.Request(handler, method, path, body, session)
	}
	const ops = "/v1/operations/tenants"
	path := ops + "/" + customer.TenantId
	testfixture.AssertStatus(t, request("GET", ops+"?query=tenant-13800002001@example.test", "", operator), http.StatusForbidden, "ACCESS_DENIED")
	execSQL(t, f.db, "INSERT INTO operator_permissions (operator_id,permission) VALUES ('cashier','read_operations')")
	for _, query := range []string{customer.PrimaryAccountName, "tenant-13800002001@example.test", "TENANT-13800002001@EXAMPLE.TEST"} {
		var found v1.WalletTenant
		readProto(t, request("GET", ops+"?query="+url.QueryEscape(query), "", operator), &found)
		if found.TenantId != customer.TenantId || found.Name != customer.Name || found.AvailableMinor != 0 {
			t.Fatalf("wrong tenant lookup result: %s", &found)
		}
	}
	quote := func(session, body string) *httptest.ResponseRecorder {
		return request("POST", path+"/wallet/recharge-quotes", body, session)
	}
	makeQuote := func(key, reason string) *v1.WalletRechargeQuote {
		t.Helper()
		var result v1.WalletRechargeQuote
		readProto(t, quote(operator, fmt.Sprintf(`{"request_key":%q,"amount_minor":"1000","reason":%q}`, key, reason)), &result)
		return &result
	}
	recharge := func(session, body string) *httptest.ResponseRecorder {
		return request("POST", path+"/wallet/recharges", body, session)
	}
	submitQuote := func(key string, quote *v1.WalletRechargeQuote) string {
		return fmt.Sprintf(`{"request_key":%q,"confirmation_token":%q}`, key, quote.ConfirmationToken)
	}
	const invalid = `{"request_key":"manual-1","amount_minor":"1000","reason":"bank transfer"}`
	testfixture.AssertStatus(t, quote(readOnly, invalid), http.StatusForbidden, "ACCESS_DENIED")
	testfixture.AssertStatus(t, quote(operator, `{"request_key":"missing","amount_minor":"100","reason":"  "}`), http.StatusBadRequest, "INVALID_WALLET_REQUEST")
	testfixture.AssertStatus(t, recharge(operator, `{"request_key":"missing"}`), http.StatusBadRequest, "INVALID_WALLET_REQUEST")
	initial := makeQuote("manual-1", "bank transfer")
	stale := makeQuote("stale", "separate payment")
	if initial.GetTenant().GetName() != customer.Name || initial.GetTenant().GetPrimaryAccountName() != customer.PrimaryAccountName ||
		initial.ExpectedAfterMinor != 1000 || initial.AmountMinor != 1000 {
		t.Fatalf("wrong confirmation: %s", initial)
	}
	body := submitQuote("manual-1", initial)
	testfixture.AssertStatus(t, recharge(token, body), http.StatusForbidden, "ACCESS_DENIED")
	testfixture.AssertStatus(t, recharge(readOnly, body), http.StatusForbidden, "ACCESS_DENIED")
	testfixture.AssertStatus(t, recharge(operator, submitQuote("other-key", initial)), http.StatusNotFound, "WALLET_RECORD_NOT_FOUND")
	var first, retry v1.WalletEntry
	readProto(t, recharge(operator, body), &first)
	execSQL(t, f.db, "UPDATE wallet_recharge_quotes SET expires_at=now()-interval '1 second' WHERE id=?", initial.ConfirmationToken)
	readProto(t, recharge(testfixture.Login(t, handler, "cashier"), body), &retry)
	if first.Id == 0 || first.Id != retry.Id || first.BeforeMinor != 0 || first.AfterMinor != 1000 ||
		first.OperatorId != "cashier" || first.OperatorName != "cashier" || first.CreatedAt == nil || first.Reason != "bank transfer" {
		t.Fatalf("wrong recharge receipt: %s / %s", &first, &retry)
	}
	changed := makeQuote("manual-1", "different reason")
	testfixture.AssertStatus(t, recharge(operator, submitQuote("manual-1", changed)), http.StatusConflict, "WALLET_REQUEST_CONFLICT")
	testfixture.AssertStatus(t, recharge(operator, submitQuote("stale", stale)), http.StatusConflict, "WALLET_BALANCE_CHANGED")
	expired := makeQuote("expired", "expiry verification")
	execSQL(t, f.db, "UPDATE wallet_recharge_quotes SET expires_at=now()-interval '1 second' WHERE id=?", expired.ConfirmationToken)
	testfixture.AssertStatus(t, recharge(operator, submitQuote("expired", expired)), http.StatusConflict, "WALLET_QUOTE_EXPIRED")

	balance := func(session string) int64 {
		t.Helper()
		var result v1.WalletTenant
		readProto(t, request("GET", "/v1/tenant/wallet", "", session), &result)
		return result.AvailableMinor
	}
	checkBalance := func(want int64) {
		t.Helper()
		if got := balance(token); got != want {
			t.Fatalf("wallet balance = %d, want %d", got, want)
		}
	}
	buy := wallet.Command{
		TenantID: customer.TenantId, Key: "traffic-1", Action: wallet.Debit,
		Amount: 200, Kind: wallet.TrafficPackage,
		Items: []wallet.OrderItem{{Specification: "5 GB", CapacityBytes: 5_000_000_000, UnitMinor: 200, Quantity: 1}},
	}
	simulated := errors.New("delivery failed")
	credit := func(tx *gorm.DB, entry wallet.Entry) error {
		if entry.OrderID == 0 {
			return errors.New("missing traffic order ID")
		}
		result := tx.Exec("UPDATE tenant_balances SET available_bytes=available_bytes+5000000000 WHERE tenant_id=?", customer.TenantId)
		if result.Error == nil && result.RowsAffected != 1 {
			return errors.New("traffic account not updated")
		}
		return result.Error
	}
	if _, err := store.Apply(ctx, buy, nil); !errors.Is(err, wallet.ErrInvalid) {
		t.Fatalf("debit without delivery callback should fail: %v", err)
	}
	_, err := store.Apply(ctx, buy, func(tx *gorm.DB, entry wallet.Entry) error {
		if err := credit(tx, entry); err != nil {
			return err
		}
		return simulated
	})
	if !errors.Is(err, simulated) {
		t.Fatalf("expected fulfillment failure, got %v", err)
	}
	checkBalance(1000)
	var orders v1.PurchaseOrders
	readProto(t, request("GET", "/v1/tenant/orders", "", token), &orders)
	if len(orders.Orders) != 0 {
		t.Fatalf("failed command left an order: %s", &orders)
	}
	debit, err := store.Apply(ctx, buy, credit)
	if err != nil || debit.AfterMinor != 800 {
		t.Fatalf("traffic retry: %+v, %v", debit, err)
	}
	idempotent, err := store.Apply(ctx, buy, func(*gorm.DB, wallet.Entry) error { return simulated })
	if err != nil || idempotent.ID != debit.ID {
		t.Fatalf("committed replay reran delivery: %+v, %v", idempotent, err)
	}
	var bytes int64
	if err := f.db.Raw("SELECT available_bytes FROM tenant_balances WHERE tenant_id=?", customer.TenantId).Scan(&bytes).Error; err != nil || bytes != 5_000_000_000 {
		t.Fatalf("traffic credit = %d, error %v", bytes, err)
	}

	var held [2]wallet.Entry
	var failures [2]error
	var wg sync.WaitGroup
	for i := range held {
		wg.Go(func() {
			held[i], failures[i] = store.Apply(ctx, wallet.Command{
				TenantID: customer.TenantId, Key: fmt.Sprintf("hold-%d", i), Action: wallet.Hold, Amount: 600, Kind: wallet.StaticPurchase,
				Items: []wallet.OrderItem{{Specification: "住宅 IP", TermDays: 30, UnitMinor: 300, Quantity: 2}},
			}, nil)
		})
	}
	wg.Wait()
	winning := held[0]
	if failures[0] != nil {
		winning = held[1]
		failures[0], failures[1] = failures[1], failures[0]
	}
	if failures[0] != nil || winning.OrderID == 0 || kerrors.Reason(failures[1]) != "WALLET_INSUFFICIENT_FUNDS" ||
		kerrors.FromError(failures[1]).Metadata["shortfall_minor"] != "400" {
		t.Fatalf("unexpected concurrent holds: %+v, %v", held, failures)
	}
	checkBalance(200)
	var pendingCount v1.PurchaseOrders
	readProto(t, request("GET", "/v1/tenant/orders?status=succeeded", "", token), &pendingCount)
	if pendingCount.GetPendingCount() != 1 || len(pendingCount.GetOrders()) != 1 ||
		pendingCount.GetOrders()[0].GetId() != debit.OrderID {
		t.Fatalf("pending count should include orders excluded by the filters: %s", &pendingCount)
	}
	release := wallet.Command{
		TenantID: customer.TenantId, Key: "release-1", Action: wallet.Release, Amount: 300,
		OrderID: winning.OrderID, ItemDeliveries: []int32{1},
	}
	settled, err := store.Apply(ctx, release, nil)
	if err != nil || settled.AfterMinor != 500 {
		t.Fatalf("partial release: %+v, %v", settled, err)
	}
	release.Key = "release-2"
	if _, err := store.Apply(ctx, release, nil); !errors.Is(err, wallet.ErrState) {
		t.Fatalf("second release: %v", err)
	}

	for i := range failures {
		wg.Go(func() {
			_, failures[i] = store.Apply(ctx, wallet.Command{
				TenantID: customer.TenantId, Key: "refund-1", Action: wallet.Refund, Amount: 300, OrderID: winning.OrderID,
			}, nil)
		})
	}
	wg.Wait()
	if failures[0] != nil || failures[1] != nil {
		t.Fatalf("same refund key should replay successfully: %v", failures)
	}
	if _, err := store.Apply(ctx, wallet.Command{
		TenantID: customer.TenantId, Key: "refund-2", Action: wallet.Refund, Amount: 1, OrderID: winning.OrderID,
	}, nil); !errors.Is(err, wallet.ErrState) {
		t.Fatalf("over-refund should fail: %v", err)
	}
	checkBalance(800)
	var totals v1.WalletTenant
	readProto(t, request("GET", "/v1/tenant/wallet", "", token), &totals)
	if totals.GetTotalRechargeMinor() != "1000" || totals.GetTotalSpendMinor() != "200" {
		t.Fatalf("unexpected wallet totals after refunds: %s", &totals)
	}
	if got := balance(otherToken); got != 0 {
		t.Fatalf("unrelated wallet changed: %d", got)
	}

	orderPath := fmt.Sprintf("/orders/%d", winning.OrderID)
	var order v1.PurchaseOrder
	readProto(t, request("GET", "/v1/tenant"+orderPath, "", token), &order)
	if order.Status != wallet.Partial || order.LockedMinor != 600 || order.DebitedMinor != 600 ||
		order.ReleasedMinor != 300 || order.RefundedMinor != 600 || order.DeliveredCount != 1 ||
		order.PrepaidMinor != 600 || order.SettledMinor != 0 || order.UnsettledMinor != 0 ||
		len(order.Items) != 1 || order.Items[0].UnitMinor != 300 || order.Items[0].TermDays != 30 || order.Items[0].DeliveredCount != 1 {
		t.Fatalf("wrong order settlement snapshot: %s", &order)
	}
	entries := order.GetWalletEntries()
	if len(entries) != 3 || entries[0].GetKind() != string(wallet.Refund) ||
		entries[1].GetKind() != string(wallet.Release) || entries[2].GetKind() != string(wallet.Hold) {
		t.Fatalf("unexpected linked wallet entries: %v", entries)
	}
	testfixture.AssertStatus(t, request("GET", "/v1/tenant"+orderPath, "", otherToken), http.StatusNotFound, "WALLET_RECORD_NOT_FOUND")
	testfixture.AssertStatus(t, request("GET", ops+"/"+other.TenantId+orderPath, "", operator), http.StatusNotFound, "WALLET_RECORD_NOT_FOUND")
	testfixture.AssertStatus(t, request("GET", path+orderPath, "", token), http.StatusForbidden, "ACCESS_DENIED")
	var netSpend int64
	for _, filter := range []string{"kind=static_purchase&status=partial", "kind=traffic_package&status=succeeded", "kind=static_purchase&status=succeeded"} {
		var customerOrders, operatorOrders v1.PurchaseOrders
		readProto(t, request("GET", "/v1/tenant/orders?"+filter, "", token), &customerOrders)
		readProto(t, request("GET", path+"/orders?"+filter, "", readOnly), &operatorOrders)
		want := 1
		if filter == "kind=static_purchase&status=succeeded" {
			want = 0
		}
		if len(customerOrders.GetOrders()) != want {
			t.Fatalf("unexpected customer results for %s: %s", filter, &customerOrders)
		}
		for _, summary := range customerOrders.GetOrders() {
			netSpend += summary.GetSettledMinor()
			if summary.GetItemCount() != 1 || summary.GetSummaryItem().GetPosition() != 1 {
				t.Fatalf("missing locked summary for %s: %s", filter, summary)
			}
		}
		if !proto.Equal(&customerOrders, &operatorOrders) {
			t.Fatalf("unexpected results for %s: %s / %s", filter, &customerOrders, &operatorOrders)
		}
	}

	if totals.GetTotalSpendMinor() != strconv.FormatInt(netSpend, 10) {
		t.Fatalf("wallet spend %s differs from settled order totals %d", totals.GetTotalSpendMinor(), netSpend)
	}

	var ownEntries, otherEntries, auditEntries v1.WalletEntries
	readProto(t, request("GET", "/v1/tenant/wallet/ledger?tenant_id="+customer.TenantId, "", otherToken), &otherEntries)
	readProto(t, request("GET", "/v1/tenant/wallet/ledger", "", token), &ownEntries)
	readProto(t, request("GET", path+"/wallet/ledger", "", readOnly), &auditEntries)
	if len(otherEntries.Entries) != 0 || len(ownEntries.Entries) != 5 || len(auditEntries.Entries) != 5 ||
		ownEntries.Entries[0].OrderId != winning.OrderID || ownEntries.Entries[4].OperatorId != "" ||
		ownEntries.Entries[4].Reason != "" || auditEntries.Entries[4].OperatorId != "cashier" {
		t.Fatalf("unexpected wallet history: %s / %s / %s", &ownEntries, &otherEntries, &auditEntries)
	}
}
