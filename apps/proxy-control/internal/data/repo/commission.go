package repo

import (
	"context"
	"database/sql"

	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/biz/wallet"
	"gorm.io/gorm"
)

const commissionRateColumns = "id AS version, first_purchase_bps, repeat_purchase_bps, COALESCE(operator_id, '') AS operator_id, created_at AS updated_at"
const currentCommissionRates = `SELECT v.id AS version, v.first_purchase_bps, v.repeat_purchase_bps,
	COALESCE(v.operator_id, '') AS operator_id, v.created_at AS updated_at
	FROM commission_rate_settings s JOIN commission_rate_versions v ON v.id=s.version_id WHERE s.id=1`

func (r *WalletRepository) CommissionRates(ctx context.Context) (wallet.CommissionRates, error) {
	var rates wallet.CommissionRates
	if r.db == nil {
		return rates, errWalletUnavailable
	}
	err := scanWalletRecord(r.db.WithContext(ctx).Raw(currentCommissionRates), &rates)
	return rates, err
}

func (r *WalletRepository) SetCommissionRates(ctx context.Context, actor string, next wallet.CommissionRates) (wallet.CommissionRates, error) {
	var rates wallet.CommissionRates
	if r.db == nil {
		return rates, errWalletUnavailable
	}
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Lock the settings pointer before reading its version so a waiting writer
		// sees its predecessor's committed version.
		var versionID int64
		if err := scanWalletRecord(tx.Raw(`SELECT version_id FROM commission_rate_settings WHERE id=1 FOR UPDATE`), &versionID); err != nil {
			return err
		}
		if err := scanWalletRecord(tx.Raw("SELECT "+commissionRateColumns+" FROM commission_rate_versions WHERE id=?", versionID), &rates); err != nil {
			return err
		}
		if rates.FirstPurchaseBPS == next.FirstPurchaseBPS && rates.RepeatPurchaseBPS == next.RepeatPurchaseBPS {
			return nil
		}
		if err := scanWalletRecord(tx.Raw(`INSERT INTO commission_rate_versions
			(first_purchase_bps, repeat_purchase_bps, operator_id) VALUES (?, ?, ?) RETURNING `+commissionRateColumns,
			next.FirstPurchaseBPS, next.RepeatPurchaseBPS, actor), &rates); err != nil {
			return err
		}
		return tx.Exec("UPDATE commission_rate_settings SET version_id=? WHERE id=1", rates.Version).Error
	})
	return rates, err
}

func (r *WalletRepository) Commissions(ctx context.Context, tenantID string, filter wallet.CommissionFilter) (wallet.CommissionPage, error) {
	var page wallet.CommissionPage
	if r.db == nil {
		return page, errWalletUnavailable
	}
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := scanWalletRecord(tx.Raw(`SELECT COALESCE(c.available_minor, 0) AS available_minor
			FROM tenants t LEFT JOIN commission_accounts c ON c.tenant_id=t.id WHERE t.id=?`, tenantID), &page.AvailableMinor); err != nil {
			return err
		}
		query := tx.Table("referral_commissions").Select(`id, customer_key::text AS customer_key, referral_source, product_kind,
			settlement_sequence, net_debit_minor, commission_minor, rate_bps, rate_version, rule_version, promotion_paused, settled_at`).
			Where("referrer_tenant_id=?", tenantID)
		if filter.BeforeID > 0 {
			query = query.Where("id<?", filter.BeforeID)
		}
		if filter.CustomerKey != "" {
			query = query.Where("customer_key=?", filter.CustomerKey)
		}
		return query.Order("id DESC").Limit(filter.Limit).Scan(&page.Records).Error
	}, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	return page, err
}

func (r *WalletRepository) TransferCommission(ctx context.Context, command wallet.Command) (wallet.Entry, error) {
	if command.Action != wallet.CommissionTransfer {
		return wallet.Entry{}, wallet.ErrInvalid
	}
	if err := command.Validate(); err != nil {
		return wallet.Entry{}, err
	}
	return r.apply(ctx, command, command, nil, debitCommissionAccount, nil)
}

// apply holds the wallet lock and has checked for a replay before reaching the
// commission account. The balance predicate is rechecked after any row lock wait.
func debitCommissionAccount(tx *gorm.DB, command *wallet.Command) error {
	result := tx.Exec(`UPDATE commission_accounts SET available_minor=available_minor-?
		WHERE tenant_id=? AND available_minor>=?`, command.Amount, command.TenantID, command.Amount)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return wallet.ErrCommissionBalance
	}
	return nil
}
