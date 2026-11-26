package repo

import (
	"context"
	"errors"
	"math"
	"net/netip"
	"slices"
	"strings"
	"time"

	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/biz/wallet"
	"gorm.io/gorm"
)

const (
	exchangeCheckLease   = 30 * time.Second
	exchangeCheckTimeout = 8 * time.Second
	exchangeClockSkew    = 30 * time.Second
)

type exchangeCheck struct {
	wallet.ExchangeOperationsRecord
	OrderID                     int64
	OriginalProxyID, NewProxyID string
	OriginalExpiry              time.Time
	DispatchedAt                *time.Time
	HistorySnapshot             *string
}

const exchangeCheckColumns = operatorExchangeColumns + `, original_proxy_id, COALESCE(new_proxy_id,'') AS new_proxy_id,
	original_expiry, dispatched_at, history_snapshot,
	(SELECT a.order_id FROM static_ip_allocations a
	 WHERE a.tenant_id=static_ip_exchanges.tenant_id AND a.allocation_id=static_ip_exchanges.allocation_id) AS order_id`

type exchangeObservation struct {
	SupplierRecordID, IP string
	Allocation           wallet.StaticAllocation
}

type exchangeReview struct {
	wallet.ExchangeOperationsResolution
	Actor       string
	Observation exchangeObservation
}

func (r *WalletRepository) ReconcileStaticExchanges(ctx context.Context) error {
	if r.db == nil || r.static == nil || r.staticCipher == nil {
		return nil
	}
	if _, ok := r.static.(wallet.StaticExchangeHistorySupplier); !ok {
		return nil
	}
	check, err := r.claimExchangeCheck(ctx)
	if errors.Is(err, wallet.ErrMissing) {
		return nil
	}
	if err != nil {
		return err
	}
	lookup, cancel := context.WithTimeout(ctx, exchangeCheckTimeout)
	defer cancel()
	observation := r.observeExchange(lookup, check)
	if observation.SupplierRecordID == "" {
		return nil
	}
	_, err = r.applyExchangeReview(ctx, check, exchangeReview{
		ExchangeOperationsResolution: wallet.ExchangeOperationsResolution{Outcome: wallet.StaticExchangeExecuted},
		Observation:                  observation,
	})
	return err
}

func (r *WalletRepository) claimExchangeCheck(ctx context.Context) (exchangeCheck, error) {
	var check exchangeCheck
	err := scanWalletRecord(r.db.WithContext(ctx).Raw(`WITH candidate AS (
		SELECT tenant_id, idempotency_key FROM static_ip_exchanges
		WHERE next_check_at <= statement_timestamp()
		ORDER BY next_check_at, tenant_id, idempotency_key LIMIT 1 FOR UPDATE SKIP LOCKED
	)
	UPDATE static_ip_exchanges SET next_check_at=statement_timestamp() + (? * interval '1 second')
	WHERE (tenant_id, idempotency_key) IN (SELECT tenant_id, idempotency_key FROM candidate)
	RETURNING `+exchangeCheckColumns, int(exchangeCheckLease/time.Second)), &check)
	return check, err
}

func (r *WalletRepository) observeExchange(ctx context.Context, check exchangeCheck) exchangeObservation {
	dispatched := check.CreatedAt
	if check.DispatchedAt != nil {
		dispatched = *check.DispatchedAt
	}
	supplier := r.static.(wallet.StaticExchangeHistorySupplier)
	history, err := supplier.ExchangeHistory(ctx, check.OldIP, dispatched)
	if err != nil {
		return exchangeObservation{}
	}
	var baseline []string
	if check.HistorySnapshot != nil {
		baseline = strings.Fields(*check.HistorySnapshot)
	}
	latest := minTime(time.Now().Add(exchangeClockSkew), dispatched.Add(time.Minute+exchangeClockSkew))
	var observed exchangeObservation
	for _, row := range history {
		if row.SupplierOrderID != check.SupplierOrderID || row.SupplierItemID != check.SupplierItemID ||
			row.OldNodeID != check.NodeID || row.NodeID != check.NodeID || !sameExchangeIP(row.OldIP, check.OldIP) ||
			sameExchangeIP(row.IP, check.OldIP) || slices.Contains(baseline, row.ID) ||
			row.CreatedAt.Before(dispatched.Add(-exchangeClockSkew)) || row.CreatedAt.After(latest) {
			continue
		}
		if observed.SupplierRecordID != "" {
			return exchangeObservation{}
		}
		observed.SupplierRecordID, observed.IP = row.ID, row.IP
	}
	if observed.SupplierRecordID != "" {
		observed.Allocation = r.exchangeDetail(ctx, check, observed.IP)
	}
	return observed
}

