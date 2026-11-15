package repo

import (
	"context"

	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/biz/subaccount"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func (r *SubaccountRepository) Acquire(ctx context.Context, gateway, client, key string) (subaccount.StickySession, error) {
	var result subaccount.StickySession
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var bound string
		if err := tx.Raw(`SELECT s.client_id FROM proxy_subaccounts s JOIN gateway_auth_references r ON r.client_id=s.client_id
            WHERE s.client_id=? AND r.gateway_id=? AND s.active FOR UPDATE OF s`, client, gateway).Scan(&bound).Error; err != nil {
			return err
		}
		if bound == "" {
			return subaccount.ErrSessionLease
		}
		var current struct {
			UpstreamSID            int64 `gorm:"column:upstream_sid"`
			Established, Available bool
		}
		if err := tx.Raw(`SELECT upstream_sid,COALESCE(expires_at > clock_timestamp(),false) AS established,
            CASE WHEN established_at IS NULL THEN lease_until <= clock_timestamp() ELSE expires_at <= clock_timestamp() END AS available
            FROM sticky_sessions WHERE client_id=? AND session_key=? FOR UPDATE`, client, key).Scan(&current).Error; err != nil {
			return err
		}
		if current.Established {
			result.UpstreamSID = current.UpstreamSID
			return nil
		}
		if current.UpstreamSID != 0 && !current.Available {
			return subaccount.ErrSessionBusy
		}
		return tx.Raw(`INSERT INTO sticky_sessions(client_id,session_key,upstream_sid,gateway_id,lease_token,lease_until)
            VALUES (?,?,nextval('upstream_session_sids'),?,?,now()+interval '20 seconds')
            ON CONFLICT(client_id,session_key) DO UPDATE SET upstream_sid=EXCLUDED.upstream_sid,
                gateway_id=EXCLUDED.gateway_id,lease_token=EXCLUDED.lease_token,lease_until=EXCLUDED.lease_until,
                established_at=NULL,expires_at=NULL
            RETURNING upstream_sid,lease_token`, client, key, gateway, uuid.NewString()).Scan(&result).Error
	})
	return result, err
}

func (r *SubaccountRepository) Commit(ctx context.Context, gateway, client, key, lease string) (subaccount.StickySession, error) {
	var result subaccount.StickySession
	err := r.db.WithContext(ctx).Raw(`UPDATE sticky_sessions SET established_at=COALESCE(established_at,now()),
        expires_at=COALESCE(expires_at,now()+interval '1 hour')
        WHERE client_id=? AND session_key=? AND gateway_id=? AND lease_token=?
            AND (expires_at > clock_timestamp() OR established_at IS NULL AND lease_until > clock_timestamp())
        RETURNING upstream_sid`, client, key, gateway, lease).Scan(&result).Error
	if err == nil && result.UpstreamSID == 0 {
		err = subaccount.ErrSessionLease
	}
	return result, err
}

func (r *SubaccountRepository) Release(ctx context.Context, gateway, client, key, lease string) error {
	return r.db.WithContext(ctx).Exec(`DELETE FROM sticky_sessions WHERE client_id=? AND session_key=?
        AND gateway_id=? AND lease_token=? AND established_at IS NULL`, client, key, gateway, lease).Error
}
