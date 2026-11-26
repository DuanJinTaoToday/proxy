package repo

import (
	"context"
	"maps"
	"slices"

	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/biz/wallet"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func (r *WalletRepository) saveStaticRenewalOutcomes(ctx context.Context, tenant string, id int64, rows []staticRenewalItem,
	results map[string]string, actor string, resolution wallet.StaticRenewalResolution) (wallet.Order, error) {
	var updated wallet.Order
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		order, err := findPurchaseOrder(tx, tenant, id, true)
		if err != nil {
			return err
		}
		updated = order
		if order.Status != wallet.Pending {
			return nil
		}
		if err := lockStaticAllocations(tx, tenant, slices.Collect(maps.Keys(results))); err != nil {
			return err
		}
		var before int64
		if err := scanWalletRecord(tx.Raw("SELECT available_minor FROM wallets WHERE tenant_id=? FOR UPDATE", tenant), &before); err != nil {
			return err
		}
		for _, row := range rows {
			outcome, ok := results[row.AllocationID]
			if !ok {
				continue
			}
			changed := tx.Exec(`UPDATE static_renewal_items SET outcome=? WHERE tenant_id=? AND order_id=?
                AND position=? AND outcome='pending'`, outcome, tenant, id, row.Position)
			if changed.Error != nil {
				return changed.Error
			}
			if changed.RowsAffected == 0 {
				continue
			}
			if outcome == wallet.Succeeded {
				result := tx.Exec(`UPDATE static_ip_allocations SET expires_at=?
                    WHERE tenant_id=? AND allocation_id=? AND ip=? AND expires_at=?`,
					row.ExpectedExpiry, tenant, row.AllocationID, row.OriginalIP, row.OriginalExpiry)
				if result.Error != nil {
					return result.Error
				}
				if result.RowsAffected != 1 {
					return wallet.ErrState
				}
			}
			if actor != "" && row.AllocationID == resolution.ItemID {
				if err := tx.Exec(`INSERT INTO static_renewal_resolutions (tenant_id, order_id, position, operator_id, evidence)
                    VALUES (?, ?, ?, ?, ?)`, tenant, id, row.Position, actor, resolution.Evidence).Error; err != nil {
					return err
				}
			}
		}
		var outcomes []string
		if err := tx.Table("static_renewal_items").Where("tenant_id=? AND order_id=?", tenant, id).
			Order("position").Pluck("outcome", &outcomes).Error; err != nil {
			return err
		}
		settlement, err := order.SettleRenewal(before, outcomes)
		if err != nil {
			return err
		}
		if _, err := saveSettlementOrder(tx, tenant, settlement.Order); err != nil {
			return err
		}
		if settlement.AfterMinor != before {
			command := wallet.Command{TenantID: tenant, Key: uuid.NewString(), Action: wallet.Release, OrderID: id}
			fingerprint, err := walletFingerprint(command)
			if err != nil {
				return err
			}
			if _, err := saveWalletEntry(tx, command, id, before, settlement.AfterMinor, fingerprint); err != nil {
				return err
			}
		}
		updated, err = findPurchaseOrder(tx, tenant, id, false)
		return err
	})
	return updated, err
}
