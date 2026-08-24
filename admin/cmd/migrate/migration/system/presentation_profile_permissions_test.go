package system

import (
	"fmt"
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
	if err := migratePresentationProfilePermissions(db, version); err != nil {
		t.Fatalf("repeat migration: %v", err)
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
	assertPresentationPermissionVersion(t, db, version, 1)
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
