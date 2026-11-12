package server_test

import (
	"context"
	"encoding/json"
	"encoding/pem"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	v1 "github.com/DuanJinTaoToday/proxy/api/proxycontrol/v1"
	"golang.org/x/net/proxy"
)

func TestWukongGatewayReportsRealRecorderUsage(t *testing.T) {
	gateway, gost := os.Getenv("PROXY_GATEWAY_TEST_BINARY"), os.Getenv("PROXY_GOST_TEST_BINARY")
	if gateway == "" || gost == "" || os.Getenv("WUKONG_ADDRESS") == "" || os.Getenv("WUKONG_USERNAME") == "" || os.Getenv("WUKONG_PASSWORD") == "" {
		t.Skip("set gateway/GOST binaries and Wukong test environment variables")
	}
	f := setupUsage(t)
	execSQL(t, f.db, `INSERT INTO traffic_grants (tenant_id, granted_bytes, starts_at, activated_at, source, created_by)
		VALUES ('alpha', 104857600, now()-interval '1 day', now()-interval '1 day', 'recharge', 'test')`)
	issued := &v1.CreateCredentialResponse{}
	readProto(t, request(f.http, "POST", "/v1/tenants/alpha/proxy-credentials", "{}", f.alice), issued)
	control := httptest.NewTLSServer(f.http)
	defer control.Close()
	root := t.TempDir()
	ca := filepath.Join(root, "control.pem")
	if err := os.WriteFile(ca, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: control.Certificate().Raw}), 0o600); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile("../../../proxy-gateway/gost/wukong.example.yaml")
	if err != nil {
		t.Fatal(err)
	}
	authAddr, socksAddr := localTestAddress(t), localTestAddress(t)
	template := strings.NewReplacer("127.0.0.1:8091", authAddr, "127.0.0.1:1080", socksAddr).Replace(string(raw))
	config := filepath.Join(root, "template.yaml")
	if err := os.WriteFile(config, []byte(template), 0o600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 45*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, gateway, "-gost-binary", gost, "-gost-config", config,
		"-auth-listen", authAddr, "-spool", filepath.Join(root, "spool"))
	cmd.Env = append(os.Environ(),
		"PROXY_GATEWAY_CONTROL_URL="+control.URL, "PROXY_GATEWAY_CONTROL_CA_FILE="+ca,
		"TMP="+root, "TEMP="+root,
		"PROXY_GATEWAY_ID=gateway-a", "PROXY_GATEWAY_LOCAL_AUTH_TOKEN="+otherAuthToken,
		"PROXY_GATEWAY_AUTH_TOKEN="+authToken, "PROXY_GATEWAY_REPORT_TOKEN="+reportToken)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if runtime.GOOS == "windows" {
			_ = exec.Command("taskkill", "/PID", strconv.Itoa(cmd.Process.Pid), "/T", "/F").Run()
		} else {
			_ = cmd.Process.Signal(os.Interrupt)
		}
		_ = cmd.Wait()
	}()
	for {
		probe, err := net.DialTimeout("tcp", socksAddr, 100*time.Millisecond)
		if err == nil {
			_ = probe.Close()
			break
		}
		select {
		case <-ctx.Done():
			t.Fatal("GOST listener did not start")
		case <-time.After(25 * time.Millisecond):
		}
	}
	dialer, err := proxy.SOCKS5("tcp", socksAddr, &proxy.Auth{User: issued.Username, Password: issued.Password}, &net.Dialer{Timeout: 10 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Timeout: 15 * time.Second, Transport: &http.Transport{DialContext: dialer.(proxy.ContextDialer).DialContext, DisableKeepAlives: true}}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.ipify.org", nil)
	if err != nil {
		t.Fatal(err)
	}
	res, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	body, readErr := io.ReadAll(io.LimitReader(res.Body, 64))
	_ = res.Body.Close()
	ip, parseErr := netip.ParseAddr(strings.TrimSpace(string(body)))
	if readErr != nil || parseErr != nil || res.StatusCode != 200 {
		t.Fatalf("Wukong egress HTTP %d, read=%v, valid IP=%t", res.StatusCode, readErr, parseErr == nil)
	}
	t.Logf("Wukong egress %s via GOST", ip)
	date := time.Now().In(time.FixedZone("UTC+08", 8*60*60)).Format(time.DateOnly)
	for {
		alpha := usageOn(t, f, "alpha", f.alice, date)
		recorded, complete := finalRecorderAmount(t, filepath.Join(root, "spool"), issued.GetClientId())
		if complete && len(alpha.Days) > 0 {
			amount := alpha.Days[0].GetAmount()
			if amount.GetInputBytes() == 0 || amount.GetOutputBytes() == 0 || amount.GetBillableBytes() != amount.GetTotalBytes() {
				t.Fatalf("unexpected metering: %v", amount)
			}
			if amount.GetInputBytes() == recorded.GetInputBytes() && amount.GetOutputBytes() == recorded.GetOutputBytes() {
				if beta := usageOn(t, f, "beta", f.bob, date); len(beta.Days) != 0 {
					t.Fatal("usage leaked to other tenant")
				}
				t.Logf("tenant alpha received %d input / %d output / %d billed bytes",
					amount.GetInputBytes(), amount.GetOutputBytes(), amount.GetBillableBytes())
				return
			}
		}
		select {
		case <-ctx.Done():
			t.Fatal("real Recorder usage did not reach the control plane")
		case <-time.After(100 * time.Millisecond):
		}
	}
}

func finalRecorderAmount(t *testing.T, spool, clientID string) (*v1.UsageAmount, bool) {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(spool, "*", "sessions.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		raw, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		for _, line := range strings.Split(string(raw), "\n") {
			var record struct {
				ClientID, Phase, Src, Dst, Err string
				InputBytes, OutputBytes        uint64
			}
			if json.Unmarshal([]byte(line), &record) == nil && record.ClientID == clientID &&
				record.Phase == "final" && record.Src != "" && record.Dst != "" && record.Err == "" {
				return &v1.UsageAmount{InputBytes: record.InputBytes, OutputBytes: record.OutputBytes}, true
			}
		}
	}
	return nil, false
}

func localTestAddress(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	return addr
}
