package system

import (
	"reflect"
	"strings"
	"testing"

	"github.com/mss-boot-io/mss-boot-admin/admin/models"
	"github.com/mss-boot-io/mss-boot-admin/admin/pkg"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/config/gormdb"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/enum"
	migrationmodels "github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/migration/models"
	"gorm.io/gorm"
)

// migration.GetFilename retains the repository's historical 13-character
// version format from the timestamped filename.
const appConfigThemePermissionsTestVersion = "2026080713000"

func TestAppConfigThemePermissionMigrationCreatesAssignableMetadataWithoutImplicitGrant(t *testing.T) {
	db, parentIDs := setupAdminRoutePermissionMigrationTest(t)
	appConfig := createAppConfigPermissionMenu(t, db, parentIDs["/super-permission"], enum.Enabled)

	report, err := migrateAppConfigThemePermissions(db, appConfigThemePermissionsTestVersion)
	if err != nil {
		t.Fatalf("run fresh app-config theme permission migration: %v", err)
	}
	if len(report.CompatibilityInherited) != 0 {
		t.Fatalf("fresh migration compatibility inheritance = %#v, want none", report.CompatibilityInherited)
	}
	firstIDs := assertAppConfigThemePermissionMetadata(t, db, appConfig.ID)
	assertAppConfigThemeAPIPolicyCount(t, db, 0)
	assertAppConfigThemePermissionVersionCount(t, db, 1)

	secondReport, err := migrateAppConfigThemePermissions(db, appConfigThemePermissionsTestVersion)
	if err != nil {
		t.Fatalf("rerun app-config theme permission migration: %v", err)
	}
	if len(secondReport.CompatibilityInherited) != 0 {
		t.Fatalf("rerun compatibility inheritance = %#v, want none", secondReport.CompatibilityInherited)
	}
	secondIDs := assertAppConfigThemePermissionMetadata(t, db, appConfig.ID)
	if !reflect.DeepEqual(firstIDs, secondIDs) {
		t.Fatalf("API permission identities changed on rerun:\nfirst: %#v\nsecond: %#v", firstIDs, secondIDs)
	}
	assertAppConfigThemeAPIPolicyCount(t, db, 0)
	assertAppConfigThemePermissionVersionCount(t, db, 1)
}

