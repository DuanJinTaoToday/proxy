package repo

import (
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/biz/usage"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/data"
	"github.com/google/wire"
)

var ProviderSet = wire.NewSet(
	ProvideAccounts,
	ProvideUsageRepository,
	wire.Bind(new(usage.Repository), new(*UsageRepository)),
)

func ProvideUsageRepository(resources *data.Data) *UsageRepository {
	return NewUsageRepository(resources.UsageDB())
}
