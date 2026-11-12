package integration

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	v1 "github.com/DuanJinTaoToday/proxy/api/proxycontrol/v1"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/integration/testfixture"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/biz/proxyauth"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/data/repo"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/server"
	httpservice "github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/service"
	"github.com/google/uuid"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

const authToken = "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
const reportToken = "BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBA"
const otherAuthToken = "CCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCA"

func TestProxyCredentialIssuanceIsTenantScoped(t *testing.T) {
	handler, _ := authFixture(t)
	admin := testfixture.Login(t, handler, "alice")
	member := testfixture.Login(t, handler, "bob")
	const create = "/v1/tenants/alpha/proxy-credentials"
	for _, denied := range []struct{ path, token string }{
		{"/v1/tenants/beta/proxy-credentials", admin}, {create, member},
	} {
		res := testfixture.Request(handler, "POST", denied.path, "{}", denied.token)
		if res.Code != 403 {
			t.Fatalf("unauthorized create: %d %s", res.Code, res.Body)
		}
	}
	reply := testfixture.Request(handler, "POST", create, `{"allowed_source_cidrs":["192.0.2.0/24"]}`, admin)
	var issued v1.CreateCredentialResponse
	readAuthProto(t, reply.Code, reply.Body.Bytes(), &issued)
	if issued.GetPassword() == "" || issued.GetUsername() == "" || issued.GetClientId() == "" || reply.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("issuance must return credentials once with cache disabled")
	}
	for _, denied := range []struct{ tenant, token string }{{"beta", admin}, {"alpha", member}} {
		res := testfixture.Request(handler, "POST", fmt.Sprintf("/v1/tenants/%s/proxy-credentials/%s:disable", denied.tenant, issued.ClientId), "{}", denied.token)
		if res.Code != 403 {
			t.Fatalf("unauthorized disable: %d", res.Code)
		}
	}
	disable := create + "/" + issued.GetClientId() + ":disable"
	for range 2 {
		if reply := testfixture.Request(handler, "POST", disable, "{}", admin); reply.Code != 200 || strings.Contains(reply.Body.String(), issued.Password) {
			t.Fatalf("disable: %d", reply.Code)
		}
	}
}

func authFixture(t *testing.T) (http.Handler, *gorm.DB) {
	t.Helper()
	dsn := os.Getenv("PROXY_CONTROL_TEST_DSN")
	if dsn == "" {
		t.Skip("set PROXY_CONTROL_TEST_DSN to a local PostgreSQL URL with schema creation permission")
	}
	address, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	admin := openAuthTestDB(t, dsn)
	schema := "auth_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	authSQL(t, admin, "CREATE SCHEMA "+schema)
	t.Cleanup(func() { authSQL(t, admin, "DROP SCHEMA "+schema+" CASCADE") })
	query := address.Query()
	query.Set("search_path", schema)
	address.RawQuery = query.Encode()
	db := openAuthTestDB(t, address.String())
	raw, err := os.ReadFile("../migrations/00001_proxy_auth.sql")
	if err != nil {
		t.Fatal(err)
	}
	up, _, _ := strings.Cut(string(raw), "-- +goose Down")
	authSQL(t, db, up)
	authSQL(t, db, `INSERT INTO gateways (id, name) VALUES ('a', 'A'), ('b', 'B');
		INSERT INTO traffic_grants (tenant_id, granted_bytes, starts_at, activated_at, source, created_by)
		VALUES ('alpha', 104857600, now()-interval '1 day', now()-interval '1 day', 'recharge', 'test');`)
	for _, token := range []struct{ gateway, scope, raw string }{{"a", "auth", authToken}, {"b", "auth", otherAuthToken}, {"a", "report", reportToken}} {
		hash := sha256.Sum256([]byte(token.raw))
		authSQL(t, db, "INSERT INTO gateway_tokens (gateway_id, scope, token_hash) VALUES (?, ?, ?)", token.gateway, token.scope, hash[:])
	}
	accessService := testfixture.AuthAccess(t)
	handler := server.NewHTTPServer("127.0.0.1:0", httpservice.NewAccess(accessService))
	server.RegisterProxyAuth(handler, httpservice.NewProxyAuth(proxyauth.New(repo.NewProxyAuthRepository(db), accessService)))
	return handler, db
}

