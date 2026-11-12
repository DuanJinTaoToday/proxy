package main

import (
	"flag"
	"log"

	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/access"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/conf"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/server"
	"github.com/go-kratos/kratos/v3"
	"github.com/go-kratos/kratos/v3/config"
	"github.com/go-kratos/kratos/v3/config/file"
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
	httpServer := server.NewHTTPServer(settings.GetHttpAddr(), accessService)
	if settings.GetProxyAuth() != nil {
		service, closeDB, err := openProxyAuth(settings.GetProxyAuth(), accessService)
		if err != nil {
			return err
		}
		defer closeDB()
		server.RegisterProxyAuth(httpServer, service)
	}
	return kratos.New(kratos.Name("proxy-control"), kratos.Server(httpServer)).Run()
}
