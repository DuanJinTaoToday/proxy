package usage

import (
	"context"
	"time"

	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/biz/access"
	"github.com/go-kratos/kratos/v3/errors"
)

type Granularity string

const (
	Minute         Granularity = "minute"
	Hour           Granularity = "hour"
	DayGranularity Granularity = "day"
)

type UsageQuery struct {
	TenantID, GatewayID string
	Start, End          time.Time
	Granularity         Granularity
	SplitByGateway      bool
	SplitByGrant        bool `json:",omitempty"`
	SplitByRate         bool `json:",omitempty"`
}

type UsageKey struct {
	StartTime time.Time
	GatewayID string
	GrantID   int64
	RateID    int64
}

type UsageBucket struct {
	UsageKey
	InputBytes, OutputBytes, BillableBytes uint64
}

type UsagePage struct {
	Buckets   []UsageBucket
	NextToken string
}

func (s *Service) QueryUsage(ctx context.Context, token string, query UsageQuery, pageSize int, pageToken string) (*UsagePage, error) {
	if err := s.access.Require(token, "", access.ReadOperations); err != nil {
		return nil, err
	}
	if query.TenantID == "" {
		return nil, errors.BadRequest("TENANT_REQUIRED", "tenant_id is required")
	}
	if err := validateUsageWindow(query, time.Now()); err != nil {
		return nil, err
	}
	query.Start, query.End = query.Start.UTC(), query.End.UTC()
	cursor, after, pageSize, err := readPage[UsageKey](query, pageSize, pageToken)
	if err != nil {
		return nil, err
	}
	if after != nil && (after.StartTime.IsZero() || !after.StartTime.Before(query.End) ||
		(!query.SplitByGateway && after.GatewayID != "") ||
		(!query.SplitByGrant && after.GrantID != 0) ||
		(!query.SplitByRate && after.RateID != 0)) {
		return nil, errInvalidPageToken
	}
	buckets, err := s.repo.QueryUsageBuckets(ctx, query, after, pageSize+1)
	if err != nil {
		return nil, err
	}
	buckets, nextToken, err := finishPage(buckets, pageSize, cursor, func(bucket UsageBucket) UsageKey { return bucket.UsageKey })
	if err != nil {
		return nil, err
	}
	for _, bucket := range buckets {
		if bucket.InputBytes+bucket.OutputBytes < bucket.InputBytes {
			return nil, errByteOverflow
		}
	}
	return &UsagePage{Buckets: buckets, NextToken: nextToken}, nil
}

func validateUsageWindow(query UsageQuery, now time.Time) error {
	start, end := query.Start, query.End
	invalid := errors.BadRequest("INVALID_TIME_RANGE", "time range exceeds the granularity or retention limits")
	if start.IsZero() || end.IsZero() || !end.After(start) {
		return invalid
	}

	var maxRange time.Duration
	switch query.Granularity {
	case Minute:
		maxRange = 24 * time.Hour
	case Hour:
		maxRange = 31 * 24 * time.Hour
	case DayGranularity:
		if end.After(start.In(shanghai).AddDate(0, 0, 366)) {
			return invalid
		}
		return nil
	default:
		return errors.BadRequest("INVALID_GRANULARITY", "granularity must be minute, hour or day")
	}
	if start.Before(usageArchiveCutoff(now)) || end.Sub(start) > maxRange {
		return invalid
	}
	return nil
}
