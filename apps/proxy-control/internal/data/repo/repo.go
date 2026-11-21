package repo

import (
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/biz/product"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/biz/usage"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/biz/wallet"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/data"
	"github.com/google/wire"
)

var ProviderSet = wire.NewSet(
	ProvideAccounts,
	ProvideIPCountryLookup,
	ProvideSubaccounts,
	ProvideWalletRepository,
	ProvideProductRepository,
	wire.Bind(new(product.Repository), new(*ProductRepository)),
	wire.Bind(new(wallet.Repository), new(*WalletRepository)),
	ProvideUsageRepository,
	wire.Bind(new(usage.Repository), new(*UsageRepository)),
)

func ProvideUsageRepository(resources *data.Data) *UsageRepository {
	db := resources.UsageDB()
	if db == nil {
		db = resources.ProxyAuthDB()
	}
	return NewUsageRepository(db)
}
