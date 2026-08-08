package middleware

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/mss-boot-io/mss-boot-admin/admin/config"
	"github.com/mss-boot-io/mss-boot-admin/admin/models"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/config/gormdb"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestSanitizeAuditRequestRedactsNestedSecrets(t *testing.T) {
	body := []byte(`{
		"source":"https://github.com/example/template",
		"status":"enabled",
		"password":"provider-password",
		"accessToken":"provider-token",
		"oauth":{"refresh_token":"refresh-token","code":"oauth-code","state":"oauth-state"},
		"items":[{"clientSecret":"client-secret"},{"credential":"opaque-handle"}]
	}`)

	result := sanitizeAuditRequest(body)
	if result == "" {
		t.Fatal("sanitized JSON should retain safe audit metadata")
	}
	for _, secret := range []string{
		"provider-password",
		"provider-token",
		"refresh-token",
		"oauth-code",
		"oauth-state",
		"client-secret",
		"opaque-handle",
	} {
		if strings.Contains(result, secret) {
			t.Fatalf("sanitized audit request contains %q: %s", secret, result)
		}
	}

	var decoded map[string]any
	if err := json.Unmarshal([]byte(result), &decoded); err != nil {
		t.Fatalf("decode sanitized request: %v", err)
	}
	if decoded["source"] != "https://github.com/example/template" || decoded["status"] != "enabled" {
		t.Fatalf("safe metadata changed: %#v", decoded)
	}
	if decoded["password"] != auditRedactedValue || decoded["accessToken"] != auditRedactedValue {
		t.Fatalf("top-level credentials were not redacted: %#v", decoded)
	}
}

func TestMatchesAuditSkipPathUsesSegmentBoundary(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{path: "/admin/api/user", want: true},
		{path: "/admin/api/user/profile", want: true},
		{path: "/admin/api/user/oauth2/callback", want: true},
		{path: "/admin/api/user-configs/theme", want: false},
		{path: "/admin/api/users", want: false},
	}
	for _, test := range tests {
		if got := matchesAuditSkipPath(test.path, "/admin/api/user"); got != test.want {
			t.Errorf("matchesAuditSkipPath(%q) = %t, want %t", test.path, got, test.want)
		}
	}
}

func TestThemeAuditScopeFromPathRecognizesReservedAliases(t *testing.T) {
	tests := []struct {
		name      string
		path      string
		wantScope string
		wantOK    bool
	}{
		{name: "application canonical", path: applicationThemeMutationPath, wantScope: "application", wantOK: true},
		{name: "application uppercase", path: "/admin/api/app-configs/Theme", wantScope: "application", wantOK: true},
		{name: "application accent", path: "/admin/api/app-configs/thème", wantScope: "application", wantOK: true},
		{name: "application escaped accent", path: "/admin/api/app-configs/th%C3%A8me", wantScope: "application", wantOK: true},
		{name: "application trailing space", path: "/admin/api/app-configs/theme ", wantScope: "application", wantOK: true},
		{name: "application escaped trailing space", path: "/admin/api/app-configs/theme%20", wantScope: "application", wantOK: true},
		{name: "user uppercase", path: "/admin/api/user-configs/THEME", wantScope: "user", wantOK: true},
		{name: "unknown group", path: "/admin/api/app-configs/themer", wantOK: false},
		{name: "nested path", path: "/admin/api/app-configs/theme/extra", wantOK: false},
		{name: "escaped nested path", path: "/admin/api/app-configs/theme%2Fextra", wantOK: false},
		{name: "malformed escape", path: "/admin/api/app-configs/theme%zz", wantOK: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			gotScope, gotOK := themeAuditScopeFromPath(test.path)
			if gotOK != test.wantOK || gotScope != test.wantScope {
				t.Fatalf("themeAuditScopeFromPath(%q) = (%q, %t), want (%q, %t)", test.path, gotScope, gotOK, test.wantScope, test.wantOK)
			}
		})
	}
}

