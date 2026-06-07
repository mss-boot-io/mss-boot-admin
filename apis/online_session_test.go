package apis

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/mss-boot-io/mss-boot-admin/models"
	"github.com/mss-boot-io/mss-boot-admin/pkg/sessioncache"
	"github.com/mss-boot-io/mss-boot-admin/service"
)

func setupOnlineSessionTest(t *testing.T) (*gin.Engine, *gorm.DB, string) {
	gin.SetMode(gin.TestMode)

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	assert.NoError(t, err)
	assert.NoError(t, db.AutoMigrate(&models.UserSession{}, &models.AuditLog{}))

	mr, err := miniredis.Run()
	assert.NoError(t, err)
	t.Cleanup(mr.Close)

	service.Session.SetCache(sessioncache.New(redis.NewClient(&redis.Options{Addr: mr.Addr()})))

	ctx := context.Background()
	sid, err := service.Session.Create(ctx, db, service.CreateSessionInput{
		UserID: "u1", Username: "alice", RoleID: "r1", IP: "1.1.1.1", UserAgent: "ua", TTL: time.Hour,
	})
	assert.NoError(t, err)

	r := gin.New()
	api := &OnlineSessionAPI{
		db:            db,
		actorResolver: func(c *gin.Context) (string, string) { return "admin", "admin" },
		sidExtractor:  func(c *gin.Context) string { return sid },
	}
	// 测试用裸 group，不引入项目的 AuthHandler。先短路 response.AuthHandler。
	g := r.Group("/admin/api")
	g.GET("/online-sessions", api.List)
	g.GET("/online-sessions/:id", api.Get)
	g.DELETE("/online-sessions/:id", api.RevokeBySID)
	g.DELETE("/online-sessions/user/:userID", api.RevokeByUserID)
	g.POST("/online-sessions/logout", api.Logout)
	return r, db, sid
}

func TestOnlineSessionList(t *testing.T) {
	r, _, _ := setupOnlineSessionTest(t)
	req := httptest.NewRequest(http.MethodGet, "/admin/api/online-sessions?status=active", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]any
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	// just sanity: should contain data array
	assert.Contains(t, resp, "data")
}

func TestOnlineSessionRevokeBySID(t *testing.T) {
	r, db, sid := setupOnlineSessionTest(t)
	req := httptest.NewRequest(http.MethodDelete, "/admin/api/online-sessions/"+sid, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNoContent, w.Code)

	var row models.UserSession
	assert.NoError(t, db.First(&row, "id = ?", sid).Error)
	assert.True(t, row.Revoked)
	assert.Equal(t, models.SessionRevokeForceBySession, row.RevokeReason)

	var logs []models.AuditLog
	assert.NoError(t, db.Find(&logs).Error)
	assert.GreaterOrEqual(t, len(logs), 1)
	assert.Equal(t, models.AuditLogTypeSecurity, logs[0].Type)
}

func TestOnlineSessionRevokeByUser(t *testing.T) {
	r, db, _ := setupOnlineSessionTest(t)
	_, err := service.Session.Create(context.Background(), db, service.CreateSessionInput{
		UserID: "u1", Username: "alice", RoleID: "r1", TTL: time.Hour,
	})
	assert.NoError(t, err)

	req := httptest.NewRequest(http.MethodDelete, "/admin/api/online-sessions/user/u1", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNoContent, w.Code)

	var count int64
	db.Model(&models.UserSession{}).Where("user_id = ? AND revoked = ?", "u1", true).Count(&count)
	assert.EqualValues(t, 2, count)
}
