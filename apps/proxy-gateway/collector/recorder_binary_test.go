package main

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"io"
	"net"
	"net/http/httptest"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	v1 "github.com/DuanJinTaoToday/proxy/api/proxycontrol/v1"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-gateway/internal/authrelay"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-gateway/internal/recorder"
)

type capturedReport struct{ lines []*v1.RecorderLine }

func (r *capturedReport) Submit(_ context.Context, b *v1.SubmitBatchRequest) error {
	r.lines = append(r.lines, b.GetRecords()...)
	return nil
}

func TestGOSTWritesAndCollectorReportsRealRecorderLines(t *testing.T) {
	binary := os.Getenv("PROXY_GOST_TEST_BINARY")
	if binary == "" {
		t.Skip("set PROXY_GOST_TEST_BINARY to GOST v3.3.3")
	}
	const token = "recorder-smoke"
	relay := httptest.NewServer(authrelay.New(smokeControl{}, token, nil))
	defer relay.Close()
	target := openTestListener(t)
	defer func() { _ = target.Close() }()
	go func() {
		for {
			conn, err := target.Accept()
			if err != nil {
				return
			}
			go func() { defer func() { _ = conn.Close() }(); _, _ = io.Copy(conn, conn) }()
		}
	}()
	unused := openTestListener(t)
	refused := unused.Addr().String()
	if err := unused.Close(); err != nil {
		t.Fatal(err)
	}
	proxy := openTestListener(t)
	proxyAddr := proxy.Addr().String()
	if err := proxy.Close(); err != nil {
		t.Fatal(err)
	}
	stall := openTestListener(t)
	defer func() { _ = stall.Close() }()
	go func() {
		conn, err := stall.Accept()
		if err != nil {
			return
		}
		defer func() { _ = conn.Close() }()
		_, _ = io.Copy(io.Discard, conn)
	}()
	slowProxy := openTestListener(t)
	slowAddr := slowProxy.Addr().String()
	if err := slowProxy.Close(); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile("../gost/auth.example.yaml")
	if err != nil {
		t.Fatal(err)
	}
	endpoint := relay.URL + "/v1/gateway/auth"
	template := strings.NewReplacer("http://127.0.0.1:8091/v1/gateway/auth", endpoint, "127.0.0.1:1080", proxyAddr,
		"auther: proxy-auth-relay", "auther: proxy-auth-relay\n    bypass: deny-target").Replace(string(raw))
	template += "\nbypasses:\n  - name: deny-target\n    matchers: [\"127.0.0.2\"]\n"
	template += "\nchains:\n  - name: timeout-chain\n    hops:\n      - name: one\n        nodes:\n          - name: stall\n            addr: " + stall.Addr().String() + "\n            connector:\n              type: socks5\n              metadata:\n                timeout: 150ms\n            dialer:\n              type: tcp\n"
	template = strings.Replace(template, "\nbypasses:", "\n  - name: slow-socks\n    addr: "+slowAddr+"\n    listener:\n      type: tcp\n    handler:\n      type: socks5\n      auther: proxy-auth-relay\n      chain: timeout-chain\n\nbypasses:", 1)
	dir := t.TempDir()
	config := filepath.Join(dir, "template.yaml")
	if err := os.WriteFile(config, []byte(template), 0o600); err != nil {
		t.Fatal(err)
	}
	spool := t.TempDir()
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	process, err := startGOST(ctx, binary, config, spool, endpoint, token)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cancel(); <-process.exited })
	waitSOCKS(t, ctx, proxyAddr)
	waitSOCKS(t, ctx, slowAddr)
	for _, payload := range []string{"metered", ""} {
		conn, reply := connectSOCKS(t, proxyAddr, target.Addr().String())
		if reply != 0 {
			t.Fatalf("CONNECT failed: %d", reply)
		}
		if payload != "" {
			if _, err := io.WriteString(conn, payload); err != nil {
				t.Fatal(err)
			}
			echo := make([]byte, len(payload))
			if _, err := io.ReadFull(conn, echo); err != nil || string(echo) != payload {
				t.Fatalf("echo %q: %v", echo, err)
			}
		}
		if err := conn.Close(); err != nil {
			t.Fatal(err)
		}
	}
	conn, reply := connectSOCKS(t, proxyAddr, refused)
	_ = conn.Close()
	if reply == 0 {
		t.Fatal("refused upstream allowed")
	}
	conn, reply = connectSOCKS(t, proxyAddr, "127.0.0.2:80")
	_ = conn.Close()
	if reply != 2 {
		t.Fatalf("policy rejection status=%d", reply)
	}

	conn, reply = connectSOCKS(t, slowAddr, target.Addr().String())
	_ = conn.Close()
	if reply == 0 {
		t.Fatal("timed out upstream allowed")
	}

	reporter := &capturedReport{}
	collector := recorder.New(recorder.Config{Spool: spool, GatewayID: "test"}, reporter)
	for len(reporter.lines) < 7 {
		if err := collector.Scan(ctx); err != nil {
			t.Fatal(err)
		}
		select {
		case <-ctx.Done():
			raw, _ := os.ReadFile(filepath.Join(spool, process.boot, recorder.SessionFile))
			t.Fatalf("got %d Recorder lines from %d bytes (%d LF)", len(reporter.lines), len(raw), strings.Count(string(raw), "\n"))
		case <-time.After(20 * time.Millisecond):
		}
	}
	var successful, failed, timedOut int
	for _, line := range reporter.lines {
		var rec struct {
			ClientID, Src, Dst, Err                                    string
			InputBytes, OutputBytes, InputBytesDelta, OutputBytesDelta uint64
		}
		if err := json.Unmarshal([]byte(line.GetLine()), &rec); err != nil {
			t.Fatal(err)
		}
		if rec.ClientID == "" && rec.Src == "" && rec.Dst == "" {
			continue // readiness probe closed before SOCKS5 authentication
		}
		if rec.ClientID != "smoke-client" {
			t.Fatalf("wrong client ID %q", rec.ClientID)
		}
		if rec.Src == "" || rec.Dst == "" || rec.Err != "" {
			failed++
			if strings.Contains(rec.Err, "timeout") {
				timedOut++
			}
			continue
		}
		successful++
		if rec.InputBytes != rec.InputBytesDelta || rec.OutputBytes != rec.OutputBytesDelta {
			t.Fatal("single sample delta differs from cumulative bytes")
		}
	}
	if successful != 2 || failed != 3 || timedOut != 1 {
		t.Fatalf("got %d successful, %d unsuccessful and %d timed out records", successful, failed, timedOut)
	}
}