func TestAppConfigThemePermissionMigrationUpgradesExactOwnersAndReloadsPolicy(t *testing.T) {
	db, parentIDs := setupAdminRoutePermissionMigrationTest(t)
	appConfig := createAppConfigPermissionMenu(t, db, parentIDs["/super-permission"], enum.Enabled)

	legacyReset := &models.Menu{
		ParentID:   parentIDs["/super-permission"],
		Name:       "legacy.app-config.reset",
		Path:       "/admin/api/app-configs/theme",
		Method:     "DELETE",
		Type:       pkg.APIAccessType,
		Permission: "legacy:write",
		Status:     enum.Disabled,
	}
	legacyReset.ID = "legacy-app-config-reset-api"
	if err := db.Session(&gorm.Session{SkipHooks: true}).Create(legacyReset).Error; err != nil {
		t.Fatalf("create legacy reset permission: %v", err)
	}
	if err := db.Session(&gorm.Session{SkipHooks: true}).Delete(legacyReset).Error; err != nil {
		t.Fatalf("soft-delete legacy reset permission: %v", err)
	}

	for _, roleID := range []string{
		"role-app-config",
		"role-app-config-controller",
		"role-unrelated",
		"role-wildcard",
		"role-wrong-method",
	} {
		role := &models.Role{Name: roleID, Status: enum.Enabled}
		role.ID = roleID
		if err := db.Session(&gorm.Session{SkipHooks: true}).Create(role).Error; err != nil {
			t.Fatalf("create role %q: %v", roleID, err)
		}
	}
	sourceRules := []models.CasbinRule{
		{PType: "p", V0: "role-app-config", V1: pkg.MenuAccessType.String(), V2: "/app-config", V3: "GET"},
		{PType: "p", V0: "role-unrelated", V1: pkg.MenuAccessType.String(), V2: "/system-config", V3: "GET"},
		{PType: "p", V0: "role-wildcard", V1: pkg.MenuAccessType.String(), V2: "/app-*", V3: "GET"},
		{PType: "p", V0: "role-wrong-method", V1: pkg.MenuAccessType.String(), V2: "/app-config", V3: "POST"},
	}
	if err := db.Create(&sourceRules).Error; err != nil {
		t.Fatalf("create pre-upgrade app-config policies: %v", err)
	}

	enforcer := newAdminRoutePermissionTestEnforcer(t, db)
	previousEnforcer := gormdb.Enforcer
	gormdb.Enforcer = enforcer
	t.Cleanup(func() { gormdb.Enforcer = previousEnforcer })
	assertEnforcerDecision(
		t,
		enforcer,
		false,
		"role-app-config",
		pkg.APIAccessType.String(),
		"/admin/api/app-configs/:group",
		"GET",
	)

	report, err := migrateAppConfigThemePermissions(db, appConfigThemePermissionsTestVersion)
	if err != nil {
		t.Fatalf("run upgrade app-config theme permission migration: %v", err)
	}
	wantInherited := []adminRoutePermissionAffectedRole{
		{
			RoleID:     "role-app-config",
			SourceType: pkg.MenuAccessType.String(),
			SourcePath: "/app-config",
			APIPath:    "/admin/api/app-configs/:group",
			Method:     "GET",
		},
	}
	if !reflect.DeepEqual(report.CompatibilityInherited, wantInherited) {
		t.Fatalf(
			"compatibility inheritance mismatch:\n got: %#v\nwant: %#v",
			report.CompatibilityInherited,
			wantInherited,
		)
	}
	permissionIDs := assertAppConfigThemePermissionMetadata(t, db, appConfig.ID)
	if permissionIDs["DELETE /admin/api/app-configs/theme"] != legacyReset.ID {
		t.Fatalf(
			"repaired reset API ID = %q, want existing %q",
			permissionIDs["DELETE /admin/api/app-configs/theme"],
			legacyReset.ID,
		)
	}
	assertAppConfigThemeAPIPolicyCount(t, db, 1)
	assertAppConfigThemePolicy(t, db, true, "role-app-config", appConfigThemePermissionSeeds[0].Path, "GET")
	for _, seed := range appConfigThemePermissionSeeds[1:] {
		assertAppConfigThemePolicy(t, db, false, "role-app-config", seed.Path, seed.Method)
	}
	for _, roleID := range []string{"role-app-config-controller", "role-unrelated", "role-wildcard", "role-wrong-method"} {
		for _, seed := range appConfigThemePermissionSeeds {
			assertAppConfigThemePolicy(t, db, false, roleID, seed.Path, seed.Method)
		}
	}

	grantAppConfigControlRole(t, db, "role-app-config-controller")
	if err := enforcer.LoadPolicy(); err != nil {
		t.Fatalf("reload explicit app-config controller policy: %v", err)
	}
	assertAppConfigThemeAPIPolicyCount(t, db, 3)

	// GET and PUT are authorized against Gin's registered FullPath template;
	// DELETE uses its dedicated static route.
	assertEnforcerDecision(t, enforcer, true, "role-app-config", "API", "/admin/api/app-configs/:group", "GET")
	assertEnforcerDecision(t, enforcer, false, "role-app-config", "API", "/admin/api/app-configs/:group", "PUT")
	assertEnforcerDecision(t, enforcer, false, "role-app-config", "API", "/admin/api/app-configs/theme", "DELETE")
	assertEnforcerDecision(t, enforcer, false, "role-app-config", "API", "/admin/api/app-configs/:group", "POST")
	assertEnforcerDecision(t, enforcer, false, "role-app-config-controller", "API", "/admin/api/app-configs/:group", "GET")
	assertEnforcerDecision(t, enforcer, true, "role-app-config-controller", "API", "/admin/api/app-configs/:group", "PUT")
	assertEnforcerDecision(t, enforcer, true, "role-app-config-controller", "API", "/admin/api/app-configs/theme", "DELETE")
	assertEnforcerDecision(t, enforcer, false, "", "API", "/admin/api/app-configs/:group", "GET")
	assertEnforcerDecision(t, enforcer, false, "role-unrelated", "API", "/admin/api/app-configs/:group", "GET")
	assertEnforcerDecision(t, enforcer, false, "role-wildcard", "API", "/admin/api/app-configs/theme", "DELETE")
	assertEnforcerDecision(t, enforcer, false, "role-wrong-method", "API", "/admin/api/app-configs/:group", "PUT")

	secondReport, err := migrateAppConfigThemePermissions(db, appConfigThemePermissionsTestVersion)
	if err != nil {
		t.Fatalf("rerun upgraded app-config theme permission migration: %v", err)
	}
	if len(secondReport.CompatibilityInherited) != 0 {
		t.Fatalf("rerun compatibility inheritance = %#v, want none", secondReport.CompatibilityInherited)
	}
	assertAppConfigThemeAPIPolicyCount(t, db, 3)
	assertAppConfigThemePermissionVersionCount(t, db, 1)
}

