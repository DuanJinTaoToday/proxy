package server

import (
	"context"
	stdhttp "net/http"

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

func RegisterOverview(srv *http.Server, overview *service.Overview) {
	v1.RegisterOverviewServiceHTTPServer(srv, overview)
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
	srv.Route("/").POST("/v1/tenant/static-ips:export", func(ctx http.Context) error {
		var request v1.StaticIPExportRequest
		if err := ctx.Bind(&request); err != nil {
			return err
		}
		http.SetOperation(ctx, "/proxycontrol.v1.WalletService/ExportStaticIPs")
		handle := ctx.Middleware(func(ctx context.Context, _ any) (any, error) {
			return wallet.ExportStaticIPs(ctx, &request)
		})
		data, err := handle(ctx, &request)
		if err != nil {
			return err
		}
		ctx.Response().Header().Set("Content-Disposition", `attachment; filename="static-ips.txt"`)
		return ctx.Blob(stdhttp.StatusOK, "text/plain; charset=utf-8", data.([]byte))
	})
}
