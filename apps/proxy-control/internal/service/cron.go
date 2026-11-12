package service

import (
	"context"

	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/biz/usage"
)

type CronService struct {
	usage *usage.Service
}

func NewCronService(usage *usage.Service) *CronService {
	cronService := &CronService{usage: usage}
	return cronService
}

// InitService 初始化服务
func (cs *CronService) InitService() error {
	return nil
}

func (cs *CronService) MaintainUsage(ctx context.Context) error {
	return cs.usage.Maintain(ctx)
}
