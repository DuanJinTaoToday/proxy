package acl

import (
	"net/http"
	"time"

	"github.com/DuanJinTaoToday/proxy/apps/proxy-gateway/internal/biz/authrelay"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-gateway/internal/biz/recorder"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-gateway/internal/conf"
	"github.com/google/wire"
)

var ProviderSet = wire.NewSet(
	NewControlTransport,
	NewAuthControl,
	NewReporter,
	wire.Bind(new(authrelay.Control), new(*HTTPControl)),
	wire.Bind(new(recorder.Reporter), new(*HTTPReporter)),
	wire.Bind(new(recorder.LifecycleReporter), new(*HTTPReporter)),
)

type AuthConfig struct {
	Endpoint string
	Token    string
}

func NewAuthControl(cfg AuthConfig, transport *http.Transport) (*HTTPControl, error) {
	return NewHTTPControl(cfg.Endpoint, cfg.Token, transport)
}

type ReportConfig struct {
	Endpoint string
	Token    string
	Timeout  time.Duration
}

func NewReporter(cfg ReportConfig, transport *http.Transport) (*HTTPReporter, error) {
	return NewHTTPReporter(cfg.Endpoint, cfg.Token, cfg.Timeout, transport)
}

func NewControlTransport(cfg *conf.Data) (*http.Transport, error) {
	return NewTransport(cfg.GetAcl().GetControl().GetCaFile())
}
