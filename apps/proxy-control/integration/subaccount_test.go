package integration

import (
	"fmt"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	v1 "github.com/DuanJinTaoToday/proxy/api/proxycontrol/v1"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/integration/testfixture"
)

func TestSubaccountAcceptance(t *testing.T) {
	f := setupUsage(t)
	tenant := func(phone string) (*v1.TenantAccess, string) {
		body := fmt.Sprintf(`{"phone":%q,"password":"tenant-password42"}`, phone)
		var a v1.TenantAccess
		readProto(t, testfixture.Request(f.http, "POST", "/v1/access/register", body, ""), &a)
		var login v1.LoginResponse
		readProto(t, testfixture.Request(f.http, "POST", "/v1/access/login", body, ""), &login)
		return &a, login.GetAccessToken()
	}
	admin, token := tenant("13800000044")
	_, other := tenant("13800000045")
	const base = "/v1/tenant/subaccounts"
	request := func(method, path, body, token string) *httptest.ResponseRecorder {
		return testfixture.Request(f.http, method, path, body, token)
	}
	create := func(name, secret, token string) *httptest.ResponseRecorder {
		return request("POST", base, fmt.Sprintf(`{"name":%q,"password":%q,"active":true,"limit_bytes":"15"}`, name, secret), token)
	}
	testfixture.AssertStatus(t, create("Bad1", "Old456", token), 400, "INVALID_SUBACCOUNT")
	testfixture.AssertStatus(t, create("acct", "123456", token), 400, "INVALID_PROXY_PASSWORD")
	testfixture.AssertStatus(t, create("acct", "abcdef", token), 400, "INVALID_PROXY_PASSWORD")
	var account v1.Subaccount
	created := create("acct", "Old456", token)
	readProto(t, created, &account)
	id := account.GetClientId()
	path := base + "/" + id
	testfixture.AssertStatus(t, create("acct", "Old456", token), 409, "SUBACCOUNT_NAME_EXISTS")
	readProto(t, create("acct", "Other456", other), &v1.Subaccount{})
	testfixture.AssertStatus(t, request("GET", path+"/password", "", other), 404, "SUBACCOUNT_NOT_FOUND")
	testfixture.AssertStatus(t, request("PATCH", path, `{"active":false,"update_mask":"active"}`, other), 404, "SUBACCOUNT_NOT_FOUND")
	testfixture.AssertStatus(t, request("PATCH", path, `{"update_mask":"name"}`, token), 400, "INVALID_UPDATE_MASK")
	list := request("GET", base, "", token)
	var accounts v1.Subaccounts
	readProto(t, list, &accounts)
	if got := accounts.GetAccounts(); len(got) != 1 || got[0].GetClientId() != id ||
		strings.Contains(list.Body.String(), "Old456") || strings.Contains(created.Body.String(), "Old456") {
		t.Fatalf("list or create exposed wrong data: %s", list.Body)
	}
	var cfg v1.SubaccountProxyConfig
	reveal := request("GET", path+"/proxy-config", "", token)
	readProto(t, reveal, &cfg)
	if cfg.GetUsername() != admin.GetPrimaryAccountName()+".acct" || cfg.GetPassword() != "Old456" ||
		cfg.GetHost() != "gateway.example.test" || cfg.GetPort() != 1080 || !cfg.GetExplicitSid() || reveal.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("incorrect proxy settings or cache policy")
	}
	loadBody := func(password string) string {
		return fmt.Sprintf(`{"username":%q,"password":%q}`, cfg.GetUsername(), password)
	}
	const load = "/v1/gateway/auth-state/load"
	const refresh = "/v1/gateway/auth-state/refresh"
	testfixture.AssertStatus(t, request("POST", load, loadBody("Old456"), reportToken), 403, "GATEWAY_DENIED")
	var state v1.AuthSnapshot
	readProto(t, request("POST", load, loadBody("Old456"), authToken), &state)
	ref := state.GetReference()
	if ref == "" || state.GetClientId() != id || !state.GetActive() || len(state.GetPasswordHash()) != 32 {
		t.Fatal("bad auth snapshot")
	}
	refreshBody := fmt.Sprintf(`{"references":[%q]}`, ref)
	current := func() *v1.AuthSnapshot {
		var batch v1.RefreshAuthStateResponse
		readProto(t, request("POST", refresh, refreshBody, authToken), &batch)
		if len(batch.GetSnapshots()) != 1 || batch.GetSnapshots()[0].GetClientId() != id {
			t.Fatal("credential identity changed")
		}
		return batch.GetSnapshots()[0]
	}
	execSQL(t, f.db, "UPDATE proxy_subaccounts SET used_bytes=13 WHERE client_id=?", id)
	testfixture.AssertStatus(t, request("PATCH", path, `{"limit_bytes":"14","update_mask":"limitBytes"}`, token), 400, "INVALID_SUBACCOUNT_LIMIT")
	readProto(t, request("PATCH", path, `{"password":"New456","active":false,"remark":"updated","update_mask":"password,active,remark,limitBytes"}`, token), &account)
	if account.LimitBytes != nil || account.GetUsedBytes() != 13 || account.GetRemark() != "updated" || account.GetClientId() != id || account.GetName() != "acct" {
		t.Fatalf("update lost account history or metadata: %s", account.String())
	}
	if disabled := current(); disabled.GetActive() || len(disabled.GetPasswordHash()) != 0 {
		t.Fatal("disabled snapshot retained verifier")
	}
	testfixture.AssertStatus(t, request("GET", path+"/proxy-config", "", token), 404, "SUBACCOUNT_NOT_FOUND")
	var password v1.SubaccountPassword
	readProto(t, request("GET", path+"/password", "", token), &password)
	if password.GetPassword() != "New456" {
		t.Fatal("password was not updated")
	}
	readProto(t, request("PATCH", path, `{"active":true,"update_mask":"active"}`, token), &account)
	testfixture.AssertStatus(t, request("POST", load, loadBody("Old456"), authToken), 404, "CREDENTIAL_DENIED")
	readProto(t, request("POST", load, loadBody("New456"), authToken), &state)
	if next := current(); !next.GetActive() || string(next.GetPasswordHash()) != string(state.GetPasswordHash()) {
		t.Fatal("refresh did not return new verifier")
	}
	readProto(t, request("PATCH", path, `{"active":false,"update_mask":"active"}`, token), &account)
	for i := range 8 {
		readProto(t, create(fmt.Sprintf("x%03d", i), "New456", token), &v1.Subaccount{})
	}
	var wg sync.WaitGroup
	var replies [2]*httptest.ResponseRecorder
	for i := range replies {
		wg.Go(func() { replies[i] = create(fmt.Sprintf("last%d", i), "New456", token) })
	}
	wg.Wait()
	if replies[0].Code+replies[1].Code != 200+409 {
		t.Fatalf("10th account race: %d and %d", replies[0].Code, replies[1].Code)
	}
}
