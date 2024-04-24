package apis

import (
	"net/http"

	"github.com/mss-boot-io/mss-boot-admin/service"

	"github.com/gin-gonic/gin"
	"github.com/mss-boot-io/mss-boot-admin/dto"
	"github.com/mss-boot-io/mss-boot/pkg/response"
	"github.com/mss-boot-io/mss-boot/pkg/response/controller"
)

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2024/1/12 17:48:43
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2024/1/12 17:48:43
 */

func init() {
	e := &Statistics{
		Simple: controller.NewSimple(),
	}
	response.AppendController(e)
}

type Statistics struct {
	*controller.Simple
	service service.Statistics
}

func (*Statistics) GetAction(string) response.Action {
	return nil
}

func (e *Statistics) Other(r *gin.RouterGroup) {
	r.GET("/statistics/:name", response.AuthHandler, e.Get)
}

// Get 获取统计
// @Summary 获取统计
// @Description 获取统计
// @Tags statistics
// @Accept application/json
// @Product application/json
// @Param name path string true "name"
// @Success 200 {object} dto.StatisticsGetResponse
// @Router /admin/api/statistics/{name} [get]
// @Security Bearer
func (e *Statistics) Get(ctx *gin.Context) {
	api := response.Make(ctx)
	req := &dto.StatisticsGetRequest{}
	if err := api.Bind(req).Error; err != nil {
		api.Err(http.StatusUnprocessableEntity)
		return
	}
	result, err := e.service.Get(ctx, req.Name)
	if err != nil {
		api.Err(http.StatusInternalServerError)
		return
	}
	api.OK(result)
}
