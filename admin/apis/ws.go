/*
 * @Author: lwnmengjing
 * @Date: 2024/5/1 15:06:37
 * @Last Modified by: lwnmengjing
 * @Last Modified time: 2024/5/1 15:06:37
 */

package apis

import (
	"context"
	"crypto/subtle"
	"errors"
	"net/http"
	"strings"
	"time"

	jwt "github.com/appleboy/gin-jwt/v2"
	"github.com/gin-gonic/gin"
	gws "github.com/gorilla/websocket"
	"github.com/mss-boot-io/mss-boot-admin/admin/center"
	"github.com/mss-boot-io/mss-boot-admin/admin/center/websocket"
	"github.com/mss-boot-io/mss-boot-admin/admin/config"
	"github.com/mss-boot-io/mss-boot-admin/admin/dto"
	"github.com/mss-boot-io/mss-boot-admin/admin/middleware"
	"github.com/mss-boot-io/mss-boot-admin/admin/models"
	"github.com/mss-boot-io/mss-boot-admin/admin/pkg/browsersecurity"
	"github.com/mss-boot-io/mss-boot-admin/admin/pkg/wsticket"
	"github.com/mss-boot-io/mss-boot-admin/admin/service"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/response"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/response/controller"
	"github.com/redis/go-redis/v9"
	"github.com/spf13/cast"
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
	tickets webSocketTicketStore
}

type webSocketTicketStore interface {
	Issue(context.Context, redis.UniversalClient, wsticket.Record, time.Duration) (string, wsticket.Record, error)
	Consume(context.Context, redis.UniversalClient, string) (wsticket.Record, error)
}

var defaultWebSocketTicketStore = wsticket.New()

func (e *WS) ticketStore() webSocketTicketStore {
	if e.tickets != nil {
		return e.tickets
	}
	return defaultWebSocketTicketStore
}

func (e *WS) GetAction(_ string) response.Action {
	return nil
}

func (e *WS) Other(r *gin.RouterGroup) {
	r.POST("/ws/tickets", response.AuthHandler, e.IssueTicket)
	r.GET("/ws/connect", e.Connect)
	r.GET("/ws/online", response.AuthHandler, requireRootManagement, e.Online)
}

// IssueTicket creates a short-lived credential for the V6 browser's
// Sec-WebSocket-Protocol header. It is available only from an authenticated
// browser cookie, never from PAT or bearer automation credentials.
// @Summary Issue a one-time WebSocket ticket
// @Tags websocket
// @Produce json
// @Success 201 {object} dto.WebSocketTicketResponse
// @Failure 401 {object} response.Response
// @Failure 403 {object} response.Response
// @Failure 503 {object} response.Response
// @Router /admin/api/ws/tickets [post]
func (e *WS) IssueTicket(ctx *gin.Context) {
	setWebSocketNoStoreHeaders(ctx)
	api := response.Make(ctx)
	if !middleware.BrowserSessionAvailable() {
		api.Err(http.StatusServiceUnavailable)
		return
	}
	if !middleware.RequestUsesBrowserSession(ctx) || !middleware.IsTrustedBrowserOrigin(ctx) {
		api.Err(http.StatusForbidden)
		return
	}
	verify := middleware.GetVerify(ctx)
	if verify == nil {
		api.Err(http.StatusUnauthorized)
		return
	}
	if middleware.IsPersonalAccessTokenVerifier(verify) {
		api.Err(http.StatusForbidden)
		return
	}
	origin, ok := browsersecurity.NormalizeOrigin(ctx.GetHeader("Origin"))
	if !ok {
		api.Err(http.StatusForbidden)
		return
	}
	sessionID := strings.TrimSpace(cast.ToString(jwt.ExtractClaims(ctx)["sid"]))
	if sessionID == "" {
		api.Err(http.StatusUnauthorized)
		return
	}
	cache := center.GetCache()
	if config.Cfg.Application.Mode == config.ModeProd && cache == nil {
		api.Err(http.StatusServiceUnavailable)
		return
	}
	ticket, record, err := e.ticketStore().Issue(ctx, cache, wsticket.Record{
		UserID:    verify.GetUserID(),
		RoleID:    verify.GetRoleID(),
		SessionID: sessionID,
		Origin:    origin,
	}, config.Cfg.Auth.BrowserSession.TicketTTL())
	if err != nil {
		api.Err(http.StatusServiceUnavailable)
		return
	}
	api.OK(dto.WebSocketTicketResponse{
		Ticket:    ticket,
		Protocol:  websocket.ApplicationSubprotocol,
		ExpiresAt: record.ExpiresAt,
	})
}

