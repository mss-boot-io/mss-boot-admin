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

const languageManagementPermissionsMigrationID migration.MigrationID = "20260815121000"

var languageManagementComponentSeeds = []adminRouteComponentSeed{
	{
		Name:       "menu.system.language.create",
		Path:       "/language/create",
		Permission: "language:create",
		ParentPath: "/language",
	},
	{
		Name:       "menu.system.language.edit",
		Path:       "/language/edit",
		Permission: "language:update",
		ParentPath: "/language",
	},
	{
		Name:       "menu.system.language.delete",
		Path:       "/language/delete",
		Permission: "language:delete",
		ParentPath: "/language",
	},
}

var languageManagementPermissionSeeds = []adminRoutePermissionSeed{
	{
		Name:       "api.language.read",
		Path:       "/admin/api/languages/:id",
		Method:     "GET",
		Permission: "language:read",
		ParentPath: "/language",
		ParentType: pkg.MenuAccessType,
	},
	{
		Name:       "api.language.create",
		Path:       "/admin/api/languages",
		Method:     "POST",
		Permission: "language:create",
		ParentPath: "/language/create",
		ParentType: pkg.ComponentAccessType,
	},
	{
		Name:       "api.language.update",
		Path:       "/admin/api/languages/:id",
		Method:     "PUT",
		Permission: "language:update",
		ParentPath: "/language/edit",
		ParentType: pkg.ComponentAccessType,
	},
	{
		Name:       "api.language.delete",
		Path:       "/admin/api/languages/:id",
		Method:     "DELETE",
		Permission: "language:delete",
		ParentPath: "/language/delete",
		ParentType: pkg.ComponentAccessType,
	},
}

func init() {
	_ = migration.Migrate.Register(
		languageManagementPermissionsMigrationID,
		_20260815121000LanguageManagementPermissions,
	)
}

func _20260815121000LanguageManagementPermissions(db *gorm.DB, version string) error {
	_, err := migrateLanguageManagementPermissions(db, version)
	return err
}

// migrateLanguageManagementPermissions keeps page-read and mutation grants
// independently assignable. Compatibility grants are inherited only from an
// already active exact MENU/COMPONENT source that existed before this run.
func migrateLanguageManagementPermissions(
	db *gorm.DB,
	version string,
) ([]adminRoutePermissionAffectedRole, error) {
	if db == nil {
		return nil, fmt.Errorf("language permission migration: database is nil")
	}
	var appliedCount int64
	if err := db.Model(&migrationmodels.Migration{}).
		Where("version = ?", version).
		Count(&appliedCount).Error; err != nil {
		return nil, fmt.Errorf("language permission migration: check version: %w", err)
	}
	alreadyApplied := appliedCount > 0
	affected := make([]adminRoutePermissionAffectedRole, 0)

	err := db.Transaction(func(tx *gorm.DB) error {
		activeSources := make(map[string]bool, len(languageManagementPermissionSeeds))
		for i := range languageManagementPermissionSeeds {
			seed := languageManagementPermissionSeeds[i]
			key := adminRoutePermissionSourceKey(seed.ParentType, seed.ParentPath)
			if _, checked := activeSources[key]; checked {
				continue
			}
			var count int64
			if err := tx.Model(&models.Menu{}).
				Where("type = ? AND path = ? AND status = ?", seed.ParentType, seed.ParentPath, enum.Enabled).
				Count(&count).Error; err != nil {
				return fmt.Errorf("language permission migration: inspect source %q: %w", seed.ParentPath, err)
			}
			activeSources[key] = count > 0
		}

		for i := range languageManagementComponentSeeds {
			if err := upsertAdminRouteComponent(tx, languageManagementComponentSeeds[i]); err != nil {
				return fmt.Errorf("language permission migration: upsert component: %w", err)
			}
		}
		for i := range languageManagementPermissionSeeds {
			seed := languageManagementPermissionSeeds[i]
			if err := upsertAdminRoutePermission(tx, seed); err != nil {
				return fmt.Errorf("language permission migration: upsert API: %w", err)
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
			return affected, fmt.Errorf("language permission migration: reload Casbin policy: %w", err)
		}
	}
	return affected, nil
}
