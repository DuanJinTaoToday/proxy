package server

import (
	"context"
	"fmt"
	"log/slog"
	"reflect"

	"github.com/DuanJinTaoToday/proxy/apps/proxy-gateway/internal/conf"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-gateway/internal/service"
	"github.com/DuanJinTaoToday/proxy/pkg/cron"
	"github.com/go-kratos/kratos/v3/config"
)

type cronServerImpl struct {
	*cron.Server
	service     *service.CronService
	cronTaskCfg []*conf.Server_CronTask
}

func NewCronServer(app *conf.AppConfig, cronService *service.CronService) (*cronServerImpl, error) {
	bootstrap := app.GetBootstrap()
	cronServer := &cronServerImpl{
		service:     cronService,
		Server:      cron.NewServer(),
		cronTaskCfg: bootstrap.GetServer().GetCronTasks(),
	}
	if err := cronService.InitService(); err != nil {
		return nil, err
	}
	if err := cronServer.addCronTask(); err != nil {
		return nil, err
	}
	app.Register("server", cronServer)
	return cronServer, nil
}

// Notify 动态加载配置
func (s *cronServerImpl) Notify(value config.Value) {
	if value == nil {
		return
	}
	var data conf.Server
	if err := value.Scan(&data); err != nil {
		slog.Error("scan cron configuration", "error", err)
		return
	}
	if reflect.DeepEqual(s.cronTaskCfg, data.CronTasks) {
		return
	}
	s.cronTaskCfg = data.CronTasks
	s.RemoveAllTask()
	if err := s.addCronTask(); err != nil {
		slog.Error("register cron tasks", "error", err)
	}
}

// addCronTask 从配置文件中读取添加定时任务
func (s *cronServerImpl) addCronTask() error {
	value := reflect.ValueOf(s.service)
	for _, cronTask := range s.cronTaskCfg {
		method := value.MethodByName(cronTask.GetName())
		if !method.IsValid() {
			continue
		}
		m, ok := method.Interface().(func(context.Context) error)
		if !ok {
			return fmt.Errorf("cron function type error: %s", cronTask.GetName())
		}
		_, err := s.AddCronTask(
			cron.WithTaskFunc(m),
			cron.WithRunSpec(cronTask.GetSpec()),
			cron.WithTaskName(cronTask.GetName()),
			cron.WithTimeout(cronTask.GetTimeout().AsDuration()),
		)
		if err != nil {
			return fmt.Errorf("register cron task %q: %w", cronTask.GetName(), err)
		}
	}
	return nil
}
