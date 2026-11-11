package server

import (
	v1 "github.com/DuanJinTaoToday/proxy/api/proxycontrol/v1"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/access"
	"github.com/go-kratos/kratos/v3/transport/http"
)

func NewHTTPServer(addr string, accessService *access.Service) *http.Server {
	srv := http.NewServer(http.Address(addr))
	v1.RegisterHealthServiceHTTPServer(srv, Health{})
	v1.RegisterAccessServiceHTTPServer(srv, accessService)
	return srv
}
