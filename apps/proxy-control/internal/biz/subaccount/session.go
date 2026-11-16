package subaccount

import (
	"context"
	"errors"
	"regexp"

	kerrors "github.com/go-kratos/kratos/v3/errors"
	"github.com/google/uuid"
)

type StickySession struct {
	UpstreamSID     int64
	ExpiresInMillis int64
	LeaseToken      string
}

type SessionRequest struct {
	Reference, Key, Attempt string
}

var (
	ErrSessionBusy  = errors.New("session first dial in progress")
	ErrSessionLease = errors.New("invalid session lease")
	sessionKey      = regexp.MustCompile(`^[a-z0-9]{0,32}$`)
)

func (s *Service) sessionGateway(ctx context.Context, token string, req SessionRequest, validate func() error) (string, error) {
	gateway, err := s.gateway(ctx, token)
	if err != nil {
		return "", err
	}
	_, refErr := uuid.Parse(req.Reference)
	_, attemptErr := uuid.Parse(req.Attempt)
	if validate() != nil || refErr != nil || attemptErr != nil || !sessionKey.MatchString(req.Key) {
		return "", kerrors.BadRequest("INVALID_SESSION", "invalid session reference, key or attempt")
	}
	return gateway, nil
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

func (s *Service) Acquire(ctx context.Context, token string, req SessionRequest, validate func() error) (StickySession, error) {
	gateway, err := s.sessionGateway(ctx, token, req, validate)
	if err != nil {
		return StickySession{}, err
	}
	session, err := s.repo.Acquire(ctx, gateway, req)
	return session, sessionError(err)
}

func (s *Service) Commit(ctx context.Context, token string, req SessionRequest, validate func() error) (StickySession, error) {
	gateway, err := s.sessionGateway(ctx, token, req, validate)
	if err != nil {
		return StickySession{}, err
	}
	session, err := s.repo.Commit(ctx, gateway, req)
	return session, sessionError(err)
}

func (s *Service) Release(ctx context.Context, token string, req SessionRequest, validate func() error) error {
	gateway, err := s.sessionGateway(ctx, token, req, validate)
	if err != nil {
		return err
	}
	return sessionError(s.repo.Release(ctx, gateway, req))
}
