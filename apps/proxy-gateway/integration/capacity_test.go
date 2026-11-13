package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/DuanJinTaoToday/proxy/apps/proxy-gateway/internal/biz/authrelay"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-gateway/internal/data/gost"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-gateway/internal/data/repo"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-gateway/internal/service"
	"golang.org/x/net/proxy"
)

const (
	probeInterval      = 5 * time.Millisecond
	probeWindow        = 3 * time.Second
	probeTimeout       = 2 * time.Second
	recorderLimit      = 5 * time.Second
	maxProbeStartDelay = 50 * time.Millisecond
)

func TestGOSTConcentratedCloseCapacity(t *testing.T) {
	if os.Getenv("PROXY_RUN_CAPACITY") != "1" {
		t.Skip("set PROXY_RUN_CAPACITY=1 in an isolated Linux test environment")
	}
	binary := os.Getenv("PROXY_GOST_TEST_BINARY")
	version, err := exec.Command(binary, "-V").CombinedOutput()
	if err != nil || !strings.Contains(string(version), "gost 3.3.3 ") {
		t.Fatalf("need GOST v3.3.3: %v, %s", err, version)
	}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	go monitorCapacityMemory(ctx, cancel)

	target := openTestListener(t)
	defer func() { _ = target.Close() }()
	go func() {
		for {
			conn, err := target.Accept()
			if err != nil {
				return
			}
			go func() {
				defer func() { _ = conn.Close() }()
				_, _ = io.Copy(conn, conn)
			}()
		}
	}()

	upstreamAddr := capacityAddress(t)
	upstreamCtx, stopUpstream := context.WithCancel(ctx)
	upstream := exec.CommandContext(upstreamCtx, binary, "-L", "socks5://"+upstreamAddr)
	if err := upstream.Start(); err != nil {
		stopUpstream()
		t.Fatal(err)
	}
	defer func() { stopUpstream(); _ = upstream.Wait() }()
	ready, stopReady := context.WithTimeout(ctx, 3*time.Second)
	waitSOCKS(t, ready, upstreamAddr)
	stopReady()

	for _, count := range []int{100, 250, 500, 1000} {
		for round := 1; round <= 3; round++ {
			if !t.Run(fmt.Sprintf("%d/round-%d", count, round), func(t *testing.T) {
				runCapacityRound(t, ctx, binary, upstreamAddr, target.Addr().String(), count)
			}) {
				return
			}
		}
	}
}

func capacityAddress(t *testing.T) string {
	t.Helper()
	listener := openTestListener(t)
	address := listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	return address
}

