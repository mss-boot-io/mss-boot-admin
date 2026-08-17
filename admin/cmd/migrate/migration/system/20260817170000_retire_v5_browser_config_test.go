package system

import (
	"fmt"
	"testing"

	"github.com/mss-boot-io/mss-boot-admin/admin/models"
	migrationmodels "github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/migration/models"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestRetireV5BrowserConfigRemovesOnlyExactRetiredKeys(t *testing.T) {
	db, err := gorm.Open(
		sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())),
		&gorm.Config{},
	)
	if err != nil {
		t.Fatal(err)
	}
	if err = db.AutoMigrate(&models.AppConfig{}, &models.UserConfig{}, &migrationmodels.Migration{}); err != nil {
		t.Fatal(err)
	}

	appRows := []models.AppConfig{
		{Group: "security", Name: "githubClientId", Value: "retired"},
		{Group: "security", Name: "githubClientSecret", Value: "retired"},
		{Group: "security", Name: "githubRedirectUrl", Value: "retired"},
		{Group: "security", Name: "larkAppSecret", Value: "retired"},
		{Group: "security", Name: "githubBrowserSessionClientId", Value: "kept"},
		{Group: "security", Name: "githubBrowserSessionClientSecret", Value: "kept"},
		{Group: "security", Name: "customProviderSecret", Value: "kept"},
		{Group: "Security", Name: "githubClientSecret", Value: "kept-ambiguous"},
		{Group: "theme", Name: "pwa", Value: "true"},
		{Group: "theme", Name: "layout", Value: "mix"},
	}
	if err = db.Create(&appRows).Error; err != nil {
		t.Fatal(err)
	}
	userRows := []models.UserConfig{
		{UserID: "user-a", Group: "theme", Name: "pwa", Value: "true"},
		{UserID: "user-b", Group: "theme", Name: "pwa", Value: "false"},
		{UserID: "user-a", Group: "theme", Name: "layout", Value: "side"},
		{UserID: "user-a", Group: "Theme", Name: "pwa", Value: "kept-ambiguous"},
	}
	if err = db.Create(&userRows).Error; err != nil {
		t.Fatal(err)
	}

	const version = "20260817170000-test"
	if err = _20260817170000RetireV5BrowserConfig(db, version); err != nil {
		t.Fatal(err)
	}
	if err = _20260817170000RetireV5BrowserConfig(db, version); err != nil {
		t.Fatalf("repeat migration: %v", err)
	}

	for _, name := range retiredV5OAuthConfigNames {
		var count int64
		if err = db.Unscoped().Model(&models.AppConfig{}).
			Where(&models.AppConfig{Group: "security", Name: name}).
			Count(&count).Error; err != nil || count != 0 {
			t.Fatalf("retired security/%s count = %d, err = %v", name, count, err)
		}
	}
	assertV6ConfigRowCount(t, db, &models.AppConfig{}, "security", "githubBrowserSessionClientId", 1)
	assertV6ConfigRowCount(t, db, &models.AppConfig{}, "security", "githubBrowserSessionClientSecret", 1)
	assertV6ConfigRowCount(t, db, &models.AppConfig{}, "security", "customProviderSecret", 1)
	assertV6ConfigRowCount(t, db, &models.AppConfig{}, "Security", "githubClientSecret", 1)
	assertV6ConfigRowCount(t, db, &models.AppConfig{}, "theme", "pwa", 0)
	assertV6ConfigRowCount(t, db, &models.AppConfig{}, "theme", "layout", 1)
	assertV6ConfigRowCount(t, db, &models.UserConfig{}, "theme", "pwa", 0)
	assertV6ConfigRowCount(t, db, &models.UserConfig{}, "theme", "layout", 1)
	assertV6ConfigRowCount(t, db, &models.UserConfig{}, "Theme", "pwa", 1)

	var versionCount int64
	if err = db.Model(&migrationmodels.Migration{}).Where("version = ?", version).Count(&versionCount).Error; err != nil || versionCount != 1 {
		t.Fatalf("migration version count = %d, err = %v", versionCount, err)
	}
}

func assertV6ConfigRowCount(t *testing.T, db *gorm.DB, model any, group, name string, want int64) {
	t.Helper()
	var count int64
	if err := db.Unscoped().Model(model).
		Where("\"group\" = ? AND name = ?", group, name).
		Count(&count).Error; err != nil || count != want {
		t.Fatalf("%T %s/%s count = %d, want %d, err = %v", model, group, name, count, want, err)
	}
}
