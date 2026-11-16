package service

import "github.com/google/wire"

var ProviderSet = wire.NewSet(NewAccess, NewSubaccounts, NewOverview, NewWallet, NewProduct, NewUsage, NewCronService)
