package repo

import (
	"context"
	"database/sql"

	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/biz/usage"
	"gorm.io/gorm"
)

const anomalyFindings = `SELECT a.id, a.gateway_id, a.boot_id, COALESCE(s.tenant_id,'') AS tenant_id,
		a.kind, a.file_id, a.file_offset, a.detected_at, a.resolved_at
		FROM record_anomalies a LEFT JOIN proxy_subaccounts s ON s.client_id=a.detail->>'client_id'`

func (r *UsageRepository) ListRecordAnomalies(ctx context.Context, filter usage.FindingFilter, after *usage.FindingKey, limit int) ([]usage.RecordAnomaly, error) {
	var rows []usage.RecordAnomaly
	err := queryFindings(r.db.WithContext(ctx), anomalyFindings, filter, after, limit).Scan(&rows).Error
	return rows, err
}

// Session reports are provisional until the collector completes replay.
// Before any reports arrive, expose one pending row for the exited boot.
const gapFindings = `SELECT g.id, g.gateway_id, g.boot_id, COALESCE(g.tenant_id, '') AS tenant_id,
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

func (r *UsageRepository) ListUsageGaps(ctx context.Context, filter usage.FindingFilter, after *usage.FindingKey, limit int) ([]usage.UsageGap, error) {
	var rows []usage.UsageGap
	err := queryFindings(r.db.WithContext(ctx), gapFindings, filter, after, limit).Scan(&rows).Error
	return rows, err
}

func (r *UsageRepository) CountOpenRecordAnomalies(ctx context.Context, filter usage.FindingFilter) (int64, error) {
	return countOpenFindings(r.db.WithContext(ctx), anomalyFindings, filter)
}

func (r *UsageRepository) CountOpenUsageGaps(ctx context.Context, filter usage.FindingFilter) (int64, error) {
	return countOpenFindings(r.db.WithContext(ctx), gapFindings, filter)
}

func countOpenFindings(db *gorm.DB, source string, filter usage.FindingFilter) (int64, error) {
	filter.Status, filter.Resolution = "", "open"
	where, args := findingConditions(filter)
	var count int64
	err := db.Raw("SELECT count(*) FROM ("+source+") findings"+where, args...).Scan(&count).Error
	return count, err
}

func findingConditions(filter usage.FindingFilter) (string, []any) {
	where := ` WHERE (@gateway = '' OR gateway_id = @gateway)
		AND (@boot = '' OR boot_id = @boot) AND (@tenant = '' OR tenant_id = @tenant)
		AND (@kind = '' OR kind = @kind)`
	args := []any{
		sql.Named("gateway", filter.GatewayID), sql.Named("boot", filter.BootID),
		sql.Named("tenant", filter.TenantID), sql.Named("kind", filter.Kind),
	}
	switch filter.Resolution {
	case "open":
		where += " AND resolved_at IS NULL"
	case "resolved":
		where += " AND resolved_at IS NOT NULL"
	}
	return where, args
}

func queryFindings(db *gorm.DB, source string, filter usage.FindingFilter, after *usage.FindingKey, limit int) *gorm.DB {
	where, args := findingConditions(filter)
	if filter.Status != "" {
		where += " AND status = @status"
		args = append(args, sql.Named("status", filter.Status))
	}
	if after != nil {
		where += " AND (detected_at, id, gateway_id, boot_id) < (@at, @id, @after_gateway, @after_boot)"
		args = append(args, sql.Named("at", after.DetectedAt), sql.Named("id", after.ID),
			sql.Named("after_gateway", after.GatewayID), sql.Named("after_boot", after.BootID))
	}
	order := " ORDER BY detected_at DESC, id DESC, gateway_id DESC, boot_id DESC"
	args = append(args, sql.Named("limit", limit))
	return db.Raw("SELECT page.*, COALESCE(t.name, '') AS tenant_name FROM (SELECT * FROM ("+source+") findings"+where+order+
		" LIMIT @limit) page LEFT JOIN tenants t ON t.id = page.tenant_id"+
		" ORDER BY page.detected_at DESC, page.id DESC, page.gateway_id DESC, page.boot_id DESC", args...)
}
