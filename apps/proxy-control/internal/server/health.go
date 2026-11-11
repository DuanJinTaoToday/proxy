package server

import (
	"context"

	proxycontrolv1 "github.com/DuanJinTaoToday/proxy/api/proxycontrol/v1"
)

// Health reports that the HTTP process can respond.
type Health struct{}

func (Health) Check(context.Context, *proxycontrolv1.CheckRequest) (*proxycontrolv1.CheckResponse, error) {
	return &proxycontrolv1.CheckResponse{}, nil
}
