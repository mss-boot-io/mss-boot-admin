package apis

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sort"
	"sync"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/mss-boot-io/mss-boot-admin/admin/center"
	"github.com/mss-boot-io/mss-boot-admin/admin/config"
	"github.com/mss-boot-io/mss-boot-admin/admin/dto"
	"github.com/mss-boot-io/mss-boot-admin/admin/models"
	"github.com/mss-boot-io/mss-boot-admin/admin/service"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/config/gormdb"
	cacheconfig "github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/config/storage/cache"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/response"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/security"
)

func setupThemeConfigAPITest(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.AutoMigrate(&models.AppConfig{}, &models.UserConfig{}, &models.ConfigRevision{}))
	require.NoError(t, db.Exec(
		`CREATE UNIQUE INDEX idx_app_config_api_test ON mss_boot_app_configs(name, "group")`,
	).Error)
	require.NoError(t, db.Exec(
		`CREATE UNIQUE INDEX idx_user_config_api_test ON mss_boot_user_configs(user_id, name, "group")`,
	).Error)

	previousDB := gormdb.DB
	previousCache := center.GetCache()
	gormdb.DB = db
	center.SetCache(nil)
	t.Cleanup(func() {
		gormdb.DB = previousDB
		center.SetCache(previousCache)
		_ = sqlDB.Close()
	})
	return db
}

func appThemeControlResponse(body string) *httptest.ResponseRecorder {
	return appThemeResponse(http.MethodPut, body, `"theme-application-0"`)
}

func appThemeResponse(method, body, ifMatch string) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Params = gin.Params{{Key: "group", Value: service.ThemeConfigGroup}}
	ctx.Request = httptest.NewRequest(method, "/admin/api/app-configs/theme", bytes.NewBufferString(body))
	ctx.Request.Header.Set("Content-Type", "application/json")
	if ifMatch != "" {
		ctx.Request.Header.Set("If-Match", ifMatch)
	}
	switch method {
	case http.MethodGet:
		(&AppConfig{}).Group(ctx)
	case http.MethodPut:
		(&AppConfig{}).Control(ctx)
	case http.MethodDelete:
		(&AppConfig{}).Reset(ctx)
	}
	return recorder
}

func userThemeResponse(method, body, ifMatch string, principal *models.User) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Params = gin.Params{{Key: "group", Value: service.ThemeConfigGroup}}
	ctx.Request = httptest.NewRequest(method, "/admin/api/user-configs/theme", bytes.NewBufferString(body))
	ctx.Request.Header.Set("Content-Type", "application/json")
	if ifMatch != "" {
		ctx.Request.Header.Set("If-Match", ifMatch)
	}
	ctx.Set(config.Cfg.Auth.IdentityKey, principal)
	switch method {
	case http.MethodGet:
		(&UserConfig{}).Group(ctx)
	case http.MethodPut:
		(&UserConfig{}).Control(ctx)
	case http.MethodDelete:
		(&UserConfig{}).Reset(ctx)
	}
	return recorder
}

func appConfigGroupResponse(method, group, body string) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Params = gin.Params{{Key: "group", Value: group}}
	ctx.Request = httptest.NewRequest(method, "/admin/api/app-configs/config", bytes.NewBufferString(body))
	ctx.Request.Header.Set("Content-Type", "application/json")
	if method == http.MethodGet {
		(&AppConfig{}).Group(ctx)
	} else {
		(&AppConfig{}).Control(ctx)
	}
	return recorder
}

func userConfigGroupResponse(
	method, group, body string,
	principal *models.User,
) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Params = gin.Params{{Key: "group", Value: group}}
	ctx.Request = httptest.NewRequest(method, "/admin/api/user-configs/config", bytes.NewBufferString(body))
	ctx.Request.Header.Set("Content-Type", "application/json")
	ctx.Set(config.Cfg.Auth.IdentityKey, principal)
	if method == http.MethodGet {
		(&UserConfig{}).Group(ctx)
	} else {
		(&UserConfig{}).Control(ctx)
	}
	return recorder
}

func decodeThemeResource(t *testing.T, recorder *httptest.ResponseRecorder) dto.ThemeResource {
	t.Helper()
	var resource dto.ThemeResource
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resource), recorder.Body.String())
	return resource
}

