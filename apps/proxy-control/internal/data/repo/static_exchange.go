package repo

import (
	"context"
	"errors"
	"log/slog"
	"net/netip"
	"strings"
	"time"

	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/biz/wallet"
	"gorm.io/gorm"
)

type exchangeAllocation struct {
	wallet.StaticAllocation
	OriginalOffer, SupplierOrderID string
	PasswordCiphertext             []byte
	Active                         bool
}

const exchangeRecordColumns = "allocation_id, node_id, idempotency_key, status, created_at, updated_at"

func (r *WalletRepository) StaticExchangeQuota(ctx context.Context, tenant string) (wallet.StaticExchangeQuota, error) {
	var quota wallet.StaticExchangeQuota
	if r.db == nil {
		return quota, errWalletUnavailable
	}
	result := r.db.WithContext(ctx).Raw(`SELECT CASE WHEN q.tenant_id IS NULL THEN 10 ELSE GREATEST(0, q.quota::bigint - q.reserved - q.used)::integer END AS remaining,
		COALESCE(q.reserved, 0) AS reserved, COALESCE(q.used, 0) AS used
		FROM tenants t LEFT JOIN static_ip_exchange_quotas q ON q.tenant_id=t.id WHERE t.id=?`, tenant)
	err := scanWalletRecord(result, &quota)
	return quota, err
}

func (r *WalletRepository) StaticExchange(ctx context.Context, tenant, key string) (wallet.StaticExchange, error) {
	if r.db == nil {
		return wallet.StaticExchange{}, errWalletUnavailable
	}
	return staticExchangeRecord(r.db.WithContext(ctx), tenant, key, false)
}

func staticExchangeRecord(db *gorm.DB, tenant, key string, lock bool) (wallet.StaticExchange, error) {
	var record wallet.StaticExchange
	query := "SELECT " + exchangeRecordColumns + " FROM static_ip_exchanges WHERE tenant_id=? AND idempotency_key=?"
	if lock {
		query += " FOR UPDATE"
	}
	err := scanWalletRecord(db.Raw(query, tenant, key), &record)
	return record, err
}

func (r *WalletRepository) ExchangeStaticIP(ctx context.Context, tenant string, req wallet.StaticExchangeRequest) (wallet.StaticExchange, error) {
	prior, err := r.StaticExchange(ctx, tenant, req.IdempotencyKey)
	if err == nil {
		return replayStaticExchange(prior, req)
	}
	if !errors.Is(err, wallet.ErrMissing) {
		return prior, err
	}
	supplier, ok := r.static.(wallet.StaticExchangeSupplier)
	if !ok || r.staticCipher == nil || r.staticExchangeProbeAddress == "" {
		return wallet.StaticExchange{}, wallet.ErrStaticUnavailable
	}
	accepted, original, created, err := r.reserveStaticExchange(ctx, tenant, req)
	if err != nil || !created {
		return accepted, err
	}

	if err := r.markExchangeDispatch(ctx, tenant, req.IdempotencyKey, original.IP); err != nil {
		return accepted, nil
	}

	// A client disconnect cannot cancel a request already reserved for dispatch.
	process, cancel := context.WithTimeout(context.WithoutCancel(ctx), 15*time.Second)
	defer cancel()
	reply, err := supplier.Exchange(process, original.SupplierItemID, req.NodeID)
	status := wallet.StaticExchangeUnknown
	var next wallet.StaticAllocation
	if err == nil {
		switch {
		case reply.Rejected:
			status = wallet.StaticExchangeFailed
		case confirmedStaticExchange(reply.Allocation, original, req):
			status = wallet.StaticExchangeActivating
			next = reply.Allocation
		}
	}
	// Preserve enough time to record an unknown state after a transport timeout.
	save, stopSave := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer stopSave()
	result, err := r.finishStaticExchange(save, tenant, req, original, next, status)
	if err == nil {
		return result, nil
	}
	slog.WarnContext(save, "static IP exchange response could not be applied", "allocation_id", req.AllocationID)
	result, err = r.finishStaticExchange(save, tenant, req, original, wallet.StaticAllocation{}, wallet.StaticExchangeUnknown)
	if err == nil {
		return result, nil
	}
	return accepted, nil
}

