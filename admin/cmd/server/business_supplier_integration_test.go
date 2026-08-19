package server

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/mss-boot-io/mss-boot-admin/admin/business"
	"github.com/mss-boot-io/mss-boot-admin/admin/center"
	"github.com/mss-boot-io/mss-boot-admin/admin/config"
	"github.com/mss-boot-io/mss-boot-admin/admin/middleware"
	"github.com/mss-boot-io/mss-boot-admin/admin/models"
	"github.com/mss-boot-io/mss-boot-admin/admin/modules/supplier"
	adminpkg "github.com/mss-boot-io/mss-boot-admin/admin/pkg"
	"github.com/mss-boot-io/mss-boot-admin/admin/router"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/migration"
	migrationmodels "github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/migration/models"
)

const supplierCompositionIdentityKey = "supplier.composition.identity"

type testBusinessEventCollector struct{}

func (testBusinessEventCollector) Collect(context.Context, business.Event) {}

func TestSupplierUsesGenericBusinessCompositionAndCurrentDatabaseLease(t *testing.T) {
	gin.SetMode(gin.TestMode)
	first := openSupplierCompositionSQLite(t)
	second := openSupplierCompositionSQLite(t)
	applySupplierCompositionMigrations(t, first)
	applySupplierCompositionMigrations(t, second)

	currentDatabase := first
	currentUser := supplierCompositionUser(t, first)
	engine, err := mountSupplierBusinessRegistry(
		t,
		func(operation func(*gorm.DB) error) error { return operation(currentDatabase) },
		func(ctx *gin.Context) {
			ctx.Set(supplierCompositionIdentityKey, currentUser)
			ctx.Next()
		},
	)
	if err != nil {
		t.Fatalf("mount generic Supplier composition: %v", err)
	}
	assertSupplierCompositionStatus(t, engine, http.StatusOK)

	currentDatabase = second
	currentUser = supplierCompositionUser(t, second)
	firstSQL, err := first.DB()
	if err != nil {
		t.Fatalf("resolve first database pool: %v", err)
	}
	if err := firstSQL.Close(); err != nil {
		t.Fatalf("close replaced database pool: %v", err)
	}
	assertSupplierCompositionStatus(t, engine, http.StatusOK)
}

func TestSupplierGenericCompositionPreservesAuthorizationAndDeclaredRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := openSupplierCompositionSQLite(t)
	applySupplierCompositionMigrations(t, db)

	var procurementRole models.Role
	if err := db.Where("name = ?", "procurement").Take(&procurementRole).Error; err != nil {
		t.Fatalf("load procurement role: %v", err)
	}
	allowed := &models.User{UserLogin: models.UserLogin{RoleID: procurementRole.ID}}
	denied := &models.User{UserLogin: models.UserLogin{RoleID: "role-without-policy"}}
	engine, err := mountSupplierBusinessRegistry(
		t,
		func(operation func(*gorm.DB) error) error { return operation(db) },
		func(ctx *gin.Context) {
			switch ctx.GetHeader("X-Test-Principal") {
			case "allowed":
				ctx.Set(supplierCompositionIdentityKey, allowed)
			case "denied":
				ctx.Set(supplierCompositionIdentityKey, denied)
			}
			ctx.Next()
		},
	)
	if err != nil {
		t.Fatalf("mount generic Supplier composition: %v", err)
	}

	mounted := make(map[string]struct{})
	for _, route := range engine.Routes() {
		mounted[route.Method+" "+route.Path] = struct{}{}
	}
	for _, operation := range supplier.OperationContracts() {
		path := strings.ReplaceAll(operation.Path, "{id}", ":id")
		if _, ok := mounted[operation.Method+" "+path]; !ok {
			t.Errorf("declared route %s %s was not mounted", operation.Method, path)
		}
	}

	assertSupplierPrincipalStatus(t, engine, "", http.StatusUnauthorized)
	assertSupplierPrincipalStatus(t, engine, "denied", http.StatusForbidden)
	assertSupplierPrincipalStatus(t, engine, "allowed", http.StatusOK)
	if err := db.Migrator().DropTable(new(models.CasbinRule)); err != nil {
		t.Fatalf("remove policy table: %v", err)
	}
	assertSupplierPrincipalStatus(t, engine, "allowed", http.StatusServiceUnavailable)
}

