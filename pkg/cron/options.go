package cron

import (
	"context"
	"time"

	"log/slog"
)

type ServerOptions func(o *Server) error

// WithContext 设置上下文
func WithContext(ctx context.Context) ServerOptions {
	return func(s *Server) error {
		s.baseCtx = ctx
		return nil
	}
}

// WithLocker 设置任务互斥锁
func WithLocker(opt Locker, keyPrefix string) ServerOptions {
	return func(s *Server) error {
		s.locker = opt
		s.keyPrefix = keyPrefix
		return nil
	}
}

// Logger 设置日志 暂时没用
func Logger(logger *slog.Logger) ServerOptions {
	return func(s *Server) error {
		s.log = logger
		return nil
	}
}

// WithStopTimeout 设置停止服务超时时间
// Default 10s
func WithStopTimeout(timeout time.Duration) ServerOptions {
	return func(s *Server) error {
		s.stopTimeout = timeout
		return nil
	}
}

type RunOptions func(o *Task) error

// WithTaskName 设置任务名称
// Default {funcName}
func WithTaskName(name string) RunOptions {
	return func(o *Task) error {
		o.name = name
		return nil
	}
}

// WithTaskFunc 设置任务执行函数
// Must be set
func WithTaskFunc(fun funcType) RunOptions {
	return func(o *Task) error {
		o.tFunc = fun
		return nil
	}
}

// WithRunSpec 设置任务执行时间规则
// Must be set
func WithRunSpec(spec string) RunOptions {
	return func(o *Task) error {
		o.spec = spec
		return nil
	}
}

// WithTimeout 设置任务执行超时时间(单位秒),用于redisLock
// Default 30s
func WithTimeout(timeout time.Duration) RunOptions {
	return func(o *Task) error {
		o.timeout = timeout
		return nil
	}
}

type Locker interface {
	Lock(ctx context.Context, key string, timeout time.Duration) (unlock func(context.Context) error, err error)
}
