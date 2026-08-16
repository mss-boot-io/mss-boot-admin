package websocket

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/mss-boot-io/mss-boot-admin/admin/config"
	"github.com/mss-boot-io/mss-boot-admin/admin/models"
	"github.com/mss-boot-io/mss-boot-admin/admin/pkg/browsersecurity"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/response"
)

const (
	ApplicationSubprotocol = "mss.v1"
	TicketProtocolPrefix   = "mss.ticket."
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	Subprotocols:    []string{ApplicationSubprotocol},
	CheckOrigin:     IsTrustedOrigin,
}

func IsTrustedOrigin(r *http.Request) bool {
	if r == nil {
		return false
	}
	return browsersecurity.IsTrustedOrigin(
		r.Header.Get("Origin"),
		config.Cfg.Application.Origin,
		config.Cfg.CORS.AllowOrigins,
	)
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
		Event:     EventConnected,
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

// BroadcastAuthorizationRevision emits only a monotonic decimal revision.
// Browser clients must use it as a reload hint and fetch their own current
// identity and authorized menu over the protected HTTP API.
func BroadcastAuthorizationRevision(revision uint64) bool {
	message := authorizationRevisionResponse(revision)
	if message == nil {
		return false
	}
	return GetHub().TryBroadcast(message)
}

func authorizationRevisionResponse(revision uint64) *WResponse {
	if revision == 0 {
		return nil
	}
	return &WResponse{
		Event: EventAuthorization,
		Code:  200,
		Data: gin.H{
			"revision": strconv.FormatUint(revision, 10),
		},
		Timestamp: time.Now().Unix(),
	}
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
