package service

import "github.com/google/wire"

var ProviderSet = wire.NewSet(NewAccess, NewSubaccounts, NewWallet, NewProduct, NewUsage, NewCronService)
