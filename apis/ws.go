/*
 * @Author: lwnmengjing
 * @Date: 2024/5/1 15:06:37
 * @Last Modified by: lwnmengjing
 * @Last Modified time: 2024/5/1 15:06:37
 */

package apis

import (
	"github.com/gin-gonic/gin"
	"github.com/mss-boot-io/mss-boot-admin/center/websocket"
	"github.com/mss-boot-io/mss-boot/pkg/response"
	"github.com/mss-boot-io/mss-boot/pkg/response/controller"
)

func init() {
	e := &WS{
		Simple: controller.NewSimple(),
	}
	response.AppendController(e)

	go websocket.GetHub().Run()
}

type WS struct {
	*controller.Simple
}

func (e *WS) GetAction(_ string) response.Action {
	return nil
}

func (e *WS) Other(r *gin.RouterGroup) {
	r.GET("/ws/connect", response.AuthHandler, e.Connect)
	r.GET("/ws/online", response.AuthHandler, e.Online)
}

func (e *WS) Connect(ctx *gin.Context) {
	websocket.HandleWebSocket(ctx)
}

func (e *WS) Online(ctx *gin.Context) {
	api := response.Make(ctx)
	api.OK(websocket.GetOnlineInfo())
}
