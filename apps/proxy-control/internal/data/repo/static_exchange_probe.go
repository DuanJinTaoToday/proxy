package repo

import (
	"context"
	"errors"
	"net"
	"strconv"
	"time"

	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/biz/wallet"
	"golang.org/x/net/proxy"
)

const (
	staleExchangeDispatchAge = 60 * time.Second
	exchangeRecoveriesPerRun = 64
	maxExchangeProbeAttempts = 4
	exchangeProbeDelay       = 15 * time.Second
	exchangeProbeTimeout     = 3 * time.Second
	exchangeProbesPerRun     = 4
)

type exchangeProbe struct {
	TenantID, IdempotencyKey, Status string
	SupplierItemID, IP, Username     string
	Port, Attempt                    int
	PasswordCiphertext               []byte
}

func (r *WalletRepository) ProbeStaticExchanges(ctx context.Context) error {
	if r.db == nil {
		return nil
	}
	if !r.staticExchangeProbing.TryLock() {
		return nil
	}
	defer r.staticExchangeProbing.Unlock()
	if err := r.recoverStaleStaticExchanges(ctx); err != nil {
		return err
	}
	if r.staticCipher == nil || r.staticExchangeProbeAddress == "" {
		return nil
	}
	for range exchangeProbesPerRun {
		if err := ctx.Err(); err != nil {
			return err
		}
		work, err := r.claimExchangeProbe(ctx)
		if errors.Is(err, wallet.ErrMissing) {
			return nil
		}
		if err != nil {
			return err
		}
		if work.Status == wallet.StaticExchangeSupport {
			continue
		}
		status := wallet.StaticExchangeActivating
		if r.dialExchangeProbe(ctx, work) {
			status = wallet.StaticExchangeReady
		} else if work.Attempt >= maxExchangeProbeAttempts {
			status = wallet.StaticExchangeSupport
		}
		if status != wallet.StaticExchangeActivating {
			if err := r.db.WithContext(ctx).Exec(`UPDATE static_ip_exchanges
				SET status=?, probe_at=NULL, updated_at=clock_timestamp()
				WHERE tenant_id=? AND idempotency_key=? AND status='activating' AND probe_attempts=?`,
				status, work.TenantID, work.IdempotencyKey, work.Attempt).Error; err != nil {
				return err
			}
		}
	}
	return nil
}

func (r *WalletRepository) recoverStaleStaticExchanges(ctx context.Context) error {
	// Dispatch and receipt persistence use separate bounded contexts. Leave their normal window intact.
	return r.db.WithContext(ctx).Exec(`WITH stale AS (
		SELECT tenant_id, idempotency_key FROM static_ip_exchanges
		WHERE status='pending' AND updated_at <= statement_timestamp() - (? * interval '1 second')
		ORDER BY updated_at, tenant_id, idempotency_key LIMIT ? FOR UPDATE SKIP LOCKED
	)
	UPDATE static_ip_exchanges x SET status='unknown', updated_at=statement_timestamp(),
		unknown_at=statement_timestamp(), next_check_at=statement_timestamp()
	FROM stale s WHERE x.tenant_id=s.tenant_id AND x.idempotency_key=s.idempotency_key AND x.status='pending'`,
		int(staleExchangeDispatchAge/time.Second), exchangeRecoveriesPerRun).Error
}

func (r *WalletRepository) claimExchangeProbe(ctx context.Context) (exchangeProbe, error) {
	var work exchangeProbe
	// Commit the next attempt time before dialing so instances cannot claim the same attempt.
	err := scanWalletRecord(r.db.WithContext(ctx).Raw(`WITH candidate AS (
		SELECT tenant_id, idempotency_key FROM static_ip_exchanges
		WHERE status='activating' AND probe_at <= clock_timestamp()
		ORDER BY probe_at, tenant_id, idempotency_key LIMIT 1 FOR UPDATE SKIP LOCKED
	), claimed AS (
		UPDATE static_ip_exchanges x SET
			probe_at=CASE WHEN x.probe_attempts>=? THEN NULL ELSE clock_timestamp() + (? * interval '1 second') END,
			status=CASE WHEN x.probe_attempts>=? THEN 'support_required' ELSE x.status END,
			probe_attempts=LEAST(x.probe_attempts+1, ?),
			updated_at=CASE WHEN x.probe_attempts>=? THEN clock_timestamp() ELSE x.updated_at END
		FROM candidate c WHERE x.tenant_id=c.tenant_id AND x.idempotency_key=c.idempotency_key
		RETURNING x.tenant_id, x.allocation_id, x.idempotency_key, x.status, x.probe_attempts
	)
	SELECT x.tenant_id, x.idempotency_key, x.status, x.probe_attempts AS attempt,
		a.supplier_item_id, a.ip, a.port, a.username, a.password_ciphertext
	FROM claimed x JOIN static_ip_allocations a ON a.tenant_id=x.tenant_id AND a.allocation_id=x.allocation_id`,
		maxExchangeProbeAttempts, int(exchangeProbeDelay/time.Second), maxExchangeProbeAttempts,
		maxExchangeProbeAttempts, maxExchangeProbeAttempts), &work)
	return work, err
}

func (r *WalletRepository) dialExchangeProbe(ctx context.Context, work exchangeProbe) bool {
	password, err := r.staticCipher.Open(nil, nil, work.PasswordCiphertext, staticPasswordScope(work.TenantID, work.SupplierItemID))
	if err != nil {
		return false
	}
	ctx, cancel := context.WithTimeout(ctx, exchangeProbeTimeout)
	defer cancel()
	address := net.JoinHostPort(work.IP, strconv.Itoa(work.Port))
	socks, err := proxy.SOCKS5("tcp", address, &proxy.Auth{User: work.Username, Password: string(password)}, &net.Dialer{})
	if err != nil {
		return false
	}
	conn, err := socks.(proxy.ContextDialer).DialContext(ctx, "tcp", r.staticExchangeProbeAddress)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}
