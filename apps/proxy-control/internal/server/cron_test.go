package server_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/biz/usage"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/conf"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/server"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/service"
)

type cronUsageRepo struct {
	usage.Repository
	runs chan time.Time
}

func (r cronUsageRepo) Transact(context.Context, func(usage.Transaction) error) error { return nil }

func (r cronUsageRepo) DueTenants(ctx context.Context, _ time.Time) ([]string, error) {
	deadline, _ := ctx.Deadline()
	select {
	case r.runs <- deadline:
		return nil, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func TestCronServerReloadsFileWithoutRestart(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 12*time.Second)
	defer cancel()
	path := filepath.Join(t.TempDir(), "server.yaml")
	const base = "server:\n  http:\n    addr: 127.0.0.1:8000\n"
	const task = "  cron_tasks:\n    - name: MaintainUsage\n      spec: "
	save := func(contents string) {
		t.Helper()
		if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	save(base + task + `"@every 1h"` + "\n")
	app := conf.NewAppConfig(&conf.Config_Source{File: path})
	t.Cleanup(func() { _ = app.Close() })
	runs := make(chan time.Time, 8)
	cronService := service.NewCronService(usage.New(cronUsageRepo{runs: runs}, nil))
	scheduler, err := server.NewCronServer(app, cronService)
	if err != nil {
		t.Fatal(err)
	}
	if err := scheduler.Start(ctx); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := scheduler.Stop(ctx); err != nil {
			t.Error(err)
		}
	}()
	waitRun := func() time.Time {
		t.Helper()
		select {
		case deadline := <-runs:
			return deadline
		case <-ctx.Done():
			t.Fatal("scheduled task did not run")
			return time.Time{}
		}
	}
	save(base + task + `"*/1 * * * * *"` + "\n      timeout: 1s\n")
	if deadline := waitRun(); deadline.IsZero() || time.Until(deadline) > 2*time.Second {
		t.Fatalf("scheduled run did not use the updated timeout: %v", deadline)
	}
	save(base + "  cron_tasks: []\n")
	// Allow a run already selected before the removal, then require silence
	// for longer than the previous interval.
	select {
	case <-runs:
	case <-time.After(time.Second):
	}
	select {
	case <-runs:
		t.Fatal("removed task is still scheduled")
	case <-time.After(1200 * time.Millisecond):
	}
}

func TestCronServerRegistersFileConfiguration(t *testing.T) {
	app := conf.NewAppConfig(&conf.Config_Source{File: "../../configs/file/server.yaml"})
	t.Cleanup(func() { _ = app.Close() })
	bootstrap := app.GetBootstrap()
	settings := bootstrap.GetServer()
	tasks := settings.GetCronTasks()
	if len(tasks) != 1 || tasks[0].GetName() != "MaintainUsage" {
		t.Fatalf("unexpected cron tasks: %v", tasks)
	}
	cronService := service.NewCronService(nil)
	if _, err := server.NewCronServer(app, cronService); err != nil {
		t.Fatal(err)
	}
}
