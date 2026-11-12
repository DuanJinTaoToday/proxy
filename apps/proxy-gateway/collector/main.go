package main

import (
	"context"
	"encoding/base64"
	"errors"
	"flag"
	"log"
	"net/http"
	"net/netip"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/DuanJinTaoToday/proxy/apps/proxy-gateway/internal/authrelay"
)

func main() {
	addr := flag.String("auth-listen", "127.0.0.1:8091", "loopback address for the GOST authentication relay")
	config := flag.String("gost-config", "", "path to the rendered GOST config to validate")
	flag.Parse()
	if err := run(*addr, *config); err != nil {
		log.Fatal(err)
	}
}

func run(addr, config string) error {
	listen, err := netip.ParseAddrPort(addr)
	if err != nil || !listen.Addr().IsLoopback() || listen.Port() == 0 {
		return errors.New("auth-listen must use a numeric loopback address and fixed port")
	}
	local := os.Getenv("PROXY_GATEWAY_LOCAL_AUTH_TOKEN")
	remote := os.Getenv("PROXY_GATEWAY_AUTH_TOKEN")
	report := os.Getenv("PROXY_GATEWAY_REPORT_TOKEN")
	if !validToken(local) || !validToken(remote) || local == remote || local == report || remote == report {
		return errors.New("configure distinct base64url gateway auth tokens of at least 32 random bytes")
	}
	if err := validateGOSTConfig(config, "http://"+listen.String()+"/v1/gateway/auth", local); err != nil {
		return err
	}
	control, err := authrelay.NewHTTPControl(os.Getenv("PROXY_GATEWAY_CONTROL_URL"), remote)
	if err != nil {
		return err
	}
	relay := authrelay.New(control, local, nil)
	srv := &http.Server{
		Addr: listen.String(), Handler: relay, ReadHeaderTimeout: time.Second,
		ReadTimeout: time.Second, WriteTimeout: time.Second, IdleTimeout: 30 * time.Second,
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	go relay.RunRefresh(ctx)
	finished := make(chan error, 1)
	go func() { finished <- srv.ListenAndServe() }()
	select {
	case err := <-finished:
		return err
	case <-ctx.Done():
		shutdown, stop := context.WithTimeout(context.Background(), time.Second)
		defer stop()
		return srv.Shutdown(shutdown)
	}
}

func validToken(token string) bool {
	bytes, err := base64.RawURLEncoding.DecodeString(token)
	return err == nil && len(bytes) >= 32 && len(token) <= 256
}
