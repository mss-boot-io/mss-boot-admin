package server

import (
	"context"
	"errors"
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
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/config/storage"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/enum"

	"github.com/mss-boot-io/mss-boot-admin/admin/business"
	"github.com/mss-boot-io/mss-boot-admin/admin/center"
	adminwebsocket "github.com/mss-boot-io/mss-boot-admin/admin/center/websocket"
	"github.com/mss-boot-io/mss-boot-admin/admin/config"
	"github.com/mss-boot-io/mss-boot-admin/admin/middleware"
	"github.com/mss-boot-io/mss-boot-admin/admin/models"
	"github.com/mss-boot-io/mss-boot-admin/admin/pkg/requestlog"
	"github.com/mss-boot-io/mss-boot-admin/admin/pkg/schemahealth"
	"github.com/mss-boot-io/mss-boot-admin/admin/pkg/sessioncache"
	"github.com/mss-boot-io/mss-boot-admin/admin/router"
	"github.com/mss-boot-io/mss-boot-admin/admin/service"
)

var (
	apiCheck       bool
	group          = "/admin"
	driver         = "mysql"
	dsn            = "root:123456@tcp(127.0.0.1:3306)/mss-boot-admin-local?charset=utf8&parseTime=True&loc=Local"
	configProvider = os.Getenv("CONFIG_PROVIDER")
	StartCmd       = NewCommand(nil)
)

type commandOptions struct {
	apiCheck       bool
	group          string
	driver         string
	dsn            string
	configProvider string
}

func defaultCommandOptions() commandOptions {
	return commandOptions{
		group:          "/admin",
		driver:         "mysql",
		dsn:            "root:123456@tcp(127.0.0.1:3306)/mss-boot-admin-local?charset=utf8&parseTime=True&loc=Local",
		configProvider: os.Getenv("CONFIG_PROVIDER"),
	}
}

// NewCommand returns an isolated server command bound to one frozen business
// module registry. Flag values never leak into another Application instance.
func NewCommand(registry *business.Registry) *cobra.Command {
	options := defaultCommandOptions()
	command := &cobra.Command{
		Use:     "server",
		Short:   "start server",
		Long:    "start mss-boot-admin server",
		Example: "mss-boot-admin server",
		PreRunE: func(cmd *cobra.Command, _ []string) error {
			return setupWithOptions(cmd.Context(), &options, registry)
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runWithOptions(cmd.Context(), options.apiCheck)
		},
	}
	command.PersistentFlags().BoolVarP(&options.apiCheck,
		"api", "a", false, "Start server with check api data")
	command.PersistentFlags().StringVarP(&options.configProvider,
		"config-provider", "c", options.configProvider, "Start server with config provider")
	command.PersistentFlags().StringVarP(&options.group,
		"group", "g", options.group, "Start server with group path")
	command.PersistentFlags().StringVarP(&options.driver,
		"gorm-driver", "r", options.driver, "Start server with db driver")
	command.PersistentFlags().StringVarP(&options.dsn,
		"gorm-dsn", "n", options.dsn, "Start server with db dsn")
	return command
}

func init() {
	center.SetVerify(&models.User{})
}

func setup(ctx context.Context) (err error) {
	options := defaultCommandOptions()
	options.apiCheck = apiCheck
	options.group = group
	options.driver = driver
	options.dsn = dsn
	options.configProvider = configProvider
	return setupWithOptions(ctx, &options, nil)
}

