package server

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/common-nighthawk/go-figure"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/robfig/cron/v3"
	"github.com/spf13/cobra"
	"gorm.io/gorm"

	frameworkserver "github.com/mss-boot-io/mss-boot-admin/mss-boot/core/server"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/core/server/listener"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/core/server/task"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/config/source"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/enum"

	"github.com/mss-boot-io/mss-boot-admin/admin/center"
	"github.com/mss-boot-io/mss-boot-admin/admin/config"
	"github.com/mss-boot-io/mss-boot-admin/admin/middleware"
	"github.com/mss-boot-io/mss-boot-admin/admin/models"
	"github.com/mss-boot-io/mss-boot-admin/admin/pkg/requestlog"
	"github.com/mss-boot-io/mss-boot-admin/admin/pkg/sessioncache"
	"github.com/mss-boot-io/mss-boot-admin/admin/router"
	"github.com/mss-boot-io/mss-boot-admin/admin/service"
)

var (
	apiCheck       bool
	group          string
	driver         string
	dsn            string
	configProvider string
	StartCmd       = &cobra.Command{
		Use:     "server",
		Short:   "start server",
		Long:    "start mss-boot-admin server",
		Example: "mss-boot-admin server",
		PreRunE: func(cmd *cobra.Command, _ []string) error {
			return setup(cmd.Context())
		},
		RunE: func(_ *cobra.Command, _ []string) error {
			return run()
		},
	}
)

func init() {
	StartCmd.PersistentFlags().BoolVarP(&apiCheck,
		"api", "a",
		false,
		"Start server with check api data")
	StartCmd.PersistentFlags().StringVarP(&configProvider,
		"config-provider", "c",
		os.Getenv("CONFIG_PROVIDER"),
		"Start server with config provider")
	StartCmd.PersistentFlags().StringVarP(&group,
		"group", "g",
		"/admin",
		"Start server with group path")
	StartCmd.PersistentFlags().StringVarP(&driver,
		"gorm-driver", "r",
		"mysql", "Start server with db driver")
	StartCmd.PersistentFlags().StringVarP(&dsn,
		"gorm-dsn", "n",
		"root:123456@tcp(127.0.0.1:3306)/mss-boot-admin-local?charset=utf8&parseTime=True&loc=Local",
		"Start server with db dsn")
	center.SetVerify(&models.User{})
}

func setup(ctx context.Context) (err error) {
	defer func() {
		if err != nil {
			if closeErr := config.Cfg.Close(); closeErr != nil {
				slog.Error("close database after setup failure", "err", closeErr)
			}
		}
	}()

	if os.Getenv("DB_DRIVER") != "" {
		driver = os.Getenv("DB_DRIVER")
	} else {
		_ = os.Setenv("DB_DRIVER", driver)
	}
	if os.Getenv("DB_DSN") != "" {
		dsn = os.Getenv("DB_DSN")
	} else {
		_ = os.Setenv("DB_DSN", dsn)
	}

	opts := []source.Option{
		source.WithDir("config"),
		source.WithProvider(source.Local),
		source.WithWatch(true),
	}
	switch pkg.GetStage() {
	case "local", "dev":
		configProvider = string(source.Local)
	}
	switch source.Provider(configProvider) {
	case source.GORM:
		opts = []source.Option{
			source.WithProvider(source.GORM),
			source.WithGORMDriver(driver),
			source.WithGORMDsn(dsn),
			source.WithDriver(&models.SystemConfig{}),
		}
	case source.FS:
		opts = []source.Option{
			source.WithProvider(source.FS),
			source.WithFrom(config.FS),
		}
	case source.ConfigMap:
		opts = []source.Option{
			source.WithProvider(source.ConfigMap),
			source.WithConfigmap("mss-boot-admin"),
			source.WithNamespace(pkg.GetStage()),
		}
	case source.Consul:
		opts = []source.Option{
			source.WithProvider(source.Consul),
			source.WithDir("mss-boot-admin/config"),
			source.WithWatch(true),
		}
	case source.APPConfig:
		opts = []source.Option{
			source.WithProvider(source.APPConfig),
			source.WithProjectName(pkg.GetProjectName()),
			source.WithNamespace(pkg.GetStage()),
		}
	case source.Local, "":
	default:
		return fmt.Errorf("config provider %q is not supported", configProvider)
	}
	if customConfig := center.GetCustomConfig(); customConfig != nil {
		opts = append(opts, source.WithPostfixHook(customConfig))
	}
	center.SetConfig(config.Cfg)
	if err := config.Cfg.InitContext(ctx, opts...); err != nil {
		return err
	}
	if customConfig := center.GetCustomConfig(); customConfig != nil {
		customConfig.Init()
	}
	if err := service.DefaultMonitor.Configure(service.MonitorOptions{
		SampleInterval: config.Cfg.Monitor.SampleInterval,
		SampleTimeout:  config.Cfg.Monitor.SampleTimeout,
		HistorySize:    config.Cfg.Monitor.HistorySize,
	}); err != nil {
		return fmt.Errorf("configure monitor sampler: %w", err)
	}

	center.SetAppConfig(&models.AppConfig{})
	center.SetUserConfig(&models.UserConfig{})
	center.SetStatistics(&models.Statistics{})
	center.SetGRPCClient(&config.Cfg.GRPC)

	middleware.Verifier = center.GetUser()
	service.Session.SetCache(sessioncache.New(redisClientFromCenter()))
	middleware.Init()

	routerEngine := gin.New()
	routerEngine.Use(requestlog.Logger(), requestlog.Recovery())
	routerEngine.Use(middleware.AuditLogMiddleware("/admin/api/auth", "/admin/api/login", "/admin/api/logout"))
	center.SetMakeRouter(router.DefaultMakeRouter)
	center.SetRouter(routerEngine)
	center.Default.MakeRouter(routerEngine.Group(group))
	config.Cfg.Application.Init(center.GetRouter())

	if apiCheck {
		if err := models.SaveAPI(routerEngine.Routes()); err != nil {
			slog.Error("save api error", "err", err)
		}
		os.Exit(0)
	}

	runnable := []frameworkserver.Runnable{
		config.Cfg.Server.Init(
			listener.WithStartedHook(tips),
			listener.WithName("admin"),
			listener.WithHandler(routerEngine)),
	}

	databaseHandle := config.Cfg.DatabaseHandle()
	if databaseHandle == nil || databaseHandle.DB == nil {
		return fmt.Errorf("application database handle is not initialized")
	}
	userTasksEnabled := config.Cfg.Task.Enable
	userTaskSpec := config.Cfg.Task.Spec
	taskOptions := []task.Option{task.WithUserSchedulesEnabled(userTasksEnabled)}
	if userTasksEnabled {
		taskOptions = append(taskOptions, task.WithStorage(&models.TaskStorage{UseDatabase: config.Cfg.WithDatabase}))
	}
	for _, schedule := range systemTaskSchedules(
		userTasksEnabled,
		userTaskSpec,
		service.DefaultMonitor,
		config.Cfg.WithDatabase,
	) {
		taskOptions = append(taskOptions, task.WithSystemSchedule(schedule.key, schedule.spec, schedule.job))
	}
	runnable = append(runnable, task.New(taskOptions...))

	if config.Cfg.Application.Mode == config.ModeDev &&
		config.Cfg.Application.UI.Enabled {
		runnable = append(runnable, config.Cfg.Application.UI.Init())
	}

	service.DefaultMonitor.Prime(ctx)
	center.Default.Add(runnable...)
	return nil
}

