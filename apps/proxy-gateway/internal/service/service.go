package service

import (
	"github.com/google/wire"
	"net/http"
)

var ProviderSet = wire.NewSet(NewAuthRelay, NewCronService, wire.Bind(new(http.Handler), new(*AuthRelay)))
