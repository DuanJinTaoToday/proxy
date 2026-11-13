package authrelay

import (
	"context"
	"crypto/sha256"
	"testing"
	"time"

	v1 "github.com/DuanJinTaoToday/proxy/api/proxycontrol/v1"
)

type reportingControl struct {
	available int64
	onRefresh func()
}

func (c *reportingControl) Load(_ context.Context, user, password string) (*v1.AuthSnapshot, error) {
	digest := sha256.Sum256([]byte(password))
	return &v1.AuthSnapshot{Username: user, Reference: user, ClientId: user, TenantId: user, Active: true,
		PasswordHash: digest[:], Quota: &v1.AuthQuota{AvailableBytes: c.available, AdmissionThresholdBytes: 10}}, nil
}

func (c *reportingControl) Refresh(ctx context.Context, refs []string) ([]*v1.AuthSnapshot, error) {
	var snapshots []*v1.AuthSnapshot
	for _, ref := range refs {
		snapshot, _ := c.Load(ctx, ref, "password")
		snapshots = append(snapshots, snapshot)
	}
	if c.onRefresh != nil {
		c.onRefresh()
	}
	return snapshots, nil
}

func TestReportingFailureFreezesQuotaWithoutFreezingCredentials(t *testing.T) {
	control := &reportingControl{available: 20}
	now := time.Now()
	relay := New(control, func() time.Time { return now })
	var generation uint64
	ready := true
	relay.reporting = func() (uint64, bool) { return generation, ready }
	auth := func(user, want string) {
		t.Helper()
		if _, got := relay.Authenticate(t.Context(), user, "password", "127.0.0.1"); got != want {
			t.Fatalf("auth for %s = %s, want %s", user, got, want)
		}
	}
	refresh := func() {
		t.Helper()
		if err := relay.Refresh(t.Context()); err != nil {
			t.Fatal(err)
		}
	}
	auth("warm", "OK")
	now = now.Add(time.Second)
	control.onRefresh = func() { generation++; ready = false }
	refresh()
	control.onRefresh = nil
	now = now.Add(28 * time.Second)
	refresh()
	auth("warm", "OK")
	auth("cold", "AUTH_STATE_UNAVAILABLE")
	now = now.Add(time.Second)
	auth("warm", "AUTH_STATE_UNAVAILABLE")
	control.available = 9
	control.onRefresh = func() { generation++; ready = true }
	refresh()
	control.onRefresh = nil
	auth("warm", "AUTH_STATE_UNAVAILABLE")
	refresh()
	auth("warm", "QUOTA_INSUFFICIENT")
	control.available = 10
	refresh()
	auth("warm", "OK")
}
