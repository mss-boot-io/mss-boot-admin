package apis

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/mss-boot-io/mss-boot-admin-api/dto"
	"github.com/mss-boot-io/mss-boot-admin-api/middleware"
	"github.com/mss-boot-io/mss-boot-admin-api/models"
	"github.com/mss-boot-io/mss-boot/pkg/config/gormdb"
	"github.com/mss-boot-io/mss-boot/pkg/response"
	"github.com/mss-boot-io/mss-boot/pkg/response/actions"
	"github.com/mss-boot-io/mss-boot/pkg/response/controller"
)

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2023/12/18 23:55:10
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2023/12/18 23:55:10
 */

func init() {
	e := &Notice{
		Simple: controller.NewSimple(
			controller.WithAuth(true),
			controller.WithModel(new(models.Notice)),
			controller.WithSearch(new(dto.NoticeSearch)),
			controller.WithModelProvider(actions.ModelProviderGorm),
		),
	}
	response.AppendController(e)
}

type Notice struct {
	*controller.Simple
}

func (e *Notice) GetAction(key string) response.Action {
	return nil
}

func (e *Notice) Other(r *gin.RouterGroup) {
	r.GET("/notice/unread", middleware.Auth.MiddlewareFunc(), e.Unread)
}

// Unread 获取未读通知列表
// @Summary 获取未读通知列表
// @Description 获取未读通知列表
// @Tags notice
// @Accept  application/json
// @Product application/json
// @Success 200 {object} []models.Notice
// @Router /admin/api/notice/unread [get]
// @Security Bearer
func (e *Notice) Unread(ctx *gin.Context) {
	api := response.Make(ctx)
	verify := middleware.GetVerify(ctx)
	list := make([]*models.Notice, 0)
	err := gormdb.DB.Model(&models.Notice{}).
		Where("read = ?", false).
		Where("user_id = ?", verify.GetUserID()).
		Find(&list).Error
	if err != nil {
		api.AddError(err).Log.Error("get notice list error")
		api.Err(http.StatusInternalServerError)
		return
	}
	api.OK(list)
}

// Get 获取通知
// @Summary 获取通知
// @Description 获取通知
// @Tags notice
// @Accept  application/json
// @Product application/json
// @Param id path string true "id"
// @Success 200 {object} models.Notice
// @Router /admin/api/notices/{id} [get]
// @Security Bearer
func (e *Notice) Get(*gin.Context) {}

// Create 创建通知
// @Summary 创建通知
// @Description 创建通知
// @Tags notice
// @Accept  application/json
// @Product application/json
// @Param data body models.Notice true "data"
// @Success 201
// @Router /admin/api/notices [post]
// @Security Bearer
func (e *Notice) Create(*gin.Context) {}

// Update 更新通知
// @Summary 更新通知
// @Description 更新通知
// @Tags notice
// @Accept application/json
// @Product application/json
// @Param id path string true "id"
// @Param data body models.Notice true "data"
// @Success 200
// @Router /admin/api/notices/{id} [put]
// @Security Bearer
func (e *Notice) Update(*gin.Context) {}

// Delete 删除通知
// @Summary 删除通知
// @Description 删除通知
// @Tags notice
// @Param id path string true "id"
// @Success 204
// @Router /admin/api/notices/{id} [delete]
// @Security Bearer
func (e *Notice) Delete(*gin.Context) {}

// List 通知列表数据
// @Summary 通知列表数据
// @Description 通知列表数据
// @Tags notice
// @Accept  application/json
// @Product application/json
// @Param title query string false "title"
// @Param status query string false "status"
// @Param userID query string false "userID"
// @Param page query int false "page"
// @Param pageSize query int false "pageSize"
// @Success 200 {object} response.Page{data=[]models.Notice}
// @Router /admin/api/notices [get]
// @Security Bearer
func (e *Notice) List(*gin.Context) {}