func TestAuditMiddlewareRecordsUserConfigThemeOutsideUserSkipBoundary(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open audit database: %v", err)
	}
	if err = db.AutoMigrate(&models.AuditLog{}); err != nil {
		t.Fatalf("migrate audit log: %v", err)
	}
	previousDB := gormdb.DB
	previousIdentityKey := config.Cfg.Auth.IdentityKey
	gormdb.DB = db
	config.Cfg.Auth.IdentityKey = "audit-theme-identity"
	t.Cleanup(func() {
		gormdb.DB = previousDB
		config.Cfg.Auth.IdentityKey = previousIdentityKey
	})

	principal := &models.User{}
	principal.ID = "audit-user"
	principal.Username = "audit-user"
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(config.Cfg.Auth.IdentityKey, principal)
		c.Next()
	})
	router.Use(AuditLogMiddleware("/admin/api/user"))
	router.PUT("/admin/api/user-configs/theme", func(c *gin.Context) { c.Status(http.StatusNoContent) })
	router.PUT("/admin/api/user/profile", func(c *gin.Context) { c.Status(http.StatusNoContent) })

	themeResponse := httptest.NewRecorder()
	themeRequest := httptest.NewRequest(
		http.MethodPut,
		"/admin/api/user-configs/theme",
		bytes.NewBufferString(`{"data":{"navTheme":"realDark","fixedHeader":false}}`),
	)
	themeRequest.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(themeResponse, themeRequest)
	if themeResponse.Code != http.StatusNoContent {
		t.Fatalf("theme response = %d", themeResponse.Code)
	}

	var audit models.AuditLog
	if err = db.First(&audit).Error; err != nil {
		t.Fatalf("load theme audit log: %v", err)
	}
	if audit.Path != "/admin/api/user-configs/theme" || audit.Method != http.MethodPut || audit.UserID != principal.ID {
		t.Fatalf("unexpected theme audit: %#v", audit)
	}
	if audit.Request != "" {
		t.Fatalf("theme audit request must not retain values: %s", audit.Request)
	}
	var metadata ThemeAuditMetadata
	if err = json.Unmarshal([]byte(audit.Message), &metadata); err != nil {
		t.Fatalf("decode fallback theme audit metadata: %v", err)
	}
	if metadata.Scope != "user" || metadata.Outcome != "success" {
		t.Fatalf("unexpected fallback theme audit metadata: %#v", metadata)
	}
	if len(metadata.ChangedKeys) != 2 || metadata.ChangedKeys[0] != "fixedHeader" || metadata.ChangedKeys[1] != "navTheme" {
		t.Fatalf("unexpected fallback theme changed keys: %#v", metadata.ChangedKeys)
	}

	skippedResponse := httptest.NewRecorder()
	skippedRequest := httptest.NewRequest(
		http.MethodPut,
		"/admin/api/user/profile",
		bytes.NewBufferString(`{"name":"updated"}`),
	)
	router.ServeHTTP(skippedResponse, skippedRequest)
	var count int64
	if err = db.Model(&models.AuditLog{}).Count(&count).Error; err != nil {
		t.Fatalf("count audit logs: %v", err)
	}
	if count != 1 {
		t.Fatalf("audit count = %d, want only the user-config theme mutation", count)
	}
}

