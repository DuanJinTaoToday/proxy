package integration

import (
	"crypto/sha256"
	"fmt"
	"net/http/httptest"
	"sync"
	"testing"

	v1 "github.com/DuanJinTaoToday/proxy/api/proxycontrol/v1"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/integration/testfixture"
	"github.com/google/uuid"
)

func TestGatewayStickySessions(t *testing.T) {
	f := setupUsage(t)
	const tenantCredentials = `{"phone":"13900000044","password":"tenant-password42"}`
	post := func(path, body, token string) *httptest.ResponseRecorder {
		return testfixture.Request(f.http, "POST", path, body, token)
	}
	var tenant v1.TenantAccess
	readProto(t, post("/v1/access/register", tenantCredentials, ""), &tenant)
	var login v1.LoginResponse
	readProto(t, post("/v1/access/login", tenantCredentials, ""), &login)
	create := func(name string) string {
		var a v1.Subaccount
		readProto(t, post("/v1/tenant/subaccounts", fmt.Sprintf(`{"name":%q,"password":"proxy123","active":true}`, name), login.GetAccessToken()), &a)
		return a.GetClientId()
	}
	first := create("first")
	create("second")
	const secondGateway = "second-auth-token"
	hash := sha256.Sum256([]byte(secondGateway))
	execSQL(t, f.db, "INSERT INTO gateway_tokens (gateway_id,scope,token_hash) VALUES ('gateway-b','auth',?)", hash[:])
	load := func(name, token string) string {
		var snapshot v1.AuthSnapshot
		readProto(t, post("/v1/gateway/auth-state/load", fmt.Sprintf(`{"username":%q,"password":"proxy123"}`, tenant.GetPrimaryAccountName()+"."+name), token), &snapshot)
		return snapshot.GetReference()
	}
	firstA, secondA := load("first", authToken), load("second", authToken)
	req := func(action, ref, key, attempt, token string) *httptest.ResponseRecorder {
		body := fmt.Sprintf(`{"reference":%q,"session_key":%q,"lease_token":%q,"request_key":%q}`, ref, key, attempt, attempt)
		return post("/v1/gateway/sticky-sessions:"+action, body, token)
	}
	testfixture.AssertStatus(t, req("acquire", firstA, "", uuid.NewString(), secondGateway), 409, "INVALID_SESSION_LEASE")
	firstB := load("first", secondGateway)
	const key = "sticky"
	var wg sync.WaitGroup
	var replies [2]*httptest.ResponseRecorder
	tokens := [2]string{authToken, secondGateway}
	refs := [2]string{firstA, firstB}
	attempts := [2]string{uuid.NewString(), uuid.NewString()}
	for i := range tokens {
		wg.Go(func() { replies[i] = req("acquire", refs[i], key, attempts[i], tokens[i]) })
	}
	wg.Wait()
	winner := 0
	if replies[1].Code == 200 {
		winner = 1
	}
	testfixture.AssertStatus(t, replies[1-winner], 409, "SESSION_ESTABLISHING")
	var lease, retry v1.StickySession
	readProto(t, replies[winner], &lease)
	readProto(t, req("acquire", refs[winner], key, attempts[winner], tokens[winner]), &retry)
	if lease.GetLeaseToken() == "" || retry.GetLeaseToken() != lease.GetLeaseToken() ||
		retry.GetUpstreamSid() != lease.GetUpstreamSid() || retry.GetExpiresInMillis() != 0 {
		t.Fatal("first dial retry changed the lease")
	}
	testfixture.AssertStatus(t, req("commit", refs[1-winner], key, lease.GetLeaseToken(), tokens[1-winner]), 409, "INVALID_SESSION_LEASE")
	var committed v1.StickySession
	for range 2 {
		readProto(t, req("commit", refs[winner], key, lease.GetLeaseToken(), tokens[winner]), &committed)
		if committed.GetUpstreamSid() != lease.GetUpstreamSid() {
			t.Fatal("commit did not keep the acquired SID")
		}
	}
	var reused v1.StickySession
	readProto(t, req("acquire", refs[1-winner], key, uuid.NewString(), tokens[1-winner]), &reused)
	if reused.GetUpstreamSid() != lease.GetUpstreamSid() || reused.GetLeaseToken() != "" || reused.GetExpiresInMillis() <= 0 {
		t.Fatal("cross-gateway reuse failed")
	}

	seen := map[int64]bool{reused.GetUpstreamSid(): true}
	for _, tc := range []struct{ ref, key string }{{firstA, ""}, {firstA, "other"}, {secondA, key}} {
		var other v1.StickySession
		readProto(t, req("acquire", tc.ref, tc.key, uuid.NewString(), authToken), &other)
		if seen[other.GetUpstreamSid()] || other.GetLeaseToken() == "" {
			t.Fatal("SID was shared between sessions")
		}
		seen[other.GetUpstreamSid()] = true
	}
	// This fixture moves the persisted clock state to test expiry without waiting an hour.
	execSQL(t, f.db, "UPDATE sticky_sessions SET expires_at=now()-interval '1 second' WHERE client_id=? AND session_key=?", first, key)
	var next v1.StickySession
	readProto(t, req("acquire", firstA, key, uuid.NewString(), authToken), &next)
	if next.GetUpstreamSid() == reused.GetUpstreamSid() || next.GetLeaseToken() == "" {
		t.Fatal("expired session did not create a new generation")
	}
	testfixture.AssertStatus(t, req("commit", refs[winner], key, lease.GetLeaseToken(), tokens[winner]), 409, "INVALID_SESSION_LEASE")
	readProto(t, req("release", firstA, key, next.GetLeaseToken(), authToken), &v1.BaseResp{})
	readProto(t, req("acquire", firstB, key, uuid.NewString(), secondGateway), &v1.StickySession{})
}
