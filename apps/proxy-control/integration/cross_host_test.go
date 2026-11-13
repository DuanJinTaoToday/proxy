package integration

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/pem"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	v1 "github.com/DuanJinTaoToday/proxy/api/proxycontrol/v1"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/integration/testfixture"
	"golang.org/x/net/proxy"
	"google.golang.org/protobuf/proto"
	"gopkg.in/yaml.v3"
)

func TestWukongCrossHostDualTenant(t *testing.T) {
	remote := os.Getenv("PROXY_CROSS_HOST_DIR")
	if remote == "" {
		t.Skip("set PROXY_CROSS_HOST_DIR to an isolated devbox directory containing gateway and gost")
	}
	if !regexp.MustCompile("^/tmp/proxy-issue10[.][A-Za-z0-9]+$").MatchString(remote) {
		t.Fatal("use an isolated issue 10 directory")
	}
	if os.Getenv("WUKONG_USERNAME") == "" || os.Getenv("WUKONG_PASSWORD") == "" {
		t.Fatal("set Wukong test credentials")
	}
	f := setupUsage(t)
	execSQL(t, f.db, `INSERT INTO traffic_grants (tenant_id,granted_bytes,starts_at,activated_at,source,created_by)
 VALUES ('alpha',104857600,now()-interval '1 day',now()-interval '1 day','recharge','test'),
 ('beta',104857600,now()-interval '1 day',now()-interval '1 day','recharge','test')`)
	betaAdmin := testfixture.Login(t, f.http, "beta-admin")
	operator := testfixture.Login(t, f.http, "operator")
	issued := make([]v1.CreateCredentialResponse, 2)
	for i, tenant := range []string{"alpha", "beta"} {
		readProto(t, testfixture.Request(f.http, "POST", "/v1/tenants/"+tenant+"/proxy-credentials", "{}", []string{f.alice, betaAdmin}[i]), &issued[i])
	}
	var reportingMode atomic.Int32
	var lostResponses atomic.Int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if reportingMode.Load() == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		if r.URL.Path == "/v1/reporting/batches" && reportingMode.Load() == 2 {
			result := httptest.NewRecorder()
			f.http.ServeHTTP(result, r)
			if result.Code == http.StatusOK {
				lostResponses.Add(1)
				conn, _, err := http.NewResponseController(w).Hijack()
				if err == nil {
					_ = conn.Close()
					return
				}
			}
			for key, values := range result.Header() {
				w.Header()[key] = values
			}
			w.WriteHeader(result.Code)
			_, _ = w.Write(result.Body.Bytes())
			return
		}
		f.http.ServeHTTP(w, r)
	}))
	defer server.Close()
	ctx, cancel := context.WithTimeout(t.Context(), 230*time.Second)
	defer cancel()
	localSOCKS := localTestAddress(t)
	forward := exec.CommandContext(ctx, "ssh", "-o", "BatchMode=yes", "-o", "ExitOnForwardFailure=yes", "-L", localSOCKS+":127.0.0.1:21080", "-R", "127.0.0.1:21443:"+server.Listener.Addr().String(), "devbox", "sleep 230")
	forward.Stderr = os.Stderr
	if err := forward.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = forward.Process.Kill(); _ = forward.Wait() }()
	raw, err := os.ReadFile("../../proxy-gateway/gost/wukong-pool.example.yaml")
	if err != nil {
		t.Fatal(err)
	}
	upload := func(name string, data []byte) {
		t.Helper()
		cmd := exec.CommandContext(ctx, "ssh", "-o", "BatchMode=yes", "devbox", "umask 077; cat > "+remote+"/"+name)
		cmd.Stdin = strings.NewReader(string(data))
		if err := cmd.Run(); err != nil {
			t.Fatal(err)
		}
	}
	upload("template.yaml", []byte(strings.NewReplacer("127.0.0.1:8091", "127.0.0.1:28091", "127.0.0.1:1080", "127.0.0.1:21080").Replace(string(raw))))
	upload("control.pem", pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: server.Certificate().Raw}))
	config, err := yaml.Marshal(map[string]any{
		"server": map[string]any{"http": map[string]string{"addr": "127.0.0.1:28091"}},
		"data": map[string]any{
			"gost": map[string]any{"binary": remote + "/gost", "config": remote + "/template.yaml", "local_auth_token": otherAuthToken,
				"wukong": map[string]any{"entries": []string{"154.81.157.72:12347", "154.81.157.73:12347"}, "username": os.Getenv("WUKONG_USERNAME"), "password": os.Getenv("WUKONG_PASSWORD"),
					"probe_url": "https://www.gstatic.com/generate_204", "backup_probe_url": "https://api.ipify.org", "probe_timeout_seconds": 12}},
			"acl":  map[string]any{"control": map[string]string{"url": "https://127.0.0.1:21443", "ca_file": remote + "/control.pem", "auth_token": authToken, "report_token": reportToken}},
			"repo": map[string]any{"recorder": map[string]string{"spool": remote + "/spool"}},
		},
		"biz": map[string]string{"gateway_id": "gateway-a"},
	})
	if err != nil {
		t.Fatal(err)
	}
	upload("gateway.yaml", config)
	upload("init.yaml", []byte("source:\n  file: "+remote+"/gateway.yaml\n"))
	startGateway := func() *exec.Cmd {
		t.Helper()
		gateway := exec.CommandContext(ctx, "ssh", "-o", "BatchMode=yes", "devbox", "cd "+remote+" && echo $$ >gateway.pid && exec timeout --signal=TERM 225s ./gateway -conf ./init.yaml >>gateway.log 2>&1")
		if err := gateway.Start(); err != nil {
			t.Fatal(err)
		}
		return gateway
	}
	gateway := startGateway()
	defer func() {
		stop := exec.Command("ssh", "-o", "BatchMode=yes", "devbox", "if test -s "+remote+"/gateway.pid; then kill -TERM $(cat "+remote+"/gateway.pid) 2>/dev/null || true; fi; rm -f "+remote+"/gateway.yaml "+remote+"/gateway.pid")
		_ = stop.Run()
		_ = gateway.Wait()
	}()
	dialers := make([]proxy.ContextDialer, 2)
	date := time.Now().In(time.FixedZone("UTC+08", 28800)).Format(time.DateOnly)
	for i, tenant := range []string{"alpha", "beta"} {
		auth := &proxy.Auth{User: issued[i].Username, Password: issued[i].Password}
		socks, err := proxy.SOCKS5("tcp", localSOCKS, auth, &net.Dialer{Timeout: 10 * time.Second})
		if err != nil {
			t.Fatal(err)
		}
		dialers[i] = socks.(proxy.ContextDialer)
		transport := &http.Transport{DialContext: dialers[i].DialContext, DisableKeepAlives: true}
		defer transport.CloseIdleConnections()
		client := &http.Client{Transport: transport, Timeout: 14 * time.Second}
		for {
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.ipify.org", nil)
			if err != nil {
				t.Fatal(err)
			}
			res, err := client.Do(req)
			if err == nil {
				_, _ = io.Copy(io.Discard, io.LimitReader(res.Body, 64))
				_ = res.Body.Close()
				if res.StatusCode == 200 {
					break
				}
			}
			select {
			case <-ctx.Done():
				t.Fatalf("%s failed through real Wukong: %v", tenant, err)
			case <-time.After(time.Second):
			}
		}
		token := []string{f.alice, f.bob}[i]
		start, _ := time.Parse(time.RFC3339, date+"T00:00:00+08:00")
		query := "/v1/operations/usage?" + url.Values{"tenant_id": {tenant}, "gateway_id": {"gateway-a"}, "granularity": {"USAGE_GRANULARITY_DAY"}, "start_time": {start.Format(time.RFC3339)}, "end_time": {start.Add(24 * time.Hour).Format(time.RFC3339)}}.Encode()
		for {
			daily := usageOn(t, f, tenant, token, date)
			var operations v1.QueryUsageResponse
			readProto(t, testfixture.Request(f.http, "GET", query, "", operator), &operations)
			if len(daily.Days) == 1 && len(operations.Buckets) == 1 && proto.Equal(daily.Days[0].Amount, operations.Buckets[0].Amount) && daily.Days[0].Amount.TotalBytes > 0 {
				t.Logf("%s real Wukong and tenant/operations reconciliation: %d bytes", tenant, daily.Days[0].Amount.TotalBytes)
				break
			}
			select {
			case <-ctx.Done():
				t.Fatal("usage did not reconcile")
			case <-time.After(200 * time.Millisecond):
			}
		}
		for _, deny := range []struct{ path, token string }{{query, token}, {"/v1/tenants/" + tenant + "/usage/daily?start_date=" + date + "&end_date=" + date, []string{f.bob, f.alice}[i]}} {
			if result := testfixture.Request(f.http, "GET", deny.path, "", deny.token); result.Code != http.StatusForbidden {
				t.Fatalf("expected 403, got %d", result.Code)
			}
		}
	}
	wait := func(label string, ready func() bool) {
		t.Helper()
		for !ready() {
			select {
			case <-ctx.Done():
				t.Fatalf("timed out waiting for %s", label)
			case <-time.After(250 * time.Millisecond):
			}
		}
	}
	amount := func() uint64 {
		daily := usageOn(t, f, "alpha", f.alice, date)
		if len(daily.Days) != 1 {
			t.Fatal("missing alpha usage")
		}
		return daily.Days[0].Amount.TotalBytes
	}
	offset := func() int64 {
		var value int64
		if err := f.db.Raw("SELECT coalesce(max(confirmed_offset),0) FROM ingest_cursors WHERE gateway_id='gateway-a'").Scan(&value).Error; err != nil {
			t.Fatal(err)
		}
		return value
	}
	remoteOffset := func() int64 {
		raw, err := exec.CommandContext(ctx, "ssh", "-o", "BatchMode=yes", "devbox", "cat "+remote+"/spool/*/confirmed.offset 2>/dev/null").Output()
		if err != nil {
			return 0
		}
		var highest int64
		for _, line := range strings.Fields(string(raw)) {
			value, err := strconv.ParseInt(line, 10, 64)
			if err != nil {
				t.Fatal(err)
			}
			highest = max(highest, value)
		}
		return highest
	}
	// Keep a real SOCKS tunnel open before sending TLS data, so the recorder
	// can report interim samples without the public target closing the tunnel.
	var live net.Conn
	wait("alpha live SOCKS tunnel", func() bool {
		attempt, stop := context.WithTimeout(ctx, 10*time.Second)
		defer stop()
		var err error
		live, err = dialers[0].DialContext(attempt, "tcp", "api.ipify.org:443")
		return err == nil
	})
	defer live.Close()
	baseline := amount()
	conn := tls.Client(live, &tls.Config{ServerName: "api.ipify.org"})
	_ = conn.SetDeadline(time.Now().Add(15 * time.Second))
	if err := conn.HandshakeContext(ctx); err != nil {
		t.Fatal(err)
	}
	wait("live interim usage", func() bool { return amount() > baseline })
	t.Logf("alpha interim before close: %d bytes", amount()-baseline)

	// Inject a complete control HTTP outage and then lose responses after
	// PostgreSQL commits. The existing tunnel continues to carry data.
	wait("settled cursor", func() bool { return remoteOffset() == offset() })
	billing := testfixture.Login(t, f.http, "billing")
	balance := func() v1.TenantBalance {
		t.Helper()
		var reply v1.TenantBalance
		readProto(t, testfixture.Request(f.http, "GET", "/v1/tenants/alpha/balance", "", f.alice), &reply)
		return reply
	}
	remaining := balance()
	var adjustment v1.BalanceLedgerEntry
	readProto(t, testfixture.Request(f.http, "POST", "/v1/operations/tenants/alpha/adjustments",
		fmt.Sprintf(`{"external_ref":"cross-host-exhaust","mode":"SUBTRACT","bytes":%q,"grant_id":%q}`,
			strconv.FormatInt(remaining.AvailableBytes, 10), remaining.Grants[0].GrantId), billing), &adjustment)
	if adjustment.AvailableAfter != 0 {
		t.Fatal("expected exhausted allowance")
	}
	wait("alpha new authentication denied", func() bool {
		attempt, stop := context.WithTimeout(ctx, 3*time.Second)
		defer stop()
		connection, err := dialers[0].DialContext(attempt, "tcp", "api.ipify.org:443")
		if connection != nil {
			_ = connection.Close()
		}
		return err != nil && strings.Contains(err.Error(), "username/password authentication failed")
	})
	_ = conn.SetDeadline(time.Now().Add(15 * time.Second))
	baseline, previous := amount(), offset()
	reportingMode.Store(1)
	if _, err := io.WriteString(conn, "GET / HTTP/1.1\r\nHost: api.ipify.org\r\nConnection: close\r\n\r\n"); err != nil {
		t.Fatal(err)
	}
	resp, err := http.ReadResponse(bufio.NewReader(conn), nil)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 64))
	_ = resp.Body.Close()
	_ = live.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("live target status %d", resp.StatusCode)
	}
	select {
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	case <-time.After(6 * time.Second):
	}
	if amount() != baseline || offset() != previous {
		t.Fatal("control accepted reports during outage")
	}
	reportingMode.Store(2)
	wait("commit with lost HTTP response", func() bool { return lostResponses.Load() > 0 && offset() > previous })
	committedAmount, committedOffset := amount(), offset()
	if committedAmount <= baseline || remoteOffset() != previous {
		t.Fatal("lost response did not leave the gateway behind")
	}
	reportingMode.Store(0)
	wait("gateway cursor catch-up", func() bool { return remoteOffset() >= committedOffset })
	if amount() != committedAmount {
		t.Fatal("retransmission changed billed amount")
	}
	debt := balance()
	if debt.OverdraftBytes == 0 || debt.AvailableBytes >= 0 {
		t.Fatal("existing connection did not incur overdraft")
	}
	var credit v1.BalanceLedgerEntry
	readProto(t, testfixture.Request(f.http, "POST", "/v1/operations/tenants/alpha/recharges",
		`{"external_ref":"cross-host-recharge","granted_bytes":"20971520"}`, billing), &credit)
	if credit.AvailableAfter != 20971520-int64(debt.OverdraftBytes) || balance().OverdraftBytes != 0 {
		t.Fatal("recharge failed to offset debt")
	}
	wait("alpha readmitted after recharge", func() bool {
		attempt, stop := context.WithTimeout(ctx, 13*time.Second)
		defer stop()
		connection, err := dialers[0].DialContext(attempt, "tcp", "api.ipify.org:443")
		if connection != nil {
			_ = connection.Close()
		}
		return err == nil
	})
	t.Logf("alpha overdraft %d bytes offset by recharge and new connection admitted", debt.OverdraftBytes)
	t.Logf("lost response: cursor %d -> %d, usage %d bytes exactly once", previous, committedOffset, committedAmount-baseline)
	// Submit two samples on either side of Shanghai midnight using the
	// reporting API. A dedicated boot keeps these offsets independent.
	crossFirst := recorder(0, issued[1].GetClientId(), beforeMidnight, 7, 11, 7, 11)
	crossSecond := recorder(crossFirst.GetEndOffset(), issued[1].GetClientId(), afterMidnight, 12, 24, 5, 13)
	submit(t, f, "gateway-a", reportToken, crossFirst, crossSecond)
	assertDays(t, daily(t, f, "beta", f.bob), usageDay("2020-11-10", 7, 11, 18), usageDay("2020-11-11", 5, 13, 18))
	t.Log("Shanghai midnight reporting samples reconciled to separate tenant days")

	// A separate live session must have an interim before killing only the
	// GOST child. On restart, the same spool feeds reconciliation.
	var unfinished net.Conn
	wait("live session for GOST exit", func() bool {
		attempt, stop := context.WithTimeout(ctx, 13*time.Second)
		defer stop()
		var err error
		unfinished, err = dialers[0].DialContext(attempt, "tcp", "api.ipify.org:443")
		return err == nil
	})
	defer unfinished.Close()
	beforeExit := amount()
	tlsConn := tls.Client(unfinished, &tls.Config{ServerName: "api.ipify.org"})
	_ = tlsConn.SetDeadline(time.Now().Add(15 * time.Second))
	if err := tlsConn.HandshakeContext(ctx); err != nil {
		t.Fatal(err)
	}
	wait("unfinished session interim", func() bool { return amount() > beforeExit })
	kill := exec.CommandContext(ctx, "ssh", "-o", "BatchMode=yes", "devbox", "cd "+remote+" && supervisor=$(cat gateway.pid) && child=$(pgrep -P \"$supervisor\" -x gateway) && gost=$(pgrep -P \"$child\" -x gost) && kill -KILL \"$gost\"")
	if err := kill.Run(); err != nil {
		t.Fatal(err)
	}
	if err := gateway.Wait(); err == nil {
		t.Fatal("expected gateway exit after GOST was killed")
	}
	gateway = startGateway()
	var gaps v1.ListUsageGapsResponse
	wait("possible gap after gateway restart", func() bool {
		readProto(t, testfixture.Request(f.http, "GET", gapsPath+"?gateway_id=gateway-a&tenant_id=alpha&kind=session&status=possible_gap", "", operator), &gaps)
		return len(gaps.Gaps) > 0
	})
	if len(gaps.Gaps) != 1 || gaps.Gaps[0].GetSessionId() == "" || gaps.Gaps[0].GetBootId() == "" {
		t.Fatal("unexpected gap attribution")
	}
	t.Logf("GOST abnormal exit reconciled with one possible session gap, boot %s", gaps.Gaps[0].GetBootId())
}
