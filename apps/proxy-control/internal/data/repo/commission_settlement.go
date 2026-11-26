package repo

import (
	"time"

	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/biz/wallet"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

const commissionRuleVersion = 1

// Call only for a new settlement with the buyer's wallet locked. Lock ordering
// is wallet, then commission account. Commission transfers follow the same order.
func recordOrderSettlement(tx *gorm.DB, buyer string, order wallet.Order) error {
	if order.Status == wallet.Pending {
		return nil
	}
	net := order.SettledMinor()
	if net == 0 {
		_, err := stampOrderSettlement(tx, buyer, order.ID)
		return err
	}

	var referral struct{ Referrer, Source string }
	result := tx.Raw(`SELECT referrer_tenant_id AS referrer, referral_source AS source
		FROM tenants WHERE id=? AND referrer_tenant_id IS NOT NULL`, buyer).Scan(&referral)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		_, err := stampOrderSettlement(tx, buyer, order.ID)
		return err
	}
	if err := ensurePromotion(tx, referral.Referrer); err != nil {
		return err
	}
	var paused bool
	if err := scanWalletRecord(tx.Raw("SELECT paused FROM tenant_promotions WHERE tenant_id=? FOR SHARE", referral.Referrer), &paused); err != nil {
		return err
	}
	var rates wallet.CommissionRates
	var versionID int64
	if err := scanWalletRecord(tx.Raw("SELECT version_id FROM commission_rate_settings WHERE id=1 FOR SHARE"), &versionID); err != nil {
		return err
	}
	if err := scanWalletRecord(tx.Raw("SELECT "+commissionRateColumns+" FROM commission_rate_versions WHERE id=?", versionID), &rates); err != nil {
		return err
	}
	var previous struct {
		SettlementSequence int64
		CustomerKey        string
	}
	if err := tx.Raw(`SELECT settlement_sequence, customer_key::text AS customer_key FROM referral_commissions
		WHERE buyer_tenant_id=? ORDER BY settlement_sequence DESC LIMIT 1`, buyer).Scan(&previous).Error; err != nil {
		return err
	}
	sequence := previous.SettlementSequence + 1
	customerKey := previous.CustomerKey
	if sequence == 1 {
		customerKey = uuid.NewString()
	}
	rate := rates.ForSettlement(sequence)
	if paused {
		rate = 0
	}
	amount := wallet.CommissionMinor(net, rate)

	if err := tx.Exec(`INSERT INTO commission_accounts AS account (tenant_id, available_minor) VALUES (?, ?)
		ON CONFLICT (tenant_id) DO UPDATE SET available_minor=account.available_minor+EXCLUDED.available_minor`,
		referral.Referrer, amount).Error; err != nil {
		return err
	}
	settledAt, err := stampOrderSettlement(tx, buyer, order.ID)
	if err != nil {
		return err
	}
	return tx.Exec(`INSERT INTO referral_commissions
		(order_id, buyer_tenant_id, referrer_tenant_id, customer_key, referral_source, product_kind, settlement_sequence,
		 net_debit_minor, commission_minor, rate_bps, rate_version, rule_version, promotion_paused, settled_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, order.ID, buyer, referral.Referrer, customerKey, referral.Source,
		order.Kind, sequence, net, amount, rate, rates.Version, commissionRuleVersion, paused, settledAt).Error
}

func stampOrderSettlement(tx *gorm.DB, buyer string, orderID int64) (time.Time, error) {
	var settledAt time.Time
	err := scanWalletRecord(tx.Raw(`UPDATE purchase_orders SET settled_at=clock_timestamp()
		WHERE tenant_id=? AND id=? AND settled_at IS NULL RETURNING settled_at`, buyer, orderID), &settledAt)
	return settledAt, err
}
