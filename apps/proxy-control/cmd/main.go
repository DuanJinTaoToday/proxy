package main

import (
	"context"
	"flag"
	"log"

	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/biz/access"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/conf"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/server"
	httpservice "github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/service"
	"github.com/go-kratos/kratos/v3"
	"github.com/go-kratos/kratos/v3/config"
	"github.com/go-kratos/kratos/v3/config/file"
	kratoshttp "github.com/go-kratos/kratos/v3/transport/http"
)

func main() {
	path := flag.String("conf", "apps/proxy-control/configs/config.yaml", "path to the control configuration")
	flag.Parse()

	if err := run(*path); err != nil {
		log.Fatal(err)
	}
}

func run(path string) error {
	source := config.New(config.WithSource(file.NewSource(path)))
	defer func() {
		if err := source.Close(); err != nil {
			log.Printf("close configuration: %v", err)
		}
	}()
	if err := source.Load(); err != nil {
		return err
	}

	var settings conf.Bootstrap
	if err := source.Scan(&settings); err != nil {
		return err
	}

	accessService, err := access.New(settings.GetAccess())
	if err != nil {
		return err
	}
	usageSettings := settings.GetUsage()
	httpServer := server.NewHTTPServer(settings.GetHttpAddr(), httpservice.NewAccess(accessService),
		kratoshttp.RequestDecoder(server.ReportingDecoder(int64(usageSettings.GetMaxBatchBodyBytes()))),
		kratoshttp.ResponseEncoder(server.UsageResponseEncoder),
	)
	if authSettings := settings.GetProxyAuth(); authSettings != nil {
		authService, closeDB, err := openProxyAuth(authSettings, accessService)
		if err != nil {
			return err
		}
		defer closeDB()
		server.RegisterProxyAuth(httpServer, httpservice.NewProxyAuth(authService))
	}
	if usageSettings != nil {
		usageService, closeDB, err := openUsage(usageSettings, accessService)
		if err != nil {
			return err
		}
		defer closeDB()
		server.RegisterUsage(httpServer, httpservice.NewUsage(usageService))
		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan struct{})
		go func() {
			defer close(done)
			maintainUsage(ctx, usageService)
		}()
		defer func() {
			cancel()
			<-done
		}()
	}
	return kratos.New(kratos.Name("proxy-control"), kratos.Server(httpServer)).Run()
}
