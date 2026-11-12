package integration

import (
	"context"
	"crypto/sha256"
	"io"
	"net"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	v1 "github.com/DuanJinTaoToday/proxy/api/proxycontrol/v1"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-gateway/internal/biz/authrelay"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-gateway/internal/data/gost"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-gateway/internal/service"
)

type smokeControl struct{}

func (smokeControl) Load(_ context.Context, username, password string) (*v1.AuthSnapshot, error) {
	if username != "smoke" || password != "pass" {
		return nil, authrelay.ErrCredentialDenied
	}
	hash := sha256.Sum256([]byte(password))
	return &v1.AuthSnapshot{Reference: "test", Username: username, ClientId: "smoke-client", TenantId: "smoke-tenant",
		Active: true, PasswordHash: hash[:], Quota: &v1.AuthQuota{AvailableBytes: 100, AdmissionThresholdBytes: 10}}, nil
}

func (smokeControl) Refresh(context.Context, []string) ([]*v1.AuthSnapshot, error) {
	return nil, authrelay.ErrGatewayDenied
}

func TestGOSTBinaryUsesRelayForSOCKS5Authentication(t *testing.T) {
	binary := os.Getenv("PROXY_GOST_TEST_BINARY")
	if binary == "" {
		t.Skip("set PROXY_GOST_TEST_BINARY to the GOST v3.3.3 executable")
	}
	const token = "gost-smoke-test-local-token"
	relay := httptest.NewServer(service.NewAuthRelay(authrelay.New(smokeControl{}, nil), token))
	defer relay.Close()
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	socksAddr := listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	template, err := os.ReadFile("../gost/auth.example.yaml")
	if err != nil {
		t.Fatal(err)
	}
	endpoint := relay.URL + "/v1/gateway/auth"
	rendered := strings.NewReplacer("${LOCAL_AUTH_TOKEN}", token,
		"http://127.0.0.1:8091/v1/gateway/auth", endpoint, "127.0.0.1:1080", socksAddr).Replace(string(template))
	file := filepath.Join(t.TempDir(), "gost.yaml")
	if err := os.WriteFile(file, []byte(rendered), 0o600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	process, err := gost.Start(ctx, binary, file, t.TempDir(), endpoint, token, gost.WukongConfig{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cancel()
		<-process.Exited
	})
	for {
		conn, err := net.DialTimeout("tcp", socksAddr, 100*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			break
		}
		select {
		case <-ctx.Done():
			t.Fatal("GOST did not start its SOCKS5 listener")
		case <-time.After(25 * time.Millisecond):
		}
	}
	if !socksAuth(t, socksAddr, "pass") || socksAuth(t, socksAddr, "wrong") {
		t.Fatal("GOST did not enforce the relay's authentication decisions")
	}
}

func socksAuth(t *testing.T, address, password string) bool {
	t.Helper()
	conn, err := net.DialTimeout("tcp", address, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close() }()
	if err := conn.SetDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Write([]byte{5, 1, 2}); err != nil {
		t.Fatal(err)
	}
	reply := make([]byte, 2)
	if _, err := io.ReadFull(conn, reply); err != nil || reply[0] != 5 || reply[1] != 2 {
		t.Fatalf("GOST SOCKS5 method selection: %v %v", reply, err)
	}
	username := "smoke"
	auth := append([]byte{1, byte(len(username))}, username...)
	auth = append(auth, byte(len(password)))
	auth = append(auth, password...)
	if _, err := conn.Write(auth); err != nil {
		t.Fatal(err)
	}
	if _, err := io.ReadFull(conn, reply); err != nil {
		t.Fatal(err)
	}
	return reply[0] == 1 && reply[1] == 0
}