func TestAppConfigThemePermissionMigrationFailsClosedWithoutOneActiveParent(t *testing.T) {
	tests := []struct {
		name  string
		setup func(t *testing.T, db *gorm.DB, parentID string)
		want  string
	}{
		{
			name:  "missing",
			setup: func(*testing.T, *gorm.DB, string) {},
			want:  "found 0",
		},
		{
			name: "disabled",
			setup: func(t *testing.T, db *gorm.DB, parentID string) {
				createAppConfigPermissionMenu(t, db, parentID, enum.Disabled)
			},
			want: "disabled",
		},
		{
			name: "soft deleted",
			setup: func(t *testing.T, db *gorm.DB, parentID string) {
				menu := createAppConfigPermissionMenu(t, db, parentID, enum.Enabled)
				if err := db.Session(&gorm.Session{SkipHooks: true}).Delete(menu).Error; err != nil {
					t.Fatalf("soft-delete app-config parent: %v", err)
				}
			},
			want: "soft-deleted",
		},
		{
			name: "ambiguous duplicates",
			setup: func(t *testing.T, db *gorm.DB, parentID string) {
				createAppConfigPermissionMenu(t, db, parentID, enum.Enabled)
				createAppConfigPermissionMenu(t, db, parentID, enum.Enabled)
			},
			want: "found 2",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			db, parentIDs := setupAdminRoutePermissionMigrationTest(t)
			test.setup(t, db, parentIDs["/super-permission"])
			if err := db.Create(&models.CasbinRule{
				PType: "p",
				V0:    "role-stale-app-config",
				V1:    pkg.MenuAccessType.String(),
				V2:    "/app-config",
				V3:    "GET",
			}).Error; err != nil {
				t.Fatalf("create stale app-config grant: %v", err)
			}

			report, err := migrateAppConfigThemePermissions(db, appConfigThemePermissionsTestVersion)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("migration error = %v, want diagnostic containing %q", err, test.want)
			}
			if len(report.CompatibilityInherited) != 0 {
				t.Fatalf("failed migration compatibility inheritance = %#v, want none", report.CompatibilityInherited)
			}
			var apiCount int64
			if err := db.Unscoped().Model(&models.Menu{}).
				Where("type = ?", pkg.APIAccessType).
				Count(&apiCount).Error; err != nil {
				t.Fatalf("count rolled-back API metadata: %v", err)
			}
			if apiCount != 0 {
				t.Fatalf("failed migration left %d API nodes, want none", apiCount)
			}
			assertAppConfigThemeAPIPolicyCount(t, db, 0)
			assertAppConfigThemePermissionVersionCount(t, db, 0)
		})
	}
}

func createAppConfigPermissionMenu(
	t *testing.T,
	db *gorm.DB,
	parentID string,
	status enum.Status,
) *models.Menu {
	t.Helper()
	menu := &models.Menu{
		ParentID:   parentID,
		Name:       "menu.system.appConfig",
		Path:       "/app-config",
		Method:     "GET",
		Component:  "./AppConfig",
		Type:       pkg.MenuAccessType,
		Permission: "/app-config",
		Status:     status,
	}
	menu.ID = "app-config-menu-" + pkg.SimpleID()
	if err := db.Session(&gorm.Session{SkipHooks: true}).Create(menu).Error; err != nil {
		t.Fatalf("create app-config menu: %v", err)
	}
	return menu
}

