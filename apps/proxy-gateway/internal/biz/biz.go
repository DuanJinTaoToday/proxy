package biz

import (
	"github.com/DuanJinTaoToday/proxy/apps/proxy-gateway/internal/biz/authrelay"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-gateway/internal/biz/recorder"
	"github.com/google/wire"
)

var ProviderSet = wire.NewSet(NewAuthRelay, recorder.New, recorder.NewLifecycle)

func NewAuthRelay(control authrelay.Control, collector *recorder.Collector) *authrelay.Relay {
	return authrelay.NewWithReporting(control, collector.QuotaRenewal)
}
