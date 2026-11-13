package authrelay

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"errors"
	"net/netip"
	"sync"
	"time"

	v1 "github.com/DuanJinTaoToday/proxy/api/proxycontrol/v1"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-gateway/internal/pkg/authmetrics"
	"golang.org/x/sync/singleflight"
)

const (
	stateLifetime = 30 * time.Second
	syncTimeout   = 200 * time.Millisecond
)

var ErrGatewayDenied = errors.New("gateway authentication rejected")
var ErrCredentialDenied = errors.New("credential authentication rejected")
var ErrCredentialDisabled = errors.New("credential disabled")
var ErrCredentialExpired = errors.New("credential expired")

type Control interface {
	Load(context.Context, string, string) (*v1.AuthSnapshot, error)
	Refresh(context.Context, []string) ([]*v1.AuthSnapshot, error)
}

type credential struct {
	snapshot *v1.AuthSnapshot
	synced   time.Time
}

type quota struct {
	snapshot *v1.AuthQuota
	synced   time.Time
}

type Relay struct {
	control     Control
	now         func() time.Time
	loads       singleflight.Group
	mu          sync.RWMutex
	credentials map[string]credential
	quotas      map[string]quota
	epoch       uint64
	reporting   func() (uint64, bool)
}

func New(control Control, now func() time.Time) *Relay {
	if now == nil {
		now = time.Now
	}
	return &Relay{control: control, now: now,
		credentials: make(map[string]credential), quotas: make(map[string]quota)}
}

func NewWithReporting(control Control, reporting func() (uint64, bool)) *Relay {
	relay := New(control, nil)
	relay.reporting = reporting
	return relay
}

func (r *Relay) quotaRenewal() (uint64, bool) {
	if r.reporting == nil {
		return 0, true
	}
	return r.reporting()
}

func (r *Relay) Authenticate(ctx context.Context, username, password, client string) (string, string) {
	if username == "" || password == "" || len(username) > 255 || len(password) > 255 {
		return "", "INVALID_CREDENTIAL"
	}
	now := r.now()
	digest := sha256.Sum256([]byte(password))
	entry, balance, epoch := r.state(username)
	if entry.snapshot == nil || !now.Before(entry.synced.Add(stateLifetime)) {
		authmetrics.Counters.Add("credential_cache_miss", 1)
		started := time.Now()
		defer authmetrics.RecordLatency("cold_load", started)
		if err := r.coldLoad(ctx, username, password, digest, epoch); err != nil {
			switch {
			case errors.Is(err, ErrCredentialDenied):
				return "", "INVALID_CREDENTIAL"
			case errors.Is(err, ErrCredentialDisabled):
				return "", "CREDENTIAL_DISABLED"
			case errors.Is(err, ErrCredentialExpired):
				return "", "CREDENTIAL_EXPIRED"
			default:
				return "", "AUTH_STATE_UNAVAILABLE"
			}
		}
		entry, balance, _ = r.state(username)
		now = r.now()
	} else {
		authmetrics.Counters.Add("credential_cache_hit", 1)
	}
	c := entry.snapshot
	switch {
	case c == nil || !now.Before(entry.synced.Add(stateLifetime)):
		return "", "AUTH_STATE_UNAVAILABLE"
	case c.GetExpiresAt() != nil && !now.Before(c.GetExpiresAt().AsTime()):
		return "", "CREDENTIAL_EXPIRED"
	case !c.GetActive():
		return "", "CREDENTIAL_DISABLED"
	case subtle.ConstantTimeCompare(c.GetPasswordHash(), digest[:]) != 1:
		return "", "INVALID_CREDENTIAL"
	case !allowedSource(client, c.GetAllowedSourceCidrs()):
		return "", "SOURCE_NOT_ALLOWED"
	case !validQuota(balance, now):
		authmetrics.Counters.Add("quota_cache_miss", 1)
		return "", "AUTH_STATE_UNAVAILABLE"
	case balance.snapshot.GetAvailableBytes() < 0 || uint64(balance.snapshot.GetAvailableBytes()) < balance.snapshot.GetAdmissionThresholdBytes():
		return "", "QUOTA_INSUFFICIENT"
	default:
		authmetrics.Counters.Add("quota_cache_hit", 1)
		return c.GetClientId(), "OK"
	}
}

