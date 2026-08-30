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

// presentationProfilePermissionsSafeAttestationVersion is deliberately not a
// registered migration. It is an independent ledger attestation written in
// the same transaction as the fail-closed implementation of 20260824121000.
// Databases that ran an older, overwrite-capable implementation have the
// predecessor version but cannot have this marker.
const presentationProfilePermissionsSafeAttestationVersion = "attestation:presentation-profile-permissions:20260824121000:fail-closed-v1"

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
		issueSafeAttestation, err := preparePresentationProfilePermissionSafeAttestation(tx, version)
		if err != nil {
			return err
		}
		menu, err := ensurePresentationProfileMenu(tx)
		if err != nil {
			return err
		}
		componentIDs := make(map[string]string, len(presentationProfileComponentSeeds))
		for i := range presentationProfileComponentSeeds {
			component, componentErr := ensurePresentationProfileComponent(
				tx,
				presentationProfileComponentSeeds[i],
				menu.ID,
			)
			if componentErr != nil {
				return componentErr
			}
			componentIDs[presentationProfileComponentSeeds[i].Path] = component.ID
		}
		for i := range presentationProfilePermissionSeeds {
			seed := presentationProfilePermissionSeeds[i]
			parentID, parentErr := presentationProfileAPIParentID(seed, menu.ID, componentIDs)
			if parentErr != nil {
				return parentErr
			}
			if _, permissionErr := ensurePresentationProfileAPI(tx, seed, parentID); permissionErr != nil {
				return permissionErr
			}
		}
		versionRow := &migrationmodels.Migration{}
		versionRow.SetVersion(version)
		if err := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "version"}},
			DoNothing: true,
		}).Create(versionRow).Error; err != nil {
			return err
		}
		if !issueSafeAttestation {
			return nil
		}
		attestation := &migrationmodels.Migration{}
		attestation.SetVersion(presentationProfilePermissionsSafeAttestationVersion)
		if err := tx.Create(attestation).Error; err != nil {
			return fmt.Errorf("presentation permission migration: record safe attestation: %w", err)
		}
		return nil
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

// preparePresentationProfilePermissionSafeAttestation prevents a historical
// database from acquiring the safe-code marker merely by invoking the helper
// after the overwrite-capable predecessor has already been recorded. Normal
// production execution reaches this function only through a runner that has
// already determined the canonical predecessor is pending.
func preparePresentationProfilePermissionSafeAttestation(tx *gorm.DB, version string) (bool, error) {
	if version != presentationProfilePermissionsMigrationID.String() {
		return false, nil
	}
	var predecessorRows int64
	if err := tx.Model(&migrationmodels.Migration{}).
		Where("version = ?", presentationProfilePermissionsMigrationID.String()).
		Count(&predecessorRows).Error; err != nil {
		return false, fmt.Errorf("presentation permission migration: inspect predecessor ledger: %w", err)
	}
	var attestationRows int64
	if err := tx.Model(&migrationmodels.Migration{}).
		Where("version = ?", presentationProfilePermissionsSafeAttestationVersion).
		Count(&attestationRows).Error; err != nil {
		return false, fmt.Errorf("presentation permission migration: inspect safe attestation: %w", err)
	}
	if predecessorRows != 0 || attestationRows != 0 {
		return false, fmt.Errorf(
			"presentation permission migration: cannot issue safe attestation because predecessor rows=%d attestation rows=%d; use the documented manual recovery review",
			predecessorRows,
			attestationRows,
		)
	}
	return true, nil
}

func ensurePresentationProfileMenu(tx *gorm.DB) (*models.Menu, error) {
	parentID, err := presentationSystemParentID(tx)
	if err != nil {
		return nil, err
	}
	return ensurePresentationProfilePermissionNode(tx, presentationProfileMenuPermissionNode(parentID))
}

