package server

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/mss-boot-io/mss-boot-admin/admin/config"
	"github.com/mss-boot-io/mss-boot-admin/admin/middleware"
	"github.com/mss-boot-io/mss-boot-admin/admin/models"
	"github.com/mss-boot-io/mss-boot-admin/admin/modules/supplier"
	adminpkg "github.com/mss-boot-io/mss-boot-admin/admin/pkg"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/config/gormdb"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/migration"
	migrationmodels "github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/migration/models"
)

const supplierCompositionIdentityKey = "supplier.composition.identity"

func TestSupplierProductionComposition(t *testing.T) {
	t.Run("mounts declared routes with Admin authorizer", testSupplierCompositionRoutes)
	t.Run("requires both applied migrations", testSupplierCompositionMigrationReadiness)
}

func TestSupplierProductionCompositionAuditsAcceptedAndRejectedMutationsWithoutSecrets(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := openSupplierCompositionSQLite(t)
	applySupplierCompositionMigrations(t, db)
	if err := db.AutoMigrate(new(models.AuditLog)); err != nil {
		t.Fatalf("create Supplier audit table: %v", err)
	}

	var procurementRole models.Role
	if err := db.Where("name = ?", "procurement").Take(&procurementRole).Error; err != nil {
		t.Fatalf("load migrated procurement role: %v", err)
	}
	allowed := &models.User{UserLogin: models.UserLogin{RoleID: procurementRole.ID}}
	allowed.ID = "supplier-audit-allowed"
	allowed.Username = "supplier-audit-allowed"
	denied := &models.User{UserLogin: models.UserLogin{RoleID: "role-without-policy"}}
	denied.ID = "supplier-audit-denied"
	denied.Username = "supplier-audit-denied"

	previousDB := gormdb.DB
	previousIdentityKey := config.Cfg.Auth.IdentityKey
	gormdb.DB = db
	config.Cfg.Auth.IdentityKey = supplierCompositionIdentityKey
	t.Cleanup(func() {
		gormdb.DB = previousDB
		config.Cfg.Auth.IdentityKey = previousIdentityKey
	})

	authentication := func(ctx *gin.Context) {
		switch ctx.GetHeader("X-Test-Principal") {
		case "allowed":
			ctx.Set(supplierCompositionIdentityKey, allowed)
		case "denied":
			ctx.Set(supplierCompositionIdentityKey, denied)
		}
		ctx.Next()
	}
	engine := gin.New()
	engine.Use(middleware.AuditLogMiddleware("/admin/api/auth", "/admin/api/login", "/admin/api/logout"))
	if err := mountSupplierRoutesWithDependencies(
		t.Context(),
		db,
		engine.Group("/admin"),
		supplierRouteDependencies{
			authentication: authentication,
			principal:      middleware.GetVerify,
			events:         supplierDomainEventLogger{},
		},
	); err != nil {
		t.Fatalf("mount Supplier production composition: %v", err)
	}

	acceptedBody := `{"code":"AUDIT_OK","name":"Audit Supplier","contactName":"Owner","creditLevel":"normal"}`
	acceptedRequest := httptest.NewRequest(http.MethodPost, "/admin/api/suppliers", strings.NewReader(acceptedBody))
	acceptedRequest.Header.Set("Content-Type", "application/json")
	acceptedRequest.Header.Set("X-Test-Principal", "allowed")
	acceptedResponse := httptest.NewRecorder()
	engine.ServeHTTP(acceptedResponse, acceptedRequest)
	if acceptedResponse.Code != http.StatusCreated {
		t.Fatalf("accepted Supplier mutation = %d, want %d; body=%s", acceptedResponse.Code, http.StatusCreated, acceptedResponse.Body.String())
	}

	deniedBody := `{"code":"AUDIT_DENIED","name":"Denied Supplier","contactName":"Owner","creditLevel":"normal","password":"never-store-password","accessToken":"never-store-token"}`
	deniedRequest := httptest.NewRequest(http.MethodPost, "/admin/api/suppliers", strings.NewReader(deniedBody))
	deniedRequest.Header.Set("Content-Type", "application/json")
	deniedRequest.Header.Set("X-Test-Principal", "denied")
	deniedResponse := httptest.NewRecorder()
	engine.ServeHTTP(deniedResponse, deniedRequest)
	if deniedResponse.Code != http.StatusForbidden {
		t.Fatalf("rejected Supplier mutation = %d, want %d; body=%s", deniedResponse.Code, http.StatusForbidden, deniedResponse.Body.String())
	}

	var audits []models.AuditLog
	if err := db.Order("created_at ASC").Find(&audits).Error; err != nil {
		t.Fatalf("load Supplier audits: %v", err)
	}
	if len(audits) != 2 {
		t.Fatalf("Supplier audit records = %d, want 2", len(audits))
	}
	for index, audit := range audits {
		if audit.Method != http.MethodPost || audit.Path != "/admin/api/suppliers" ||
			audit.Resource != "/admin/api/suppliers" || audit.Action != "POST /admin/api/suppliers" {
			t.Fatalf("Supplier audit %d metadata = %#v", index, audit)
		}
		if audit.Duration < 0 {
			t.Fatalf("Supplier audit %d duration = %d, want non-negative", index, audit.Duration)
		}
		if strings.Contains(audit.Request, "never-store-password") || strings.Contains(audit.Request, "never-store-token") {
			t.Fatalf("Supplier audit %d retained a secret: %s", index, audit.Request)
		}
	}
	if audits[0].UserID != allowed.ID || audits[0].Status != "enabled" {
		t.Fatalf("accepted Supplier audit = %#v", audits[0])
	}
	if audits[1].UserID != denied.ID || audits[1].Status != "disabled" || !strings.Contains(audits[1].Request, "[REDACTED]") {
		t.Fatalf("rejected Supplier audit = %#v", audits[1])
	}
}

func testSupplierCompositionRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := openSupplierCompositionSQLite(t)
	applySupplierCompositionMigrations(t, db)

	var procurementRole models.Role
	if err := db.Where("name = ?", "procurement").Take(&procurementRole).Error; err != nil {
		t.Fatalf("load migrated procurement role: %v", err)
	}
	allowed := &models.User{UserLogin: models.UserLogin{RoleID: procurementRole.ID}}
	denied := &models.User{UserLogin: models.UserLogin{RoleID: "role-without-policy"}}

	previousIdentityKey := config.Cfg.Auth.IdentityKey
	config.Cfg.Auth.IdentityKey = supplierCompositionIdentityKey
	t.Cleanup(func() { config.Cfg.Auth.IdentityKey = previousIdentityKey })

	authentication := func(ctx *gin.Context) {
		switch ctx.GetHeader("X-Test-Principal") {
		case "allowed":
			ctx.Set(supplierCompositionIdentityKey, allowed)
		case "denied":
			ctx.Set(supplierCompositionIdentityKey, denied)
		}
		ctx.Next()
	}
	engine := gin.New()
	if err := mountSupplierRoutesWithDependencies(
		t.Context(),
		db,
		engine.Group("/admin"),
		supplierRouteDependencies{
			authentication: authentication,
			principal:      middleware.GetVerify,
			events:         supplierDomainEventLogger{},
		},
	); err != nil {
		t.Fatalf("mount Supplier production composition: %v", err)
	}

	routes := engine.Routes()
	if got, want := len(routes), len(supplier.OperationContracts()); got != want {
		t.Fatalf("mounted Supplier routes = %d, want %d", got, want)
	}
	mounted := make(map[string]struct{}, len(routes))
	for _, route := range routes {
		mounted[route.Method+" "+route.Path] = struct{}{}
	}
	for _, operation := range supplier.OperationContracts() {
		path := strings.ReplaceAll(operation.Path, "{id}", ":id")
		if _, ok := mounted[operation.Method+" "+path]; !ok {
			t.Errorf("declared route %s %s was not mounted", operation.Method, path)
		}
	}

	assertSupplierCompositionStatus(t, engine, "", http.StatusUnauthorized)
	assertSupplierCompositionStatus(t, engine, "denied", http.StatusForbidden)
	assertSupplierCompositionStatus(t, engine, "allowed", http.StatusOK)

	if err := db.Migrator().DropTable(new(models.CasbinRule)); err != nil {
		t.Fatalf("remove temporary policy table: %v", err)
	}
	assertSupplierCompositionStatus(t, engine, "allowed", http.StatusServiceUnavailable)
}

func testSupplierCompositionMigrationReadiness(t *testing.T) {
	gin.SetMode(gin.TestMode)
	if err := migration.Migrate.ValidateRegistrations(); err != nil {
		t.Fatalf("global migration import preflight: %v", err)
	}

	for _, missingVersion := range []string{
		supplier.SupplierMigrationID.String(),
		supplier.SupplierAuthorizationMigrationID.String(),
	} {
		t.Run(missingVersion, func(t *testing.T) {
			db := openSupplierCompositionSQLite(t)
			applySupplierCompositionMigrations(t, db)
			if err := db.Delete(
				new(migrationmodels.Migration),
				"version = ?",
				missingVersion,
			).Error; err != nil {
				t.Fatalf("remove temporary migration version: %v", err)
			}
			engine := gin.New()
			err := mountSupplierRoutesWithDependencies(
				t.Context(),
				db,
				engine.Group("/admin"),
				supplierRouteDependencies{
					authentication: func(ctx *gin.Context) { ctx.Next() },
					principal:      middleware.GetVerify,
					events:         supplierDomainEventLogger{},
				},
			)
			if err == nil || !strings.Contains(err.Error(), missingVersion) {
				t.Fatalf("composition error = %v, want missing migration %s", err, missingVersion)
			}
			if got := len(engine.Routes()); got != 0 {
				t.Fatalf("routes mounted before migration readiness = %d, want 0", got)
			}
		})
	}
}

func assertSupplierCompositionStatus(
	t *testing.T,
	engine *gin.Engine,
	principal string,
	want int,
) {
	t.Helper()
	request := httptest.NewRequest(http.MethodGet, "/admin/api/suppliers", nil)
	if principal != "" {
		request.Header.Set("X-Test-Principal", principal)
	}
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, request)
	if response.Code != want {
		t.Fatalf("Supplier list as %q = %d, want %d; body=%s", principal, response.Code, want, response.Body.String())
	}
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
		t.Fatalf("open Supplier composition database handle: %v", err)
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
	if err := runner.ValidateRegistrations(); err != nil {
		t.Fatalf("validate Supplier migration registration: %v", err)
	}
	if err := runner.MigrateContext(t.Context()); err != nil {
		t.Fatalf("apply Supplier migrations: %v", err)
	}
	for _, version := range []string{
		supplier.SupplierMigrationID.String(),
		supplier.SupplierAuthorizationMigrationID.String(),
	} {
		var count int64
		if err := db.Model(new(migrationmodels.Migration)).
			Where("version = ?", version).
			Count(&count).Error; err != nil || count != 1 {
			t.Fatalf("Supplier migration %s count = %d, err=%v", version, count, err)
		}
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
