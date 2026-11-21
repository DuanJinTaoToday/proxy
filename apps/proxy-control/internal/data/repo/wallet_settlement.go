package repo

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"

	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/biz/wallet"
	"gorm.io/gorm"
)

func (r *WalletRepository) ApplyRecharge(ctx context.Context, command wallet.Command) (wallet.Entry, error) {
	if command.Action != wallet.Recharge {
		return wallet.Entry{}, wallet.ErrInvalid
	}
	return r.Apply(ctx, command, nil)
}

// Apply serializes monetary operations on a tenant wallet. Products can write
// their delivery records using the supplied transaction and order ID. The
// callback must verify the expected state change and return any failure. A
// committed retry returns the original receipt without running fulfill again.
// Remote supplier calls belong outside this transaction.
func (r *WalletRepository) Apply(ctx context.Context, command wallet.Command, fulfill func(*gorm.DB, wallet.Entry) error) (wallet.Entry, error) {
	if err := command.Validate(); err != nil {
		return wallet.Entry{}, err
	}
	if command.Action == wallet.Debit && command.Kind == wallet.TrafficPackage && fulfill == nil {
		return wallet.Entry{}, wallet.ErrInvalid
	}
	return r.apply(ctx, command, command, nil, fulfill)
}

func (r *WalletRepository) apply(ctx context.Context, command wallet.Command, request any,
	prepare func(*gorm.DB, *wallet.Command) error, fulfill func(*gorm.DB, wallet.Entry) error,
) (wallet.Entry, error) {
	if r.db == nil {
		return wallet.Entry{}, errWalletUnavailable
	}
	fingerprint, err := walletFingerprint(request)
	if err != nil {
		return wallet.Entry{}, err
	}
	var receipt wallet.Entry
	err = r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var order wallet.Order
		if command.OrderID != 0 {
			var err error
			order, err = findPurchaseOrder(tx, command.TenantID, command.OrderID, true)
			if err != nil {
				return err
			}
		}
		var before int64
		if err := scanWalletRecord(tx.Raw("SELECT available_minor FROM wallets WHERE tenant_id = ? FOR UPDATE", command.TenantID), &before); err != nil {
			return err
		}
		previous, exists, err := walletReplay(tx, command.TenantID, command.Key, fingerprint)
		if err != nil {
			return err
		}
		if exists {
			receipt = previous
			return nil
		}
		if prepare != nil {
			if err := prepare(tx, &command); err != nil {
				return err
			}
			if err := command.Validate(); err != nil {
				return err
			}
		}
		receipt, err = applyWalletCommand(tx, command, order, before, fingerprint)
		if err != nil {
			return err
		}
		if fulfill != nil {
			return fulfill(tx, receipt)
		}
		return nil
	})
	if err != nil {
		return wallet.Entry{}, err
	}
	return receipt, nil
}

func applyWalletCommand(tx *gorm.DB, c wallet.Command, order wallet.Order, before int64, fingerprint string) (wallet.Entry, error) {
	settlement, err := c.Settle(before, order)
	if err != nil {
		return wallet.Entry{}, err
	}
	orderID, err := saveSettlementOrder(tx, c.TenantID, settlement.Order)
	if err != nil {
		return wallet.Entry{}, err
	}
	return saveWalletEntry(tx, c, orderID, before, settlement.AfterMinor, fingerprint)
}

func saveWalletEntry(tx *gorm.DB, c wallet.Command, orderID, before, after int64, fingerprint string) (wallet.Entry, error) {
	if before != after {
		if err := tx.Exec("UPDATE wallets SET available_minor = ? WHERE tenant_id = ?", after, c.TenantID).Error; err != nil {
			return wallet.Entry{}, err
		}
	}
	var receipt wallet.Entry
	err := scanWalletRecord(tx.Raw(`WITH entry AS (
		INSERT INTO wallet_entries (tenant_id, request_key, request_hash, order_id, kind, actor, reason, before_minor, after_minor)
		VALUES (?, ?, ?, NULLIF(?::bigint, 0), ?, NULLIF(?, ''), ?, ?, ?) RETURNING *
	) SELECT entry.*, o.username AS operator_name FROM entry LEFT JOIN operators o ON o.id=entry.actor`,
		c.TenantID, c.Key, fingerprint, orderID, c.Action, c.Actor, c.Reason, before, after), &receipt)
	return receipt, err
}

func saveSettlementOrder(tx *gorm.DB, tenantID string, order wallet.Order) (int64, error) {
	if order.ID != 0 {
		err := tx.Exec(`UPDATE purchase_orders
			SET status = ?, released_minor = ?, refunded_minor = ?, delivered_count = ?
			WHERE tenant_id = ? AND id = ?`,
			order.Status, order.ReleasedMinor, order.RefundedMinor, order.DeliveredCount, tenantID, order.ID).Error
		if err != nil {
			return 0, err
		}
		for _, item := range order.Items {
			if err := tx.Exec(`UPDATE order_items SET delivered_count = ?
				WHERE tenant_id = ? AND order_id = ? AND position = ?`,
				item.DeliveredCount, tenantID, order.ID, item.Position).Error; err != nil {
				return 0, err
			}
		}
		return order.ID, nil
	}
	if order.Kind == "" {
		return 0, nil
	}
	var id int64
	err := scanWalletRecord(tx.Raw(`INSERT INTO purchase_orders
		(tenant_id, kind, status, locked_minor, debited_minor, released_minor, refunded_minor, delivered_bytes, delivered_count)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?) RETURNING id`,
		tenantID, order.Kind, order.Status, order.LockedMinor, order.DebitedMinor,
		order.ReleasedMinor, order.RefundedMinor, order.DeliveredBytes, order.DeliveredCount), &id)
	if err != nil {
		return 0, err
	}
	for index, item := range order.Items {
		err := tx.Exec(`INSERT INTO order_items (tenant_id, order_id, position, specification, reference, capacity_bytes,
			term_days, unit_minor, quantity, delivered_count, static_region, static_ip_type)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			tenantID, id, index+1, item.Specification, item.Reference, item.CapacityBytes, item.TermDays,
			item.UnitMinor, item.Quantity, item.DeliveredCount, item.StaticRegion, item.StaticIPType).Error
		if err != nil {
			return 0, err
		}
	}
	return id, nil
}

func walletFingerprint(request any) (string, error) {
	body, err := json.Marshal(request)
	if err != nil {
		return "", err
	}
	hash := sha256.Sum256(body)
	return hex.EncodeToString(hash[:]), nil
}

func walletReplay(db *gorm.DB, tenant, key, fingerprint string) (wallet.Entry, bool, error) {
	var previous struct {
		wallet.Entry
		RequestHash string
	}
	result := db.Raw(`SELECT e.*, o.username AS operator_name FROM wallet_entries e
        LEFT JOIN operators o ON o.id=e.actor WHERE e.tenant_id = ? AND e.request_key = ?`, tenant, key).Scan(&previous)
	if result.Error != nil || result.RowsAffected == 0 {
		return wallet.Entry{}, false, result.Error
	}
	if previous.RequestHash != fingerprint {
		return wallet.Entry{}, false, wallet.ErrReplay
	}
	return previous.Entry, true, nil
}
