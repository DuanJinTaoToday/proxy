package usage

import (
	"context"
	"time"

	"github.com/go-kratos/kratos/v3/errors"
)

type UnfinishedSession struct {
	SessionID, ClientID string
	LastRecordedAt      time.Time
}

func (s *Service) ReportUnfinishedSessions(ctx context.Context, token, gatewayID, bootID string, sessions []UnfinishedSession) error {
	if len(sessions) == 0 || len(sessions) > 1000 {
		return errors.BadRequest("INVALID_REPLAY", "expected 1 to 1000 unfinished sessions")
	}
	for _, session := range sessions {
		if !validID(session.SessionID) || !validID(session.ClientID) || session.LastRecordedAt.IsZero() {
			return errors.BadRequest("INVALID_REPLAY", "invalid unfinished session")
		}
	}
	return s.withReplay(ctx, token, gatewayID, bootID, func(tx Transaction, completed bool) error {
		if completed {
			// A lost completion response can make the collector resend all sessions.
			return nil
		}
		now := time.Now().UTC()
		for _, session := range sessions {
			if err := tx.SaveSessionGap(gatewayID, bootID, session, now); err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *Service) CompleteReplay(ctx context.Context, token, gatewayID, bootID string, count uint32, hasSessionRecords *bool) error {
	return s.withReplay(ctx, token, gatewayID, bootID, func(tx Transaction, completed bool) error {
		actual, err := tx.CountSessionGaps(gatewayID, bootID)
		if err != nil {
			return err
		}
		if actual != int64(count) {
			return errors.Conflict("UNFINISHED_SESSIONS_MISMATCH", "unfinished session count differs")
		}
		if completed {
			return nil
		}
		return tx.FinishReplay(gatewayID, bootID, hasSessionRecords, time.Now().UTC())
	})
}

func (s *Service) withReplay(ctx context.Context, token, gatewayID, bootID string, run func(Transaction, bool) error) error {
	hash, err := reportToken(token)
	if err != nil {
		return err
	}
	if !validID(gatewayID) || !validID(bootID) {
		return errors.BadRequest("INVALID_REPLAY", "gateway and boot IDs are required")
	}
	return s.repo.Transact(ctx, func(tx Transaction) error {
		if err := authorizeGateway(tx, hash, gatewayID); err != nil {
			return err
		}
		exited, completed, err := tx.LockReplay(gatewayID, bootID)
		if err != nil {
			return err
		}
		if !exited {
			return errors.Conflict("BOOT_NOT_EXITED", "boot exit has not been confirmed")
		}
		return run(tx, completed)
	})
}
