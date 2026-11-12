package data

import (
	"errors"
	"fmt"
	"log"

	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/conf"
	"github.com/DuanJinTaoToday/proxy/pkg/client/db"
	"github.com/google/wire"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var ProviderSet = wire.NewSet(NewData)

type Data struct {
	repo repoClients
}

type repoClients struct {
	proxyAuthDB *gorm.DB
	usageDB     *gorm.DB
}

func NewData(settings *conf.Data) (*Data, func(), error) {
	data := &Data{}
	var cleanups []func() error
	cleanup := func() {
		for i := len(cleanups) - 1; i >= 0; i-- {
			if err := cleanups[i](); err != nil {
				log.Printf("close PostgreSQL pool: %v", err)
			}
		}
	}
	repoSettings := settings.GetRepo()
	if authSettings := repoSettings.GetProxyAuth(); authSettings != nil {
		dsn := authSettings.GetDatabaseDsn()
		if dsn == "" {
			return nil, nil, errors.New("configure data.repo.proxy_auth.database_dsn")
		}
		// Authentication SQL includes verifier and token hashes.
		client, closeDB, err := db.NewPgsqlClient(db.WithSource(dsn), db.WithLogger(logger.Discard))
		if err != nil {
			return nil, nil, errors.New("could not connect to the proxy auth database")
		}
		cleanups = append(cleanups, closeDB)
		data.repo.proxyAuthDB = client
	}
	if usageSettings := repoSettings.GetUsage(); usageSettings != nil {
		dsn := usageSettings.GetDatabaseDsn()
		if dsn == "" {
			cleanup()
			return nil, nil, errors.New("configure data.repo.usage.database_dsn")
		}
		client, closeDB, err := db.NewPgsqlClient(db.WithSource(dsn), db.WithLogger(logger.Default))
		if err != nil {
			cleanup()
			return nil, nil, errors.New("could not connect to the control plane PostgreSQL database")
		}
		cleanups = append(cleanups, closeDB)
		if err := client.Exec("SELECT client_id, tenant_id FROM proxy_credentials LIMIT 0").Error; err != nil {
			cleanup()
			return nil, nil, fmt.Errorf("usage requires the proxy_credentials mapping from Issue #3: %w", err)
		}
		data.repo.usageDB = client
	}
	return data, cleanup, nil
}

func (d *Data) ProxyAuthDB() *gorm.DB { return d.repo.proxyAuthDB }

func (d *Data) UsageDB() *gorm.DB { return d.repo.usageDB }
