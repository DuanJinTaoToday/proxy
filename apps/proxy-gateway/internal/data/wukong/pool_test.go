package wukong

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	v1 "github.com/DuanJinTaoToday/proxy/api/proxycontrol/v1"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-gateway/internal/biz/authrelay"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-gateway/internal/data/gost"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-gateway/internal/service"
	"github.com/go-gost/gosocks5"
	"golang.org/x/net/proxy"
)

type tenants struct{}

func (tenants) Load(_ context.Context, username, password string) (*v1.AuthSnapshot, error) {
	if password != "pass" || (username != "A" && username != "A2" && username != "B") {
		return nil, authrelay.ErrCredentialDenied
	}
	hash := sha256.Sum256([]byte(password))
	return &v1.AuthSnapshot{Reference: username, ClientId: username, Username: username, TenantId: username[:1],
		Active: true, PasswordHash: hash[:], Quota: &v1.AuthQuota{AvailableBytes: 10}}, nil
}

func (tenants) Refresh(context.Context, []string) ([]*v1.AuthSnapshot, error) {
	return nil, authrelay.ErrGatewayDenied
}

type testEntry struct {
	address string
	down    atomic.Bool
	users   chan string
}

func listenEntry(t *testing.T, probeTarget string) *testEntry {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	entry := &testEntry{address: listener.Addr().String(), users: make(chan string, 16)}
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go entry.handle(conn, probeTarget)
		}
	}()
	return entry
}

func (e *testEntry) handle(conn net.Conn, probeTarget string) {
	defer conn.Close()
	if e.down.Load() {
		return
	}
	_ = conn.SetDeadline(time.Now().Add(10 * time.Second))
	if _, err := gosocks5.ReadMethods(conn); err != nil {
		return
	}
	if _, err := conn.Write([]byte{5, 2}); err != nil {
		return
	}
	auth, err := gosocks5.ReadUserPassRequest(conn)
	if err != nil || auth.Password != "supplier-pass" {
		return
	}
	if _, err := conn.Write([]byte{1, 0}); err != nil {
		return
	}
	request, err := gosocks5.ReadRequest(conn)
	if err != nil || request.Cmd != gosocks5.CmdConnect {
		return
	}
	if request.Addr.String() == probeTarget {
		upstream, err := net.DialTimeout("tcp", probeTarget, time.Second)
		if err != nil {
			return
		}
		defer upstream.Close()
		if gosocks5.NewReply(gosocks5.Succeeded, nil).Write(conn) != nil {
			return
		}
		go func() { _, _ = io.Copy(upstream, conn) }()
		_, _ = io.Copy(conn, upstream)
		return
	}
	e.users <- auth.Username
	if gosocks5.NewReply(gosocks5.Succeeded, nil).Write(conn) == nil {
		_, _ = io.Copy(conn, conn)
	}
}