func TestSupplierRoutesAreNotMountedBeforeEveryMigrationIsReady(t *testing.T) {
	for _, missingVersion := range []string{
		supplier.SupplierMigrationID.String(),
		supplier.SupplierAuthorizationMigrationID.String(),
	} {
		t.Run(missingVersion, func(t *testing.T) {
			db := openSupplierCompositionSQLite(t)
			applySupplierCompositionMigrations(t, db)
			if err := db.Delete(new(migrationmodels.Migration), "version = ?", missingVersion).Error; err != nil {
				t.Fatalf("remove migration version: %v", err)
			}
			engine, err := mountSupplierBusinessRegistry(
				t,
				func(operation func(*gorm.DB) error) error { return operation(db) },
				func(ctx *gin.Context) { ctx.Next() },
			)
			if err == nil || !strings.Contains(err.Error(), missingVersion) {
				t.Fatalf("composition error = %v, want missing migration %s", err, missingVersion)
			}
			for _, route := range engine.Routes() {
				if strings.HasPrefix(route.Path, "/admin/api/suppliers") {
					t.Fatalf("business route mounted before readiness: %s %s", route.Method, route.Path)
				}
			}
		})
	}
}

func TestSupplierMutationsShareCoreCSRFSecurityBoundary(t *testing.T) {
	db := openSupplierCompositionSQLite(t)
	applySupplierCompositionMigrations(t, db)
	user := supplierCompositionUser(t, db)
	previousKey := config.Cfg.Auth.Key
	previousOrigin := config.Cfg.Application.Origin
	previousCORS := config.Cfg.CORS
	config.Cfg.Auth.Key = "supplier-csrf-test-key-at-least-32-bytes"
	config.Cfg.Application.Origin = "https://admin.example"
	config.Cfg.CORS.AllowOrigins = []string{"https://admin.example"}
	t.Cleanup(func() {
		config.Cfg.Auth.Key = previousKey
		config.Cfg.Application.Origin = previousOrigin
		config.Cfg.CORS = previousCORS
	})
	engine, err := mountSupplierBusinessRegistry(
		t,
		func(operation func(*gorm.DB) error) error { return operation(db) },
		func(ctx *gin.Context) {
			ctx.Set(supplierCompositionIdentityKey, user)
			ctx.Next()
		},
	)
	if err != nil {
		t.Fatalf("mount generic Supplier composition: %v", err)
	}

	mutation := func(code string) *http.Request {
		body := `{"code":"` + code + `","name":"CSRF Supplier","contactName":"Owner","creditLevel":"normal"}`
		request := httptest.NewRequest(http.MethodPost, "/admin/api/suppliers", strings.NewReader(body))
		request.Header.Set("Content-Type", "application/json")
		return request
	}
	session := "browser-session-token"

	missing := mutation("CSRF_MISSING")
	missing.AddCookie(&http.Cookie{Name: middleware.BrowserSessionCookieName, Value: session})
	assertRequestStatus(t, engine, missing, http.StatusForbidden)

	csrf := supplierCSRFFixture(session, config.Cfg.Auth.Key)
	valid := mutation("CSRF_VALID")
	valid.AddCookie(&http.Cookie{Name: middleware.BrowserSessionCookieName, Value: session})
	valid.AddCookie(&http.Cookie{Name: middleware.BrowserCSRFCookieName, Value: csrf})
	valid.Header.Set(middleware.BrowserCSRFHeaderName, csrf)
	valid.Header.Set("Origin", "https://admin.example")
	assertRequestStatus(t, engine, valid, http.StatusCreated)

	bearer := mutation("CSRF_BEARER")
	bearer.AddCookie(&http.Cookie{Name: middleware.BrowserSessionCookieName, Value: session})
	bearer.Header.Set("Authorization", "Bearer test-personal-access-token")
	assertRequestStatus(t, engine, bearer, http.StatusCreated)
}

