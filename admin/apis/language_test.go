package apis

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
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
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/enum"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/response"
	gormaction "github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/response/actions/gorm"
	"gorm.io/driver/sqlite"
)

func setupLanguageAPITestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open SQLite database: %v", err)
	}
	if err := db.AutoMigrate(&models.Language{}); err != nil {
		t.Fatalf("migrate language table: %v", err)
	}
	previousDB := gormdb.DB
	gormdb.DB = db
	t.Cleanup(func() { gormdb.DB = previousDB })
	previousCache := center.GetCache()
	center.SetCache(nil)
	t.Cleanup(func() { center.SetCache(previousCache) })
	return db
}

func TestLanguagePublicEndpointsOnlyExposeEnabledProjection(t *testing.T) {
	db := setupLanguageAPITestDB(t)
	enabledDefinitions := models.LanguageDefines{{Group: "menu", Key: "welcome", Value: "Welcome"}}
	disabledDefinitions := models.LanguageDefines{{Group: "menu", Key: "secret", Value: "Disabled"}}
	for _, language := range []*models.Language{
		{Name: "en-US", Status: enum.Enabled, Remark: "management-only", Defines: &enabledDefinitions},
		{Name: "zh-CN", Status: enum.Disabled, Defines: &disabledDefinitions},
	} {
		if err := db.Create(language).Error; err != nil {
			t.Fatalf("create language %q: %v", language.Name, err)
		}
	}

	gin.SetMode(gin.TestMode)
	publicRecorder := httptest.NewRecorder()
	publicContext, _ := gin.CreateTestContext(publicRecorder)
	publicContext.Request = httptest.NewRequest(http.MethodGet, "/admin/api/languages/public", nil)
	(&Language{}).PublicList(publicContext)
	if publicRecorder.Code != http.StatusOK {
		t.Fatalf("public list response = %d: %s", publicRecorder.Code, publicRecorder.Body.String())
	}
	var publicRecords []map[string]any
	if err := json.Unmarshal(publicRecorder.Body.Bytes(), &publicRecords); err != nil {
		t.Fatalf("decode public list: %v", err)
	}
	if len(publicRecords) != 1 || publicRecords[0]["name"] != "en-US" {
		t.Fatalf("public list = %#v, want only en-US", publicRecords)
	}
	for _, forbidden := range []string{"id", "status", "remark", "updatedAt", "createdAt"} {
		if _, exists := publicRecords[0][forbidden]; exists {
			t.Fatalf("public list disclosed management field %q: %#v", forbidden, publicRecords[0])
		}
	}

	profileRecorder := httptest.NewRecorder()
	profileContext, _ := gin.CreateTestContext(profileRecorder)
	profileContext.Request = httptest.NewRequest(http.MethodGet, "/admin/api/language/profile", nil)
	(&Language{}).Profile(profileContext)
	if profileRecorder.Code != http.StatusOK {
		t.Fatalf("profile response = %d: %s", profileRecorder.Code, profileRecorder.Body.String())
	}
	var profile map[string]map[string]string
	if err := json.Unmarshal(profileRecorder.Body.Bytes(), &profile); err != nil {
		t.Fatalf("decode profile: %v", err)
	}
	if profile["en-US"]["menu.welcome"] != "Welcome" || profile["zh-CN"] != nil {
		t.Fatalf("profile = %#v, want enabled definitions only", profile)
	}
}

