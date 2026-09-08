package litellmops

import (
	"context"
	"errors"
	"fmt"

	"github.com/mss-boot-io/mss-boot-admin/admin/business"
	"github.com/mss-boot-io/mss-boot-admin/admin/models"
	adminpkg "github.com/mss-boot-io/mss-boot-admin/admin/pkg"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/migration"
	"gorm.io/gorm"
)

// Migration identifiers, lossless and forward-only.
const (
	SnapshotMigrationID      migration.MigrationID = "20260908170000"
	AuthorizationMigrationID migration.MigrationID = "20260908170200"
)

// ApplySnapshotMigration applies the snapshot schema directly. It exists for
// probes and focused tests; production uses the registered migration runner.
func ApplySnapshotMigration(db *gorm.DB) error {
	return migrateSnapshots(db, SnapshotMigrationID.String())
}

// RegisterMigration registers both migration steps on the explicit runner.
func RegisterMigration(runner *migration.Migration) error {
	if runner == nil {
		return errors.New("litellmops migration runner is required")
	}
	if err := runner.Register(SnapshotMigrationID, migrateSnapshots); err != nil {
		return err
	}
	return runner.Register(AuthorizationMigrationID, migrateAuthorization)
}

var createUserSnapshotTableDDL = map[string]string{
	"sqlite": "CREATE TABLE IF NOT EXISTS \"litellmops_user_snapshot\" (\n" +
		"  \"id\" VARCHAR(64) NOT NULL PRIMARY KEY,\n" +
		"  \"created_at\" DATETIME NOT NULL,\n" +
		"  \"updated_at\" DATETIME NOT NULL,\n" +
		"  \"deleted_at\" DATETIME NULL,\n" +
		"  \"user_id\" VARCHAR(64) NOT NULL,\n" +
		"  \"email\" VARCHAR(254),\n" +
		"  \"user_role\" VARCHAR(64),\n" +
		"  \"models\" TEXT,\n" +
		"  \"max_budget\" REAL NULL,\n" +
		"  \"budget_duration\" VARCHAR(16) NULL,\n" +
		"  \"budget_reset_at\" DATETIME NULL,\n" +
		"  \"spend\" REAL NOT NULL DEFAULT 0,\n" +
		"  \"synced_at\" DATETIME NOT NULL\n" +
		")",
	"postgres": "CREATE TABLE IF NOT EXISTS \"litellmops_user_snapshot\" (\n" +
		"  \"id\" VARCHAR(64) NOT NULL PRIMARY KEY,\n" +
		"  \"created_at\" TIMESTAMPTZ NOT NULL,\n" +
		"  \"updated_at\" TIMESTAMPTZ NOT NULL,\n" +
		"  \"deleted_at\" TIMESTAMPTZ NULL,\n" +
		"  \"user_id\" VARCHAR(64) NOT NULL,\n" +
		"  \"email\" VARCHAR(254),\n" +
		"  \"user_role\" VARCHAR(64),\n" +
		"  \"models\" TEXT,\n" +
		"  \"max_budget\" DOUBLE PRECISION NULL,\n" +
		"  \"budget_duration\" VARCHAR(16) NULL,\n" +
		"  \"budget_reset_at\" TIMESTAMPTZ NULL,\n" +
		"  \"spend\" DOUBLE PRECISION NOT NULL DEFAULT 0,\n" +
		"  \"synced_at\" TIMESTAMPTZ NOT NULL\n" +
		")",
	"mysql": "CREATE TABLE IF NOT EXISTS `litellmops_user_snapshot` (\n" +
		"  `id` VARCHAR(64) COLLATE utf8mb4_bin NOT NULL PRIMARY KEY,\n" +
		"  `created_at` DATETIME(3) NOT NULL,\n" +
		"  `updated_at` DATETIME(3) NOT NULL,\n" +
		"  `deleted_at` DATETIME(3) NULL,\n" +
		"  `user_id` VARCHAR(64) COLLATE utf8mb4_bin NOT NULL,\n" +
		"  `email` VARCHAR(254) COLLATE utf8mb4_bin,\n" +
		"  `user_role` VARCHAR(64) COLLATE utf8mb4_bin,\n" +
		"  `models` TEXT,\n" +
		"  `max_budget` DOUBLE NULL,\n" +
		"  `budget_duration` VARCHAR(16) COLLATE utf8mb4_bin NULL,\n" +
		"  `budget_reset_at` DATETIME(3) NULL,\n" +
		"  `spend` DOUBLE NOT NULL DEFAULT 0,\n" +
		"  `synced_at` DATETIME(3) NOT NULL\n" +
		") ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin",
}

