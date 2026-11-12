package recorder_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	v1 "github.com/DuanJinTaoToday/proxy/api/proxycontrol/v1"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-gateway/internal/biz/recorder"
	recorderdata "github.com/DuanJinTaoToday/proxy/apps/proxy-gateway/internal/data/repo"
	"google.golang.org/protobuf/proto"
)

type reporting struct {
	batches []*v1.SubmitBatchRequest
	fail    bool
	max     int
}

func (r *reporting) Submit(_ context.Context, batch *v1.SubmitBatchRequest) error {
	r.batches = append(r.batches, proto.CloneOf(batch))
	if r.max > 0 && len(batch.GetRecords()) > r.max {
		return recorder.ErrBatchTooLarge
	}
	if r.fail {
		return errors.New("ack lost")
	}
	return nil
}

func TestCollectorResumesFromDurableAcknowledgement(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "boot-a")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "sessions.jsonl")
	if err := os.WriteFile(path, []byte("甲\r\n{}\npartial"), 0o600); err != nil {
		t.Fatal(err)
	}
	out := &reporting{fail: true}
	cfg := recorder.Config{GatewayID: "gateway"}
	collector := recorder.New(cfg, recorderdata.NewSpool(root), out)
	if err := collector.Scan(t.Context()); err == nil {
		t.Fatal("expected lost acknowledgement")
	}
	out.fail = false
	if err := recorder.New(cfg, recorderdata.NewSpool(root), out).Scan(t.Context()); err != nil {
		t.Fatal(err)
	}
	if len(out.batches) != 2 {
		t.Fatalf("want a replay, got %d batches", len(out.batches))
	}
	for _, batch := range out.batches {
		lines := batch.GetRecords()
		if batch.GetBootId() != "boot-a" || len(lines) != 2 || lines[0].GetFileId() != "sessions" ||
			lines[0].GetFileOffset() != 0 || lines[0].GetEndOffset() != 5 || lines[0].GetLine() != "甲" ||
			lines[1].GetFileOffset() != 5 || lines[1].GetEndOffset() != 8 {
			t.Fatalf("invalid complete records: %v", batch)
		}
	}
	if err := recorder.New(cfg, recorderdata.NewSpool(root), out).Scan(t.Context()); err != nil {
		t.Fatal(err)
	}
	if len(out.batches) != 2 {
		t.Fatal("resubmitted acknowledged lines or partial tail")
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString("\n"); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	if err := recorder.New(cfg, recorderdata.NewSpool(root), out).Scan(t.Context()); err != nil {
		t.Fatal(err)
	}
	if len(out.batches) != 3 || out.batches[2].GetRecords()[0].GetFileOffset() != 8 {
		t.Fatal("restart did not resume exactly at the persisted offset")
	}
}

func TestCollectorSavesGoodPrefixAndContinuesOtherBoots(t *testing.T) {
	root := t.TempDir()
	for boot, data := range map[string][]byte{"boot-a": {'a', '\n', 'b', '\n', 255, '\n'}, "boot-b": []byte("{}\n")} {
		dir := filepath.Join(root, boot)
		if err := os.Mkdir(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "sessions.jsonl"), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	out := &reporting{max: 1}
	collector := recorder.New(recorder.Config{GatewayID: "gateway"}, recorderdata.NewSpool(root), out)
	if err := collector.Scan(t.Context()); err != nil {
		t.Fatal(err)
	}
	if err := collector.Scan(t.Context()); err == nil {
		t.Fatal("expected invalid UTF-8 to be reported")
	}
	if err := collector.Scan(t.Context()); err != nil {
		t.Fatal(err)
	}
	var offsets []uint64
	for _, batch := range out.batches {
		if batch.GetBootId() == "boot-a" && len(batch.GetRecords()) == 1 {
			offsets = append(offsets, batch.GetRecords()[0].GetEndOffset())
		}
	}
	if len(offsets) != 2 || offsets[0] != 2 || offsets[1] != 4 || len(out.batches) != 4 {
		t.Fatalf("valid prefixes and other boot were not independently acknowledged: %v", out.batches)
	}
}