func TestThemeResponsesAlwaysUseCanonicalContentType(t *testing.T) {
	setupThemeConfigAPITest(t)
	gin.SetMode(gin.TestMode)

	success := appThemeResponse(http.MethodGet, "", "")
	require.Equal(t, http.StatusOK, success.Code, success.Body.String())
	require.Equal(t, themeContractMediaType, success.Header().Get("Content-Type"))
	require.Contains(t, success.Body.String(), `"_meta"`)

	update := appThemeResponse(
		http.MethodPut,
		`{"data":{"fixedHeader":false}}`,
		`"theme-application-0"`,
	)
	require.Equal(t, http.StatusOK, update.Code, update.Body.String())
	require.Equal(t, themeContractMediaType, update.Header().Get("Content-Type"))

	conflict := appThemeResponse(
		http.MethodPut,
		`{"data":{"layout":"side"}}`,
		`"theme-application-0"`,
	)
	require.Equal(t, http.StatusPreconditionFailed, conflict.Code, conflict.Body.String())
	require.Equal(t, themeContractMediaType, conflict.Header().Get("Content-Type"))
}

func TestConfigurationProfilesAreNeverStoredByHTTPSharedCaches(t *testing.T) {
	setupThemeConfigAPITest(t)
	gin.SetMode(gin.TestMode)
	previousVerifyHandler := response.VerifyHandler
	response.VerifyHandler = func(*gin.Context) security.Verifier { return nil }
	t.Cleanup(func() { response.VerifyHandler = previousVerifyHandler })

	appRecorder := httptest.NewRecorder()
	appCtx, _ := gin.CreateTestContext(appRecorder)
	appCtx.Request = httptest.NewRequest(http.MethodGet, "/admin/api/app-configs/profile", nil)
	(&AppConfig{}).Profile(appCtx)
	require.Equal(t, http.StatusOK, appRecorder.Code, appRecorder.Body.String())
	require.Equal(t, "no-store", appRecorder.Header().Get("Cache-Control"))

	userRecorder := httptest.NewRecorder()
	userCtx, _ := gin.CreateTestContext(userRecorder)
	userCtx.Request = httptest.NewRequest(http.MethodGet, "/admin/api/user-configs/profile", nil)
	(&UserConfig{}).Profile(userCtx)
	require.Equal(t, http.StatusOK, userRecorder.Code, userRecorder.Body.String())
	require.Equal(t, "no-store", userRecorder.Header().Get("Cache-Control"))
}

func TestReservedThemeAndPublicIdentifierVariantsReturn422WithoutWriting(t *testing.T) {
	db := setupThemeConfigAPITest(t)
	gin.SetMode(gin.TestMode)
	previousIdentityKey := config.Cfg.Auth.IdentityKey
	config.Cfg.Auth.IdentityKey = "theme-identifier-identity"
	t.Cleanup(func() { config.Cfg.Auth.IdentityKey = previousIdentityKey })
	principal := &models.User{}
	principal.ID = "user-a"

	for _, group := range []string{"Theme", "thème", "theme "} {
		for _, method := range []string{http.MethodGet, http.MethodPut} {
			appResponse := appConfigGroupResponse(method, group, `{"data":{"layout":"side"}}`)
			require.Equal(t, http.StatusUnprocessableEntity, appResponse.Code, "%s %q: %s", method, group, appResponse.Body.String())
			userResponse := userConfigGroupResponse(method, group, `{"data":{"layout":"side"}}`, principal)
			require.Equal(t, http.StatusUnprocessableEntity, userResponse.Code, "%s %q: %s", method, group, userResponse.Body.String())
		}
	}

	for _, test := range []struct {
		group string
		name  string
	}{
		{group: "Base", name: "websiteName"},
		{group: "báse", name: "websiteName"},
		{group: "base ", name: "websiteName"},
		{group: "base", name: "WebsiteName"},
		{group: "base", name: "wéBSITEName"},
		{group: "base", name: "websiteName "},
	} {
		response := appConfigGroupResponse(
			http.MethodPut,
			test.group,
			`{"data":{"`+test.name+`":"poison"}}`,
		)
		require.Equal(t, http.StatusUnprocessableEntity, response.Code, "%s/%s: %s", test.group, test.name, response.Body.String())
	}

	var appCount int64
	require.NoError(t, db.Model(&models.AppConfig{}).Count(&appCount).Error)
	require.Zero(t, appCount)
	var userCount int64
	require.NoError(t, db.Model(&models.UserConfig{}).Count(&userCount).Error)
	require.Zero(t, userCount)
	var revisionCount int64
	require.NoError(t, db.Model(&models.ConfigRevision{}).Count(&revisionCount).Error)
	require.Zero(t, revisionCount)
}

