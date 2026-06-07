package apis

import (
	"errors"
	"log/slog"
	"net/http"
	"time"

	jwt "github.com/appleboy/gin-jwt/v2"
	"github.com/gin-gonic/gin"
	"github.com/mss-boot-io/mss-boot/pkg/response"
	"github.com/mss-boot-io/mss-boot/pkg/response/actions"
	"github.com/mss-boot-io/mss-boot/pkg/response/controller"
	"github.com/spf13/cast"
	"gorm.io/gorm"

	"github.com/mss-boot-io/mss-boot-admin/center"
	"github.com/mss-boot-io/mss-boot-admin/middleware"
	"github.com/mss-boot-io/mss-boot-admin/models"
	"github.com/mss-boot-io/mss-boot-admin/service"
)

func init() {
	e := &OnlineSessionAPI{
		Simple: controller.NewSimple(
			controller.WithAuth(true),
			controller.WithModel(new(models.UserSession)),
			controller.WithModelProvider(actions.ModelProviderGorm),
		),
	}
	response.AppendController(e)
}

type OnlineSessionAPI struct {
	*controller.Simple
	db             *gorm.DB
	actorResolver  func(*gin.Context) (string, string)
	sidExtractor   func(*gin.Context) string
	verifyResolver func(*gin.Context) (userID, username string, ok bool)
}

func (e *OnlineSessionAPI) GetAction(_ string) response.Action { return nil }

func (e *OnlineSessionAPI) Other(r *gin.RouterGroup) {
	r.GET("/online-sessions", response.AuthHandler, e.List)
	r.GET("/online-sessions/:id", response.AuthHandler, e.Get)
	r.DELETE("/online-sessions/:id", response.AuthHandler, e.RevokeBySID)
	r.DELETE("/online-sessions/user/:userID", response.AuthHandler, e.RevokeByUserID)
	r.POST("/online-sessions/logout", response.AuthHandler, e.Logout)
}

func (e *OnlineSessionAPI) getDB(c *gin.Context) *gorm.DB {
	if e.db != nil {
		return e.db
	}
	return center.Default.GetDB(c, &models.UserSession{})
}

func (e *OnlineSessionAPI) actor(c *gin.Context) (string, string) {
	if e.actorResolver != nil {
		return e.actorResolver(c)
	}
	v := middleware.GetVerify(c)
	if v == nil {
		return "", ""
	}
	return v.GetUserID(), v.GetUsername()
}

func (e *OnlineSessionAPI) extractSID(c *gin.Context) string {
	if e.sidExtractor != nil {
		return e.sidExtractor(c)
	}
	claims := jwt.ExtractClaims(c)
	return cast.ToString(claims["sid"])
}

type onlineSessionListQuery struct {
	UserID   string `form:"userID"`
	Username string `form:"username"`
	IP       string `form:"ip"`
	Status   string `form:"status"`
	Current  int    `form:"current"`
	PageSize int    `form:"pageSize"`
}

// List 在线会话列表
// @Summary 在线会话列表
// @Tags OnlineSession
// @Accept application/json
// @Produce application/json
// @Param status query string false "active|revoked|expired|all (default active)"
// @Param userID query string false "用户ID"
// @Param username query string false "用户名"
// @Param ip query string false "登录IP"
// @Param current query int false "当前页"
// @Param pageSize query int false "每页数量"
// @Success 200 {object} response.Page{data=[]models.UserSession}
// @Failure 400 {object} response.Response "unknown status value"
// @Router /admin/api/online-sessions [get]
// @Security Bearer
func (e *OnlineSessionAPI) List(c *gin.Context) {
	api := response.Make(c)
	var q onlineSessionListQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		api.AddError(err).Err(http.StatusBadRequest)
		return
	}
	if q.Current <= 0 {
		q.Current = 1
	}
	if q.PageSize <= 0 {
		q.PageSize = 20
	}

	db := e.getDB(c).Model(&models.UserSession{})
	now := time.Now()
	switch q.Status {
	case "", "active":
		db = db.Where("revoked = ? AND expired_at > ?", false, now)
	case "revoked":
		db = db.Where("revoked = ?", true)
	case "expired":
		db = db.Where("revoked = ? AND expired_at <= ?", false, now)
	case "all":
		// no status filter
	default:
		api.AddError(errors.New("unknown status: " + q.Status)).Err(http.StatusBadRequest)
		return
	}
	if q.UserID != "" {
		db = db.Where("user_id = ?", q.UserID)
	}
	if q.Username != "" {
		db = db.Where("username LIKE ?", "%"+q.Username+"%")
	}
	if q.IP != "" {
		db = db.Where("ip = ?", q.IP)
	}

	var total int64
	if err := db.Count(&total).Error; err != nil {
		api.AddError(err).Err(http.StatusInternalServerError)
		return
	}
	var rows []models.UserSession
	if err := db.Order("last_seen_at DESC").
		Offset((q.Current - 1) * q.PageSize).Limit(q.PageSize).
		Find(&rows).Error; err != nil {
		api.AddError(err).Err(http.StatusInternalServerError)
		return
	}

	api.PageOK(rows, total, int64(q.Current), int64(q.PageSize))
}

