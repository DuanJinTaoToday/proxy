package subaccount

import (
	"context"
	"errors"
	"regexp"

	kerrors "github.com/go-kratos/kratos/v3/errors"
	"github.com/google/uuid"
)

type StickySession struct {
	UpstreamSID int64 `gorm:"column:upstream_sid"`
	LeaseToken  string
}

var (
	ErrSessionBusy  = errors.New("session first dial in progress")
	ErrSessionLease = errors.New("invalid session lease")
	sessionKey      = regexp.MustCompile(`^[a-z0-9]{0,32}$`)
)

func (s *Service) sessionGateway(ctx context.Context, token, client, key string) (string, error) {
	if _, err := uuid.Parse(client); err != nil || !sessionKey.MatchString(key) {
		return "", kerrors.BadRequest("INVALID_SESSION", "invalid client or session identifier")
	}
	return s.gateway(ctx, token)
}

func sessionError(err error) error {
	switch {
	case errors.Is(err, ErrSessionBusy):
		return kerrors.Conflict("SESSION_ESTABLISHING", "session is being established")
	case errors.Is(err, ErrSessionLease):
		return kerrors.Conflict("INVALID_SESSION_LEASE", "session lease is no longer valid")
	case err != nil:
		return unavailable()
	default:
		return nil
	}
}

func (s *Service) Acquire(ctx context.Context, token, client, key string) (StickySession, error) {
	gateway, err := s.sessionGateway(ctx, token, client, key)
	if err != nil {
		return StickySession{}, err
	}
	session, err := s.repo.Acquire(ctx, gateway, client, key)
	return session, sessionError(err)
}

func (s *Service) Commit(ctx context.Context, token, client, key, lease string) (StickySession, error) {
	gateway, err := s.sessionGateway(ctx, token, client, key)
	if err != nil {
		return StickySession{}, err
	}
	session, err := s.repo.Commit(ctx, gateway, client, key, lease)
	return session, sessionError(err)
}

func (s *Service) Release(ctx context.Context, token, client, key, lease string) error {
	gateway, err := s.sessionGateway(ctx, token, client, key)
	if err != nil {
		return err
	}
	return sessionError(s.repo.Release(ctx, gateway, client, key, lease))
}
