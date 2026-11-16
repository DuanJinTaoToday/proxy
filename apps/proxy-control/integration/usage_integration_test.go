package integration

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	v1 "github.com/DuanJinTaoToday/proxy/api/proxycontrol/v1"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/integration/testfixture"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/biz/access"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/biz/product"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/biz/subaccount"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/biz/usage"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/biz/wallet"
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
	db   *gorm.DB
	http http.Handler
}

func setupUsage(t *testing.T, suppliers ...wallet.StaticSupplier) usageFixture {
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
	for _, path := range []string{"00001_proxy_auth.sql", "00002_usage.sql", "00003_gateway_lifecycle.sql", "00004_usage_gaps.sql", "00007_tenant_accounts.sql", "00008_wallet_orders.sql", "00009_products.sql", "00010_proxy_subaccounts.sql", "00011_traffic_metering.sql", "00012_traffic_alerts.sql", "00013_static_purchases.sql", "00014_static_renewals.sql"} {
		migration, err := os.ReadFile("../migrations/" + path)
		if err != nil {
			t.Fatal(err)
		}
		up, _, _ := strings.Cut(string(migration), "-- +goose Down")
		execSQL(t, db, up)
	}
	execSQL(t, db, "INSERT INTO gateways (id, name) VALUES ('gateway-a', 'A'), ('gateway-b', 'B')")
	for _, token := range []struct{ raw, scope string }{{reportToken, "report"}, {authToken, "auth"}} {
		hash := sha256.Sum256([]byte(token.raw))
		execSQL(t, db, "INSERT INTO gateway_tokens (gateway_id, scope, token_hash) VALUES ('gateway-a', ?, ?)", token.scope, hash[:])
	}

	execSQL(t, db, "INSERT INTO operators (id, username, password_hash) VALUES ('operator', 'operator', ?)", testfixture.PasswordHash)
	execSQL(t, db, "INSERT INTO operator_permissions (operator_id, permission) VALUES ('operator', 'read_operations')")
	accessService, err := access.NewPersistent(repo.NewAccountRepository(db))
	if err != nil {
		t.Fatal(err)
	}
	service := usage.New(repo.NewUsageRepository(db), accessService)
	handler := server.NewHTTPServer("127.0.0.1:0", httpservice.NewAccess(accessService),
		kratoshttp.RequestDecoder(server.ReportingDecoder(0)), kratoshttp.ResponseEncoder(server.ControlResponseEncoder))
	server.RegisterUsage(handler, httpservice.NewUsage(service))
	walletStore := repo.NewWalletRepository(db)
	if len(suppliers) != 0 {
		walletStore, err = repo.NewWalletRepositoryWithStatic(db, suppliers[0], base64.RawURLEncoding.EncodeToString(make([]byte, 32)))
		if err != nil {
			t.Fatal(err)
		}
	}
	server.RegisterWallet(handler, httpservice.NewWallet(wallet.NewService(walletStore, accessService)))
	server.RegisterProduct(handler, httpservice.NewProduct(product.NewService(repo.NewProductRepository(db), accessService)))
	subs, err := subaccount.NewService(repo.NewSubaccountRepository(db), accessService, &conf.Biz{
		Subaccounts: &conf.Subaccounts{PasswordKey: base64.RawURLEncoding.EncodeToString(make([]byte, 32)), GatewayHost: "gateway.example.test", GatewayPort: 1080},
	})
	if err != nil {
		t.Fatal(err)
	}
	server.RegisterSubaccounts(handler, httpservice.NewSubaccounts(subs))
	return usageFixture{db: db, http: handler}
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

const authToken = "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
const reportToken = "BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBA"
const beforeMidnight = "2020-11-10T23:59:00+08:00"

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
