package system

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/mss-boot-io/mss-boot-admin/admin/models"
	"github.com/mss-boot-io/mss-boot-admin/admin/pkg"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/enum"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/migration"
	migrationmodels "github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/migration/models"
	"gorm.io/gorm"
)

func TestPresentationPermissionCollisionGuardFreshRunnerAppliesPredecessorAndGuard(t *testing.T) {
	db := setupPresentationPermissionMigrationTest(t, true)
	oldCalls := 0
	guardCalls := 0
	runner := newPresentationPermissionCollisionGuardRunner(
		t,
		db,
		func(db *gorm.DB, version string) error {
			oldCalls++
			return _20260824121000PresentationProfilePermissions(db, version)
		},
		func(db *gorm.DB, version string) error {
			guardCalls++
			return _20260830193000PresentationProfilePermissionCollisionGuard(db, version)
		},
	)

	if err := runner.MigrateContext(context.Background()); err != nil {
		t.Fatalf("migrate fresh permission database: %v", err)
	}
	if oldCalls != 1 || guardCalls != 1 {
		t.Fatalf("migration calls = predecessor:%d guard:%d, want 1 and 1", oldCalls, guardCalls)
	}
	assertPresentationPermissionVersion(t, db, presentationProfilePermissionsMigrationID.String(), 1)
	assertPresentationPermissionVersion(t, db, presentationProfilePermissionsSafeAttestationVersion, 1)
	assertPresentationPermissionVersion(t, db, presentationProfilePermissionCollisionGuardMigrationID.String(), 1)

	nodesBeforeRepeat, policiesBeforeRepeat := presentationPermissionState(t, db)
	guardBeforeRepeat := presentationPermissionMigrationRow(
		t,
		db,
		presentationProfilePermissionCollisionGuardMigrationID.String(),
	)
	if err := runner.MigrateContext(context.Background()); err != nil {
		t.Fatalf("repeat fresh permission migration runner: %v", err)
	}
	if oldCalls != 1 || guardCalls != 1 {
		t.Fatalf("repeat migration calls = predecessor:%d guard:%d, want unchanged 1 and 1", oldCalls, guardCalls)
	}
	nodesAfterRepeat, policiesAfterRepeat := presentationPermissionState(t, db)
	if !reflect.DeepEqual(nodesBeforeRepeat, nodesAfterRepeat) {
		t.Fatalf("repeat runner changed permission nodes:\nbefore: %#v\nafter:  %#v", nodesBeforeRepeat, nodesAfterRepeat)
	}
	if !reflect.DeepEqual(policiesBeforeRepeat, policiesAfterRepeat) {
		t.Fatalf("repeat runner changed role policies:\nbefore: %#v\nafter:  %#v", policiesBeforeRepeat, policiesAfterRepeat)
	}
	guardAfterRepeat := presentationPermissionMigrationRow(
		t,
		db,
		presentationProfilePermissionCollisionGuardMigrationID.String(),
	)
	if !guardBeforeRepeat.ApplyTime.Equal(guardAfterRepeat.ApplyTime) {
		t.Fatalf("repeat runner changed guard ApplyTime from %s to %s", guardBeforeRepeat.ApplyTime, guardAfterRepeat.ApplyTime)
	}
}

func TestPresentationPermissionCollisionGuardRunnerSkipsAttestedPredecessor(t *testing.T) {
	db := setupPresentationPermissionMigrationTest(t, true)
	if err := migratePresentationProfilePermissions(db, presentationProfilePermissionsMigrationID.String()); err != nil {
		t.Fatalf("apply safe predecessor: %v", err)
	}
	menu := presentationPermissionNode(t, db, pkg.MenuAccessType, presentationProfileMenuPath, "GET")
	seedPresentationPermissionRolePolicy(t, db, "attested-presentation-reader", menu)
	nodesBefore, policiesBefore := presentationPermissionState(t, db)

	oldCalls := 0
	guardCalls := 0
	runner := newPresentationPermissionCollisionGuardRunner(
		t,
		db,
		func(*gorm.DB, string) error {
			oldCalls++
			return fmt.Errorf("recorded predecessor must not rerun")
		},
		func(db *gorm.DB, version string) error {
			guardCalls++
			return _20260830193000PresentationProfilePermissionCollisionGuard(db, version)
		},
	)

	if err := runner.MigrateContext(context.Background()); err != nil {
		t.Fatalf("migrate attested predecessor database: %v", err)
	}
	if oldCalls != 0 || guardCalls != 1 {
		t.Fatalf("migration calls = predecessor:%d guard:%d, want 0 and 1", oldCalls, guardCalls)
	}
	nodesAfter, policiesAfter := presentationPermissionState(t, db)
	if !reflect.DeepEqual(nodesBefore, nodesAfter) {
		t.Fatalf("guard changed attested permission nodes:\nbefore: %#v\nafter:  %#v", nodesBefore, nodesAfter)
	}
	if !reflect.DeepEqual(policiesBefore, policiesAfter) {
		t.Fatalf("guard changed attested role policies:\nbefore: %#v\nafter:  %#v", policiesBefore, policiesAfter)
	}
	assertPresentationPermissionVersion(t, db, presentationProfilePermissionCollisionGuardMigrationID.String(), 1)
}