func (r *WalletRepository) exchangeDetail(ctx context.Context, check exchangeCheck, ip string) wallet.StaticAllocation {
	if r.static == nil {
		return wallet.StaticAllocation{}
	}
	delivery, err := r.static.Detail(ctx, check.SupplierOrderID)
	if err != nil || delivery.SupplierOrderID != check.SupplierOrderID {
		return wallet.StaticAllocation{}
	}
	var next wallet.StaticAllocation
	for _, item := range delivery.Allocations {
		if item.SupplierItemID != check.SupplierItemID {
			continue
		}
		if next.SupplierItemID != "" {
			return wallet.StaticAllocation{}
		}
		next = item
	}
	if !sameExchangeIP(next.IP, ip) || next.NodeID != check.NodeID || next.ProxyID == "" {
		return wallet.StaticAllocation{}
	}
	return next
}

func minTime(a, b time.Time) time.Time {
	if a.Before(b) {
		return a
	}
	return b
}

func sameExchangeIP(first, second string) bool {
	a, firstErr := netip.ParseAddr(first)
	b, secondErr := netip.ParseAddr(second)
	return firstErr == nil && secondErr == nil && a.Unmap() == b.Unmap()
}

func (r *WalletRepository) ResolveExchange(ctx context.Context, actor string, cmd wallet.ExchangeOperationsResolution) (wallet.ExchangeOperationsRecord, error) {
	var check exchangeCheck
	if r.db == nil {
		return check.ExchangeOperationsRecord, errWalletUnavailable
	}
	err := scanWalletRecord(r.db.WithContext(ctx).Raw("SELECT "+exchangeCheckColumns+
		" FROM static_ip_exchanges WHERE tenant_id=? AND idempotency_key=?", cmd.TenantID, cmd.IdempotencyKey), &check)
	if err != nil {
		return check.ExchangeOperationsRecord, err
	}
	review := exchangeReview{ExchangeOperationsResolution: cmd, Actor: actor}
	if cmd.Outcome == wallet.StaticExchangeExecuted || cmd.Correction {
		ip := cmd.ExpectedIP
		if cmd.Outcome == wallet.StaticExchangeNotExecuted {
			ip = check.OldIP
		}
		lookup, cancel := context.WithTimeout(ctx, exchangeCheckTimeout)
		defer cancel()
		review.Observation = exchangeObservation{IP: cmd.ExpectedIP, Allocation: r.exchangeDetail(lookup, check, ip)}
	}
	return r.applyExchangeReview(ctx, check, review)
}

func (r *WalletRepository) applyExchangeReview(ctx context.Context, check exchangeCheck, review exchangeReview) (wallet.ExchangeOperationsRecord, error) {
	var result wallet.ExchangeOperationsRecord
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var orderID int64
		if err := scanWalletRecord(tx.Raw("SELECT id FROM purchase_orders WHERE tenant_id=? AND id=? FOR UPDATE",
			check.TenantID, check.OrderID), &orderID); err != nil {
			return err
		}
		var current wallet.StaticAllocation
		if err := scanWalletRecord(tx.Raw("SELECT ip, proxy_id, expires_at FROM static_ip_allocations WHERE tenant_id=? AND allocation_id=? FOR UPDATE",
			check.TenantID, check.AllocationID), &current); err != nil {
			return err
		}
		var locked exchangeCheck
		if err := scanWalletRecord(tx.Raw("SELECT "+exchangeCheckColumns+
			" FROM static_ip_exchanges WHERE tenant_id=? AND idempotency_key=? FOR UPDATE", check.TenantID, check.IdempotencyKey), &locked); err != nil {
			return err
		}
		result = locked.ExchangeOperationsRecord
		before, err := lockedExchangeQuota(tx, check.TenantID)
		if err != nil {
			return err
		}
		if review.Actor != "" {
			prior, err := exchangeAuditByRequest(tx, check.TenantID, review.RequestKey)
			if err == nil {
				if prior.ActorID == review.Actor && prior.IdempotencyKey == check.IdempotencyKey &&
					prior.Outcome == review.Outcome && prior.Evidence == review.Evidence &&
					prior.ObservedIP == review.ExpectedIP && (prior.Kind == "correction") == review.Correction {
					return nil
				}
				return wallet.ErrReplay
			}
			if !errors.Is(err, wallet.ErrMissing) {
				return err
			}
		}
		if locked.Status != wallet.StaticExchangeUnknown && !review.Correction {
			return recordClosedExchangeObservation(tx, locked, review, before)
		}
		if review.Actor != "" && !review.Correction && !locked.InQueue {
			return wallet.ErrExchangeNotQueued
		}
		if review.Correction && (!locked.ResolutionConflict || locked.Corrected) {
			return wallet.ErrExchangeAlreadyResolved
		}
		if review.Actor == "" {
			if !confirmedExchangeDetail(locked, review.Observation.Allocation) {
				return nil
			}
			if locked.DispatchedAt == nil || locked.HistorySnapshot == nil {
				return recordUnverifiedExchangeObservation(tx, locked, review.Observation, before)
			}
			var recorded bool
			if err := tx.Raw("SELECT EXISTS (SELECT 1 FROM static_ip_exchange_audit WHERE supplier_record_id=? AND kind='automatic')",
				review.Observation.SupplierRecordID).Scan(&recorded).Error; err != nil || recorded {
				return err
			}
		}
		after, err := r.finishExchangeReview(tx, locked, review, before, current)
		if err != nil {
			return err
		}
		if err := storeExchangeQuota(tx, locked.TenantID, after); err != nil {
			return err
		}
		return scanWalletRecord(tx.Raw("SELECT "+operatorExchangeColumns+
			" FROM static_ip_exchanges WHERE tenant_id=? AND idempotency_key=?", locked.TenantID, locked.IdempotencyKey), &result)
	})
	return result, err
}