func assertAppConfigThemePermissionMetadata(t *testing.T, db *gorm.DB, parentID string) map[string]string {
	t.Helper()
	component := accessNodeByPath(t, db, appConfigControlComponentSeed.Path, pkg.ComponentAccessType)
	if component.ParentID != parentID || component.Name != appConfigControlComponentSeed.Name ||
		component.Method != "GET" || component.Permission != appConfigControlComponentSeed.Permission ||
		component.Status != enum.Enabled || !component.HideInMenu || component.DeletedAt.Valid {
		t.Fatalf("app-config control component metadata is wrong: %+v", component)
	}

	ids := make(map[string]string, len(appConfigThemePermissionSeeds)+1)
	ids["COMPONENT "+component.Path] = component.ID
	for index, seed := range appConfigThemePermissionSeeds {
		permission := accessAPIByRoute(t, db, seed.Method, seed.Path)
		expectedParentID := component.ID
		if index == 0 {
			expectedParentID = parentID
		}
		if permission.ParentID != expectedParentID || permission.Name != seed.Name ||
			permission.Permission != seed.Permission || permission.Status != enum.Enabled ||
			!permission.HideInMenu || permission.DeletedAt.Valid {
			t.Fatalf("API permission %s %s metadata is wrong: %+v", seed.Method, seed.Path, permission)
		}
		ids[seed.Method+" "+seed.Path] = permission.ID
	}

	var directReadChildren int64
	if err := db.Model(&models.Menu{}).
		Where("parent_id = ? AND type = ?", parentID, pkg.APIAccessType).
		Count(&directReadChildren).Error; err != nil {
		t.Fatalf("count direct app-config API children: %v", err)
	}
	if directReadChildren != 1 {
		t.Fatalf("direct app-config API child count = %d, want read-only child", directReadChildren)
	}
	var componentChildren int64
	if err := db.Model(&models.Menu{}).
		Where("parent_id = ? AND type = ?", component.ID, pkg.APIAccessType).
		Count(&componentChildren).Error; err != nil {
		t.Fatalf("count app-config control API children: %v", err)
	}
	if componentChildren != 2 {
		t.Fatalf("app-config control API child count = %d, want update and reset", componentChildren)
	}
	return ids
}

func grantAppConfigControlRole(t *testing.T, db *gorm.DB, roleID string) {
	t.Helper()
	component := accessNodeByPath(t, db, appConfigControlComponentSeed.Path, pkg.ComponentAccessType)
	rules := []models.CasbinRule{
		{
			PType: "p",
			V0:    roleID,
			V1:    pkg.ComponentAccessType.String(),
			V2:    component.Path,
			V3:    "GET",
		},
	}
	for _, seed := range appConfigThemePermissionSeeds[1:] {
		rules = append(rules, models.CasbinRule{
			PType: "p",
			V0:    roleID,
			V1:    pkg.APIAccessType.String(),
			V2:    seed.Path,
			V3:    seed.Method,
		})
	}
	if err := db.Create(&rules).Error; err != nil {
		t.Fatalf("grant explicit app-config control component: %v", err)
	}
}

func assertAppConfigThemePolicy(
	t *testing.T,
	db *gorm.DB,
	want bool,
	roleID string,
	path string,
	method string,
) {
	t.Helper()
	var count int64
	if err := db.Model(&models.CasbinRule{}).
		Where(
			"ptype = ? AND v0 = ? AND v1 = ? AND v2 = ? AND v3 = ?",
			"p",
			roleID,
			pkg.APIAccessType.String(),
			path,
			method,
		).
		Count(&count).Error; err != nil {
		t.Fatalf("count policy for %s %s %s: %v", roleID, method, path, err)
	}
	if got := count > 0; got != want {
		t.Fatalf("policy for %s %s %s present = %t, want %t", roleID, method, path, got, want)
	}
}

func assertAppConfigThemeAPIPolicyCount(t *testing.T, db *gorm.DB, want int64) {
	t.Helper()
	paths := []string{
		"/admin/api/app-configs/:group",
		"/admin/api/app-configs/theme",
	}
	var count int64
	if err := db.Model(&models.CasbinRule{}).
		Where("ptype = ? AND v1 = ? AND v2 IN ?", "p", pkg.APIAccessType.String(), paths).
		Count(&count).Error; err != nil {
		t.Fatalf("count app-config theme API policies: %v", err)
	}
	if count != want {
		t.Fatalf("app-config theme API policy count = %d, want %d", count, want)
	}
}

func assertAppConfigThemePermissionVersionCount(t *testing.T, db *gorm.DB, want int64) {
	t.Helper()
	var count int64
	if err := db.Model(&migrationmodels.Migration{}).
		Where("version = ?", appConfigThemePermissionsTestVersion).
		Count(&count).Error; err != nil {
		t.Fatalf("count app-config theme permission migration version: %v", err)
	}
	if count != want {
		t.Fatalf("app-config theme permission migration version count = %d, want %d", count, want)
	}
}
