package system

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/mss-boot-io/mss-boot-admin/admin/models"
	"github.com/mss-boot-io/mss-boot-admin/admin/pkg"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/config/gormdb"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/enum"
	migrationmodels "github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/migration/models"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestPresentationPermissionMigrationCreatesIndependentDefaultDenyNodes(t *testing.T) {
	db := setupPresentationPermissionMigrationTest(t, true)
	if err := db.Create(&models.CasbinRule{
		PType: "p", V0: "system-reader", V1: pkg.DirectoryAccessType.String(), V2: "/system", V3: "GET",
	}).Error; err != nil {
		t.Fatalf("seed existing system grant: %v", err)
	}

	const version = "20260824121000-test"
	if err := migratePresentationProfilePermissions(db, version); err != nil {
		t.Fatal(err)
	}

	menu := presentationPermissionNode(t, db, pkg.MenuAccessType, presentationProfileMenuPath, "GET")
	if menu.Permission != "presentation:read" || menu.ParentID != "system-menu" || menu.Component != "./PresentationConfig" {
		t.Fatalf("presentation menu metadata = %+v", menu)
	}
	componentIDs := make(map[string]string, len(presentationProfileComponentSeeds))
	for _, seed := range presentationProfileComponentSeeds {
		node := presentationPermissionNode(t, db, pkg.ComponentAccessType, seed.Path, "GET")
		if node.Permission != seed.Permission || node.ParentID != menu.ID || !node.HideInMenu {
			t.Fatalf("component %q metadata = %+v", seed.Path, node)
		}
		componentIDs[seed.Path] = node.ID
	}
	for _, seed := range presentationProfilePermissionSeeds {
		node := presentationPermissionNode(t, db, pkg.APIAccessType, seed.Path, seed.Method)
		if node.Permission != seed.Permission || !node.HideInMenu {
			t.Fatalf("API %s %q metadata = %+v", seed.Method, seed.Path, node)
		}
		wantParent := menu.ID
		if seed.ParentType == pkg.ComponentAccessType {
			wantParent = componentIDs[seed.ParentPath]
		}
		if node.ParentID != wantParent {
			t.Fatalf("API %s %q parent = %q, want %q", seed.Method, seed.Path, node.ParentID, wantParent)
		}
	}

	var inherited int64
	if err := db.Model(&models.CasbinRule{}).
		Where("v0 = ? AND (v2 = ? OR v2 LIKE ?)", "system-reader", presentationProfileMenuPath, "/admin/api/presentation-%").
		Count(&inherited).Error; err != nil {
		t.Fatalf("count inherited presentation policies: %v", err)
	}
	if inherited != 0 {
		t.Fatalf("new presentation authority inherited %d policies from /system", inherited)
	}
	managedPolicies := []models.CasbinRule{
		{PType: "p", V0: "presentation-reader", V1: pkg.MenuAccessType.String(), V2: presentationProfileMenuPath, V3: "GET"},
		{PType: "p", V0: "presentation-publisher", V1: pkg.ComponentAccessType.String(), V2: "/presentation-config/publish", V3: "GET"},
		{PType: "p", V0: "presentation-publisher", V1: pkg.APIAccessType.String(), V2: "/admin/api/presentation-profiles/:id/publish", V3: "POST"},
	}
	if err := db.Create(&managedPolicies).Error; err != nil {
		t.Fatalf("seed presentation role policies: %v", err)
	}
	_, policiesBeforeRepeat := presentationPermissionState(t, db)
	if err := migratePresentationProfilePermissions(db, version); err != nil {
		t.Fatalf("repeat migration: %v", err)
	}
	_, policiesAfterRepeat := presentationPermissionState(t, db)
	if !reflect.DeepEqual(policiesBeforeRepeat, policiesAfterRepeat) {
		t.Fatalf("repeat migration changed role policies:\nbefore: %#v\nafter:  %#v", policiesBeforeRepeat, policiesAfterRepeat)
	}
	assertPresentationPermissionVersion(t, db, version, 1)
}

