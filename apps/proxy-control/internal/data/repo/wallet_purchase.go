package repo

import (
	"context"
	"time"

	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/biz/wallet"
	"gorm.io/gorm"
)

func (r *WalletRepository) PurchasePackage(ctx context.Context, tenant string, purchase wallet.PackagePurchase) (wallet.Order, error) {
	command := wallet.Command{TenantID: tenant, Key: purchase.Key, Kind: wallet.TrafficPackage, Action: wallet.Debit}
	var capacity int64
	prepare := func(tx *gorm.DB, command *wallet.Command) error {
		pkg, err := LockTrafficPackage(ctx, tx, purchase.PackageID, purchase.Version)
		if err != nil {
			return err
		}
		capacity = pkg.CapacityBytes
		command.Amount = pkg.PriceMinor
		command.Items = []wallet.OrderItem{{
			Specification: pkg.ID, Reference: pkg.ID, CapacityBytes: capacity, UnitMinor: pkg.PriceMinor, Quantity: 1,
		}}
		return nil
	}
	receipt, err := r.apply(ctx, command, purchase, prepare, func(tx *gorm.DB, receipt wallet.Entry) error {
		return grantPurchasedTraffic(tx, tenant, receipt.OrderID, capacity, receipt.CreatedAt)
	})
	if err != nil {
		return wallet.Order{}, err
	}
	return r.Order(ctx, tenant, receipt.OrderID)
}

func grantPurchasedTraffic(tx *gorm.DB, tenant string, orderID, capacity int64, now time.Time) error {
	var before int64
	if err := scanWalletRecord(tx.Raw(`SELECT available_bytes FROM tenant_balances
		WHERE tenant_id = ? FOR UPDATE`, tenant), &before); err != nil {
		return err
	}
	offset := wallet.PurchasedTrafficDebtOffset(before, capacity)
	if err := tx.Exec(`UPDATE tenant_balances SET available_bytes = available_bytes + ?,
		usage_version = usage_version + 1, updated_at = ? WHERE tenant_id = ?`, capacity, now, tenant).Error; err != nil {
		return err
	}
	return tx.Exec(`INSERT INTO traffic_grants
		(tenant_id, order_id, order_position, purchased_bytes, debt_offset_bytes, remaining_bytes, granted_at)
		VALUES (?, ?, 1, ?, ?, ?, ?)`, tenant, orderID, capacity, offset, capacity-offset, now).Error
}