func TestAppThemeControlReturns422ForInvalidThemeValues(t *testing.T) {
	setupThemeConfigAPITest(t)
	gin.SetMode(gin.TestMode)

	for _, test := range []struct {
		name string
		body string
	}{
		{name: "unknown field", body: `{"data":{"unknownThemeSetting":true}}`},
		{name: "invalid legacy pwa", body: `{"data":{"pwa":"true"}}`},
		{name: "invalid enum", body: `{"data":{"navTheme":"dark"}}`},
		{name: "invalid boolean", body: `{"data":{"fixedHeader":"false"}}`},
		{name: "invalid color", body: `{"data":{"colorPrimary":"blue"}}`},
	} {
		t.Run(test.name, func(t *testing.T) {
			response := appThemeControlResponse(test.body)
			require.Equal(t, http.StatusUnprocessableEntity, response.Code, response.Body.String())
		})
	}
}

func TestCanonicalAppThemeRejectsLegacyPWAWithoutWriting(t *testing.T) {
	db := setupThemeConfigAPITest(t)
	gin.SetMode(gin.TestMode)

	response := appThemeResponse(
		http.MethodPut,
		`{"data":{"pwa":true,"layout":"side"}}`,
		`"theme-application-0"`,
	)
	require.Equal(t, http.StatusUnprocessableEntity, response.Code, response.Body.String())

	var configCount int64
	require.NoError(t, db.Model(&models.AppConfig{}).Count(&configCount).Error)
	require.Zero(t, configCount)
	var revisionCount int64
	require.NoError(t, db.Model(&models.ConfigRevision{}).Count(&revisionCount).Error)
	require.Zero(t, revisionCount)
}

func TestUserThemeRejectsLegacyPWA(t *testing.T) {
	db := setupThemeConfigAPITest(t)
	gin.SetMode(gin.TestMode)

	previousIdentityKey := config.Cfg.Auth.IdentityKey
	config.Cfg.Auth.IdentityKey = "theme-legacy-pwa-identity"
	t.Cleanup(func() { config.Cfg.Auth.IdentityKey = previousIdentityKey })
	principal := &models.User{}
	principal.ID = "user-a"
	response := userThemeResponse(
		http.MethodPut,
		`{"data":{"pwa":false,"colorWeak":false}}`,
		`"theme-user-0"`,
		principal,
	)
	require.Equal(t, http.StatusUnprocessableEntity, response.Code, response.Body.String())

	var configCount int64
	require.NoError(t, db.Model(&models.UserConfig{}).Count(&configCount).Error)
	require.Zero(t, configCount)
	var revisionCount int64
	require.NoError(t, db.Model(&models.ConfigRevision{}).Count(&revisionCount).Error)
	require.Zero(t, revisionCount)
}

