package server

import (
	v1 "github.com/DuanJinTaoToday/proxy/api/proxycontrol/v1"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/service"
	"github.com/go-kratos/kratos/v3/transport/http"
)

func RegisterUsage(srv *http.Server, usage *service.Usage) {
	v1.RegisterReportingServiceHTTPServer(srv, usage)
	v1.RegisterUsageServiceHTTPServer(srv, usage)
}
