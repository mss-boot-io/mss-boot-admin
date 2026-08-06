package apis

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mss-boot-io/mss-boot-admin/admin/dto"
	"github.com/mss-boot-io/mss-boot-admin/admin/service"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/response"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/response/controller"
)

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2024/3/23 23:40:31
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2024/3/23 23:40:31
 */

func init() {
	response.AppendController(newMonitorController(service.DefaultMonitor))
}

type monitorSnapshotService interface {
	Snapshot(historyLimit int) (*dto.MonitorResponse, error)
	SampleInterval() time.Duration
}

type Monitor struct {
	*controller.Simple
	service monitorSnapshotService
}

func newMonitorController(monitorService monitorSnapshotService) *Monitor {
	return &Monitor{
		Simple:  controller.NewSimple(),
		service: monitorService,
	}
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
// @Param historyLimit query int false "Recent samples to return" minimum(0) maximum(120) default(120)
// @Success 200 {object} dto.MonitorResponse
// @Failure 400 {object} response.Response
// @Failure 503 {object} response.Response
// @Router /admin/api/monitor [get]
// @Security Bearer
func (e *Monitor) Monitor(ctx *gin.Context) {
	api := response.Make(ctx)
	historyLimit, err := monitorHistoryLimit(ctx)
	if err != nil {
		api.AddError(err)
		api.Err(http.StatusBadRequest)
		return
	}
	resp, err := e.service.Snapshot(historyLimit)
	if err != nil {
		if errors.Is(err, service.ErrMonitorNotReady) {
			retryAfter := (e.service.SampleInterval() + time.Second - 1) / time.Second
			if retryAfter < 1 {
				retryAfter = 1
			}
			ctx.Header("Retry-After", strconv.FormatInt(int64(retryAfter), 10))
			api.AddError(err)
			api.Err(http.StatusServiceUnavailable)
			return
		}
		if errors.Is(err, service.ErrInvalidMonitorHistoryLimit) {
			api.AddError(err)
			api.Err(http.StatusBadRequest)
			return
		}
		api.AddError(err).Log.Error("get monitor error")
		api.Err(http.StatusInternalServerError)
		return
	}
	api.OK(resp)
}

func monitorHistoryLimit(ctx *gin.Context) (int, error) {
	raw, exists := ctx.GetQuery("historyLimit")
	if !exists {
		return service.DefaultMonitorHistorySize, nil
	}
	limit, err := strconv.Atoi(raw)
	if err != nil || limit < 0 || limit > service.MaxMonitorHistorySize {
		return 0, service.ErrInvalidMonitorHistoryLimit
	}
	return limit, nil
}
