package repo

import (
	"context"
	"database/sql"

	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/biz/usage"
	"gorm.io/gorm"
)

func (r *UsageRepository) QueryUsageBuckets(ctx context.Context, query usage.UsageQuery, after *usage.UsageKey, limit int) ([]usage.UsageBucket, error) {
	return queryUsageBuckets(r.db.WithContext(ctx), query, after, limit)
}

func queryUsageBuckets(db *gorm.DB, query usage.UsageQuery, after *usage.UsageKey, limit int) ([]usage.UsageBucket, error) {
	// Restrict the partition key independently from the bucket key. A daily
	// query reads both stores in one statement, including during archival.
	const dimensions = `gateway_id, input_bytes, output_bytes, billable_bytes`
	const filters = ` WHERE tenant_id = @tenant
		AND (@gateway = '' OR gateway_id = @gateway)
		AND usage_date BETWEEN (CAST(@start AS timestamptz) AT TIME ZONE 'Asia/Shanghai')::date
			AND (CAST(@end AS timestamptz) AT TIME ZONE 'Asia/Shanghai')::date`

	bucketStart := "minute_start"
	switch query.Granularity {
	case usage.Hour:
		bucketStart = "date_trunc('hour', minute_start, 'Asia/Shanghai')"
	case usage.DayGranularity:
		bucketStart = "usage_date::timestamp AT TIME ZONE 'Asia/Shanghai'"
	}
	window := " AND minute_start >= @start AND minute_start < @end"
	if query.Granularity == usage.DayGranularity {
		// Select complete days by their Shanghai midnight in both stores so
		// archival cannot change the answer to an unaligned day query.
		window = " AND " + bucketStart + " >= @start AND " + bucketStart + " < @end"
	}
	source := "SELECT " + bucketStart + " AS start_time, " + dimensions + " FROM usage_minutely" + filters + window
	if query.Granularity == usage.DayGranularity {
		source += " UNION ALL SELECT " + bucketStart + " AS start_time, " + dimensions + " FROM usage_daily" + filters + window
	}

	args := []any{
		sql.Named("tenant", query.TenantID), sql.Named("gateway", query.GatewayID),
		sql.Named("start", query.Start), sql.Named("end", query.End),
		sql.Named("split_gateway", query.SplitByGateway), sql.Named("limit", limit),
	}
	cursor := ""
	if after != nil {
		cursor = " WHERE (start_time, gateway_id) > (@after_start, @after_gateway)"
		args = append(args, sql.Named("after_start", after.StartTime), sql.Named("after_gateway", after.GatewayID))
	}

	var rows []usage.UsageBucket
	err := db.Raw(`SELECT start_time, gateway_id,
		SUM(input_bytes) AS input_bytes, SUM(output_bytes) AS output_bytes, SUM(billable_bytes) AS billable_bytes
		FROM (SELECT start_time, CASE WHEN @split_gateway THEN gateway_id ELSE '' END AS gateway_id,
			input_bytes, output_bytes, billable_bytes FROM (`+source+`) source) buckets`+cursor+`
		GROUP BY start_time, gateway_id
		HAVING SUM(input_bytes) > 0 OR SUM(output_bytes) > 0 OR SUM(billable_bytes) > 0
		ORDER BY start_time, gateway_id
		LIMIT @limit`, args...).Scan(&rows).Error
	return rows, err
}
