//go:build wireinject

package main

import (
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/biz/access"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/biz/proxyauth"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/biz/usage"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/conf"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/data"
	"github.com/google/wire"
)

func openProxyAuth(*conf.ProxyAuth, *access.Service) (*proxyauth.Service, func(), error) {
	panic(wire.Build(
		data.OpenProxyAuth,
		wire.Bind(new(proxyauth.Repository), new(*data.ProxyAuthRepository)),
		wire.Bind(new(proxyauth.TenantAccess), new(*access.Service)),
		proxyauth.New,
	))
}

func openUsage(*conf.Usage, *access.Service) (*usage.Service, func(), error) {
	panic(wire.Build(
		data.OpenUsage,
		wire.Bind(new(usage.Repository), new(*data.UsageRepository)),
		wire.Bind(new(usage.TenantAccess), new(*access.Service)),
		usage.New,
	))
}