type systemTaskSchedule struct {
	key  string
	spec string
	job  cron.Job
}

type databaseAccess func(func(*gorm.DB) error) error

func systemTaskSchedules(
	userTasksEnabled bool,
	userTaskSpec string,
	monitor *service.Monitor,
	useDatabase databaseAccess,
) []systemTaskSchedule {
	schedules := []systemTaskSchedule{
		{key: "monitor-sampler", spec: monitor.ScheduleSpec(), job: monitor},
		{key: "session-cleanup", spec: "0 30 3 * * *", job: taskSessionCleanup{UseDatabase: useDatabase}},
	}
	if !userTasksEnabled {
		return schedules
	}
	return append(schedules, systemTaskSchedule{
		key: "task", spec: userTaskSpec, job: &taskE{UseDatabase: useDatabase},
	})
}

func run() error {
	ctx := context.Background()
	if center.GetQueue() != nil {
		go center.GetQueue().Run(ctx)
	}
	return center.Default.Start(ctx)
}

func tips() {
	figure.NewFigure(config.Cfg.Application.Name, "rectangles", true).Print()
	fmt.Println()
}

type taskE struct {
	UseDatabase databaseAccess
}

func (t *taskE) Run() {
	if t == nil || t.UseDatabase == nil {
		slog.Error("task scheduler database is not initialized")
		return
	}
	tasks := make([]*models.Task, 0)
	err := t.UseDatabase(func(db *gorm.DB) error {
		return db.
			Where("checked_at < ? or checked_at is null", time.Now().Add(-time.Minute)).
			Where("status = ?", enum.Enabled).
			Find(&tasks).Error
	})
	if err != nil {
		slog.Error("task run get tasks error", slog.Any("err", err))
		return
	}
	for i := range tasks {
		if tasks[i] == nil || tasks[i].Provider == models.TaskProviderK8S {
			continue
		}
		slog.Info("task", "id", tasks[i].ID, "checked_at", tasks[i].CheckedAt)
		if err = task.UpdateJob(tasks[i].ID, tasks[i].Spec, tasks[i]); err != nil {
			slog.Error("task run update job error", slog.Any("err", err))
		}
	}

	tasks = tasks[:0]
	if err = t.UseDatabase(func(db *gorm.DB) error {
		return db.Not("provider = ?", models.TaskProviderK8S).
			Where("status = ?", enum.Enabled).
			Find(&tasks).Error
	}); err != nil {
		slog.Error("task run get tasks error", slog.Any("err", err))
		return
	}
	for i := range tasks {
		if entry := task.Entry(cron.EntryID(tasks[i].EntryID)); entry.ID > 0 {
			if err = t.UseDatabase(func(db *gorm.DB) error {
				return db.Model(&models.Task{}).
					Where("id = ?", tasks[i].ID).
					Update("checked_at", time.Now()).Error
			}); err != nil {
				slog.Error("task run update task error", slog.Any("err", err))
			}
		}
	}
}

type taskSessionCleanup struct {
	UseDatabase databaseAccess
}

func (t taskSessionCleanup) Run() {
	if t.UseDatabase == nil {
		slog.Error("session cleanup database is not initialized")
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	var n int64
	err := t.UseDatabase(func(db *gorm.DB) error {
		var cleanupErr error
		n, cleanupErr = service.Session.CleanupOlderThan(ctx, db, 30*24*time.Hour)
		return cleanupErr
	})
	if err != nil {
		slog.Error("session cleanup failed", "err", err)
		return
	}
	slog.Info("session cleanup done", "deleted", n)
}

func redisClientFromCenter() redis.UniversalClient {
	cacheClient := center.GetCache()
	if cacheClient == nil {
		return nil
	}
	return cacheClient
}
