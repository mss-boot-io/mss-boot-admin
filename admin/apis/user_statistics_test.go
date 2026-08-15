package apis

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mss-boot-io/mss-boot-admin/admin/center"
	"github.com/mss-boot-io/mss-boot-admin/admin/config"
	"github.com/mss-boot-io/mss-boot-admin/admin/models"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/config/gormdb"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/response"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestUserControllerUpdatesStatisticsAfterMutationCommit(t *testing.T) {
	dsn := filepath.Join(t.TempDir(), "user-statistics.db") + "?_busy_timeout=100&_journal_mode=WAL"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		t.Fatalf("open user statistics database: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("unwrap user statistics database: %v", err)
	}
	sqlDB.SetMaxOpenConns(4)
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := db.AutoMigrate(&models.User{}, &models.Statistics{}); err != nil {
		t.Fatalf("migrate user statistics schema: %v", err)
	}

	previousDB := gormdb.DB
	previousStatistics := center.GetStatistics()
	previousAuth := response.AuthHandler
	previousIdentityKey := config.Cfg.Auth.IdentityKey
	gormdb.DB = db
	center.SetStatistics(&models.Statistics{})
	config.Cfg.Auth.IdentityKey = "test-user-statistics-identity"
	root := &models.User{UserLogin: models.UserLogin{Role: &models.Role{Root: true}}}
	root.ID = "root-user"
	response.AuthHandler = func(c *gin.Context) {
		c.Set(config.Cfg.Auth.IdentityKey, root)
		c.Next()
	}
	t.Cleanup(func() {
		gormdb.DB = previousDB
		center.SetStatistics(previousStatistics)
		response.AuthHandler = previousAuth
		config.Cfg.Auth.IdentityKey = previousIdentityKey
	})

	controller := newUserController()
	controlAction := controller.GetAction(response.Control)
	if controlAction == nil {
		t.Fatal("user controller did not expose the control action")
	}
	deleteAction := controller.GetAction(response.Delete)
	if deleteAction == nil {
		t.Fatal("user controller did not expose the delete action")
	}
	gin.SetMode(gin.TestMode)
	router := gin.New()
	routes := router.Group("/admin/api/"+controller.Path(), controller.Handlers()...)
	routes.POST("", controlAction.Handler()...)
	routes.DELETE("/:"+controller.GetKey(), deleteAction.Handler()...)

	request := httptest.NewRequest(
		http.MethodPost,
		"/admin/api/users",
		bytes.NewBufferString(`{"username":"statistics-user","password":"Statistics123!"}`),
	)
	request.Header.Set("Content-Type", "application/json")
	result := httptest.NewRecorder()
	started := time.Now()
	router.ServeHTTP(result, request)
	elapsed := time.Since(started)

	if result.Code < http.StatusOK || result.Code >= http.StatusMultipleChoices {
		t.Fatalf("create response = %d: %s", result.Code, result.Body.String())
	}
	if elapsed >= time.Second {
		t.Fatalf("user create took %s; statistics likely contended with the writer transaction", elapsed)
	}

	var statistic models.Statistics
	if err := db.Where(
		"name = ? AND type = ? AND time = ?",
		(&models.User{}).StatisticsName(),
		(&models.User{}).StatisticsType(),
		(&models.User{}).StatisticsTime(),
	).Take(&statistic).Error; err != nil {
		t.Fatalf("load user statistic: %v", err)
	}
	if statistic.Value != (&models.User{}).StatisticsStep() {
		t.Fatalf("user statistic value = %d, want %d", statistic.Value, (&models.User{}).StatisticsStep())
	}

	var created models.User
	if err := db.Where("username = ?", "statistics-user").Take(&created).Error; err != nil {
		t.Fatalf("load created user: %v", err)
	}
	deleteRequest := httptest.NewRequest(http.MethodDelete, "/admin/api/users/"+created.ID, nil)
	deleteResult := httptest.NewRecorder()
	router.ServeHTTP(deleteResult, deleteRequest)
	if deleteResult.Code < http.StatusOK || deleteResult.Code >= http.StatusMultipleChoices {
		t.Fatalf("delete response = %d: %s", deleteResult.Code, deleteResult.Body.String())
	}
	if err := db.Where("id = ?", statistic.ID).Take(&statistic).Error; err != nil {
		t.Fatalf("reload user statistic: %v", err)
	}
	if statistic.Value != 0 {
		t.Fatalf("user statistic value after delete = %d, want 0", statistic.Value)
	}
}