func TestPresentationPermissionMigrationFailsClosedOnOccupiedManagedPaths(t *testing.T) {
	targets := []struct {
		name   string
		kind   pkg.AccessType
		path   string
		method string
	}{
		{name: "menu", kind: pkg.MenuAccessType, path: presentationProfileMenuPath, method: "GET"},
		{name: "component", kind: pkg.ComponentAccessType, path: "/presentation-config/publish", method: "GET"},
		{name: "api", kind: pkg.APIAccessType, path: "/admin/api/presentation-profiles/:id/publish", method: "POST"},
	}

	for _, target := range targets {
		t.Run(target.name, func(t *testing.T) {
			db := setupPresentationPermissionMigrationTest(t, true)
			var node *models.Menu
			if target.kind == pkg.MenuAccessType {
				node = &models.Menu{
					ParentID:   "system-menu",
					Name:       "downstream.presentation",
					Path:       target.path,
					Method:     target.method,
					Component:  "./DownstreamPresentation",
					Icon:       "custom",
					Type:       target.kind,
					Permission: "downstream:presentation",
					Status:     enum.Enabled,
					Sort:       7,
				}
				node.ID = "downstream-presentation-menu"
				if err := db.Session(&gorm.Session{SkipHooks: true}).Create(node).Error; err != nil {
					t.Fatalf("seed downstream presentation menu: %v", err)
				}
			} else {
				if err := migratePresentationProfilePermissions(db, "presentation-collision-baseline-"+target.name); err != nil {
					t.Fatalf("seed managed presentation permissions: %v", err)
				}
				node = presentationPermissionNode(t, db, target.kind, target.path, target.method)
				if err := db.Unscoped().Model(&models.Menu{}).Where("id = ?", node.ID).Updates(map[string]any{
					"name":       "downstream." + target.name,
					"permission": "downstream:" + target.name,
				}).Error; err != nil {
					t.Fatalf("replace %s metadata with downstream owner: %v", target.name, err)
				}
				node = presentationPermissionNode(t, db, target.kind, target.path, target.method)
			}
			seedPresentationPermissionRolePolicy(t, db, "role-collision-"+target.name, node)
			nodesBefore, policiesBefore := presentationPermissionState(t, db)

			version := "presentation-collision-attempt-" + target.name
			err := migratePresentationProfilePermissions(db, version)
			if err == nil || !strings.Contains(err.Error(), "does not match managed metadata") {
				t.Fatalf("migration error = %v, want managed metadata collision", err)
			}
			nodesAfter, policiesAfter := presentationPermissionState(t, db)
			if !reflect.DeepEqual(nodesBefore, nodesAfter) {
				t.Fatalf("failed migration changed %s nodes:\nbefore: %#v\nafter:  %#v", target.name, nodesBefore, nodesAfter)
			}
			if !reflect.DeepEqual(policiesBefore, policiesAfter) {
				t.Fatalf("failed migration changed %s role policies:\nbefore: %#v\nafter:  %#v", target.name, policiesBefore, policiesAfter)
			}
			assertPresentationPermissionVersion(t, db, version, 0)
		})
	}
}

func TestPresentationPermissionMigrationFailsClosedOnInactiveManagedPaths(t *testing.T) {
	targets := []struct {
		name   string
		kind   pkg.AccessType
		path   string
		method string
	}{
		{name: "menu", kind: pkg.MenuAccessType, path: presentationProfileMenuPath, method: "GET"},
		{name: "component", kind: pkg.ComponentAccessType, path: "/presentation-config/publish", method: "GET"},
		{name: "api", kind: pkg.APIAccessType, path: "/admin/api/presentation-profiles/:id/publish", method: "POST"},
	}
	states := []struct {
		name      string
		wantError string
		mutate    func(t *testing.T, db *gorm.DB, node *models.Menu)
	}{
		{
			name:      "disabled",
			wantError: "is disabled",
			mutate: func(t *testing.T, db *gorm.DB, node *models.Menu) {
				t.Helper()
				if err := db.Unscoped().Model(&models.Menu{}).Where("id = ?", node.ID).Update("status", enum.Disabled).Error; err != nil {
					t.Fatalf("disable managed presentation node: %v", err)
				}
			},
		},
		{
			name:      "soft-deleted",
			wantError: "is soft-deleted",
			mutate: func(t *testing.T, db *gorm.DB, node *models.Menu) {
				t.Helper()
				if err := db.Session(&gorm.Session{SkipHooks: true}).Delete(node).Error; err != nil {
					t.Fatalf("soft-delete managed presentation node: %v", err)
				}
			},
		},
	}

	for _, target := range targets {
		for _, state := range states {
			t.Run(target.name+"/"+state.name, func(t *testing.T) {
				db := setupPresentationPermissionMigrationTest(t, true)
				if err := migratePresentationProfilePermissions(db, "presentation-inactive-baseline-"+target.name+"-"+state.name); err != nil {
					t.Fatalf("seed managed presentation permissions: %v", err)
				}
				node := presentationPermissionNode(t, db, target.kind, target.path, target.method)
				state.mutate(t, db, node)
				node = presentationPermissionNode(t, db, target.kind, target.path, target.method)
				seedPresentationPermissionRolePolicy(t, db, "role-inactive-"+target.name+"-"+state.name, node)
				nodesBefore, policiesBefore := presentationPermissionState(t, db)

				version := "presentation-inactive-attempt-" + target.name + "-" + state.name
				err := migratePresentationProfilePermissions(db, version)
				if err == nil || !strings.Contains(err.Error(), state.wantError) {
					t.Fatalf("migration error = %v, want diagnostic containing %q", err, state.wantError)
				}
				nodesAfter, policiesAfter := presentationPermissionState(t, db)
				if !reflect.DeepEqual(nodesBefore, nodesAfter) {
					t.Fatalf("failed migration changed %s %s nodes:\nbefore: %#v\nafter:  %#v", state.name, target.name, nodesBefore, nodesAfter)
				}
				if !reflect.DeepEqual(policiesBefore, policiesAfter) {
					t.Fatalf("failed migration changed %s %s policies:\nbefore: %#v\nafter:  %#v", state.name, target.name, policiesBefore, policiesAfter)
				}
				assertPresentationPermissionVersion(t, db, version, 0)
			})
		}
	}
}

