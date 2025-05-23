package apis

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/mss-boot-io/mss-boot-admin/center"
	"github.com/mss-boot-io/mss-boot-admin/config"
	"github.com/mss-boot-io/mss-boot-admin/dto"
	"github.com/mss-boot-io/mss-boot-admin/models"
	"github.com/mss-boot-io/mss-boot/core/server/task"
	"github.com/mss-boot-io/mss-boot/pkg/enum"
	"github.com/mss-boot-io/mss-boot/pkg/response"
	"github.com/mss-boot-io/mss-boot/pkg/response/actions"
	"github.com/mss-boot-io/mss-boot/pkg/response/controller"
	"gorm.io/gorm"
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
			controller.WithScope(center.Default.Scope),
		),
	}
	response.AppendController(e)
}

type Task struct {
	*controller.Simple
}

func (e *Task) Other(r *gin.RouterGroup) {
	r.GET("/task/:operate/:id", e.Operate)
	r.GET("/task/func-list", e.FuncList)
}

// FuncList 任务函数列表
// @Summary 任务函数列表
// @Description 任务函数列表
// @Tags task
// @Accept  application/json
// @Product application/json
// @Success 200 {object} []dto.TaskFuncItem
// @Router /admin/api/task/func-list [get]
// @Security Bearer
func (e *Task) FuncList(c *gin.Context) {
	api := response.Make(c)
	resp := make([]*dto.TaskFuncItem, 0)
	// 获取所有的任务函数
	for k := range models.TaskFuncMap {
		resp = append(resp, &dto.TaskFuncItem{
			Name: k,
		})
	}
	api.OK(resp)
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
	t := &models.Task{}
	err := center.Default.GetDB(c, &models.Task{}).
		Model(&models.Task{}).
		Where("id = ?", req.ID).
		First(t).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			api.Err(http.StatusNotFound)
			return
		}
		api.AddError(err).Log.Error("get task error")
		api.Err(http.StatusInternalServerError)
		return
	}
	var status enum.Status
	switch req.Operate {
	case "start":
		err = task.UpdateJob(t.ID, t.Spec, t)
		status = enum.Enabled
	case "stop":
		err = task.RemoveJob(t.ID)
		status = enum.Disabled
	default:
		api.Err(http.StatusBadRequest, "operate not support")
		return
	}
	if err != nil {
		api.AddError(err).Log.Error("task operate error")
		api.Err(http.StatusInternalServerError)
		return
	}

	err = center.Default.GetDB(c, &models.Task{}).Model(&models.Task{}).Where("id = ?", req.ID).Update("status", status).Error
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
// @Success 201 {object} models.Task
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
// @Success 200 {object} models.Task
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
// @Param current query int false "current"
// @Param pageSize query int false "pageSize"
// @Param id query string false "id"
// @Param name query string false "name"
// @Param status query string false "status"
// @Param remark query string false "remark"
// @Success 200 {object} response.Page{data=[]models.Task}
// @Router /admin/api/tasks [get]
// @Security Bearer
func (e *Task) List(*gin.Context) {}
