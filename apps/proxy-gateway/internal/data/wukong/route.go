package wukong

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/DuanJinTaoToday/proxy/apps/proxy-gateway/internal/biz/stickysession"
)

type routeTicket struct {
	identity                   stickysession.Identity
	reference, target, sidtime string
	until                      time.Time
}

func (p *Pool) Route(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost || subtle.ConstantTimeCompare([]byte(req.Header.Get("Authorization")), []byte("Bearer "+p.token)) != 1 {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	var input struct{ Client, Network, Addr string }
	if json.NewDecoder(http.MaxBytesReader(w, req.Body, 16<<10)).Decode(&input) != nil || input.Network != "tcp" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	if host, port, err := net.SplitHostPort(input.Addr); err != nil || host == "" || port == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	p.mu.Lock()
	now := time.Now()
	client := p.clients[input.Client]
	delete(p.clients, input.Client)
	if client.identity.Client == "" || !now.Before(client.until) || p.selectEntry(-1) < 0 {
		p.mu.Unlock()
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}
	key := strings.ToLower(rand.Text()) + ".route.invalid:1"
	p.tickets[key] = routeTicket{identity: client.identity, reference: client.reference, target: input.Addr, sidtime: client.sidtime, until: now.Add(5 * time.Second)}
	p.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"name": "wukong", "addr": key,
		"connector": map[string]string{"type": "forward"},
		"dialer":    map[string]string{"type": "tcp"},
	})
}

func (p *Pool) Connect(ctx context.Context, w http.ResponseWriter, req *http.Request) {
	auth := http.Request{Header: http.Header{"Authorization": {req.Header.Get("Proxy-Authorization")}}}
	user, password, ok := auth.BasicAuth()
	if !ok || user != "gateway" || subtle.ConstantTimeCompare([]byte(password), []byte(p.token)) != 1 {
		w.WriteHeader(http.StatusProxyAuthRequired)
		return
	}
	if req.Header.Get("X-Gost-Protocol") != "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	p.mu.Lock()
	ticket := p.tickets[req.Host]
	delete(p.tickets, req.Host)
	available := p.selectEntry(-1) >= 0
	p.mu.Unlock()
	if ticket.identity.Client == "" || !time.Now().Before(ticket.until) || !available {
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}
	attempt, err := p.sessions.Resolve(req.Context(), ticket.identity, ticket.reference)
	if err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}
	defer p.sessions.Release(req.Context(), &attempt)

	p.mu.Lock()
	var candidates []int
	if preferred := p.selectEntry(attempt.Preferred); preferred >= 0 {
		candidates = append(candidates, preferred)
		for i := range p.entries {
			if i != preferred && p.entries[i].healthy {
				candidates = append(candidates, i)
			}
		}
	}
	p.mu.Unlock()
	_ = http.NewResponseController(w).SetWriteDeadline(time.Now().Add(14 * time.Second))
	dialCtx, cancel := context.WithTimeout(req.Context(), 12*time.Second)
	stopDial := context.AfterFunc(ctx, cancel)
	var upstream *net.TCPConn
	var index int
	for _, candidate := range candidates {
		dialAttempt, stop := context.WithTimeout(dialCtx, 6*time.Second)
		upstream, _, err = p.dial(dialAttempt, p.entries[candidate].address, attempt.SID, ticket.sidtime, ticket.target)
		stop()
		if err == nil {
			index = candidate
			break
		}
		if dialCtx.Err() != nil {
			break
		}
	}
	stopDial()
	cancel()
	if upstream == nil {
		w.WriteHeader(http.StatusBadGateway)
		return
	}
	defer func() { _ = upstream.Close() }()
	if err := p.sessions.Confirm(req.Context(), &attempt, index); err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}

	client, rw, err := http.NewResponseController(w).Hijack()
	if err != nil {
		return
	}
	defer func() { _ = client.Close() }()
	_ = client.SetDeadline(time.Time{})
	if _, err := rw.WriteString("HTTP/1.1 200 Connection Established\r\n\r\n"); err != nil || rw.Flush() != nil {
		return
	}

	stop := context.AfterFunc(ctx, func() {
		_ = client.Close()
		_ = upstream.Close()
	})
	defer stop()
	done := make(chan struct{})
	go func() {
		defer close(done)
		if _, err := io.Copy(upstream, rw); err != nil {
			_ = upstream.Close()
			return
		}
		_ = upstream.CloseWrite()
	}()
	if _, err := io.Copy(client, upstream); err != nil {
		_ = client.Close()
	} else if tcp, ok := client.(*net.TCPConn); ok {
		_ = tcp.CloseWrite()
	} else {
		_ = client.Close()
	}
	<-done
}
