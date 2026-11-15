package repo

import (
	"context"

	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/biz/subaccount"
	"gorm.io/gorm"
)

const ttlMillis = `GREATEST(0,FLOOR(EXTRACT(epoch FROM expires_at-clock_timestamp())*1000))::bigint AS expires_in_millis`

type stickySessionRow struct {
	UpstreamSid, ExpiresInMillis int64
	LeaseToken, GatewayID        string
	Available                    bool
}

func (row stickySessionRow) session() subaccount.StickySession {
	return subaccount.StickySession{UpstreamSID: row.UpstreamSid, LeaseToken: row.LeaseToken, ExpiresInMillis: row.ExpiresInMillis}
}

func (r *SubaccountRepository) Acquire(ctx context.Context, gateway string, req subaccount.SessionRequest) (subaccount.StickySession, error) {
	var result stickySessionRow
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var client string
		if err := tx.Raw(`SELECT s.client_id FROM proxy_subaccounts s JOIN gateway_auth_references r ON r.client_id=s.client_id
            WHERE r.reference=? AND r.gateway_id=? AND s.active FOR UPDATE OF s`, req.Reference, gateway).Scan(&client).Error; err != nil {
			return err
		}
		if client == "" {
			return subaccount.ErrSessionLease
		}
		var current stickySessionRow
		if err := tx.Raw(`SELECT upstream_sid,lease_token,gateway_id,`+ttlMillis+`,
            CASE WHEN established_at IS NULL THEN lease_until <= clock_timestamp() ELSE expires_at <= clock_timestamp() END AS available
            FROM sticky_sessions WHERE client_id=? AND session_key=? FOR UPDATE`, client, req.Key).Scan(&current).Error; err != nil {
			return err
		}
		if current.ExpiresInMillis > 0 {
			current.LeaseToken = ""
			result = current
			return nil
		}
		if current.UpstreamSid != 0 && !current.Available {
			if current.LeaseToken == req.Attempt && current.GatewayID == gateway {
				result = current
				return nil
			}
			return subaccount.ErrSessionBusy
		}
		return tx.Raw(`INSERT INTO sticky_sessions(client_id,session_key,upstream_sid,gateway_id,lease_token,lease_until)
            VALUES (?,?,nextval('upstream_session_sids'),?,?,clock_timestamp()+interval '20 seconds')
            ON CONFLICT(client_id,session_key) DO UPDATE SET upstream_sid=EXCLUDED.upstream_sid,
                gateway_id=EXCLUDED.gateway_id,lease_token=EXCLUDED.lease_token,lease_until=EXCLUDED.lease_until,
                established_at=NULL,expires_at=NULL
            RETURNING upstream_sid,lease_token`, client, req.Key, gateway, req.Attempt).Scan(&result).Error
	})
	return result.session(), err
}

func (r *SubaccountRepository) Commit(ctx context.Context, gateway string, req subaccount.SessionRequest) (subaccount.StickySession, error) {
	var result stickySessionRow
	err := r.db.WithContext(ctx).Raw(`UPDATE sticky_sessions AS s SET established_at=COALESCE(s.established_at,now()),
        expires_at=COALESCE(s.expires_at,now()+interval '1 hour')
        FROM gateway_auth_references r
        WHERE s.client_id=r.client_id AND r.reference=? AND r.gateway_id=s.gateway_id
            AND s.session_key=? AND s.gateway_id=? AND s.lease_token=?
            AND (s.expires_at > clock_timestamp() OR s.established_at IS NULL AND s.lease_until > clock_timestamp())
        RETURNING s.upstream_sid,`+ttlMillis, req.Reference, req.Key, gateway, req.Attempt).Scan(&result).Error
	if err == nil && result.UpstreamSid == 0 {
		err = subaccount.ErrSessionLease
	}
	return result.session(), err
}

func (r *SubaccountRepository) Release(ctx context.Context, gateway string, req subaccount.SessionRequest) error {
	return r.db.WithContext(ctx).Exec(`DELETE FROM sticky_sessions AS s USING gateway_auth_references r
        WHERE s.client_id=r.client_id AND r.reference=? AND r.gateway_id=s.gateway_id
            AND s.session_key=? AND s.gateway_id=? AND s.lease_token=? AND s.established_at IS NULL`,
		req.Reference, req.Key, gateway, req.Attempt).Error
}
