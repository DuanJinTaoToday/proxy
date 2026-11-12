package conf

import (
	"log/slog"
	"sync"

	"github.com/go-kratos/kratos/contrib/config/apollo/v3"
	"github.com/go-kratos/kratos/v3/config"
	"github.com/go-kratos/kratos/v3/config/file"
)

type AppConfig struct {
	conf      config.Config
	mu        sync.RWMutex
	listeners map[string][]Listener
}

func NewAppConfig(sourceCfg *Config_Source) *AppConfig {
	remote := sourceCfg.GetApollo()
	if (sourceCfg.GetFile() != "") == (remote != nil) {
		panic("configure exactly one of source.file and source.apollo")
	}

	var source config.Source
	if remote != nil {
		options := []apollo.Option{
			apollo.WithAppID(remote.GetAppId()),
			apollo.WithCluster(remote.GetCluster()),
			apollo.WithEndpoint(remote.GetEndpoint()),
			apollo.WithNamespace(remote.GetNamespace()),
			apollo.WithOriginalConfig(),
			apollo.WithSecret(remote.GetSecret()),
		}
		if remote.GetEnableBackup() {
			if remote.GetBackupPath() == "" {
				panic("configure source.apollo.backup_path when backup is enabled")
			}
			options = append(options, apollo.WithEnableBackup(), apollo.WithBackupPath(remote.GetBackupPath()))
		}
		source = apollo.NewSource(options...)
	} else {
		source = file.NewSource(sourceCfg.GetFile())
	}

	// Preserve literal values such as credentials containing ${...}.
	cfg := config.New(config.WithSource(source), config.WithResolver(func(map[string]any) error { return nil }))
	if err := cfg.Load(); err != nil {
		_ = cfg.Close()
		panic(err)
	}
	return &AppConfig{conf: cfg, listeners: map[string][]Listener{}}
}

func (app *AppConfig) GetBootstrap() *Bootstrap {
	var bs Bootstrap
	if err := app.conf.Scan(&bs); err != nil {
		panic(err)
	}
	if bs.GetServer().GetHttp() == nil {
		panic("configure server.http in the application configuration")
	}
	return &bs
}

func (app *AppConfig) handlerDataChange(key string, value config.Value) {
	app.mu.RLock()
	listeners := app.listeners[key]
	app.mu.RUnlock()
	for _, listener := range listeners {
		listener.Notify(value)
	}
}

func (app *AppConfig) Register(key string, listener Listener) {
	if listener == nil || key == "" {
		return
	}
	app.mu.Lock()
	defer app.mu.Unlock()
	app.listeners[key] = append(app.listeners[key], listener)
	if err := app.conf.Watch(key, app.handlerDataChange); err != nil {
		slog.Error("watch configuration", "key", key, "error", err)
	}
}

func (app *AppConfig) Close() error {
	return app.conf.Close()
}

type Listener interface {
	Notify(config.Value)
}
