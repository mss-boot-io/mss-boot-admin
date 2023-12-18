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
 * @Date: 2023/8/6 08:33:26
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2023/8/6 08:33:26
 */

func init() {
	e := &Message{
		Simple: controller.NewSimple(
			controller.WithAuth(true),
			controller.WithModel(new(models.Message)),
			controller.WithSearch(new(dto.MessageSearch)),
			controller.WithModelProvider(actions.ModelProviderGorm),
		),
	}
	response.AppendController(e)
}

type Message struct {
	*controller.Simple
}

// GetAction get action
func (e *Message) GetAction(key string) response.Action {
	if key != response.Search {
		return nil
	}
	return e.Simple.GetAction(key)
}

func (e *Message) Other(r *gin.RouterGroup) {
	r.Use(middleware.Auth.MiddlewareFunc())
	r.GET("/message/list", middleware.Auth.MiddlewareFunc(), e.List)
	r.POST("/message/read", middleware.Auth.MiddlewareFunc(), e.Read)
}

func (e *Message) List(ctx *gin.Context) {
	api := response.Make(ctx)
	verify := middleware.GetVerify(ctx)
	list := make([]*models.Message, 0)
	err := gormdb.DB.Where("user_id", verify.GetUserID()).Where("read", false).Find(&list).Error
	if err != nil {
		api.Log.Error("list error", "err", err)
		api.Err(http.StatusInternalServerError, err.Error())
		return
	}
	api.OK(list)
}

func (e *Message) Read(ctx *gin.Context) {
	api := response.Make(ctx)
	req := &dto.MessageReadRequest{}
	err := api.Bind(req).Error
	if err != nil {
		api.Err(http.StatusUnprocessableEntity, err.Error())
		return
	}
	verify := middleware.GetVerify(ctx)
	err = gormdb.DB.Model(&models.Message{}).
		Where("user_id", verify.GetUserID()).
		Where("id", req.IDS).Update("read", true).Error
	if err != nil {
		api.Err(http.StatusInternalServerError, err.Error())
		return
	}
	api.OK(nil)
}
