package usage

import (
	"context"
	"time"

	"github.com/go-kratos/kratos/v3/errors"
)

type TrafficAlert struct {
	AvailableBytes int64
	ThresholdBytes *int64
	Occurrence     int64
	TriggeredAt    *time.Time
}

func (s *Service) TrafficAlert(ctx context.Context, token string) (TrafficAlert, error) {
	tenant, err := s.access.Account(ctx, token)
	if err != nil {
		return TrafficAlert{}, err
	}
	return s.repo.TrafficAlert(ctx, tenant.ID)
}

func (s *Service) SetTrafficAlert(ctx context.Context, token string, threshold *int64) (TrafficAlert, error) {
	tenant, err := s.access.Account(ctx, token)
	if err != nil {
		return TrafficAlert{}, err
	}
	if threshold != nil && *threshold < 0 {
		return TrafficAlert{}, errors.BadRequest("INVALID_THRESHOLD", "threshold_bytes must be non-negative")
	}
	return s.repo.SetTrafficAlert(ctx, tenant.ID, threshold)
}