func TestLanguageManagedWriteOwnsIdentityAndRejectsStaleRevision(t *testing.T) {
	db := setupLanguageAPITestDB(t)
	gin.SetMode(gin.TestMode)
	router := gin.New()
	controller := &Language{}
	router.POST("/languages", controller.ManagedCreate)
	router.PUT("/languages/:id", controller.ManagedUpdate)

	createRecorder := httptest.NewRecorder()
	createRequest := httptest.NewRequest(http.MethodPost, "/languages", bytes.NewBufferString(`{
		"id":"client-owned",
		"createdAt":"2000-01-01T00:00:00Z",
		"name":"en-US",
		"status":"enabled",
		"defines":[{"id":"welcome","group":"menu","key":"welcome","value":"Welcome"}]
	}`))
	createRequest.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(createRecorder, createRequest)
	if createRecorder.Code != http.StatusCreated {
		t.Fatalf("create response = %d: %s", createRecorder.Code, createRecorder.Body.String())
	}
	var created models.Language
	if err := json.Unmarshal(createRecorder.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode created language: %v", err)
	}
	if created.ID == "" || created.ID == "client-owned" {
		t.Fatalf("created ID = %q, want a server-owned ID", created.ID)
	}
	if created.Defines == nil || len(*created.Defines) != 1 || (*created.Defines)[0].ID == "welcome" {
		t.Fatalf("created definitions = %#v, want a server-owned definition ID", created.Defines)
	}
	var persisted models.Language
	if err := db.First(&persisted, "id = ?", created.ID).Error; err != nil {
		t.Fatalf("load created language: %v", err)
	}
	if persisted.CreatedAt.Year() == 2000 {
		t.Fatalf("client overwrote CreatedAt: %s", persisted.CreatedAt)
	}

	updateBody := fmt.Sprintf(`{
		"name":"en-US",
		"remark":"first update",
		"status":"enabled",
		"expectedUpdatedAt":%q,
		"defines":[{"id":%q,"group":"menu","key":"welcome","value":"Updated"}]
	}`, created.UpdatedAt.Format(time.RFC3339Nano), (*created.Defines)[0].ID)
	updateRecorder := httptest.NewRecorder()
	updateRequest := httptest.NewRequest(http.MethodPut, "/languages/"+created.ID, bytes.NewBufferString(updateBody))
	updateRequest.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(updateRecorder, updateRequest)
	if updateRecorder.Code != http.StatusOK {
		t.Fatalf("update response = %d: %s", updateRecorder.Code, updateRecorder.Body.String())
	}

	staleRecorder := httptest.NewRecorder()
	staleRequest := httptest.NewRequest(http.MethodPut, "/languages/"+created.ID, bytes.NewBufferString(updateBody))
	staleRequest.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(staleRecorder, staleRequest)
	if staleRecorder.Code != http.StatusConflict {
		t.Fatalf("stale update response = %d: %s", staleRecorder.Code, staleRecorder.Body.String())
	}
	var staleError response.Response
	if err := json.Unmarshal(staleRecorder.Body.Bytes(), &staleError); err != nil {
		t.Fatalf("decode stale response: %v", err)
	}
	if staleError.ErrorCode != "LANGUAGE_REVISION_CONFLICT" {
		t.Fatalf("stale error code = %q", staleError.ErrorCode)
	}

	var latest models.Language
	if err := json.Unmarshal(updateRecorder.Body.Bytes(), &latest); err != nil {
		t.Fatalf("decode updated language: %v", err)
	}
	foreignIDBody := fmt.Sprintf(`{
		"name":"en-US",
		"status":"enabled",
		"expectedUpdatedAt":%q,
		"defines":[{"id":"client-replaced","group":"menu","key":"welcome","value":"Changed"}]
	}`, latest.UpdatedAt.Format(time.RFC3339Nano))
	foreignIDRecorder := httptest.NewRecorder()
	foreignIDRequest := httptest.NewRequest(
		http.MethodPut,
		"/languages/"+created.ID,
		bytes.NewBufferString(foreignIDBody),
	)
	foreignIDRequest.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(foreignIDRecorder, foreignIDRequest)
	if foreignIDRecorder.Code != http.StatusUnprocessableEntity {
		t.Fatalf("foreign definition ID response = %d: %s", foreignIDRecorder.Code, foreignIDRecorder.Body.String())
	}
}

func TestLanguageManagedSummaryIsBoundedAndOmitsDefinitions(t *testing.T) {
	db := setupLanguageAPITestDB(t)
	definitions := models.LanguageDefines{{Group: "menu", Key: "welcome", Value: "Welcome"}}
	if err := db.Create(&models.Language{Name: "en-US", Status: enum.Enabled, Defines: &definitions}).Error; err != nil {
		t.Fatalf("create language: %v", err)
	}
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/languages", (&Language{}).ManagedList)

	result := httptest.NewRecorder()
	router.ServeHTTP(result, httptest.NewRequest(http.MethodGet, "/languages?view=summary&current=1&pageSize=20", nil))
	if result.Code != http.StatusOK {
		t.Fatalf("summary response = %d: %s", result.Code, result.Body.String())
	}
	if bytes.Contains(result.Body.Bytes(), []byte(`"defines"`)) {
		t.Fatalf("summary response disclosed definitions: %s", result.Body.String())
	}

	escapedFilter := httptest.NewRecorder()
	router.ServeHTTP(
		escapedFilter,
		httptest.NewRequest(http.MethodGet, "/languages?view=summary&name=en_&pageSize=20", nil),
	)
	if escapedFilter.Code != http.StatusOK {
		t.Fatalf("escaped name filter response = %d: %s", escapedFilter.Code, escapedFilter.Body.String())
	}
	var filtered struct {
		Total int64 `json:"total"`
	}
	if err := json.Unmarshal(escapedFilter.Body.Bytes(), &filtered); err != nil {
		t.Fatalf("decode escaped name filter: %v", err)
	}
	if filtered.Total != 0 {
		t.Fatalf("escaped wildcard filter total = %d, want 0", filtered.Total)
	}

	unbounded := httptest.NewRecorder()
	router.ServeHTTP(unbounded, httptest.NewRequest(http.MethodGet, "/languages?view=summary&pageSize=101", nil))
	if unbounded.Code != http.StatusUnprocessableEntity {
		t.Fatalf("unbounded response = %d: %s", unbounded.Code, unbounded.Body.String())
	}
}

