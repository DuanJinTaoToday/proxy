package main

import (
	"flag"
	"log/slog"
	"time"

	"github.com/DuanJinTaoToday/proxy/apps/proxy-gateway/internal/conf"
	plog "github.com/DuanJinTaoToday/proxy/pkg/log"
	"github.com/go-kratos/kratos/v3/config"
	"github.com/go-kratos/kratos/v3/config/file"
)

var (
	AppName   = "proxy-gateway"
	GitTag    = "dev"
	GitCommit string
	BuildTime string
	flagconf  string
)

func init() {
	flag.StringVar(&flagconf, "conf", "./configs", "configuration directory or init.yaml path")
}

func main() {
	flag.Parse()
	time.Local = time.FixedZone("CST", 8*60*60)
	c := config.New(config.WithSource(file.NewSource(flagconf)))
	defer func() { _ = c.Close() }()
	if err := c.Load(); err != nil {
		panic(err)
	}
	var settings conf.Config
	if err := c.Scan(&settings); err != nil {
		panic(err)
	}

	appConf := conf.NewAppConfig(settings.GetSource())
	defer func() { _ = appConf.Close() }()
	bootstrap := appConf.GetBootstrap()

	zap := settings.GetZap()
	logger, closeLogger, err := plog.NewLogger(AppName, plog.Options{
		File: zap.GetFile(), Console: zap == nil || zap.Console == nil || zap.GetConsole(),
		Level: zap.GetLevel(), MaxSize: int(zap.GetMaxSize()),
		MaxBackups: int(zap.GetMaxBackups()), MaxAge: int(zap.GetMaxAge()),
	})
	if err != nil {
		panic(err)
	}
	defer closeLogger()
	slog.SetDefault(logger)

	app, cleanup, err := wireApp(bootstrap.GetServer(), bootstrap.GetData(), bootstrap.GetBiz(), appConf, logger)
	if err != nil {
		panic(err)
	}
	defer cleanup()
	logger.Info("service initialized", "config", flagconf, "server_http", app.Address(),
		"version", GitTag, "commit", GitCommit, "build_time", BuildTime)
	if err := app.Run(); err != nil {
		panic(err)
	}
}
