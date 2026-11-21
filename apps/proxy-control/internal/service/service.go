package service

import "github.com/google/wire"

var ProviderSet = wire.NewSet(NewAccess, NewAccessLocation, NewSubaccounts, NewOverview, NewWallet, NewProduct, NewUsage, NewCronService)
