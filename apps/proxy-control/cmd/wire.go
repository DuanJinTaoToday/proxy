//go:build wireinject

package main

import (
	"log/slog"

	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/biz"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/conf"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/data"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/data/repo"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/server"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/service"
	"github.com/go-kratos/kratos/v3"
	"github.com/google/wire"
)

func wireApp(*conf.Server, *conf.Data, *conf.Biz, *conf.AppConfig, *slog.Logger) (*kratos.App, func(), error) {
	panic(wire.Build(conf.NewModules, server.ProviderSet, data.ProviderSet, repo.ProviderSet, service.ProviderSet, biz.ProviderSet, newApp))
}