func TestPresentationPermissionMigrationFailsClosedOnDuplicateAndCrossTypeManagedPaths(t *testing.T) {
	targets := []struct {
		name   string
		kind   pkg.AccessType
		path   string
		method string
	}{
		{name: "menu", kind: pkg.MenuAccessType, path: presentationProfileMenuPath, method: "GET"},
		{name: "component", kind: pkg.ComponentAccessType, path: "/presentation-config/publish", method: "GET"},
		{name: "api", kind: pkg.APIAccessType, path: "/admin/api/presentation-profiles/:id/publish", method: "POST"},
	}
	states := []struct {
		name      string
		wantError string
		mutate    func(t *testing.T, db *gorm.DB, node *models.Menu)
	}{
		{
			name:      "duplicate",
			wantError: "is occupied by 2 access nodes",
			mutate: func(t *testing.T, db *gorm.DB, node *models.Menu) {
				t.Helper()
				duplicate := *node
				duplicate.ID = "duplicate-" + node.ID
				if err := db.Session(&gorm.Session{SkipHooks: true}).Create(&duplicate).Error; err != nil {
					t.Fatalf("duplicate managed presentation node: %v", err)
				}
			},
		},
		{
			name:      "cross-type",
			wantError: "does not match managed metadata",
			mutate: func(t *testing.T, db *gorm.DB, node *models.Menu) {
				t.Helper()
				replacement := pkg.APIAccessType
				if node.Type == pkg.APIAccessType {
					replacement = pkg.ComponentAccessType
				}
				if err := db.Unscoped().Model(&models.Menu{}).Where("id = ?", node.ID).Update("type", replacement).Error; err != nil {
					t.Fatalf("replace managed presentation node type: %v", err)
				}
			},
		},
	}

	for _, target := range targets {
		for _, state := range states {
			t.Run(target.name+"/"+state.name, func(t *testing.T) {
				db := setupPresentationPermissionMigrationTest(t, true)
				baselineVersion := "presentation-ambiguous-baseline-" + target.name + "-" + state.name
				if err := migratePresentationProfilePermissions(db, baselineVersion); err != nil {
					t.Fatalf("seed managed presentation permissions: %v", err)
				}
				node := presentationPermissionNode(t, db, target.kind, target.path, target.method)
				seedPresentationPermissionRolePolicy(t, db, "role-ambiguous-"+target.name+"-"+state.name, node)
				state.mutate(t, db, node)
				nodesBefore, policiesBefore := presentationPermissionState(t, db)

				version := "presentation-ambiguous-attempt-" + target.name + "-" + state.name
				err := migratePresentationProfilePermissions(db, version)
				if err == nil || !strings.Contains(err.Error(), state.wantError) {
					t.Fatalf("migration error = %v, want diagnostic containing %q", err, state.wantError)
				}
				nodesAfter, policiesAfter := presentationPermissionState(t, db)
				if !reflect.DeepEqual(nodesBefore, nodesAfter) {
					t.Fatalf("failed migration changed %s %s nodes:\nbefore: %#v\nafter:  %#v", state.name, target.name, nodesBefore, nodesAfter)
				}
				if !reflect.DeepEqual(policiesBefore, policiesAfter) {
					t.Fatalf("failed migration changed %s %s policies:\nbefore: %#v\nafter:  %#v", state.name, target.name, policiesBefore, policiesAfter)
				}
				assertPresentationPermissionVersion(t, db, version, 0)
			})
		}
	}
}