func TestGOSTWukongPool(t *testing.T) {
	binary := os.Getenv("PROXY_GOST_TEST_BINARY")
	if binary == "" {
		t.Skip("set PROXY_GOST_TEST_BINARY to the GOST v3.3.3 executable")
	}
	target := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer target.Close()
	probeAddress := strings.TrimPrefix(target.URL, "https://")
	first, second := listenEntry(t, probeAddress), listenEntry(t, probeAddress)
	entries := []string{first.address, second.address}
	dir := t.TempDir()
	const token = "local-pool-token"
	pool, release, err := Open(dir, "supplier", "supplier-pass", entries, target.URL+"/primary", target.URL+"/backup", token, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { release() }()
	roots := x509.NewCertPool()
	roots.AddCert(target.Certificate())
	pool.probeTLS = &tls.Config{RootCAs: roots}
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
	defer cancel()
	auth := service.NewAuthRelay(authrelay.New(tenants{}, nil), token)
	auth.OnAuthenticated(pool.RememberClient)
	var routeUnavailable atomic.Bool
	relay := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodConnect:
			pool.Connect(ctx, w, r)
		case r.URL.Path == "/v1/gateway/route":
			if routeUnavailable.Load() {
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}
			pool.Route(w, r)
		default:
			auth.ServeHTTP(w, r)
		}
	}))
	defer relay.Close()

	address, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	socksAddr := address.Addr().String()
	_ = address.Close()
	raw, err := os.ReadFile("../../../gost/wukong-pool.example.yaml")
	if err != nil {
		t.Fatal(err)
	}
	endpoint := relay.URL + "/v1/gateway/auth"
	template := strings.NewReplacer("http://127.0.0.1:8091/v1/gateway/auth", endpoint, "127.0.0.1:1080", socksAddr).Replace(string(raw))
	file := filepath.Join(t.TempDir(), "gost.yaml")
	if err := os.WriteFile(file, []byte(template), 0600); err != nil {
		t.Fatal(err)
	}
	process, err := gost.Start(ctx, binary, file, dir, endpoint, token, gost.WukongConfig{Entries: entries})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { cancel(); <-process.Exited }()
	for {
		conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", socksAddr)
		if err == nil {
			_ = conn.Close()
			break
		}
		select {
		case <-ctx.Done():
			t.Fatal("GOST listener did not start")
		case <-time.After(25 * time.Millisecond):
		}
	}

	dial := func(user string) (net.Conn, error) {
		t.Helper()
		dialer, err := proxy.SOCKS5("tcp", socksAddr, &proxy.Auth{User: user, Password: "pass"}, &net.Dialer{})
		if err != nil {
			t.Fatal(err)
		}
		attempt, stop := context.WithTimeout(ctx, 2*time.Second)
		defer stop()
		return dialer.(proxy.ContextDialer).DialContext(attempt, "tcp", probeAddress)
	}
	reject := func() {
		t.Helper()
		conn, err := dial("A")
		if err == nil {
			_ = conn.Close()
			t.Fatal("unexpected connection to directly reachable target")
		}
	}
	// Business connects use the same host but a separate port alias in the mock.
	// The directly reachable probe address is tested separately for fail-closed routing.
	open := func(user string, entry *testEntry) (net.Conn, string) {
		t.Helper()
		dialer, _ := proxy.SOCKS5("tcp", socksAddr, &proxy.Auth{User: user, Password: "pass"}, &net.Dialer{})
		attempt, stop := context.WithTimeout(ctx, 2*time.Second)
		defer stop()
		conn, err := dialer.(proxy.ContextDialer).DialContext(attempt, "tcp", "example.invalid:443")
		if err != nil {
			t.Fatal(err)
		}
		select {
		case username := <-entry.users:
			return conn, username
		case <-attempt.Done():
			_ = conn.Close()
			t.Fatal("selected wrong entry")
			return nil, ""
		}
	}
	check := func() { pool.check(ctx) }
	reject()
	check()
	reject()
	check()
	routeUnavailable.Store(true)
	reject()
	routeUnavailable.Store(false)
	live, sidA := open("A", first)
	defer live.Close()
	a2, sidA2 := open("A2", first)
	_ = a2.Close()
	b, sidB := open("B", first)
	_ = b.Close()
	if !regexp.MustCompile("^supplier-res-any-sid-[1-9][0-9]{8}-sidtime-3600$").MatchString(sidA) || sidA != sidA2 || sidA == sidB {
		t.Fatal("wrong SID assignment across tenants and credentials")
	}

	first.down.Store(true)
	for range 3 {
		check()
	}
	a, switched := open("A", second)
	_ = a.Close()
	if switched != sidA {
		t.Fatal("SID changed during failover")
	}
	_ = live.SetDeadline(time.Now().Add(time.Second))
	if _, err := live.Write([]byte("alive")); err != nil {
		t.Fatal(err)
	}
	echo := make([]byte, 5)
	if _, err := io.ReadFull(live, echo); err != nil || string(echo) != "alive" {
		t.Fatal("established connection did not survive ejection")
	}
	first.down.Store(false)
	check()
	check()
	a, _ = open("A", second)
	_ = a.Close()

	first.down.Store(true)
	second.down.Store(true)
	for range 3 {
		check()
	}
	reject()
	first.down.Store(false)
	check()
	reject()
	check()
	a, recovered := open("A", first)
	_ = a.Close()
	if recovered != sidA {
		t.Fatal("SID changed after recovery")
	}

	probeSID := pool.probeSID
	release()
	store, unlock, err := openSIDs(dir, "supplier")
	if err != nil {
		t.Fatal(err)
	}
	release = unlock
	for tenant, username := range map[string]string{"tenant:A": sidA, "tenant:B": sidB} {
		sid, err := store.get(tenant)
		if err != nil || sid == probeSID || !strings.Contains(username, "-sid-"+strconv.FormatInt(sid, 10)+"-") {
			t.Fatal("SID was not retained on reopen or collided with probe")
		}
	}
}
