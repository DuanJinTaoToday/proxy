package db

import (
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type Option func(*options)

type options struct {
	source      string
	maxConn     int
	maxIdleConn int
	maxLifeTime time.Duration
	logger      logger.Interface
	logLevel    logger.LogLevel
}

func WithSource(source string) Option {
	return func(o *options) {
		o.source = source
	}
}

func WithLogger(logger logger.Interface) Option {
	return func(o *options) {
		o.logger = logger
	}
}

func WithMaxConn(maxConn int) Option {
	return func(o *options) {
		if maxConn > 0 {
			o.maxConn = maxConn
		}
	}
}

func WithMaxIdleConn(maxIdleConn int) Option {
	return func(o *options) {
		if maxIdleConn > 0 {
			o.maxIdleConn = maxIdleConn
		}
	}
}

func WithMaxLifeTime(maxLifeTime time.Duration) Option {
	return func(o *options) {
		if maxLifeTime > 0 {
			o.maxLifeTime = maxLifeTime
		}
	}
}

func WithLogLevel(logLevel logger.LogLevel) Option {
	return func(o *options) {
		o.logLevel = logLevel
	}
}

func NewPgsqlClient(opts ...Option) (*gorm.DB, func() error, error) {
	settings := options{
		maxConn:     100,
		maxIdleConn: 10,
		maxLifeTime: 5 * time.Minute,
		logLevel:    logger.Silent,
	}
	for _, opt := range opts {
		opt(&settings)
	}
	if settings.logger == nil {
		settings.logger = logger.Default.LogMode(settings.logLevel)
	}

	// Initialization errors may include connection parameters. Let the caller
	// decide how to report them before enabling query logging.
	client, err := gorm.Open(postgres.Open(settings.source), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		return nil, nil, err
	}
	pool, err := client.DB()
	if err != nil {
		return nil, nil, err
	}
	pool.SetMaxOpenConns(settings.maxConn)
	pool.SetMaxIdleConns(settings.maxIdleConn)
	pool.SetConnMaxLifetime(settings.maxLifeTime)
	client.Logger = settings.logger
	return client, pool.Close, nil
}
