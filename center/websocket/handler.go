package websocket

import (
	"encoding/json"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/mss-boot-io/mss-boot-admin/models"
	"github.com/mss-boot-io/mss-boot/pkg/response"
	"net/http"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type EventHandler func(*Client, json.RawMessage)

var eventHandlers = make(map[EventType]EventHandler)

func RegisterEventHandler(event EventType, handler EventHandler) {
	eventHandlers[event] = handler
}

func init() {
	RegisterEventHandler(EventPong, handlePong)
	RegisterEventHandler(EventJoin, handleJoin)
	RegisterEventHandler(EventQuit, handleQuit)
}

func handlePong(c *Client, data json.RawMessage) {
	c.HeartbeatTime = time.Now()
}

func handleJoin(c *Client, data json.RawMessage) {
	c.SendMsg(&WResponse{
		Event:     EventJoin,
		Code:      200,
		Timestamp: time.Now().Unix(),
	})
}

func handleQuit(c *Client, data json.RawMessage) {
	c.SendMsg(&WResponse{
		Event:     EventQuit,
		Code:      200,
		Timestamp: time.Now().Unix(),
	})
}

func HandleWebSocket(ctx *gin.Context) {
	api := response.Make(ctx)

	verify := response.VerifyHandler(ctx)
	if verify == nil {
		api.Err(http.StatusUnauthorized)
		return
	}

	userID := verify.GetUserID()
	if userID == "" {
		api.Err(http.StatusUnauthorized)
		return
	}

	conn, err := upgrader.Upgrade(ctx.Writer, ctx.Request, nil)
	if err != nil {
		api.AddError(err).Log.Error("websocket upgrade error")
		return
	}

	clientID := GenerateClientID()
	client := NewClient(
		clientID,
		userID,
		conn,
		ctx.ClientIP(),
		ctx.GetHeader("User-Agent"),
	)

	hub := GetHub()
	hub.Register(client)

	go client.WritePump()
	go client.ReadPump(handleMessage)

	client.SendMsg(&WResponse{
		Event:     "connected",
		Code:      200,
		Data:      gin.H{"clientId": clientID},
		Timestamp: time.Now().Unix(),
	})
}

func handleMessage(c *Client, req *WRequest) {
	if handler, ok := eventHandlers[req.Event]; ok {
		data, _ := json.Marshal(req.Data)
		handler(c, data)
		return
	}

	c.SendMsg(&WResponse{
		Event:     req.Event,
		Code:      400,
		ErrorMsg:  "unknown event",
		Timestamp: time.Now().Unix(),
	})
}

func SendNotificationToUser(userID string, notice *models.Notice) bool {
	hub := GetHub()
	return hub.SendToUserDirect(userID, &WResponse{
		Event: EventNotify,
		Code:  200,
		Data: gin.H{
			"id":          notice.ID,
			"type":        notice.Type,
			"title":       notice.Title,
			"description": notice.Description,
			"createdAt":   notice.CreatedAt,
		},
		Timestamp: time.Now().Unix(),
	}) > 0
}

func SendKickToUser(userID string, reason string) {
	hub := GetHub()
	hub.SendToUserDirect(userID, &WResponse{
		Event:     EventKick,
		Code:      200,
		Data:      gin.H{"reason": reason},
		Timestamp: time.Now().Unix(),
	})
}

func BroadcastNotification(notice *models.Notice) {
	hub := GetHub()
	hub.Broadcast(&WResponse{
		Event: EventNotify,
		Code:  200,
		Data: gin.H{
			"id":          notice.ID,
			"type":        notice.Type,
			"title":       notice.Title,
			"description": notice.Description,
			"createdAt":   notice.CreatedAt,
		},
		Timestamp: time.Now().Unix(),
	})
}

func GetOnlineInfo() gin.H {
	hub := GetHub()
	return gin.H{
		"onlineConnections": hub.GetOnlineCount(),
		"onlineUsers":       hub.GetOnlineUserCount(),
	}
}
