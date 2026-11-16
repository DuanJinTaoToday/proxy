package stickysession

import (
	"context"
	"errors"
	"sync"
	"time"

	v1 "github.com/DuanJinTaoToday/proxy/api/proxycontrol/v1"
	"github.com/google/uuid"
)

var ErrRejected = errors.New("sticky session request rejected")

type Control interface {
	AcquireStickySession(context.Context, *v1.StickySessionRequest) (*v1.StickySession, error)
	CommitStickySession(context.Context, *v1.StickySessionRequest) (*v1.StickySession, error)
	ReleaseStickySession(context.Context, *v1.StickySessionRequest) error
}

type Identity struct{ Client, Key string }

type Mapping struct {
	SID       int64
	Preferred int
	until     time.Time
}

type Attempt struct {
	Mapping
	identity Identity
	request  *v1.StickySessionRequest
}

type Manager struct {
	control   Control
	mu        sync.Mutex
	confirmed map[Identity]Mapping
}

func New(control Control) *Manager {
	return &Manager{control: control, confirmed: make(map[Identity]Mapping)}
}

func (m *Manager) Resolve(ctx context.Context, identity Identity, reference string) (Attempt, error) {
	req := &v1.StickySessionRequest{Reference: reference, SessionKey: identity.Key, RequestKey: uuid.NewString()}
	started := time.Now()
	session, err := m.control.AcquireStickySession(ctx, req)
	if err != nil && !errors.Is(err, ErrRejected) && ctx.Err() == nil {
		session, err = m.control.AcquireStickySession(ctx, req)
	}
	m.mu.Lock()
	prior := m.confirmed[identity]
	if errors.Is(err, ErrRejected) {
		delete(m.confirmed, identity)
	}
	m.mu.Unlock()
	if err != nil {
		if !errors.Is(err, ErrRejected) && ctx.Err() == nil && time.Now().Before(prior.until) {
			return Attempt{Mapping: prior, identity: identity}, nil
		}
		return Attempt{}, err
	}
	sid := session.GetUpstreamSid()
	if sid < 100000000 || sid >= 900000000 {
		return Attempt{}, ErrRejected
	}
	attempt := Attempt{Mapping: Mapping{SID: sid, Preferred: -1}, identity: identity}
	if lease := session.GetLeaseToken(); lease != "" {
		req.LeaseToken = lease
		attempt.request = req
		return attempt, nil
	}
	attempt.until = confirmedUntil(started, session)
	if !time.Now().Before(attempt.until) {
		return Attempt{}, ErrRejected
	}
	if prior.SID == sid && time.Now().Before(prior.until) {
		attempt.Preferred = prior.Preferred
	}
	return attempt, nil
}

func confirmedUntil(started time.Time, session *v1.StickySession) time.Time {
	millis := session.GetExpiresInMillis()
	if millis <= 0 || millis > time.Hour.Milliseconds() {
		return time.Time{}
	}
	return started.Add(time.Duration(millis) * time.Millisecond)
}

func (m *Manager) Confirm(ctx context.Context, attempt *Attempt, entry int) error {
	if attempt.request != nil {
		started := time.Now()
		session, err := m.control.CommitStickySession(ctx, attempt.request)
		if err != nil && !errors.Is(err, ErrRejected) && ctx.Err() == nil {
			session, err = m.control.CommitStickySession(ctx, attempt.request)
		}
		if err != nil {
			return err
		}
		attempt.until = confirmedUntil(started, session)
		if session.GetUpstreamSid() != attempt.SID || !time.Now().Before(attempt.until) {
			return ErrRejected
		}
		attempt.request = nil
	}
	// An established connection may finish its handshake after expiry, but
	// the expired mapping must never be offered to another new connection.
	m.mu.Lock()
	defer m.mu.Unlock()
	if time.Now().Before(attempt.until) && m.confirmed[attempt.identity].SID <= attempt.SID {
		attempt.Preferred = entry
		m.confirmed[attempt.identity] = attempt.Mapping
	}
	return nil
}

func (m *Manager) Release(ctx context.Context, attempt *Attempt) {
	if attempt.request == nil {
		return
	}
	releaseCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), time.Second)
	defer cancel()
	_ = m.control.ReleaseStickySession(releaseCtx, attempt.request)
}

func (m *Manager) Prune(now time.Time) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for key, session := range m.confirmed {
		if !now.Before(session.until) {
			delete(m.confirmed, key)
		}
	}
}
