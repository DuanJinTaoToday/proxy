package service

import (
	"context"
	"log/slog"
)

type CronService struct {
	logger *slog.Logger
}

func NewCronService(logger *slog.Logger) *CronService {
	return &CronService{logger: logger}
}

// InitService 初始化服务。
func (cs *CronService) InitService() error {
	return nil
}

func (cs *CronService) PrintHello(ctx context.Context) error {
	cs.logger.InfoContext(ctx, "Hello from proxy-gateway")
	return nil
}
