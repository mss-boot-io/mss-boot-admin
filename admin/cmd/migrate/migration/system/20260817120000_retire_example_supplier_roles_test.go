package system

import (
	"fmt"
	"testing"

	"github.com/mss-boot-io/mss-boot-admin/admin/models"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/enum"
	migrationmodels "github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/migration/models"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestRetireExampleSupplierRolesRemovesOnlyUnusedGeneratedRoles(t *testing.T) {
	db, err := gorm.Open(
		sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())),
		&gorm.Config{},
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(
		&models.Role{},
		&models.CasbinRule{},
		&models.ConfigRevision{},
		&migrationmodels.Migration{},
	); err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("CREATE TABLE mss_boot_users (id TEXT PRIMARY KEY, role_id TEXT)").Error; err != nil {
		t.Fatal(err)
	}

	unusedFinance := createExampleSupplierRoleFixture(t, db, "finance", exampleSupplierGeneratedRoleRemark)
	assignedProcurement := createExampleSupplierRoleFixture(t, db, "procurement", exampleSupplierGeneratedRoleRemark)
	customFinance := createExampleSupplierRoleFixture(t, db, "finance", "kept by downstream application")
	policyProcurement := createExampleSupplierRoleFixture(t, db, "procurement", exampleSupplierGeneratedRoleRemark)

	if err := db.Exec(
		"INSERT INTO mss_boot_users (id, role_id) VALUES (?, ?)",
		"user-procurement",
		assignedProcurement.ID,
	).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.CasbinRule{
		PType: "p",
		V0:    policyProcurement.ID,
		V1:    "MENU",
		V2:    "/contracts",
		V3:    "GET",
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.ConfigRevision{
		Scope: "role", OwnerID: unusedFinance.ID, Resource: "authorization", Revision: 1,
	}).Error; err != nil {
		t.Fatal(err)
	}

	const version = "20260817120000-test"
	if err := _20260817120000RetireExampleSupplierRoles(db, version); err != nil {
		t.Fatal(err)
	}
	if err := _20260817120000RetireExampleSupplierRoles(db, version); err != nil {
		t.Fatalf("repeat migration: %v", err)
	}

	assertExampleSupplierRolePresence(t, db, unusedFinance.ID, false)
	assertExampleSupplierRolePresence(t, db, assignedProcurement.ID, true)
	assertExampleSupplierRolePresence(t, db, customFinance.ID, true)
	assertExampleSupplierRolePresence(t, db, policyProcurement.ID, true)

	var revisionCount int64
	if err := db.Model(&models.ConfigRevision{}).
		Where("scope = ? AND owner_id = ? AND resource = ?", "role", unusedFinance.ID, "authorization").
		Count(&revisionCount).Error; err != nil || revisionCount != 0 {
		t.Fatalf("retired role revision count = %d, err = %v", revisionCount, err)
	}
	var versionCount int64
	if err := db.Model(&migrationmodels.Migration{}).
		Where("version = ?", version).
		Count(&versionCount).Error; err != nil || versionCount != 1 {
		t.Fatalf("migration version count = %d, err = %v", versionCount, err)
	}
}

func createExampleSupplierRoleFixture(t *testing.T, db *gorm.DB, name, remark string) models.Role {
	t.Helper()
	role := models.Role{Name: name, Status: enum.Enabled, Remark: remark}
	if err := db.Create(&role).Error; err != nil {
		t.Fatal(err)
	}
	return role
}

func assertExampleSupplierRolePresence(t *testing.T, db *gorm.DB, roleID string, want bool) {
	t.Helper()
	var count int64
	if err := db.Unscoped().Model(&models.Role{}).Where("id = ?", roleID).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if got := count == 1; got != want {
		t.Fatalf("role %q presence = %t, want %t", roleID, got, want)
	}
}