func (r *WalletRepository) markExchangeDispatch(ctx context.Context, tenant, key, oldIP string) error {
	var snapshot *string
	if supplier, ok := r.static.(wallet.StaticExchangeHistorySupplier); ok {
		lookup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 4*time.Second)
		rows, err := supplier.ExchangeHistory(lookup, oldIP, time.Now())
		cancel()
		if err == nil {
			ids := make([]string, 0, len(rows))
			for _, row := range rows {
				ids = append(ids, row.ID)
			}
			encoded := strings.Join(ids, " ")
			snapshot = &encoded
		}
	}
	save, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
	defer cancel()
	result := r.db.WithContext(save).Exec(`UPDATE static_ip_exchanges SET dispatched_at=clock_timestamp(), history_snapshot=?
		WHERE tenant_id=? AND idempotency_key=? AND status='pending'`, snapshot, tenant, key)
	if result.Error == nil && result.RowsAffected != 1 {
		return wallet.ErrState
	}
	return result.Error
}

func replayStaticExchange(prior wallet.StaticExchange, req wallet.StaticExchangeRequest) (wallet.StaticExchange, error) {
	if prior.StaticExchangeRequest != req {
		return wallet.StaticExchange{}, wallet.ErrReplay
	}
	return prior, nil
}

func confirmedStaticExchange(next wallet.StaticAllocation, original exchangeAllocation, req wallet.StaticExchangeRequest) bool {
	newIP, newErr := netip.ParseAddr(next.IP)
	originalIP, oldErr := netip.ParseAddr(original.IP)
	return newErr == nil && oldErr == nil && newIP.Unmap() != originalIP.Unmap() &&
		next.SupplierItemID == original.SupplierItemID && (next.NodeID == 0 || next.NodeID == req.NodeID) &&
		next.ProxyID != "" && next.Port > 0 && next.Port <= 65535 && next.Username != "" && next.Password != "" &&
		next.ExpiresAt.Equal(original.ExpiresAt)
}

func (r *WalletRepository) reserveStaticExchange(ctx context.Context, tenant string, req wallet.StaticExchangeRequest) (wallet.StaticExchange, exchangeAllocation, bool, error) {
	var record wallet.StaticExchange
	var original exchangeAllocation
	created := false
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var orderID int64
		if err := scanWalletRecord(tx.Raw(`SELECT o.id FROM purchase_orders o
			JOIN static_ip_allocations a ON a.tenant_id=o.tenant_id AND a.order_id=o.id
			WHERE a.tenant_id=? AND a.allocation_id=? FOR UPDATE OF o`, tenant, req.AllocationID), &orderID); err != nil {
			return err
		}
		if err := scanWalletRecord(tx.Raw(`SELECT a.*, i.reference AS original_offer,
			COALESCE(p.supplier_order_id, '') AS supplier_order_id, a.expires_at > clock_timestamp() AS active
			FROM static_ip_allocations a
			JOIN static_purchases p ON p.tenant_id=a.tenant_id AND p.order_id=a.order_id
			JOIN order_items i ON i.tenant_id=a.tenant_id AND i.order_id=a.order_id AND i.position=a.order_item_position
			WHERE a.tenant_id=? AND a.allocation_id=? AND a.order_id=? FOR UPDATE OF a`,
			tenant, req.AllocationID, orderID), &original); err != nil {
			return err
		}
		if err := tx.Exec(`INSERT INTO static_ip_exchange_quotas (tenant_id) VALUES (?) ON CONFLICT DO NOTHING`, tenant).Error; err != nil {
			return err
		}
		var remaining int32
		if err := scanWalletRecord(tx.Raw(`SELECT GREATEST(0, quota::bigint - reserved - used) FROM static_ip_exchange_quotas WHERE tenant_id=? FOR UPDATE`, tenant), &remaining); err != nil {
			return err
		}
		prior, err := staticExchangeRecord(tx, tenant, req.IdempotencyKey, false)
		if err == nil {
			record, err = replayStaticExchange(prior, req)
			return err
		}
		if !errors.Is(err, wallet.ErrMissing) {
			return err
		}
		if err := validateStaticExchange(tx, tenant, req, original); err != nil {
			return err
		}
		if remaining <= 0 {
			return wallet.ErrStaticExchangeQuota
		}
		if err := scanWalletRecord(tx.Raw(`INSERT INTO static_ip_exchanges
			(tenant_id, idempotency_key, allocation_id, node_id, status, supplier_order_id, supplier_item_id,
			original_proxy_id, original_ip, original_port, original_username, original_password_ciphertext, original_expiry)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?) RETURNING `+exchangeRecordColumns,
			tenant, req.IdempotencyKey, req.AllocationID, req.NodeID, wallet.StaticExchangePending,
			original.SupplierOrderID, original.SupplierItemID, original.ProxyID, original.IP,
			original.Port, original.Username, original.PasswordCiphertext, original.ExpiresAt), &record); err != nil {
			return err
		}
		created = true
		return tx.Exec(`UPDATE static_ip_exchange_quotas SET reserved=reserved+1 WHERE tenant_id=?`, tenant).Error
	})
	return record, original, created, err
}