func openTestListener(t *testing.T) net.Listener {
	t.Helper()
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	return listener
}

func waitSOCKS(t *testing.T, ctx context.Context, address string) {
	t.Helper()
	for {
		conn, err := net.DialTimeout("tcp", address, 100*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			return
		}
		select {
		case <-ctx.Done():
			t.Fatal("GOST did not start")
		case <-time.After(20 * time.Millisecond):
		}
	}
}

func connectSOCKS(t *testing.T, address, target string) (net.Conn, byte) {
	t.Helper()
	conn, err := net.DialTimeout("tcp", address, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	if err := conn.SetDeadline(time.Now().Add(3 * time.Second)); err != nil {
		t.Fatal(err)
	}
	exchange := func(data []byte, length int) []byte {
		t.Helper()
		if _, err := conn.Write(data); err != nil {
			t.Fatal(err)
		}
		reply := make([]byte, length)
		if _, err := io.ReadFull(conn, reply); err != nil {
			t.Fatal(err)
		}
		return reply
	}
	if reply := exchange([]byte{5, 1, 2}, 2); reply[1] != 2 {
		t.Fatal("GOST did not request credentials")
	}
	if reply := exchange([]byte{1, 5, 's', 'm', 'o', 'k', 'e', 4, 'p', 'a', 's', 's'}, 2); reply[1] != 0 {
		t.Fatal("GOST denied test credential")
	}
	dst, err := netip.ParseAddrPort(target)
	if err != nil {
		t.Fatal(err)
	}
	ip := dst.Addr().As4()
	req := binary.BigEndian.AppendUint16(append([]byte{5, 1, 0, 1}, ip[:]...), dst.Port())
	return conn, exchange(req, 10)[1]
}

func TestGatewayReturnsFailureWhenManagedGOSTExits(t *testing.T) {
	binary := os.Getenv("PROXY_GOST_TEST_BINARY")
	if binary == "" {
		t.Skip("set PROXY_GOST_TEST_BINARY to GOST v3.3.3")
	}
	occupied := openTestListener(t)
	defer func() { _ = occupied.Close() }()
	auth := openTestListener(t)
	authAddr := auth.Addr().String()
	if err := auth.Close(); err != nil {
		t.Fatal(err)
	}
	template, err := os.ReadFile("../gost/auth.example.yaml")
	if err != nil {
		t.Fatal(err)
	}
	rendered := strings.NewReplacer("127.0.0.1:8091", authAddr, "127.0.0.1:1080", occupied.Addr().String()).Replace(string(template))
	config := filepath.Join(t.TempDir(), "gost.yaml")
	if err := os.WriteFile(config, []byte(rendered), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PROXY_GATEWAY_LOCAL_AUTH_TOKEN", "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA")
	t.Setenv("PROXY_GATEWAY_AUTH_TOKEN", "BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBA")
	t.Setenv("PROXY_GATEWAY_REPORT_TOKEN", "CCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCA")
	t.Setenv("PROXY_GATEWAY_CONTROL_URL", "https://127.0.0.1:1")
	t.Setenv("PROXY_GATEWAY_CONTROL_CA_FILE", "")
	spool := t.TempDir()
	done := make(chan error, 1)
	go func() {
		done <- run(authAddr, config, binary, recorder.Config{Spool: spool, GatewayID: "test", MaxRecords: 1000, MaxBytes: 1 << 20, Interval: time.Second}, time.Second)
	}()
	select {
	case err := <-done:
		if err == nil || !strings.Contains(err.Error(), "managed GOST process exited") {
			t.Fatalf("gateway did not report child failure: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("gateway stayed running after child exit")
	}
	files, err := filepath.Glob(filepath.Join(spool, "*", recorder.ExitFile))
	if err != nil || len(files) != 1 {
		t.Fatalf("durable exit marker: %v %v", files, err)
	}
}
