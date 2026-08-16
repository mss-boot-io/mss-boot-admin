package system

import (
	"fmt"
	"sort"

	"github.com/mss-boot-io/mss-boot-admin/admin/models"
	"github.com/mss-boot-io/mss-boot-admin/admin/pkg"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/config/gormdb"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/enum"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/migration"
	migrationmodels "github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/migration/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const optionManagementPermissionsMigrationID migration.MigrationID = "20260815131000"

var optionManagementComponentSeeds = []adminRouteComponentSeed{
	{
		Name:       "menu.system.option.create",
		Path:       "/option/create",
		Permission: "option:create",
		ParentPath: "/option",
	},
	{
		Name:       "menu.system.option.edit",
		Path:       "/option/edit",
		Permission: "option:update",
		ParentPath: "/option",
	},
	{
		Name:       "menu.system.option.delete",
		Path:       "/option/delete",
		Permission: "option:delete",
		ParentPath: "/option",
	},
}

var optionManagementPermissionSeeds = []adminRoutePermissionSeed{
	{
		Name:       "api.option.list",
		Path:       "/admin/api/options",
		Method:     "GET",
		Permission: "option:read",
		ParentPath: "/option",
		ParentType: pkg.MenuAccessType,
	},
	{
		Name:       "api.option.read",
		Path:       "/admin/api/options/:id",
		Method:     "GET",
		Permission: "option:read",
		ParentPath: "/option",
		ParentType: pkg.MenuAccessType,
	},
	{
		Name:       "api.option.create",
		Path:       "/admin/api/options",
		Method:     "POST",
		Permission: "option:create",
		ParentPath: "/option/create",
		ParentType: pkg.ComponentAccessType,
	},
	{
		Name:       "api.option.update",
		Path:       "/admin/api/options/:id",
		Method:     "PUT",
		Permission: "option:update",
		ParentPath: "/option/edit",
		ParentType: pkg.ComponentAccessType,
	},
	{
		Name:       "api.option.delete",
		Path:       "/admin/api/options/:id",
		Method:     "DELETE",
		Permission: "option:delete",
		ParentPath: "/option/delete",
		ParentType: pkg.ComponentAccessType,
	},
}

func init() {
	_ = migration.Migrate.Register(
		optionManagementPermissionsMigrationID,
		_20260815131000OptionManagementPermissions,
	)
}

func _20260815131000OptionManagementPermissions(db *gorm.DB, version string) error {
	_, err := migrateOptionManagementPermissions(db, version)
	return err
}

// migrateOptionManagementPermissions makes read, create, update and delete
// independently assignable. Existing exact page/action owners inherit only
// their corresponding API grants on the first run.
func migrateOptionManagementPermissions(
	db *gorm.DB,
	version string,
) ([]adminRoutePermissionAffectedRole, error) {
	if db == nil {
		return nil, fmt.Errorf("option permission migration: database is nil")
	}
	var appliedCount int64
	if err := db.Model(&migrationmodels.Migration{}).
		Where("version = ?", version).
		Count(&appliedCount).Error; err != nil {
		return nil, fmt.Errorf("option permission migration: check version: %w", err)
	}
	alreadyApplied := appliedCount > 0
	affected := make([]adminRoutePermissionAffectedRole, 0)

	err := db.Transaction(func(tx *gorm.DB) error {
		activeSources := make(map[string]bool, len(optionManagementPermissionSeeds))
		for i := range optionManagementPermissionSeeds {
			seed := optionManagementPermissionSeeds[i]
			key := adminRoutePermissionSourceKey(seed.ParentType, seed.ParentPath)
			if _, checked := activeSources[key]; checked {
				continue
			}
			var count int64
			if err := tx.Model(&models.Menu{}).
				Where("type = ? AND path = ? AND status = ?", seed.ParentType, seed.ParentPath, enum.Enabled).
				Count(&count).Error; err != nil {
				return fmt.Errorf("option permission migration: inspect source %q: %w", seed.ParentPath, err)
			}
			activeSources[key] = count > 0
		}

		for i := range optionManagementComponentSeeds {
			if err := upsertAdminRouteComponent(tx, optionManagementComponentSeeds[i]); err != nil {
				return fmt.Errorf("option permission migration: upsert component: %w", err)
			}
		}
		for i := range optionManagementPermissionSeeds {
			seed := optionManagementPermissionSeeds[i]
			if err := upsertAdminRoutePermission(tx, seed); err != nil {
				return fmt.Errorf("option permission migration: upsert API: %w", err)
			}
			if alreadyApplied || !activeSources[adminRoutePermissionSourceKey(seed.ParentType, seed.ParentPath)] {
				continue
			}
			inherited, err := inheritExistingAdminRoutePermission(tx, seed)
			if err != nil {
				return err
			}
			affected = append(affected, inherited...)
		}

		versionRow := &migrationmodels.Migration{}
		versionRow.SetVersion(version)
		return tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "version"}},
			DoNothing: true,
		}).Create(versionRow).Error
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(affected, func(i, j int) bool {
		if affected[i].RoleID != affected[j].RoleID {
			return affected[i].RoleID < affected[j].RoleID
		}
		if affected[i].APIPath != affected[j].APIPath {
			return affected[i].APIPath < affected[j].APIPath
		}
		return affected[i].Method < affected[j].Method
	})
	if gormdb.Enforcer != nil {
		if err := gormdb.Enforcer.LoadPolicy(); err != nil {
			return affected, fmt.Errorf("option permission migration: reload Casbin policy: %w", err)
		}
	}
	return affected, nil
}
