package server

import (
	"context"
	stdhttp "net/http"

	v1 "github.com/DuanJinTaoToday/proxy/api/proxycontrol/v1"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/service"
	"github.com/go-kratos/kratos/v3/transport/http"
)

func RegisterUsage(srv *http.Server, usage *service.Usage) {
	v1.RegisterReportingServiceHTTPServer(srv, usage)
	v1.RegisterUsageServiceHTTPServer(srv, usage)
	srv.Route("/").GET("/v1/tenant/usage/daily.csv", func(ctx http.Context) error {
		var request v1.DailyUsageRequest
		if err := ctx.BindQuery(&request); err != nil {
			return err
		}
		http.SetOperation(ctx, "/proxycontrol.v1.UsageService/ExportDailyUsage")
		handle := ctx.Middleware(func(ctx context.Context, _ any) (any, error) {
			return usage.ExportDailyUsage(ctx, &request)
		})
		data, err := handle(ctx, &request)
		if err != nil {
			return err
		}
		ctx.Response().Header().Set("Content-Disposition", `attachment; filename="daily-usage.csv"`)
		return ctx.Blob(stdhttp.StatusOK, "text/csv; charset=utf-8", data.([]byte))
	})
}
