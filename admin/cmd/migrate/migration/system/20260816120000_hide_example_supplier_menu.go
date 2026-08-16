package system

import (
	"errors"
	"fmt"
	"sort"

	"github.com/mss-boot-io/mss-boot-admin/admin/models"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/config/gormdb"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/migration"
	migrationmodels "github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/migration/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const hideExampleSupplierMenuMigrationID migration.MigrationID = "20260816120000"

func init() {
	_ = migration.Migrate.Register(
		hideExampleSupplierMenuMigrationID,
		_20260816120000HideExampleSupplierMenu,
	)
}

func _20260816120000HideExampleSupplierMenu(db *gorm.DB, version string) error {
	if db == nil {
		return fmt.Errorf("hide example supplier menu: database is nil")
	}

	err := db.Transaction(func(tx *gorm.DB) error {
		var menus []models.Menu
		if tx.Migrator().HasTable(&models.Menu{}) {
			if err := tx.Unscoped().Find(&menus).Error; err != nil {
				return fmt.Errorf("hide example supplier menu: load menu metadata: %w", err)
			}
		}

		retiredIDs := make(map[string]bool)
		retiredParentIDs := make(map[string]string)
		for i := range menus {
			if !isExampleSupplierMenuPath(menus[i].Path) {
				continue
			}
			retiredIDs[menus[i].ID] = true
			retiredParentIDs[menus[i].ID] = menus[i].ParentID
		}

		// A downstream application may have attached an unrelated menu below the
		// demonstration /procurement directory. Preserve that customization while
		// removing only the checked-in Supplier example from the initialized app.
		for i := range menus {
			if retiredIDs[menus[i].ID] || !retiredIDs[menus[i].ParentID] {
				continue
			}
			parentID := retiredParentIDs[menus[i].ParentID]
			for retiredIDs[parentID] {
				parentID = retiredParentIDs[parentID]
			}
			if err := tx.Unscoped().Model(&models.Menu{}).
				Where("id = ?", menus[i].ID).
				Update("parent_id", parentID).Error; err != nil {
				return fmt.Errorf("hide example supplier menu: preserve child %q: %w", menus[i].Path, err)
			}
		}

		if tx.Migrator().HasTable(&models.CasbinRule{}) {
			if err := tx.Where(
				"ptype = ? AND (v2 = ? OR v2 = ? OR v2 LIKE ?)",
				"p", "/procurement", "/suppliers", "/suppliers/%",
			).Delete(&models.CasbinRule{}).Error; err != nil {
				return fmt.Errorf("hide example supplier menu: delete page policies: %w", err)
			}
			if err := tx.Where(
				"ptype = ? AND (v2 = ? OR v2 LIKE ?)",
				"p", "/admin/api/suppliers", "/admin/api/suppliers/%",
			).Delete(&models.CasbinRule{}).Error; err != nil {
				return fmt.Errorf("hide example supplier menu: delete API policies: %w", err)
			}
		}
		if tx.Migrator().HasTable(&models.API{}) {
			if err := tx.Unscoped().Where(
				"path = ? OR path LIKE ?",
				"/admin/api/suppliers", "/admin/api/suppliers/%",
			).Delete(&models.API{}).Error; err != nil {
				return fmt.Errorf("hide example supplier menu: delete API inventory: %w", err)
			}
		}

		ids := make([]string, 0, len(retiredIDs))
		for id := range retiredIDs {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		if len(ids) > 0 {
			if err := tx.Session(&gorm.Session{SkipHooks: true}).Unscoped().
				Where("id IN ?", ids).
				Delete(&models.Menu{}).Error; err != nil {
				return fmt.Errorf("hide example supplier menu: delete menu metadata: %w", err)
			}
		}

		versionRow := &migrationmodels.Migration{}
		versionRow.SetVersion(version)
		return tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "version"}},
			DoNothing: true,
		}).Create(versionRow).Error
	})
	if err != nil {
		return err
	}
	if gormdb.Enforcer != nil {
		if err := gormdb.Enforcer.LoadPolicy(); err != nil {
			cause := fmt.Errorf("hide example supplier menu: reload Casbin policy: %w", err)
			cleanupErr := db.Where("version = ?", version).Delete(&migrationmodels.Migration{}).Error
			if cleanupErr != nil {
				return errors.Join(
					cause,
					fmt.Errorf("hide example supplier menu: clear migration version for retry: %w", cleanupErr),
				)
			}
			return cause
		}
	}
	return nil
}

func isExampleSupplierMenuPath(path string) bool {
	return path == "/procurement" || path == "/suppliers" ||
		len(path) > len("/suppliers/") && path[:len("/suppliers/")] == "/suppliers/" ||
		path == "/admin/api/suppliers" ||
		len(path) > len("/admin/api/suppliers/") && path[:len("/admin/api/suppliers/")] == "/admin/api/suppliers/"
}