func confirmedExchangeDetail(check exchangeCheck, next wallet.StaticAllocation) bool {
	return confirmedStaticExchange(next, exchangeAllocation{StaticAllocation: wallet.StaticAllocation{
		IP: check.OldIP, SupplierItemID: check.SupplierItemID, ExpiresAt: check.OriginalExpiry,
	}}, check.StaticExchangeRequest)
}

func recordUnverifiedExchangeObservation(tx *gorm.DB, check exchangeCheck, observation exchangeObservation, quota wallet.ExchangeOperationsQuota) error {
	var recorded bool
	if err := tx.Raw(`SELECT EXISTS (SELECT 1 FROM static_ip_exchange_audit
		WHERE tenant_id=? AND idempotency_key=? AND supplier_record_id=? AND kind='observation')`,
		check.TenantID, check.IdempotencyKey, observation.SupplierRecordID).Scan(&recorded).Error; err != nil || recorded {
		return err
	}
	_, err := insertExchangeAudit(tx, check.TenantID, wallet.ExchangeOperationsAudit{
		IdempotencyKey: check.IdempotencyKey, Kind: "observation", SupplierRecordID: observation.SupplierRecordID,
		ObservedIP: observation.IP, Evidence: "No complete pre-dispatch Crown history snapshot",
	}, quota, quota)
	return err
}

func recordClosedExchangeObservation(tx *gorm.DB, check exchangeCheck, review exchangeReview, quota wallet.ExchangeOperationsQuota) error {
	if check.Resolution == "" {
		if review.Actor != "" {
			return wallet.ErrExchangeAlreadyResolved
		}
		return nil
	}
	if review.Actor == "" && (check.Corrected || check.ResolutionConflict) {
		return nil
	}
	same := check.Resolution == review.Outcome &&
		(review.Outcome == wallet.StaticExchangeNotExecuted || sameExchangeIP(check.NewIP, review.Observation.IP))
	if review.Actor == "" && !same && check.HistorySnapshot == nil {
		return recordUnverifiedExchangeObservation(tx, check, review.Observation, quota)
	}
	if review.Actor == "" && check.Resolution == review.Outcome && !same && review.Observation.Allocation.SupplierItemID == "" {
		return nil
	}
	if err := tx.Exec(`UPDATE static_ip_exchanges SET next_check_at=CASE WHEN ? THEN NULL ELSE next_check_at END,
		resolution_conflict=resolution_conflict OR ?
		WHERE tenant_id=? AND idempotency_key=?`, review.Actor == "", !same && !check.Corrected,
		check.TenantID, check.IdempotencyKey).Error; err != nil {
		return err
	}
	kind := "conflict"
	if same {
		kind = "duplicate"
	}
	_, err := insertExchangeAudit(tx, check.TenantID, wallet.ExchangeOperationsAudit{
		IdempotencyKey: check.IdempotencyKey, Kind: kind, Outcome: review.Outcome, RequestKey: review.RequestKey,
		Evidence: review.Evidence, ActorID: review.Actor, SupplierRecordID: review.Observation.SupplierRecordID, ObservedIP: review.Observation.IP,
	}, quota, quota)
	return err
}

