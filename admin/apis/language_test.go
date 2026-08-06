package apis

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"github.com/mss-boot-io/mss-boot-admin/admin/center"
	"github.com/mss-boot-io/mss-boot-admin/admin/models"
	"github.com/mss-boot-io/mss-boot-admin/admin/pkg"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/config/gormdb"
	cacheconfig "github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/config/storage/cache"
	gormaction "github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/response/actions/gorm"
	"gorm.io/driver/sqlite"
)

func TestLanguageOtherRegistersPublicListRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()

	(&Language{}).Other(engine.Group("/admin/api"))

	for _, route := range engine.Routes() {
		if route.Method == "GET" && route.Path == "/admin/api/languages/public" {
			return
		}
	}

	t.Fatalf("expected public language route to be registered")
}

func TestLanguageDeleteCacheInvalidatesSnapshotWithoutLoadedName(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	cache, err := cacheconfig.NewRedis(client, nil)
	if err != nil {
		t.Fatalf("create Redis cache: %v", err)
	}
	previousCache := center.GetCache()
	center.SetCache(cache)
	t.Cleanup(func() {
		center.SetCache(previousCache)
		_ = cache.Close()
	})
	ctx := context.Background()
	profile := pkg.LanguageProfile{
		"en-US": {"menu.welcome": "Welcome"},
		"zh-CN": {"menu.welcome": "欢迎"},
	}
	if stored, err := pkg.StoreLanguageProfileCache(ctx, cache, 0, profile); err != nil || !stored {
		t.Fatalf("seed language profile = (%v, %v), want (true, nil)", stored, err)
	}
	ginContext, _ := gin.CreateTestContext(httptest.NewRecorder())
	if err := LanguageDeleteCache(ginContext, &gorm.DB{}, &models.Language{}); err != nil {
		t.Fatalf("delete language cache: %v", err)
	}
	loaded, generation, hit, err := pkg.LoadLanguageProfileCache(ctx, cache)
	if err != nil || hit || generation != 1 || loaded != nil {
		t.Fatalf("load invalidated profile = (%v, %d, %v, %v)", loaded, generation, hit, err)
	}
}

type languageCacheMutationRecord struct {
	ID   uint   `json:"id" gorm:"primaryKey"`
	Name string `json:"name"`
}

func (*languageCacheMutationRecord) TableName() string {
	return "language_cache_mutation_records"
}

func TestLanguageMutationsSucceedWhenRedisInvalidationFails(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open SQLite database: %v", err)
	}
	if err := db.AutoMigrate(&languageCacheMutationRecord{}); err != nil {
		t.Fatalf("migrate mutation record: %v", err)
	}
	previousDB := gormdb.DB
	gormdb.DB = db
	t.Cleanup(func() { gormdb.DB = previousDB })

	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{
		Addr:         server.Addr(),
		DialTimeout:  50 * time.Millisecond,
		ReadTimeout:  50 * time.Millisecond,
		WriteTimeout: 50 * time.Millisecond,
		MaxRetries:   -1,
	})
	cache, err := cacheconfig.NewRedis(client, nil)
	if err != nil {
		t.Fatalf("create Redis cache: %v", err)
	}
	previousCache := center.GetCache()
	center.SetCache(cache)
	t.Cleanup(func() {
		center.SetCache(previousCache)
		_ = cache.Close()
	})
	server.Close()

	control := gormaction.NewControl(
		gormaction.WithModel(&languageCacheMutationRecord{}),
		gormaction.WithKey("id"),
		gormaction.WithAfterCommitCreate(LanguageAddCache),
		gormaction.WithAfterUpdate(LanguageAddCache),
	)
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/records", control.Handler()...)
	router.PUT("/records/:id", control.Handler()...)
	deleteAction := gormaction.NewDelete(
		gormaction.WithModel(&languageCacheMutationRecord{}),
		gormaction.WithKey("id"),
		gormaction.WithAfterDelete(LanguageDeleteCache),
	)
	router.DELETE("/records/:id", deleteAction.Handler()...)

	createResponse := httptest.NewRecorder()
	createRequest := httptest.NewRequest(
		http.MethodPost,
		"/records",
		bytes.NewBufferString(`{"name":"created"}`),
	)
	createRequest.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(createResponse, createRequest)
	if createResponse.Code < http.StatusOK || createResponse.Code >= http.StatusMultipleChoices {
		t.Fatalf("create response = %d: %s", createResponse.Code, createResponse.Body.String())
	}

	var created languageCacheMutationRecord
	if err := db.First(&created, "name = ?", "created").Error; err != nil {
		t.Fatalf("load committed create: %v", err)
	}
	updateResponse := httptest.NewRecorder()
	updateRequest := httptest.NewRequest(
		http.MethodPut,
		fmt.Sprintf("/records/%d", created.ID),
		bytes.NewBufferString(`{"name":"updated"}`),
	)
	updateRequest.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(updateResponse, updateRequest)
	if updateResponse.Code < http.StatusOK || updateResponse.Code >= http.StatusMultipleChoices {
		t.Fatalf("update response = %d: %s", updateResponse.Code, updateResponse.Body.String())
	}
	if err := db.First(&created, created.ID).Error; err != nil {
		t.Fatalf("reload committed update: %v", err)
	}
	if created.Name != "updated" {
		t.Fatalf("updated name = %q, want updated", created.Name)
	}

	deleteResponse := httptest.NewRecorder()
	deleteRequest := httptest.NewRequest(
		http.MethodDelete,
		fmt.Sprintf("/records/%d", created.ID),
		nil,
	)
	router.ServeHTTP(deleteResponse, deleteRequest)
	if deleteResponse.Code < http.StatusOK || deleteResponse.Code >= http.StatusMultipleChoices {
		t.Fatalf("delete response = %d: %s", deleteResponse.Code, deleteResponse.Body.String())
	}
	var remaining int64
	if err := db.Model(&languageCacheMutationRecord{}).Where("id = ?", created.ID).Count(&remaining).Error; err != nil {
		t.Fatalf("count deleted record: %v", err)
	}
	if remaining != 0 {
		t.Fatalf("remaining deleted records = %d, want 0", remaining)
	}
}