func TestAuditMiddlewareStoresValueFreeThemeMetadata(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open audit database: %v", err)
	}
	if err = db.AutoMigrate(&models.AuditLog{}); err != nil {
		t.Fatalf("migrate audit log: %v", err)
	}
	previousDB := gormdb.DB
	previousIdentityKey := config.Cfg.Auth.IdentityKey
	gormdb.DB = db
	config.Cfg.Auth.IdentityKey = "audit-theme-metadata-identity"
	t.Cleanup(func() {
		gormdb.DB = previousDB
		config.Cfg.Auth.IdentityKey = previousIdentityKey
	})

	principal := &models.User{}
	principal.ID = "audit-user"
	principal.Username = "audit-user"
	gGin := gin.New()
	gGin.Use(func(c *gin.Context) {
		c.Set(config.Cfg.Auth.IdentityKey, principal)
		c.Next()
	})
	gGin.Use(AuditLogMiddleware())
	gGin.PUT("/admin/api/app-configs/theme", func(c *gin.Context) {
		SetThemeAuditMetadata(c, ThemeAuditMetadata{
			Scope:       "application",
			ChangedKeys: []string{"colorPrimary", "fixedHeader"},
			Outcome:     "success",
			Revision:    "7",
		})
		c.Status(http.StatusOK)
	})

	request := httptest.NewRequest(
		http.MethodPut,
		"/admin/api/app-configs/theme",
		bytes.NewBufferString(`{"data":{"colorPrimary":"#123456","fixedHeader":true}}`),
	)
	response := httptest.NewRecorder()
	gGin.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("theme response = %d", response.Code)
	}

	var audit models.AuditLog
	if err = db.First(&audit).Error; err != nil {
		t.Fatalf("load theme audit log: %v", err)
	}
	if audit.Request != "" {
		t.Fatalf("theme audit must not retain submitted values: %s", audit.Request)
	}
	if strings.Contains(audit.Message, "#123456") || strings.Contains(audit.Message, "true") {
		t.Fatalf("theme audit metadata contains submitted values: %s", audit.Message)
	}
	var metadata ThemeAuditMetadata
	if err = json.Unmarshal([]byte(audit.Message), &metadata); err != nil {
		t.Fatalf("decode theme audit metadata: %v", err)
	}
	if metadata.Scope != "application" || metadata.Outcome != "success" || metadata.Revision != "7" {
		t.Fatalf("unexpected theme audit metadata: %#v", metadata)
	}
	if len(metadata.ChangedKeys) != 2 || metadata.ChangedKeys[0] != "colorPrimary" || metadata.ChangedKeys[1] != "fixedHeader" {
		t.Fatalf("unexpected theme changed keys: %#v", metadata.ChangedKeys)
	}
}

