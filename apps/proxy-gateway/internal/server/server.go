package server

import (
	"github.com/google/wire"
	"net/http"
	"net/netip"
)

var ProviderSet = wire.NewSet(
	NewGatewaySettings,
	wire.FieldsOf(new(GatewaySettings), "Address", "LocalToken", "Spool", "Auth", "Report", "Recorder"),
	NewGatewayListener,
	NewAuthHTTPServer,
	NewCronServer,
	NewApp,
)

func NewAuthHTTPServer(address netip.AddrPort, handler http.Handler) *http.Server {
	return NewHTTP(address.String(), handler)
}
