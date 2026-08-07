package system

import (
	"context"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/casbin/casbin/v2"
	"github.com/mss-boot-io/mss-boot-admin/admin/models"
	"github.com/mss-boot-io/mss-boot-admin/admin/pkg"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/config/gormdb"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/enum"
	migrationmodels "github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/migration/models"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

const adminRoutePermissionsTestVersion = "20260806130000"

func TestAdminRoutePermissionMigrationFreshAndIdempotent(t *testing.T) {
	db, parentIDs := setupAdminRoutePermissionMigrationTest(t)
	visibleBefore := visibleMenuState(t, db)

	report, err := migrateAdminRoutePermissionsWithReport(db, adminRoutePermissionsTestVersion)
	if err != nil {
		t.Fatalf("run fresh migration: %v", err)
	}
	if len(report.AffectedRoles) != 0 {
		t.Fatalf("fresh migration affected roles = %#v, want none", report.AffectedRoles)
	}
	firstLogID := assertAdminLogMenu(t, db, parentIDs["/system"])
	assertAdminRouteComponents(t, db)
	assertAdminRoutePermissionRows(t, db)
	assertNoAdminRoutePolicies(t, db)
	assertExistingVisibleMenusUnchanged(t, db, visibleBefore)
	assertVisibleMenuCount(t, db, len(visibleBefore)+1)
	assertPageSelectionsHaveNoMutationAPIChildren(t, db)
	assertNoAdminAPIPermissionHasDirectoryParent(t, db)

	firstAPIIDs := adminAccessNodeIDs(t, db, pkg.APIAccessType)
	firstComponentIDs := adminAccessNodeIDs(t, db, pkg.ComponentAccessType)
	report, err = migrateAdminRoutePermissionsWithReport(db, adminRoutePermissionsTestVersion)
	if err != nil {
		t.Fatalf("rerun migration: %v", err)
	}
	if len(report.AffectedRoles) != 0 {
		t.Fatalf("fresh migration rerun affected roles = %#v, want none", report.AffectedRoles)
	}
	secondLogID := assertAdminLogMenu(t, db, parentIDs["/system"])
	assertAdminRouteComponents(t, db)
	assertAdminRoutePermissionRows(t, db)
	assertNoAdminRoutePolicies(t, db)
	assertExistingVisibleMenusUnchanged(t, db, visibleBefore)
	assertVisibleMenuCount(t, db, len(visibleBefore)+1)
	assertPageSelectionsHaveNoMutationAPIChildren(t, db)

	if firstLogID != secondLogID {
		t.Fatalf("/log menu ID changed on rerun: first %q, second %q", firstLogID, secondLogID)
	}
	if secondAPIIDs := adminAccessNodeIDs(t, db, pkg.APIAccessType); !reflect.DeepEqual(firstAPIIDs, secondAPIIDs) {
		t.Fatalf("API permission rows changed identity on rerun:\nfirst: %#v\nsecond: %#v", firstAPIIDs, secondAPIIDs)
	}
	if secondComponentIDs := adminAccessNodeIDs(t, db, pkg.ComponentAccessType); !reflect.DeepEqual(firstComponentIDs, secondComponentIDs) {
		t.Fatalf("component permission rows changed identity on rerun:\nfirst: %#v\nsecond: %#v", firstComponentIDs, secondComponentIDs)
	}

	var versionCount int64
	if err := db.Model(&migrationmodels.Migration{}).
		Where("version = ?", adminRoutePermissionsTestVersion).
		Count(&versionCount).Error; err != nil {
		t.Fatalf("count migration versions: %v", err)
	}
	if versionCount != 1 {
		t.Fatalf("migration version count = %d, want 1", versionCount)
	}
}

func TestAdminRoutePermissionMigrationRepairsExistingComponentAndAPI(t *testing.T) {
	db, parentIDs := setupAdminRoutePermissionMigrationTest(t)
	visibleBefore := visibleMenuState(t, db)

	component := &models.Menu{
		Name:       "legacy.password.reset.component",
		Path:       "/users/password-reset",
		Method:     "",
		ParentID:   parentIDs["/system"],
		Type:       pkg.ComponentAccessType,
		Permission: "/users/password-reset",
		Status:     enum.Disabled,
		HideInMenu: false,
	}
	component.ID = "legacy-password-reset-component"
	createAndSoftDeleteMenu(t, db, component)

	apiMenu := &models.Menu{
		Name:       "legacy.password.reset.api",
		Path:       "/admin/api/user/:userID/password-reset",
		Method:     "PUT",
		ParentID:   parentIDs["/system"],
		Type:       pkg.APIAccessType,
		Permission: "/users/password-reset",
		Status:     enum.Disabled,
		HideInMenu: false,
	}
	apiMenu.ID = "legacy-password-reset-api"
	createAndSoftDeleteMenu(t, db, apiMenu)

	staleRule := &models.CasbinRule{
		PType: "p",
		V0:    "role-stale-password-reset",
		V1:    pkg.ComponentAccessType.String(),
		V2:    "/users/password-reset",
		V3:    "GET",
	}
	if err := db.Create(staleRule).Error; err != nil {
		t.Fatalf("create stale component grant: %v", err)
	}

	report, err := migrateAdminRoutePermissionsWithReport(db, adminRoutePermissionsTestVersion)
	if err != nil {
		t.Fatalf("run upgrade migration: %v", err)
	}
	if len(report.AffectedRoles) != 0 {
		t.Fatalf("soft-deleted source inherited API grants: %#v", report.AffectedRoles)
	}

	repairedComponent := accessNodeByPath(t, db, "/users/password-reset", pkg.ComponentAccessType)
	if repairedComponent.ID != component.ID {
		t.Fatalf("repaired component ID = %q, want existing %q", repairedComponent.ID, component.ID)
	}
	if repairedComponent.ParentID != parentIDs["/users"] || repairedComponent.Method != "GET" {
		t.Fatalf("repaired component hierarchy/method is wrong: %+v", repairedComponent)
	}
	if repairedComponent.Name != "menu.origination.user.password-reset" ||
		repairedComponent.Permission != "user:password-reset" ||
		repairedComponent.Status != enum.Enabled || !repairedComponent.HideInMenu || repairedComponent.DeletedAt.Valid {
		t.Fatalf("repaired component metadata is unsafe: %+v", repairedComponent)
	}

	repairedAPI := accessAPIByRoute(t, db, "PUT", "/admin/api/user/:userID/password-reset")
	if repairedAPI.ID != apiMenu.ID {
		t.Fatalf("repaired API ID = %q, want existing %q", repairedAPI.ID, apiMenu.ID)
	}
	if repairedAPI.ParentID != repairedComponent.ID || repairedAPI.Permission != "user:password-reset" ||
		repairedAPI.Status != enum.Enabled || !repairedAPI.HideInMenu || repairedAPI.DeletedAt.Valid {
		t.Fatalf("repaired API metadata is unsafe: %+v", repairedAPI)
	}

	assertAdminLogMenu(t, db, parentIDs["/system"])
	assertAdminRouteComponents(t, db)
	assertAdminRoutePermissionRows(t, db)
	assertNoAdminRoutePolicies(t, db)
	assertExistingVisibleMenusUnchanged(t, db, visibleBefore)
	assertVisibleMenuCount(t, db, len(visibleBefore)+1)

	secondReport, err := migrateAdminRoutePermissionsWithReport(db, adminRoutePermissionsTestVersion)
	if err != nil {
		t.Fatalf("rerun repaired migration: %v", err)
	}
	if len(secondReport.AffectedRoles) != 0 {
		t.Fatalf("rerun inherited stale API grants: %#v", secondReport.AffectedRoles)
	}
	assertNoAdminRoutePolicies(t, db)
}

func TestAdminRoutePermissionMigrationPreservesExistingLogMenuCustomization(t *testing.T) {
	db, parentIDs := setupAdminRoutePermissionMigrationTest(t)

	logMenu := &models.Menu{
		Name:       "custom.log",
		Path:       "/log",
		Method:     "POST",
		Component:  "./CustomLog",
		Icon:       "warning",
		ParentID:   parentIDs["/origination"],
		Type:       pkg.MenuAccessType,
		Permission: "custom:log",
		Status:     enum.Disabled,
		HideInMenu: true,
		Sort:       1,
	}
	logMenu.ID = "custom-log-menu"
	if err := db.Session(&gorm.Session{SkipHooks: true}).Create(logMenu).Error; err != nil {
		t.Fatalf("create custom /log menu: %v", err)
	}
	visibleBefore := visibleMenuState(t, db)

	if err := _20260806130000AdminRoutePermissions(db, adminRoutePermissionsTestVersion); err != nil {
		t.Fatalf("run migration with custom /log: %v", err)
	}

	got := accessNodeByPath(t, db, "/log", pkg.MenuAccessType)
	if got.ID != logMenu.ID || got.Name != logMenu.Name || got.Method != logMenu.Method ||
		got.Component != logMenu.Component || got.Icon != logMenu.Icon || got.ParentID != logMenu.ParentID ||
		got.Permission != logMenu.Permission || got.Status != logMenu.Status ||
		got.HideInMenu != logMenu.HideInMenu || got.Sort != logMenu.Sort || got.DeletedAt.Valid {
		t.Fatalf("existing /log customization was overwritten:\n got: %+v\nwant: %+v", got, *logMenu)
	}
	assertNoAdminRoutePolicies(t, db)
	assertExistingVisibleMenusUnchanged(t, db, visibleBefore)
	assertVisibleMenuCount(t, db, len(visibleBefore))
}

func TestAdminRoutePermissionMigrationDoesNotReviveSoftDeletedLogMenu(t *testing.T) {
	db, parentIDs := setupAdminRoutePermissionMigrationTest(t)
	logMenu := &models.Menu{
		Name:       "deleted.log",
		Path:       "/log",
		Method:     "GET",
		Component:  "./Log",
		ParentID:   parentIDs["/system"],
		Type:       pkg.MenuAccessType,
		Permission: "audit:read",
		Status:     enum.Enabled,
	}
	logMenu.ID = "deleted-log-menu"
	createAndSoftDeleteMenu(t, db, logMenu)

	err := _20260806130000AdminRoutePermissions(db, adminRoutePermissionsTestVersion)
	if err == nil || !strings.Contains(err.Error(), "soft-deleted") {
		t.Fatalf("migration error = %v, want diagnostic soft-deleted /log error", err)
	}
	var got models.Menu
	if err := db.Unscoped().First(&got, "id = ?", logMenu.ID).Error; err != nil {
		t.Fatalf("load soft-deleted /log: %v", err)
	}
	if !got.DeletedAt.Valid {
		t.Fatal("soft-deleted /log was revived")
	}
	assertMigrationVersionCount(t, db, 0)
	assertSeededAccessNodeCount(t, db, 0, 0)
}

func TestAdminRoutePermissionMigrationMissingParentRollsBackWithoutVersion(t *testing.T) {
	db, _ := setupAdminRoutePermissionMigrationTest(t)
	if err := db.Session(&gorm.Session{SkipHooks: true}).Where("path = ?", "/task").Delete(&models.Menu{}).Error; err != nil {
		t.Fatalf("remove /task parent: %v", err)
	}

	err := _20260806130000AdminRoutePermissions(db, adminRoutePermissionsTestVersion)
	if err == nil || !strings.Contains(err.Error(), `missing MENU parent "/task"`) {
		t.Fatalf("migration error = %v, want missing /task MENU diagnostic", err)
	}
	assertMigrationVersionCount(t, db, 0)
	assertSeededAccessNodeCount(t, db, 0, 0)
	var logCount int64
	if err := db.Unscoped().Model(&models.Menu{}).
		Where("type = ? AND path = ?", pkg.MenuAccessType, "/log").
		Count(&logCount).Error; err != nil {
		t.Fatalf("count rolled-back /log: %v", err)
	}
	if logCount != 0 {
		t.Fatalf("rolled-back /log count = %d, want 0", logCount)
	}
}

func TestAdminRoutePermissionMigrationInheritsExactGrantsAndReloadsEnforcer(t *testing.T) {
	db, parentIDs := setupAdminRoutePermissionMigrationTest(t)
	createExistingLogPermissionNodes(t, db, parentIDs["/system"])
	departmentEdit := &models.Menu{
		Name:       "existing.department.edit",
		Path:       "/departments/edit",
		Method:     "GET",
		ParentID:   parentIDs["/departments"],
		Type:       pkg.ComponentAccessType,
		Permission: "department:update",
		Status:     enum.Enabled,
		HideInMenu: true,
	}
	departmentEdit.ID = "existing-department-edit"
	if err := db.Session(&gorm.Session{SkipHooks: true}).Create(departmentEdit).Error; err != nil {
		t.Fatalf("create active department edit component: %v", err)
	}

	for _, roleID := range []string{
		"role-audit-reader",
		"role-department-editor",
		"role-department-reader",
		"role-log-exporter",
		"role-language-reader",
		"role-runtime-reader",
		"role-wildcard",
		"role-wrong-method",
		"role-denied",
	} {
		role := &models.Role{Name: roleID, Status: enum.Enabled}
		role.ID = roleID
		if err := db.Session(&gorm.Session{SkipHooks: true}).Create(role).Error; err != nil {
			t.Fatalf("create role %q: %v", roleID, err)
		}
	}

	sourceRules := []models.CasbinRule{
		{PType: "p", V0: "role-department-reader", V1: pkg.MenuAccessType.String(), V2: "/departments", V3: "GET"},
		{PType: "p", V0: "role-department-editor", V1: pkg.ComponentAccessType.String(), V2: "/departments/edit", V3: "GET"},
		{PType: "p", V0: "role-audit-reader", V1: pkg.MenuAccessType.String(), V2: "/log", V3: "GET"},
		{PType: "p", V0: "role-runtime-reader", V1: pkg.ComponentAccessType.String(), V2: "/log/runtime", V3: "GET"},
		{PType: "p", V0: "role-log-exporter", V1: pkg.ComponentAccessType.String(), V2: "/log/export", V3: "GET"},
		{PType: "p", V0: "role-language-reader", V1: pkg.MenuAccessType.String(), V2: "/language", V3: "GET"},
		{PType: "p", V0: "default-role", V1: pkg.MenuAccessType.String(), V2: "/posts", V3: "GET"},
		// Wildcard paths and a method that does not authorize GET are not exact
		// upgrade sources and must not broaden the affected role set.
		{PType: "p", V0: "role-wildcard", V1: pkg.MenuAccessType.String(), V2: "/departments/*", V3: "GET"},
		{PType: "p", V0: "role-wrong-method", V1: pkg.MenuAccessType.String(), V2: "/system-config", V3: "POST"},
	}
	if err := db.Create(&sourceRules).Error; err != nil {
		t.Fatalf("create pre-upgrade exact grants: %v", err)
	}

	enforcer := newAdminRoutePermissionTestEnforcer(t, db)
	previousEnforcer := gormdb.Enforcer
	gormdb.Enforcer = enforcer
	t.Cleanup(func() { gormdb.Enforcer = previousEnforcer })
	assertEnforcerDecision(t, enforcer, false, "role-department-reader", pkg.APIAccessType.String(), "/admin/api/departments", "GET")

	report, err := migrateAdminRoutePermissionsWithReport(db, adminRoutePermissionsTestVersion)
	if err != nil {
		t.Fatalf("run upgrade migration: %v", err)
	}
	wantReport := []adminRoutePermissionAffectedRole{
		{RoleID: "default-role", SourceType: "MENU", SourcePath: "/posts", APIPath: "/admin/api/posts", Method: "GET"},
		{RoleID: "default-role", SourceType: "MENU", SourcePath: "/posts", APIPath: "/admin/api/posts/:id", Method: "GET"},
		{RoleID: "role-audit-reader", SourceType: "MENU", SourcePath: "/log", APIPath: "/admin/api/audit-logs/login", Method: "GET"},
		{RoleID: "role-audit-reader", SourceType: "MENU", SourcePath: "/log", APIPath: "/admin/api/audit-logs/operation", Method: "GET"},
		{RoleID: "role-department-editor", SourceType: "COMPONENT", SourcePath: "/departments/edit", APIPath: "/admin/api/departments/:id", Method: "PUT"},
		{RoleID: "role-department-reader", SourceType: "MENU", SourcePath: "/departments", APIPath: "/admin/api/departments", Method: "GET"},
		{RoleID: "role-department-reader", SourceType: "MENU", SourcePath: "/departments", APIPath: "/admin/api/departments/:id", Method: "GET"},
		{RoleID: "role-language-reader", SourceType: "MENU", SourcePath: "/language", APIPath: "/admin/api/languages", Method: "GET"},
		{RoleID: "role-log-exporter", SourceType: "COMPONENT", SourcePath: "/log/export", APIPath: "/admin/api/logs/export", Method: "GET"},
		{RoleID: "role-runtime-reader", SourceType: "COMPONENT", SourcePath: "/log/runtime", APIPath: "/admin/api/logs", Method: "GET"},
		{RoleID: "role-runtime-reader", SourceType: "COMPONENT", SourcePath: "/log/runtime", APIPath: "/admin/api/logs/files", Method: "GET"},
	}
	if !reflect.DeepEqual(report.AffectedRoles, wantReport) {
		t.Fatalf("affected-role report mismatch:\n got: %#v\nwant: %#v", report.AffectedRoles, wantReport)
	}

	// LoadPolicy is invoked by the migration after commit: allowed decisions
	// become visible immediately while unrelated roles/actions remain denied.
	assertEnforcerDecision(t, enforcer, true, "role-department-reader", "API", "/admin/api/departments", "GET")
	assertEnforcerDecision(t, enforcer, true, "role-department-reader", "API", "/admin/api/departments/:id", "GET")
	assertEnforcerDecision(t, enforcer, false, "role-department-reader", "API", "/admin/api/departments", "POST")
	assertEnforcerDecision(t, enforcer, true, "role-department-editor", "API", "/admin/api/departments/:id", "PUT")
	assertEnforcerDecision(t, enforcer, false, "role-department-editor", "API", "/admin/api/departments/:id", "GET")
	assertEnforcerDecision(t, enforcer, true, "role-audit-reader", "API", "/admin/api/audit-logs/login", "GET")
	assertEnforcerDecision(t, enforcer, false, "role-audit-reader", "API", "/admin/api/logs", "GET")
	assertEnforcerDecision(t, enforcer, true, "role-runtime-reader", "API", "/admin/api/logs", "GET")
	assertEnforcerDecision(t, enforcer, true, "role-runtime-reader", "API", "/admin/api/logs/files", "GET")
	assertEnforcerDecision(t, enforcer, false, "role-runtime-reader", "API", "/admin/api/logs/export", "GET")
	assertEnforcerDecision(t, enforcer, true, "role-log-exporter", "API", "/admin/api/logs/export", "GET")
	assertEnforcerDecision(t, enforcer, false, "role-log-exporter", "API", "/admin/api/logs", "GET")
	assertEnforcerDecision(t, enforcer, true, "role-language-reader", "API", "/admin/api/languages", "GET")
	assertEnforcerDecision(t, enforcer, false, "role-language-reader", "API", "/admin/api/languages", "POST")
	assertEnforcerDecision(t, enforcer, false, "role-denied", "API", "/admin/api/languages", "GET")
	assertEnforcerDecision(t, enforcer, false, "role-wildcard", "API", "/admin/api/departments", "GET")
	assertEnforcerDecision(t, enforcer, false, "role-wrong-method", "API", "/admin/api/system-configs", "GET")
	assertEnforcerDecision(t, enforcer, false, "role-denied", "API", "/admin/api/departments", "GET")
	assertEnforcerDecision(t, enforcer, true, "default-role", "API", "/admin/api/posts", "GET")
	assertEnforcerDecision(t, enforcer, false, "default-role", "API", "/admin/api/posts", "POST")

	secondReport, err := migrateAdminRoutePermissionsWithReport(db, adminRoutePermissionsTestVersion)
	if err != nil {
		t.Fatalf("rerun upgrade migration: %v", err)
	}
	if len(secondReport.AffectedRoles) != 0 {
		t.Fatalf("idempotent rerun affected roles = %#v, want none", secondReport.AffectedRoles)
	}
}

func setupAdminRoutePermissionMigrationTest(t *testing.T) (*gorm.DB, map[string]string) {
	t.Helper()
	databasePath := filepath.Join(t.TempDir(), "admin-route-permissions.db")
	db, err := gorm.Open(sqlite.Open(databasePath), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get SQL database: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	if err := db.AutoMigrate(
		&models.Role{},
		&models.Menu{},
		&models.CasbinRule{},
		&migrationmodels.Migration{},
	); err != nil {
		t.Fatalf("migrate test schema: %v", err)
	}

	parentIDs := map[string]string{}
	fixtures := []struct {
		id        string
		path      string
		parent    string
		menuType  pkg.AccessType
		component string
	}{
		{id: "menu-welcome", path: "/welcome", menuType: pkg.MenuAccessType, component: "./Welcome"},
		{id: "directory-origination", path: "/origination", menuType: pkg.DirectoryAccessType},
		{id: "menu-users", path: "/users", parent: "/origination", menuType: pkg.MenuAccessType, component: "./User"},
		{id: "menu-departments", path: "/departments", parent: "/origination", menuType: pkg.MenuAccessType, component: "./Department"},
		{id: "menu-posts", path: "/posts", parent: "/origination", menuType: pkg.MenuAccessType, component: "./Post"},
		{id: "directory-system", path: "/system", menuType: pkg.DirectoryAccessType},
		{id: "menu-task", path: "/task", parent: "/system", menuType: pkg.MenuAccessType, component: "./Task"},
		{id: "menu-language", path: "/language", parent: "/system", menuType: pkg.MenuAccessType, component: "./Language"},
		{id: "directory-super-permission", path: "/super-permission", menuType: pkg.DirectoryAccessType},
		{id: "menu-system-config", path: "/system-config", parent: "/super-permission", menuType: pkg.MenuAccessType, component: "./SystemConfig"},
	}
	for _, fixture := range fixtures {
		parentIDs[fixture.path] = fixture.id
		menu := &models.Menu{
			Name:       "fixture" + strings.ReplaceAll(fixture.path, "/", "."),
			Path:       fixture.path,
			Method:     "GET",
			ParentID:   parentIDs[fixture.parent],
			Type:       fixture.menuType,
			Permission: fixture.path,
			Component:  fixture.component,
			Status:     enum.Enabled,
		}
		menu.ID = fixture.id
		if err := db.Session(&gorm.Session{SkipHooks: true}).Create(menu).Error; err != nil {
			t.Fatalf("create visible menu fixture %q: %v", fixture.path, err)
		}
	}

	defaultRole := &models.Role{Name: "default-role", Status: enum.Enabled}
	defaultRole.ID = "default-role"
	if err := db.Session(&gorm.Session{SkipHooks: true}).Create(defaultRole).Error; err != nil {
		t.Fatalf("create default role fixture: %v", err)
	}
	if err := db.Exec(`UPDATE mss_boot_roles SET "default" = ? WHERE id = ?`, true, defaultRole.ID).Error; err != nil {
		t.Fatalf("mark default role fixture: %v", err)
	}

	return db, parentIDs
}

func createExistingLogPermissionNodes(t *testing.T, db *gorm.DB, systemID string) {
	t.Helper()
	rows := []*models.Menu{
		{
			Name:       "menu.system.log",
			Path:       "/log",
			Method:     "GET",
			Component:  "./Log",
			ParentID:   systemID,
			Type:       pkg.MenuAccessType,
			Permission: "audit:read",
			Status:     enum.Enabled,
		},
		{
			Name:       "legacy.log.runtime",
			Path:       "/log/runtime",
			Method:     "GET",
			ParentID:   "existing-log-menu",
			Type:       pkg.ComponentAccessType,
			Permission: "log:read",
			Status:     enum.Enabled,
			HideInMenu: true,
		},
		{
			Name:       "legacy.log.export",
			Path:       "/log/export",
			Method:     "GET",
			ParentID:   "existing-log-menu",
			Type:       pkg.ComponentAccessType,
			Permission: "log:export",
			Status:     enum.Enabled,
			HideInMenu: true,
		},
	}
	rows[0].ID = "existing-log-menu"
	rows[1].ID = "existing-log-runtime"
	rows[2].ID = "existing-log-export"
	for _, row := range rows {
		if err := db.Session(&gorm.Session{SkipHooks: true}).Create(row).Error; err != nil {
			t.Fatalf("create existing log permission node %q: %v", row.Path, err)
		}
	}
}

func newAdminRoutePermissionTestEnforcer(t *testing.T, db *gorm.DB) casbin.IEnforcer {
	t.Helper()
	type databaseInfo struct {
		Name string
		File string
	}
	var databases []databaseInfo
	if err := db.Raw("PRAGMA database_list").Scan(&databases).Error; err != nil {
		t.Fatalf("resolve SQLite test database: %v", err)
	}
	databasePath := ""
	for _, database := range databases {
		if database.Name == "main" {
			databasePath = database.File
			break
		}
	}
	if databasePath == "" {
		t.Fatal("SQLite test database has no main file")
	}

	config := gormdb.Database{
		Driver: "sqlite",
		Source: databasePath,
		CasbinModel: `[request_definition]
r = sub, tp, obj, act

[policy_definition]
p = sub, tp, obj, act

[policy_effect]
e = some(where (p.eft == allow))

[matchers]
m = r.sub == p.sub && r.tp == p.tp && keyMatch(r.obj, p.obj) && regexMatch(r.act, p.act)
	`,
	}
	handle, err := config.Open(context.Background())
	if err != nil {
		t.Fatalf("open Casbin test database handle: %v", err)
	}
	t.Cleanup(func() { _ = handle.Close() })
	if err := handle.Enforcer.LoadPolicy(); err != nil {
		t.Fatalf("load pre-upgrade Casbin policy: %v", err)
	}
	return handle.Enforcer
}

func assertEnforcerDecision(t *testing.T, enforcer casbin.IEnforcer, want bool, request ...any) {
	t.Helper()
	got, err := enforcer.Enforce(request...)
	if err != nil {
		t.Fatalf("enforce %#v: %v", request, err)
	}
	if got != want {
		t.Fatalf("enforce %#v = %t, want %t", request, got, want)
	}
}

func assertMigrationVersionCount(t *testing.T, db *gorm.DB, want int64) {
	t.Helper()
	var count int64
	if err := db.Model(&migrationmodels.Migration{}).
		Where("version = ?", adminRoutePermissionsTestVersion).
		Count(&count).Error; err != nil {
		t.Fatalf("count migration version: %v", err)
	}
	if count != want {
		t.Fatalf("migration version count = %d, want %d", count, want)
	}
}

func assertSeededAccessNodeCount(t *testing.T, db *gorm.DB, wantComponents, wantAPIs int64) {
	t.Helper()
	for accessType, want := range map[pkg.AccessType]int64{
		pkg.ComponentAccessType: wantComponents,
		pkg.APIAccessType:       wantAPIs,
	} {
		var count int64
		if err := db.Unscoped().Model(&models.Menu{}).Where("type = ?", accessType).Count(&count).Error; err != nil {
			t.Fatalf("count %s nodes: %v", accessType, err)
		}
		if count != want {
			t.Fatalf("%s node count = %d, want %d", accessType, count, want)
		}
	}
}

func createAndSoftDeleteMenu(t *testing.T, db *gorm.DB, menu *models.Menu) {
	t.Helper()
	if err := db.Session(&gorm.Session{SkipHooks: true}).Create(menu).Error; err != nil {
		t.Fatalf("create legacy menu %q: %v", menu.Path, err)
	}
	if err := db.Session(&gorm.Session{SkipHooks: true}).Delete(menu).Error; err != nil {
		t.Fatalf("soft-delete legacy menu %q: %v", menu.Path, err)
	}
}

func assertAdminLogMenu(t *testing.T, db *gorm.DB, systemID string) string {
	t.Helper()
	logMenu := accessNodeByPath(t, db, "/log", pkg.MenuAccessType)
	if logMenu.ParentID != systemID || logMenu.Name != "menu.system.log" || logMenu.Method != "GET" ||
		logMenu.Component != "./Log" || logMenu.Icon != "fileText" || logMenu.Permission != "audit:read" ||
		logMenu.Status != enum.Enabled || logMenu.HideInMenu || logMenu.Sort != 65 || logMenu.DeletedAt.Valid {
		t.Fatalf("/log menu metadata or hierarchy is wrong: %+v", logMenu)
	}
	var total int64
	if err := db.Unscoped().Model(&models.Menu{}).
		Where("type = ? AND path = ?", pkg.MenuAccessType, "/log").
		Count(&total).Error; err != nil {
		t.Fatalf("count /log menus: %v", err)
	}
	if total != 1 {
		t.Fatalf("/log menu count = %d, want 1", total)
	}
	return logMenu.ID
}

func assertAdminRouteComponents(t *testing.T, db *gorm.DB) {
	t.Helper()
	var count int64
	if err := db.Model(&models.Menu{}).Where("type = ?", pkg.ComponentAccessType).Count(&count).Error; err != nil {
		t.Fatalf("count component permissions: %v", err)
	}
	if count != int64(len(adminRouteComponentSeeds)) {
		t.Fatalf("component permission count = %d, want %d", count, len(adminRouteComponentSeeds))
	}
	for _, seed := range adminRouteComponentSeeds {
		component := accessNodeByPath(t, db, seed.Path, pkg.ComponentAccessType)
		parent := accessNodeByPath(t, db, seed.ParentPath, pkg.MenuAccessType)
		if component.ParentID != parent.ID || component.Name != seed.Name || component.Method != "GET" ||
			component.Permission != seed.Permission || component.Status != enum.Enabled || !component.HideInMenu || component.DeletedAt.Valid {
			t.Fatalf("component permission %q metadata or hierarchy is wrong: %+v", seed.Path, component)
		}
	}
}

func assertAdminRoutePermissionRows(t *testing.T, db *gorm.DB) {
	t.Helper()
	var count int64
	if err := db.Model(&models.Menu{}).Where("type = ?", pkg.APIAccessType).Count(&count).Error; err != nil {
		t.Fatalf("count API permission menus: %v", err)
	}
	if count != int64(len(adminRoutePermissionSeeds)) {
		t.Fatalf("API permission menu count = %d, want %d", count, len(adminRoutePermissionSeeds))
	}

	for _, seed := range adminRoutePermissionSeeds {
		row := accessAPIByRoute(t, db, seed.Method, seed.Path)
		parent := accessNodeByPath(t, db, seed.ParentPath, pkg.MenuAccessType, pkg.ComponentAccessType)
		if row.ParentID != parent.ID || row.Name != seed.Name || row.Permission != seed.Permission ||
			row.Status != enum.Enabled || !row.HideInMenu || row.DeletedAt.Valid {
			t.Fatalf("API permission %s %s metadata or hierarchy is wrong: %+v", seed.Method, seed.Path, row)
		}
		if strings.Contains(row.Path, "*") {
			t.Fatalf("API permission path was broadened to wildcard: %q", row.Path)
		}
	}
}

func assertNoAdminRoutePolicies(t *testing.T, db *gorm.DB) {
	t.Helper()
	var apiCount int64
	if err := db.Model(&models.CasbinRule{}).
		Where("v1 = ?", pkg.APIAccessType.String()).
		Count(&apiCount).Error; err != nil {
		t.Fatalf("count Casbin API policies: %v", err)
	}
	if apiCount != 0 {
		t.Fatalf("migration created %d Casbin API policies; want none", apiCount)
	}

	var defaultGrantCount int64
	componentPaths := make([]string, 0, len(adminRouteComponentSeeds))
	for _, seed := range adminRouteComponentSeeds {
		componentPaths = append(componentPaths, seed.Path)
	}
	if err := db.Model(&models.CasbinRule{}).
		Where("v0 = ?", "default-role").
		Where("(v1 = ? AND v2 = ?) OR (v1 = ? AND v2 IN ?)",
			pkg.MenuAccessType.String(), "/log", pkg.ComponentAccessType.String(), componentPaths).
		Count(&defaultGrantCount).Error; err != nil {
		t.Fatalf("count implicit default-role grants: %v", err)
	}
	if defaultGrantCount != 0 {
		t.Fatalf("migration created %d implicit default-role menu/component grants; want none", defaultGrantCount)
	}
}

// SetAuthorize expands only direct API children of a selected MENU or
// COMPONENT. Therefore a page MENU must contain read children only; mutation
// APIs belong beneath separately selectable COMPONENT nodes.
func assertPageSelectionsHaveNoMutationAPIChildren(t *testing.T, db *gorm.DB) {
	t.Helper()
	for _, path := range []string{"/users", "/task", "/system-config", "/departments", "/posts", "/log"} {
		page := accessNodeByPath(t, db, path, pkg.MenuAccessType)
		var children []models.Menu
		if err := db.Where("parent_id = ? AND type = ?", page.ID, pkg.APIAccessType).Find(&children).Error; err != nil {
			t.Fatalf("load direct API children for page %q: %v", path, err)
		}
		for _, child := range children {
			if child.Method != "GET" && child.Method != "HEAD" && child.Method != "OPTIONS" {
				t.Fatalf("selecting page %q would implicitly grant mutation %s %s", path, child.Method, child.Path)
			}
		}
	}
}

func assertNoAdminAPIPermissionHasDirectoryParent(t *testing.T, db *gorm.DB) {
	t.Helper()
	var apiMenus []models.Menu
	if err := db.Where("type = ?", pkg.APIAccessType).Find(&apiMenus).Error; err != nil {
		t.Fatalf("load API permissions: %v", err)
	}
	for _, apiMenu := range apiMenus {
		if apiMenu.ParentID == "" {
			continue
		}
		var parent models.Menu
		if err := db.First(&parent, "id = ?", apiMenu.ParentID).Error; err != nil {
			t.Fatalf("load parent for API %s %s: %v", apiMenu.Method, apiMenu.Path, err)
		}
		if parent.Type == pkg.DirectoryAccessType {
			t.Fatalf("API %s %s is attached to non-assignable directory %q", apiMenu.Method, apiMenu.Path, parent.Path)
		}
	}
}

func accessNodeByPath(t *testing.T, db *gorm.DB, path string, accessTypes ...pkg.AccessType) models.Menu {
	t.Helper()
	var rows []models.Menu
	if err := db.Where("path = ? AND type IN ?", path, accessTypes).Find(&rows).Error; err != nil {
		t.Fatalf("load access node %q: %v", path, err)
	}
	if len(rows) != 1 {
		t.Fatalf("access node %q count = %d, want 1", path, len(rows))
	}
	return rows[0]
}

func accessAPIByRoute(t *testing.T, db *gorm.DB, method, path string) models.Menu {
	t.Helper()
	var rows []models.Menu
	if err := db.Where("type = ? AND path = ? AND method = ?", pkg.APIAccessType, path, method).Find(&rows).Error; err != nil {
		t.Fatalf("load API permission %s %s: %v", method, path, err)
	}
	if len(rows) != 1 {
		t.Fatalf("API permission %s %s count = %d, want 1", method, path, len(rows))
	}
	return rows[0]
}

type visibleMenuSnapshot struct {
	ID         string
	ParentID   string
	Name       string
	Path       string
	Method     string
	Component  string
	Type       pkg.AccessType
	Permission string
	Status     enum.Status
	HideInMenu bool
	Deleted    bool
}

func visibleMenuState(t *testing.T, db *gorm.DB) []visibleMenuSnapshot {
	t.Helper()
	var menus []models.Menu
	if err := db.Where("type IN ?", []pkg.AccessType{pkg.MenuAccessType, pkg.DirectoryAccessType}).Find(&menus).Error; err != nil {
		t.Fatalf("load visible menus: %v", err)
	}
	state := make([]visibleMenuSnapshot, 0, len(menus))
	for _, menu := range menus {
		state = append(state, visibleMenuSnapshot{
			ID:         menu.ID,
			ParentID:   menu.ParentID,
			Name:       menu.Name,
			Path:       menu.Path,
			Method:     menu.Method,
			Component:  menu.Component,
			Type:       menu.Type,
			Permission: menu.Permission,
			Status:     menu.Status,
			HideInMenu: menu.HideInMenu,
			Deleted:    menu.DeletedAt.Valid,
		})
	}
	sort.Slice(state, func(i, j int) bool { return state[i].ID < state[j].ID })
	return state
}

func assertExistingVisibleMenusUnchanged(t *testing.T, db *gorm.DB, want []visibleMenuSnapshot) {
	t.Helper()
	got := visibleMenuState(t, db)
	gotByID := make(map[string]visibleMenuSnapshot, len(got))
	for _, menu := range got {
		gotByID[menu.ID] = menu
	}
	for _, expected := range want {
		actual, ok := gotByID[expected.ID]
		if !ok {
			t.Fatalf("existing dynamic menu %q (%s) disappeared", expected.Path, expected.ID)
		}
		if !reflect.DeepEqual(actual, expected) {
			t.Fatalf("existing dynamic menu %q changed:\n got: %#v\nwant: %#v", expected.Path, actual, expected)
		}
	}
}

func assertVisibleMenuCount(t *testing.T, db *gorm.DB, want int) {
	t.Helper()
	if got := len(visibleMenuState(t, db)); got != want {
		t.Fatalf("visible menu count = %d, want %d", got, want)
	}
}

func adminAccessNodeIDs(t *testing.T, db *gorm.DB, accessType pkg.AccessType) map[string]string {
	t.Helper()
	var menus []models.Menu
	if err := db.Where("type = ?", accessType).Find(&menus).Error; err != nil {
		t.Fatalf("load %s access node IDs: %v", accessType, err)
	}
	ids := make(map[string]string, len(menus))
	for _, menu := range menus {
		ids[menu.Method+" "+menu.Path] = menu.ID
	}
	return ids
}
