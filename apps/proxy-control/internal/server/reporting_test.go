package server_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/server"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/usage"
	kratoshttp "github.com/go-kratos/kratos/v3/transport/http"
)

func TestReportingRequiresGatewayToken(t *testing.T) {
	handler := kratoshttp.NewServer(kratoshttp.RequestDecoder(server.ReportingDecoder(0)))
	server.RegisterUsage(handler, usage.New(nil, nil))
	body := `{"gateway_id":"g1","boot_id":"boot","records":[{"file_id":"sessions","line":"{}","end_offset":"3"}]}`
	for _, token := range []string{"", "console.session"} {
		reply := request(handler, "POST", "/v1/reporting/batches", body, token)
		if reply.Code != http.StatusUnauthorized {
			t.Fatalf("submit: %d %s", reply.Code, reply.Body)
		}
	}
}

func TestReportingRejectsOversizedAndMalformedBodies(t *testing.T) {
	handler := kratoshttp.NewServer(kratoshttp.RequestDecoder(server.ReportingDecoder(64)))
	server.RegisterUsage(handler, usage.New(nil, nil))
	assertStatus(t, request(handler, "POST", "/v1/reporting/batches", strings.Repeat(" ", 65), ""), 413, "BATCH_TOO_LARGE")
	assertStatus(t, request(handler, "POST", "/v1/reporting/batches", "{", ""), 400, "INVALID_BATCH")
}
