package litellmops

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mss-boot-io/mss-boot-admin/admin/business"
	"gorm.io/gorm"
	"gorm.io/driver/postgres"
)

// registerBusinessRoutes mounts the module routes below the protected /api
// group. Every route is bound to a casbin-enforced permission.
func registerBusinessRoutes(group *gin.RouterGroup, runtime business.Runtime) error {
	if group == nil {
		return errors.New("litellmops protected route group is required")
	}
	authorizer, err := NewAdminAuthorizer(runtime.RequestDatabase, runtime.Principal)
	if err != nil {
		return err
	}
	handler := &requestHandler{runtime: runtime, authorizer: authorizer}

	resource := group.Group("/litellmops")
	resource.GET("/users", handler.secure(PermissionUserList, handler.listUsers))
	resource.GET("/users/:id", handler.secure(PermissionUserRead, handler.getUser))
	resource.GET("/keys", handler.secure(PermissionKeyList, handler.listKeys))
	resource.GET("/keys/:id", handler.secure(PermissionKeyRead, handler.getKey))
	resource.POST("/sync", handler.secure(PermissionSync, handler.sync))
	resource.GET("/bills", handler.secure(PermissionBills, handler.bills))
	return nil
}

type requestHandler struct {
	runtime    business.Runtime
	authorizer *AdminAuthorizer
}

// secure enforces the casbin permission before the handler runs.
func (handler *requestHandler) secure(permission string, next gin.HandlerFunc) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		if err := handler.authorizer.Authorize(ctx, permission); err != nil {
			status := http.StatusForbidden
			if errors.Is(err, ErrAuthenticationRequired) {
				status = http.StatusUnauthorized
			}
			ctx.AbortWithStatusJSON(status, gin.H{"error": err.Error()})
			return
		}
		next(ctx)
	}
}

func (handler *requestHandler) database(ctx *gin.Context) (*gorm.DB, bool) {
	db, ok := handler.runtime.RequestDatabase(ctx.Request.Context())
	if !ok || db == nil {
		ctx.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": "litellmops database unavailable"})
		return nil, false
	}
	return db, true
}

type pageQuery struct {
	Page     int
	PageSize int
}

func bindPage(ctx *gin.Context) pageQuery {
	page, _ := strconv.Atoi(ctx.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(ctx.DefaultQuery("page_size", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	return pageQuery{Page: page, PageSize: pageSize}
}

func parseTimeQuery(value string) *time.Time {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	if parsed, err := time.Parse(time.RFC3339, value); err == nil {
		utc := parsed.UTC()
		return &utc
	}
	if parsed, err := time.Parse("2006-01-02", value); err == nil {
		utc := parsed.UTC()
		return &utc
	}
	return nil
}

func (handler *requestHandler) listUsers(ctx *gin.Context) {
	db, ok := handler.database(ctx)
	if !ok {
		return
	}
	query := db.Model(&UserSnapshot{})
	if email := strings.TrimSpace(ctx.Query("email")); email != "" {
		query = query.Where("email LIKE ?", "%"+email+"%")
	}
	if role := strings.TrimSpace(ctx.Query("user_role")); role != "" {
		query = query.Where("user_role = ?", role)
	}
	var total int64
	if err := query.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	page := bindPage(ctx)
	items := make([]UserSnapshot, 0, page.PageSize)
	if err := query.Session(&gorm.Session{}).
		Order("spend DESC").
		Offset((page.Page - 1) * page.PageSize).
		Limit(page.PageSize).
		Find(&items).Error; err != nil {
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"items": items, "total": total, "page": page.Page, "page_size": page.PageSize})
}

func (handler *requestHandler) getUser(ctx *gin.Context) {
	db, ok := handler.database(ctx)
	if !ok {
		return
	}
	var user UserSnapshot
	if err := db.First(&user, "id = ?", ctx.Param("id")).Error; err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, gorm.ErrRecordNotFound) {
			status = http.StatusNotFound
		}
		ctx.AbortWithStatusJSON(status, gin.H{"error": "litellmops user snapshot not found"})
		return
	}
	keys := make([]KeySnapshot, 0)
	if err := db.Where("user_id = ?", user.UserID).Order("is_session_key ASC, spend DESC").Find(&keys).Error; err != nil {
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"user": user, "keys": keys})
}

func (handler *requestHandler) listKeys(ctx *gin.Context) {
	db, ok := handler.database(ctx)
	if !ok {
		return
	}
	query := db.Model(&KeySnapshot{})
	if alias := strings.TrimSpace(ctx.Query("alias")); alias != "" {
		query = query.Where("alias LIKE ?", "%"+alias+"%")
	}
	if email := strings.TrimSpace(ctx.Query("user_email")); email != "" {
		query = query.Where("user_email LIKE ?", "%"+email+"%")
	}
	if session := strings.TrimSpace(ctx.Query("is_session_key")); session != "" {
		query = query.Where("is_session_key = ?", session == "true" || session == "1")
	}
	var total int64
	if err := query.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	page := bindPage(ctx)
	items := make([]KeySnapshot, 0, page.PageSize)
	if err := query.Session(&gorm.Session{}).
		Order("is_session_key ASC, synced_at DESC").
		Offset((page.Page - 1) * page.PageSize).
		Limit(page.PageSize).
		Find(&items).Error; err != nil {
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"items": items, "total": total, "page": page.Page, "page_size": page.PageSize})
}

func (handler *requestHandler) getKey(ctx *gin.Context) {
	db, ok := handler.database(ctx)
	if !ok {
		return
	}
	var key KeySnapshot
	if err := db.First(&key, "id = ?", ctx.Param("id")).Error; err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, gorm.ErrRecordNotFound) {
			status = http.StatusNotFound
		}
		ctx.AbortWithStatusJSON(status, gin.H{"error": "litellmops key snapshot not found"})
		return
	}
	ctx.JSON(http.StatusOK, key)
}

func (handler *requestHandler) sync(ctx *gin.Context) {
	db, ok := handler.database(ctx)
	if !ok {
		return
	}
	client, err := NewClientFromEnv()
	if err != nil {
		ctx.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
		return
	}
	report, err := SyncSnapshots(ctx.Request.Context(), db, client)
	if err != nil {
		ctx.AbortWithStatusJSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, report)
}

func (handler *requestHandler) bills(ctx *gin.Context) {
	db, err := billsDatabase(func(dsn string) (*gorm.DB, error) {
		return gorm.Open(postgres.Open(dsn), &gorm.Config{})
	})
	if err != nil {
		ctx.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
		return
	}
	service, err := NewBillsService(db)
	if err != nil {
		ctx.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
		return
	}
	page := bindPage(ctx)
	result, err := service.List(ctx.Request.Context(), BillsQuery{
		Model:         ctx.Query("model"),
		Status:        ctx.Query("status"),
		KeyHashPrefix: ctx.Query("key_hash_prefix"),
		Since:         parseTimeQuery(ctx.Query("since")),
		Until:         parseTimeQuery(ctx.Query("until")),
		Page:          page.Page,
		PageSize:      page.PageSize,
	})
	if err != nil {
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, result)
}
