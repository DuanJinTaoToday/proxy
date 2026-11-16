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
	if repoSettings.GetProxyAuth() != nil && repoSettings.GetUsage() != nil && repoSettings.GetProxyAuth().GetDatabaseDsn() != repoSettings.GetUsage().GetDatabaseDsn() {
		return nil, nil, errors.New("proxy auth and usage must use the same control plane database DSN")
	}
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
		if err := client.Exec("SELECT usage_version FROM tenant_balances LIMIT 0").Error; err != nil {
			cleanup()
			return nil, nil, fmt.Errorf("proxy auth requires migration 00011: %w", err)
		}
		if err := checkTrafficAlerts(client); err != nil {
			cleanup()
			return nil, nil, err
		}
		data.repo.proxyAuthDB = client
	}
	if usageSettings := repoSettings.GetUsage(); usageSettings != nil {
		dsn := usageSettings.GetDatabaseDsn()
		if dsn == "" {
			cleanup()
			return nil, nil, errors.New("configure data.repo.usage.database_dsn")
		}
		client := data.repo.proxyAuthDB
		if client == nil {
			var closeDB func() error
			var err error
			client, closeDB, err = db.NewPgsqlClient(db.WithSource(dsn), db.WithLogger(logger.Default))
			if err != nil {
				cleanup()
				return nil, nil, errors.New("could not connect to the control plane PostgreSQL database")
			}
			cleanups = append(cleanups, closeDB)
			if err := checkTrafficAlerts(client); err != nil {
				cleanup()
				return nil, nil, err
			}
		}
		if err := client.Exec("SELECT gateway_id, boot_id, session_id FROM usage_gaps LIMIT 0").Error; err != nil {
			cleanup()
			return nil, nil, fmt.Errorf("usage requires the usage_gaps migration: %w", err)
		}
		if err := client.Exec(`SELECT m.client_id,d.client_id,g.remaining_bytes,b.usage_version
            FROM usage_minutely m CROSS JOIN usage_daily d CROSS JOIN traffic_grants g CROSS JOIN tenant_balances b LIMIT 0`).Error; err != nil {
			cleanup()
			return nil, nil, fmt.Errorf("usage requires migration 00011: %w", err)
		}
		data.repo.usageDB = client
	}
	return data, cleanup, nil
}

func checkTrafficAlerts(client *gorm.DB) error {
	if err := client.Exec(`SELECT b.low_balance_threshold_bytes, b.low_balance_cycle, n.resolved_at
		FROM tenant_balances b CROSS JOIN tenant_notifications n LIMIT 0`).Error; err != nil {
		return fmt.Errorf("traffic alerts require migration 00012: %w", err)
	}
	return nil
}

func (d *Data) ProxyAuthDB() *gorm.DB { return d.repo.proxyAuthDB }

func (d *Data) UsageDB() *gorm.DB { return d.repo.usageDB }
