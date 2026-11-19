package server_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	v1 "github.com/DuanJinTaoToday/proxy/api/proxycontrol/v1"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-gateway/internal/biz/recorder"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-gateway/internal/conf"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-gateway/internal/data/repo"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-gateway/internal/server"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-gateway/internal/service"
)

type cronReporter struct {
	recorder.Reporter
	batches chan *v1.SubmitBatchRequest
	release chan struct{}
}

func (r cronReporter) Submit(ctx context.Context, batch *v1.SubmitBatchRequest) error {
	select {
	case r.batches <- batch:
	case <-ctx.Done():
		return ctx.Err()
	}
	select {
	case <-r.release:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func TestReportUsageCronReloadsWithoutRestart(t *testing.T) {
	root := t.TempDir()
	boot := filepath.Join(root, "boot-a")
	if err := os.Mkdir(boot, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(boot, "sessions.jsonl"), []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	spool := repo.NewSpool(root)
	reporter := cronReporter{batches: make(chan *v1.SubmitBatchRequest, 1), release: make(chan struct{})}
	collector := recorder.New(recorder.Config{GatewayID: "gateway"}, spool, reporter)

	path := filepath.Join(t.TempDir(), "server.yaml")
	save := func(spec string) {
		t.Helper()
		contents := fmt.Sprintf(`server:
  http:
    addr: 127.0.0.1:8091
  cron_tasks:
    - name: ReportUsage
      spec: %q
`, spec)
		if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	save("@every 1h")
	app := conf.NewAppConfig(&conf.Config_Source{File: path})
	t.Cleanup(func() { _ = app.Close() })
	scheduler, err := server.NewCronServer(app, service.NewCronService(nil, collector))
	if err != nil {
		t.Fatal(err)
	}
	if err := scheduler.Start(t.Context()); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := scheduler.Stop(t.Context()); err != nil {
			t.Error(err)
		}
	}()

	save("*/1 * * * * *")
	select {
	case batch := <-reporter.batches:
		if records := batch.GetRecords(); batch.GetBootId() != "boot-a" || len(records) != 1 || records[0].GetEndOffset() != 3 {
			t.Fatalf("unexpected Recorder batch: %v", batch)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("ReportUsage did not run with the updated cron expression")
	}

	select {
	case <-reporter.batches:
		t.Fatal("cron submitted another batch while the previous report was still running")
	case <-time.After(1200 * time.Millisecond):
	}
	close(reporter.release)
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		offset, err := spool.Offset("boot-a")
		if err != nil {
			t.Fatal(err)
		}
		if offset == 3 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("Recorder offset did not advance after acknowledgement")
}
