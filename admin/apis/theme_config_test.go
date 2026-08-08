package apis

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
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

func registerOutsideTransactionPWAQueryFailure(
	t *testing.T,
	db *gorm.DB,
) (*int, func()) {
	t.Helper()
	callbackName := "test:legacy-pwa-post-commit-" + strings.ReplaceAll(t.Name(), "/", "-")
	outsideQueries := 0
	injectedErr := errors.New("injected transaction-external pwa read failure")
	require.NoError(t, db.Callback().Query().After("gorm:query").Register(callbackName, func(tx *gorm.DB) {
		if tx.Statement == nil || tx.Statement.Table != (&models.AppConfig{}).TableName() {
			return
		}
		if _, inTransaction := tx.Statement.ConnPool.(*sql.Tx); inTransaction {
			return
		}
		for _, value := range tx.Statement.Vars {
			if value == "pwa" {
				outsideQueries++
				tx.AddError(injectedErr)
				return
			}
		}
	}))

	var removeOnce sync.Once
	remove := func() {
		removeOnce.Do(func() {
			require.NoError(t, db.Callback().Query().Remove(callbackName))
		})
	}
	t.Cleanup(remove)
	return &outsideQueries, remove
}

func appThemeControlResponse(body string) *httptest.ResponseRecorder {
	return appThemeResponse(http.MethodPut, body, "")
}

func appThemeResponse(method, body, ifMatch string) *httptest.ResponseRecorder {
	return appThemeResponseWithContract(method, body, ifMatch, true)
}

func appThemeLegacyResponse(method, body, ifMatch string) *httptest.ResponseRecorder {
	return appThemeResponseWithContract(method, body, ifMatch, false)
}

func appThemeResponseWithContract(method, body, ifMatch string, canonical bool) *httptest.ResponseRecorder {
	accept := ""
	if canonical {
		accept = themeContractMediaType
	}
	return appThemeResponseWithAccept(method, body, ifMatch, accept)
}

func appThemeResponseWithAccept(method, body, ifMatch, accept string) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Params = gin.Params{{Key: "group", Value: service.ThemeConfigGroup}}
	ctx.Request = httptest.NewRequest(method, "/admin/api/app-configs/theme", bytes.NewBufferString(body))
	ctx.Request.Header.Set("Content-Type", "application/json")
	if ifMatch != "" {
		ctx.Request.Header.Set("If-Match", ifMatch)
	}
	if accept != "" {
		ctx.Request.Header.Set("Accept", accept)
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
	return userThemeResponseWithContract(method, body, ifMatch, principal, true)
}

func userThemeLegacyResponse(method, body, ifMatch string, principal *models.User) *httptest.ResponseRecorder {
	return userThemeResponseWithContract(method, body, ifMatch, principal, false)
}

func userThemeResponseWithContract(
	method, body, ifMatch string,
	principal *models.User,
	canonical bool,
) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Params = gin.Params{{Key: "group", Value: service.ThemeConfigGroup}}
	ctx.Request = httptest.NewRequest(method, "/admin/api/user-configs/theme", bytes.NewBufferString(body))
	ctx.Request.Header.Set("Content-Type", "application/json")
	if ifMatch != "" {
		ctx.Request.Header.Set("If-Match", ifMatch)
	}
	if canonical {
		ctx.Request.Header.Set("Accept", themeContractMediaType)
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

func TestThemeContractAcceptNegotiationUsesExactMediaType(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, test := range []struct {
		name      string
		accept    string
		canonical bool
	}{
		{name: "exact", accept: themeContractMediaType, canonical: true},
		{name: "with quality", accept: themeContractMediaType + "; q=0.9", canonical: true},
		{name: "quoted comma parameter", accept: themeContractMediaType + `; profile="a,b"; q=0.5`, canonical: true},
		{name: "among alternatives", accept: "application/json, " + themeContractMediaType, canonical: true},
		{name: "zero quality", accept: themeContractMediaType + "; q=0", canonical: false},
		{name: "zero decimal quality", accept: themeContractMediaType + "; q=0.000", canonical: false},
		{name: "invalid quality", accept: themeContractMediaType + "; q=not-a-number", canonical: false},
		{name: "quality above one", accept: themeContractMediaType + "; q=1.1", canonical: false},
		{name: "quality precision overflow", accept: themeContractMediaType + "; q=0.0001", canonical: false},
		{name: "substring is legacy", accept: "application/vnd.mss.theme.v1+json-extra", canonical: false},
		{name: "ordinary json is legacy", accept: "application/json", canonical: false},
		{name: "missing is legacy", canonical: false},
	} {
		t.Run(test.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(recorder)
			ctx.Request = httptest.NewRequest(http.MethodGet, "/admin/api/app-configs/theme", nil)
			if test.accept != "" {
				ctx.Request.Header.Set("Accept", test.accept)
			}
			require.Equal(t, test.canonical, wantsCanonicalThemeContract(ctx))
			require.Contains(t, recorder.Header().Values("Vary"), "Accept")
		})
	}
}

