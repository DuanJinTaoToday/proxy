package main

import (
	"errors"
	"log"
	"os"

	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/access"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/conf"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/proxyauth"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func openProxyAuth(settings *conf.ProxyAuth, accessService *access.Service) (*proxyauth.Service, func(), error) {
	dsn := os.Getenv(settings.GetDatabaseDsnEnv())
	if dsn == "" {
		return nil, nil, errors.New("configure the proxy_auth database DSN environment variable")
	}
	// Authentication SQL includes verifier and token hashes.
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		return nil, nil, errors.New("could not connect to the proxy auth database")
	}
	pool, err := db.DB()
	if err != nil {
		return nil, nil, err
	}
	return proxyauth.New(db, accessService), func() {
		if err := pool.Close(); err != nil {
			log.Printf("close proxy auth database: %v", err)
		}
	}, nil
}
