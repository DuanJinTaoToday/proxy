package repo

import (
	"context"
	"database/sql"

	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/biz/usage"
	"gorm.io/gorm"
)

func (r *UsageRepository) ListRecordAnomalies(ctx context.Context, filter usage.FindingFilter, after *usage.FindingKey, limit int) ([]usage.RecordAnomaly, error) {
	// The immutable client-to-tenant mapping lets old evidence participate in
	// tenant filters without returning stored detail or credential identifiers.
	const source = `SELECT a.id, a.gateway_id, a.boot_id, COALESCE(c.tenant_id, '') AS tenant_id,
		a.kind, a.file_id, a.file_offset, a.detected_at, a.resolved_at
		FROM record_anomalies a LEFT JOIN proxy_credentials c ON c.client_id = a.detail->>'client_id'`
	var rows []usage.RecordAnomaly
	err := queryFindings(r.db.WithContext(ctx), source, filter, after, limit).Scan(&rows).Error
	return rows, err
}

func (r *UsageRepository) ListUsageGaps(ctx context.Context, filter usage.FindingFilter, after *usage.FindingKey, limit int) ([]usage.UsageGap, error) {
	// Session reports are provisional until the collector completes replay.
	// Before any reports arrive, expose one pending row for the exited boot.
	const source = `SELECT g.id, g.gateway_id, g.boot_id, COALESCE(g.tenant_id, '') AS tenant_id,
		CASE WHEN g.session_id IS NULL THEN 'boot' ELSE 'session' END AS kind,
		CASE WHEN b.replay_completed_at IS NULL THEN 'pending_replay' ELSE 'possible_gap' END AS status,
		COALESCE(g.session_id, '') AS session_id, g.from_at, g.detected_at, g.resolved_at, b.exited_at
		FROM usage_gaps g JOIN gateway_boots b USING (gateway_id, boot_id)
		UNION ALL
		SELECT 0, b.gateway_id, b.boot_id, '', 'boot', 'pending_replay', '',
			COALESCE(b.last_record_at, b.last_heartbeat_at, b.exited_at), b.exited_at, NULL, b.exited_at
		FROM gateway_boots b
		WHERE b.exited_at IS NOT NULL AND b.replay_completed_at IS NULL
			AND NOT EXISTS (SELECT 1 FROM usage_gaps g WHERE g.gateway_id = b.gateway_id AND g.boot_id = b.boot_id)`
	var rows []usage.UsageGap
	err := queryFindings(r.db.WithContext(ctx), source, filter, after, limit).Scan(&rows).Error
	return rows, err
}

func queryFindings(db *gorm.DB, source string, filter usage.FindingFilter, after *usage.FindingKey, limit int) *gorm.DB {
	where := ` WHERE (@gateway = '' OR gateway_id = @gateway)
		AND (@boot = '' OR boot_id = @boot) AND (@tenant = '' OR tenant_id = @tenant)
		AND (@kind = '' OR kind = @kind)`
	args := []any{
		sql.Named("gateway", filter.GatewayID), sql.Named("boot", filter.BootID),
		sql.Named("tenant", filter.TenantID), sql.Named("kind", filter.Kind), sql.Named("limit", limit),
	}
	if filter.Status != "" {
		where += " AND status = @status"
		args = append(args, sql.Named("status", filter.Status))
	}
	if after != nil {
		where += " AND (detected_at, id, gateway_id, boot_id) < (@at, @id, @after_gateway, @after_boot)"
		args = append(args, sql.Named("at", after.DetectedAt), sql.Named("id", after.ID),
			sql.Named("after_gateway", after.GatewayID), sql.Named("after_boot", after.BootID))
	}
	return db.Raw("SELECT * FROM ("+source+") findings"+where+
		" ORDER BY detected_at DESC, id DESC, gateway_id DESC, boot_id DESC LIMIT @limit", args...)
}
