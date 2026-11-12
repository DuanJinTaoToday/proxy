package data

import (
	"fmt"
	"log"
	"os"

	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/conf"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func OpenUsage(settings *conf.Usage) (*UsageRepository, func(), error) {
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
	return NewUsageRepository(db), func() {
		if err := pool.Close(); err != nil {
			log.Printf("close PostgreSQL pool: %v", err)
		}
	}, nil
}
