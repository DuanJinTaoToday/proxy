package usage

import (
	"context"
	"time"

	"github.com/go-kratos/kratos/v3/errors"
)

func (s *Service) ReportLifecycle(ctx context.Context, token, gatewayID, bootID string, exit *time.Time, invalidExit bool) error {
	hash, err := reportToken(token)
	if err != nil {
		return err
	}
	if !validID(gatewayID) || !validID(bootID) {
		return errors.BadRequest("INVALID_LIFECYCLE", "gateway and boot IDs are required")
	}
	now := time.Now().UTC()
	if invalidExit {
		return errors.BadRequest("INVALID_LIFECYCLE", "invalid exit timestamp")
	}
	var heartbeat *time.Time
	if exit == nil {
		heartbeat = &now
	}
	return s.repo.Transact(ctx, func(tx Transaction) error {
		if err := authorizeGateway(tx, hash, gatewayID); err != nil {
			return err
		}
		return tx.ReportLifecycle(gatewayID, bootID, now, heartbeat, exit)
	})
}
