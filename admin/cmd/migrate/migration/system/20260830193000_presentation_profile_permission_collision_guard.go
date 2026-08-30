package system

import (
	"errors"
	"fmt"

	"github.com/mss-boot-io/mss-boot-admin/admin/models"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/migration"
	migrationmodels "github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/migration/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const presentationProfilePermissionCollisionGuardMigrationID migration.MigrationID = "20260830193000"

func init() {
	_ = migration.Migrate.Register(
		presentationProfilePermissionCollisionGuardMigrationID,
		_20260830193000PresentationProfilePermissionCollisionGuard,
	)
}

// _20260830193000PresentationProfilePermissionCollisionGuard is a forward-only
// audit for databases that already recorded 20260824121000. It never creates,
// updates, revives, reparents, or deletes a permission node or Casbin policy.
// The new version is recorded only when the complete inventory is canonical
// and the fail-closed predecessor implementation left its independent
// transaction-bound attestation. Canonical metadata alone is not proof that an
// older implementation did not overwrite a downstream-owned node.
func _20260830193000PresentationProfilePermissionCollisionGuard(db *gorm.DB, version string) error {
	if db == nil {
		return fmt.Errorf("presentation permission collision guard: database is nil")
	}
	return db.Transaction(func(tx *gorm.DB) error {
		oldVersion := &migrationmodels.Migration{}
		if err := tx.Where(
			"version = ?",
			presentationProfilePermissionsMigrationID.String(),
		).Take(oldVersion).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return fmt.Errorf(
					"presentation permission collision guard: required predecessor %s is not recorded",
					presentationProfilePermissionsMigrationID,
				)
			}
			return fmt.Errorf("presentation permission collision guard: read predecessor ledger: %w", err)
		}
		if oldVersion.ApplyTime.IsZero() {
			return presentationPermissionCollisionGuardRecoveryError(
				"the predecessor migration has no trustworthy ApplyTime",
			)
		}
		attestation := &migrationmodels.Migration{}
		if err := tx.Where(
			"version = ?",
			presentationProfilePermissionsSafeAttestationVersion,
		).Take(attestation).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return presentationPermissionCollisionGuardRecoveryError(
					fmt.Sprintf("required safe-code attestation %q is not recorded", presentationProfilePermissionsSafeAttestationVersion),
				)
			}
			return fmt.Errorf("presentation permission collision guard: read safe-code attestation: %w", err)
		}
		if attestation.ApplyTime.IsZero() || attestation.ApplyTime.Before(oldVersion.ApplyTime) {
			return presentationPermissionCollisionGuardRecoveryError(
				fmt.Sprintf(
					"safe-code attestation %q has an invalid ledger time",
					presentationProfilePermissionsSafeAttestationVersion,
				),
			)
		}

		parentID, err := presentationSystemParentID(tx)
		if err != nil {
			return err
		}
		menu, err := auditPresentationProfilePermissionNode(
			tx,
			presentationProfileMenuPermissionNode(parentID),
		)
		if err != nil {
			return err
		}

		componentIDs := make(map[string]string, len(presentationProfileComponentSeeds))
		for i := range presentationProfileComponentSeeds {
			seed := presentationProfileComponentSeeds[i]
			component, componentErr := auditPresentationProfilePermissionNode(
				tx,
				presentationProfileComponentPermissionNode(seed, menu.ID),
			)
			if componentErr != nil {
				return componentErr
			}
			componentIDs[seed.Path] = component.ID
		}

		for i := range presentationProfilePermissionSeeds {
			seed := presentationProfilePermissionSeeds[i]
			apiParentID, parentErr := presentationProfileAPIParentID(seed, menu.ID, componentIDs)
			if parentErr != nil {
				return parentErr
			}
			if _, auditErr := auditPresentationProfilePermissionNode(
				tx,
				presentationProfileAPIPermissionNode(seed, apiParentID),
			); auditErr != nil {
				return auditErr
			}
		}

		versionRow := &migrationmodels.Migration{}
		versionRow.SetVersion(version)
		if err := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "version"}},
			DoNothing: true,
		}).Create(versionRow).Error; err != nil {
			return fmt.Errorf("presentation permission collision guard: record version: %w", err)
		}
		return nil
	})
}

func auditPresentationProfilePermissionNode(
	tx *gorm.DB,
	expected models.Menu,
) (*models.Menu, error) {
	nodes, identity, err := findPresentationProfilePermissionNodes(tx, expected)
	if err != nil {
		return nil, err
	}
	if len(nodes) == 0 {
		return nil, presentationPermissionCollisionGuardRecoveryError(
			fmt.Sprintf("%s is missing", identity),
		)
	}
	if len(nodes) != 1 {
		return nil, presentationPermissionCollisionGuardRecoveryError(
			fmt.Sprintf("%s is occupied by %d access nodes", identity, len(nodes)),
		)
	}
	current := &nodes[0]
	if current.DeletedAt.Valid {
		return nil, presentationPermissionCollisionGuardRecoveryError(
			fmt.Sprintf("%s node %q is soft-deleted", identity, current.ID),
		)
	}
	if current.Status != expected.Status {
		return nil, presentationPermissionCollisionGuardRecoveryError(
			fmt.Sprintf("%s node %q is disabled", identity, current.ID),
		)
	}
	if !matchesPresentationProfilePermissionNode(current, &expected) {
		return nil, presentationPermissionCollisionGuardRecoveryError(
			fmt.Sprintf("%s node %q does not match managed metadata", identity, current.ID),
		)
	}
	return current, nil
}

func presentationPermissionCollisionGuardRecoveryError(reason string) error {
	return fmt.Errorf(
		"presentation permission collision guard %s: %s; databases that executed %s without safe-code attestation %q may have overwritten downstream menu metadata, and canonical values alone cannot prove otherwise, so automatic recovery is unsafe: stop writers, back up the database, restore the affected menu rows and Casbin policies from a pre-upgrade backup (or explicitly accept and review the complete canonical inventory), record the documented operator attestation, then rerun migration",
		presentationProfilePermissionCollisionGuardMigrationID,
		reason,
		presentationProfilePermissionsMigrationID,
		presentationProfilePermissionsSafeAttestationVersion,
	)
}
