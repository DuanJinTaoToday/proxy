package usage

import (
	"context"
	"time"

	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/biz/access"
	"github.com/go-kratos/kratos/v3/errors"
)

type AuditPage[T any] struct {
	Items     []T
	NextToken string
	OpenCount int64
}

type FindingFilter struct {
	GatewayID, BootID, TenantID, Kind, Status string
	Resolution                                string `json:",omitempty"`
}

type FindingKey struct {
	DetectedAt        time.Time
	ID                int64
	GatewayID, BootID string
}

type RecordAnomaly struct {
	FindingKey
	TenantID, TenantName, Kind, FileID string
	FileOffset                         uint64
	ResolvedAt                         *time.Time
}

type UsageGap struct {
	FindingKey
	TenantID, TenantName, Kind, Status, SessionID string
	FromAt                                        time.Time
	ResolvedAt                                    *time.Time
	ExitedAt                                      *time.Time
}

func (s *Service) ListRecordAnomalies(ctx context.Context, token string, filter FindingFilter, size int, pageToken string) (AuditPage[RecordAnomaly], error) {
	if err := s.access.Require(token, "", access.ReadOperations); err != nil {
		return AuditPage[RecordAnomaly]{}, err
	}
	return readFindingPage("anomalies", filter, size, pageToken, func(after *FindingKey, limit int) ([]RecordAnomaly, error) {
		return s.repo.ListRecordAnomalies(ctx, filter, after, limit)
	}, func(row RecordAnomaly) FindingKey { return row.FindingKey }, func() (int64, error) {
		return s.repo.CountOpenRecordAnomalies(ctx, filter)
	})
}

func (s *Service) ListUsageGaps(ctx context.Context, token string, filter FindingFilter, size int, pageToken string) (AuditPage[UsageGap], error) {
	if err := s.access.Require(token, "", access.ReadOperations); err != nil {
		return AuditPage[UsageGap]{}, err
	}
	if filter.Kind != "" && filter.Kind != "boot" && filter.Kind != "session" {
		return AuditPage[UsageGap]{}, errors.BadRequest("INVALID_GAP_KIND", "kind must be boot or session")
	}
	if filter.Status != "" && filter.Status != "pending_replay" && filter.Status != "possible_gap" {
		return AuditPage[UsageGap]{}, errors.BadRequest("INVALID_GAP_STATUS", "status must be pending_replay or possible_gap")
	}
	return readFindingPage("gaps", filter, size, pageToken, func(after *FindingKey, limit int) ([]UsageGap, error) {
		return s.repo.ListUsageGaps(ctx, filter, after, limit)
	}, func(row UsageGap) FindingKey { return row.FindingKey }, func() (int64, error) {
		return s.repo.CountOpenUsageGaps(ctx, filter)
	})
}

func readFindingPage[T any](list string, filter FindingFilter, size int, token string, load func(*FindingKey, int) ([]T, error), key func(T) FindingKey, countOpen func() (int64, error)) (AuditPage[T], error) {
	var page AuditPage[T]
	if filter.Resolution == "all" {
		filter.Resolution = ""
	}
	if filter.Resolution != "" && filter.Resolution != "open" && filter.Resolution != "resolved" {
		return page, errors.BadRequest("INVALID_FINDING_RESOLUTION", "resolution must be open, resolved, or all")
	}
	scope := struct {
		List   string
		Filter FindingFilter
	}{list, filter}
	cursor, after, size, err := readPage[FindingKey](scope, size, token)
	if err != nil {
		return page, err
	}
	if after != nil && (after.DetectedAt.IsZero() || after.ID < 0 || after.GatewayID == "" || after.BootID == "") {
		return page, errInvalidPageToken
	}
	rows, err := load(after, size+1)
	if err != nil {
		return page, err
	}
	page.Items, page.NextToken, err = finishPage(rows, size, cursor, key)
	if err == nil && after == nil {
		page.OpenCount, err = countOpen()
	}
	return page, err
}
