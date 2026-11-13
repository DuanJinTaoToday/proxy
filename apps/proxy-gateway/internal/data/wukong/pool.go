package wukong

import (
	"context"
	"crypto/tls"
	"errors"

	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"sync"

	"time"
)

const (
	probeInterval       = 5 * time.Second
	defaultProbeTimeout = 2 * time.Second
)

type entry struct {
	address             string
	healthy             bool
	successes, failures int
}

type Pool struct {
	probeTLS           *tls.Config
	probeTimeout       time.Duration
	poolUnavailable    bool
	mu                 sync.Mutex
	entries            []entry
	preferred          map[string]int
	clients            map[string]clientTenant
	tickets            map[string]routeTicket
	username, password string
	primary, backup    string
	sids               *sidStore
	probeSID           int64
	token              string
}

type clientTenant struct {
	tenant string
	until  time.Time
}

func Open(dir, username, password string, entries []string, primary, backup, token string, timeout time.Duration) (*Pool, func(), error) {
	if len(entries) == 0 || username == "" || len(username)+36 > 255 || password == "" || len(password) > 255 {
		return nil, nil, errors.New("configure Wukong entries and SOCKS5 credentials")
	}
	for _, raw := range []string{primary, backup} {
		target, err := url.Parse(raw)
		if err != nil || target.Scheme != "https" || target.Hostname() == "" || target.User != nil {
			return nil, nil, errors.New("configure two distinct HTTPS Wukong probe URLs")
		}
	}
	if primary == backup {
		return nil, nil, errors.New("configure distinct Wukong probe URLs")
	}
	if timeout == 0 {
		timeout = defaultProbeTimeout
	}
	if timeout < 0 || timeout > 15*time.Second {
		return nil, nil, errors.New("Wukong probe timeout must be between 1 and 15 seconds")
	}
	pool := &Pool{
		probeTimeout: timeout,
		username:     username, password: password, primary: primary, backup: backup, token: token,
		preferred: make(map[string]int), clients: make(map[string]clientTenant), tickets: make(map[string]routeTicket),
	}
	seen := make(map[string]bool)
	for _, address := range entries {
		host, port, err := net.SplitHostPort(address)
		number, portErr := strconv.ParseUint(port, 10, 16)
		if err != nil || host == "" || portErr != nil || number == 0 || seen[address] {
			return nil, nil, errors.New("configure unique Wukong entry host:port addresses")
		}
		seen[address] = true
		pool.entries = append(pool.entries, entry{address: address})
	}
	store, release, err := openSIDs(dir, username)
	if err != nil {
		return nil, nil, err
	}
	pool.sids = store
	pool.probeSID, err = store.get("probe")
	if err != nil {
		release()
		return nil, nil, err
	}
	return pool, release, nil
}

func (p *Pool) RememberClient(client, tenant string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.clients[client] = clientTenant{tenant: tenant, until: time.Now().Add(time.Minute)}
}

func (p *Pool) Run(ctx context.Context) {
	ticker := time.NewTicker(probeInterval)
	defer ticker.Stop()
	for ctx.Err() == nil {
		p.check(ctx)
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

type probeResult int

const (
	probeUnknown probeResult = iota
	probeSucceeded
	probeFailed
)

func (p *Pool) check(ctx context.Context) {
	var probes sync.WaitGroup
	for i := range p.entries {
		probes.Go(func() {
			address := p.entries[i].address
			result := p.probe(ctx, address)
			if ctx.Err() == nil {
				p.observe(i, result)
			}
		})
	}
	probes.Wait()
	p.mu.Lock()
	defer p.mu.Unlock()
	unavailable := p.selectEntry("") < 0
	if unavailable && !p.poolUnavailable {
		slog.Warn("Wukong pool has no healthy entries")
	}
	p.poolUnavailable = unavailable
	now := time.Now()
	for client, state := range p.clients {
		if !now.Before(state.until) {
			delete(p.clients, client)
		}
	}
	for key, ticket := range p.tickets {
		if !now.Before(ticket.until) {
			delete(p.tickets, key)
		}
	}
}

func (p *Pool) observe(index int, result probeResult) {
	p.mu.Lock()
	defer p.mu.Unlock()
	entry := &p.entries[index]
	wasHealthy := entry.healthy
	switch result {
	case probeSucceeded:
		entry.failures = 0
		entry.successes++
		if entry.successes >= 2 {
			entry.healthy = true
			entry.successes = 0
		}
	case probeFailed:
		entry.successes = 0
		entry.failures++
		if entry.failures >= 3 {
			entry.healthy = false
			entry.failures = 0
		}
	default:
		entry.successes, entry.failures = 0, 0
		slog.Warn("Wukong probe failure attribution unavailable", "entry", entry.address)
	}
	if wasHealthy != entry.healthy {
		slog.Info("Wukong entry health changed", "entry", entry.address, "healthy", entry.healthy)
	}
}

// Caller holds p.mu.
func (p *Pool) selectEntry(tenant string) int {
	if index, ok := p.preferred[tenant]; ok && p.entries[index].healthy {
		return index
	}
	for index := range p.entries {
		if p.entries[index].healthy {
			return index
		}
	}
	return -1
}

func (p *Pool) probe(ctx context.Context, entry string) probeResult {
	stage, ok := p.probeThroughEntry(ctx, entry, p.primary)
	if ok {
		return probeSucceeded
	}
	slog.Debug("Wukong probe failed", "entry", entry, "stage", stage)
	if stage == stageTCP || stage == stageSOCKS {
		return probeFailed
	}

	// A destination or TLS error can be target-specific. Verify the backup
	// through the same entry and directly before attributing it to the entry.
	backupStage, backupOK := p.probeThroughEntry(ctx, entry, p.backup)
	if backupOK {
		return probeUnknown
	}
	if backupStage == stageTCP || backupStage == stageSOCKS {
		return probeFailed
	}
	directCtx, cancel := context.WithTimeout(ctx, p.probeTimeout)
	defer cancel()
	if p.probeHTTPS(directCtx, p.backup, (&net.Dialer{}).DialContext) {
		return probeFailed
	}
	return probeUnknown
}

func (p *Pool) probeThroughEntry(ctx context.Context, entry, target string) (probeStage, bool) {
	ctx, cancel := context.WithTimeout(ctx, p.probeTimeout)
	defer cancel()
	url, _ := url.Parse(target)
	port := url.Port()
	if port == "" {
		port = "443"
	}
	conn, stage, err := p.dial(ctx, entry, p.probeSID, net.JoinHostPort(url.Hostname(), port))
	if err != nil {
		return stage, false
	}
	defer conn.Close()
	return stage, p.probeHTTPS(ctx, target, func(context.Context, string, string) (net.Conn, error) {
		return conn, nil
	})
}

func (p *Pool) probeHTTPS(ctx context.Context, target string, dial func(context.Context, string, string) (net.Conn, error)) bool {
	transport := &http.Transport{DialContext: dial, DisableKeepAlives: true, TLSClientConfig: p.probeTLS}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return false
	}
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	_ = resp.Body.Close()
	return resp.StatusCode >= 200 && resp.StatusCode < 300
}
