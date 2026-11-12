package repo

import (
	"time"

	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/biz/usage"
)

func (t usageTx) LockReplay(gatewayID, bootID string) (bool, bool, error) {
	var row struct{ Exited, Completed bool }
	err := t.db.Raw(`SELECT exited_at IS NOT NULL AS exited, replay_completed_at IS NOT NULL AS completed
		FROM gateway_boots WHERE gateway_id = ? AND boot_id = ? FOR UPDATE`, gatewayID, bootID).Scan(&row).Error
	return row.Exited, row.Completed, err
}

func (t usageTx) SaveSessionGap(gatewayID, bootID string, session usage.UnfinishedSession, now time.Time) error {
	return t.db.Exec(`INSERT INTO usage_gaps (gateway_id, boot_id, session_id, client_id, tenant_id, from_at, detected_at)
		VALUES (?, ?, ?, ?, (SELECT tenant_id FROM proxy_credentials WHERE client_id = ?), ?, ?)
		ON CONFLICT (gateway_id, boot_id, session_id) DO NOTHING`,
		gatewayID, bootID, session.SessionID, session.ClientID, session.ClientID, session.LastRecordedAt, now).Error
}

func (t usageTx) CountSessionGaps(gatewayID, bootID string) (int64, error) {
	var count int64
	err := t.db.Raw(`SELECT count(*) FROM usage_gaps WHERE gateway_id = ? AND boot_id = ? AND session_id IS NOT NULL`,
		gatewayID, bootID).Scan(&count).Error
	return count, err
}

func (t usageTx) FinishReplay(gatewayID, bootID string, hasSessionRecords *bool, now time.Time) error {
	if err := t.db.Exec(`INSERT INTO usage_gaps (gateway_id, boot_id, from_at, detected_at)
		SELECT gateway_id, boot_id, COALESCE(last_heartbeat_at, exited_at), ?
		FROM gateway_boots AS boot WHERE gateway_id = ? AND boot_id = ?
		AND NOT COALESCE(?::boolean, EXISTS (SELECT 1 FROM ingest_cursors AS cursor WHERE cursor.gateway_id = boot.gateway_id AND cursor.boot_id = boot.boot_id))
		AND NOT EXISTS (SELECT 1 FROM usage_gaps AS gap WHERE gap.gateway_id = boot.gateway_id AND gap.boot_id = boot.boot_id)
		ON CONFLICT DO NOTHING`, now, gatewayID, bootID, hasSessionRecords).Error; err != nil {
		return err
	}
	return t.db.Exec(`UPDATE gateway_boots SET replay_completed_at = ? WHERE gateway_id = ? AND boot_id = ?`,
		now, gatewayID, bootID).Error
}
