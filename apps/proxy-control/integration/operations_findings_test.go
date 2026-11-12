package integration

import (
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	v1 "github.com/DuanJinTaoToday/proxy/api/proxycontrol/v1"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/integration/testfixture"
	"google.golang.org/protobuf/proto"
)

const anomaliesPath = "/v1/operations/anomalies"
const gapsPath = "/v1/operations/gaps"

func findingsReader(t *testing.T, f usageFixture) func(string, proto.Message) {
	t.Helper()
	operator := testfixture.Login(t, f.http, "operator")
	return func(path string, reply proto.Message) {
		t.Helper()
		response := testfixture.Request(f.http, "GET", path, "", operator)
		readProto(t, response, reply)
		for _, private := range []string{
			`"client_id":`, "client-a", "client-b", "credential-marker", "payload-marker", "secret_hash", "fixture-a", "detail",
		} {
			if strings.Contains(response.Body.String(), private) {
				t.Fatalf("response exposed %q: %s", private, response.Body)
			}
		}
	}
}

func TestOperationsRecordAnomalies(t *testing.T) {
	f := setupUsage(t)
	// Include historical extra fields to test the response allowlist independently of ingestion.
	execSQL(t, f.db, `INSERT INTO record_anomalies
		(gateway_id, boot_id, file_id, file_offset, kind, detail, detected_at, resolved_at) VALUES
		('gateway-a', 'boot-a', 'sessions', 11, 'invalid_counters',
		 '{"client_id":"client-a","password":"credential-marker","raw":"payload-marker"}', '2026-11-12T02:00:01Z', NULL),
		('gateway-b', 'boot-b', 'sessions', 22, 'invalid_counters',
		 '{"client_id":"client-b"}', '2026-11-12T02:00:01Z', '2026-11-12T02:00:02Z'),
		('gateway-a', 'boot-b', 'sessions', 33, 'invalid_record',
		 '{}', '2026-11-12T02:00:00Z', NULL)`)
	read := findingsReader(t, f)
	for _, tc := range []struct {
		query string
		want  int
	}{
		{"gateway_id=gateway-a", 2}, {"boot_id=boot-b", 2}, {"tenant_id=alpha", 1},
		{"kind=invalid_record", 1}, {"gateway_id=gateway-b&boot_id=boot-a", 0},
		{"gateway_id=gateway-a&boot_id=boot-a&tenant_id=alpha&kind=invalid_counters", 1},
	} {
		var result v1.ListRecordAnomaliesResponse
		read(anomaliesPath+"?"+tc.query, &result)
		if len(result.GetAnomalies()) != tc.want {
			t.Fatalf("filter %s returned %v, want %d rows", tc.query, &result, tc.want)
		}
	}
	path := anomaliesPath + "?page_size=1"
	operator := testfixture.Login(t, f.http, "operator")
	for i, offset := range []uint64{22, 11, 33} {
		var page v1.ListRecordAnomaliesResponse
		read(path, &page)
		rows := page.GetAnomalies()
		if len(rows) != 1 || rows[0].GetFileOffset() != offset || rows[0].GetFileId() != "sessions" ||
			rows[0].GetAnomalyId() == "" || (page.GetNextPageToken() != "") != (i < 2) {
			t.Fatalf("unexpected anomaly page %d: %v", i, &page)
		}
		if i == 0 && (rows[0].GetTenantId() != "beta" || rows[0].GetResolvedAt() == nil) {
			t.Fatalf("missing tenant or resolution time: %v", rows[0])
		}
		if i == 2 && rows[0].GetTenantId() != "" {
			t.Fatalf("unattributed anomaly received a tenant: %v", rows[0])
		}
		token := url.QueryEscape(page.GetNextPageToken())
		if i == 0 {
			testfixture.AssertStatus(t, testfixture.Request(f.http, "GET", anomaliesPath+"?tenant_id=alpha&page_token="+token, "", operator), 400, "INVALID_PAGE_TOKEN")
		}
		path = anomaliesPath + "?page_size=1&page_token=" + token
	}

	// The operator uses the returned locator on a separately accessible
	// gateway file. Control only receives the HTTP batch.
	first := recorder(0, "client-a", beforeMidnight, 10, 20, 10, 20)
	bad := recorder(first.GetEndOffset(), "client-a", beforeMidnight, 10, 20, 11, 20)
	root := t.TempDir()
	dir := filepath.Join(root, "gateway-a", "boot")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "sessions"), []byte(first.GetLine()+"\n"+bad.GetLine()+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if ack := submit(t, f, "gateway-a", reportToken, first, bad); ack.GetBillable() != 1 || ack.GetQuarantined() != 1 {
		t.Fatalf("unexpected dispositions: %v", ack)
	}
	var located v1.ListRecordAnomaliesResponse
	read(anomaliesPath+"?gateway_id=gateway-a&boot_id=boot&tenant_id=alpha&kind=invalid_counters", &located)
	if len(located.GetAnomalies()) != 1 {
		t.Fatalf("expected one recorded anomaly: %v", &located)
	}
	row := located.GetAnomalies()[0]
	file, err := os.Open(filepath.Join(root, row.GetGatewayId(), row.GetBootId(), row.GetFileId()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := file.Close(); err != nil {
			t.Error(err)
		}
	})
	line := make([]byte, len(bad.GetLine())+1)
	if _, err := file.ReadAt(line, int64(row.GetFileOffset())); err != nil || string(line) != bad.GetLine()+"\n" {
		t.Fatalf("anomaly offset did not locate the quarantined Recorder line: %v", err)
	}
}

