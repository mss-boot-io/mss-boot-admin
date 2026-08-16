package system

import (
	"fmt"
	"testing"

	"github.com/mss-boot-io/mss-boot-admin/admin/models"
	"github.com/mss-boot-io/mss-boot-admin/admin/pkg"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/enum"
	migrationmodels "github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/migration/models"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupLanguagePermissionMigrationTest(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(
		sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())),
		&gorm.Config{},
	)
	if err != nil {
		t.Fatalf("open SQLite database: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get SQLite handle: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := db.AutoMigrate(&models.Menu{}, &models.CasbinRule{}, &migrationmodels.Migration{}); err != nil {
		t.Fatalf("migrate language permission schema: %v", err)
	}
	return db
}

func createLanguageAccessNode(
	t *testing.T,
	db *gorm.DB,
	id string,
	path string,
	accessType pkg.AccessType,
	parentID string,
) {
	t.Helper()
	menu := &models.Menu{
		Name:       "fixture" + path,
		Path:       path,
		Method:     "GET",
		ParentID:   parentID,
		Type:       accessType,
		Permission: path,
		Status:     enum.Enabled,
	}
	menu.ID = id
	if err := db.Session(&gorm.Session{SkipHooks: true}).Create(menu).Error; err != nil {
		t.Fatalf("create access node %q: %v", path, err)
	}
}

func TestLanguagePermissionMigrationSeparatesReadAndWriteGrants(t *testing.T) {
	db := setupLanguagePermissionMigrationTest(t)
	createLanguageAccessNode(t, db, "language-menu", "/language", pkg.MenuAccessType, "")
	createLanguageAccessNode(t, db, "language-create", "/language/create", pkg.ComponentAccessType, "language-menu")
	for _, rule := range []*models.CasbinRule{
		{PType: "p", V0: "reader", V1: pkg.MenuAccessType.String(), V2: "/language", V3: "GET"},
		{PType: "p", V0: "creator", V1: pkg.ComponentAccessType.String(), V2: "/language/create", V3: "GET"},
	} {
		if err := db.Create(rule).Error; err != nil {
			t.Fatalf("create source policy: %v", err)
		}
	}

	const version = "20260815121000-test"
	affected, err := migrateLanguageManagementPermissions(db, version)
	if err != nil {
		t.Fatal(err)
	}
	if len(affected) != 2 {
		t.Fatalf("affected grants = %#v, want two compatibility grants", affected)
	}
	if _, err := migrateLanguageManagementPermissions(db, version); err != nil {
		t.Fatalf("repeat migration: %v", err)
	}

	for _, seed := range languageManagementComponentSeeds {
		var count int64
		if err := db.Model(&models.Menu{}).
			Where("type = ? AND path = ? AND status = ?", pkg.ComponentAccessType, seed.Path, enum.Enabled).
			Count(&count).Error; err != nil || count != 1 {
			t.Fatalf("component %q count = %d, err = %v", seed.Path, count, err)
		}
	}
	for _, seed := range languageManagementPermissionSeeds {
		var count int64
		if err := db.Model(&models.Menu{}).
			Where("type = ? AND path = ? AND method = ?", pkg.APIAccessType, seed.Path, seed.Method).
			Count(&count).Error; err != nil || count != 1 {
			t.Fatalf("API %s %q count = %d, err = %v", seed.Method, seed.Path, count, err)
		}
	}

	assertLanguagePolicyCount(t, db, "reader", "/admin/api/languages/:id", "GET", 1)
	assertLanguagePolicyCount(t, db, "reader", "/admin/api/languages", "POST", 0)
	assertLanguagePolicyCount(t, db, "creator", "/admin/api/languages", "POST", 1)
	assertLanguagePolicyCount(t, db, "creator", "/admin/api/languages/:id", "PUT", 0)
	for _, componentPath := range []string{"/language/edit", "/language/delete"} {
		assertLanguagePolicyCount(t, db, "reader", componentPath, "GET", 0)
	}

	var versionCount int64
	if err := db.Model(&migrationmodels.Migration{}).
		Where("version = ?", version).
		Count(&versionCount).Error; err != nil || versionCount != 1 {
		t.Fatalf("version count = %d, err = %v", versionCount, err)
	}
}

func TestLanguagePermissionMigrationFailsClosedWithoutPageMenu(t *testing.T) {
	db := setupLanguagePermissionMigrationTest(t)
	const version = "20260815121000-missing-parent"
	if _, err := migrateLanguageManagementPermissions(db, version); err == nil {
		t.Fatal("migration succeeded without /language menu")
	}
	var count int64
	if err := db.Model(&migrationmodels.Migration{}).
		Where("version = ?", version).
		Count(&count).Error; err != nil || count != 0 {
		t.Fatalf("version count = %d, err = %v", count, err)
	}
}

func assertLanguagePolicyCount(
	t *testing.T,
	db *gorm.DB,
	roleID string,
	path string,
	method string,
	want int64,
) {
	t.Helper()
	var count int64
	if err := db.Model(&models.CasbinRule{}).
		Where("ptype = ? AND v0 = ? AND v1 = ? AND v2 = ? AND v3 = ?", "p", roleID, pkg.APIAccessType.String(), path, method).
		Count(&count).Error; err != nil {
		t.Fatalf("count policy %s %s: %v", method, path, err)
	}
	if count != want {
		t.Fatalf("policy %s %s count = %d, want %d", method, path, count, want)
	}
}
