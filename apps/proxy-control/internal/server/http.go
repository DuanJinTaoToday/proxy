package server

import (
	v1 "github.com/DuanJinTaoToday/proxy/api/proxycontrol/v1"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/access"
	"github.com/go-kratos/kratos/v3/transport/http"
)

func NewHTTPServer(addr string, accessService *access.Service, options ...http.ServerOption) *http.Server {
	srv := http.NewServer(append([]http.ServerOption{http.Address(addr)}, options...)...)
	v1.RegisterHealthServiceHTTPServer(srv, Health{})
	v1.RegisterAccessServiceHTTPServer(srv, accessService)
	return srv
}