var createKeySnapshotTableDDL = map[string]string{
	"sqlite": "CREATE TABLE IF NOT EXISTS \"litellmops_key_snapshot\" (\n" +
		"  \"id\" VARCHAR(64) NOT NULL PRIMARY KEY,\n" +
		"  \"created_at\" DATETIME NOT NULL,\n" +
		"  \"updated_at\" DATETIME NOT NULL,\n" +
		"  \"deleted_at\" DATETIME NULL,\n" +
		"  \"key_hash_prefix\" VARCHAR(32) NOT NULL,\n" +
		"  \"alias\" VARCHAR(128) NULL,\n" +
		"  \"user_id\" VARCHAR(64) NOT NULL,\n" +
		"  \"user_email\" VARCHAR(254),\n" +
		"  \"max_budget\" REAL NULL,\n" +
		"  \"spend\" REAL NOT NULL DEFAULT 0,\n" +
		"  \"tpm_limit\" BIGINT NULL,\n" +
		"  \"rpm_limit\" BIGINT NULL,\n" +
		"  \"max_parallel_requests\" INTEGER NULL,\n" +
		"  \"expires\" DATETIME NULL,\n" +
		"  \"is_session_key\" BOOLEAN NOT NULL DEFAULT 0,\n" +
		"  \"synced_at\" DATETIME NOT NULL\n" +
		")",
	"postgres": "CREATE TABLE IF NOT EXISTS \"litellmops_key_snapshot\" (\n" +
		"  \"id\" VARCHAR(64) NOT NULL PRIMARY KEY,\n" +
		"  \"created_at\" TIMESTAMPTZ NOT NULL,\n" +
		"  \"updated_at\" TIMESTAMPTZ NOT NULL,\n" +
		"  \"deleted_at\" TIMESTAMPTZ NULL,\n" +
		"  \"key_hash_prefix\" VARCHAR(32) NOT NULL,\n" +
		"  \"alias\" VARCHAR(128) NULL,\n" +
		"  \"user_id\" VARCHAR(64) NOT NULL,\n" +
		"  \"user_email\" VARCHAR(254),\n" +
		"  \"max_budget\" DOUBLE PRECISION NULL,\n" +
		"  \"spend\" DOUBLE PRECISION NOT NULL DEFAULT 0,\n" +
		"  \"tpm_limit\" BIGINT NULL,\n" +
		"  \"rpm_limit\" BIGINT NULL,\n" +
		"  \"max_parallel_requests\" INTEGER NULL,\n" +
		"  \"expires\" TIMESTAMPTZ NULL,\n" +
		"  \"is_session_key\" BOOLEAN NOT NULL DEFAULT FALSE,\n" +
		"  \"synced_at\" TIMESTAMPTZ NOT NULL\n" +
		")",
	"mysql": "CREATE TABLE IF NOT EXISTS `litellmops_key_snapshot` (\n" +
		"  `id` VARCHAR(64) COLLATE utf8mb4_bin NOT NULL PRIMARY KEY,\n" +
		"  `created_at` DATETIME(3) NOT NULL,\n" +
		"  `updated_at` DATETIME(3) NOT NULL,\n" +
		"  `deleted_at` DATETIME(3) NULL,\n" +
		"  `key_hash_prefix` VARCHAR(32) COLLATE utf8mb4_bin NOT NULL,\n" +
		"  `alias` VARCHAR(128) COLLATE utf8mb4_bin NULL,\n" +
		"  `user_id` VARCHAR(64) COLLATE utf8mb4_bin NOT NULL,\n" +
		"  `user_email` VARCHAR(254) COLLATE utf8mb4_bin,\n" +
		"  `max_budget` DOUBLE NULL,\n" +
		"  `spend` DOUBLE NOT NULL DEFAULT 0,\n" +
		"  `tpm_limit` BIGINT NULL,\n" +
		"  `rpm_limit` BIGINT NULL,\n" +
		"  `max_parallel_requests` INTEGER NULL,\n" +
		"  `expires` DATETIME(3) NULL,\n" +
		"  `is_session_key` BOOLEAN NOT NULL DEFAULT FALSE,\n" +
		"  `synced_at` DATETIME(3) NOT NULL\n" +
		") ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin",
}

