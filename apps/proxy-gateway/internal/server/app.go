package server

import (
	"context"
	"errors"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/DuanJinTaoToday/proxy/apps/proxy-gateway/internal/biz/authrelay"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-gateway/internal/biz/recorder"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-gateway/internal/data/gost"
	recorderdata "github.com/DuanJinTaoToday/proxy/apps/proxy-gateway/internal/data/repo"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-gateway/internal/data/wukong"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-gateway/internal/service"
)

type App struct {
	binary, template, spool, localToken string
	wukong                              gost.WukongConfig
	gatewayID                           string
	reportTimeout                       time.Duration
	relay                               *authrelay.Relay
	httpServer                          *http.Server
	collector                           *recorder.Collector
	lifecycle                           *recorder.Lifecycle
	listener                            net.Listener
	scheduler                           *cronServerImpl
}

func NewApp(settings GatewaySettings, relay *authrelay.Relay, httpServer *http.Server, listener net.Listener,
	collector *recorder.Collector, lifecycle *recorder.Lifecycle, scheduler *cronServerImpl,
) *App {
	return &App{
		binary: settings.binary, template: settings.template,
		spool: string(settings.Spool), localToken: settings.LocalToken,
		wukong: settings.wukong, gatewayID: settings.Recorder.GatewayID,
		reportTimeout: settings.Report.Timeout,
		relay:         relay, httpServer: httpServer, listener: listener,
		collector: collector, lifecycle: lifecycle, scheduler: scheduler,
	}
}

func (a *App) Address() string {
	return a.listener.Addr().String()
}

func (a *App) Run() error {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	if len(a.wukong.Entries) > 0 {
		pool, release, err := wukong.Open(a.spool, a.wukong.Username, a.wukong.Password, a.wukong.Entries,
			a.wukong.ProbeURL, a.wukong.BackupProbeURL, a.localToken, a.wukong.ProbeTimeout)
		if err != nil {
			return err
		}
		defer release()
		auth := service.NewAuthRelay(a.relay, a.localToken)
		auth.OnAuthenticated(pool.RememberClient)
		a.httpServer.Handler = http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			switch {
			case req.Method == http.MethodConnect:
				pool.Connect(ctx, w, req)
			case req.URL.Path == "/v1/gateway/route":
				pool.Route(w, req)
			default:
				auth.ServeHTTP(w, req)
			}
		})
		go pool.Run(ctx)
	}

	process, err := gost.Start(ctx, a.binary, a.template, a.spool, "http://"+a.listener.Addr().String()+"/v1/gateway/auth", a.localToken, a.wukong)
	if err != nil {
		return err
	}
	defer func() { cancel(); <-process.Exited }()
	if err := a.scheduler.Start(ctx); err != nil {
		return err
	}
	defer func() {
		if err := a.scheduler.Stop(context.WithoutCancel(ctx)); err != nil {
			log.Printf("stop cron server: %v", err)
		}
	}()
	go a.relay.RunRefresh(ctx)
	go a.collector.Run(ctx)
	go a.lifecycle.RunLifecycle(ctx, a.gatewayID, process.Boot, process.Exited)
	go recorderdata.MonitorDisk(ctx, a.spool)
	finished := make(chan error, 1)
	go func() { finished <- a.httpServer.Serve(a.listener) }()
	select {
	case err := <-finished:
		return err
	case <-process.Exited:
		if ctx.Err() == nil {
			flush, stop := context.WithTimeout(context.Background(), a.reportTimeout)
			if err := a.lifecycle.ReportBootExit(flush, a.gatewayID, process.Boot); err != nil {
				log.Printf("GOST exit report pending retry: %v", err)
			}
			stop()
			return errors.New("managed GOST process exited; restart the gateway to continue")
		}
	case <-ctx.Done():
	}
	shutdown, stop := context.WithTimeout(context.Background(), time.Second)
	defer stop()
	return a.httpServer.Shutdown(shutdown)
}
