package repo

import (
	"context"
	"database/sql"
	"time"

	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/biz/usage"
	"gorm.io/gorm"
)

func (r *UsageRepository) DailyUsage(ctx context.Context, id string, start, end time.Time) ([]usage.DailyTotals, *time.Time, error) {
	var rows []usage.DailyTotals
	var balance struct{ LastReceivedAt *time.Time }
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		buckets, err := queryUsageBuckets(tx, usage.UsageQuery{
			TenantID: id, Start: start, End: end, Granularity: usage.DayGranularity,
		}, nil, 93)
		if err != nil {
			return err
		}
		for _, bucket := range buckets {
			rows = append(rows, usage.DailyTotals{
				Date:       bucket.StartTime.In(start.Location()).Format(time.DateOnly),
				InputBytes: bucket.InputBytes, OutputBytes: bucket.OutputBytes, BillableBytes: bucket.BillableBytes,
			})
		}
		return tx.Table("tenant_balances").Select("last_received_at").Where("tenant_id = ?", id).Find(&balance).Error
	}, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	return rows, balance.LastReceivedAt, err
}

func (r *UsageRepository) QueryUsageBuckets(ctx context.Context, query usage.UsageQuery, after *usage.UsageKey, limit int) ([]usage.UsageBucket, error) {
	return queryUsageBuckets(r.db.WithContext(ctx), query, after, limit)
}

func queryUsageBuckets(db *gorm.DB, query usage.UsageQuery, after *usage.UsageKey, limit int) ([]usage.UsageBucket, error) {
	// Restrict the partition key independently from the bucket key. A daily
	// query reads both stores in one statement, including during archival.
	const dimensions = `gateway_id, grant_id, rate_id, input_bytes, output_bytes, billable_bytes`
	const filters = ` WHERE tenant_id = @tenant AND source = 'tunnel'
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
		sql.Named("split_gateway", query.SplitByGateway), sql.Named("split_grant", query.SplitByGrant),
		sql.Named("split_rate", query.SplitByRate), sql.Named("limit", limit),
	}
	cursor := ""
	if after != nil {
		cursor = " WHERE (start_time, gateway_id, grant_id, rate_id) > (@after_start, @after_gateway, @after_grant, @after_rate)"
		args = append(args, sql.Named("after_start", after.StartTime), sql.Named("after_gateway", after.GatewayID),
			sql.Named("after_grant", after.GrantID), sql.Named("after_rate", after.RateID))
	}

	var rows []usage.UsageBucket
	err := db.Raw(`SELECT start_time, gateway_id, grant_id, rate_id,
		SUM(input_bytes) AS input_bytes, SUM(output_bytes) AS output_bytes, SUM(billable_bytes) AS billable_bytes
		FROM (SELECT start_time, CASE WHEN @split_gateway THEN gateway_id ELSE '' END AS gateway_id,
			CASE WHEN @split_grant THEN grant_id ELSE 0 END AS grant_id,
			CASE WHEN @split_rate THEN rate_id ELSE 0 END AS rate_id,
			input_bytes, output_bytes, billable_bytes FROM (`+source+`) source) buckets`+cursor+`
		GROUP BY start_time, gateway_id, grant_id, rate_id
		HAVING SUM(input_bytes) > 0 OR SUM(output_bytes) > 0 OR SUM(billable_bytes) > 0
		ORDER BY start_time, gateway_id, grant_id, rate_id
		LIMIT @limit`, args...).Scan(&rows).Error
	return rows, err
}
