package repo

import (
	"github.com/DuanJinTaoToday/proxy/apps/proxy-gateway/internal/biz/recorder"
	"github.com/google/wire"
)

var ProviderSet = wire.NewSet(
	NewConfiguredSpool,
	wire.Bind(new(recorder.Spool), new(*Spool)),
	wire.Bind(new(recorder.LifecycleSpool), new(*Spool)),
)

type SpoolPath string

func NewConfiguredSpool(path SpoolPath) *Spool {
	return NewSpool(string(path))
}
