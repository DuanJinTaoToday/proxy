package repo

import (
	"context"
	"errors"
	"math"
	"time"

	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/biz/wallet"
	"gorm.io/gorm"
)

const exchangeQueueAge = 15 * time.Minute

const operatorExchangeColumns = exchangeRecordColumns + `, tenant_id, supplier_order_id, supplier_item_id,
	original_ip AS old_ip, COALESCE(new_ip, '') AS new_ip, COALESCE(resolution, '') AS resolution,
	unknown_at, resolution_conflict, corrected,
	(resolution_conflict OR (status='unknown' AND unknown_at <= statement_timestamp() - interval '15 minutes')) AS in_queue`

const exchangeAuditColumns = `id, kind, COALESCE(idempotency_key,'') AS idempotency_key, COALESCE(request_key,'') AS request_key,
	COALESCE(actor_id,'') AS actor_id, COALESCE(evidence,'') AS evidence, COALESCE(outcome,'') AS outcome,
	COALESCE(supplier_record_id,'') AS supplier_record_id, COALESCE(observed_ip,'') AS observed_ip,
	quota_before, reserved_before, used_before, quota_after, reserved_after, used_after, created_at`

func (r *WalletRepository) OperatorExchangeQuota(ctx context.Context, tenant string) (wallet.ExchangeOperationsQuota, error) {
	var quota wallet.ExchangeOperationsQuota
	if r.db == nil {
		return quota, errWalletUnavailable
	}
	err := scanWalletRecord(r.db.WithContext(ctx).Raw(`SELECT COALESCE(q.quota,10) AS total,
		COALESCE(q.reserved,0) AS reserved, COALESCE(q.used,0) AS used FROM tenants t
		LEFT JOIN static_ip_exchange_quotas q ON q.tenant_id=t.id WHERE t.id=?`, tenant), &quota)
	return quota, err
}

func (r *WalletRepository) OperatorExchangeHistory(ctx context.Context, filter wallet.ExchangeOperationsFilter, queue bool) (wallet.ExchangeOperationsPage, error) {
	var page wallet.ExchangeOperationsPage
	if r.db == nil {
		return page, errWalletUnavailable
	}
	if filter.TenantID != "" {
		if _, err := r.OperatorExchangeQuota(ctx, filter.TenantID); err != nil {
			return page, err
		}
	}
	query := r.db.WithContext(ctx).Table("static_ip_exchanges").Select(operatorExchangeColumns)
	if filter.TenantID != "" {
		query = query.Where("tenant_id=?", filter.TenantID)
	}
	if queue {
		query = query.Where("resolution_conflict OR (status='unknown' AND unknown_at <= statement_timestamp() - (? * interval '1 second'))",
			int(exchangeQueueAge/time.Second))
	}
	if !filter.BeforeTime.IsZero() {
		query = query.Where("(created_at, tenant_id, idempotency_key) < (?, ?, ?)", filter.BeforeTime, filter.BeforeTenant, filter.BeforeKey)
	}
	err := query.Order("created_at DESC, tenant_id DESC, idempotency_key DESC").
		Limit(wallet.PageSize + 1).Scan(&page.Records).Error
	return page, err
}

func (r *WalletRepository) OperatorExchangeAudit(ctx context.Context, tenant string, before int64) ([]wallet.ExchangeOperationsAudit, error) {
	if _, err := r.OperatorExchangeQuota(ctx, tenant); err != nil {
		return nil, err
	}
	var audits []wallet.ExchangeOperationsAudit
	query := r.db.WithContext(ctx).Table("static_ip_exchange_audit").Select(exchangeAuditColumns).Where("tenant_id=?", tenant)
	if before != 0 {
		query = query.Where("id < ?", before)
	}
	err := query.Order("id DESC").Limit(wallet.PageSize + 1).Scan(&audits).Error
	return audits, err
}

