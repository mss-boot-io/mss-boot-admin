package server

import (
	"context"
	"log/slog"
	"time"

	"github.com/mss-boot-io/mss-boot/core/server/task"
	"github.com/mss-boot-io/mss-boot/pkg/config/gormdb"
	"github.com/mss-boot-io/mss-boot/pkg/enum"
	"github.com/robfig/cron/v3"

	"github.com/gin-gonic/gin"
	"github.com/mss-boot-io/mss-boot/core/server"
	"github.com/mss-boot-io/mss-boot/core/server/listener"
	"github.com/mss-boot-io/mss-boot/virtual/action"
	"github.com/spf13/cobra"

	"github.com/mss-boot-io/mss-boot-admin-api/config"
	"github.com/mss-boot-io/mss-boot-admin-api/middleware"
	"github.com/mss-boot-io/mss-boot-admin-api/models"
	"github.com/mss-boot-io/mss-boot-admin-api/router"
)

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2023/8/10 00:33:48
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2023/8/10 00:33:48
 */

var (
	apiCheck bool
	group    string
	StartCmd = &cobra.Command{
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
	StartCmd.PersistentFlags().StringVarP(&group,
		"group", "g",
		"/admin",
		"Start server with group path")
}

func setup() error {
	// setup 01 config init
	config.Cfg.Init()

	// setup 02 middleware init
	middleware.Verifier = &models.User{}
	middleware.Init()

	// setup 03 router init
	r := gin.Default()
	router.Init(r.Group(group))
	config.Cfg.Application.Init(r)

	// setup 04 api check
	if apiCheck {
		err := models.SaveAPI(r.Routes())
		if err != nil {
			slog.Error("save api error", "err", err)
		}
	}

	// setup 05 server init
	runnable := []server.Runnable{
		config.Cfg.Server.Init(
			listener.WithName("admin"),
			listener.WithHandler(r)),
	}

	// setup 06 task init
	if config.Cfg.Task.Enable {
		runnable = append(runnable,
			task.New(task.WithStorage(&models.TaskStorage{DB: gormdb.DB}), task.WithSchedule("task", config.Cfg.Task.Spec, &taskE{})))
	}

	// setup 07 init virtual models
	ms, err := models.GetModels()
	if err != nil {
		return err
	}
	for i := range ms {
		action.SetModel(ms[i].Path, ms[i].MakeVirtualModel())
	}

	// setup 08 add runnable to manager
	server.Manage.Add(runnable...)

	return nil
}

func run() error {
	ctx := context.Background()

	return server.Manage.Start(ctx)
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
