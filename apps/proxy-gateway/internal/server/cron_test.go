package server_test

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/DuanJinTaoToday/proxy/apps/proxy-gateway/internal/conf"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-gateway/internal/server"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-gateway/internal/service"
)

type cronOutput chan string

func (out cronOutput) Write(p []byte) (int, error) {
	select {
	case out <- string(p):
	default:
	}
	return len(p), nil
}

func TestPrintHelloCronReloadsWithoutRestart(t *testing.T) {
	config, err := os.ReadFile("../../configs/file/server.yaml")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "server.yaml")
	save := func(contents string) {
		t.Helper()
		if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	const defaultSpec = "0 * * * * *"
	initial := strings.Replace(string(config), defaultSpec, "@every 1h", 1)
	save(initial)
	app := conf.NewAppConfig(&conf.Config_Source{File: path})
	t.Cleanup(func() { _ = app.Close() })
	output := make(cronOutput, 8)
	logger := slog.New(slog.NewTextHandler(output, nil))
	scheduler, err := server.NewCronServer(app, service.NewCronService(logger))
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

	save(strings.Replace(string(config), defaultSpec, "*/1 * * * * *", 1))
	select {
	case line := <-output:
		if !strings.Contains(line, "Hello from proxy-gateway") {
			t.Fatalf("unexpected log: %s", line)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("PrintHello did not run with the updated cron expression")
	}
}
