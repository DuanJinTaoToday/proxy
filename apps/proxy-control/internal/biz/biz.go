package biz

import (
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/biz/access"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/biz/usage"
	"github.com/google/wire"
)

var ProviderSet = wire.NewSet(
	access.NewPersistent,
	usage.NewConfigured,
	wire.Bind(new(usage.TenantAccess), new(*access.Service)),
)