func validQuota(q quota, now time.Time) bool {
	return q.snapshot != nil && now.Before(q.synced.Add(stateLifetime)) &&
		(q.snapshot.GetValidUntil() == nil || now.Before(q.snapshot.GetValidUntil().AsTime()))
}

func allowedSource(client string, cidrs []string) bool {
	if len(cidrs) == 0 {
		return true
	}
	var address netip.Addr
	if addrPort, err := netip.ParseAddrPort(client); err == nil {
		address = addrPort.Addr()
	} else {
		address, _ = netip.ParseAddr(client)
	}
	for _, cidr := range cidrs {
		prefix, err := netip.ParsePrefix(cidr)
		if err == nil && prefix.Contains(address.Unmap()) {
			return true
		}
	}
	return false
}

func (r *Relay) state(username string) (credential, quota, uint64) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	entry := r.credentials[username]
	return entry, r.quotas[entry.snapshot.GetTenantId()], r.epoch
}

func (r *Relay) coldLoad(ctx context.Context, username, password string, digest [32]byte, epoch uint64) error {
	ctx, cancel := context.WithTimeout(ctx, syncTimeout)
	defer cancel()
	// Equal credentials can share a lookup. A wrong password cannot block a
	// concurrent lookup for the same username with a different password.
	key := username + "\x00" + string(digest[:])
	result := r.loads.DoChan(key, func() (any, error) {
		lookup, stop := context.WithTimeout(context.WithoutCancel(ctx), syncTimeout)
		defer stop()
		started := r.now()
		generation, renewQuota := r.quotaRenewal()
		snapshot, err := r.control.Load(lookup, username, password)
		if err != nil {
			r.invalidateOnDenial(err, epoch)
			return nil, err
		}
		if !validSnapshot(snapshot) || snapshot.GetUsername() != username {
			return nil, errors.New("invalid authentication state")
		}
		if err := lookup.Err(); err != nil {
			return nil, err
		}
		r.apply([]*v1.AuthSnapshot{snapshot}, started, epoch, generation, renewQuota)
		return nil, nil
	})
	select {
	case <-ctx.Done():
		return ctx.Err()
	case out := <-result:
		return out.Err
	}
}

func validSnapshot(s *v1.AuthSnapshot) bool {
	return s != nil && s.GetUsername() != "" && s.GetReference() != "" && s.GetClientId() != "" &&
		s.GetTenantId() != "" && (!s.GetActive() || len(s.GetPasswordHash()) == sha256.Size && s.GetQuota() != nil)
}

func (r *Relay) apply(snapshots []*v1.AuthSnapshot, started time.Time, epoch, generation uint64, renewQuota bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.epoch != epoch {
		return
	}
	currentGeneration, reportingReady := r.quotaRenewal()
	renewQuota = renewQuota && reportingReady && currentGeneration == generation
	for _, s := range snapshots {
		if existing := r.credentials[s.GetUsername()]; !started.Before(existing.synced) &&
			(existing.snapshot == nil || existing.snapshot.GetActive() || !s.GetActive()) {
			r.credentials[s.GetUsername()] = credential{s, started}
		}
		if existing := r.quotas[s.GetTenantId()]; renewQuota && s.GetQuota() != nil && !started.Before(existing.synced) {
			r.quotas[s.GetTenantId()] = quota{s.GetQuota(), started}
		}
	}
}

func (r *Relay) invalidateOnDenial(err error, epoch uint64) {
	if !errors.Is(err, ErrGatewayDenied) {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.epoch == epoch {
		clear(r.credentials)
		clear(r.quotas)
		r.epoch++
	}
}
