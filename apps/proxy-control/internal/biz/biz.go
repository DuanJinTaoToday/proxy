package biz

import (
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/biz/access"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/biz/proxyauth"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/biz/usage"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/conf"
	"github.com/google/wire"
)

func NewAccess(settings *conf.Biz) (*access.Service, error) {
	return access.New(settings.GetAccess())
}

var ProviderSet = wire.NewSet(
	NewAccess,
	proxyauth.New,
	usage.NewConfigured,
	wire.Bind(new(proxyauth.TenantAccess), new(*access.Service)),
	wire.Bind(new(usage.TenantAccess), new(*access.Service)),
)
