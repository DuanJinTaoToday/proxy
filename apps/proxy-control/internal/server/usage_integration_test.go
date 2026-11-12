package server_test

import (
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	v1 "github.com/DuanJinTaoToday/proxy/api/proxycontrol/v1"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/access"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/conf"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/server"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/usage"
	kratoshttp "github.com/go-kratos/kratos/v3/transport/http"
	"github.com/google/uuid"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

const reportToken = "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"

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
	migration, err := os.ReadFile("../../migrations/00001_usage.sql")
	if err != nil {
		t.Fatal(err)
	}
	up, _, _ := strings.Cut(string(migration), "-- +goose Down")
	execSQL(t, db, up)
	// Issue #3 owns the production credential table. Only its immutable
	// attribution columns are needed by these controlled Recorder samples.
	execSQL(t, db, `CREATE TABLE proxy_credentials (client_id text PRIMARY KEY, tenant_id text NOT NULL);
		INSERT INTO proxy_credentials VALUES ('client-a', 'alpha'), ('client-b', 'beta');
		INSERT INTO gateways (id, name) VALUES ('gateway-a', 'A'), ('gateway-b', 'B');`)
	hash := sha256.Sum256([]byte(reportToken))
	execSQL(t, db, "INSERT INTO gateway_tokens (gateway_id, scope, token_hash) VALUES ('gateway-a', 'report', ?)", hash[:])

	t.Setenv("TEST_USAGE_PASSWORD_HASH", fixtureHash)
	member := conf.TenantRole_TENANT_ROLE_MEMBER
	accessService, err := access.New(&conf.Access{
		Tenants: []*conf.Tenant{{Id: "alpha", Name: "A"}, {Id: "beta", Name: "B"}},
		Users: []*conf.User{
			{Username: "alice", PasswordHashEnv: "TEST_USAGE_PASSWORD_HASH", Memberships: []*conf.Membership{{TenantId: "alpha", Role: member}}},
			{Username: "bob", PasswordHashEnv: "TEST_USAGE_PASSWORD_HASH", Memberships: []*conf.Membership{{TenantId: "beta", Role: member}}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	service := usage.New(db, accessService)
	handler := server.NewHTTPServer("127.0.0.1:0", accessService,
		kratoshttp.RequestDecoder(server.ReportingDecoder(0)), kratoshttp.ResponseEncoder(server.UsageResponseEncoder))
	server.RegisterUsage(handler, service)
	return usageFixture{db: db, service: service, http: handler, alice: login(t, handler, "alice"), bob: login(t, handler, "bob")}
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
	return request(f.http, "POST", "/v1/reporting/batches", string(body), token)
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
	readProto(t, request(f.http, "GET", path, "", token), &reply)
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

func assertDays(t *testing.T, reply *v1.DailyUsageResponse, days ...*v1.DailyUsage) {
	t.Helper()
	if !proto.Equal(&v1.DailyUsageResponse{Days: reply.Days}, &v1.DailyUsageResponse{Days: days}) || reply.LastReceivedAt == nil {
		t.Fatalf("daily response = %v, want %v and a received timestamp", reply, days)
	}
}
