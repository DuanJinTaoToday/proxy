package cron

import (
	"context"
	"fmt"
	"log/slog"
	"reflect"
	"runtime"
	"runtime/debug"
	"time"

	"github.com/go-kratos/kratos/v3/transport"
	"github.com/pkg/errors"
	"github.com/robfig/cron/v3"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

var (
	// "归一化"写法:
	// Server结构体需要实现 transport.Server 这个interface对应的方法
	_ transport.Server = (*Server)(nil)
)

// Server 定时任务服务
type Server struct {
	cron       *cron.Cron
	mapEntryId map[string]cron.EntryID

	locker    Locker
	keyPrefix string

	baseCtx     context.Context
	started     bool
	err         error
	stopTimeout time.Duration // 停止服务超时时间 默认10秒 可配置

	log *slog.Logger
}

type funcType func(context.Context) error

// Task 定时任务主体
type Task struct {
	name    string        // 任务名称
	entryID cron.EntryID  // 任务ID
	tFunc   funcType      // 任务执行函数
	spec    string        // 任务执行时间规则
	timeout time.Duration // 任务执行超时时间
	err     error         // 任务执行错误
	errMsg  string        // 任务执行错误信息
}

func NewServer(opts ...ServerOptions) *Server {
	srv := &Server{
		cron:        cron.New(cron.WithSeconds()),
		baseCtx:     context.Background(),
		mapEntryId:  make(map[string]cron.EntryID),
		started:     false,
		stopTimeout: 10 * time.Second,
		log:         slog.Default(),
	}
	for _, o := range opts {
		if err := o(srv); err != nil {
			srv.err = err
			break
		}
	}
	return srv
}

func (s *Server) Name() string {
	return "CronTaskServer"
}

func (s *Server) GetMapEntryId() map[string]cron.EntryID {
	return s.mapEntryId
}

// AddCronTask is a method of the Server struct that adds a new cron task.
// It accepts a variadic parameter of RunOptions, which are used to configure the task.
// The method returns the ID of the created task and an error if any occurred during the task creation.
//
// The method works as follows:
// 1. It creates a new task using the provided options.
// 2. If an error occurred during the task creation, it returns the error.
// 3. It adds the created task to the cron scheduler.
// 4. If an error occurred while adding the task to the scheduler, it returns the error.
// 5. It logs the created task.
// 6. Finally, it returns the ID of the created task and nil as the error.
//
// Usage:
// entryID, err := server.AddCronTask(
//
//	WithTaskName("MyTask"),
//	WithTaskFunc(myFunc),
//	WithRunSpec("* * * * * ?"),
//	WithTimeout(time.Second * 30),
//
// )
//
//	if err != nil {
//	    log.Fatalf("Failed to add cron task: %v", err)
//	}
func (s *Server) AddCronTask(opts ...RunOptions) (cron.EntryID, error) {
	task := s.createTask(opts...)
	if task.err != nil {
		return 0, task.err
	}

	entryID, err := s.addTaskToCron(task)
	if err != nil {
		return 0, err
	}

	s.log.InfoContext(s.baseCtx, "[cron-task] AddCronTask", "task", task.name, "spec", task.spec)
	return entryID, nil
}

func (s *Server) createTask(opts ...RunOptions) *Task {
	task := &Task{timeout: 0}
	for _, o := range opts {
		if err := o(task); err != nil {
			task.err = err
			return task
		}
	}
	if task.tFunc == nil {
		task.err = errors.WithStack(ErrTaskFuncIsNil)
		return task
	}
	if task.spec == "" {
		task.err = errors.WithStack(ErrTaskSpecIsNil)
		return task
	}
	if task.name == "" {
		task.name = runtime.FuncForPC(reflect.ValueOf(task.tFunc).Pointer()).Name()
	}
	return task
}

func (s *Server) addTaskToCron(task *Task) (cron.EntryID, error) {
	entryID, err := s.cron.AddFunc(task.spec, s.wrapFunc(task))
	if err != nil {
		return 0, err
	}
	task.entryID = entryID
	s.mapEntryId[task.name] = entryID
	return entryID, nil
}

// wrapFunc is a method of the Server struct that wraps a Task's function with additional functionality.
// It accepts a Task as a parameter and returns a function.
//
// The returned function does the following when called:
//  1. It records the start time of the task.
//  2. It defers a function that does the following:
//     a. It recovers from any panic that might occur during the task execution and logs the panic.
//     b. It calculates the elapsed time since the start of the task.
//     c. If an error occurred during the task execution, it logs the error and resets the task's error.
//     d. If no error occurred, it logs the successful completion of the task.
//  3. It attempts to acquire a Redis lock to prevent concurrent execution of the task.
//     a. If it fails to acquire the lock, it logs the failure and returns.
//     b. If it succeeds in acquiring the lock, it defers a function that releases the lock.
//  4. It sets a timeout for the task execution if a timeout is specified in the task.
//  5. It executes the task's function.
//     a. If an error occurs during the execution, it sets the error on the task.
func (s *Server) wrapFunc(task *Task) func() {
	return func() {
		// 创建 tracer
		tracer := otel.Tracer("cron.task")
		// 记录任务耗时
		start := time.Now()
		// 创建 span，使用任务名称作为 span 名称
		ctx, span := tracer.Start(s.baseCtx, task.name,
			trace.WithAttributes(
				attribute.String("task.name", task.name),
				attribute.String("task.spec", task.spec),
			))

		defer func() {
			// 捕获panic
			if r := recover(); r != nil {
				task.err = errors.Errorf("Recover from panic --> %+v", r)
				task.errMsg = fmt.Sprintf("panic stack -->\n%s", string(debug.Stack()))
			}

			elapsed := time.Since(start)
			elapsedInSec := float64(elapsed) / float64(time.Second)

			// 记录任务执行时间
			span.SetAttributes(attribute.Float64("task.elapsed_sec", elapsedInSec))

			if task.err != nil {
				// 记录错误到 span
				span.RecordError(task.err)
				span.SetStatus(codes.Error, task.errMsg)

				s.log.ErrorContext(ctx, "[cron-task] Task Failed", "task", task.name, "error", task.err, "detail", task.errMsg, "elapsed_sec", elapsedInSec)
				task.err = nil
				task.errMsg = ""
			} else {
				// 成功完成
				span.SetStatus(codes.Ok, "Task completed successfully")
				//log.Debugf(ctx, "[cron-task] Task Finshed: %s [cost]: %.4fs", task.name, elapsedInSec)
			}

			// 结束 span
			span.End()
		}()

		// 使用redis锁防止并发执行任务
		if s.locker != nil && len(s.keyPrefix) > 0 && task.timeout > 0 {
			redisKey := fmt.Sprintf("%s_%s", s.keyPrefix, task.name)
			unlock, err := s.locker.Lock(ctx, redisKey, task.timeout+200*time.Millisecond)
			if err != nil {
				// log.Warnf(s.baseCtx, "[cron-task] %s lock fail: %v", task.name, err)
				span.AddEvent("redis.lock.fail", trace.WithAttributes(
					attribute.String("error", err.Error()),
				))
				return
			} else {
				span.AddEvent("redis.lock.success")
				defer func() {
					err := unlock(ctx)
					if err != nil {
						task.err = err
						task.errMsg = fmt.Sprintf("unlock fail: %v", err)
						span.AddEvent("redis.unlock.fail", trace.WithAttributes(
							attribute.String("error", err.Error()),
						))
					} else {
						span.AddEvent("redis.unlock.success")
					}
				}()
			}
		}

		// 设置超时时间
		ctxInFunc := ctx
		if task.timeout > 0 {
			ctxInFuncWithTimeOut, cancel := context.WithTimeout(ctx, task.timeout)
			defer func() {
				cancel()
			}()
			ctxInFunc = ctxInFuncWithTimeOut
			go func() {
				<-ctxInFunc.Done()
				if errors.Is(ctxInFunc.Err(), context.DeadlineExceeded) {
					s.log.ErrorContext(ctx, "[cron-task] Task Timeout", "task", task.name, "timeout", task.timeout)
					span.AddEvent("task.timeout", trace.WithAttributes(
						attribute.Float64("timeout.seconds", task.timeout.Seconds()),
					))
				}
			}()
		}

		// 执行定时任务前记录事件
		span.AddEvent("task.execution.start")

		// 执行定时任务
		if err := task.tFunc(ctxInFunc); err != nil {
			task.err = err
			task.errMsg = fmt.Sprintf("task exec fail: %v", err)
			span.AddEvent("task.execution.error", trace.WithAttributes(
				attribute.String("error", err.Error()),
			))
		} else {
			span.AddEvent("task.execution.success")
		}
	}
}

// RemoveCronTask is a method of the Server struct that removes a cron task.
// It accepts the name of the task to be removed and returns an error if any occurred during the task removal.
//
// The method works as follows:
// 1. It retrieves the ID of the task to be removed from the MapEntryId map.
// 2. If the task ID is not found in the map, it returns an error.
// 3. It removes the task from the cron scheduler using the retrieved ID.
// 4. If an error occurred while removing the task from the scheduler, it returns the error.
// 5. It logs the removal of the task.
// 6. Finally, it returns nil as the error.
//
// Usage:
// err := server.RemoveCronTask("MyTask")
//
//	if err != nil {
//	    log.Fatalf("Failed to remove cron task: %v", err)
//	}
func (s *Server) RemoveCronTask(name string) error {
	entryID, ok := s.mapEntryId[name]
	if !ok {
		return ErrTaskNotFound
	}
	s.cron.Remove(entryID)
	delete(s.mapEntryId, name)
	s.log.InfoContext(s.baseCtx, "[cron-task] RemoveCronTask", "task", name)
	return nil
}

// RemoveAllTask is a method of the Server struct that removes all cron tasks.
func (s *Server) RemoveAllTask() {
	for name, entryID := range s.mapEntryId {
		s.cron.Remove(entryID)
		delete(s.mapEntryId, name)
	}
	s.log.InfoContext(s.baseCtx, "[cron-task] Empty Cron Tasks")
}

func (s *Server) Start(ctx context.Context) error {
	if s.err != nil {
		return s.err
	}
	if s.started {
		return nil
	}
	s.cron.Start()
	s.log.InfoContext(ctx, "[cron-task] server start success")
	s.started = true
	return nil
}

func (s *Server) Stop(_ context.Context) error {
	if !s.started {
		return nil
	}
	s.log.InfoContext(s.baseCtx, "[cron-task] server stopping")
	s.started = false
	stopCtx := s.cron.Stop()
	select {
	case <-stopCtx.Done():
		s.log.InfoContext(s.baseCtx, "[cron-task] server stop success")
		return nil
	case <-time.After(s.stopTimeout):
		return errors.New("cron server stop timeout")
	}
}
