package usage

import (
	"context"
	"time"

	v1 "github.com/DuanJinTaoToday/proxy/api/proxycontrol/v1"
	"github.com/go-kratos/kratos/v3/errors"
	"gorm.io/gorm"
)

func (s *Service) ReportLifecycle(ctx context.Context, req *v1.ReportLifecycleRequest) (*v1.ReportLifecycleResponse, error) {
	hash, err := reportToken(ctx)
	if err != nil {
		return nil, err
	}
	if !validID(req.GetGatewayId()) || !validID(req.GetBootId()) {
		return nil, errors.BadRequest("INVALID_LIFECYCLE", "gateway and boot IDs are required")
	}
	now := time.Now().UTC()
	var heartbeat, exit *time.Time
	if at := req.GetExitedAt(); at != nil {
		if !at.IsValid() {
			return nil, errors.BadRequest("INVALID_LIFECYCLE", "invalid exit timestamp")
		}
		value := at.AsTime()
		exit = &value
	} else {
		heartbeat = &now
	}
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := authorizeGateway(tx, hash, req.GetGatewayId()); err != nil {
			return err
		}
		return tx.Exec(`INSERT INTO gateway_boots (gateway_id, boot_id, first_seen_at, last_heartbeat_at, exited_at)
			VALUES (?, ?, ?, ?, ?)
			ON CONFLICT (gateway_id, boot_id) DO UPDATE SET
			last_heartbeat_at = CASE WHEN gateway_boots.exited_at IS NULL AND EXCLUDED.exited_at IS NULL
				THEN GREATEST(gateway_boots.last_heartbeat_at, EXCLUDED.last_heartbeat_at) ELSE gateway_boots.last_heartbeat_at END,
			exited_at = COALESCE(gateway_boots.exited_at, EXCLUDED.exited_at)`,
			req.GetGatewayId(), req.GetBootId(), now, heartbeat, exit).Error
	})
	if err != nil {
		return nil, err
	}
	return &v1.ReportLifecycleResponse{}, nil
}
