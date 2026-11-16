package server

import (
	v1 "github.com/DuanJinTaoToday/proxy/api/proxycontrol/v1"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/service"
	"github.com/go-kratos/kratos/v3/transport/http"
)

func NewHTTPServer(addr string, accessService *service.Access, options ...http.ServerOption) *http.Server {
	srv := http.NewServer(append([]http.ServerOption{http.Address(addr), http.RequestDecoder(DecodeProxyAuthRequest)}, options...)...)
	v1.RegisterHealthServiceHTTPServer(srv, service.Health{})
	v1.RegisterAccessServiceHTTPServer(srv, accessService)
	return srv
}

func RegisterSubaccounts(srv *http.Server, subaccounts *service.Subaccounts) {
	v1.RegisterSubaccountServiceHTTPServer(srv, subaccounts)
	v1.RegisterGatewayAuthServiceHTTPServer(srv, subaccounts)
	v1.RegisterStickySessionServiceHTTPServer(srv, subaccounts)
}

func RegisterProduct(srv *http.Server, product *service.Product) {
	v1.RegisterProductServiceHTTPServer(srv, product)
}

func RegisterWallet(srv *http.Server, wallet *service.Wallet) {
	v1.RegisterWalletServiceHTTPServer(srv, wallet)
}
