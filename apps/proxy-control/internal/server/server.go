package server

import (
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/conf"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/service"
	"github.com/go-kratos/kratos/v3/transport"
	kratoshttp "github.com/go-kratos/kratos/v3/transport/http"
	"github.com/google/wire"
)

var ProviderSet = wire.NewSet(NewConfiguredHTTPServer, NewCronServer, NewServerList)

func NewServerList(modules conf.Modules, httpServer *kratoshttp.Server,
	usageHandler *service.Usage, scheduler *cronServerImpl,
) []transport.Server {
	servers := []transport.Server{httpServer}
	if modules.Usage {
		RegisterUsage(httpServer, usageHandler)
		servers = append(servers, scheduler)
	}
	return servers
}

func NewConfiguredHTTPServer(server *conf.Server, biz *conf.Biz, access *service.Access, wallet *service.Wallet, product *service.Product, subaccounts *service.Subaccounts) *kratoshttp.Server {
	srv := NewHTTPServer(server.GetHttp().GetAddr(), access,
		kratoshttp.RequestDecoder(ReportingDecoder(int64(biz.GetUsage().GetMaxBatchBodyBytes()))),
		kratoshttp.ResponseEncoder(ControlResponseEncoder),
	)
	RegisterWallet(srv, wallet)
	RegisterProduct(srv, product)
	RegisterSubaccounts(srv, subaccounts)
	return srv
}
