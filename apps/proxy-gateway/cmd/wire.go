//go:build wireinject

package main

import (
	"github.com/DuanJinTaoToday/proxy/apps/proxy-gateway/internal/biz/authrelay"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-gateway/internal/biz/recorder"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-gateway/internal/data/control"
	recorderdata "github.com/DuanJinTaoToday/proxy/apps/proxy-gateway/internal/data/recorder"
	"github.com/google/wire"
)

func wireAuthRelay(*control.HTTPControl) *authrelay.Relay {
	panic(wire.Build(
		wire.Bind(new(authrelay.Control), new(*control.HTTPControl)),
		newAuthRelay,
	))
}

func wireRecorder(recorder.Config, string, *control.HTTPReporter) recorderWorkers {
	panic(wire.Build(
		recorderdata.NewSpool,
		wire.Bind(new(recorder.Spool), new(*recorderdata.Spool)),
		wire.Bind(new(recorder.LifecycleSpool), new(*recorderdata.Spool)),
		wire.Bind(new(recorder.Reporter), new(*control.HTTPReporter)),
		wire.Bind(new(recorder.LifecycleReporter), new(*control.HTTPReporter)),
		recorder.New,
		recorder.NewLifecycle,
		wire.Struct(new(recorderWorkers), "*"),
	))
}
