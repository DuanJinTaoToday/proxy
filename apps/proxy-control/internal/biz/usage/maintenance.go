package usage

import (
	"context"
	"time"
)

func (s *Service) Maintain(ctx context.Context) error {
	return s.Archive(ctx, time.Now())
}
