package acl

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	v1 "github.com/DuanJinTaoToday/proxy/api/proxycontrol/v1"
)

func TestReporterRequiresCommittedWatermarkForEachFile(t *testing.T) {
	for _, test := range []struct {
		name, watermarks string
		wantError        bool
	}{
		{"missing", `[]`, true},
		{"below batch end", `[{"file_id":"sessions","confirmed_offset":"9"}]`, true},
		{"exact offset", `[{"file_id":"sessions","confirmed_offset":"10"}]`, false},
		{"ahead on replay", `[{"file_id":"sessions","confirmed_offset":"15"}]`, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_, _ = fmt.Fprint(w, `{"billable":1,"watermarks":`+test.watermarks+`}`)
			}))
			defer server.Close()
			reporter := &HTTPReporter{endpoint: server.URL, client: server.Client()}
			batch := &v1.SubmitBatchRequest{Records: []*v1.RecorderLine{{FileId: "sessions", EndOffset: 10}}}
			if err := reporter.Submit(t.Context(), batch); (err != nil) != test.wantError {
				t.Fatalf("submit error=%v", err)
			}
		})
	}
}
