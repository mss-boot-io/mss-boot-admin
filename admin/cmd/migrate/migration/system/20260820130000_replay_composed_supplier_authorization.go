package system

import (
	"fmt"

	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/migration"
	migrationmodels "github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/migration/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	replayComposedSupplierAuthorizationMigrationID migration.MigrationID = "20260820130000"
	exampleSupplierSchemaMigrationID               migration.MigrationID = "20260810160000" // mss:migration-reference supplier
	exampleSupplierAuthorizationMigrationID        migration.MigrationID = "20260811120000" // mss:migration-reference supplier
)

func init() {
	_ = migration.Migrate.Register(
		replayComposedSupplierAuthorizationMigrationID,
		_20260820130000ReplayComposedSupplierAuthorization,
	)
}

// _20260820130000ReplayComposedSupplierAuthorization repairs the one release
// window where business migrations were globally sorted ahead of the Admin
// migration that retired the checked-in Supplier example. When the Supplier
// schema and both historical migration records are present, clearing only the
// authorization record lets the following Business migration phase replay its
// own idempotent projection after every Core migration has completed.
//
// A host that does not compose Supplier has no matching Business registration,
// so this compatibility marker never recreates the retired example by itself.
func _20260820130000ReplayComposedSupplierAuthorization(db *gorm.DB, version string) error {
	if db == nil {
		return fmt.Errorf("replay composed supplier authorization: database is nil")
	}
	return db.Transaction(func(tx *gorm.DB) error {
		if tx.Migrator().HasTable("biz_suppliers") {
			complete, err := supplierMigrationVersionsApplied(
				tx,
				exampleSupplierSchemaMigrationID,
				exampleSupplierAuthorizationMigrationID,
				hideExampleSupplierMenuMigrationID,
			)
			if err != nil {
				return err
			}
			if complete {
				if err := tx.Where("version = ?", exampleSupplierAuthorizationMigrationID.String()).
					Delete(&migrationmodels.Migration{}).Error; err != nil {
					return fmt.Errorf("replay composed supplier authorization: clear stale authorization version: %w", err)
				}
			}
		}

		versionRow := &migrationmodels.Migration{}
		versionRow.SetVersion(version)
		if err := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "version"}},
			DoNothing: true,
		}).Create(versionRow).Error; err != nil {
			return fmt.Errorf("replay composed supplier authorization: record version: %w", err)
		}
		return nil
	})
}

func supplierMigrationVersionsApplied(db *gorm.DB, ids ...migration.MigrationID) (bool, error) {
	versions := make([]string, 0, len(ids))
	for _, id := range ids {
		versions = append(versions, id.String())
	}
	var count int64
	if err := db.Model(&migrationmodels.Migration{}).
		Where("version IN ?", versions).
		Count(&count).Error; err != nil {
		return false, fmt.Errorf("replay composed supplier authorization: inspect migration history: %w", err)
	}
	return count == int64(len(versions)), nil
}