func setupWithOptions(
	ctx context.Context,
	options *commandOptions,
	businessRegistry *business.Registry,
) (err error) {
	if options == nil {
		return errors.New("server command options are required")
	}
	defer func() {
		if err != nil {
			if closeErr := config.Cfg.Close(); closeErr != nil {
				slog.Error("close database after setup failure", "err", closeErr)
			}
		}
	}()

	if os.Getenv("DB_DRIVER") != "" {
		options.driver = os.Getenv("DB_DRIVER")
	} else {
		_ = os.Setenv("DB_DRIVER", options.driver)
	}
	if os.Getenv("DB_DSN") != "" {
		options.dsn = os.Getenv("DB_DSN")
	} else {
		_ = os.Setenv("DB_DSN", options.dsn)
	}

	opts := []source.Option{
		source.WithDir("config"),
		source.WithProvider(source.Local),
		source.WithWatch(true),
	}
	switch pkg.GetStage() {
	case "local", "dev":
		options.configProvider = string(source.Local)
	}
	switch source.Provider(options.configProvider) {
	case source.GORM:
		opts = []source.Option{
			source.WithProvider(source.GORM),
			source.WithGORMDriver(options.driver),
			source.WithGORMDsn(options.dsn),
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
		return fmt.Errorf("config provider %q is not supported", options.configProvider)
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
	databaseHandle := config.Cfg.DatabaseHandle()
	if databaseHandle == nil || databaseHandle.DB == nil {
		return fmt.Errorf("application database handle is not initialized")
	}

	center.SetAppConfig(&models.AppConfig{})
	center.SetUserConfig(&models.UserConfig{})
	center.SetStatistics(&models.Statistics{})
	center.SetGRPCClient(&config.Cfg.GRPC)

	middleware.Verifier = center.GetUser()
	service.Session.SetCache(sessioncache.New(redisClientFromCenter()))
	if err := middleware.Init(); err != nil {
		return err
	}

	routerEngine := gin.New()
	if err := routerEngine.SetTrustedProxies(config.Cfg.Application.TrustedProxies); err != nil {
		return fmt.Errorf("configure trusted reverse proxies: %w", err)
	}
	routerEngine.Use(requestlog.Logger(), requestlog.Recovery())
	routerEngine.Use(middleware.AuditLogMiddleware("/admin/api/auth", "/admin/api/login", "/admin/api/logout"))
	center.SetServerManager(frameworkserver.New())
	center.SetMakeRouter(router.DefaultMakeRouter)
	center.SetRouter(routerEngine)
	businessRoutes := routerEngine.Group(options.group)
	routeGroups, err := mountCoreRouteGroupsAfterSchemaReadiness(
		ctx,
		databaseHandle.DB,
		businessRoutes,
	)
	if err != nil {
		return err
	}
	if businessRegistry != nil {
		if middleware.Auth == nil {
			return errors.New("business authentication middleware is not initialized")
		}
		protectedBusinessAPI := routeGroups.ProtectedAPI.Group(
			"",
			leaseBusinessRequestDatabase(config.Cfg.WithDatabase),
			middleware.Auth.MiddlewareFunc(),
		)
		if err := businessRegistry.Mount(
			ctx,
			business.DatabaseAccess(config.Cfg.WithDatabase),
			protectedBusinessAPI,
			business.Runtime{
				RequestDatabase: center.RequestDatabase,
				Principal:       middleware.GetVerify,
				Events:          businessEventLogger{},
			},
		); err != nil {
			return err
		}
	}
	if err := initializeApplicationDelivery(ctx, config.Cfg, center.GetRouter()); err != nil {
		return err
	}

	if options.apiCheck {
		if err := models.SaveAPI(routerEngine.Routes()); err != nil {
			return fmt.Errorf("save API routes: %w", err)
		}
		return nil
	}

	runnable := []frameworkserver.Runnable{
		config.Cfg.Server.Init(
			listener.WithStartedHook(tips),
			listener.WithName("admin"),
			listener.WithHandler(routerEngine)),
	}
	authorizationRuntime, err := buildAuthorizationEventRuntime(config.Cfg.WithDatabase)
	if err != nil {
		return err
	}
	if err := authorizationRuntime.Open(ctx); err != nil {
		return fmt.Errorf("open authorization revision event runtime: %w", err)
	}
	runnable = append(runnable, authorizationRuntime)
	if queueRunnable := managedQueueRunnable(config.Cfg.ManagedQueue()); queueRunnable != nil {
		runnable = append(runnable, queueRunnable)
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

func mountCoreRouteGroupsAfterSchemaReadiness(
	ctx context.Context,
	db *gorm.DB,
	group *gin.RouterGroup,
) (router.RouteGroups, error) {
	if group == nil {
		return router.RouteGroups{}, errors.New("core route composition is not initialized")
	}
	if err := schemahealth.VerifyCanonicalEmailIdentity(
		ctx,
		db,
		schemahealth.CanonicalEmailRuntimeReadiness,
	); err != nil {
		return router.RouteGroups{}, fmt.Errorf("application schema readiness failed: %w", err)
	}
	return router.InitRouteGroups(group), nil
}

type systemTaskSchedule struct {
	key  string
	spec string
	job  cron.Job
}

type databaseAccess func(func(*gorm.DB) error) error

func buildAuthorizationEventRuntime(
	useDatabase databaseAccess,
) (*service.AuthorizationEventRuntime, error) {
	if useDatabase == nil {
		return nil, errors.New("authorization database lease is required")
	}
	runtime, err := service.BuildMemoryAuthorizationEventRuntime(
		service.AuthorizationPolicies,
		func(ctx context.Context, operation func(*gorm.DB) error) error {
			if ctx == nil {
				return errors.New("authorization database context is required")
			}
			if err := ctx.Err(); err != nil {
				return err
			}
			return useDatabase(func(db *gorm.DB) error {
				return operation(db.WithContext(ctx))
			})
		},
		0,
		service.WithAuthorizationRevisionNotifier(func(revision uint64) {
			if !adminwebsocket.BroadcastAuthorizationRevision(revision) {
				slog.Warn("authorization revision realtime signal dropped", "revision", revision)
			}
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("build authorization revision event runtime: %w", err)
	}
	return runtime, nil
}

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

func run(ctx context.Context) (err error) {
	return runWithOptions(ctx, apiCheck)
}

func runWithOptions(ctx context.Context, apiOnly bool) (err error) {
	if ctx == nil {
		ctx = context.Background()
	}
	defer func() {
		closeCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if closeErr := config.Cfg.CloseContext(closeCtx); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("close application resources: %w", closeErr))
		}
	}()
	if apiOnly {
		return nil
	}
	startLegacyQueue(ctx, center.GetQueue())
	return center.Default.Start(ctx)
}

func managedQueueRunnable(adapter storage.AdapterQueue) frameworkserver.Runnable {
	managed, _ := adapter.(storage.ManagedAdapterQueue)
	if managed == nil {
		return nil
	}
	return &managedQueueRuntime{queue: managed}
}

type managedQueueRuntime struct {
	queue storage.ManagedAdapterQueue
}

func (r *managedQueueRuntime) String() string {
	return r.queue.String()
}

func (r *managedQueueRuntime) Start(ctx context.Context) error {
	if ctx == nil {
		return errors.New("managed queue runtime context is required")
	}
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	result := make(chan error, 1)
	go func() { result <- r.queue.Start(runCtx) }()
	errorsCh := r.queue.Errors()
	for {
		select {
		case err := <-result:
			return errors.Join(err, drainManagedQueueErrors(errorsCh))
		case runtimeErr, ok := <-errorsCh:
			if !ok {
				errorsCh = nil
				continue
			}
			if runtimeErr == nil {
				continue
			}
			cancel()
			return errors.Join(
				fmt.Errorf("managed queue runtime error: %w", runtimeErr),
				stopManagedQueueAfterRuntimeError(ctx, r.queue, result),
			)
		}
	}
}

func drainManagedQueueErrors(errorsCh <-chan error) error {
	var observed error
	for errorsCh != nil {
		select {
		case runtimeErr, ok := <-errorsCh:
			if !ok {
				return observed
			}
			if runtimeErr != nil {
				observed = errors.Join(observed, runtimeErr)
			}
		default:
			return observed
		}
	}
	return observed
}

func stopManagedQueueAfterRuntimeError(
	parent context.Context,
	queue storage.ManagedAdapterQueue,
	result <-chan error,
) error {
	closeCtx, cancel := context.WithTimeout(context.WithoutCancel(parent), 5*time.Second)
	closeErr := queue.Close(closeCtx)
	var startErr error
	select {
	case startErr = <-result:
		if errors.Is(startErr, context.Canceled) {
			startErr = nil
		}
	case <-closeCtx.Done():
		startErr = fmt.Errorf("wait for managed queue runtime: %w", closeCtx.Err())
	}
	cancel()
	return errors.Join(closeErr, startErr)
}

// startLegacyQueue confines the detached compatibility bridge to providers
// that have not adopted ManagedAdapterQueue. Kafka implements the managed
// contract and therefore can only be started by the server manager.
func startLegacyQueue(ctx context.Context, adapter storage.AdapterQueue) {
	if adapter == nil || managedQueueRunnable(adapter) != nil {
		return
	}
	go adapter.Run(ctx)
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