func TestThemeResponsesUseNegotiatedContentType(t *testing.T) {
	setupThemeConfigAPITest(t)
	gin.SetMode(gin.TestMode)

	canonicalSuccess := appThemeResponse(http.MethodGet, "", "")
	require.Equal(t, http.StatusOK, canonicalSuccess.Code, canonicalSuccess.Body.String())
	require.Equal(t, themeContractMediaType, canonicalSuccess.Header().Get("Content-Type"))

	canonicalUpdate := appThemeResponse(
		http.MethodPut,
		`{"data":{"fixedHeader":false}}`,
		`"theme-application-0"`,
	)
	require.Equal(t, http.StatusOK, canonicalUpdate.Code, canonicalUpdate.Body.String())
	require.Equal(t, themeContractMediaType, canonicalUpdate.Header().Get("Content-Type"))

	canonicalConflict := appThemeResponse(
		http.MethodPut,
		`{"data":{"layout":"side"}}`,
		`"theme-application-0"`,
	)
	require.Equal(t, http.StatusPreconditionFailed, canonicalConflict.Code, canonicalConflict.Body.String())
	require.Equal(t, themeContractMediaType, canonicalConflict.Header().Get("Content-Type"))

	legacySuccess := appThemeLegacyResponse(http.MethodGet, "", "")
	require.Equal(t, http.StatusOK, legacySuccess.Code, legacySuccess.Body.String())
	require.Equal(t, "application/json; charset=utf-8", legacySuccess.Header().Get("Content-Type"))

	legacyConflict := appThemeLegacyResponse(
		http.MethodPut,
		`{"data":{"layout":"mix"}}`,
		`"theme-application-0"`,
	)
	require.Equal(t, http.StatusPreconditionFailed, legacyConflict.Code, legacyConflict.Body.String())
	require.Equal(t, "application/json; charset=utf-8", legacyConflict.Header().Get("Content-Type"))

	zeroQuality := appThemeResponseWithAccept(
		http.MethodGet,
		"",
		"",
		themeContractMediaType+"; q=0",
	)
	require.Equal(t, http.StatusOK, zeroQuality.Code, zeroQuality.Body.String())
	require.Equal(t, "application/json; charset=utf-8", zeroQuality.Header().Get("Content-Type"))
	require.NotContains(t, zeroQuality.Body.String(), `"_meta"`)
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

func TestLegacyAppThemeGETPutGETPreservesPWA(t *testing.T) {
	db := setupThemeConfigAPITest(t)
	gin.SetMode(gin.TestMode)

	response := appThemeLegacyResponse(http.MethodGet, "", "")
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	require.Equal(t, `"theme-application-0"`, response.Header().Get("ETag"))
	require.Equal(t, "no-store", response.Header().Get("Cache-Control"))
	require.Contains(t, response.Header().Values("Vary"), "Accept")

	response = appThemeLegacyResponse(
		http.MethodPut,
		`{"data":{"pwa":true,"layout":"side"}}`,
		`"theme-application-0"`,
	)
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	require.Equal(t, `"theme-application-1"`, response.Header().Get("ETag"))
	require.Equal(t, "no-store", response.Header().Get("Cache-Control"))
	require.Contains(t, response.Header().Values("Vary"), "Accept")
	require.Contains(t, response.Body.String(), `"pwa":true`)
	require.Contains(t, response.Body.String(), `"layout":"side"`)
	require.NotContains(t, response.Body.String(), `"_meta"`)

	response = appThemeLegacyResponse(http.MethodGet, "", "")
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	require.Contains(t, response.Body.String(), `"pwa":true`)
	require.Contains(t, response.Body.String(), `"layout":"side"`)
	require.NotContains(t, response.Body.String(), `"_meta"`)

	// The same persisted state projected through the canonical media type has
	// seven fields plus metadata and intentionally excludes pwa.
	response = appThemeResponse(http.MethodGet, "", "")
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	require.NotContains(t, response.Body.String(), `"pwa"`)
	application := decodeThemeResource(t, response)
	require.Equal(t, "side", *application.Layout)
	require.Equal(t, "1", application.Meta.Revision)

	var applicationPWA models.AppConfig
	require.NoError(t, db.First(&applicationPWA, `"group" = ? AND name = ?`, service.ThemeConfigGroup, "pwa").Error)
	require.Equal(t, "true", applicationPWA.Value)
}

func TestLegacyApplicationThemeGETKeepsPWAAndStrongETagOnOneRevision(t *testing.T) {
	db := setupThemeConfigAPITest(t)
	gin.SetMode(gin.TestMode)

	require.NoError(t, db.Create(&[]models.AppConfig{
		{Group: service.ThemeConfigGroup, Name: "layout", Value: "side"},
		{Group: service.ThemeConfigGroup, Name: "pwa", Value: "false"},
	}).Error)
	require.NoError(t, db.Create(&models.ConfigRevision{
		Scope: service.ThemeScopeApplication, Resource: "theme", Revision: 1,
	}).Error)

	callbackName := "test:legacy-theme-revision-flip"
	revisionReads := 0
	var mutationErr error
	require.NoError(t, db.Callback().Query().After("gorm:query").Register(callbackName, func(tx *gorm.DB) {
		if tx.Statement == nil || tx.Statement.Table != (&models.ConfigRevision{}).TableName() {
			return
		}
		revisionReads++
		if revisionReads != 2 {
			return
		}
		mutationErr = db.Model(&models.AppConfig{}).
			Where(&models.AppConfig{Group: service.ThemeConfigGroup, Name: "pwa"}).
			UpdateColumn("value", "true").Error
		if mutationErr == nil {
			mutationErr = db.Model(&models.ConfigRevision{}).
				Where("scope = ? AND owner_id = ? AND resource = ?", service.ThemeScopeApplication, "", "theme").
				UpdateColumn("revision", 2).Error
		}
	}))
	var removeOnce sync.Once
	removeCallback := func() {
		removeOnce.Do(func() {
			require.NoError(t, db.Callback().Query().Remove(callbackName))
		})
	}
	t.Cleanup(removeCallback)

	response := appThemeLegacyResponse(http.MethodGet, "", "")
	removeCallback()
	require.NoError(t, mutationErr)
	require.GreaterOrEqual(t, revisionReads, 2)
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	require.Equal(t, `"theme-application-1"`, response.Header().Get("ETag"))

	var projection map[string]any
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &projection))
	require.Equal(t, "side", projection["layout"])
	require.Equal(t, false, projection["pwa"], "pwa must belong to the revision named by the ETag")

	var storedPWA models.AppConfig
	require.NoError(t, db.First(
		&storedPWA,
		`"group" = ? AND name = ?`,
		service.ThemeConfigGroup,
		"pwa",
	).Error)
	require.Equal(t, "true", storedPWA.Value)
	var storedRevision models.ConfigRevision
	require.NoError(t, db.First(
		&storedRevision,
		"scope = ? AND owner_id = ? AND resource = ?",
		service.ThemeScopeApplication,
		"",
		"theme",
	).Error)
	require.EqualValues(t, 2, storedRevision.Revision)
}

