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

func TestHideExampleSupplierMenuKeepsGeneratorDataAndUnrelatedChildren(t *testing.T) {
	db, err := gorm.Open(
		sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())),
		&gorm.Config{},
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(
		&models.Menu{},
		&models.CasbinRule{},
		&models.API{},
		&migrationmodels.Migration{},
	); err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("CREATE TABLE biz_suppliers (id TEXT PRIMARY KEY, name TEXT)").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO biz_suppliers (id, name) VALUES (?, ?)", "supplier-1", "kept").Error; err != nil {
		t.Fatal(err)
	}

	procurement := createExampleSupplierMenuFixture(t, db, "procurement", "/procurement", "", pkg.DirectoryAccessType)
	suppliers := createExampleSupplierMenuFixture(t, db, "suppliers", "/suppliers", procurement.ID, pkg.MenuAccessType)
	createExampleSupplierMenuFixture(t, db, "supplier-read", "/suppliers/permissions/read", suppliers.ID, pkg.ComponentAccessType)
	createExampleSupplierMenuFixture(t, db, "supplier-api", "/admin/api/suppliers", suppliers.ID, pkg.APIAccessType)
	custom := createExampleSupplierMenuFixture(t, db, "custom", "/contracts", procurement.ID, pkg.MenuAccessType)

	for _, path := range []string{"/suppliers", "/suppliers/permissions/read", "/admin/api/suppliers"} {
		if err := db.Create(&models.CasbinRule{PType: "p", V0: "admin", V1: "MENU", V2: path, V3: "GET"}).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Create(&models.CasbinRule{PType: "p", V0: "custom-role", V1: "MENU", V2: "/contracts", V3: "GET"}).Error; err != nil {
		t.Fatal(err)
	}
	api := &models.API{Path: "/admin/api/suppliers", Method: "GET"}
	if err := db.Create(api).Error; err != nil {
		t.Fatal(err)
	}

	const version = "20260816120000-test"
	if err := _20260816120000HideExampleSupplierMenu(db, version); err != nil {
		t.Fatal(err)
	}
	if err := _20260816120000HideExampleSupplierMenu(db, version); err != nil {
		t.Fatalf("repeat migration: %v", err)
	}

	var retiredCount int64
	if err := db.Unscoped().Model(&models.Menu{}).
		Where("path = ? OR path LIKE ? OR path = ?", "/procurement", "/suppliers%", "/admin/api/suppliers").
		Count(&retiredCount).Error; err != nil || retiredCount != 0 {
		t.Fatalf("retired menu count = %d, err = %v", retiredCount, err)
	}
	var preserved models.Menu
	if err := db.First(&preserved, "id = ?", custom.ID).Error; err != nil {
		t.Fatal(err)
	}
	if preserved.ParentID != "" {
		t.Fatalf("preserved menu parent = %q, want root", preserved.ParentID)
	}
	var supplierCount int64
	if err := db.Table("biz_suppliers").Count(&supplierCount).Error; err != nil || supplierCount != 1 {
		t.Fatalf("supplier data count = %d, err = %v", supplierCount, err)
	}
	var policyCount int64
	if err := db.Model(&models.CasbinRule{}).Count(&policyCount).Error; err != nil || policyCount != 1 {
		t.Fatalf("preserved policy count = %d, err = %v", policyCount, err)
	}
	var versionCount int64
	if err := db.Model(&migrationmodels.Migration{}).Where("version = ?", version).Count(&versionCount).Error; err != nil || versionCount != 1 {
		t.Fatalf("migration version count = %d, err = %v", versionCount, err)
	}
}

func createExampleSupplierMenuFixture(
	t *testing.T,
	db *gorm.DB,
	id string,
	path string,
	parentID string,
	accessType pkg.AccessType,
) models.Menu {
	t.Helper()
	menu := models.Menu{
		ParentID:   parentID,
		Name:       id,
		Path:       path,
		Method:     "GET",
		Type:       accessType,
		Permission: path,
		Status:     enum.Enabled,
	}
	menu.ID = id
	if err := db.Session(&gorm.Session{SkipHooks: true}).Create(&menu).Error; err != nil {
		t.Fatal(err)
	}
	return menu
}
