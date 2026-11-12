package usage

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
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
}

type UsageKey struct {
	StartTime time.Time
	GatewayID string
}

type UsageBucket struct {
	UsageKey
	InputBytes, OutputBytes, BillableBytes uint64
}

type UsagePage struct {
	Buckets   []UsageBucket
	NextToken string
}

type usageCursor struct {
	QueryHash [sha256.Size]byte
	Key       UsageKey
}

func (s *Service) QueryUsage(ctx context.Context, token string, query UsageQuery, splitBilling bool, pageSize int, pageToken string) (*UsagePage, error) {
	if err := s.access.Require(token, "", access.ReadOperations); err != nil {
		return nil, err
	}
	if query.TenantID == "" {
		return nil, errors.BadRequest("TENANT_REQUIRED", "tenant_id is required")
	}
	if splitBilling {
		return nil, errors.BadRequest("UNSUPPORTED_USAGE_QUERY", "grant and rate splits are not yet supported")
	}
	if err := validateUsageWindow(query, time.Now()); err != nil {
		return nil, err
	}
	if pageSize == 0 {
		pageSize = 100
	}
	if pageSize < 1 || pageSize > 500 {
		return nil, errors.BadRequest("INVALID_PAGE_SIZE", "page_size must be between 1 and 500")
	}

	// Normalize equivalent RFC 3339 timestamps before binding the cursor to the query.
	query.Start, query.End = query.Start.UTC(), query.End.UTC()
	encoded, err := json.Marshal(query)
	if err != nil {
		return nil, err
	}
	cursor := usageCursor{QueryHash: sha256.Sum256(encoded)}
	var after *UsageKey
	if pageToken != "" {
		if len(pageToken) > 2048 {
			return nil, errors.BadRequest("INVALID_PAGE_TOKEN", "page_token does not match the usage query")
		}
		data, err := base64.RawURLEncoding.DecodeString(pageToken)
		var decoded usageCursor
		if err != nil || json.Unmarshal(data, &decoded) != nil ||
			decoded.QueryHash != cursor.QueryHash || decoded.Key.StartTime.IsZero() || !decoded.Key.StartTime.Before(query.End) ||
			(!query.SplitByGateway && decoded.Key.GatewayID != "") {
			return nil, errors.BadRequest("INVALID_PAGE_TOKEN", "page_token does not match the usage query")
		}
		after = &decoded.Key
	}

	buckets, err := s.repo.QueryUsageBuckets(ctx, query, after, pageSize+1)
	if err != nil {
		return nil, err
	}
	page := &UsagePage{Buckets: buckets}
	if len(buckets) > pageSize {
		page.Buckets = buckets[:pageSize]
		cursor.Key = page.Buckets[pageSize-1].UsageKey
		encoded, err := json.Marshal(cursor)
		if err != nil {
			return nil, err
		}
		page.NextToken = base64.RawURLEncoding.EncodeToString(encoded)
	}
	for _, bucket := range page.Buckets {
		if bucket.InputBytes+bucket.OutputBytes < bucket.InputBytes {
			return nil, errByteOverflow
		}
	}
	return page, nil
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
