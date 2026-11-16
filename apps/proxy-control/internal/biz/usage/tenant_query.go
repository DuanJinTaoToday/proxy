package usage

import (
	"context"
	"time"

	"github.com/go-kratos/kratos/v3/errors"
	"github.com/google/uuid"
)

type DailyUsage struct {
	Date          string
	BillableBytes int64
	UpdatedAt     *time.Time
}

type DailyUsageFilter struct {
	ClientID, StartDate, EndDate string
}

func (s *Service) DailyUsage(ctx context.Context, token string, filter DailyUsageFilter) ([]DailyUsage, error) {
	tenant, err := s.access.Account(ctx, token)
	if err != nil {
		return nil, err
	}
	if filter.ClientID != "" {
		id, err := uuid.Parse(filter.ClientID)
		if len(filter.ClientID) != 36 || err != nil {
			return nil, errors.BadRequest("INVALID_CLIENT_ID", "client_id must be a valid UUID")
		}
		filter.ClientID = id.String()
	}
	start, end := filter.StartDate, filter.EndDate
	if start == "" && end == "" {
		start = time.Now().In(shanghai).Format(time.DateOnly)
	}
	if start == "" {
		start = end
	}
	if end == "" {
		end = start
	}
	first, firstErr := time.Parse(time.DateOnly, start)
	last, lastErr := time.Parse(time.DateOnly, end)
	if firstErr != nil || lastErr != nil || first.Year() < 1 || last.Before(first) {
		return nil, errors.BadRequest("INVALID_DATE_RANGE", "expected an inclusive YYYY-MM-DD range")
	}
	filter.StartDate, filter.EndDate = start, end
	return s.repo.QueryDailyUsage(ctx, tenant.ID, filter)
}
