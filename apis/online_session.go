package apis

import (
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
	db            *gorm.DB
	actorResolver func(*gin.Context) (string, string)
	sidExtractor  func(*gin.Context) string
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
// @Param status query string false "active|revoked|expired"
// @Param userID query string false "用户ID"
// @Param username query string false "用户名"
// @Param ip query string false "登录IP"
// @Param current query int false "当前页"
// @Param pageSize query int false "每页数量"
// @Success 200 {object} response.Page{data=[]models.UserSession}
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
	if q.Status == "" {
		q.Status = "active"
	}

	db := e.getDB(c).Model(&models.UserSession{})
	now := time.Now()
	switch q.Status {
	case "active":
		db = db.Where("revoked = ? AND expired_at > ?", false, now)
	case "revoked":
		db = db.Where("revoked = ?", true)
	case "expired":
		db = db.Where("revoked = ? AND expired_at <= ?", false, now)
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
	if err := e.getDB(c).Where("id = ?", c.Param("id")).First(&row).Error; err != nil {
		api.AddError(err).Err(http.StatusNotFound)
		return
	}
	api.OK(row)
}

// RevokeBySID 强制下线指定会话
// @Summary 强制下线指定会话
// @Tags OnlineSession
// @Param id path string true "session id"
// @Success 200
// @Router /admin/api/online-sessions/{id} [delete]
// @Security Bearer
func (e *OnlineSessionAPI) RevokeBySID(c *gin.Context) {
	api := response.Make(c)
	sid := c.Param("id")
	actorID, actorName := e.actor(c)

	row, err := service.Session.RevokeBySID(c, e.getDB(c), sid, actorID, models.SessionRevokeForceBySession)
	if err != nil {
		api.AddError(err).Err(http.StatusInternalServerError)
		return
	}
	_ = service.Audit.LogSecurity(e.getDB(c), "force_logout", "session:"+sid, actorID, actorName,
		c.ClientIP(), c.GetHeader("User-Agent"), "force-by-session")

	api.OK(gin.H{"id": row.ID, "userID": row.UserID, "revokedAt": row.RevokedAt})
}

// RevokeByUserID 强制下线该用户全部会话
// @Summary 强制下线该用户全部会话
// @Tags OnlineSession
// @Param userID path string true "user id"
// @Success 200
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
	_ = service.Audit.LogSecurity(e.getDB(c), "force_logout", "user:"+uid, actorID, actorName,
		c.ClientIP(), c.GetHeader("User-Agent"), "force-by-user")

	api.OK(gin.H{"affected": n, "userID": uid})
}

// Logout 当前用户自登出
// @Summary 当前用户自登出
// @Tags OnlineSession
// @Success 200
// @Router /admin/api/online-sessions/logout [post]
// @Security Bearer
func (e *OnlineSessionAPI) Logout(c *gin.Context) {
	api := response.Make(c)
	v := middleware.GetVerify(c)
	if v == nil {
		api.Err(http.StatusUnauthorized)
		return
	}
	sid := e.extractSID(c)
	if sid == "" {
		api.Err(http.StatusBadRequest)
		return
	}
	if _, err := service.Session.RevokeBySID(c, e.getDB(c), sid, v.GetUserID(), models.SessionRevokeLogout); err != nil {
		api.AddError(err).Err(http.StatusInternalServerError)
		return
	}
	_ = service.Audit.LogSecurity(e.getDB(c), "logout", "session:"+sid, v.GetUserID(), v.GetUsername(),
		c.ClientIP(), c.GetHeader("User-Agent"), "self-logout")
	api.OK(gin.H{"ok": true})
}