// Connect consumes the ticket from Sec-WebSocket-Protocol before restoring
// current database authority and upgrading the connection. Ticket validation
// is intentionally handler-owned because browser WebSocket APIs cannot set an
// Authorization header.
// @Summary Connect an authenticated browser WebSocket
// @Tags websocket
// @Router /admin/api/ws/connect [get]
func (e *WS) Connect(ctx *gin.Context) {
	setWebSocketNoStoreHeaders(ctx)
	api := response.Make(ctx)
	if !middleware.BrowserSessionAvailable() {
		api.Err(http.StatusServiceUnavailable)
		return
	}
	if !middleware.IsTrustedBrowserOrigin(ctx) {
		api.Err(http.StatusForbidden)
		return
	}
	ticket, ok := webSocketTicketFromProtocols(ctx.Request)
	if !ok {
		api.Err(http.StatusUnauthorized)
		return
	}
	cache := center.GetCache()
	if config.Cfg.Application.Mode == config.ModeProd && cache == nil {
		api.Err(http.StatusServiceUnavailable)
		return
	}
	record, err := e.ticketStore().Consume(ctx, cache, ticket)
	if errors.Is(err, wsticket.ErrNotFound) || errors.Is(err, wsticket.ErrExpired) {
		api.Err(http.StatusUnauthorized)
		return
	}
	if err != nil {
		api.Err(http.StatusServiceUnavailable)
		return
	}
	origin, validOrigin := browsersecurity.NormalizeOrigin(ctx.GetHeader("Origin"))
	if !validOrigin || subtle.ConstantTimeCompare([]byte(origin), []byte(record.Origin)) != 1 {
		api.Err(http.StatusUnauthorized)
		return
	}
	principal, err := models.LoadCurrentUserPrincipal(
		ctx,
		center.Default.GetDB(ctx, &models.User{}),
		record.UserID,
	)
	if err != nil || principal.GetRoleID() != record.RoleID {
		api.Err(http.StatusUnauthorized)
		return
	}
	lookup, err := service.Session.Lookup(
		ctx,
		center.Default.GetDB(ctx, &models.UserSession{}),
		record.SessionID,
	)
	if err != nil || lookup.Status != service.LookupActive ||
		lookup.Entry.UserID != record.UserID || lookup.Entry.RoleID != record.RoleID {
		api.Err(http.StatusUnauthorized)
		return
	}
	ctx.Set(config.Cfg.Auth.IdentityKey, principal)
	websocket.HandleWebSocket(ctx)
}

func webSocketTicketFromProtocols(request *http.Request) (string, bool) {
	if request == nil {
		return "", false
	}
	hasApplicationProtocol := false
	ticket := ""
	for _, protocol := range gws.Subprotocols(request) {
		switch {
		case protocol == websocket.ApplicationSubprotocol:
			hasApplicationProtocol = true
		case strings.HasPrefix(protocol, websocket.TicketProtocolPrefix):
			candidate := strings.TrimPrefix(protocol, websocket.TicketProtocolPrefix)
			if !wsticket.Valid(candidate) || ticket != "" {
				return "", false
			}
			ticket = candidate
		}
	}
	if !hasApplicationProtocol || ticket == "" {
		return "", false
	}
	return ticket, true
}

func setWebSocketNoStoreHeaders(ctx *gin.Context) {
	ctx.Header("Cache-Control", "no-store")
	ctx.Header("Pragma", "no-cache")
	ctx.Header("Referrer-Policy", "no-referrer")
}

func (e *WS) Online(ctx *gin.Context) {
	api := response.Make(ctx)
	api.OK(websocket.GetOnlineInfo())
}
