package repo

import (
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/biz/proxyauth"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/biz/usage"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/data"
	"github.com/google/wire"
)

var ProviderSet = wire.NewSet(
	ProvideProxyAuthRepository,
	ProvideUsageRepository,
	wire.Bind(new(proxyauth.Repository), new(*ProxyAuthRepository)),
	wire.Bind(new(usage.Repository), new(*UsageRepository)),
)

func ProvideProxyAuthRepository(resources *data.Data) *ProxyAuthRepository {
	return NewProxyAuthRepository(resources.ProxyAuthDB())
}

func ProvideUsageRepository(resources *data.Data) *UsageRepository {
	return NewUsageRepository(resources.UsageDB())
}
