package stickysession_test

import (
	"context"
	"errors"
	"testing"

	v1 "github.com/DuanJinTaoToday/proxy/api/proxycontrol/v1"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-gateway/internal/biz/stickysession"
)

type control struct {
	stickysession.Control
	seen     []string
	failOnce bool
	outage   error
}

func (c *control) AcquireStickySession(_ context.Context, req *v1.StickySessionRequest) (*v1.StickySession, error) {
	c.seen = append(c.seen, req.GetRequestKey())
	if c.failOnce {
		c.failOnce = false
		return nil, errors.New("lost response")
	}
	if c.outage != nil {
		return nil, c.outage
	}
	sid := int64(100000001)
	if req.GetSessionKey() == "b" {
		sid++
	}
	return &v1.StickySession{UpstreamSid: sid, ExpiresInMillis: 10000}, nil
}

func TestConfirmedMappingsSurviveControlOutageAndKeepSeparateEntryPreferences(t *testing.T) {
	remote := &control{failOnce: true}
	manager := stickysession.New(remote)
	a := stickysession.Identity{Client: "client", Key: "a"}
	b := stickysession.Identity{Client: "client", Key: "b"}
	for index, identity := range []stickysession.Identity{a, b} {
		attempt, err := manager.Resolve(t.Context(), identity, "reference")
		if err != nil {
			t.Fatal(err)
		}
		if err := manager.Confirm(t.Context(), &attempt, index); err != nil {
			t.Fatal(err)
		}
	}
	if len(remote.seen) != 3 || remote.seen[0] == "" || remote.seen[0] != remote.seen[1] {
		t.Fatal("acquire retry did not keep its request key")
	}
	remote.outage = errors.New("control unavailable")
	for index, identity := range []stickysession.Identity{a, b} {
		attempt, err := manager.Resolve(t.Context(), identity, "reference")
		if err != nil || attempt.SID != int64(100000001+index) || attempt.Preferred != index {
			t.Fatalf("outage lost confirmed mapping or entry preference for %s", identity.Key)
		}
	}
	if _, err := manager.Resolve(t.Context(), stickysession.Identity{Client: "other", Key: "a"}, "reference"); err == nil {
		t.Fatal("created a new mapping locally with control unavailable")
	}
	remote.outage = stickysession.ErrRejected
	if _, err := manager.Resolve(t.Context(), a, "reference"); !errors.Is(err, stickysession.ErrRejected) {
		t.Fatal("a confirmed cache overrode a definitive denial")
	}
}
