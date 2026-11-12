package recorder_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/DuanJinTaoToday/proxy/apps/proxy-gateway/internal/biz/recorder"
	recorderdata "github.com/DuanJinTaoToday/proxy/apps/proxy-gateway/internal/data/repo"
)

type replayReporting struct {
	reporting
	unfinished map[string][]recorder.UnfinishedSession
	completed  map[string]uint32
	hasRecords map[string]bool
}

func (r *replayReporting) ReportUnfinishedSessions(_ context.Context, _, boot string, sessions []recorder.UnfinishedSession) error {
	r.unfinished[boot] = append(r.unfinished[boot], sessions...)
	return nil
}

func (r *replayReporting) CompleteReplay(_ context.Context, _, boot string, count uint32, hasRecords bool) error {
	r.completed[boot] = count
	r.hasRecords[boot] = hasRecords
	return nil
}

func TestCollectorReconcilesConfirmedPrefixesAndRetainsTruncatedFiles(t *testing.T) {
	root := t.TempDir()
	const records = `{"sessionID":"closed","clientID":"client","phase":"interim","time":"2026-11-12T02:00:00Z"}
{"sessionID":"active","clientID":"client","phase":"interim","time":"2026-11-12T02:00:00Z"}
{"sessionID":"closed","phase":"final","time":"2026-11-12T02:00:01Z"}
{"sessionID":"active","clientID":"client","phase":"interim","time":"2026-11-12T02:00:02Z"}
`
	for boot, content := range map[string]string{"closed": records, "running": records, "truncated": records + "partial", "invalid": "invalid\n"} {
		dir := filepath.Join(root, boot)
		if err := os.Mkdir(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "sessions.jsonl"), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	spool := recorderdata.NewSpool(root)
	for _, boot := range []string{"closed", "truncated", "invalid"} {
		if err := os.WriteFile(filepath.Join(root, boot, "exited_at"), []byte("2026-11-12T02:00:03Z"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := spool.ConfirmExit(boot); err != nil {
			t.Fatal(err)
		}
	}
	out := &replayReporting{
		unfinished: make(map[string][]recorder.UnfinishedSession),
		completed:  make(map[string]uint32),
		hasRecords: make(map[string]bool),
	}
	collector := recorder.New(recorder.Config{GatewayID: "gateway"}, spool, out)
	if err := collector.Scan(t.Context()); err == nil {
		t.Fatal("expected an error for the truncated boot")
	}
	if len(out.unfinished) != 2 || len(out.completed) != 3 || out.completed["closed"] != 1 ||
		out.completed["truncated"] != 1 || out.completed["invalid"] != 0 ||
		!out.hasRecords["closed"] || !out.hasRecords["truncated"] || out.hasRecords["invalid"] {
		t.Fatalf("unexpected reconciliation: %v, %v", out.unfinished, out.completed)
	}
	for _, boot := range []string{"closed", "truncated"} {
		sessions := out.unfinished[boot]
		if len(sessions) != 1 || sessions[0].SessionID != "active" || sessions[0].ClientID != "client" ||
			!sessions[0].LastRecordedAt.Equal(time.Date(2026, 11, 12, 2, 0, 2, 0, time.UTC)) {
			t.Fatalf("unexpected unfinished sessions for %s: %v", boot, sessions)
		}
	}
	// Age all source files. Only a durable replay acknowledgement starts retention.
	old := time.Now().Add(-15 * 24 * time.Hour)
	for _, boot := range []string{"closed", "running", "truncated"} {
		if err := os.Chtimes(filepath.Join(root, boot, "sessions.jsonl"), old, old); err != nil {
			t.Fatal(err)
		}
	}
	if err := collector.Scan(t.Context()); err != nil {
		t.Fatal(err)
	}
	if _, err := spool.SessionSize("closed"); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(filepath.Join(root, "closed", "replay.confirmed"), old, old); err != nil {
		t.Fatal(err)
	}
	if err := collector.Scan(t.Context()); err != nil {
		t.Fatal(err)
	}
	if _, err := spool.SessionSize("closed"); !os.IsNotExist(err) {
		t.Fatalf("expired reconciled file was not removed: %v", err)
	}
	for _, boot := range []string{"running", "truncated"} {
		if _, err := spool.SessionSize(boot); err != nil {
			t.Fatal(err)
		}
	}
}
