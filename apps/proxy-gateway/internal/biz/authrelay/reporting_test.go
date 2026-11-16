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
		PasswordHash: digest[:], Quota: &v1.AuthQuota{AvailableBytes: c.available}}, nil
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
	control.available = 0
	control.onRefresh = func() { generation++; ready = true }
	refresh()
	control.onRefresh = nil
	auth("warm", "AUTH_STATE_UNAVAILABLE")
	refresh()
	auth("warm", "QUOTA_INSUFFICIENT")
	control.available = 1
	refresh()
	auth("warm", "OK")
}
func TestCumulativeLimitUsesCurrentTenantUsageVersion(t *testing.T) {
	control := &versionedControl{rows: make(map[string]*v1.AuthSnapshot)}
	limit := int64(5)
	set := func(user string, version, used int64, limit *int64) {
		digest := sha256.Sum256([]byte("password"))
		control.rows[user] = &v1.AuthSnapshot{Username: user, Reference: user, ClientId: user, TenantId: "tenant",
			Active: true, PasswordHash: digest[:], LimitBytes: limit, UsedBytes: used,
			Quota: &v1.AuthQuota{AvailableBytes: 1, UsageVersion: version}}
	}
	set("first", 1, 4, &limit)
	set("second", 1, 0, nil)
	now := time.Now()
	relay := New(control, func() time.Time { return now })
	auth := func(user, want string) {
		t.Helper()
		if _, got := relay.Authenticate(t.Context(), user, "password", ""); got != want {
			t.Fatalf("%s auth=%s want %s", user, got, want)
		}
	}
	refresh := func() {
		t.Helper()
		now = now.Add(time.Second)
		if err := relay.Refresh(t.Context()); err != nil {
			t.Fatal(err)
		}
	}
	auth("first", "OK")
	auth("second", "OK")
	set("first", 1, 4, &limit)
	set("second", 2, 0, nil)
	refresh()
	auth("first", "AUTH_STATE_UNAVAILABLE")
	auth("second", "OK")
	set("first", 2, 5, &limit)
	refresh()
	auth("first", "SUBACCOUNT_LIMIT_REACHED")
	set("first", 1, 4, &limit)
	refresh()
	auth("first", "SUBACCOUNT_LIMIT_REACHED")
	set("first", 3, 5, nil)
	set("second", 3, 0, nil)
	refresh()
	auth("first", "OK")
}

type versionedControl struct {
	rows map[string]*v1.AuthSnapshot
}

func (c *versionedControl) Load(_ context.Context, user, _ string) (*v1.AuthSnapshot, error) {
	return c.rows[user], nil
}

func (c *versionedControl) Refresh(_ context.Context, refs []string) ([]*v1.AuthSnapshot, error) {
	snapshots := make([]*v1.AuthSnapshot, 0, len(refs))
	for _, ref := range refs {
		snapshots = append(snapshots, c.rows[ref])
	}
	return snapshots, nil
}
