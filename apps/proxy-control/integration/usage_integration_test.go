package integration

import (
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	v1 "github.com/DuanJinTaoToday/proxy/api/proxycontrol/v1"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/integration/testfixture"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/biz/access"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/biz/proxyauth"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/biz/usage"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/conf"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/data/repo"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/server"
	httpservice "github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/service"
	kratoshttp "github.com/go-kratos/kratos/v3/transport/http"
	"github.com/google/uuid"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type usageFixture struct {
	db      *gorm.DB
	service *usage.Service
	http    http.Handler
	alice   string
	bob     string
}

func setupUsage(t *testing.T) usageFixture {
	t.Helper()
	dsn := os.Getenv("PROXY_CONTROL_TEST_DSN")
	if dsn == "" {
		t.Skip("set PROXY_CONTROL_TEST_DSN to a local PostgreSQL URL with schema creation permission")
	}
	address, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	admin := connect(t, dsn)
	schema := "usage_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	execSQL(t, admin, "CREATE SCHEMA "+schema)
	t.Cleanup(func() { execSQL(t, admin, "DROP SCHEMA "+schema+" CASCADE") })
	query := address.Query()
	query.Set("search_path", schema)
	address.RawQuery = query.Encode()
	db := connect(t, address.String())
	for _, path := range []string{"00001_proxy_auth.sql", "00002_usage.sql", "00003_gateway_lifecycle.sql", "00004_usage_gaps.sql", "00005_mock_supplier_usage.sql"} {
		migration, err := os.ReadFile("../migrations/" + path)
		if err != nil {
			t.Fatal(err)
		}
		up, _, _ := strings.Cut(string(migration), "-- +goose Down")
		execSQL(t, db, up)
	}
	execSQL(t, db, `INSERT INTO proxy_credentials (client_id, tenant_id, username, secret_hash, created_by)
		VALUES ('client-a', 'alpha', 'fixture-a', decode(repeat('00', 32), 'hex'), 'test'),
		       ('client-b', 'beta', 'fixture-b', decode(repeat('00', 32), 'hex'), 'test');
		INSERT INTO gateways (id, name) VALUES ('gateway-a', 'A'), ('gateway-b', 'B');`)
	for _, token := range []struct{ raw, scope string }{{reportToken, "report"}, {authToken, "auth"}} {
		hash := sha256.Sum256([]byte(token.raw))
		execSQL(t, db, "INSERT INTO gateway_tokens (gateway_id, scope, token_hash) VALUES ('gateway-a', ?, ?)", token.scope, hash[:])
	}

	member := conf.TenantRole_TENANT_ROLE_MEMBER
	accessService, err := access.New(&conf.Access{
		Tenants: []*conf.Tenant{{Id: "alpha", Name: "A"}, {Id: "beta", Name: "B"}},
		Users: []*conf.User{
			{Username: "alice", PasswordHash: testfixture.PasswordHash, Memberships: []*conf.Membership{{TenantId: "alpha", Role: conf.TenantRole_TENANT_ROLE_ADMIN}}},
			{Username: "bob", PasswordHash: testfixture.PasswordHash, Memberships: []*conf.Membership{{TenantId: "beta", Role: member}}},
			{Username: "operator", PasswordHash: testfixture.PasswordHash, PlatformOperator: true},
			{Username: "beta-admin", PasswordHash: testfixture.PasswordHash, Memberships: []*conf.Membership{{TenantId: "beta", Role: conf.TenantRole_TENANT_ROLE_ADMIN}}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	service := usage.New(repo.NewUsageRepository(db), accessService)
	handler := server.NewHTTPServer("127.0.0.1:0", httpservice.NewAccess(accessService),
		kratoshttp.RequestDecoder(server.ReportingDecoder(0)), kratoshttp.ResponseEncoder(server.UsageResponseEncoder))
	server.RegisterProxyAuth(handler, httpservice.NewProxyAuth(proxyauth.New(repo.NewProxyAuthRepository(db), accessService)))
	server.RegisterUsage(handler, httpservice.NewUsage(service))
	return usageFixture{db: db, service: service, http: handler, alice: testfixture.Login(t, handler, "alice"), bob: testfixture.Login(t, handler, "bob")}
}

func connect(t *testing.T, dsn string) *gorm.DB {
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

func execSQL(t *testing.T, db *gorm.DB, statement string, args ...any) {
	t.Helper()
	if err := db.Exec(statement, args...).Error; err != nil {
		t.Fatal(err)
	}
}

func recorder(offset uint64, client, recordedAt string, input, output, inputDelta, outputDelta int) *v1.RecorderLine {
	line := fmt.Sprintf(`{"network":"tcp","clientID":%q,"src":"127.0.0.1:1000","dst":"127.0.0.1:2000",
		"time":%q,"inputBytes":%d,"outputBytes":%d,"inputBytesDelta":%d,"outputBytesDelta":%d}`,
		client, recordedAt, input, output, inputDelta, outputDelta)
	line = strings.ReplaceAll(line, "\n", "")
	return &v1.RecorderLine{FileId: "sessions", FileOffset: offset, EndOffset: offset + uint64(len(line)) + 1, Line: line}
}

func submit(t *testing.T, f usageFixture, gateway, token string, lines ...*v1.RecorderLine) *v1.SubmitBatchResponse {
	t.Helper()
	reply := submitResponse(t, f, gateway, token, lines...)
	var batch v1.SubmitBatchResponse
	readProto(t, reply, &batch)
	return &batch
}

func submitResponse(t *testing.T, f usageFixture, gateway, token string, lines ...*v1.RecorderLine) *httptest.ResponseRecorder {
	t.Helper()
	body, err := protojson.Marshal(&v1.SubmitBatchRequest{GatewayId: gateway, BootId: "boot", Records: lines})
	if err != nil {
		t.Fatal(err)
	}
	return testfixture.Request(f.http, "POST", "/v1/reporting/batches", string(body), token)
}

func readProto(t *testing.T, reply *httptest.ResponseRecorder, message proto.Message) {
	t.Helper()
	if reply.Code != http.StatusOK {
		t.Fatalf("unexpected HTTP status %d: %s", reply.Code, reply.Body)
	}
	if err := protojson.Unmarshal(reply.Body.Bytes(), message); err != nil {
		t.Fatal(err)
	}
}

func daily(t *testing.T, f usageFixture, tenant, token string) *v1.DailyUsageResponse {
	t.Helper()
	var reply v1.DailyUsageResponse
	path := "/v1/tenants/" + tenant + "/usage/daily?start_date=2020-11-10&end_date=2020-11-11"
	readProto(t, testfixture.Request(f.http, "GET", path, "", token), &reply)
	return &reply
}

func TestUsageShanghaiMidnight(t *testing.T) {
	f := setupUsage(t)
	first := recorder(0, "client-a", "2020-11-10T23:59:00+08:00", 130, 214, 130, 214)
	second := recorder(first.GetEndOffset(), "client-a", "2020-11-11T00:01:00+08:00", 180, 294, 50, 80)
	if got := submit(t, f, "gateway-a", reportToken, first, second).GetBillable(); got != 2 {
		t.Fatalf("billable records = %d, want 2", got)
	}
	assertDays(t, daily(t, f, "alpha", f.alice),
		usageDay("2020-11-10", 130, 214, 344), usageDay("2020-11-11", 50, 80, 130))
}

func usageDay(date string, input, output, billed uint64) *v1.DailyUsage {
	return &v1.DailyUsage{Date: date, Amount: &v1.UsageAmount{InputBytes: input, OutputBytes: output, TotalBytes: input + output, BillableBytes: billed}}
}

func TestOperationsDailyUsage(t *testing.T) {
	f := setupUsage(t)
	operator := testfixture.Login(t, f.http, "operator")
	alpha := recorder(0, "client-a", "2020-11-10T23:59:00+08:00", 130, 214, 130, 214)
	beta := recorder(alpha.GetEndOffset(), "client-b", "2020-11-11T00:01:00+08:00", 50, 80, 50, 80)
	if got := submit(t, f, "gateway-a", reportToken, alpha, beta).GetBillable(); got != 2 {
		t.Fatalf("billable records = %d, want 2", got)
	}

	for _, tc := range []struct {
		role, tenant, token string
		day                 *v1.DailyUsage
	}{
		{"tenant admin", "alpha", f.alice, usageDay("2020-11-10", 130, 214, 344)},
		{"tenant member", "beta", f.bob, usageDay("2020-11-11", 50, 80, 130)},
	} {
		t.Run(tc.role, func(t *testing.T) {
			want := daily(t, f, tc.tenant, tc.token)
			assertDays(t, want, tc.day)
			path := "/v1/operations/usage?tenant_id=" + tc.tenant +
				"&start_time=2020-11-09T16:00:00Z&end_time=2020-11-11T16:00:00Z&granularity=USAGE_GRANULARITY_DAY"
			var got v1.QueryUsageResponse
			readProto(t, testfixture.Request(f.http, "GET", path, "", operator), &got)
			buckets := got.GetBuckets()
			start, err := time.Parse(time.RFC3339, tc.day.GetDate()+"T00:00:00+08:00")
			if err != nil {
				t.Fatal(err)
			}
			if len(buckets) != 1 || !buckets[0].GetStartTime().AsTime().Equal(start) ||
				!proto.Equal(buckets[0].GetAmount(), want.GetDays()[0].GetAmount()) {
				t.Fatalf("operations buckets = %v, tenant days = %v", buckets, want.GetDays())
			}
			for _, denied := range []struct {
				token string
				code  int
			}{{tc.token, http.StatusForbidden}, {"", http.StatusUnauthorized}} {
				reply := testfixture.Request(f.http, "GET", path, "", denied.token)
				if reply.Code != denied.code {
					t.Fatalf("HTTP status = %d, want %d: %s", reply.Code, denied.code, reply.Body)
				}
			}
		})
	}

	t.Run("daily range limit", func(t *testing.T) {
		path := "/v1/operations/usage?tenant_id=alpha&granularity=USAGE_GRANULARITY_DAY&start_time=2019-11-10T16:00:00Z&end_time="
		var got v1.QueryUsageResponse
		readProto(t, testfixture.Request(f.http, "GET", path+"2020-11-10T16:00:00Z", "", operator), &got)
		if len(got.GetBuckets()) != 1 {
			t.Fatalf("366-day query returned %d buckets, want 1", len(got.GetBuckets()))
		}
		reply := testfixture.Request(f.http, "GET", path+"2020-11-11T16:00:00Z", "", operator)
		if reply.Code != http.StatusBadRequest || !strings.Contains(reply.Body.String(), "INVALID_TIME_RANGE") {
			t.Fatalf("367-day query returned HTTP %d: %s", reply.Code, reply.Body)
		}
	})
}

func assertDays(t *testing.T, reply *v1.DailyUsageResponse, days ...*v1.DailyUsage) {
	t.Helper()
	if !proto.Equal(&v1.DailyUsageResponse{Days: reply.Days}, &v1.DailyUsageResponse{Days: days}) || reply.LastReceivedAt == nil {
		t.Fatalf("daily response = %v, want %v and a received timestamp", reply, days)
	}
}