func TestAuditMiddlewareThemeAuthorizationFailuresNeverStoreValues(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open audit database: %v", err)
	}
	if err = db.AutoMigrate(&models.AuditLog{}); err != nil {
		t.Fatalf("migrate audit log: %v", err)
	}
	previousDB := gormdb.DB
	previousIdentityKey := config.Cfg.Auth.IdentityKey
	gormdb.DB = db
	config.Cfg.Auth.IdentityKey = "audit-theme-auth-identity"
	t.Cleanup(func() {
		gormdb.DB = previousDB
		config.Cfg.Auth.IdentityKey = previousIdentityKey
	})

	principal := &models.User{}
	principal.ID = "forbidden-user"
	principal.Username = "forbidden-user"
	router := gin.New()
	router.Use(func(c *gin.Context) {
		if c.Request.URL.Path == userThemeMutationPath {
			c.Set(config.Cfg.Auth.IdentityKey, principal)
		}
		c.Next()
	})
	router.Use(AuditLogMiddleware())
	router.PUT(applicationThemeMutationPath, func(c *gin.Context) {
		// Simulate authentication rejecting the request before the theme
		// controller can attach explicit audit metadata.
		c.Status(http.StatusUnauthorized)
	})
	router.PUT(userThemeMutationPath, func(c *gin.Context) {
		// Simulate authorization rejecting an authenticated user before the
		// theme controller runs.
		c.Status(http.StatusForbidden)
	})

	tests := []struct {
		path        string
		body        string
		status      int
		scope       string
		outcome     string
		changedKeys []string
		userID      string
	}{
		{
			path:   applicationThemeMutationPath,
			body:   `{"data":{"navTheme":"realDark","colorPrimary":"#123456"}}`,
			status: http.StatusUnauthorized, scope: "application", outcome: "authentication_failed",
			changedKeys: []string{"colorPrimary", "navTheme"},
		},
		{
			path:   userThemeMutationPath,
			body:   `{"data":{"fixedHeader":true,"layout":"mix"}}`,
			status: http.StatusForbidden, scope: "user", outcome: "authorization_failed",
			changedKeys: []string{"fixedHeader", "layout"}, userID: principal.ID,
		},
	}
	for _, test := range tests {
		response := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPut, test.path, bytes.NewBufferString(test.body))
		request.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(response, request)
		if response.Code != test.status {
			t.Fatalf("%s response = %d, want %d", test.path, response.Code, test.status)
		}
	}

	var anonymousCount int64
	if err = db.Model(&models.AuditLog{}).
		Where("path = ?", applicationThemeMutationPath).
		Count(&anonymousCount).Error; err != nil {
		t.Fatalf("count anonymous theme audits: %v", err)
	}
	if anonymousCount != 0 {
		t.Fatalf("anonymous authentication failures must not create business audit rows, got %d", anonymousCount)
	}

	test := tests[1]
	var audit models.AuditLog
	if err = db.First(&audit, "path = ?", userThemeMutationPath).Error; err != nil {
		t.Fatalf("load forbidden theme authorization audit: %v", err)
	}
	if audit.Request != "" {
		t.Fatalf("%s audit retained submitted values: %s", test.path, audit.Request)
	}
	if audit.UserID != test.userID {
		t.Fatalf("%s audit user = %q, want %q", test.path, audit.UserID, test.userID)
	}
	if strings.Contains(audit.Message, "realDark") || strings.Contains(audit.Message, "#123456") ||
		strings.Contains(audit.Message, `"mix"`) || strings.Contains(audit.Message, "true") {
		t.Fatalf("%s audit metadata contains submitted values: %s", test.path, audit.Message)
	}
	var metadata ThemeAuditMetadata
	if err = json.Unmarshal([]byte(audit.Message), &metadata); err != nil {
		t.Fatalf("decode %s audit metadata: %v", test.path, err)
	}
	if metadata.Scope != test.scope || metadata.Outcome != test.outcome {
		t.Fatalf("unexpected %s audit metadata: %#v", test.path, metadata)
	}
	if len(metadata.ChangedKeys) != len(test.changedKeys) {
		t.Fatalf("%s changed keys = %#v, want %#v", test.path, metadata.ChangedKeys, test.changedKeys)
	}
	for i := range test.changedKeys {
		if metadata.ChangedKeys[i] != test.changedKeys[i] {
			t.Fatalf("%s changed keys = %#v, want %#v", test.path, metadata.ChangedKeys, test.changedKeys)
		}
	}
}