func TestLegacyApplicationThemeMutationsDoNotReadPWAAfterCommit(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("put", func(t *testing.T) {
		db := setupThemeConfigAPITest(t)
		outsideQueries, removeCallback := registerOutsideTransactionPWAQueryFailure(t, db)

		response := appThemeLegacyResponse(
			http.MethodPut,
			`{"data":{"pwa":true,"layout":"side"}}`,
			`"theme-application-0"`,
		)
		removeCallback()
		require.Equal(t, http.StatusOK, response.Code, response.Body.String())
		require.Equal(t, `"theme-application-1"`, response.Header().Get("ETag"))
		require.Zero(t, *outsideQueries, "legacy response must be formed before the transaction commits")

		var projection map[string]any
		require.NoError(t, json.Unmarshal(response.Body.Bytes(), &projection))
		require.Equal(t, true, projection["pwa"])
		require.Equal(t, "side", projection["layout"])
	})

	t.Run("delete", func(t *testing.T) {
		db := setupThemeConfigAPITest(t)
		seed := appThemeLegacyResponse(
			http.MethodPut,
			`{"data":{"pwa":true,"layout":"side"}}`,
			`"theme-application-0"`,
		)
		require.Equal(t, http.StatusOK, seed.Code, seed.Body.String())

		outsideQueries, removeCallback := registerOutsideTransactionPWAQueryFailure(t, db)
		response := appThemeLegacyResponse(http.MethodDelete, "", seed.Header().Get("ETag"))
		removeCallback()
		require.Equal(t, http.StatusOK, response.Code, response.Body.String())
		require.Equal(t, `"theme-application-2"`, response.Header().Get("ETag"))
		require.Zero(t, *outsideQueries, "legacy response must be formed before the transaction commits")

		var projection map[string]any
		require.NoError(t, json.Unmarshal(response.Body.Bytes(), &projection))
		require.Equal(t, true, projection["pwa"], "legacy pwa remains outside the canonical reset set")
		require.NotContains(t, projection, "layout")
	})
}

