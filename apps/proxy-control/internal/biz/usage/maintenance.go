package usage

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// Maintain archives usage and settles traffic grants for the same point in time.
func (s *Service) Maintain(ctx context.Context) error {
	now := time.Now()
	archiveErr := s.Archive(ctx, now)
	if archiveErr != nil {
		archiveErr = fmt.Errorf("archive usage: %w", archiveErr)
	}
	settleErr := s.SettleGrants(ctx, now)
	if settleErr != nil {
		settleErr = fmt.Errorf("settle traffic grants: %w", settleErr)
	}
	return errors.Join(archiveErr, settleErr)
}
