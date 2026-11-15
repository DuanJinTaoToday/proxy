package service

import "github.com/google/wire"

var ProviderSet = wire.NewSet(NewAccess, NewWallet, NewProduct, NewUsage, NewCronService)
