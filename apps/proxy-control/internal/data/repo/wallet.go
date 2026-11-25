package repo

import (
	"context"
	"crypto/cipher"
	"database/sql"
	"fmt"

	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/biz/wallet"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/conf"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/data"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/data/acl/ipcrown"
	"github.com/go-kratos/kratos/v3/errors"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type WalletRepository struct {
	db           *gorm.DB
	static       wallet.StaticSupplier
	staticCipher cipher.AEAD
}

func ProvideWalletRepository(resources *data.Data, biz *conf.Biz) (*WalletRepository, error) {
	db := resources.UsageDB()
	if db == nil {
		db = resources.ProxyAuthDB()
	}
	configuration := biz.GetIpcrown()
	if configuration == nil {
		return NewWalletRepository(db), nil
	}
	supplier := ipcrown.New(configuration.GetBaseUrl(), configuration.GetPartnerId(), configuration.GetApiSecret())
	if supplier == nil && (configuration.GetBaseUrl() != "" || configuration.GetPartnerId() != "" || configuration.GetApiSecret() != "") {
		return nil, fmt.Errorf("invalid IPCrown base_url, partner_id or api_secret")
	}
	return NewWalletRepositoryWithStatic(db, supplier, configuration.GetPasswordKey())
}

func NewWalletRepository(db *gorm.DB) *WalletRepository {
	if db != nil {
		db = db.Session(&gorm.Session{Logger: logger.Discard})
	}
	return &WalletRepository{db: db}
}

var errWalletUnavailable = errors.ServiceUnavailable("WALLET_UNAVAILABLE", "wallet service unavailable")

func (r *WalletRepository) FindTenant(ctx context.Context, account, email string) (wallet.Tenant, error) {
	return r.walletTenant(ctx, func(db *gorm.DB) *gorm.DB {
		if email != "" {
			return db.Where("a.email = ?", email)
		}
		return db.Where("t.primary_account_name = ?", account)
	})
}

func (r *WalletRepository) Tenant(ctx context.Context, id string) (wallet.Tenant, error) {
	return r.walletTenant(ctx, func(db *gorm.DB) *gorm.DB { return db.Where("t.id = ?", id) })
}

func (r *WalletRepository) CustomerWallet(ctx context.Context, id string) (wallet.Tenant, error) {
	return r.walletTenant(ctx, func(db *gorm.DB) *gorm.DB {
		return db.Where("t.id = ?", id).Select(walletTenantColumns + `,
   COALESCE((SELECT SUM(e.after_minor - e.before_minor) FROM wallet_entries e
      WHERE e.tenant_id = w.tenant_id AND e.kind = 'recharge' AND e.before_minor <> e.after_minor), 0)::text AS total_recharge_minor,
   COALESCE((SELECT SUM(CASE WHEN status = 'pending' THEN (` + confirmedOrderMinor + `) - refunded_minor
      ELSE debited_minor - released_minor - refunded_minor END)
      FROM purchase_orders WHERE tenant_id = w.tenant_id), 0)::text AS total_spend_minor`)
	})
}

const walletTenantColumns = "t.id, t.name, t.primary_account_name, w.available_minor, b.available_bytes AS available_traffic_bytes, t.created_at AS registered_at, COALESCE(a.email, '') AS email"

func (r *WalletRepository) walletTenant(ctx context.Context, match func(*gorm.DB) *gorm.DB) (wallet.Tenant, error) {
	var tenant wallet.Tenant
	if r.db == nil {
		return tenant, errWalletUnavailable
	}
	db := r.db.WithContext(ctx).Table("wallets w").
		Select(walletTenantColumns).
		Joins("JOIN tenants t ON t.id=w.tenant_id").
		Joins("JOIN tenant_balances b ON b.tenant_id=w.tenant_id").
		Joins("LEFT JOIN tenant_admins a ON a.tenant_id=t.id")
	err := scanWalletRecord(match(db), &tenant)
	return tenant, err
}

func (r *WalletRepository) CreateRechargeQuote(ctx context.Context, quote wallet.RechargeQuote) error {
	if r.db == nil {
		return errWalletUnavailable
	}
	return r.db.WithContext(ctx).Exec(`INSERT INTO wallet_recharge_quotes
		(id, tenant_id, actor, request_key, amount_minor, reason, expected_before_minor, expires_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, quote.ID, quote.TenantID, quote.Actor, quote.RequestKey,
		quote.AmountMinor, quote.Reason, quote.ExpectedBeforeMinor, quote.ExpiresAt).Error
}

func (r *WalletRepository) RechargeQuote(ctx context.Context, tenantID, actor, key, id string) (wallet.RechargeQuote, error) {
	var quote wallet.RechargeQuote
	if r.db == nil {
		return quote, errWalletUnavailable
	}
	err := scanWalletRecord(r.db.WithContext(ctx).Table("wallet_recharge_quotes").
		Where("id = ? AND tenant_id = ? AND actor = ? AND request_key = ?", id, tenantID, actor, key), &quote)
	return quote, err
}
func walletEntries(db *gorm.DB, tenantID string) *gorm.DB {
	return db.Table("wallet_entries e").
		Select("e.id, e.order_id, e.before_minor, e.after_minor, e.kind, e.reason, e.actor, o.username AS operator_name, e.created_at").
		Joins("LEFT JOIN operators o ON o.id=e.actor").
		Where("e.tenant_id = ? AND e.before_minor <> e.after_minor", tenantID).
		Order("e.id DESC")
}

func (r *WalletRepository) Entries(ctx context.Context, tenantID string, f wallet.Filter) ([]wallet.Entry, error) {
	var entries []wallet.Entry
	if r.db == nil {
		return nil, errWalletUnavailable
	}
	db := walletEntries(r.db.WithContext(ctx), tenantID)
	switch wallet.Action(f.Kind) {
	case wallet.Debit:
		db = db.Where("e.kind IN ?", []wallet.Action{wallet.Debit, wallet.Hold})
	case wallet.Refund:
		db = db.Where("e.kind IN ?", []wallet.Action{wallet.Refund, wallet.Release})
	case "":
	default:
		db = db.Where("e.kind = ?", f.Kind)
	}
	if f.BeforeID > 0 {
		db = db.Where("e.id < ?", f.BeforeID)
	}
	err := filterCreatedAt(db, "e.created_at", f).Limit(f.Limit).Scan(&entries).Error
	return entries, err
}

// Convert inclusive bounds in Shanghai to an index-friendly half-open timestamp range.
func filterCreatedAt(db *gorm.DB, column string, f wallet.Filter) *gorm.DB {
	if f.StartDate != "" {
		db = db.Where(column+" >= (CAST(? AS date)::timestamp AT TIME ZONE 'Asia/Shanghai')", f.StartDate)
	}
	if f.EndDate != "" {
		db = db.Where(column+" < ((CAST(? AS date) + 1)::timestamp AT TIME ZONE 'Asia/Shanghai')", f.EndDate)
	}
	return db
}

func readOrders(db *gorm.DB, tenantID string, f wallet.Filter) ([]wallet.Order, error) {
	var orders []wallet.Order
	query := db.Table("purchase_orders").Select(orderColumns).Where("tenant_id = ?", tenantID)
	if f.Kind != "" {
		query = query.Where("kind = ?", f.Kind)
	}
	switch f.Product {
	case "dynamic":
		query = query.Where("kind = ?", wallet.TrafficPackage)
	case "static":
		query = query.Where("kind IN ?", []string{wallet.StaticPurchase, wallet.StaticRenewal})
	}
	if f.Status != "" {
		query = query.Where("status = ?", f.Status)
	}
	if f.OrderQuery != "" {
		query = query.Where("strpos(id::text, ?) > 0", f.OrderQuery)
	}
	if f.BeforeID > 0 {
		query = query.Where("id < ?", f.BeforeID)
	}
	err := filterCreatedAt(query, "created_at", f).Order("id DESC").Limit(f.Limit).Scan(&orders).Error
	return orders, err
}

func (r *WalletRepository) Orders(ctx context.Context, tenantID string, f wallet.Filter) (wallet.OrderPage, error) {
	var page wallet.OrderPage
	if r.db == nil {
		return page, errWalletUnavailable
	}
	db := r.db.WithContext(ctx)
	err := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Table("purchase_orders").Where("tenant_id = ? AND status = ?", tenantID, wallet.Pending).
			Count(&page.PendingCount).Error; err != nil {
			return err
		}
		var err error
		page.Orders, err = readOrders(tx, tenantID, f)
		if err != nil || len(page.Orders) == 0 {
			return err
		}
		return loadOrderSummaries(tx, tenantID, page.Orders)
	}, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	return page, err
}

func loadOrderSummaries(db *gorm.DB, tenantID string, orders []wallet.Order) error {
	ids := make([]int64, 0, len(orders))
	for _, order := range orders {
		ids = append(ids, order.ID)
	}
	var summaries []struct {
		wallet.OrderItem
		ItemCount int64
	}
	if err := db.Raw(`SELECT DISTINCT ON (order_id)
   order_id, position, specification, capacity_bytes, term_days, unit_minor, quantity, delivered_count,
   count(*) OVER (PARTITION BY order_id) AS item_count
   FROM order_items WHERE tenant_id = ? AND order_id IN ?
   ORDER BY order_id, position`, tenantID, ids).Scan(&summaries).Error; err != nil {
		return err
	}
	byOrder := make(map[int64]int, len(orders))
	for i, order := range orders {
		byOrder[order.ID] = i
	}
	for _, summary := range summaries {
		order := &orders[byOrder[summary.OrderID]]
		order.SummaryItem, order.ItemCount = &summary.OrderItem, summary.ItemCount
	}
	return nil
}

func (r *WalletRepository) Order(ctx context.Context, tenantID string, id int64) (wallet.Order, error) {
	return r.readOrder(ctx, tenantID, id, false)
}

func (r *WalletRepository) OrderDetail(ctx context.Context, tenantID string, id int64) (wallet.Order, error) {
	return r.readOrder(ctx, tenantID, id, true)
}

func (r *WalletRepository) readOrder(ctx context.Context, tenantID string, id int64, includeEntries bool) (wallet.Order, error) {
	if r.db == nil {
		return wallet.Order{}, errWalletUnavailable
	}
	var order wallet.Order
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var err error
		order, err = findPurchaseOrder(tx, tenantID, id, false)
		if err != nil || !includeEntries {
			return err
		}
		return walletEntries(tx, tenantID).Where("e.order_id = ?", id).Scan(&order.Entries).Error
	}, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	return order, err
}

const confirmedOrderMinor = `COALESCE((SELECT SUM(unit_minor * delivered_count)::bigint FROM order_items
  WHERE tenant_id = purchase_orders.tenant_id AND order_id = purchase_orders.id), 0)`

const orderColumns = `purchase_orders.*, CASE WHEN kind IN ('static_purchase', 'static_renewal') AND status = 'pending'
    THEN ` + confirmedOrderMinor + ` ELSE 0 END AS confirmed_minor`

func findPurchaseOrder(db *gorm.DB, tenantID string, id int64, lock bool) (wallet.Order, error) {
	var order wallet.Order
	query := "SELECT " + orderColumns + " FROM purchase_orders WHERE tenant_id = ? AND id = ?"
	if lock {
		query += " FOR UPDATE"
	}
	if err := scanWalletRecord(db.Raw(query, tenantID, id), &order); err != nil {
		return wallet.Order{}, err
	}
	items := db.Table("order_items AS i").Where("i.tenant_id = ? AND i.order_id = ?", tenantID, id)
	if order.Kind == wallet.StaticRenewal {
		items = items.Select("i.*, r.outcome").
			Joins("JOIN static_renewal_items r ON r.tenant_id=i.tenant_id AND r.order_id=i.order_id AND r.position=i.position")
	}
	err := items.Order("i.position").Scan(&order.Items).Error
	return order, err
}

func scanWalletRecord(db *gorm.DB, target any) error {
	result := db.Scan(target)
	if result.Error != nil {
		return fmt.Errorf("scan wallet record: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return wallet.ErrMissing
	}
	return nil
}