func TestPresentationPermissionCollisionGuardRejectsUnattestedHistoricalPredecessorAtSecondPrecision(t *testing.T) {
	db := setupPresentationPermissionMigrationTest(t, true)
	if err := migratePresentationProfilePermissions(db, "historical-permission-fixture"); err != nil {
		t.Fatalf("create canonical historical inventory: %v", err)
	}
	secondPrecision := time.Now().UTC().Truncate(time.Second)
	if err := db.Model(&models.Menu{}).
		Where(
			"path = ? OR path LIKE ? OR path LIKE ?",
			presentationProfileMenuPath,
			presentationProfileMenuPath+"/%",
			"/admin/api/presentation-%",
		).
		UpdateColumns(map[string]any{
			"created_at": secondPrecision,
			"updated_at": secondPrecision,
		}).Error; err != nil {
		t.Fatalf("set second-precision historical timestamps: %v", err)
	}
	if err := db.Create(&migrationmodels.Migration{
		Version:   presentationProfilePermissionsMigrationID.String(),
		ApplyTime: secondPrecision,
	}).Error; err != nil {
		t.Fatalf("record historical predecessor: %v", err)
	}
	menu := presentationPermissionNode(t, db, pkg.MenuAccessType, presentationProfileMenuPath, "GET")
	seedPresentationPermissionRolePolicy(t, db, "historical-presentation-reader", menu)
	nodesBefore, policiesBefore := presentationPermissionState(t, db)

	if err := migratePresentationProfilePermissions(db, presentationProfilePermissionsMigrationID.String()); err == nil ||
		!strings.Contains(err.Error(), "cannot issue safe attestation") {
		t.Fatalf("direct predecessor retry error = %v, want attestation refusal", err)
	}
	assertPresentationPermissionVersion(t, db, presentationProfilePermissionsSafeAttestationVersion, 0)

	oldCalls := 0
	runner := newPresentationPermissionCollisionGuardRunner(
		t,
		db,
		func(*gorm.DB, string) error {
			oldCalls++
			return fmt.Errorf("recorded predecessor must not rerun")
		},
		_20260830193000PresentationProfilePermissionCollisionGuard,
	)
	err := runner.MigrateContext(context.Background())
	if err == nil || !strings.Contains(err.Error(), "required safe-code attestation") ||
		!strings.Contains(err.Error(), "canonical values alone cannot prove otherwise") {
		t.Fatalf("unattested migration error = %v, want explicit fail-closed recovery diagnostic", err)
	}
	if oldCalls != 0 {
		t.Fatalf("recorded predecessor reran %d times", oldCalls)
	}
	assertPresentationPermissionVersion(t, db, presentationProfilePermissionCollisionGuardMigrationID.String(), 0)
	nodesAfter, policiesAfter := presentationPermissionState(t, db)
	if !reflect.DeepEqual(nodesBefore, nodesAfter) {
		t.Fatalf("unattested guard changed permission nodes:\nbefore: %#v\nafter:  %#v", nodesBefore, nodesAfter)
	}
	if !reflect.DeepEqual(policiesBefore, policiesAfter) {
		t.Fatalf("unattested guard changed role policies:\nbefore: %#v\nafter:  %#v", policiesBefore, policiesAfter)
	}
}

