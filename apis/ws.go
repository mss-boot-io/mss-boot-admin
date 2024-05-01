/*
 * @Author: lwnmengjing
 * @Date: 2024/5/1 15:06:37
 * @Last Modified by: lwnmengjing
 * @Last Modified time: 2024/5/1 15:06:37
 */

package apis

import (
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/mss-boot-io/mss-boot/pkg/response"
	"github.com/mss-boot-io/mss-boot/pkg/response/controller"
	"net/http"
)

func init() {
	e := &WS{
		Simple: controller.NewSimple(),
	}
	response.AppendController(e)
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	Subprotocols: []string{
		"Sec-WebSocket-Extensions",
	},
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type WS struct {
	*controller.Simple
}

func (e *WS) GetAction(_ string) response.Action {
	return nil
}

func (e *WS) Other(r *gin.RouterGroup) {
	r.GET("/ws/event", e.Event)
}

// Event 长连接事件
func (e *WS) Event(ctx *gin.Context) {
	api := response.Make(ctx)
	conn, err := upgrader.Upgrade(ctx.Writer, ctx.Request, nil)
	if err != nil {
		api.AddError(err).Log.Error("websocket upgrade error")
		api.Err(http.StatusInternalServerError)
		return
	}
	err = conn.WriteJSON(gin.H{
		"code": 200,
	})
	if err != nil {
		api.AddError(err).Log.Error("websocket write error")
	}
	return
}