func openAuthTestDB(t *testing.T, dsn string) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	pool, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := pool.Close(); err != nil {
			t.Error(err)
		}
	})
	return db
}

func authSQL(t *testing.T, db *gorm.DB, statement string, args ...any) {
	t.Helper()
	if err := db.Exec(statement, args...).Error; err != nil {
		t.Fatal(err)
	}
}

func readAuthProto(t *testing.T, status int, body []byte, msg proto.Message) {
	t.Helper()
	if status != 200 {
		t.Fatalf("expected success, got HTTP %d", status)
	}
	if err := protojson.Unmarshal(body, msg); err != nil {
		t.Fatal(err)
	}
}

func TestGatewayAuthStateOnlyRefreshesItsOwnReferences(t *testing.T) {
	handler, db := authFixture(t)
	admin := testfixture.Login(t, handler, "alice")
	const create = "/v1/tenants/alpha/proxy-credentials"
	reply := testfixture.Request(handler, "POST", create, "{}", admin)
	var issued v1.CreateCredentialResponse
	readAuthProto(t, reply.Code, reply.Body.Bytes(), &issued)

	load := &v1.LoadAuthStateRequest{Username: issued.Username, Password: issued.Password}
	const loadPath = "/v1/gateway/auth-state/load"
	const refreshPath = "/v1/gateway/auth-state/refresh"
	for _, token := range []string{"", reportToken, admin} {
		if res := authStateRequest(t, handler, loadPath, token, load); res.Code != 401 {
			t.Fatalf("wrong auth token: %d", res.Code)
		}
	}
	bad := authStateRequest(t, handler, loadPath, authToken, &v1.LoadAuthStateRequest{Username: issued.Username, Password: "wrong"})
	if bad.Code != 404 {
		t.Fatalf("bad password: %d", bad.Code)
	}
	var state v1.AuthSnapshot
	res := authStateRequest(t, handler, loadPath, authToken, load)
	readAuthProto(t, res.Code, res.Body.Bytes(), &state)
	if state.ClientId != issued.ClientId || !state.Active || state.GetQuota().GetAvailableBytes() != 104857600 ||
		state.GetQuota().GetAdmissionThresholdBytes() != 10485760 {
		t.Fatal("valid auth state lacks the stable ID or committed quota")
	}

	refresh := &v1.RefreshAuthStateRequest{References: []string{state.Reference}}
	for _, tc := range []struct {
		token  string
		status int
	}{{reportToken, 401}, {admin, 401}, {otherAuthToken, 400}} {
		if res := authStateRequest(t, handler, refreshPath, tc.token, refresh); res.Code != tc.status {
			t.Fatalf("wrong refresh identity: %d, want %d", res.Code, tc.status)
		}
	}
	if res := testfixture.Request(handler, "POST", create+"/"+issued.ClientId+":disable", "{}", admin); res.Code != 200 {
		t.Fatalf("disable: %d", res.Code)
	}
	res = authStateRequest(t, handler, refreshPath, authToken, refresh)
	var update v1.RefreshAuthStateResponse
	readAuthProto(t, res.Code, res.Body.Bytes(), &update)
	if len(update.Snapshots) != 1 || update.Snapshots[0].Active || len(update.Snapshots[0].PasswordHash) != 0 ||
		update.Snapshots[0].ClientId != issued.ClientId || update.Snapshots[0].TenantId != "alpha" {
		t.Fatal("refresh must report disabled status and preserve the historical mapping")
	}
	authSQL(t, db, "UPDATE gateways SET status='disabled' WHERE id='a'")
	if res := authStateRequest(t, handler, refreshPath, authToken, refresh); res.Code != 403 {
		t.Fatalf("disabled gateway: %d", res.Code)
	}
}

func authStateRequest(t *testing.T, handler http.Handler, path, token string, input proto.Message) *httptest.ResponseRecorder {
	t.Helper()
	body, err := protojson.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest("POST", path, bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/protojson")
	req.Header.Set("Accept", "application/protojson")
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	return res
}
