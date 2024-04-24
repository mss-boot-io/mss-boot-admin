package apis

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/mss-boot-io/mss-boot-admin/middleware"
	"github.com/mss-boot-io/mss-boot-admin/service"
	"github.com/mss-boot-io/mss-boot/pkg/response"
	"github.com/mss-boot-io/mss-boot/pkg/response/controller"
)

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2024/3/29 00:36:27
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2024/3/29 00:36:27
 */

func init() {
	e := &Storage{
		Simple: controller.NewSimple(),
	}
	response.AppendController(e)
}

type Storage struct {
	*controller.Simple
	service service.Storage
}

func (*Storage) GetKey() string {
	return "storage"
}

func (*Storage) GetAction(string) response.Action {
	return nil
}

func (e *Storage) Other(r *gin.RouterGroup) {
	r.POST("/storage/upload", middleware.Auth.MiddlewareFunc(), e.Upload)
}

func (e *Storage) Upload(ctx *gin.Context) {
	api := response.Make(ctx)
	verify := middleware.GetVerify(ctx)
	file, err := ctx.FormFile("file")
	if err != nil {
		api.AddError(err).Log.Error("FormFile error")
		api.Err(http.StatusInternalServerError)
		return
	}
	u, err := e.service.Upload(ctx, file, verify.GetTenantID(), verify.GetUserID())
	if err != nil {
		api.AddError(err).Log.Error("upload error")
		api.Err(http.StatusInternalServerError)
		return
	}
	api.OK(u)
}