func (r *WalletRepository) AdjustExchangeQuota(ctx context.Context, actor string, cmd wallet.ExchangeQuotaAdjustment) (wallet.ExchangeOperationsAudit, error) {
	var audit wallet.ExchangeOperationsAudit
	if r.db == nil {
		return audit, errWalletUnavailable
	}
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec(`INSERT INTO static_ip_exchange_quotas (tenant_id)
			SELECT id FROM tenants WHERE id=? ON CONFLICT DO NOTHING`, cmd.TenantID).Error; err != nil {
			return err
		}
		before, err := lockedExchangeQuota(tx, cmd.TenantID)
		if err != nil {
			return err
		}
		audit, err = exchangeAuditByRequest(tx, cmd.TenantID, cmd.RequestKey)
		if err == nil {
			if audit.Kind == "adjustment" && audit.ActorID == actor && audit.Evidence == cmd.Evidence &&
				int64(audit.QuotaAfter)-int64(audit.QuotaBefore) == int64(cmd.Delta) {
				return nil
			}
			return wallet.ErrReplay
		}
		if !errors.Is(err, wallet.ErrMissing) {
			return err
		}
		afterTotal := int64(before.Total) + int64(cmd.Delta)
		if afterTotal > math.MaxInt32 || afterTotal < 0 ||
			(cmd.Delta < 0 && afterTotal < int64(before.Reserved)+int64(before.Used)) {
			return wallet.ErrExchangeQuotaAdjustment
		}
		after := before
		after.Total = int32(afterTotal)
		if err := storeExchangeQuota(tx, cmd.TenantID, after); err != nil {
			return err
		}
		audit, err = insertExchangeAudit(tx, cmd.TenantID, wallet.ExchangeOperationsAudit{
			Kind: "adjustment", ActorID: actor, RequestKey: cmd.RequestKey, Evidence: cmd.Evidence,
		}, before, after)
		return err
	})
	return audit, err
}

func lockedExchangeQuota(tx *gorm.DB, tenant string) (wallet.ExchangeOperationsQuota, error) {
	var quota wallet.ExchangeOperationsQuota
	err := scanWalletRecord(tx.Raw("SELECT quota AS total, reserved, used FROM static_ip_exchange_quotas WHERE tenant_id=? FOR UPDATE", tenant), &quota)
	return quota, err
}

func storeExchangeQuota(tx *gorm.DB, tenant string, quota wallet.ExchangeOperationsQuota) error {
	return tx.Exec("UPDATE static_ip_exchange_quotas SET quota=?, reserved=?, used=? WHERE tenant_id=?",
		quota.Total, quota.Reserved, quota.Used, tenant).Error
}

func exchangeAuditByRequest(db *gorm.DB, tenant, requestKey string) (wallet.ExchangeOperationsAudit, error) {
	var audit wallet.ExchangeOperationsAudit
	err := scanWalletRecord(db.Raw("SELECT "+exchangeAuditColumns+" FROM static_ip_exchange_audit WHERE tenant_id=? AND request_key=?",
		tenant, requestKey), &audit)
	return audit, err
}

func insertExchangeAudit(tx *gorm.DB, tenant string, audit wallet.ExchangeOperationsAudit, before, after wallet.ExchangeOperationsQuota) (wallet.ExchangeOperationsAudit, error) {
	var result wallet.ExchangeOperationsAudit
	err := scanWalletRecord(tx.Raw(`INSERT INTO static_ip_exchange_audit
		(tenant_id, idempotency_key, request_key, actor_id, kind, evidence, outcome, supplier_record_id, observed_ip,
		quota_before, reserved_before, used_before, quota_after, reserved_after, used_after)
		VALUES (?, NULLIF(?,''), NULLIF(?,''), NULLIF(?,''), ?, NULLIF(?,''), NULLIF(?,''), NULLIF(?,''), NULLIF(?,''), ?, ?, ?, ?, ?, ?)
		RETURNING `+exchangeAuditColumns, tenant, audit.IdempotencyKey, audit.RequestKey, audit.ActorID, audit.Kind, audit.Evidence,
		audit.Outcome, audit.SupplierRecordID, audit.ObservedIP, before.Total, before.Reserved, before.Used,
		after.Total, after.Reserved, after.Used), &result)
	return result, err
}
