//go:build wireinject

package main

import (
	"log/slog"

	"github.com/DuanJinTaoToday/proxy/apps/proxy-gateway/internal/biz"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-gateway/internal/conf"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-gateway/internal/data/acl"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-gateway/internal/data/repo"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-gateway/internal/server"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-gateway/internal/service"
	"github.com/google/wire"
)

func wireApp(*conf.Server, *conf.Data, *conf.Biz, *conf.AppConfig, *slog.Logger) (*server.App, func(), error) {
	panic(wire.Build(server.ProviderSet, acl.ProviderSet, repo.ProviderSet, service.ProviderSet, biz.ProviderSet))
}