// Get 在线会话详情
// @Summary 在线会话详情
// @Tags OnlineSession
// @Param id path string true "session id"
// @Success 200 {object} models.UserSession
// @Router /admin/api/online-sessions/{id} [get]
// @Security Bearer
func (e *OnlineSessionAPI) Get(c *gin.Context) {
	api := response.Make(c)
	var row models.UserSession
	err := e.getDB(c).Where("id = ?", c.Param("id")).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		api.Err(http.StatusNotFound)
		return
	}
	if err != nil {
		api.AddError(err).Err(http.StatusInternalServerError)
		return
	}
	api.OK(row)
}

// RevokeBySID 强制下线指定会话
// @Summary 强制下线指定会话
// @Tags OnlineSession
// @Param id path string true "session id"
// @Success 204 "session revoked, no body"
// @Failure 404 "session not found"
// @Router /admin/api/online-sessions/{id} [delete]
// @Security Bearer
func (e *OnlineSessionAPI) RevokeBySID(c *gin.Context) {
	api := response.Make(c)
	sid := c.Param("id")
	actorID, actorName := e.actor(c)

	_, err := service.Session.RevokeBySID(c, e.getDB(c), sid, actorID, models.SessionRevokeForceBySession)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		api.Err(http.StatusNotFound)
		return
	}
	if err != nil {
		api.AddError(err).Err(http.StatusInternalServerError)
		return
	}
	if err := service.Audit.LogSecurity(e.getDB(c), "force_logout", "session:"+sid, actorID, actorName,
		c.ClientIP(), c.GetHeader("User-Agent"), "force-by-session"); err != nil {
		slog.Warn("audit log force_logout failed", "sid", sid, "err", err)
	}

	// DELETE single resource → 204 No Content, no body (per RFC 7231 §6.3.5
	// and mss-boot's response.OK(nil) convention).
	api.OK(nil)
}

// RevokeByUserID 强制下线该用户全部会话
// @Summary 强制下线该用户全部会话
// @Tags OnlineSession
// @Param userID path string true "user id"
// @Success 200 {object} response.Response{data=object{affected=int,userID=string}} "batch revoke result"
// @Router /admin/api/online-sessions/user/{userID} [delete]
// @Security Bearer
func (e *OnlineSessionAPI) RevokeByUserID(c *gin.Context) {
	api := response.Make(c)
	uid := c.Param("userID")
	actorID, actorName := e.actor(c)

	n, err := service.Session.RevokeByUserID(c, e.getDB(c), uid, actorID, models.SessionRevokeForceByUser)
	if err != nil {
		api.AddError(err).Err(http.StatusInternalServerError)
		return
	}
	if err := service.Audit.LogSecurity(e.getDB(c), "force_logout", "user:"+uid, actorID, actorName,
		c.ClientIP(), c.GetHeader("User-Agent"), "force-by-user"); err != nil {
		slog.Warn("audit log force_logout failed", "userID", uid, "err", err)
	}

	// Batch revoke returns a real payload (frontend reads `affected`), so we
	// emit 200 explicitly instead of going through api.OK which would map
	// DELETE to 204 and silently drop the body.
	c.AbortWithStatusJSON(http.StatusOK, gin.H{"affected": n, "userID": uid})
}

// Logout 当前用户自登出
// @Summary 当前用户自登出
// @Tags OnlineSession
// @Success 201 "session revoked, no body"
// @Failure 400 "missing sid in token"
// @Failure 401 "unauthenticated"
// @Failure 404 "session not found"
// @Router /admin/api/online-sessions/logout [post]
// @Security Bearer
func (e *OnlineSessionAPI) Logout(c *gin.Context) {
	api := response.Make(c)
	userID, username, ok := e.resolveVerify(c)
	if !ok {
		api.Err(http.StatusUnauthorized)
		return
	}
	sid := e.extractSID(c)
	if sid == "" {
		api.Err(http.StatusBadRequest)
		return
	}
	if _, err := service.Session.RevokeBySID(c, e.getDB(c), sid, userID, models.SessionRevokeLogout); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			api.Err(http.StatusNotFound)
			return
		}
		api.AddError(err).Err(http.StatusInternalServerError)
		return
	}
	if err := service.Audit.LogSecurity(e.getDB(c), "logout", "session:"+sid, userID, username,
		c.ClientIP(), c.GetHeader("User-Agent"), "self-logout"); err != nil {
		slog.Warn("audit log logout failed", "sid", sid, "err", err)
	}
	// Logout doesn't create a resource; mss-boot's OK on POST maps to 201
	// with no body which is the contract used elsewhere for "operation
	// accepted, nothing to return".
	api.OK(nil)
}

func (e *OnlineSessionAPI) resolveVerify(c *gin.Context) (string, string, bool) {
	if e.verifyResolver != nil {
		return e.verifyResolver(c)
	}
	v := middleware.GetVerify(c)
	if v == nil {
		return "", "", false
	}
	return v.GetUserID(), v.GetUsername(), true
}
