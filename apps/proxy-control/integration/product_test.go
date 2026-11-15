package integration

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	v1 "github.com/DuanJinTaoToday/proxy/api/proxycontrol/v1"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/integration/testfixture"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/biz/product"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/biz/wallet"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/data/repo"
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
	orderID := settleCatalogOrder(t, f.db, customer.TenantId)

	readProto(t, testfixture.Request(f.http, "PATCH", packages+"/50gb", `{"version":"2","capacity_bytes":"50000000000","price_minor":"4500","listed":false}`, cashier), &configured)
	if len(list().GetProducts()[0].GetPackages()) != 2 {
		t.Fatal("delisted package still visible")
	}
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
	readProto(t, testfixture.Request(f.http, "GET", fmt.Sprintf("/v1/tenant/orders/%d", orderID), "", session.AccessToken), &order)
	if order.GetLockedMinor() != 4500 || order.GetDeliveredBytes() != 50_000_000_000 {
		t.Fatalf("historical order changed after delisting: %v", &order)
	}
	var balance int64
	err := f.db.Table("tenant_balances").Select("available_bytes").Where("tenant_id = ?", customer.TenantId).Scan(&balance).Error
	if err != nil || balance != 50_000_000_000 {
		t.Fatalf("delisting changed delivered traffic: %d, %v", balance, err)
	}
}

func settleCatalogOrder(t *testing.T, db *gorm.DB, tenantID string) int64 {
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
	entry, err := store.Apply(ctx, wallet.Command{
		TenantID: tenantID, Key: "product-order", Action: wallet.Debit,
		Amount: 4500, Kind: wallet.TrafficPackage,
		Items: []wallet.OrderItem{{Specification: "50gb", CapacityBytes: 50_000_000_000, UnitMinor: 4500, Quantity: 1}},
	}, func(tx *gorm.DB, _ wallet.Entry) error {
		pkg, err := repo.LockTrafficPackage(ctx, tx, "50gb", 2)
		if err != nil {
			return err
		}
		return tx.Exec("UPDATE tenant_balances SET available_bytes = available_bytes + ? WHERE tenant_id = ?", pkg.CapacityBytes, tenantID).Error
	})
	if err != nil {
		t.Fatal(err)
	}
	return entry.OrderID
}
