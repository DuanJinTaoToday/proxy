package integration

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/integration/testfixture"
)

func TestReplayReconciliation(t *testing.T) {
	f := setupUsage(t)
	post := func(path, body string, status int, reason string) {
		t.Helper()
		req := httptest.NewRequest(http.MethodPost, "/v1/reporting/"+path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+reportToken)
		response := httptest.NewRecorder()
		f.http.ServeHTTP(response, req)
		testfixture.AssertStatus(t, response, status, reason)
	}
	const boot = `"gateway_id":"gateway-a","boot_id":"replay"`
	const session = `{"session_id":"interim-only","client_id":"client-a","last_recorded_at":"2026-11-12T02:00:00Z"}`
	const complete = "boots/replay/replay-complete"
	const unfinished = "boots/replay/unfinished-sessions"
	post(complete, "{"+boot+"}", 409, "BOOT_NOT_EXITED")
	post("lifecycle", "{"+boot+`,"exited_at":"2026-11-12T02:00:01Z"}`, 200, "")
	post(complete, "{"+boot+`,"unfinished_session_count":1}`, 409, "UNFINISHED_SESSIONS_MISMATCH")
	for range 2 {
		post(unfinished, "{"+boot+`,"sessions":[`+session+"]}", 200, "")
		post(complete, "{"+boot+`,"unfinished_session_count":1}`, 200, "")
	}
	post(complete, "{"+boot+"}", 409, "UNFINISHED_SESSIONS_MISMATCH")

	const invalidBoot = `"gateway_id":"gateway-a","boot_id":"invalid"`
	post("batches", "{"+invalidBoot+`,"records":[{"file_id":"sessions","end_offset":"8","line":"invalid"}]}`, 200, "")
	post("lifecycle", "{"+invalidBoot+`,"exited_at":"2026-11-12T02:00:01Z"}`, 200, "")
	post("boots/invalid/replay-complete", "{"+invalidBoot+`,"has_session_records":false}`, 200, "")
	var gaps int64
	if err := f.db.Table("usage_gaps").Where("boot_id = ? AND session_id IS NULL", "invalid").Count(&gaps).Error; err != nil || gaps != 1 {
		t.Fatalf("expected a boot-level gap despite quarantined input, count = %d, err = %v", gaps, err)
	}
}
