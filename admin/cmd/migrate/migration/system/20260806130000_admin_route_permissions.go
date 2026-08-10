package system

import (
	"fmt"
	"log/slog"
	"regexp"
	"runtime"
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

func init() {
	_, fileName, _, _ := runtime.Caller(0)
	migration.Migrate.SetV100Version(fileName, _20260806130000AdminRoutePermissions)
}

type adminRoutePermissionSeed struct {
	Name       string
	Path       string
	Method     string
	Permission string
	ParentPath string
	ParentType pkg.AccessType
}

type adminRouteComponentSeed struct {
	Name       string
	Path       string
	Permission string
	ParentPath string
}

// adminRoutePermissionAffectedRole is deliberately limited to permission
// metadata. It is safe to include in migration logs and gives operators an
// auditable, deterministic record of every compatibility policy added.
type adminRoutePermissionAffectedRole struct {
	RoleID     string
	SourceType string
	SourcePath string
	APIPath    string
	Method     string
}

type adminRoutePermissionMigrationReport struct {
	AffectedRoles []adminRoutePermissionAffectedRole
}

var adminRouteComponentSeeds = []adminRouteComponentSeed{
	{
		Name:       "menu.origination.user.password-reset",
		Path:       "/users/password-reset",
		Permission: "user:password-reset",
		ParentPath: "/users",
	},
	{
		Name:       "menu.system.task.operate",
		Path:       "/task/operate",
		Permission: "task:operate",
		ParentPath: "/task",
	},
	{
		Name:       "menu.system.log.runtime",
		Path:       "/log/runtime",
		Permission: "log:read",
		ParentPath: "/log",
	},
	{
		Name:       "menu.system.log.export",
		Path:       "/log/export",
		Permission: "log:export",
		ParentPath: "/log",
	},
	{
		Name:       "menu.origination.department.create",
		Path:       "/departments/create",
		Permission: "department:create",
		ParentPath: "/departments",
	},
	{
		Name:       "menu.origination.department.edit",
		Path:       "/departments/edit",
		Permission: "department:update",
		ParentPath: "/departments",
	},
	{
		Name:       "menu.origination.department.delete",
		Path:       "/departments/delete",
		Permission: "department:delete",
		ParentPath: "/departments",
	},
	{
		Name:       "menu.origination.post.create",
		Path:       "/posts/create",
		Permission: "post:create",
		ParentPath: "/posts",
	},
	{
		Name:       "menu.origination.post.edit",
		Path:       "/posts/edit",
		Permission: "post:update",
		ParentPath: "/posts",
	},
	{
		Name:       "menu.origination.post.delete",
		Path:       "/posts/delete",
		Permission: "post:delete",
		ParentPath: "/posts",
	},
}

var monitorReadPermissionSeed = adminRoutePermissionSeed{
	Name:       "api.monitor.read",
	Path:       "/admin/api/monitor",
	Method:     "GET",
	Permission: "monitor:read",
	ParentPath: "/welcome",
	ParentType: pkg.MenuAccessType,
}

var adminRoutePermissionSeeds = []adminRoutePermissionSeed{
	{
		Name:       "api.user.passwordReset",
		Path:       "/admin/api/user/:userID/password-reset",
		Method:     "PUT",
		Permission: "user:password-reset",
		ParentPath: "/users/password-reset",
		ParentType: pkg.ComponentAccessType,
	},
	{
		Name:       "api.audit.login.read",
		Path:       "/admin/api/audit-logs/login",
		Method:     "GET",
		Permission: "audit:read",
		ParentPath: "/log",
		ParentType: pkg.MenuAccessType,
	},
	{
		Name:       "api.audit.operation.read",
		Path:       "/admin/api/audit-logs/operation",
		Method:     "GET",
		Permission: "audit:read",
		ParentPath: "/log",
		ParentType: pkg.MenuAccessType,
	},
	{
		Name:       "api.log.runtime.read",
		Path:       "/admin/api/logs",
		Method:     "GET",
		Permission: "log:read",
		ParentPath: "/log/runtime",
		ParentType: pkg.ComponentAccessType,
	},
	{
		Name:       "api.log.runtime.files",
		Path:       "/admin/api/logs/files",
		Method:     "GET",
		Permission: "log:read",
		ParentPath: "/log/runtime",
		ParentType: pkg.ComponentAccessType,
	},
	{
		Name:       "api.log.runtime.export",
		Path:       "/admin/api/logs/export",
		Method:     "GET",
		Permission: "log:export",
		ParentPath: "/log/export",
		ParentType: pkg.ComponentAccessType,
	},
	{
		Name:       "api.task.operate",
		Path:       "/admin/api/tasks/:id/actions/:operate",
		Method:     "POST",
		Permission: "task:operate",
		ParentPath: "/task/operate",
		ParentType: pkg.ComponentAccessType,
	},
	{
		Name:       "api.task.funcList",
		Path:       "/admin/api/task/func-list",
		Method:     "GET",
		Permission: "task:read",
		ParentPath: "/task",
		ParentType: pkg.MenuAccessType,
	},
	{
		Name:       "api.systemConfig.read",
		Path:       "/admin/api/system-configs",
		Method:     "GET",
		Permission: "config:read",
		ParentPath: "/system-config",
		ParentType: pkg.MenuAccessType,
	},
	{
		Name:       "api.language.list",
		Path:       "/admin/api/languages",
		Method:     "GET",
		Permission: "language:read",
		ParentPath: "/language",
		ParentType: pkg.MenuAccessType,
	},
	{
		Name:       "api.department.list",
		Path:       "/admin/api/departments",
		Method:     "GET",
		Permission: "department:read",
		ParentPath: "/departments",
		ParentType: pkg.MenuAccessType,
	},
	{
		Name:       "api.department.create",
		Path:       "/admin/api/departments",
		Method:     "POST",
		Permission: "department:create",
		ParentPath: "/departments/create",
		ParentType: pkg.ComponentAccessType,
	},
	{
		Name:       "api.department.read",
		Path:       "/admin/api/departments/:id",
		Method:     "GET",
		Permission: "department:read",
		ParentPath: "/departments",
		ParentType: pkg.MenuAccessType,
	},
	{
		Name:       "api.department.update",
		Path:       "/admin/api/departments/:id",
		Method:     "PUT",
		Permission: "department:update",
		ParentPath: "/departments/edit",
		ParentType: pkg.ComponentAccessType,
	},
	{
		Name:       "api.department.delete",
		Path:       "/admin/api/departments/:id",
		Method:     "DELETE",
		Permission: "department:delete",
		ParentPath: "/departments/delete",
		ParentType: pkg.ComponentAccessType,
	},
	{
		Name:       "api.post.list",
		Path:       "/admin/api/posts",
		Method:     "GET",
		Permission: "post:read",
		ParentPath: "/posts",
		ParentType: pkg.MenuAccessType,
	},
	{
		Name:       "api.post.create",
		Path:       "/admin/api/posts",
		Method:     "POST",
		Permission: "post:create",
		ParentPath: "/posts/create",
		ParentType: pkg.ComponentAccessType,
	},
	{
		Name:       "api.post.read",
		Path:       "/admin/api/posts/:id",
		Method:     "GET",
		Permission: "post:read",
		ParentPath: "/posts",
		ParentType: pkg.MenuAccessType,
	},
	{
		Name:       "api.post.update",
		Path:       "/admin/api/posts/:id",
		Method:     "PUT",
		Permission: "post:update",
		ParentPath: "/posts/edit",
		ParentType: pkg.ComponentAccessType,
	},
	{
		Name:       "api.post.delete",
		Path:       "/admin/api/posts/:id",
		Method:     "DELETE",
		Permission: "post:delete",
		ParentPath: "/posts/delete",
		ParentType: pkg.ComponentAccessType,
	},
}

// _20260806130000AdminRoutePermissions registers discoverable API permission
// metadata for release-blocking custom routes. Upgrade compatibility is
// intentionally narrow: an exact API rule is added only when the same role
// already owns the exact MENU/COMPONENT parent rule. Fresh installations and
// unrelated roles receive no implicit grants.
func _20260806130000AdminRoutePermissions(db *gorm.DB, version string) error {
	report, err := migrateAdminRoutePermissionsWithReport(db, version)
	if err != nil {
		return err
	}
	if len(report.AffectedRoles) > 0 {
		slog.Info(
			"admin route permission migration inherited existing exact grants",
			"affectedCount", len(report.AffectedRoles),
			"affectedRoles", report.AffectedRoles,
		)
	}
	return nil
}

func migrateAdminRoutePermissionsWithReport(db *gorm.DB, version string) (adminRoutePermissionMigrationReport, error) {
	report := adminRoutePermissionMigrationReport{AffectedRoles: make([]adminRoutePermissionAffectedRole, 0)}
	if db == nil {
		return report, fmt.Errorf("admin route permission migration: database is nil")
	}
	var appliedCount int64
	if err := db.Model(&migrationmodels.Migration{}).Where("version = ?", version).Count(&appliedCount).Error; err != nil {
		return report, fmt.Errorf("admin route permission migration: check migration version: %w", err)
	}
	alreadyApplied := appliedCount > 0

	err := db.Transaction(func(tx *gorm.DB) error {
		activeSources, err := loadActiveAdminRoutePermissionSources(tx)
		if err != nil {
			return err
		}
		if err := ensureAdminLogMenu(tx); err != nil {
			return err
		}
		for i := range adminRouteComponentSeeds {
			if err := upsertAdminRouteComponent(tx, adminRouteComponentSeeds[i]); err != nil {
				return err
			}
		}
		for i := range adminRoutePermissionSeeds {
			seed := adminRoutePermissionSeeds[i]
			if err := upsertAdminRoutePermission(tx, seed); err != nil {
				return err
			}
			if !alreadyApplied && activeSources[adminRoutePermissionSourceKey(seed.ParentType, seed.ParentPath)] {
				affected, err := inheritExistingAdminRoutePermission(tx, seed)
				if err != nil {
					return err
				}
				report.AffectedRoles = append(report.AffectedRoles, affected...)
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
		return adminRoutePermissionMigrationReport{AffectedRoles: make([]adminRoutePermissionAffectedRole, 0)}, err
	}

	sort.Slice(report.AffectedRoles, func(i, j int) bool {
		left, right := report.AffectedRoles[i], report.AffectedRoles[j]
		if left.RoleID != right.RoleID {
			return left.RoleID < right.RoleID
		}
		if left.SourceType != right.SourceType {
			return left.SourceType < right.SourceType
		}
		if left.SourcePath != right.SourcePath {
			return left.SourcePath < right.SourcePath
		}
		if left.APIPath != right.APIPath {
			return left.APIPath < right.APIPath
		}
		return left.Method < right.Method
	})

	// Migrations commonly run before the HTTP server starts, but tests and
	// embedded deployments may already have an installed enforcer. Reload only
	// after the transaction commits so the in-memory policy never observes a
	// state that could still roll back.
	if gormdb.Enforcer != nil {
		if err := gormdb.Enforcer.LoadPolicy(); err != nil {
			return report, fmt.Errorf("admin route permission migration: reload Casbin policy: %w", err)
		}
	}
	return report, nil
}

func adminRoutePermissionSourceKey(accessType pkg.AccessType, path string) string {
	return accessType.String() + "\x00" + path
}

// loadActiveAdminRoutePermissionSources snapshots inheritance eligibility
// before this migration creates or repairs permission nodes. A stale Casbin
// rule attached to a missing, disabled, or soft-deleted source must never be
// converted into a live API grant merely because the migration repairs its
// metadata later in the same transaction.
func loadActiveAdminRoutePermissionSources(tx *gorm.DB) (map[string]bool, error) {
	active := make(map[string]bool)
	for i := range adminRoutePermissionSeeds {
		seed := adminRoutePermissionSeeds[i]
		key := adminRoutePermissionSourceKey(seed.ParentType, seed.ParentPath)
		if _, checked := active[key]; checked {
			continue
		}
		var count int64
		if err := tx.Model(&models.Menu{}).
			Where("type = ? AND path = ? AND status = ?", seed.ParentType, seed.ParentPath, enum.Enabled).
			Count(&count).Error; err != nil {
			return nil, fmt.Errorf(
				"admin route permission migration: inspect active %s source %q: %w",
				seed.ParentType,
				seed.ParentPath,
				err,
			)
		}
		active[key] = count > 0
	}
	return active, nil
}

// ensureAdminLogMenu makes the audit APIs assignable through SetAuthorize.
// The frontend historically supplied /log as a static route only, while role
// authorization accepts MENU/COMPONENT rows and then includes their API
// children. Hooks are skipped only on initial creation so this migration does
// not silently grant /log to the default role. An existing active /log row is
// treated as operator-owned configuration: only empty compatibility fields are
// filled. A soft-deleted row requires an explicit operator restore and is never
// revived by this migration.
func ensureAdminLogMenu(tx *gorm.DB) error {
	var menu models.Menu
	result := tx.Unscoped().
		Where("type = ? AND path = ?", pkg.MenuAccessType, "/log").
		Limit(1).
		Find(&menu)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected > 0 {
		if menu.DeletedAt.Valid {
			return fmt.Errorf("admin route permission migration: /log menu %q is soft-deleted; restore it explicitly before retrying", menu.ID)
		}
		if menu.ParentID != "" {
			if err := validateAdminLogMenuParent(tx, menu.ParentID); err != nil {
				return err
			}
		}
		metadata := make(map[string]any)
		if strings.TrimSpace(menu.Name) == "" {
			metadata["name"] = "menu.system.log"
		}
		if strings.TrimSpace(menu.ParentID) == "" {
			parentID, err := resolveAdminLogMenuParentID(tx)
			if err != nil {
				return err
			}
			metadata["parent_id"] = parentID
		}
		if strings.TrimSpace(menu.Method) == "" {
			metadata["method"] = "GET"
		}
		if strings.TrimSpace(menu.Component) == "" {
			metadata["component"] = "./Log"
		}
		if strings.TrimSpace(menu.Icon) == "" {
			metadata["icon"] = "fileText"
		}
		if strings.TrimSpace(menu.Permission) == "" {
			metadata["permission"] = "audit:read"
		}
		if menu.Status == "" {
			metadata["status"] = enum.Enabled
		}
		if len(metadata) == 0 {
			return nil
		}
		return tx.Model(&menu).Updates(metadata).Error
	}

	parentID, err := resolveAdminLogMenuParentID(tx)
	if err != nil {
		return err
	}

	menu = models.Menu{
		Name:       "menu.system.log",
		Path:       "/log",
		Method:     "GET",
		Component:  "./Log",
		Icon:       "fileText",
		ParentID:   parentID,
		Type:       pkg.MenuAccessType,
		Permission: "audit:read",
		Status:     enum.Enabled,
		Sort:       65,
	}
	menu.ID = pkg.SimpleID()
	return tx.Session(&gorm.Session{SkipHooks: true}).Create(&menu).Error
}

func upsertAdminRouteComponent(tx *gorm.DB, seed adminRouteComponentSeed) error {
	parentID, err := resolveAdminRouteComponentParentID(tx, seed.ParentPath)
	if err != nil {
		return err
	}

	var component models.Menu
	result := tx.Unscoped().
		Where("type = ? AND path = ?", pkg.ComponentAccessType, seed.Path).
		Limit(1).
		Find(&component)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		component = models.Menu{
			Name:       seed.Name,
			Path:       seed.Path,
			Method:     "GET",
			ParentID:   parentID,
			Type:       pkg.ComponentAccessType,
			Permission: seed.Permission,
			Status:     enum.Enabled,
			HideInMenu: true,
		}
		return tx.Create(&component).Error
	}

	return tx.Unscoped().Model(&component).Updates(map[string]any{
		"name":         seed.Name,
		"method":       "GET",
		"parent_id":    parentID,
		"permission":   seed.Permission,
		"status":       enum.Enabled,
		"hide_in_menu": true,
		"deleted_at":   nil,
	}).Error
}

func resolveAdminRouteComponentParentID(tx *gorm.DB, path string) (string, error) {
	var parent models.Menu
	result := tx.Select("id").
		Where("path = ? AND type = ?", path, pkg.MenuAccessType).
		Order("id").
		Limit(1).
		Find(&parent)
	if result.Error != nil {
		return "", result.Error
	}
	if result.RowsAffected == 0 {
		return "", fmt.Errorf("admin route permission migration: missing MENU parent %q for COMPONENT seed", path)
	}
	return parent.ID, nil
}

func resolveAdminLogMenuParentID(tx *gorm.DB) (string, error) {
	var parent models.Menu
	result := tx.Select("id").
		Where("path = ? AND type IN ?", "/system", []pkg.AccessType{pkg.MenuAccessType, pkg.DirectoryAccessType}).
		Order("id").
		Limit(1).
		Find(&parent)
	if result.Error != nil {
		return "", result.Error
	}
	if result.RowsAffected == 0 {
		return "", fmt.Errorf("admin route permission migration: missing MENU/DIRECTORY parent %q for /log", "/system")
	}
	return parent.ID, nil
}

func validateAdminLogMenuParent(tx *gorm.DB, parentID string) error {
	var count int64
	if err := tx.Model(&models.Menu{}).
		Where("id = ? AND type IN ?", parentID, []pkg.AccessType{pkg.MenuAccessType, pkg.DirectoryAccessType}).
		Count(&count).Error; err != nil {
		return err
	}
	if count == 0 {
		return fmt.Errorf("admin route permission migration: /log references missing MENU/DIRECTORY parent id %q", parentID)
	}
	return nil
}

func upsertAdminRoutePermission(tx *gorm.DB, seed adminRoutePermissionSeed) error {
	parentID, err := resolveAdminRoutePermissionParentID(tx, seed.ParentPath, seed.ParentType, seed.Method, seed.Path)
	if err != nil {
		return err
	}

	var menu models.Menu
	result := tx.Unscoped().
		Where("type = ? AND path = ? AND method = ?", pkg.APIAccessType, seed.Path, seed.Method).
		Limit(1).
		Find(&menu)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		menu = models.Menu{
			Name:       seed.Name,
			Path:       seed.Path,
			Method:     seed.Method,
			ParentID:   parentID,
			Type:       pkg.APIAccessType,
			Permission: seed.Permission,
			Status:     enum.Enabled,
			HideInMenu: true,
		}
		return tx.Create(&menu).Error
	}

	return tx.Unscoped().Model(&menu).Updates(map[string]any{
		"name":         seed.Name,
		"parent_id":    parentID,
		"permission":   seed.Permission,
		"status":       enum.Enabled,
		"hide_in_menu": true,
		"deleted_at":   nil,
	}).Error
}

func resolveAdminRoutePermissionParentID(
	tx *gorm.DB,
	path string,
	accessType pkg.AccessType,
	method string,
	apiPath string,
) (string, error) {
	var parent models.Menu
	result := tx.Select("id").
		Where("path = ? AND type = ?", path, accessType).
		Order("id").
		Limit(1).
		Find(&parent)
	if result.Error != nil {
		return "", result.Error
	}
	if result.RowsAffected == 0 {
		return "", fmt.Errorf(
			"admin route permission migration: missing %s parent %q for API %s %s",
			accessType,
			path,
			method,
			apiPath,
		)
	}
	return parent.ID, nil
}

func inheritExistingAdminRoutePermission(
	tx *gorm.DB,
	seed adminRoutePermissionSeed,
) ([]adminRoutePermissionAffectedRole, error) {
	var sourceRules []models.CasbinRule
	if err := tx.Where(
		"ptype = ? AND v1 = ? AND v2 = ?",
		"p",
		seed.ParentType.String(),
		seed.ParentPath,
	).Order("v0, id").Find(&sourceRules).Error; err != nil {
		return nil, fmt.Errorf(
			"admin route permission migration: load exact %s grants for %q: %w",
			seed.ParentType,
			seed.ParentPath,
			err,
		)
	}

	affected := make([]adminRoutePermissionAffectedRole, 0, len(sourceRules))
	seenRoles := make(map[string]struct{}, len(sourceRules))
	for i := range sourceRules {
		roleID := strings.TrimSpace(sourceRules[i].V0)
		if roleID == "" || !adminSourceMethodAllows(sourceRules[i].V3, "GET") {
			continue
		}
		if _, ok := seenRoles[roleID]; ok {
			continue
		}
		seenRoles[roleID] = struct{}{}

		var count int64
		if err := tx.Model(&models.CasbinRule{}).
			Where(
				"ptype = ? AND v0 = ? AND v1 = ? AND v2 = ? AND v3 = ?",
				"p",
				roleID,
				pkg.APIAccessType.String(),
				seed.Path,
				seed.Method,
			).
			Count(&count).Error; err != nil {
			return nil, fmt.Errorf(
				"admin route permission migration: check inherited API grant for role %q, %s %s: %w",
				roleID,
				seed.Method,
				seed.Path,
				err,
			)
		}
		if count > 0 {
			continue
		}

		rule := &models.CasbinRule{
			PType: "p",
			V0:    roleID,
			V1:    pkg.APIAccessType.String(),
			V2:    seed.Path,
			V3:    seed.Method,
		}
		if err := tx.Create(rule).Error; err != nil {
			return nil, fmt.Errorf(
				"admin route permission migration: inherit API grant for role %q from %s %q to %s %s: %w",
				roleID,
				seed.ParentType,
				seed.ParentPath,
				seed.Method,
				seed.Path,
				err,
			)
		}
		affected = append(affected, adminRoutePermissionAffectedRole{
			RoleID:     roleID,
			SourceType: seed.ParentType.String(),
			SourcePath: seed.ParentPath,
			APIPath:    seed.Path,
			Method:     seed.Method,
		})
	}
	return affected, nil
}

func adminSourceMethodAllows(pattern, method string) bool {
	pattern = strings.TrimSpace(pattern)
	if pattern == "" {
		return false
	}
	compiled, err := regexp.Compile("^(?:" + pattern + ")$")
	return err == nil && compiled.MatchString(method)
}
