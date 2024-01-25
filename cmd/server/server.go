package server

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/common-nighthawk/go-figure"
	"github.com/gin-gonic/gin"
	"github.com/mss-boot-io/mss-boot/core/server"
	"github.com/mss-boot-io/mss-boot/core/server/listener"
	"github.com/mss-boot-io/mss-boot/core/server/task"
	"github.com/mss-boot-io/mss-boot/pkg/config/gormdb"
	"github.com/mss-boot-io/mss-boot/pkg/config/source"
	"github.com/mss-boot-io/mss-boot/pkg/enum"
	"github.com/mss-boot-io/mss-boot/virtual/action"
	"github.com/robfig/cron/v3"
	"github.com/spf13/cobra"

	"github.com/mss-boot-io/mss-boot-admin/app/admin"
	"github.com/mss-boot-io/mss-boot-admin/app/admin/models"
	"github.com/mss-boot-io/mss-boot-admin/center"
	"github.com/mss-boot-io/mss-boot-admin/config"
	"github.com/mss-boot-io/mss-boot-admin/middleware"
)

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2023/8/10 00:33:48
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2023/8/10 00:33:48
 */

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
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return setup()
		},
		RunE: func(cmd *cobra.Command, args []string) error {
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
		"config-provider", "p",
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
	center.SetTenant(&models.Tenant{}).
		SetVerify(&models.User{})
}

func setup() error {
	// setup 00 set params
	// env overwrite args
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
	// setup 01 config init
	opts := []source.Option{
		// use local config file
		source.WithDir("config"),
		source.WithProvider(source.Local),
	}
	switch source.Provider(configProvider) {
	case source.GORM, "":
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
	case source.Local:
	default:
		slog.Error("config provider not support", "provider", configProvider)
		os.Exit(-1)
	}
	center.SetConfig(config.Cfg).Init(opts...)
	err := models.InitTenant(gormdb.DB)
	if err != nil {
		return err
	}

	// app config
	center.SetAppConfig(&models.AppConfig{})
	// statistics config
	center.SetStatistics(&models.Statistics{})
	center.SetGRPCClient(&config.Cfg.GRPC)

	// setup 02 middleware init
	middleware.Verifier = center.GetUser()
	middleware.Init()

	// setup 03 router init
	r := gin.Default()
	center.SetMakeRouter(admin.DefaultMakeRouter)
	center.SetRouter(r)
	center.Default.MakeRouter(r.Group(group))
	config.Cfg.Application.Init(center.GetRouter())

	// setup 04 api check
	if apiCheck {
		err := models.SaveAPI(r.Routes())
		if err != nil {
			slog.Error("save api error", "err", err)
		}
		os.Exit(0)
	}

	// setup 05 server init
	runnable := []server.Runnable{
		config.Cfg.Server.Init(
			listener.WithStartedHook(tips),
			listener.WithName("admin"),
			listener.WithHandler(r)),
	}

	// setup 06 task init
	if config.Cfg.Task.Enable {
		runnable = append(runnable,
			task.New(task.WithStorage(&models.TaskStorage{DB: gormdb.DB}), task.WithSchedule("task", config.Cfg.Task.Spec, &taskE{})))
	}

	// setup 07 init virtual models
	//todo every tenant has different models
	ms, err := center.SetVirtualModel(&models.Model{}).GetModels(nil)
	if err != nil {
		return err
	}
	for i := range ms {
		action.SetModel(ms[i].GetKey(), ms[i].Make())
	}

	// setup 08 add runnable to manager
	center.Default.Add(runnable...)

	return nil
}

func run() error {
	ctx := context.Background()

	return center.Default.Start(ctx)
}

func tips() {
	figure.NewFigure(config.Cfg.Application.Name, "rectangles", true).Print()
	fmt.Println()
}

type taskE struct {
}

func (t *taskE) Run() {
	tasks := make([]*models.Task, 0)
	err := gormdb.DB.Where("checked_at < ? or checked_at is null", time.Now().Add(-1*time.Minute)).
		Where("status = ?", enum.Enabled).Find(&tasks).Error
	if err != nil {
		slog.Error("task run get tasks error", slog.Any("err", err))
		return
	}
	for i := range tasks {
		slog.Info("task", "id", tasks[i].ID, "checked_at", tasks[i].CheckedAt)
		err = task.UpdateJob(tasks[i].ID, tasks[i].Spec, tasks[i])
		if err != nil {
			slog.Error("task run update job error", slog.Any("err", err))
			continue
		}
	}
	//check
	err = gormdb.DB.Where("status = ?", enum.Enabled).Find(&tasks).Error
	if err != nil {
		slog.Error("task run get tasks error", slog.Any("err", err))
		return
	}
	for i := range tasks {
		if entry := task.Entry(cron.EntryID(tasks[i].EntryID)); entry.ID > 0 {
			err = gormdb.DB.Model(&models.Task{}).
				Where("id = ?", tasks[i].ID).
				Update("checked_at", time.Now()).Error
			if err != nil {
				slog.Error("task run update task error", slog.Any("err", err))
				continue
			}
		}
	}
}
