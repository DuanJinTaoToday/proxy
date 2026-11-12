package main

import (
	"github.com/DuanJinTaoToday/proxy/apps/proxy-gateway/internal/biz/authrelay"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-gateway/internal/biz/recorder"
)

func newAuthRelay(control authrelay.Control) *authrelay.Relay {
	return authrelay.New(control, nil)
}

type recorderWorkers struct {
	collector *recorder.Collector
	lifecycle *recorder.Lifecycle
}
