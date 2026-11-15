package biz

import (
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/biz/access"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/biz/usage"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/biz/wallet"
	"github.com/google/wire"
)

var ProviderSet = wire.NewSet(
	access.NewPersistent,
	wallet.NewService,
	usage.NewConfigured,
	wire.Bind(new(usage.TenantAccess), new(*access.Service)),
)
