package main

import (
	"flag"
	"log"

	proxycontrolv1 "github.com/DuanJinTaoToday/proxy/api/proxycontrol/v1"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/conf"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/server"
	"github.com/go-kratos/kratos/v3"
	"github.com/go-kratos/kratos/v3/config"
	"github.com/go-kratos/kratos/v3/config/file"
	"github.com/go-kratos/kratos/v3/transport/http"
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

	httpServer := http.NewServer(http.Address(settings.GetHttpAddr()))
	proxycontrolv1.RegisterHealthServiceHTTPServer(httpServer, server.Health{})
	return kratos.New(kratos.Name("proxy-control"), kratos.Server(httpServer)).Run()
}
