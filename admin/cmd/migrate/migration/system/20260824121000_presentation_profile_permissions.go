package system

import (
	"fmt"
	"sort"
	"strings"

	"github.com/mss-boot-io/mss-boot-admin/admin/models"
	"github.com/mss-boot-io/mss-boot-admin/admin/pkg"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/config/gormdb"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/enum"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/migration"
	migrationmodels "github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/migration/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const presentationProfilePermissionsMigrationID migration.MigrationID = "20260824121000"

const presentationProfileMenuPath = "/presentation-config"

var presentationProfileComponentSeeds = []adminRouteComponentSeed{
	{
		Name:       "menu.system.presentation-config.draft",
		Path:       "/presentation-config/draft",
		Permission: "presentation:draft-write",
		ParentPath: presentationProfileMenuPath,
	},
	{
		Name:       "menu.system.presentation-config.publish",
		Path:       "/presentation-config/publish",
		Permission: "presentation:publish",
		ParentPath: presentationProfileMenuPath,
	},
	{
		Name:       "menu.system.presentation-config.rollback",
		Path:       "/presentation-config/rollback",
		Permission: "presentation:rollback",
		ParentPath: presentationProfileMenuPath,
	},
}

var presentationProfilePermissionSeeds = []adminRoutePermissionSeed{
	{
		Name: "api.presentation.capabilities.read", Path: "/admin/api/presentation-capabilities", Method: "GET",
		Permission: "presentation:read", ParentPath: presentationProfileMenuPath, ParentType: pkg.MenuAccessType,
	},
	{
		Name: "api.presentation.validate", Path: "/admin/api/presentation-profiles/validate", Method: "POST",
		Permission: "presentation:read", ParentPath: presentationProfileMenuPath, ParentType: pkg.MenuAccessType,
	},
	{
		Name: "api.presentation.list", Path: "/admin/api/presentation-profiles", Method: "GET",
		Permission: "presentation:read", ParentPath: presentationProfileMenuPath, ParentType: pkg.MenuAccessType,
	},
	{
		Name: "api.presentation.read", Path: "/admin/api/presentation-profiles/:id", Method: "GET",
		Permission: "presentation:read", ParentPath: presentationProfileMenuPath, ParentType: pkg.MenuAccessType,
	},
	{
		Name: "api.presentation.history.list", Path: "/admin/api/presentation-profiles/:id/revisions", Method: "GET",
		Permission: "presentation:read", ParentPath: presentationProfileMenuPath, ParentType: pkg.MenuAccessType,
	},
	{
		Name: "api.presentation.history.read", Path: "/admin/api/presentation-profiles/:id/revisions/:revision", Method: "GET",
		Permission: "presentation:read", ParentPath: presentationProfileMenuPath, ParentType: pkg.MenuAccessType,
	},
	{
		Name: "api.presentation.draft.create", Path: "/admin/api/presentation-profiles", Method: "POST",
		Permission: "presentation:draft-write", ParentPath: "/presentation-config/draft", ParentType: pkg.ComponentAccessType,
	},
	{
		Name: "api.presentation.draft.replace", Path: "/admin/api/presentation-profiles/:id/draft", Method: "PUT",
		Permission: "presentation:draft-write", ParentPath: "/presentation-config/draft", ParentType: pkg.ComponentAccessType,
	},
	{
		Name: "api.presentation.publish", Path: "/admin/api/presentation-profiles/:id/publish", Method: "POST",
		Permission: "presentation:publish", ParentPath: "/presentation-config/publish", ParentType: pkg.ComponentAccessType,
	},
	{
		Name: "api.presentation.rollback", Path: "/admin/api/presentation-profiles/:id/rollback", Method: "POST",
		Permission: "presentation:rollback", ParentPath: "/presentation-config/rollback", ParentType: pkg.ComponentAccessType,
	},
}

func init() {
	_ = migration.Migrate.Register(
		presentationProfilePermissionsMigrationID,
		_20260824121000PresentationProfilePermissions,
	)
}

func _20260824121000PresentationProfilePermissions(db *gorm.DB, version string) error {
	return migratePresentationProfilePermissions(db, version)
}

// migratePresentationProfilePermissions installs discoverable, independently
// assignable authority nodes. It deliberately adds no compatibility grants:
// this is a new governance capability and ordinary roles remain default-deny.
func migratePresentationProfilePermissions(db *gorm.DB, version string) error {
	if db == nil {
		return fmt.Errorf("presentation permission migration: database is nil")
	}
	err := db.Transaction(func(tx *gorm.DB) error {
		if err := ensurePresentationProfileMenu(tx); err != nil {
			return err
		}
		for i := range presentationProfileComponentSeeds {
			if err := upsertAdminRouteComponent(tx, presentationProfileComponentSeeds[i]); err != nil {
				return fmt.Errorf("presentation permission migration: upsert component: %w", err)
			}
		}
		for i := range presentationProfilePermissionSeeds {
			if err := upsertAdminRoutePermission(tx, presentationProfilePermissionSeeds[i]); err != nil {
				return fmt.Errorf("presentation permission migration: upsert API: %w", err)
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
			return fmt.Errorf("presentation permission migration: reload Casbin policy: %w", err)
		}
	}
	return nil
}

func ensurePresentationProfileMenu(tx *gorm.DB) error {
	parentID, err := presentationSystemParentID(tx)
	if err != nil {
		return err
	}
	var menu models.Menu
	result := tx.Unscoped().
		Where("type = ? AND path = ?", pkg.MenuAccessType, presentationProfileMenuPath).
		Order("id").Limit(1).Find(&menu)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		menu = models.Menu{
			Name:       "menu.system.presentation-config",
			Path:       presentationProfileMenuPath,
			Method:     "GET",
			Component:  "./PresentationConfig",
			Icon:       "layout",
			ParentID:   parentID,
			Type:       pkg.MenuAccessType,
			Permission: "presentation:read",
			Status:     enum.Enabled,
			Sort:       65,
		}
		menu.ID = pkg.SimpleID()
		return tx.Session(&gorm.Session{SkipHooks: true}).Create(&menu).Error
	}
	if menu.DeletedAt.Valid {
		return fmt.Errorf(
			"presentation permission migration: menu %q is soft-deleted; restore it explicitly before retrying",
			menu.ID,
		)
	}
	return tx.Unscoped().Model(&menu).Updates(map[string]any{
		"name":       "menu.system.presentation-config",
		"method":     "GET",
		"component":  "./PresentationConfig",
		"icon":       "layout",
		"parent_id":  parentID,
		"permission": "presentation:read",
		"status":     enum.Enabled,
		"sort":       65,
	}).Error
}

func presentationSystemParentID(tx *gorm.DB) (string, error) {
	var parents []models.Menu
	if err := tx.Select("id", "type", "status").
		Where("path = ? AND type IN ?", "/system", []pkg.AccessType{pkg.DirectoryAccessType, pkg.MenuAccessType}).
		Order("id").Find(&parents).Error; err != nil {
		return "", err
	}
	active := make([]string, 0, len(parents))
	for i := range parents {
		if parents[i].Status == enum.Enabled && strings.TrimSpace(parents[i].ID) != "" {
			active = append(active, parents[i].ID)
		}
	}
	sort.Strings(active)
	if len(active) != 1 {
		return "", fmt.Errorf("presentation permission migration: expected one active /system parent, found %d", len(active))
	}
	return active[0], nil
}