func TestLanguageManagedWriteRejectsChunkedOversizedPayload(t *testing.T) {
	setupLanguageAPITestDB(t)
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/languages", (&Language{}).ManagedCreate)

	payload := `{"name":"en-US","status":"enabled","remark":"` +
		strings.Repeat("x", maxLanguageWriteRequestSize) + `"}`
	request := httptest.NewRequest(http.MethodPost, "/languages", strings.NewReader(payload))
	request.ContentLength = -1
	request.Header.Set("Content-Type", "application/json")
	result := httptest.NewRecorder()
	router.ServeHTTP(result, request)

	if result.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversized response = %d: %s", result.Code, result.Body.String())
	}
	var apiError response.Response
	if err := json.Unmarshal(result.Body.Bytes(), &apiError); err != nil {
		t.Fatalf("decode oversized response: %v", err)
	}
	if apiError.ErrorCode != "LANGUAGE_PAYLOAD_TOO_LARGE" {
		t.Fatalf("oversized error code = %q", apiError.ErrorCode)
	}
}

func TestLanguageReadsFailClosedForCorruptStoredDefinitions(t *testing.T) {
	db := setupLanguageAPITestDB(t)
	language := &models.Language{Name: "en-US", Status: enum.Enabled}
	if err := db.Create(language).Error; err != nil {
		t.Fatalf("create language: %v", err)
	}
	corrupt := `[
		{"id":"one","group":"menu","key":"welcome","value":"A"},
		{"id":"two","group":"menu","key":"welcome","value":"B"}
	]`
	if err := db.Exec(
		"UPDATE mss_boot_languages SET defines = ? WHERE id = ?",
		corrupt,
		language.ID,
	).Error; err != nil {
		t.Fatalf("corrupt language definitions: %v", err)
	}

	gin.SetMode(gin.TestMode)
	for _, test := range []struct {
		name    string
		path    string
		handler gin.HandlerFunc
	}{
		{name: "profile", path: "/language/profile", handler: (&Language{}).Profile},
		{name: "detail", path: "/languages/" + language.ID, handler: (&Language{}).ManagedGet},
	} {
		t.Run(test.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(recorder)
			ctx.Request = httptest.NewRequest(http.MethodGet, test.path, nil)
			if test.name == "detail" {
				ctx.Params = gin.Params{{Key: "id", Value: language.ID}}
			}
			test.handler(ctx)
			if recorder.Code != http.StatusServiceUnavailable {
				t.Fatalf("response = %d: %s", recorder.Code, recorder.Body.String())
			}
			var apiError response.Response
			if err := json.Unmarshal(recorder.Body.Bytes(), &apiError); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if apiError.ErrorCode != "LANGUAGE_DATA_INVALID" {
				t.Fatalf("error code = %q", apiError.ErrorCode)
			}
		})
	}
}

func TestLanguagePublicPayloadHasAggregateResponseLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/language/profile", nil)

	if validateLanguagePublicPayload(ctx, strings.Repeat("x", maxLanguagePublicPayload)) {
		t.Fatal("aggregate public language payload unexpectedly passed its response limit")
	}
	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("aggregate limit response = %d: %s", recorder.Code, recorder.Body.String())
	}
}

func TestLanguageOtherRegistersPublicListRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()

	(&Language{}).Other(engine.Group("/admin/api"))

	routes := make(map[string]bool)
	for _, route := range engine.Routes() {
		routes[route.Method+" "+route.Path] = true
	}
	for _, expected := range []string{
		"GET /admin/api/languages/public",
		"GET /admin/api/languages/:id",
		"POST /admin/api/languages",
		"PUT /admin/api/languages/:id",
	} {
		if !routes[expected] {
			t.Fatalf("expected language route %q to be registered: %#v", expected, routes)
		}
	}
	if action := newLanguageController().GetAction(response.Get); action != nil {
		t.Fatal("generic language detail action remains enabled alongside the bounded route")
	}
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

func TestLanguageProfileCachesEmptySnapshot(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open SQLite database: %v", err)
	}
	if err := db.AutoMigrate(&models.Language{}); err != nil {
		t.Fatalf("migrate language table: %v", err)
	}
	previousDB := gormdb.DB
	gormdb.DB = db
	t.Cleanup(func() { gormdb.DB = previousDB })

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

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/admin/api/language/profile", nil)
	(&Language{}).Profile(ctx)
	if recorder.Code != http.StatusOK {
		t.Fatalf("profile response = %d: %s", recorder.Code, recorder.Body.String())
	}

	loaded, generation, hit, err := pkg.LoadLanguageProfileCache(context.Background(), cache)
	if err != nil || !hit || generation != 0 || len(loaded) != 0 {
		t.Fatalf("load empty language profile = (%v, %d, %v, %v), want cached empty generation 0", loaded, generation, hit, err)
	}

	// Remove the source table so a second successful response can only come
	// from the cached empty snapshot rather than another database query.
	if err := db.Migrator().DropTable(&models.Language{}); err != nil {
		t.Fatalf("drop language table: %v", err)
	}
	secondRecorder := httptest.NewRecorder()
	secondContext, _ := gin.CreateTestContext(secondRecorder)
	secondContext.Request = httptest.NewRequest(http.MethodGet, "/admin/api/language/profile", nil)
	(&Language{}).Profile(secondContext)
	if secondRecorder.Code != http.StatusOK {
		t.Fatalf("cached profile response = %d: %s", secondRecorder.Code, secondRecorder.Body.String())
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