func TestOperationsUsageGapStates(t *testing.T) {
	f := setupUsage(t)
	read := findingsReader(t, f)
	post := func(path, body string) {
		t.Helper()
		testfixture.AssertStatus(t, testfixture.Request(f.http, "POST", "/v1/reporting/"+path, body, reportToken), 200, "")
	}
	execSQL(t, f.db, `INSERT INTO gateway_boots (gateway_id, boot_id, first_seen_at, last_heartbeat_at, exited_at)
		VALUES ('gateway-a', 'disconnected', '2026-11-12T02:00:00Z', '2026-11-12T02:00:00Z', NULL),
		       ('gateway-b', 'replay', '2026-11-12T02:00:00Z', NULL, '2026-11-12T02:00:02Z')`)
	const boot = `"gateway_id":"gateway-a","boot_id":"replay"`
	post("lifecycle", "{"+boot+`,"exited_at":"2026-11-12T02:00:02Z"}`)
	var state v1.ListUsageGapsResponse
	read(gapsPath+"?kind=boot&status=pending_replay&page_size=1", &state)
	if len(state.GetGaps()) != 1 || state.GetGaps()[0].GetGatewayId() != "gateway-b" || state.GetNextPageToken() == "" {
		t.Fatalf("unexpected first pending boot page: %v", &state)
	}
	read(gapsPath+"?kind=boot&status=pending_replay&page_size=1&page_token="+url.QueryEscape(state.GetNextPageToken()), &state)
	if len(state.GetGaps()) != 1 || state.GetGaps()[0].GetGatewayId() != "gateway-a" ||
		state.GetGaps()[0].GetGapId() != "" || state.GetNextPageToken() != "" {
		t.Fatalf("unexpected second pending boot page: %v", &state)
	}
	read(gapsPath+"?gateway_id=gateway-a&tenant_id=alpha", &state)
	if len(state.GetGaps()) != 0 {
		t.Fatalf("unattributed pending boot matched a tenant: %v", &state)
	}

	post("boots/replay/unfinished-sessions", "{"+boot+`,"sessions":[
		{"session_id":"session-a","client_id":"client-a","last_recorded_at":"2026-11-12T02:00:00Z"},
		{"session_id":"session-b","client_id":"client-b","last_recorded_at":"2026-11-12T02:00:01Z"}]}`)
	const sessionPath = gapsPath + "?gateway_id=gateway-a&boot_id=replay&tenant_id=alpha&kind=session"
	read(sessionPath+"&status=pending_replay", &state)
	if len(state.GetGaps()) != 1 || state.GetGaps()[0].GetSessionId() != "session-a" || state.GetGaps()[0].GetStatus() != "pending_replay" {
		t.Fatalf("expected provisional session: %v", &state)
	}
	gapID := state.GetGaps()[0].GetGapId()
	read(sessionPath+"&status=possible_gap", &state)
	if len(state.GetGaps()) != 0 {
		t.Fatalf("session reported as possible gap before replay completed: %v", &state)
	}

	post("boots/replay/replay-complete", "{"+boot+`,"unfinished_session_count":2}`)
	read(sessionPath+"&status=pending_replay", &state)
	if len(state.GetGaps()) != 0 {
		t.Fatalf("completed session still pending: %v", &state)
	}
	read(sessionPath+"&status=possible_gap", &state)
	if len(state.GetGaps()) != 1 {
		t.Fatalf("expected confirmed replay result: %v", &state)
	}
	gap := state.GetGaps()[0]
	start := time.Date(2026, 11, 12, 2, 0, 0, 0, time.UTC)
	if gap.GetGapId() != gapID || gapID == "" || gap.GetStatus() != "possible_gap" || gap.GetTenantId() != "alpha" ||
		!gap.GetFromAt().AsTime().Equal(start) || !gap.GetExitedAt().AsTime().Equal(start.Add(2*time.Second)) {
		t.Fatalf("unexpected gap: %v", gap)
	}

	const emptyBoot = `"gateway_id":"gateway-a","boot_id":"empty"`
	post("lifecycle", "{"+emptyBoot+`,"exited_at":"2026-11-12T02:00:02Z"}`)
	post("boots/empty/replay-complete", "{"+emptyBoot+`,"has_session_records":false}`)
	read(gapsPath+"?gateway_id=gateway-a&boot_id=empty&kind=boot&status=possible_gap", &state)
	if len(state.GetGaps()) != 1 || state.GetGaps()[0].GetGapId() == "" || state.GetGaps()[0].GetSessionId() != "" {
		t.Fatalf("expected boot-level possible gap: %v", &state)
	}

	const cleanBoot = `"gateway_id":"gateway-a","boot_id":"clean"`
	post("lifecycle", "{"+cleanBoot+`,"exited_at":"2026-11-12T02:00:02Z"}`)
	post("boots/clean/replay-complete", "{"+cleanBoot+`,"has_session_records":true}`)
	read(gapsPath+"?boot_id=clean", &state)
	if len(state.GetGaps()) != 0 {
		t.Fatalf("completed replay without unfinished sessions produced a gap: %v", &state)
	}

	for _, path := range []string{anomaliesPath, gapsPath} {
		for _, role := range []struct{ tenant, token string }{{"alpha", f.alice}, {"beta", f.bob}} {
			testfixture.AssertStatus(t, testfixture.Request(f.http, "GET", path+"?tenant_id="+role.tenant, "", role.token), 403, "ACCESS_DENIED")
		}
		testfixture.AssertStatus(t, testfixture.Request(f.http, "GET", path, "", ""), 401, "")
	}
}
