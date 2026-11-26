package service

import (
	"context"

	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/biz/usage"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/biz/wallet"
)

type CronService struct {
	usage  *usage.Service
	wallet *wallet.Service
}

func NewCronService(usage *usage.Service, wallet *wallet.Service) *CronService {
	cronService := &CronService{usage: usage, wallet: wallet}
	return cronService
}

// InitService 初始化服务
func (cs *CronService) InitService() error {
	return nil
}

func (cs *CronService) MaintainUsage(ctx context.Context) error {
	return cs.usage.Maintain(ctx)
}

func (cs *CronService) ProbeStaticExchanges(ctx context.Context) error {
	return cs.wallet.ProbeStaticExchanges(ctx)
}
