package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/conf"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/usage"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func openUsage(settings *conf.Usage, tenants usage.TenantAccess) (*usage.Service, func(), error) {
	dsn := os.Getenv(settings.GetDatabaseDsnEnv())
	if dsn == "" {
		return nil, nil, fmt.Errorf("set %s to the control plane PostgreSQL connection", settings.GetDatabaseDsnEnv())
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, nil, fmt.Errorf("could not connect to the control plane PostgreSQL database")
	}
	pool, err := db.DB()
	if err != nil {
		return nil, nil, err
	}
	if err := db.Exec("SELECT client_id, tenant_id FROM proxy_credentials LIMIT 0").Error; err != nil {
		if closeErr := pool.Close(); closeErr != nil {
			log.Printf("close PostgreSQL pool: %v", closeErr)
		}
		return nil, nil, fmt.Errorf("usage requires the proxy_credentials mapping from Issue #3: %w", err)
	}
	service := usage.New(db, tenants)
	service.SetMaxRecords(int(settings.GetMaxBatchRecords()))
	return service, func() {
		if err := pool.Close(); err != nil {
			log.Printf("close PostgreSQL pool: %v", err)
		}
	}, nil
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