func monitorCapacityMemory(ctx context.Context, stop context.CancelFunc) {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		data, err := os.ReadFile("/proc/meminfo")
		_, available, found := strings.Cut(string(data), "MemAvailable:")
		fields := strings.Fields(available)
		if err != nil || !found || len(fields) == 0 {
			stop()
			return
		}
		kib, err := strconv.Atoi(fields[0])
		if err != nil || kib < 256*1024 {
			stop()
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func runCapacityRound(t *testing.T, ctx context.Context, binary, upstream, target string, count int) {
	t.Helper()
	ctx, stop := context.WithTimeout(ctx, 45*time.Second)
	defer stop()
	const token = "capacity-test-local-token"
	relay := httptest.NewServer(service.NewAuthRelay(authrelay.New(smokeControl{}, nil), token))
	defer relay.Close()
	endpoint := relay.URL + "/v1/gateway/auth"
	address := capacityAddress(t)
	template, err := os.ReadFile("../gost/auth.example.yaml")
	if err != nil {
		t.Fatal(err)
	}
	config := strings.NewReplacer("http://127.0.0.1:8091/v1/gateway/auth", endpoint,
		"127.0.0.1:1080", address, "auther: proxy-auth-relay", "auther: proxy-auth-relay\n      chain: capacity-upstream").Replace(string(template))
	config += fmt.Sprintf("\nchains:\n  - name: capacity-upstream\n    hops:\n      - name: one\n        nodes:\n          - name: upstream\n            addr: %q\n            connector:\n              type: socks5\n            dialer:\n              type: tcp\n", upstream)
	spool := t.TempDir()
	path := filepath.Join(spool, "template.yaml")
	if err := os.WriteFile(path, []byte(config), 0o600); err != nil {
		t.Fatal(err)
	}
	process, err := gost.Start(ctx, binary, path, spool, endpoint, token, gost.WukongConfig{})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { stop(); <-process.Exited }()
	ready, stopReady := context.WithTimeout(ctx, 3*time.Second)
	waitSOCKS(t, ready, address)
	stopReady()

	newConnect := func(source net.IP) func() (net.Conn, error) {
		raw, err := proxy.SOCKS5("tcp4", address, &proxy.Auth{User: "smoke", Password: "pass"},
			&net.Dialer{LocalAddr: &net.TCPAddr{IP: source}})
		if err != nil {
			t.Fatal(err)
		}
		dialer := raw.(proxy.ContextDialer)
		return func() (net.Conn, error) {
			dialCtx, cancel := context.WithTimeout(ctx, probeTimeout)
			defer cancel()
			conn, err := dialer.DialContext(dialCtx, "tcp", target)
			if err != nil && errors.Is(dialCtx.Err(), context.DeadlineExceeded) {
				return nil, context.DeadlineExceeded
			}
			return conn, err
		}
	}
	// Prevent probes from reusing a candidate's source address during the round.
	connectCandidate := newConnect(net.IPv4(127, 0, 0, 1))
	connectProbe := newConnect(net.IPv4(127, 0, 0, 2))
	expected := make(map[string]struct{}, count)
	held := make([]net.Conn, 0, count)
	defer func() {
		for _, conn := range held {
			_ = conn.Close()
		}
	}()
	for range count {
		conn, err := connectCandidate()
		if err != nil {
			t.Fatalf("populate %d/%d: %v", len(held), count, err)
		}
		held = append(held, conn)
		expected[conn.LocalAddr().String()] = struct{}{}
	}
	if len(expected) != count {
		t.Fatal("candidate source addresses are not unique")
	}
	select {
	case <-time.After(1100 * time.Millisecond):
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}

	start := time.Now()
	go func() {
		for _, conn := range held {
			_ = conn.Close()
		}
	}()
	readout := make(chan capacityRecorderResult, 1)
	go func() {
		readout <- readCapacityRecorder(ctx, filepath.Join(spool, process.Boot, repo.SessionFile), start, expected)
	}()
	probes := runCapacityProbes(ctx, start, connectProbe)
	recorder := <-readout
	p99 := probes.p99()
	t.Logf("scheduled=%d started=%d success=%d failure=%d timeout=%d P99=%s max_start_delay=%s final=%d/%d duplicate=%d bad=%d read=%s",
		int(probeWindow/probeInterval), probes.started, len(probes.success), probes.failed, probes.timeout, p99,
		probes.maxDelay, recorder.finals, count, recorder.duplicates, recorder.bad, recorder.elapsed)
	if ctx.Err() != nil {
		t.Fatalf("round aborted: %v", ctx.Err())
	}
	if probes.started != int(probeWindow/probeInterval) || probes.maxDelay > maxProbeStartDelay {
		t.Fatal("invalid round: generator did not sustain the scheduled rate")
	}
	if len(probes.success)*1000 < probes.started*995 || p99 > 250*time.Millisecond {
		t.Error("new connection success rate or latency missed its threshold")
	}
	if recorder.err != nil || recorder.finals != count || recorder.duplicates != 0 || recorder.bad != 0 || recorder.elapsed > recorderLimit {
		t.Errorf("Recorder missed its threshold: %+v", recorder)
	}
}

type capacityProbes struct {
	started, failed, timeout int
	success                  []time.Duration
	maxDelay                 time.Duration
}

func (p capacityProbes) p99() time.Duration {
	if len(p.success) == 0 {
		return 0
	}
	slices.Sort(p.success)
	return p.success[(99*len(p.success)+99)/100-1]
}

func runCapacityProbes(ctx context.Context, start time.Time, connect func() (net.Conn, error)) capacityProbes {
	type outcome struct {
		delay, elapsed time.Duration
		err            error
	}
	count := int(probeWindow / probeInterval)
	results := make(chan outcome, count)
	for i := range count {
		due := start.Add(time.Duration(i) * probeInterval)
		time.Sleep(time.Until(due))
		go func() {
			began := time.Now()
			if !began.Before(start.Add(probeWindow)) || ctx.Err() != nil {
				results <- outcome{delay: began.Sub(due), err: context.Canceled}
				return
			}
			conn, err := connect()
			elapsed := time.Since(began)
			if err == nil {
				_ = conn.Close()
			}
			results <- outcome{began.Sub(due), elapsed, err}
		}()
	}
	var probes capacityProbes
	for range count {
		result := <-results
		probes.maxDelay = max(probes.maxDelay, result.delay)
		switch {
		case errors.Is(result.err, context.Canceled):
			continue
		case errors.Is(result.err, context.DeadlineExceeded):
			probes.timeout++
		case result.err != nil:
			probes.failed++
		default:
			probes.success = append(probes.success, result.elapsed)
		}
		probes.started++
	}
	return probes
}

type capacityRecorderResult struct {
	finals, duplicates, bad int
	elapsed                 time.Duration
	err                     error
}

func readCapacityRecorder(ctx context.Context, path string, start time.Time, expected map[string]struct{}) capacityRecorderResult {
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	var readAt time.Duration
	for {
		raw, err := os.ReadFile(path)
		if err != nil {
			return capacityRecorderResult{err: err}
		}
		result := inspectCapacityRecords(raw, expected, start)
		elapsed := time.Since(start)
		if result.finals == len(expected) && readAt == 0 {
			readAt = elapsed
		}
		if elapsed >= recorderLimit || ctx.Err() != nil {
			if readAt == 0 {
				readAt = elapsed
			}
			if len(raw) > 0 && raw[len(raw)-1] != '\n' {
				result.bad++
			}
			result.elapsed = readAt
			result.err = ctx.Err()
			return result
		}
		<-ticker.C
	}
}

func inspectCapacityRecords(raw []byte, expected map[string]struct{}, start time.Time) capacityRecorderResult {
	var result capacityRecorderResult
	ids := make(map[string]struct{}, len(expected))
	sessions := make(map[string]struct{}, len(expected))
	for line := range bytes.SplitSeq(raw[:bytes.LastIndexByte(raw, '\n')+1], []byte{'\n'}) {
		if len(line) == 0 {
			continue
		}
		var record struct {
			Phase, Remote, SessionID, Network, ClientID, Src, Dst, Err string
			Time                                                       time.Time
		}
		if err := json.Unmarshal(line, &record); err != nil || record.SessionID == "" {
			result.bad++
			continue
		}
		if _, wanted := expected[record.Remote]; !wanted || record.Phase != "final" {
			continue
		}
		if record.Time.Before(start) || record.Network != "tcp" || record.ClientID != "smoke-client" || record.Src == "" || record.Dst == "" || record.Err != "" {
			result.bad++
			continue
		}
		_, addressSeen := ids[record.Remote]
		_, sessionSeen := sessions[record.SessionID]
		if addressSeen || sessionSeen {
			result.duplicates++
			continue
		}
		ids[record.Remote] = struct{}{}
		sessions[record.SessionID] = struct{}{}
	}
	result.finals = len(ids)
	return result
}
