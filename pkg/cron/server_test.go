package cron

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"testing"
	"time"

	plog "github.com/DuanJinTaoToday/proxy/pkg/log"
)

func TestServer(t *testing.T) {
	logger, closeLogger, err := plog.NewLogger("cron-test", plog.Options{Level: "debug", Console: true})
	if err != nil {
		t.Fatal(err)
	}
	defer closeLogger()
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	defer signal.Stop(interrupt)
	ctx := context.Background()
	srv := NewServer(
		Logger(logger),
		WithContext(context.Background()),
		//WithRedisLock(redis.Options{}, "test"),
	)
	if _, err := srv.AddCronTask(
		WithTaskFunc(doWork1),
		WithRunSpec("@every 1s"),
		WithTaskName("doWork1"),
		WithTimeout(3*time.Second),
	); err != nil {
		t.Fatal(err)
	}
	if _, err := srv.AddCronTask(
		WithTaskFunc(doWork2),
		WithRunSpec("@every 3s"),
		WithTaskName("doWork2"),
		WithTimeout(5*time.Second),
	); err != nil {
		t.Fatal(err)
	}
	//srv.AddCronTask(
	//	WithTaskFunc(TroubleMaker),
	//	WithRunSpec("*/10 * * * * *"),
	//	WithTaskName("TroubleMaker"),
	//)
	//srv.AddCronTask(
	//	WithTaskFunc(PanicMaker),
	//	WithRunSpec("@every 3s"),
	//	WithTaskName("PanicMaker"),
	//)
	if _, err := srv.AddCronTask(
		WithTaskFunc(TimeoutMaker),
		WithRunSpec("@every 10s"),
		WithTaskName("TimeoutMaker"),
		WithTimeout(3*time.Second),
	); err != nil {
		t.Fatal(err)
	}

	if err := srv.Start(ctx); err != nil {
		t.Fatal(err)
	}
	// 起一个goroutine，分批停止定时任务
	go func() {
		time.Sleep(3 * time.Second)
		logger.InfoContext(ctx, "remove doWork1")
		if err := srv.RemoveCronTask("doWork1"); err != nil {
			t.Error(err)
		}
		time.Sleep(30 * time.Second)
		interrupt <- syscall.SIGTERM
	}()

	defer func() {
		if err := srv.Stop(ctx); err != nil {
			t.Errorf("expected nil got %v", err)
		}
	}()
	<-interrupt
}

func doWork1(ctx context.Context) error {
	slog.InfoContext(ctx, "doWork1", "spec", "@every 1s")
	return nil
}

func doWork2(ctx context.Context) error {
	slog.InfoContext(ctx, "doWork2", "spec", "@every 3s")
	return nil
}

func TroubleMaker(ctx context.Context) error {
	return errors.New("OH!! Fuck TroubleMaker")
}

func PanicMaker(ctx context.Context) error {
	panic("OH!! Fuck PanicMaker")
}

// 睡眠大于ctx超时时间的定时任务测试
func TimeoutMaker(ctx context.Context) error {
	// 模拟一个耗时操作
	sleepDuration := 5 * time.Second
	slog.InfoContext(ctx, "Task started", "sleep", sleepDuration)
	time.Sleep(sleepDuration)

	// 检查上下文是否已超时
	select {
	case <-ctx.Done():
		slog.InfoContext(ctx, "Task aborted due to timeout")
		return ctx.Err()
	default:
		slog.InfoContext(ctx, "Task completed successfully")
		return nil
	}
}