func mountSupplierBusinessRegistry(
	t *testing.T,
	useDatabase databaseAccess,
	authentication gin.HandlerFunc,
) (*gin.Engine, error) {
	t.Helper()
	previousIdentityKey := config.Cfg.Auth.IdentityKey
	previousMode := config.Cfg.Application.Mode
	config.Cfg.Auth.IdentityKey = supplierCompositionIdentityKey
	config.Cfg.Application.Mode = config.ModeTest
	t.Cleanup(func() {
		config.Cfg.Auth.IdentityKey = previousIdentityKey
		config.Cfg.Application.Mode = previousMode
	})

	registry, err := business.Compose(migration.New(), supplier.Module())
	if err != nil {
		return nil, err
	}
	engine := gin.New()
	groups := router.InitRouteGroups(engine.Group("/admin"))
	protectedBusinessAPI := groups.ProtectedAPI.Group(
		"",
		leaseBusinessRequestDatabase(useDatabase),
		authentication,
	)
	err = registry.Mount(
		t.Context(),
		business.DatabaseAccess(useDatabase),
		protectedBusinessAPI,
		business.Runtime{
			RequestDatabase: center.RequestDatabase,
			Principal:       middleware.GetVerify,
			Events:          testBusinessEventCollector{},
		},
	)
	return engine, err
}

func assertSupplierCompositionStatus(t *testing.T, engine *gin.Engine, want int) {
	t.Helper()
	assertSupplierPrincipalStatus(t, engine, "", want)
}

func assertSupplierPrincipalStatus(t *testing.T, engine *gin.Engine, principal string, want int) {
	t.Helper()
	request := httptest.NewRequest(http.MethodGet, "/admin/api/suppliers", nil)
	if principal != "" {
		request.Header.Set("X-Test-Principal", principal)
	}
	assertRequestStatus(t, engine, request, want)
}

func assertRequestStatus(t *testing.T, engine *gin.Engine, request *http.Request, want int) {
	t.Helper()
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, request)
	if response.Code != want {
		t.Fatalf("%s %s = %d, want %d; body=%s", request.Method, request.URL.Path, response.Code, want, response.Body.String())
	}
}

func supplierCompositionUser(t *testing.T, db *gorm.DB) *models.User {
	t.Helper()
	var role models.Role
	if err := db.Where("name = ?", "procurement").Take(&role).Error; err != nil {
		t.Fatalf("load procurement role: %v", err)
	}
	return &models.User{UserLogin: models.UserLogin{RoleID: role.ID}}
}

func openSupplierCompositionSQLite(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := filepath.Join(t.TempDir(), "supplier-composition.db") + "?_busy_timeout=5000"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		t.Fatalf("open Supplier composition database: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("open Supplier database handle: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	return db
}

func applySupplierCompositionMigrations(t *testing.T, db *gorm.DB) {
	t.Helper()
	if err := db.AutoMigrate(
		new(migrationmodels.Migration),
		new(models.Role),
		new(models.Menu),
		new(models.CasbinRule),
		new(models.ConfigRevision),
	); err != nil {
		t.Fatalf("create Supplier migration prerequisites: %v", err)
	}
	runner := migration.New()
	runner.SetDb(db)
	runner.SetModel(new(migrationmodels.Migration))
	if err := supplier.RegisterMigration(runner); err != nil {
		t.Fatalf("register Supplier migrations: %v", err)
	}
	if err := runner.MigrateContext(t.Context()); err != nil {
		t.Fatalf("apply Supplier migrations: %v", err)
	}
	var policyCount int64
	if err := db.Model(new(models.CasbinRule)).Where(
		"ptype = ? AND v1 = ? AND v2 = ? AND v3 = ?",
		"p",
		adminpkg.APIAccessType.String(),
		"/admin/api/suppliers",
		http.MethodGet,
	).Count(&policyCount).Error; err != nil || policyCount == 0 {
		t.Fatalf("persisted Supplier list policies = %d, err=%v", policyCount, err)
	}
}

func supplierCSRFFixture(session, key string) string {
	nonce := make([]byte, 32)
	for index := range nonce {
		nonce[index] = byte(index + 1)
	}
	encodedNonce := base64.RawURLEncoding.EncodeToString(nonce)
	mac := hmac.New(sha256.New, []byte(key))
	_, _ = mac.Write([]byte("v1"))
	_, _ = mac.Write([]byte{0})
	_, _ = mac.Write([]byte(encodedNonce))
	_, _ = mac.Write([]byte{0})
	tokenDigest := sha256.Sum256([]byte(session))
	_, _ = mac.Write(tokenDigest[:])
	return "v1." + encodedNonce + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}