func TestPresentationPermissionCollisionGuardFailsClosedOnAttestedInventoryDrift(t *testing.T) {
	tests := []struct {
		name      string
		kind      pkg.AccessType
		path      string
		method    string
		wantError string
		mutate    func(t *testing.T, db *gorm.DB, node *models.Menu)
	}{
		{
			name:      "mismatched-menu",
			kind:      pkg.MenuAccessType,
			path:      presentationProfileMenuPath,
			method:    "GET",
			wantError: "does not match managed metadata",
			mutate: func(t *testing.T, db *gorm.DB, node *models.Menu) {
				t.Helper()
				if err := db.Model(&models.Menu{}).Where("id = ?", node.ID).
					Update("name", "downstream.presentation").Error; err != nil {
					t.Fatalf("replace menu metadata: %v", err)
				}
			},
		},
		{
			name:      "disabled-component",
			kind:      pkg.ComponentAccessType,
			path:      "/presentation-config/publish",
			method:    "GET",
			wantError: "is disabled",
			mutate: func(t *testing.T, db *gorm.DB, node *models.Menu) {
				t.Helper()
				if err := db.Model(&models.Menu{}).Where("id = ?", node.ID).
					Update("status", enum.Disabled).Error; err != nil {
					t.Fatalf("disable component: %v", err)
				}
			},
		},
		{
			name:      "soft-deleted-api",
			kind:      pkg.APIAccessType,
			path:      "/admin/api/presentation-profiles/:id/publish",
			method:    "POST",
			wantError: "is soft-deleted",
			mutate: func(t *testing.T, db *gorm.DB, node *models.Menu) {
				t.Helper()
				if err := db.Session(&gorm.Session{SkipHooks: true}).Delete(node).Error; err != nil {
					t.Fatalf("soft-delete API: %v", err)
				}
			},
		},
		{
			name:      "duplicate-api",
			kind:      pkg.APIAccessType,
			path:      "/admin/api/presentation-profiles/:id/rollback",
			method:    "POST",
			wantError: "is occupied by 2 access nodes",
			mutate: func(t *testing.T, db *gorm.DB, node *models.Menu) {
				t.Helper()
				duplicate := *node
				duplicate.ID = "duplicate-guard-api"
				if err := db.Session(&gorm.Session{SkipHooks: true}).Create(&duplicate).Error; err != nil {
					t.Fatalf("duplicate API: %v", err)
				}
			},
		},
		{
			name:      "cross-type-component",
			kind:      pkg.ComponentAccessType,
			path:      "/presentation-config/rollback",
			method:    "GET",
			wantError: "does not match managed metadata",
			mutate: func(t *testing.T, db *gorm.DB, node *models.Menu) {
				t.Helper()
				if err := db.Model(&models.Menu{}).Where("id = ?", node.ID).
					Update("type", pkg.MenuAccessType).Error; err != nil {
					t.Fatalf("replace component access type: %v", err)
				}
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			db := setupPresentationPermissionMigrationTest(t, true)
			if err := migratePresentationProfilePermissions(db, presentationProfilePermissionsMigrationID.String()); err != nil {
				t.Fatalf("apply safe predecessor: %v", err)
			}
			node := presentationPermissionNode(t, db, test.kind, test.path, test.method)
			seedPresentationPermissionRolePolicy(t, db, "role-guard-"+test.name, node)
			test.mutate(t, db, node)
			nodesBefore, policiesBefore := presentationPermissionState(t, db)

			oldCalls := 0
			runner := newPresentationPermissionCollisionGuardRunner(
				t,
				db,
				func(*gorm.DB, string) error {
					oldCalls++
					return fmt.Errorf("recorded predecessor must not rerun")
				},
				_20260830193000PresentationProfilePermissionCollisionGuard,
			)
			err := runner.MigrateContext(context.Background())
			if err == nil || !strings.Contains(err.Error(), test.wantError) {
				t.Fatalf("guard error = %v, want diagnostic containing %q", err, test.wantError)
			}
			if oldCalls != 0 {
				t.Fatalf("recorded predecessor reran %d times", oldCalls)
			}
			assertPresentationPermissionVersion(t, db, presentationProfilePermissionCollisionGuardMigrationID.String(), 0)
			nodesAfter, policiesAfter := presentationPermissionState(t, db)
			if !reflect.DeepEqual(nodesBefore, nodesAfter) {
				t.Fatalf("failed guard changed nodes:\nbefore: %#v\nafter:  %#v", nodesBefore, nodesAfter)
			}
			if !reflect.DeepEqual(policiesBefore, policiesAfter) {
				t.Fatalf("failed guard changed policies:\nbefore: %#v\nafter:  %#v", policiesBefore, policiesAfter)
			}
		})
	}
}

func newPresentationPermissionCollisionGuardRunner(
	t *testing.T,
	db *gorm.DB,
	predecessor migration.MigrationFunc,
	guard migration.MigrationFunc,
) *migration.Migration {
	t.Helper()
	runner := migration.New()
	if err := runner.Register(presentationProfilePermissionsMigrationID, predecessor); err != nil {
		t.Fatalf("register presentation permission predecessor: %v", err)
	}
	if err := runner.Register(presentationProfilePermissionCollisionGuardMigrationID, guard); err != nil {
		t.Fatalf("register presentation permission collision guard: %v", err)
	}
	runner.SetDb(db)
	runner.SetModel(&migrationmodels.Migration{})
	return runner
}

func presentationPermissionMigrationRow(t *testing.T, db *gorm.DB, version string) migrationmodels.Migration {
	t.Helper()
	var row migrationmodels.Migration
	if err := db.Where("version = ?", version).Take(&row).Error; err != nil {
		t.Fatalf("read migration row %q: %v", version, err)
	}
	return row
}
