package main

import (
	"context"
	"log"
	"time"

	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/biz/usage"
)

func maintainUsage(ctx context.Context, service *usage.Service) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		now := time.Now()
		if err := service.Archive(ctx, now); err != nil && ctx.Err() == nil {
			log.Printf("archive usage: %v", err)
		}
		if err := service.SettleGrants(ctx, now); err != nil && ctx.Err() == nil {
			log.Printf("settle traffic grants: %v", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
