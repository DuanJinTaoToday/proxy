package service

import (
	"context"

	proxycontrolv1 "github.com/DuanJinTaoToday/proxy/api/proxycontrol/v1"
)

// Health reports that the HTTP process can respond.
type Health struct{}

func (Health) Check(context.Context, *proxycontrolv1.BaseRequest) (*proxycontrolv1.BaseResp, error) {
	return &proxycontrolv1.BaseResp{}, nil
}