func TestPresentationPermissionMigrationFailsClosedWithoutSystemParent(t *testing.T) {
	db := setupPresentationPermissionMigrationTest(t, false)
	const version = "20260824121000-missing-parent"
	if err := migratePresentationProfilePermissions(db, version); err == nil {
		t.Fatal("permission migration succeeded without /system parent")
	}
	assertPresentationPermissionVersion(t, db, version, 0)
	var nodes int64
	if err := db.Model(&models.Menu{}).Where("path = ?", presentationProfileMenuPath).Count(&nodes).Error; err != nil {
		t.Fatal(err)
	}
	if nodes != 0 {
		t.Fatalf("failed migration left %d presentation menu nodes", nodes)
	}
}

func setupPresentationPermissionMigrationTest(t *testing.T, withSystem bool) *gorm.DB {
	t.Helper()
	previousEnforcer := gormdb.Enforcer
	gormdb.Enforcer = nil
	t.Cleanup(func() { gormdb.Enforcer = previousEnforcer })
	db, err := gorm.Open(
		sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())),
		&gorm.Config{},
	)
	if err != nil {
		t.Fatalf("open permission database: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get permission database handle: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := db.AutoMigrate(&models.Menu{}, &models.CasbinRule{}, &migrationmodels.Migration{}); err != nil {
		t.Fatalf("migrate permission fixtures: %v", err)
	}
	if withSystem {
		menu := &models.Menu{
			Name: "menu.system", Path: "/system", Method: "GET", Type: pkg.DirectoryAccessType,
			Permission: "/system", Status: enum.Enabled,
		}
		menu.ID = "system-menu"
		if err := db.Session(&gorm.Session{SkipHooks: true}).Create(menu).Error; err != nil {
			t.Fatalf("create system menu: %v", err)
		}
	}
	return db
}

func presentationPermissionNode(t *testing.T, db *gorm.DB, kind pkg.AccessType, path, method string) *models.Menu {
	t.Helper()
	var nodes []models.Menu
	if err := db.Unscoped().Where("type = ? AND path = ? AND method = ?", kind, path, method).Find(&nodes).Error; err != nil {
		t.Fatalf("find %s %s %q: %v", kind, method, path, err)
	}
	if len(nodes) != 1 {
		t.Fatalf("%s %s %q count = %d, want 1", kind, method, path, len(nodes))
	}
	return &nodes[0]
}

func seedPresentationPermissionRolePolicy(t *testing.T, db *gorm.DB, roleID string, node *models.Menu) {
	t.Helper()
	if node == nil {
		t.Fatal("cannot seed presentation role policy for nil node")
	}
	if err := db.Create(&models.CasbinRule{
		PType: "p",
		V0:    roleID,
		V1:    node.Type.String(),
		V2:    node.Path,
		V3:    node.Method,
	}).Error; err != nil {
		t.Fatalf("seed role policy for %s %s %q: %v", node.Type, node.Method, node.Path, err)
	}
}

func presentationPermissionState(t *testing.T, db *gorm.DB) ([]models.Menu, []models.CasbinRule) {
	t.Helper()
	var nodes []models.Menu
	if err := db.Unscoped().Order("id").Find(&nodes).Error; err != nil {
		t.Fatalf("read presentation permission nodes: %v", err)
	}
	var policies []models.CasbinRule
	if err := db.Order("id").Find(&policies).Error; err != nil {
		t.Fatalf("read presentation role policies: %v", err)
	}
	return nodes, policies
}

func assertPresentationPermissionVersion(t *testing.T, db *gorm.DB, version string, want int64) {
	t.Helper()
	var count int64
	if err := db.Model(&migrationmodels.Migration{}).Where("version = ?", version).Count(&count).Error; err != nil {
		t.Fatalf("count permission migration version: %v", err)
	}
	if count != want {
		t.Fatalf("permission migration version count = %d, want %d", count, want)
	}
}
