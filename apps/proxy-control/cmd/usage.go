package main

import (
	"context"
	"log"
	"time"

	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/biz/usage"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/conf"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/data"
)

func openUsage(settings *conf.Usage, tenants usage.TenantAccess) (*usage.Service, func(), error) {
	repo, closeDB, err := data.OpenUsage(settings)
	if err != nil {
		return nil, nil, err
	}
	svc := usage.New(repo, tenants)
	svc.SetMaxRecords(int(settings.GetMaxBatchRecords()))
	return svc, closeDB, nil
}

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