func TestAuditMiddlewareThemeAliasAuthorizationFailuresNeverStoreValues(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open audit database: %v", err)
	}
	if err = db.AutoMigrate(&models.AuditLog{}); err != nil {
		t.Fatalf("migrate audit log: %v", err)
	}
	previousDB := gormdb.DB
	previousIdentityKey := config.Cfg.Auth.IdentityKey
	gormdb.DB = db
	config.Cfg.Auth.IdentityKey = "audit-theme-alias-auth-identity"
	t.Cleanup(func() {
		gormdb.DB = previousDB
		config.Cfg.Auth.IdentityKey = previousIdentityKey
	})

	principal := &models.User{}
	principal.ID = "forbidden-alias-user"
	principal.Username = "forbidden-alias-user"
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(config.Cfg.Auth.IdentityKey, principal)
		c.Next()
	})
	router.Use(AuditLogMiddleware())
	router.PUT("/admin/api/app-configs/:group", func(c *gin.Context) {
		c.Status(http.StatusForbidden)
	})
	router.PUT("/admin/api/user-configs/:group", func(c *gin.Context) {
		c.Status(http.StatusForbidden)
	})

	tests := []struct {
		name       string
		requestURL string
		storedPath string
		scope      string
	}{
		{name: "application uppercase", requestURL: "/admin/api/app-configs/Theme", storedPath: "/admin/api/app-configs/Theme", scope: "application"},
		{name: "application accent", requestURL: "/admin/api/app-configs/th%C3%A8me", storedPath: "/admin/api/app-configs/thème", scope: "application"},
		{name: "application trailing space", requestURL: "/admin/api/app-configs/theme%20", storedPath: "/admin/api/app-configs/theme ", scope: "application"},
		{name: "user uppercase", requestURL: "/admin/api/user-configs/Theme", storedPath: "/admin/api/user-configs/Theme", scope: "user"},
		{name: "user accent", requestURL: "/admin/api/user-configs/th%C3%A8me", storedPath: "/admin/api/user-configs/thème", scope: "user"},
		{name: "user trailing space", requestURL: "/admin/api/user-configs/theme%20", storedPath: "/admin/api/user-configs/theme ", scope: "user"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			response := httptest.NewRecorder()
			request := httptest.NewRequest(
				http.MethodPut,
				test.requestURL,
				bytes.NewBufferString(`{"data":{"navTheme":"realDark","colorPrimary":"#123456"}}`),
			)
			request.Header.Set("Content-Type", "application/json")
			router.ServeHTTP(response, request)
			if response.Code != http.StatusForbidden {
				t.Fatalf("response = %d, want %d", response.Code, http.StatusForbidden)
			}

			var audit models.AuditLog
			if err := db.Last(&audit, "path = ?", test.storedPath).Error; err != nil {
				t.Fatalf("load alias audit: %v", err)
			}
			if audit.UserID != principal.ID {
				t.Fatalf("audit user = %q, want %q", audit.UserID, principal.ID)
			}
			if audit.Request != "" {
				t.Fatalf("alias audit retained submitted values: %s", audit.Request)
			}
			if strings.Contains(audit.Message, "realDark") || strings.Contains(audit.Message, "#123456") {
				t.Fatalf("alias audit metadata contains submitted values: %s", audit.Message)
			}

			var metadata ThemeAuditMetadata
			if err := json.Unmarshal([]byte(audit.Message), &metadata); err != nil {
				t.Fatalf("decode alias audit metadata: %v", err)
			}
			if metadata.Scope != test.scope || metadata.Outcome != "authorization_failed" {
				t.Fatalf("unexpected alias audit metadata: %#v", metadata)
			}
			if len(metadata.ChangedKeys) != 2 || metadata.ChangedKeys[0] != "colorPrimary" || metadata.ChangedKeys[1] != "navTheme" {
				t.Fatalf("unexpected alias changed keys: %#v", metadata.ChangedKeys)
			}
		})
	}

	var count int64
	if err := db.Model(&models.AuditLog{}).Count(&count).Error; err != nil {
		t.Fatalf("count alias audit logs: %v", err)
	}
	if count != int64(len(tests)) {
		t.Fatalf("alias audit count = %d, want %d", count, len(tests))
	}
}

func TestSanitizeAuditRequestFailsClosedForOpaqueBodies(t *testing.T) {
	for _, body := range [][]byte{
		[]byte("password=secret&token=value"),
		[]byte("not-json"),
		nil,
	} {
		if result := sanitizeAuditRequest(body); result != "" {
			t.Fatalf("opaque body should not be audited, got %q", result)
		}
	}
}

func TestSensitiveAuditKeyDoesNotRedactOrdinaryStateWords(t *testing.T) {
	for _, key := range []string{"status", "statement", "source", "template"} {
		if isSensitiveAuditKey(key) {
			t.Fatalf("ordinary key %q should not be redacted", key)
		}
	}
	for _, key := range []string{"state", "access_token", "client-secret", "apiKey", "newPassword"} {
		if !isSensitiveAuditKey(key) {
			t.Fatalf("sensitive key %q should be redacted", key)
		}
	}
}