type indexSpec struct {
	name    string
	table   string
	unique  bool
	columns []string
}

var snapshotIndexes = []indexSpec{
	{name: "ux_litellmops_user_snapshot_user_id", table: "litellmops_user_snapshot", unique: true, columns: []string{"user_id"}},
	{name: "idx_litellmops_user_snapshot_email", table: "litellmops_user_snapshot", columns: []string{"email"}},
	{name: "idx_litellmops_user_snapshot_synced_at", table: "litellmops_user_snapshot", columns: []string{"synced_at"}},
	{name: "ux_litellmops_key_snapshot_key_hash_prefix", table: "litellmops_key_snapshot", unique: true, columns: []string{"key_hash_prefix"}},
	{name: "idx_litellmops_key_snapshot_user_id", table: "litellmops_key_snapshot", columns: []string{"user_id"}},
	{name: "idx_litellmops_key_snapshot_is_session_key", table: "litellmops_key_snapshot", columns: []string{"is_session_key"}},
}

func migrateSnapshots(db *gorm.DB, version string) error {
	if db == nil {
		return errors.New("litellmops migration database is required")
	}
	if version != SnapshotMigrationID.String() {
		return errors.New("litellmops snapshot migration version mismatch")
	}
	dialect := db.Dialector.Name()
	userDDL, supported := createUserSnapshotTableDDL[dialect]
	if !supported {
		return fmt.Errorf("litellmops migration: unsupported database dialect %q", dialect)
	}
	keyDDL := createKeySnapshotTableDDL[dialect]
	if !db.Migrator().HasTable(new(UserSnapshot)) {
		if err := db.Exec(userDDL).Error; err != nil {
			return fmt.Errorf("litellmops migration: create user snapshot table: %w", err)
		}
	}
	if !db.Migrator().HasTable(new(KeySnapshot)) {
		if err := db.Exec(keyDDL).Error; err != nil {
			return fmt.Errorf("litellmops migration: create key snapshot table: %w", err)
		}
	}
	for _, index := range snapshotIndexes {
		if err := createIndexIfMissing(db, index); err != nil {
			return err
		}
	}
	return nil
}

// createIndexIfMissing applies additive indexes without relying on
// dialect-specific "IF NOT EXISTS" support (MySQL lacks it).
func createIndexIfMissing(db *gorm.DB, index indexSpec) error {
	if db.Migrator().HasIndex(index.table, index.name) {
		return nil
	}
	quote := "`"
	if db.Dialector.Name() != "mysql" {
		quote = `"`
	}
	columns := ""
	for i, column := range index.columns {
		if i > 0 {
			columns += ", "
		}
		columns += quote + column + quote
	}
	kind := "INDEX"
	if index.unique {
		kind = "UNIQUE INDEX"
	}
	ddl := fmt.Sprintf("CREATE %s %s%s%s ON %s%s%s (%s)",
		kind, quote, index.name, quote, quote, index.table, quote, columns)
	if err := db.Exec(ddl).Error; err != nil {
		return fmt.Errorf("litellmops migration: create index %s: %w", index.name, err)
	}
	return nil
}

