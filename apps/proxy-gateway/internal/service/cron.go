package service

import (
	"context"
	"log/slog"

	"github.com/DuanJinTaoToday/proxy/apps/proxy-gateway/internal/biz/recorder"
)

type CronService struct {
	logger    *slog.Logger
	collector *recorder.Collector
}

func NewCronService(logger *slog.Logger, collector *recorder.Collector) *CronService {
	return &CronService{logger: logger, collector: collector}
}

// InitService 初始化服务。
func (cs *CronService) InitService() error {
	return nil
}

func (cs *CronService) PrintHello(ctx context.Context) error {
	cs.logger.InfoContext(ctx, "Hello from proxy-gateway")
	return nil
}

func (cs *CronService) ReportUsage(ctx context.Context) error {
	return cs.collector.Scan(ctx)
}