func TestApplicationThemeEndpointsExposeCanonicalETagsAndStaleResource(t *testing.T) {
	setupThemeConfigAPITest(t)
	gin.SetMode(gin.TestMode)

	response := appThemeResponse(http.MethodGet, "", "")
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	require.Equal(t, `"theme-application-0"`, response.Header().Get("ETag"))
	require.Equal(t, "no-store", response.Header().Get("Cache-Control"))
	resource := decodeThemeResource(t, response)
	require.Equal(t, service.ThemeScopeApplication, resource.Meta.Scope)
	require.Equal(t, "0", resource.Meta.Revision)

	response = appThemeResponse(
		http.MethodPut,
		`{"data":{"fixedHeader":false,"colorPrimary":"#ABCDEF"}}`,
		`"theme-application-0"`,
	)
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	require.Equal(t, `"theme-application-1"`, response.Header().Get("ETag"))
	resource = decodeThemeResource(t, response)
	require.Equal(t, "1", resource.Meta.Revision)
	require.NotNil(t, resource.FixedHeader)
	require.False(t, *resource.FixedHeader)
	require.Equal(t, "#abcdef", *resource.ColorPrimary)

	response = appThemeResponse(
		http.MethodPut,
		`{"data":{"layout":"side"}}`,
		`"theme-application-0"`,
	)
	require.Equal(t, http.StatusPreconditionFailed, response.Code, response.Body.String())
	require.Equal(t, `"theme-application-1"`, response.Header().Get("ETag"))
	var conflict struct {
		ErrorCode string `json:"errorCode"`
		Data      struct {
			Current dto.ThemeResource `json:"current"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &conflict))
	require.Equal(t, themeRevisionConflictCode, conflict.ErrorCode)
	require.Equal(t, "1", conflict.Data.Current.Meta.Revision)
	require.Nil(t, conflict.Data.Current.Layout, "a stale write must not reach the database")
	require.Equal(t, "#abcdef", *conflict.Data.Current.ColorPrimary)

	// V6 requires an explicit revision for every mutation.
	response = appThemeResponse(http.MethodPut, `{"data":{"layout":"mix"}}`, "")
	require.Equal(t, http.StatusPreconditionRequired, response.Code, response.Body.String())
	require.Contains(t, response.Body.String(), themeIfMatchRequiredCode)

	response = appThemeResponse(http.MethodDelete, "", `"theme-application-1"`)
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	require.Equal(t, `"theme-application-2"`, response.Header().Get("ETag"))
	resource = decodeThemeResource(t, response)
	require.Equal(t, "2", resource.Meta.Revision)
	require.Nil(t, resource.FixedHeader)
	require.Nil(t, resource.ColorPrimary)
	require.Nil(t, resource.Layout)
}

func TestThemeEndpointRejectsMalformedIfMatchBeforeWriting(t *testing.T) {
	db := setupThemeConfigAPITest(t)
	gin.SetMode(gin.TestMode)

	for _, ifMatch := range []string{
		" ",
		"0",
		`W/"theme-application-0"`,
		`"theme-user-0"`,
		`"theme-application-00"`,
		`"theme-application-\x30"`,
		`"theme-application-not-a-number"`,
		`"theme-application-0", "theme-application-1"`,
		"*",
	} {
		response := appThemeResponse(http.MethodPut, `{"data":{"layout":"side"}}`, ifMatch)
		require.Equal(t, http.StatusBadRequest, response.Code, "%s: %s", ifMatch, response.Body.String())
		require.Contains(t, response.Body.String(), themeIfMatchInvalidCode)
	}

	var configCount int64
	require.NoError(t, db.Model(&models.AppConfig{}).Count(&configCount).Error)
	require.Zero(t, configCount)
	var revisionCount int64
	require.NoError(t, db.Model(&models.ConfigRevision{}).Count(&revisionCount).Error)
	require.Zero(t, revisionCount)
}

func TestConcurrentApplicationThemeWritersUsingSameETagAllowOneCommit(t *testing.T) {
	setupThemeConfigAPITest(t)
	gin.SetMode(gin.TestMode)

	type result struct {
		code int
		etag string
		body string
	}
	start := make(chan struct{})
	results := make(chan result, 2)
	var writers sync.WaitGroup
	for _, body := range []string{
		`{"data":{"layout":"side"}}`,
		`{"data":{"navTheme":"realDark"}}`,
	} {
		writers.Add(1)
		go func(body string) {
			defer writers.Done()
			<-start
			response := appThemeResponse(http.MethodPut, body, `"theme-application-0"`)
			results <- result{code: response.Code, etag: response.Header().Get("ETag"), body: response.Body.String()}
		}(body)
	}
	close(start)
	writers.Wait()
	close(results)

	var codes []int
	for result := range results {
		codes = append(codes, result.code)
		require.Equal(t, `"theme-application-1"`, result.etag, result.body)
	}
	sort.Ints(codes)
	require.Equal(t, []int{http.StatusOK, http.StatusPreconditionFailed}, codes)

	response := appThemeResponse(http.MethodGet, "", "")
	require.Equal(t, `"theme-application-1"`, response.Header().Get("ETag"))
	resource := decodeThemeResource(t, response)
	require.Equal(t, "1", resource.Meta.Revision)
	setValues := 0
	if resource.Layout != nil {
		setValues++
	}
	if resource.NavTheme != nil {
		setValues++
	}
	require.Equal(t, 1, setValues, "only one same-revision writer may commit")
}

func TestUserThemeEndpointsUseOwnerScopedETagsAndDeleteReturnsCanonicalResource(t *testing.T) {
	setupThemeConfigAPITest(t)
	gin.SetMode(gin.TestMode)
	previousIdentityKey := config.Cfg.Auth.IdentityKey
	config.Cfg.Auth.IdentityKey = "theme-user-etag-identity"
	t.Cleanup(func() { config.Cfg.Auth.IdentityKey = previousIdentityKey })
	principal := &models.User{}
	principal.ID = "user-a"

	response := userThemeResponse(http.MethodGet, "", "", principal)
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	require.Equal(t, `"theme-user-0"`, response.Header().Get("ETag"))
	require.Equal(t, "no-store", response.Header().Get("Cache-Control"))
	require.Contains(t, response.Body.String(), `"_meta"`)

	response = userThemeResponse(
		http.MethodPut,
		`{"data":{"colorWeak":false}}`,
		`"theme-user-0"`,
		principal,
	)
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	require.Equal(t, `"theme-user-1"`, response.Header().Get("ETag"))

	response = userThemeResponse(
		http.MethodPut,
		`{"data":{"layout":"side"}}`,
		`"theme-user-0"`,
		principal,
	)
	require.Equal(t, http.StatusPreconditionFailed, response.Code, response.Body.String())
	require.Equal(t, `"theme-user-1"`, response.Header().Get("ETag"))

	response = userThemeResponse(http.MethodDelete, "", `"theme-user-1"`, principal)
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	require.Equal(t, `"theme-user-2"`, response.Header().Get("ETag"))
	resource := decodeThemeResource(t, response)
	require.Equal(t, service.ThemeScopeUser, resource.Meta.Scope)
	require.Equal(t, "2", resource.Meta.Revision)
	require.Nil(t, resource.ColorWeak)
}

func TestUserThemeControlReturns422ForInvalidThemeValues(t *testing.T) {
	setupThemeConfigAPITest(t)
	gin.SetMode(gin.TestMode)

	previousIdentityKey := config.Cfg.Auth.IdentityKey
	config.Cfg.Auth.IdentityKey = "theme-test-identity"
	t.Cleanup(func() { config.Cfg.Auth.IdentityKey = previousIdentityKey })

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Params = gin.Params{{Key: "group", Value: service.ThemeConfigGroup}}
	ctx.Request = httptest.NewRequest(
		http.MethodPut,
		"/admin/api/user-configs/theme",
		bytes.NewBufferString(`{"data":{"layout":"invalid"}}`),
	)
	ctx.Request.Header.Set("Content-Type", "application/json")
	ctx.Request.Header.Set("If-Match", `"theme-user-0"`)
	principal := &models.User{}
	principal.ID = "user-a"
	ctx.Set(config.Cfg.Auth.IdentityKey, principal)

	(&UserConfig{}).Control(ctx)
	require.Equal(t, http.StatusUnprocessableEntity, recorder.Code, recorder.Body.String())
}

func TestAppThemeControlReturns500AndRollsBackDatabaseFailure(t *testing.T) {
	db := setupThemeConfigAPITest(t)
	gin.SetMode(gin.TestMode)
	require.NoError(t, db.Exec(`
		CREATE TRIGGER fail_app_theme_layout_api
		BEFORE INSERT ON mss_boot_app_configs
		WHEN NEW.name = 'layout'
		BEGIN
			SELECT RAISE(ABORT, 'forced theme api failure');
		END;
	`).Error)

	response := appThemeControlResponse(`{"data":{"colorPrimary":"#123456","layout":"side"}}`)
	require.Equal(t, http.StatusInternalServerError, response.Code, response.Body.String())

	var count int64
	require.NoError(t, db.Model(&models.AppConfig{}).
		Where(&models.AppConfig{Group: service.ThemeConfigGroup}).Count(&count).Error)
	require.Zero(t, count)
}

func TestAppThemeResetInvalidatesPublicProfileCache(t *testing.T) {
	db := setupThemeConfigAPITest(t)
	gin.SetMode(gin.TestMode)
	require.NoError(t, db.Create(&models.AppConfig{
		Group: service.ThemeConfigGroup,
		Name:  "navTheme",
		Value: "realDark",
	}).Error)

	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	cache, err := cacheconfig.NewRedis(client, nil)
	require.NoError(t, err)
	previousCache := center.GetCache()
	center.SetCache(cache)
	t.Cleanup(func() {
		center.SetCache(previousCache)
		_ = cache.Close()
	})
	server.Set("app-configs:{profile:public}:v2:0", `{"v":2,"profileRevision":0,"themeRevision":0,"profile":{"theme":{"navTheme":"realDark"}}}`)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodDelete, "/admin/api/app-configs/theme", nil)
	ctx.Request.Header.Set("If-Match", `"theme-application-0"`)
	(&AppConfig{}).Reset(ctx)

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.Contains(t, recorder.Body.String(), `"revision":"1"`)
	require.False(t, server.Exists("app-configs:{profile:public}:v2:0"))
	var revisions []models.ConfigRevision
	require.NoError(t, db.Order("resource").Find(&revisions).Error)
	require.Len(t, revisions, 2)
	require.Equal(t, int64(1), revisions[0].Revision)
	require.Equal(t, int64(1), revisions[1].Revision)
}
