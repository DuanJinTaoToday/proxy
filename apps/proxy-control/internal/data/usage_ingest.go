package data

import (
	"time"

	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/data/model"
	"gorm.io/gorm/clause"
)

func (t usageTx) EnsureBoot(gatewayID, bootID string, now time.Time) error {
	return t.db.Exec(`INSERT INTO gateway_boots (gateway_id, boot_id, first_seen_at)
			VALUES (?, ?, ?) ON CONFLICT DO NOTHING`, gatewayID, bootID, now).Error
}

func (t usageTx) ReportLifecycle(gatewayID, bootID string, now time.Time, heartbeat, exit *time.Time) error {
	return t.db.Exec(`INSERT INTO gateway_boots (gateway_id, boot_id, first_seen_at, last_heartbeat_at, exited_at)
			VALUES (?, ?, ?, ?, ?)
			ON CONFLICT (gateway_id, boot_id) DO UPDATE SET
			last_heartbeat_at = CASE WHEN gateway_boots.exited_at IS NULL AND EXCLUDED.exited_at IS NULL
				THEN GREATEST(gateway_boots.last_heartbeat_at, EXCLUDED.last_heartbeat_at) ELSE gateway_boots.last_heartbeat_at END,
			exited_at = COALESCE(gateway_boots.exited_at, EXCLUDED.exited_at)`,
		gatewayID, bootID, now, heartbeat, exit).Error
}

func (t usageTx) Cursor(gatewayID, bootID, fileID string) (int64, error) {
	var row model.IngestCursor
	err := t.db.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("gateway_id = ? AND boot_id = ? AND file_id = ?", gatewayID, bootID, fileID).Find(&row).Error
	return row.ConfirmedOffset, err
}

func (t usageTx) SaveCursor(gatewayID, bootID, fileID string, offset int64, now time.Time) error {
	return t.db.Exec(`INSERT INTO ingest_cursors (gateway_id, boot_id, file_id, confirmed_offset, updated_at)
			VALUES (?, ?, ?, ?, ?) ON CONFLICT (gateway_id, boot_id, file_id) DO UPDATE
			SET confirmed_offset = EXCLUDED.confirmed_offset, updated_at = EXCLUDED.updated_at`,
		gatewayID, bootID, fileID, offset, now).Error
}

func (t usageTx) SaveLastRecord(gatewayID, bootID string, recordedAt time.Time) error {
	return t.db.Exec(`UPDATE gateway_boots SET last_record_at = GREATEST(last_record_at, ?)
			WHERE gateway_id = ? AND boot_id = ?`, recordedAt, gatewayID, bootID).Error
}

func (t usageTx) TenantForClient(clientID string) (string, error) {
	var tenantID string
	err := t.db.Raw("SELECT tenant_id FROM proxy_credentials WHERE client_id = ?", clientID).Scan(&tenantID).Error
	return tenantID, err
}

func (t usageTx) Quarantine(gatewayID, bootID, fileID string, offset uint64, kind, detail string, now time.Time) error {
	return t.db.Exec(`INSERT INTO record_anomalies (gateway_id, boot_id, file_id, file_offset, kind, detail, detected_at)
		VALUES (?, ?, ?, ?, ?, ?::jsonb, ?)`, gatewayID, bootID, fileID, offset, kind, detail, now).Error
}
