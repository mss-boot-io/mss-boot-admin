package system

import (
	"fmt"
	"runtime"
	"strings"
	"time"

	"github.com/mss-boot-io/mss-boot-admin/admin/models"
	"github.com/mss-boot-io/mss-boot-admin/admin/pkg"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/enum"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/migration"
	migrationmodels "github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/migration/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	leastPrivilegeDefaultRoleName          = "user"
	leastPrivilegeAuthorizationScopeRole   = "role"
	leastPrivilegeAuthorizationScopeGlobal = "global"
	leastPrivilegeAuthorizationResource    = "authorization"
)

func init() {
	_, fileName, _, _ := runtime.Caller(0)
	migration.Migrate.SetVersion(
		migration.GetFilename(fileName),
		_20260808100000LeastPrivilegeDefaultRole,
	)
}

func _20260808100000LeastPrivilegeDefaultRole(db *gorm.DB, version string) error {
	return migrateLeastPrivilegeDefaultRole(db, version)
}

// migrateLeastPrivilegeDefaultRole separates public provisioning from the
// historical root+default Admin role. It is deliberately forward-only: no
// existing user is reassigned and root authority is never copied. Ambiguous
// installations fail before recording the migration version so an operator
// can repair the role topology explicitly and rerun it.
func migrateLeastPrivilegeDefaultRole(db *gorm.DB, version string) error {
	if db == nil {
		return fmt.Errorf("least-privilege default role migration: database is nil")
	}
	return db.Transaction(func(tx *gorm.DB) error {
		var applied int64
		if err := tx.Model(&migrationmodels.Migration{}).
			Where("version = ?", version).
			Count(&applied).Error; err != nil {
			return fmt.Errorf("least-privilege default role migration: check version: %w", err)
		}
		if applied > 0 {
			return nil
		}

		defaultRole, err := resolveLeastPrivilegeDefaultRole(tx)
		if err != nil {
			return err
		}
		if err := seedLeastPrivilegeWelcome(tx, defaultRole.ID); err != nil {
			return err
		}
		if err := advanceLeastPrivilegeAuthorizationRevisions(tx, defaultRole.ID); err != nil {
			return err
		}

		versionRow := &migrationmodels.Migration{}
		versionRow.SetVersion(version)
		if err := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "version"}},
			DoNothing: true,
		}).Create(versionRow).Error; err != nil {
			return fmt.Errorf("least-privilege default role migration: record version: %w", err)
		}
		return nil
	})
}

// advanceLeastPrivilegeAuthorizationRevisions records the migration's policy
// projection in the same durable revision domain used by request middleware.
// No watcher is required during migration: every upgraded process observes the
// new global value on its next authorized request and reloads synchronously.
func advanceLeastPrivilegeAuthorizationRevisions(tx *gorm.DB, roleID string) error {
	if strings.TrimSpace(roleID) == "" {
		return fmt.Errorf("least-privilege default role migration: authorization revision role id is empty")
	}
	keys := []models.ConfigRevision{
		{
			Scope:    leastPrivilegeAuthorizationScopeGlobal,
			OwnerID:  "",
			Resource: leastPrivilegeAuthorizationResource,
		},
		{
			Scope:    leastPrivilegeAuthorizationScopeRole,
			OwnerID:  roleID,
			Resource: leastPrivilegeAuthorizationResource,
		},
	}
	for i := range keys {
		if err := advanceLeastPrivilegeAuthorizationRevision(tx, keys[i]); err != nil {
			return err
		}
	}
	return nil
}

func advanceLeastPrivilegeAuthorizationRevision(tx *gorm.DB, key models.ConfigRevision) error {
	key.Revision = 0
	key.UpdatedAt = time.Now()
	if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&key).Error; err != nil {
		return fmt.Errorf(
			"least-privilege default role migration: ensure authorization revision %s/%s: %w",
			key.Scope,
			key.OwnerID,
			err,
		)
	}
	var current models.ConfigRevision
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where(
		"scope = ? AND owner_id = ? AND resource = ?",
		key.Scope,
		key.OwnerID,
		key.Resource,
	).Take(&current).Error; err != nil {
		return fmt.Errorf(
			"least-privilege default role migration: lock authorization revision %s/%s: %w",
			key.Scope,
			key.OwnerID,
			err,
		)
	}
	if current.Revision < 0 || current.Revision == int64(^uint64(0)>>1) {
		return fmt.Errorf(
			"least-privilege default role migration: authorization revision cannot advance for %s/%s",
			key.Scope,
			key.OwnerID,
		)
	}
	result := tx.Model(&models.ConfigRevision{}).Where(
		"scope = ? AND owner_id = ? AND resource = ? AND revision = ?",
		key.Scope,
		key.OwnerID,
		key.Resource,
		current.Revision,
	).Updates(map[string]any{
		"revision":   current.Revision + 1,
		"updated_at": time.Now(),
	})
	if result.Error != nil {
		return fmt.Errorf(
			"least-privilege default role migration: advance authorization revision %s/%s: %w",
			key.Scope,
			key.OwnerID,
			result.Error,
		)
	}
	if result.RowsAffected != 1 {
		return fmt.Errorf(
			"least-privilege default role migration: authorization revision changed concurrently for %s/%s",
			key.Scope,
			key.OwnerID,
		)
	}
	return nil
}