func validateStaticExchange(tx *gorm.DB, tenant string, req wallet.StaticExchangeRequest, original exchangeAllocation) error {
	node, shareMode, _, valid := wallet.ParseStaticPurchaseReference(original.OriginalOffer)
	if !valid || shareMode == "shared" || !original.Active || !time.Now().Before(original.ExpiresAt) || original.SupplierOrderID == "" {
		return wallet.ErrStaticExchangeIneligible
	}
	if req.NodeID != node {
		return wallet.ErrStaticExchangeRegion
	}
	var conflicts struct{ Renewal, Exchange bool }
	if err := tx.Raw(`SELECT
		EXISTS (SELECT 1 FROM static_renewal_items WHERE tenant_id=? AND allocation_id=? AND outcome='pending') AS renewal,
		EXISTS (SELECT 1 FROM static_ip_exchanges WHERE tenant_id=? AND allocation_id=? AND status IN ('pending', 'unknown', 'activating')) AS exchange`,
		tenant, req.AllocationID, tenant, req.AllocationID).Scan(&conflicts).Error; err != nil {
		return err
	}
	if conflicts.Renewal {
		return wallet.ErrStaticRenewalPending
	}
	if conflicts.Exchange {
		return wallet.ErrStaticExchangePending
	}
	return nil
}

func (r *WalletRepository) finishStaticExchange(ctx context.Context, tenant string, req wallet.StaticExchangeRequest,
	original exchangeAllocation, next wallet.StaticAllocation, status string,
) (wallet.StaticExchange, error) {
	var record wallet.StaticExchange
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Purchase reconciliation holds the same parent order before writing allocations.
		var orderID int64
		if err := scanWalletRecord(tx.Raw(`SELECT id FROM purchase_orders WHERE tenant_id=? AND id=? FOR UPDATE`,
			tenant, original.OrderID), &orderID); err != nil {
			return err
		}
		var current struct {
			IP, ProxyID string
			ExpiresAt   time.Time
		}
		if err := scanWalletRecord(tx.Raw(`SELECT ip, proxy_id, expires_at FROM static_ip_allocations WHERE tenant_id=? AND allocation_id=? FOR UPDATE`,
			tenant, req.AllocationID), &current); err != nil {
			return err
		}
		var err error
		record, err = staticExchangeRecord(tx, tenant, req.IdempotencyKey, true)
		if err != nil || record.Status != wallet.StaticExchangePending {
			return err
		}
		if status == wallet.StaticExchangeActivating {
			if current.IP != original.IP || current.ProxyID != original.ProxyID || !current.ExpiresAt.Equal(original.ExpiresAt) {
				return wallet.ErrState
			}
			password := r.staticCipher.Seal(nil, nil, []byte(next.Password), staticPasswordScope(tenant, original.SupplierItemID))
			if err := tx.Exec(`UPDATE static_ip_allocations SET proxy_id=?, ip=?, port=?, username=?, password_ciphertext=?
				WHERE tenant_id=? AND allocation_id=?`, next.ProxyID, next.IP, next.Port, next.Username, password, tenant, req.AllocationID).Error; err != nil {
				return err
			}
		}
		if status == wallet.StaticExchangeActivating || status == wallet.StaticExchangeFailed {
			used := 0
			if status == wallet.StaticExchangeActivating {
				used = 1
			}
			result := tx.Exec(`UPDATE static_ip_exchange_quotas SET reserved=reserved-1, used=used+?
				WHERE tenant_id=? AND reserved>0`, used, tenant)
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected != 1 {
				return wallet.ErrState
			}
		}
		return scanWalletRecord(tx.Raw(`UPDATE static_ip_exchanges SET status=?, updated_at=clock_timestamp(),
			new_proxy_id=NULLIF(?, ''), new_ip=NULLIF(?, ''),
			probe_at=CASE WHEN ? THEN clock_timestamp() END,
			unknown_at=CASE WHEN ? THEN statement_timestamp() END, next_check_at=CASE WHEN ? THEN statement_timestamp() END
			WHERE tenant_id=? AND idempotency_key=? RETURNING `+exchangeRecordColumns,
			status, next.ProxyID, next.IP, status == wallet.StaticExchangeActivating, status == wallet.StaticExchangeUnknown,
			status == wallet.StaticExchangeUnknown, tenant, req.IdempotencyKey), &record)
	})
	return record, err
}