func (r *WalletRepository) finishExchangeReview(tx *gorm.DB, check exchangeCheck, review exchangeReview,
	quota wallet.ExchangeOperationsQuota, current wallet.StaticAllocation) (wallet.ExchangeOperationsQuota, error) {
	status := wallet.StaticExchangeFailed
	next := review.Observation.Allocation
	executed := review.Outcome == wallet.StaticExchangeExecuted
	superseded := false
	if review.Correction {
		if err := tx.Raw(`SELECT EXISTS (SELECT 1 FROM static_ip_exchanges WHERE tenant_id=? AND allocation_id=?
			AND (created_at, idempotency_key) > (?, ?) AND status <> 'failed')`,
			check.TenantID, check.AllocationID, check.CreatedAt, check.IdempotencyKey).Scan(&superseded).Error; err != nil {
			return quota, err
		}
		var observed bool
		if err := tx.Raw(`SELECT EXISTS (SELECT 1 FROM static_ip_exchange_audit WHERE tenant_id=? AND idempotency_key=?
			AND kind='conflict' AND outcome=? AND observed_ip=?)`, check.TenantID, check.IdempotencyKey,
			review.Outcome, review.ExpectedIP).Scan(&observed).Error; err != nil {
			return quota, err
		}
		if !observed {
			return quota, wallet.ErrExchangeEvidence
		}
	}
	if executed {
		status = wallet.StaticExchangeReady
		if !superseded {
			if !confirmedExchangeReviewConfig(check, current, next, review.Correction) {
				return quota, wallet.ErrExchangeEvidence
			}
			if err := r.storeExchangeReviewConfig(tx, check, next); err != nil {
				return quota, err
			}
			status = wallet.StaticExchangeActivating
		}
	} else if review.Correction && !superseded {
		if !sameExchangeIP(next.IP, check.OldIP) || !sameExchangeIP(current.IP, check.NewIP) ||
			current.ProxyID != check.NewProxyID || !next.ExpiresAt.Equal(current.ExpiresAt) {
			return quota, wallet.ErrExchangeEvidence
		}
		if err := r.storeExchangeReviewConfig(tx, check, next); err != nil {
			return quota, err
		}
	}
	after := quota
	wasExecuted := check.Resolution == wallet.StaticExchangeExecuted
	if !review.Correction {
		if quota.Reserved <= 0 {
			return quota, wallet.ErrState
		}
		after.Reserved--
	}
	switch {
	case executed && !wasExecuted && quota.Used < math.MaxInt32:
		after.Used++
	case !executed && wasExecuted && quota.Used > 0:
		after.Used--
	case executed != wasExecuted:
		return quota, wallet.ErrState
	}
	keepChecking := review.Actor != "" && !review.Correction
	if err := tx.Exec(`UPDATE static_ip_exchanges SET status=?, resolution=?, resolution_conflict=false, corrected=?,
		updated_at=clock_timestamp(), new_proxy_id=NULLIF(?,''),
		new_ip=NULLIF(?,''), probe_at=CASE WHEN ? THEN clock_timestamp() END,
		next_check_at=CASE WHEN ? THEN next_check_at END, probe_attempts=0
		WHERE tenant_id=? AND idempotency_key=?`, status, review.Outcome, review.Correction, next.ProxyID, review.Observation.IP,
		status == wallet.StaticExchangeActivating, keepChecking, check.TenantID, check.IdempotencyKey).Error; err != nil {
		return quota, err
	}
	kind := "automatic"
	switch {
	case review.Correction:
		kind = "correction"
	case review.Actor != "":
		kind = "manual"
	}
	_, err := insertExchangeAudit(tx, check.TenantID, wallet.ExchangeOperationsAudit{
		IdempotencyKey: check.IdempotencyKey, RequestKey: review.RequestKey, ActorID: review.Actor,
		Kind: kind, Evidence: review.Evidence, Outcome: review.Outcome,
		SupplierRecordID: review.Observation.SupplierRecordID, ObservedIP: review.Observation.IP,
	}, quota, after)
	return after, err
}

func confirmedExchangeReviewConfig(check exchangeCheck, current, next wallet.StaticAllocation, correction bool) bool {
	ip, proxy := check.OldIP, check.OriginalProxyID
	if check.Resolution == wallet.StaticExchangeExecuted {
		ip, proxy = check.NewIP, check.NewProxyID
	}
	if correction {
		// Renewal may have advanced the expiry after the previous resolution. Compare with the
		// current allocation and preserve its expiry when updating the verified connection data.
		check.OriginalExpiry = current.ExpiresAt
	}
	return sameExchangeIP(current.IP, ip) && current.ProxyID == proxy &&
		current.ExpiresAt.Equal(check.OriginalExpiry) && confirmedExchangeDetail(check, next)
}

func (r *WalletRepository) storeExchangeReviewConfig(tx *gorm.DB, check exchangeCheck, next wallet.StaticAllocation) error {
	if r.staticCipher == nil {
		return wallet.ErrStaticUnavailable
	}
	password := r.staticCipher.Seal(nil, nil, []byte(next.Password), staticPasswordScope(check.TenantID, check.SupplierItemID))
	return tx.Exec(`UPDATE static_ip_allocations SET proxy_id=?, ip=?, port=?, username=?, password_ciphertext=?
		WHERE tenant_id=? AND allocation_id=?`, next.ProxyID, next.IP, next.Port, next.Username,
		password, check.TenantID, check.AllocationID).Error
}
