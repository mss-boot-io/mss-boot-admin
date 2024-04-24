package apis

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/mss-boot-io/mss-boot-admin/service"
	"github.com/mss-boot-io/mss-boot/pkg/response"
	"github.com/mss-boot-io/mss-boot/pkg/response/controller"
)

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2024/3/23 23:40:31
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2024/3/23 23:40:31
 */

func init() {
	e := &Monitor{
		Simple: controller.NewSimple(),
	}
	response.AppendController(e)
}

type Monitor struct {
	*controller.Simple
	service service.Monitor
}

func (e *Monitor) GetAction(string) response.Action {
	return nil
}

func (e *Monitor) Other(r *gin.RouterGroup) {
	r.GET("/monitor", response.AuthHandler, e.Monitor)
}

// Monitor 获取监控信息
// @Summary 获取监控信息
// @Description 获取监控信息
// @Tags monitor
// @Accept application/json
// @Product application/json
// @Success 200 {object} dto.MonitorResponse
// @Router /admin/api/monitor [get]
// @Security Bearer
func (e *Monitor) Monitor(ctx *gin.Context) {
	api := response.Make(ctx)
	resp, err := e.service.Monitor(ctx)
	if err != nil {
		api.AddError(err).Log.Error("get monitor error")
		api.Err(http.StatusInternalServerError)
		return
	}
	api.OK(resp)
}