// authorizationRouteSeed binds one protected route to the roles allowed by
// the module descriptor. The casbin authorizer enforces exactly these rows.
type authorizationRouteSeed struct {
	permission string
	method     string
	path       string
	roles      []string
}

var authorizationRouteSeeds = []authorizationRouteSeed{
	{permission: PermissionUserList, method: "GET", path: "/admin/api/litellmops/users", roles: []string{"admin", "litellmops-finance", "litellmops-readonly"}},
	{permission: PermissionUserRead, method: "GET", path: "/admin/api/litellmops/users/:id", roles: []string{"admin", "litellmops-finance", "litellmops-readonly"}},
	{permission: PermissionKeyList, method: "GET", path: "/admin/api/litellmops/keys", roles: []string{"admin", "litellmops-finance", "litellmops-readonly"}},
	{permission: PermissionKeyRead, method: "GET", path: "/admin/api/litellmops/keys/:id", roles: []string{"admin", "litellmops-finance", "litellmops-readonly"}},
	{permission: PermissionSync, method: "POST", path: "/admin/api/litellmops/sync", roles: []string{"admin", "litellmops-finance"}},
	{permission: PermissionBills, method: "GET", path: "/admin/api/litellmops/bills", roles: []string{"admin", "litellmops-finance", "litellmops-readonly"}},
}

func migrateAuthorization(db *gorm.DB, version string) error {
	if db == nil {
		return errors.New("litellmops authorization migration database is required")
	}
	if version != AuthorizationMigrationID.String() {
		return errors.New("litellmops authorization migration version mismatch")
	}
	for _, seed := range authorizationRouteSeeds {
		for _, role := range seed.roles {
			rule := models.CasbinRule{
				PType: "p",
				V0:    role,
				V1:    adminpkg.APIAccessType.String(),
				V2:    seed.path,
				V3:    seed.method,
			}
			var count int64
			if err := db.Model(&models.CasbinRule{}).
				Where("ptype = ? AND v0 = ? AND v1 = ? AND v2 = ? AND v3 = ?",
					rule.PType, rule.V0, rule.V1, rule.V2, rule.V3).
				Count(&count).Error; err != nil {
				return fmt.Errorf("litellmops authorization migration: read policy: %w", err)
			}
			if count > 0 {
				continue
			}
			if err := db.Create(&rule).Error; err != nil {
				return fmt.Errorf("litellmops authorization migration: seed policy: %w", err)
			}
		}
	}
	return nil
}

// verifyRuntimeReadiness proves migrations and policy storage before routes mount.
func verifyRuntimeReadiness(ctx context.Context, db *gorm.DB) error {
	if ctx == nil {
		return errors.New("litellmops schema readiness context is required")
	}
	if db == nil {
		return errors.New("litellmops schema readiness database is required")
	}
	if err := business.RequireAppliedMigrations(ctx, db, SnapshotMigrationID, AuthorizationMigrationID); err != nil {
		return fmt.Errorf("litellmops migration readiness failed: %w", err)
	}
	readyDB := db.WithContext(ctx)
	if !readyDB.Migrator().HasTable(new(UserSnapshot)) {
		return errors.New("litellmops migration readiness failed: user snapshot table is unavailable")
	}
	if !readyDB.Migrator().HasTable(new(KeySnapshot)) {
		return errors.New("litellmops migration readiness failed: key snapshot table is unavailable")
	}
	if !readyDB.Migrator().HasTable(new(models.CasbinRule)) {
		return errors.New("litellmops authorization readiness failed: Admin policy table is unavailable")
	}
	return nil
}
