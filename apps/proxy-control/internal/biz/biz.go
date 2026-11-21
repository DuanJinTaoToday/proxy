package biz

import (
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/biz/access"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/biz/overview"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/biz/product"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/biz/subaccount"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/biz/usage"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/biz/wallet"
	"github.com/google/wire"
)

var ProviderSet = wire.NewSet(subaccount.NewService,
	access.NewPersistent,
	access.NewLocationService,
	wallet.NewService,
	overview.New,
	product.NewService,
	usage.NewConfigured,
	wire.Bind(new(usage.TenantAccess), new(*access.Service)),
)