func TestUserThemeRejectsLegacyPWAWithAndWithoutCanonicalAccept(t *testing.T) {
	db := setupThemeConfigAPITest(t)
	gin.SetMode(gin.TestMode)

	previousIdentityKey := config.Cfg.Auth.IdentityKey
	config.Cfg.Auth.IdentityKey = "theme-legacy-pwa-identity"
	t.Cleanup(func() { config.Cfg.Auth.IdentityKey = previousIdentityKey })
	principal := &models.User{}
	principal.ID = "user-a"
	for _, request := range []func(string, string, string, *models.User) *httptest.ResponseRecorder{
		userThemeResponse,
		userThemeLegacyResponse,
	} {
		response := request(
			http.MethodPut,
			`{"data":{"pwa":false,"colorWeak":false}}`,
			`"theme-user-0"`,
			principal,
		)
		require.Equal(t, http.StatusUnprocessableEntity, response.Code, response.Body.String())
	}

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
	require.Contains(t, response.Header().Values("Vary"), "Accept")
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

	// Omitting If-Match keeps old clients working and still returns the new
	// canonical representation and revision.
	response = appThemeResponse(http.MethodPut, `{"data":{"layout":"mix"}}`, "")
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	require.Equal(t, `"theme-application-2"`, response.Header().Get("ETag"))

	response = appThemeResponse(http.MethodDelete, "", `"theme-application-2"`)
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	require.Equal(t, `"theme-application-3"`, response.Header().Get("ETag"))
	resource = decodeThemeResource(t, response)
	require.Equal(t, "3", resource.Meta.Revision)
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

	legacyResponse := userThemeLegacyResponse(http.MethodGet, "", "", principal)
	require.Equal(t, http.StatusOK, legacyResponse.Code, legacyResponse.Body.String())
	require.Equal(t, `"theme-user-0"`, legacyResponse.Header().Get("ETag"))
	require.Equal(t, "no-store", legacyResponse.Header().Get("Cache-Control"))
	require.Contains(t, legacyResponse.Header().Values("Vary"), "Accept")
	require.NotContains(t, legacyResponse.Body.String(), `"_meta"`)

	response := userThemeResponse(http.MethodGet, "", "", principal)
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	require.Equal(t, `"theme-user-0"`, response.Header().Get("ETag"))
	require.Equal(t, "no-store", response.Header().Get("Cache-Control"))
	require.Contains(t, response.Header().Values("Vary"), "Accept")

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
	ctx.Request.Header.Set("Accept", themeContractMediaType)
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
