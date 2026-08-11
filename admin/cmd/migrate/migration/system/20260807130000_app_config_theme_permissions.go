package system

import (
	"fmt"
	"log/slog"
	"runtime"
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

func init() {
	_, fileName, _, _ := runtime.Caller(0)
	migration.Migrate.SetV100Version(fileName, _20260807130000AppConfigThemePermissions)
}

// GET and PUT use the registered Gin route template because authorization is
// evaluated against Context.FullPath(). DELETE is a dedicated static route.
// Storing the browser-visible /theme URL for GET or PUT would create policies
// that can never match the authorization object.
var appConfigThemePermissionSeeds = []adminRoutePermissionSeed{
	{
		Name:       "api.appConfig.theme.read",
		Path:       "/admin/api/app-configs/:group",
		Method:     "GET",
		Permission: "config:read",
		ParentPath: "/app-config",
		ParentType: pkg.MenuAccessType,
	},
	{
		Name:       "api.appConfig.theme.update",
		Path:       "/admin/api/app-configs/:group",
		Method:     "PUT",
		Permission: "config:write",
		ParentPath: "/app-config/control",
		ParentType: pkg.ComponentAccessType,
	},
	{
		Name:       "api.appConfig.theme.reset",
		Path:       "/admin/api/app-configs/theme",
		Method:     "DELETE",
		Permission: "config:write",
		ParentPath: "/app-config/control",
		ParentType: pkg.ComponentAccessType,
	},
}

var appConfigControlComponentSeed = adminRouteComponentSeed{
	Name:       "menu.super-permission.appConfig.control",
	Path:       "/app-config/control",
	Permission: "config:write",
	ParentPath: "/app-config",
}

type appConfigThemePermissionMigrationReport struct {
	CompatibilityInherited []adminRoutePermissionAffectedRole
}

func _20260807130000AppConfigThemePermissions(db *gorm.DB, version string) error {
	report, err := migrateAppConfigThemePermissions(db, version)
	if err != nil {
		return err
	}
	if len(report.CompatibilityInherited) > 0 {
		slog.Info(
			"application theme read permission inherited for upgrade compatibility",
			"affectedCount", len(report.CompatibilityInherited),
			"compatibilityInherited", report.CompatibilityInherited,
		)
	}
	return nil
}

// migrateAppConfigThemePermissions makes the three protected application
// theme operations discoverable through the existing /app-config menu. On the
// first application only, roles with an exact grant to the active menu inherit
// only the new read policy so an upgrade does not revoke an already visible
// page. Write and reset remain behind a separately assignable component and
// are never granted by compatibility inheritance. Unrelated, wildcard, stale,
// and wrong-method grants remain fail-closed.
func migrateAppConfigThemePermissions(
	db *gorm.DB,
	version string,
) (appConfigThemePermissionMigrationReport, error) {
	report := appConfigThemePermissionMigrationReport{
		CompatibilityInherited: make([]adminRoutePermissionAffectedRole, 0),
	}
	if db == nil {
		return report, fmt.Errorf("app-config theme permission migration: database is nil")
	}

	err := db.Transaction(func(tx *gorm.DB) error {
		var appliedCount int64
		if err := tx.Model(&migrationmodels.Migration{}).
			Where("version = ?", version).
			Count(&appliedCount).Error; err != nil {
			return fmt.Errorf("app-config theme permission migration: check version: %w", err)
		}

		if err := requireActiveAppConfigMenu(tx); err != nil {
			return err
		}
		if err := upsertAdminRouteComponent(tx, appConfigControlComponentSeed); err != nil {
			return fmt.Errorf("app-config theme permission migration: upsert write component: %w", err)
		}
		for i := range appConfigThemePermissionSeeds {
			if err := upsertAdminRoutePermission(tx, appConfigThemePermissionSeeds[i]); err != nil {
				return fmt.Errorf("app-config theme permission migration: upsert API metadata: %w", err)
			}
		}
		if appliedCount == 0 {
			inherited, err := inheritExistingAdminRoutePermission(tx, appConfigThemePermissionSeeds[0])
			if err != nil {
				return err
			}
			report.CompatibilityInherited = append(report.CompatibilityInherited, inherited...)
		}

		versionRow := &migrationmodels.Migration{}
		versionRow.SetVersion(version)
		return tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "version"}},
			DoNothing: true,
		}).Create(versionRow).Error
	})
	if err != nil {
		return appConfigThemePermissionMigrationReport{}, err
	}

	sort.Slice(report.CompatibilityInherited, func(i, j int) bool {
		left, right := report.CompatibilityInherited[i], report.CompatibilityInherited[j]
		if left.RoleID != right.RoleID {
			return left.RoleID < right.RoleID
		}
		if left.APIPath != right.APIPath {
			return left.APIPath < right.APIPath
		}
		return left.Method < right.Method
	})

	if gormdb.Enforcer != nil {
		if err := gormdb.Enforcer.LoadPolicy(); err != nil {
			return report, fmt.Errorf("app-config theme permission migration: reload Casbin policy: %w", err)
		}
	}
	return report, nil
}

func requireActiveAppConfigMenu(tx *gorm.DB) error {
	var menus []models.Menu
	if err := tx.Unscoped().
		Where("type = ? AND path = ?", pkg.MenuAccessType, "/app-config").
		Order("id").
		Find(&menus).Error; err != nil {
		return fmt.Errorf("app-config theme permission migration: inspect /app-config menu: %w", err)
	}
	if len(menus) != 1 {
		return fmt.Errorf(
			"app-config theme permission migration: expected one /app-config MENU parent, found %d",
			len(menus),
		)
	}
	if menus[0].DeletedAt.Valid {
		return fmt.Errorf("app-config theme permission migration: /app-config MENU parent is soft-deleted")
	}
	if menus[0].Status != enum.Enabled {
		return fmt.Errorf("app-config theme permission migration: /app-config MENU parent is disabled")
	}
	return nil
}
