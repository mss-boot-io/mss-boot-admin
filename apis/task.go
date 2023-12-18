package apis

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/mss-boot-io/mss-boot-admin-api/config"
	"github.com/mss-boot-io/mss-boot-admin-api/dto"
	"github.com/mss-boot-io/mss-boot-admin-api/models"
	"github.com/mss-boot-io/mss-boot/pkg/config/gormdb"
	"github.com/mss-boot-io/mss-boot/pkg/enum"
	"github.com/mss-boot-io/mss-boot/pkg/response"
	"github.com/mss-boot-io/mss-boot/pkg/response/actions"
	"github.com/mss-boot-io/mss-boot/pkg/response/controller"
)

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2023/12/7 13:24:59
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2023/12/7 13:24:59
 */

func init() {
	e := &Task{
		Simple: controller.NewSimple(
			controller.WithAuth(true),
			controller.WithModel(new(models.Task)),
			controller.WithSearch(new(dto.TaskSearch)),
			controller.WithModelProvider(actions.ModelProviderGorm),
		),
	}
	response.AppendController(e)
}

type Task struct {
	*controller.Simple
}

func (e *Task) Other(r *gin.RouterGroup) {
	r.GET("/task/:operate/:id", e.Operate)
}

// Operate 操作任务
// @Summary 操作任务
// @Description 操作任务
// @Tags task
// @Param id path string true "任务ID"
// @Param operate path string true "操作类型"
// @Success 200
// @Router /admin/api/task/{operate}/{id} [get]
// @Security Bearer
func (e *Task) Operate(c *gin.Context) {
	api := response.Make(c)
	req := &dto.TaskOperateRequest{}
	if api.Bind(req).Error != nil {
		api.Err(http.StatusUnprocessableEntity)
		return
	}
	var count int64
	err := gormdb.DB.Model(&models.Task{}).Where("id = ?", req.ID).Count(&count).Error
	if err != nil {
		api.AddError(err).Log.Error("count task error")
		api.Err(http.StatusInternalServerError)
		return
	}
	if count == 0 {
		api.Err(http.StatusNotFound)
		return
	}
	var status enum.Status
	switch req.Operate {
	case "start":
		status = enum.Enabled
	case "stop":
		status = enum.Disabled
	default:
		api.Err(http.StatusBadRequest, "operate not support")
		return
	}

	err = gormdb.DB.Model(&models.Task{}).Where("id = ?", req.ID).Update("status", status).Error
	if err != nil {
		api.AddError(err).Log.Error("update task status error")
		api.Err(http.StatusInternalServerError)
		return
	}
	if status == enum.Enabled && config.Cfg.Task.Enable {
		go func() {
			err = models.TaskOnce(req.ID)
			if err != nil {
				slog.Error("task run error", slog.Any("err", err))
			}
		}()
	}
	api.OK(nil)
}

// Create 创建任务
// @Summary 创建任务
// @Description 创建任务
// @Tags task
// @Accept  application/json
// @Product application/json
// @Param data body models.Task true "data"
// @Success 201
// @Router /admin/api/tasks [post]
// @Security Bearer
func (e *Task) Create(*gin.Context) {}

// Delete 删除任务
// @Summary 删除任务
// @Description 删除任务
// @Tags task
// @Param id path string true "id"
// @Success 204
// @Router /admin/api/tasks/{id} [delete]
// @Security Bearer
func (e *Task) Delete(*gin.Context) {}

// Update 更新任务
// @Summary 更新任务
// @Description 更新任务
// @Tags task
// @Accept  application/json
// @Product application/json
// @Param id path string true "id"
// @Param data body models.Task true "data"
// @Success 200
// @Router /admin/api/tasks/{id} [put]
// @Security Bearer
func (e *Task) Update(*gin.Context) {}

// Get 获取任务
// @Summary 获取任务
// @Description 获取任务
// @Tags task
// @Param id path string true "id"
// @Success 200 {object} models.Task
// @Router /admin/api/tasks/{id} [get]
// @Security Bearer
func (e *Task) Get(*gin.Context) {}

// List 任务列表
// @Summary 任务列表
// @Description 任务列表
// @Tags task
// @Accept  application/json
// @Product application/json
// @Param page query int false "page"
// @Param pageSize query int false "pageSize"
// @Param id query string false "id"
// @Param name query string false "name"
// @Param status query int false "status"
// @Param remark query string false "remark"
// @Success 200 {object} response.Page{data=[]models.Task}
// @Router /admin/api/tasks [get]
// @Security Bearer
func (e *Task) List(*gin.Context) {}