func presentationProfileMenuPermissionNode(parentID string) models.Menu {
	return models.Menu{
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
}

func ensurePresentationProfileComponent(
	tx *gorm.DB,
	seed adminRouteComponentSeed,
	parentID string,
) (*models.Menu, error) {
	return ensurePresentationProfilePermissionNode(tx, presentationProfileComponentPermissionNode(seed, parentID))
}

func presentationProfileComponentPermissionNode(seed adminRouteComponentSeed, parentID string) models.Menu {
	return models.Menu{
		Name:       seed.Name,
		Path:       seed.Path,
		Method:     "GET",
		ParentID:   parentID,
		Type:       pkg.ComponentAccessType,
		Permission: seed.Permission,
		Status:     enum.Enabled,
		HideInMenu: true,
	}
}

func ensurePresentationProfileAPI(
	tx *gorm.DB,
	seed adminRoutePermissionSeed,
	parentID string,
) (*models.Menu, error) {
	return ensurePresentationProfilePermissionNode(tx, presentationProfileAPIPermissionNode(seed, parentID))
}

func presentationProfileAPIPermissionNode(seed adminRoutePermissionSeed, parentID string) models.Menu {
	return models.Menu{
		Name:       seed.Name,
		Path:       seed.Path,
		Method:     seed.Method,
		ParentID:   parentID,
		Type:       pkg.APIAccessType,
		Permission: seed.Permission,
		Status:     enum.Enabled,
		HideInMenu: true,
	}
}

func presentationProfileAPIParentID(
	seed adminRoutePermissionSeed,
	menuID string,
	componentIDs map[string]string,
) (string, error) {
	switch seed.ParentType {
	case pkg.MenuAccessType:
		if seed.ParentPath != presentationProfileMenuPath {
			return "", fmt.Errorf(
				"presentation permission migration: API %s %q references unknown MENU parent %q",
				seed.Method,
				seed.Path,
				seed.ParentPath,
			)
		}
		return menuID, nil
	case pkg.ComponentAccessType:
		parentID, ok := componentIDs[seed.ParentPath]
		if !ok {
			return "", fmt.Errorf(
				"presentation permission migration: API %s %q references unknown COMPONENT parent %q",
				seed.Method,
				seed.Path,
				seed.ParentPath,
			)
		}
		return parentID, nil
	default:
		return "", fmt.Errorf(
			"presentation permission migration: API %s %q has unsupported parent type %q",
			seed.Method,
			seed.Path,
			seed.ParentType,
		)
	}
}

// ensurePresentationProfilePermissionNode creates one managed permission node
// or reuses an exact active copy from an earlier successful run. Paths are the
// durable authorization identity: a downstream-owned, disabled, soft-deleted,
// duplicated, or cross-type occupant must never be repaired in place because
// doing so could silently change an operator's menu or role boundary.
func ensurePresentationProfilePermissionNode(tx *gorm.DB, expected models.Menu) (*models.Menu, error) {
	if tx == nil {
		return nil, fmt.Errorf("presentation permission migration: database is nil")
	}
	nodes, identity, err := findPresentationProfilePermissionNodes(tx, expected)
	if err != nil {
		return nil, err
	}
	if len(nodes) == 0 {
		expected.ID = pkg.SimpleID()
		create := tx
		if expected.Type == pkg.MenuAccessType {
			create = tx.Session(&gorm.Session{SkipHooks: true})
		}
		if err := create.Create(&expected).Error; err != nil {
			return nil, fmt.Errorf("presentation permission migration: create %s: %w", identity, err)
		}
		return &expected, nil
	}
	if len(nodes) != 1 {
		return nil, fmt.Errorf(
			"presentation permission migration: %s is occupied by %d access nodes; resolve the collision explicitly",
			identity,
			len(nodes),
		)
	}
	current := &nodes[0]
	if current.DeletedAt.Valid {
		return nil, fmt.Errorf(
			"presentation permission migration: %s node %q is soft-deleted; resolve the collision explicitly",
			identity,
			current.ID,
		)
	}
	if current.Status != enum.Enabled {
		return nil, fmt.Errorf(
			"presentation permission migration: %s node %q is disabled; resolve the collision explicitly",
			identity,
			current.ID,
		)
	}
	if !matchesPresentationProfilePermissionNode(current, &expected) {
		return nil, fmt.Errorf(
			"presentation permission migration: %s node %q does not match managed metadata; resolve the collision explicitly",
			identity,
			current.ID,
		)
	}
	return current, nil
}

func findPresentationProfilePermissionNodes(
	tx *gorm.DB,
	expected models.Menu,
) ([]models.Menu, string, error) {
	if tx == nil {
		return nil, "", fmt.Errorf("presentation permission migration: database is nil")
	}
	query := tx.Unscoped().Where("path = ?", expected.Path)
	identity := fmt.Sprintf("path %q", expected.Path)
	if expected.Type == pkg.APIAccessType {
		query = query.Where("method = ?", expected.Method)
		identity = fmt.Sprintf("API %s %q", expected.Method, expected.Path)
	}
	var nodes []models.Menu
	if err := query.Order("id").Find(&nodes).Error; err != nil {
		return nil, identity, fmt.Errorf("presentation permission migration: inspect %s: %w", identity, err)
	}
	return nodes, identity, nil
}

func matchesPresentationProfilePermissionNode(current, expected *models.Menu) bool {
	if current == nil || expected == nil {
		return false
	}
	return current.ParentID == expected.ParentID &&
		current.Name == expected.Name &&
		current.Path == expected.Path &&
		current.Method == expected.Method &&
		current.Component == expected.Component &&
		current.Icon == expected.Icon &&
		current.Target == expected.Target &&
		current.HeaderRender == expected.HeaderRender &&
		current.FooterRender == expected.FooterRender &&
		current.MenuRender == expected.MenuRender &&
		current.MenuHeaderRender == expected.MenuHeaderRender &&
		current.HideChildrenInMenu == expected.HideChildrenInMenu &&
		current.HideInMenu == expected.HideInMenu &&
		current.HideInBreadcrumb == expected.HideInBreadcrumb &&
		current.FlatMenu == expected.FlatMenu &&
		current.FixedHeader == expected.FixedHeader &&
		current.FixSiderbar == expected.FixSiderbar &&
		current.NavTheme == expected.NavTheme &&
		current.Layout == expected.Layout &&
		current.HeaderTheme == expected.HeaderTheme &&
		current.Type == expected.Type &&
		current.Permission == expected.Permission &&
		current.Status == expected.Status &&
		current.Sort == expected.Sort
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
