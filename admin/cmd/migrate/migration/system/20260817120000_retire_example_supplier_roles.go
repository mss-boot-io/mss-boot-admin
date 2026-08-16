package system

import (
	"fmt"
	"strings"

	"github.com/mss-boot-io/mss-boot-admin/admin/models"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/migration"
	migrationmodels "github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/migration/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	retireExampleSupplierRolesMigrationID migration.MigrationID = "20260817120000"
	exampleSupplierGeneratedRoleRemark                          = "generated supplier default role"
)

var exampleSupplierGeneratedRoleNames = []string{"finance", "procurement"}

func init() {
	_ = migration.Migrate.Register(
		retireExampleSupplierRolesMigrationID,
		_20260817120000RetireExampleSupplierRoles,
	)
}

// _20260817120000RetireExampleSupplierRoles removes only unused roles that the
// checked-in Supplier golden module created itself. A role is preserved when
// its provenance is ambiguous, its descriptive metadata changed, a user still
// owns it, or any retained authorization policy references it.
func _20260817120000RetireExampleSupplierRoles(db *gorm.DB, version string) error {
	if db == nil {
		return fmt.Errorf("retire example supplier roles: database is nil")
	}

	return db.Transaction(func(tx *gorm.DB) error {
		var applied int64
		if err := tx.Model(&migrationmodels.Migration{}).
			Where("version = ?", version).
			Count(&applied).Error; err != nil {
			return fmt.Errorf("retire example supplier roles: check version: %w", err)
		}
		if applied > 0 {
			return nil
		}

		if tx.Migrator().HasTable(&models.Role{}) {
			var candidates []models.Role
			if err := tx.Unscoped().
				Where("name IN ? AND remark = ?", exampleSupplierGeneratedRoleNames, exampleSupplierGeneratedRoleRemark).
				Order("name, id").
				Find(&candidates).Error; err != nil {
				return fmt.Errorf("retire example supplier roles: load candidates: %w", err)
			}
			for i := range candidates {
				if candidates[i].DeletedAt.Valid {
					continue
				}
				removable, err := exampleSupplierRoleIsUnused(tx, candidates[i].ID)
				if err != nil {
					return err
				}
				if !removable {
					continue
				}
				if tx.Migrator().HasTable(&models.ConfigRevision{}) {
					if err := tx.Where(
						"scope = ? AND owner_id = ? AND resource = ?",
						"role",
						candidates[i].ID,
						"authorization",
					).Delete(&models.ConfigRevision{}).Error; err != nil {
						return fmt.Errorf("retire example supplier roles: delete revision for %q: %w", candidates[i].Name, err)
					}
				}
				result := tx.Session(&gorm.Session{SkipHooks: true}).Unscoped().
					Where("id = ? AND name = ? AND remark = ?", candidates[i].ID, candidates[i].Name, exampleSupplierGeneratedRoleRemark).
					Delete(&models.Role{})
				if result.Error != nil {
					return fmt.Errorf("retire example supplier roles: delete %q: %w", candidates[i].Name, result.Error)
				}
				if result.RowsAffected != 1 {
					return fmt.Errorf("retire example supplier roles: candidate %q changed concurrently", candidates[i].Name)
				}
			}
		}

		versionRow := &migrationmodels.Migration{}
		versionRow.SetVersion(version)
		if err := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "version"}},
			DoNothing: true,
		}).Create(versionRow).Error; err != nil {
			return fmt.Errorf("retire example supplier roles: record version: %w", err)
		}
		return nil
	})
}

func exampleSupplierRoleIsUnused(tx *gorm.DB, roleID string) (bool, error) {
	roleID = strings.TrimSpace(roleID)
	if roleID == "" {
		return false, fmt.Errorf("retire example supplier roles: candidate role id is empty")
	}
	if tx.Migrator().HasTable(&models.CasbinRule{}) {
		var policies int64
		if err := tx.Model(&models.CasbinRule{}).
			Where("v0 = ? OR (ptype LIKE ? AND v1 = ?)", roleID, "g%", roleID).
			Count(&policies).Error; err != nil {
			return false, fmt.Errorf("retire example supplier roles: count policies for %q: %w", roleID, err)
		}
		if policies > 0 {
			return false, nil
		}
	}
	if tx.Migrator().HasTable("mss_boot_users") {
		var users int64
		if err := tx.Table("mss_boot_users").Where("role_id = ?", roleID).Count(&users).Error; err != nil {
			return false, fmt.Errorf("retire example supplier roles: count users for %q: %w", roleID, err)
		}
		if users > 0 {
			return false, nil
		}
	}
	return true, nil
}
