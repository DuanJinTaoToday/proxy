package main

import (
	"context"
	"encoding/base64"
	"errors"
	"flag"
	"log"
	"net"
	"net/netip"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/DuanJinTaoToday/proxy/apps/proxy-gateway/internal/biz/recorder"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-gateway/internal/data/control"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-gateway/internal/data/gost"
	recorderdata "github.com/DuanJinTaoToday/proxy/apps/proxy-gateway/internal/data/recorder"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-gateway/internal/server"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-gateway/internal/service"
)

func main() {
	addr := flag.String("auth-listen", "127.0.0.1:8091", "loopback address for the GOST authentication relay")
	config := flag.String("gost-config", "", "path to the GOST YAML template")
	binary := flag.String("gost-binary", "", "path to GOST v3.3.3")
	spool := flag.String("spool", "", "Recorder spool directory")
	maxRecords := flag.Int("batch-records", 1000, "maximum Recorder records per request")
	maxBytes := flag.Int("batch-bytes", 1<<20, "maximum encoded Recorder request bytes")
	interval := flag.Duration("scan-interval", time.Second, "Recorder scan interval")
	timeout := flag.Duration("report-timeout", 3*time.Second, "Recorder HTTP timeout")
	flag.Parse()
	if err := run(*addr, *config, *binary, *spool, recorder.Config{GatewayID: os.Getenv("PROXY_GATEWAY_ID"),
		MaxRecords: *maxRecords, MaxBytes: *maxBytes, Interval: *interval}, *timeout); err != nil {
		log.Fatal(err)
	}
}

func run(addr, config, binary, spoolPath string, cfg recorder.Config, reportTimeout time.Duration) error {
	if spoolPath == "" || binary == "" || cfg.GatewayID == "" || len(cfg.GatewayID) > 128 ||
		cfg.MaxRecords < 1 || cfg.MaxBytes < 128 || cfg.MaxBytes > 16<<20 ||
		cfg.Interval <= 0 || reportTimeout <= 0 {
		return errors.New("configure GOST binary, Recorder spool, gateway ID and positive report limits")
	}
	spool, err := filepath.Abs(spoolPath)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(spool, 0o700); err != nil {
		return err
	}
	listen, err := netip.ParseAddrPort(addr)
	if err != nil || !listen.Addr().IsLoopback() || listen.Port() == 0 {
		return errors.New("auth-listen must use a numeric loopback address and fixed port")
	}
	local := os.Getenv("PROXY_GATEWAY_LOCAL_AUTH_TOKEN")
	remote := os.Getenv("PROXY_GATEWAY_AUTH_TOKEN")
	report := os.Getenv("PROXY_GATEWAY_REPORT_TOKEN")
	if !validToken(local) || !validToken(remote) || !validToken(report) || local == remote || local == report || remote == report {
		return errors.New("configure distinct base64url gateway auth tokens of at least 32 random bytes")
	}
	transport, err := control.NewTransport(os.Getenv("PROXY_GATEWAY_CONTROL_CA_FILE"))
	if err != nil {
		return err
	}
	authControl, err := control.NewHTTPControl(os.Getenv("PROXY_GATEWAY_CONTROL_URL"), remote, transport)
	if err != nil {
		return err
	}
	relay := wireAuthRelay(authControl)
	srv := server.NewHTTP(listen.String(), service.NewAuthRelay(relay, local))
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	listener, err := net.Listen("tcp", listen.String())
	if err != nil {
		return err
	}
	defer func() { _ = listener.Close() }()
	reporter, err := control.NewHTTPReporter(os.Getenv("PROXY_GATEWAY_CONTROL_URL"), report, reportTimeout, transport)
	if err != nil {
		return err
	}
	process, err := gost.Start(ctx, binary, config, spool, "http://"+listen.String()+"/v1/gateway/auth", local)
	if err != nil {
		return err
	}
	defer func() { cancel(); <-process.Exited }()
	go relay.RunRefresh(ctx)
	workers := wireRecorder(cfg, spool, reporter)
	go workers.collector.Run(ctx)
	go workers.lifecycle.RunLifecycle(ctx, cfg.GatewayID, process.Boot, process.Exited)
	go recorderdata.MonitorDisk(ctx, spool)
	finished := make(chan error, 1)
	go func() { finished <- srv.Serve(listener) }()
	select {
	case err := <-finished:
		return err
	case <-process.Exited:
		if ctx.Err() == nil {
			flush, stop := context.WithTimeout(context.Background(), reportTimeout)
			if err := workers.lifecycle.ReportBootExit(flush, cfg.GatewayID, process.Boot); err != nil {
				log.Printf("GOST exit report pending retry: %v", err)
			}
			stop()
			return errors.New("managed GOST process exited; restart the gateway to continue")
		}
	case <-ctx.Done():
	}
	shutdown, stop := context.WithTimeout(context.Background(), time.Second)
	defer stop()
	return srv.Shutdown(shutdown)
}

func validToken(token string) bool {
	bytes, err := base64.RawURLEncoding.DecodeString(token)
	return err == nil && len(bytes) >= 32 && len(token) <= 256
}
