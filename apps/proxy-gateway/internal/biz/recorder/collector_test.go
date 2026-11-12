package recorder_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	v1 "github.com/DuanJinTaoToday/proxy/api/proxycontrol/v1"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-gateway/internal/biz/recorder"
	recorderdata "github.com/DuanJinTaoToday/proxy/apps/proxy-gateway/internal/data/repo"
	"google.golang.org/protobuf/proto"
)

func TestCollectorHonorsBatchLimits(t *testing.T) {
	for _, tc := range []struct {
		name       string
		maxRecords int
		maxBytes   int
	}{
		{"record count", 1, 1024},
		// The encoded request fits one fixture record at either offset, but not both.
		{"encoded byte count", 10, 160},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			dir := filepath.Join(root, "boot-a")
			if err := os.Mkdir(dir, 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "sessions.jsonl"), []byte("{}\n{}\n"), 0o600); err != nil {
				t.Fatal(err)
			}

			out := &reporting{}
			collector := recorder.New(recorder.Config{
				GatewayID: "gateway", MaxRecords: tc.maxRecords, MaxBytes: tc.maxBytes,
			}, recorderdata.NewSpool(root), out)
			for i := range 2 {
				if err := collector.Scan(t.Context()); err != nil {
					t.Fatal(err)
				}
				if len(out.batches) != i+1 {
					t.Fatalf("scan %d: got %d submissions", i+1, len(out.batches))
				}
				batch := out.batches[i]
				lines := batch.GetRecords()
				if batch.GetGatewayId() != "gateway" || batch.GetBootId() != "boot-a" ||
					len(lines) != 1 || lines[0].GetFileId() != "sessions" ||
					lines[0].GetFileOffset() != uint64(3*i) || lines[0].GetEndOffset() != uint64(3*(i+1)) {
					t.Fatalf("scan %d: unexpected batch %v", i+1, batch)
				}
				raw, err := json.Marshal(batch)
				if err != nil {
					t.Fatal(err)
				}
				if len(raw) > tc.maxBytes {
					t.Fatalf("batch size %d exceeds limit %d", len(raw), tc.maxBytes)
				}
			}
			if err := collector.Scan(t.Context()); err != nil {
				t.Fatal(err)
			}
			if len(out.batches) != 2 {
				t.Fatal("submitted an empty batch")
			}
		})
	}
}

type reporting struct {
	recorder.Reporter
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

type interruptedCheckpoint struct {
	*recorderdata.Spool
}

func (s interruptedCheckpoint) SaveOffset(string, uint64) error {
	return errors.New("collector stopped before saving the offset")
}

func TestCollectorReplaysAcknowledgedRecordsAfterInterruptedCheckpoint(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "boot")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "sessions.jsonl"), []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	out := &reporting{}
	spool := recorderdata.NewSpool(root)
	cfg := recorder.Config{GatewayID: "gateway"}
	if err := recorder.New(cfg, interruptedCheckpoint{spool}, out).Scan(t.Context()); err == nil {
		t.Fatal("expected checkpoint failure after successful submission")
	}
	if err := recorder.New(cfg, spool, out).Scan(t.Context()); err != nil {
		t.Fatal(err)
	}
	if len(out.batches) != 2 || !proto.Equal(out.batches[0], out.batches[1]) {
		t.Fatalf("restart did not replay the original batch: %v", out.batches)
	}
	offset, err := spool.Offset("boot")
	if err != nil || offset != 3 {
		t.Fatalf("confirmed offset = %d, err = %v", offset, err)
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