func resolveLeastPrivilegeDefaultRole(tx *gorm.DB) (*models.Role, error) {
	var defaults []models.Role
	if err := tx.Model(&models.Role{}).
		Where(map[string]any{"default": true}).
		Order("id ASC").
		Find(&defaults).Error; err != nil {
		return nil, fmt.Errorf("least-privilege default role migration: inspect defaults: %w", err)
	}
	if len(defaults) > 1 {
		ids := make([]string, 0, len(defaults))
		for i := range defaults {
			ids = append(ids, defaults[i].ID)
		}
		return nil, fmt.Errorf(
			"least-privilege default role migration: ambiguous default roles count=%d ids=%s",
			len(defaults),
			strings.Join(ids, ","),
		)
	}
	if len(defaults) == 0 {
		return createLeastPrivilegeDefaultRole(tx)
	}

	current := &defaults[0]
	if !current.Root {
		if current.Status != enum.Enabled {
			return nil, fmt.Errorf(
				"least-privilege default role migration: existing non-root default role %q is disabled",
				current.ID,
			)
		}
		return current, nil
	}
	if current.Name != "admin" {
		return nil, fmt.Errorf(
			"least-privilege default role migration: root-default role %q name=%q is not canonical admin",
			current.ID,
			current.Name,
		)
	}

	var assignedUsers int64
	if err := tx.Model(&models.User{}).
		Where("role_id = ?", current.ID).
		Count(&assignedUsers).Error; err != nil {
		return nil, fmt.Errorf(
			"least-privilege default role migration: inspect users assigned to root-default %q: %w",
			current.ID,
			err,
		)
	}
	if assignedUsers != 1 {
		return nil, fmt.Errorf(
			"least-privilege default role migration: root-default role %q is assigned to %d users; expected exactly one canonical admin",
			current.ID,
			assignedUsers,
		)
	}

	provisioningRole, err := createLeastPrivilegeDefaultRole(tx)
	if err != nil {
		return nil, err
	}
	if err := tx.Table(current.TableName()).
		Where("id = ?", current.ID).
		UpdateColumn("default", false).Error; err != nil {
		return nil, fmt.Errorf(
			"least-privilege default role migration: clear historical admin default: %w",
			err,
		)
	}
	return provisioningRole, nil
}

func createLeastPrivilegeDefaultRole(tx *gorm.DB) (*models.Role, error) {
	role := &models.Role{
		Name:   leastPrivilegeDefaultRoleName,
		Status: enum.Enabled,
		Remark: "least-privilege role for public user provisioning",
	}
	if err := tx.Create(role).Error; err != nil {
		return nil, fmt.Errorf("least-privilege default role migration: create user role: %w", err)
	}
	if err := tx.Table(role.TableName()).
		Where("id = ?", role.ID).
		Updates(map[string]any{"default": true, "root": false}).Error; err != nil {
		return nil, fmt.Errorf("least-privilege default role migration: mark user role default: %w", err)
	}
	role.Default = true
	role.Root = false
	return role, nil
}

func seedLeastPrivilegeWelcome(tx *gorm.DB, roleID string) error {
	if roleID == "" {
		return fmt.Errorf("least-privilege default role migration: provisioning role id is empty")
	}
	rules := []models.CasbinRule{
		{PType: "p", V0: roleID, V1: pkg.MenuAccessType.String(), V2: "/welcome", V3: "GET"},
		{PType: "p", V0: roleID, V1: pkg.APIAccessType.String(), V2: "/admin/api/monitor", V3: "GET"},
	}
	for i := range rules {
		rule := &rules[i]
		if err := tx.Where(
			"ptype = ? AND v0 = ? AND v1 = ? AND v2 = ? AND v3 = ?",
			rule.PType,
			rule.V0,
			rule.V1,
			rule.V2,
			rule.V3,
		).FirstOrCreate(rule).Error; err != nil {
			return fmt.Errorf(
				"least-privilege default role migration: seed %s %s: %w",
				rule.V1,
				rule.V2,
				err,
			)
		}
	}
	return nil
}
