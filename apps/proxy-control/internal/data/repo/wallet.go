package repo

import (
	"context"
	"crypto/cipher"
	"database/sql"
	"fmt"

	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/biz/wallet"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/conf"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/data"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/data/ipcrown"
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

func (r *WalletRepository) FindTenant(ctx context.Context, account, phone string) (wallet.Tenant, error) {
	return r.walletTenant(ctx, func(db *gorm.DB) *gorm.DB {
		if phone != "" {
			return db.Joins("JOIN tenant_admins a ON a.tenant_id=t.id").Where("a.phone_e164 = ?", phone)
		}
		return db.Where("t.primary_account_name = ?", account)
	})
}

func (r *WalletRepository) Tenant(ctx context.Context, id string) (wallet.Tenant, error) {
	return r.walletTenant(ctx, func(db *gorm.DB) *gorm.DB { return db.Where("t.id = ?", id) })
}

func (r *WalletRepository) walletTenant(ctx context.Context, match func(*gorm.DB) *gorm.DB) (wallet.Tenant, error) {
	var tenant wallet.Tenant
	if r.db == nil {
		return tenant, errWalletUnavailable
	}
	db := r.db.WithContext(ctx).Table("wallets w").
		Select("t.id, t.name, t.primary_account_name, w.available_minor, b.available_bytes AS available_traffic_bytes").
		Joins("JOIN tenants t ON t.id=w.tenant_id").
		Joins("JOIN tenant_balances b ON b.tenant_id=w.tenant_id")
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
func (r *WalletRepository) Entries(ctx context.Context, tenantID string, f wallet.Filter) ([]wallet.Entry, error) {
	var entries []wallet.Entry
	if r.db == nil {
		return nil, errWalletUnavailable
	}
	db := r.db.WithContext(ctx).Table("wallet_entries e").
		Select("e.id, e.order_id, e.before_minor, e.after_minor, e.kind, e.reason, e.actor, o.username AS operator_name, e.created_at").
		Joins("LEFT JOIN operators o ON o.id=e.actor").
		Where("e.tenant_id = ? AND e.before_minor <> e.after_minor", tenantID)
	if f.BeforeID > 0 {
		db = db.Where("e.id < ?", f.BeforeID)
	}
	err := db.Order("e.id DESC").Limit(f.Limit).Scan(&entries).Error
	return entries, err
}

func (r *WalletRepository) Orders(ctx context.Context, tenantID string, f wallet.Filter) ([]wallet.Order, error) {
	var orders []wallet.Order
	if r.db == nil {
		return nil, errWalletUnavailable
	}
	db := r.db.WithContext(ctx).Table("purchase_orders").Select(orderColumns).Where("tenant_id = ?", tenantID)
	if f.Kind != "" {
		db = db.Where("kind = ?", f.Kind)
	}
	if f.Status != "" {
		db = db.Where("status = ?", f.Status)
	}
	if f.BeforeID > 0 {
		db = db.Where("id < ?", f.BeforeID)
	}
	err := db.Order("id DESC").Limit(f.Limit).Scan(&orders).Error
	return orders, err
}

func (r *WalletRepository) Order(ctx context.Context, tenantID string, id int64) (wallet.Order, error) {
	if r.db == nil {
		return wallet.Order{}, errWalletUnavailable
	}
	var order wallet.Order
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var err error
		order, err = findPurchaseOrder(tx, tenantID, id, false)
		return err
	}, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	return order, err
}

const orderColumns = `purchase_orders.*, CASE WHEN kind = 'static_purchase' AND status = 'pending'
    THEN COALESCE((SELECT SUM(unit_minor * delivered_count)::bigint FROM order_items WHERE order_id = purchase_orders.id), 0)
    ELSE 0 END AS confirmed_minor`

func findPurchaseOrder(db *gorm.DB, tenantID string, id int64, lock bool) (wallet.Order, error) {
	var order wallet.Order
	query := "SELECT " + orderColumns + " FROM purchase_orders WHERE tenant_id = ? AND id = ?"
	if lock {
		query += " FOR UPDATE"
	}
	if err := scanWalletRecord(db.Raw(query, tenantID, id), &order); err != nil {
		return wallet.Order{}, err
	}
	err := db.Table("order_items").Where("tenant_id = ? AND order_id = ?", tenantID, id).
		Order("position").Scan(&order.Items).Error
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
