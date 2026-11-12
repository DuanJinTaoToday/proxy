package service_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	v1 "github.com/DuanJinTaoToday/proxy/api/proxycontrol/v1"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-gateway/internal/biz/authrelay"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-gateway/internal/service"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type control struct {
	load    *v1.AuthSnapshot
	refresh []*v1.AuthSnapshot
	err     error
}

func (c *control) Load(context.Context, string, string) (*v1.AuthSnapshot, error) {
	return c.load, c.err
}

func (c *control) Refresh(context.Context, []string) ([]*v1.AuthSnapshot, error) {
	return c.refresh, c.err
}

func snapshot() *v1.AuthSnapshot {
	hash := sha256.Sum256([]byte("password"))
	return &v1.AuthSnapshot{Reference: "ref", Username: "user", ClientId: "client", TenantId: "tenant",
		PasswordHash: hash[:], Active: true, Quota: &v1.AuthQuota{AvailableBytes: 11, AdmissionThresholdBytes: 10}}
}

const localToken = "test-local-token"

func authenticate(t *testing.T, handler http.Handler, token string) (int, bool, string) {
	t.Helper()
	req := httptest.NewRequest("POST", "/v1/gateway/auth", bytes.NewBufferString(`{"username":"user","password":"password","client":"127.0.0.1:3000"}`))
	req.Header.Set("Authorization", "Bearer "+token)
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	var result struct {
		OK bool
		ID string
	}
	if res.Code == 200 && json.Unmarshal(res.Body.Bytes(), &result) != nil {
		t.Fatalf("invalid GOST response: %s", res.Body)
	}
	return res.Code, result.OK, result.ID
}

func TestOfflineAuthenticationExpiresAtThirtySeconds(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	remote := &control{load: snapshot()}
	relay := authrelay.New(remote, func() time.Time { return now })
	handler := service.NewAuthRelay(relay, localToken)
	if status, ok, id := authenticate(t, handler, localToken); status != 200 || !ok || id != "client" {
		t.Fatalf("warmup: %d %v %q", status, ok, id)
	}
	remote.err = errors.New("offline")
	now = now.Add(29 * time.Second)
	if err := relay.Refresh(t.Context()); err == nil {
		t.Fatal("expected refresh failure")
	}
	if _, ok, _ := authenticate(t, handler, localToken); !ok {
		t.Fatal("warm state should allow within its original window")
	}
	now = now.Add(time.Second)
	if _, ok, _ := authenticate(t, handler, localToken); ok {
		t.Fatal("failed refresh extended the window")
	}
	restarted := authrelay.New(remote, func() time.Time { return now })
	restartedHandler := service.NewAuthRelay(restarted, localToken)
	if _, ok, _ := authenticate(t, restartedHandler, localToken); ok {
		t.Fatal("restart must not restore warm state")
	}
}

func TestRefreshAppliesDisableAndGatewayRevocation(t *testing.T) {
	now := time.Now()
	remote := &control{load: snapshot()}
	relay := authrelay.New(remote, func() time.Time { return now })
	handler := service.NewAuthRelay(relay, localToken)
	if code, ok, _ := authenticate(t, handler, "report-token"); code != 401 || ok {
		t.Fatalf("wrong token: %d %v", code, ok)
	}
	if _, ok, _ := authenticate(t, handler, localToken); !ok {
		t.Fatal("warmup failed")
	}
	remote.err = authrelay.ErrGatewayDenied
	if err := relay.Refresh(t.Context()); !errors.Is(err, authrelay.ErrGatewayDenied) {
		t.Fatalf("refresh: %v", err)
	}
	if _, ok, _ := authenticate(t, handler, localToken); ok {
		t.Fatal("revoked gateway retained warm state")
	}
	remote.err = nil
	if _, ok, _ := authenticate(t, handler, localToken); !ok {
		t.Fatal("warmup failed")
	}
	now = now.Add(time.Second)
	disabled := snapshot()
	disabled.Active, disabled.PasswordHash, disabled.Quota = false, nil, nil
	remote.refresh = []*v1.AuthSnapshot{disabled}
	if err := relay.Refresh(t.Context()); err != nil {
		t.Fatal(err)
	}
	if _, ok, _ := authenticate(t, handler, localToken); ok {
		t.Fatal("disabled credential retained warm state")
	}
}

func TestOfflineAdmissionStillChecksSourceExpiryAndQuota(t *testing.T) {
	now := time.Now()
	state := snapshot()
	state.AllowedSourceCidrs = []string{"192.0.2.0/24"}
	remote := &control{load: state}
	relay := authrelay.New(remote, func() time.Time { return now })
	handler := service.NewAuthRelay(relay, localToken)
	if _, ok, _ := authenticate(t, handler, localToken); ok {
		t.Fatal("disallowed source was accepted")
	}
	update := snapshot()
	update.AllowedSourceCidrs = []string{"127.0.0.0/8"}
	update.ExpiresAt = timestamppb.New(now.Add(3 * time.Second))
	update.Quota.ValidUntil = timestamppb.New(now.Add(2 * time.Second))
	remote.refresh = []*v1.AuthSnapshot{update}
	if err := relay.Refresh(t.Context()); err != nil {
		t.Fatal(err)
	}
	remote.err = errors.New("offline")
	now = now.Add(time.Second)
	if _, ok, _ := authenticate(t, handler, localToken); !ok {
		t.Fatal("allowed source should pass within both validity windows")
	}
	now = now.Add(time.Second)
	if _, ok, _ := authenticate(t, handler, localToken); ok {
		t.Fatal("quota snapshot crossed its known grant boundary")
	}
	remote.err = nil
	update = snapshot()
	update.ExpiresAt = timestamppb.New(now.Add(time.Second))
	remote.refresh = []*v1.AuthSnapshot{update}
	if err := relay.Refresh(t.Context()); err != nil {
		t.Fatal(err)
	}
	now = now.Add(time.Second)
	if _, ok, _ := authenticate(t, handler, localToken); ok {
		t.Fatal("credential expiry must be enforced independently of quota freshness")
	}
}
