package litellmops

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/mss-boot-io/mss-boot-admin/admin/business"
	"github.com/mss-boot-io/mss-boot-admin/admin/models"
	adminpkg "github.com/mss-boot-io/mss-boot-admin/admin/pkg"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/migration"
	migrationmodels "github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/migration/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Migration identifiers, lossless and forward-only.
const (
	SnapshotMigrationID        migration.MigrationID = "20260908170000"
	AuthorizationMigrationID   migration.MigrationID = "20260908170200"
	RechargeMigrationID        migration.MigrationID = "20260909180000"
	OrganizationMigrationID    migration.MigrationID = "20260909210000"
	OperationsMigrationID      migration.MigrationID = "20260910150000"
	ManagementFenceMigrationID migration.MigrationID = "20260910170000"
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
	if err := runner.Register(AuthorizationMigrationID, migrateAuthorization); err != nil {
		return err
	}
	if err := runner.Register(RechargeMigrationID, migrateRecharge); err != nil {
		return err
	}
	if err := runner.Register(OrganizationMigrationID, migrateOrganizations); err != nil {
		return err
	}
	if err := runner.Register(OperationsMigrationID, migrateOperations); err != nil {
		return err
	}
	return runner.Register(ManagementFenceMigrationID, migrateManagementFence)
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
	return recordMigrationVersion(db, version)
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
	return recordMigrationVersion(db, version)
}

var createRechargeTableDDL = map[string]string{
	"sqlite": "CREATE TABLE IF NOT EXISTS \"litellmops_recharge\" (\n" +
		"  \"id\" VARCHAR(64) NOT NULL PRIMARY KEY,\n" +
		"  \"created_at\" DATETIME NOT NULL,\n" +
		"  \"user_id\" VARCHAR(64) NOT NULL,\n" +
		"  \"email\" VARCHAR(254),\n" +
		"  \"amount\" REAL NOT NULL,\n" +
		"  \"before_budget\" REAL NOT NULL,\n" +
		"  \"after_budget\" REAL NOT NULL,\n" +
		"  \"raise_keys\" BOOLEAN NOT NULL DEFAULT 0,\n" +
		"  \"keys_updated\" TEXT,\n" +
		"  \"operator\" VARCHAR(128) NOT NULL,\n" +
		"  \"reason\" VARCHAR(512),\n" +
		"  \"idempotency_key\" VARCHAR(64),\n" +
		"  \"status\" VARCHAR(16) NOT NULL\n" +
		")",
	"postgres": "CREATE TABLE IF NOT EXISTS \"litellmops_recharge\" (\n" +
		"  \"id\" VARCHAR(64) NOT NULL PRIMARY KEY,\n" +
		"  \"created_at\" TIMESTAMPTZ NOT NULL,\n" +
		"  \"user_id\" VARCHAR(64) NOT NULL,\n" +
		"  \"email\" VARCHAR(254),\n" +
		"  \"amount\" DOUBLE PRECISION NOT NULL,\n" +
		"  \"before_budget\" DOUBLE PRECISION NOT NULL,\n" +
		"  \"after_budget\" DOUBLE PRECISION NOT NULL,\n" +
		"  \"raise_keys\" BOOLEAN NOT NULL DEFAULT FALSE,\n" +
		"  \"keys_updated\" TEXT,\n" +
		"  \"operator\" VARCHAR(128) NOT NULL,\n" +
		"  \"reason\" VARCHAR(512),\n" +
		"  \"idempotency_key\" VARCHAR(64),\n" +
		"  \"status\" VARCHAR(16) NOT NULL\n" +
		")",
	"mysql": "CREATE TABLE IF NOT EXISTS `litellmops_recharge` (\n" +
		"  `id` VARCHAR(64) COLLATE utf8mb4_bin NOT NULL PRIMARY KEY,\n" +
		"  `created_at` DATETIME(3) NOT NULL,\n" +
		"  `user_id` VARCHAR(64) COLLATE utf8mb4_bin NOT NULL,\n" +
		"  `email` VARCHAR(254) COLLATE utf8mb4_bin,\n" +
		"  `amount` DOUBLE NOT NULL,\n" +
		"  `before_budget` DOUBLE NOT NULL,\n" +
		"  `after_budget` DOUBLE NOT NULL,\n" +
		"  `raise_keys` BOOLEAN NOT NULL DEFAULT FALSE,\n" +
		"  `keys_updated` TEXT,\n" +
		"  `operator` VARCHAR(128) COLLATE utf8mb4_bin NOT NULL,\n" +
		"  `reason` VARCHAR(512) COLLATE utf8mb4_bin,\n" +
		"  `idempotency_key` VARCHAR(64) COLLATE utf8mb4_bin,\n" +
		"  `status` VARCHAR(16) COLLATE utf8mb4_bin NOT NULL\n" +
		") ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin",
}

func migrateRecharge(db *gorm.DB, version string) error {
	if db == nil {
		return errors.New("litellmops recharge migration database is required")
	}
	if version != RechargeMigrationID.String() {
		return errors.New("litellmops recharge migration version mismatch")
	}
	dialect := db.Dialector.Name()
	ddl, supported := createRechargeTableDDL[dialect]
	if !supported {
		return fmt.Errorf("litellmops recharge migration: unsupported database dialect %q", dialect)
	}
	if !db.Migrator().HasTable(new(RechargeRecord)) {
		if err := db.Exec(ddl).Error; err != nil {
			return fmt.Errorf("litellmops recharge migration: create table: %w", err)
		}
	}
	if err := createIndexIfMissing(db, indexSpec{
		name:    "idx_litellmops_recharge_user_created",
		table:   "litellmops_recharge",
		columns: []string{"user_id", "created_at"},
	}); err != nil {
		return err
	}
	if err := createIndexIfMissing(db, indexSpec{
		name:    "ux_litellmops_recharge_idempotency",
		table:   "litellmops_recharge",
		unique:  true,
		columns: []string{"user_id", "idempotency_key"},
	}); err != nil {
		return err
	}
	seed := authorizationRouteSeed{
		permission: PermissionRecharge,
		method:     "POST",
		path:       "/admin/api/litellmops/users/:id/recharge",
		roles:      []string{"admin", "litellmops-finance"},
	}
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
			return fmt.Errorf("litellmops recharge migration: read policy: %w", err)
		}
		if count > 0 {
			continue
		}
		if err := db.Create(&rule).Error; err != nil {
			return fmt.Errorf("litellmops recharge migration: seed policy: %w", err)
		}
	}
	return recordMigrationVersion(db, version)
}

// recordMigrationVersion marks the migration applied, mirroring the generated
// module pattern (conflict-safe ledger insert).
func recordMigrationVersion(db *gorm.DB, version string) error {
	versionRow := &migrationmodels.Migration{}
	versionRow.SetVersion(version)
	if err := db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "version"}},
		DoNothing: true,
	}).Create(versionRow).Error; err != nil {
		return errors.New("litellmops migration: record version failed")
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
	if err := business.RequireAppliedMigrations(ctx, db, SnapshotMigrationID, AuthorizationMigrationID, RechargeMigrationID, OrganizationMigrationID, OperationsMigrationID, ManagementFenceMigrationID); err != nil {
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
	if !readyDB.Migrator().HasTable(new(RechargeRecord)) {
		return errors.New("litellmops migration readiness failed: recharge table is unavailable")
	}
	if !readyDB.Migrator().HasTable(new(OrgAuditRecord)) {
		return errors.New("litellmops migration readiness failed: organization audit table is unavailable")
	}
	for _, model := range []any{new(UserLease), new(OperationAttempt), new(SalesProduct), new(SalesOrder), new(ManagementCommand)} {
		if !readyDB.Migrator().HasTable(model) {
			return errors.New("litellmops migration readiness failed: operations table is unavailable")
		}
	}
	for _, field := range []string{"LastObservedKeyPrefix", "LastObservedKeyUSDMicro", "LastObservedKeyAt"} {
		if !readyDB.Migrator().HasColumn(new(RechargeRecord), field) {
			return errors.New("litellmops migration readiness failed: recharge reconciliation schema is unavailable")
		}
	}
	return nil
}

func migrateOrganizations(db *gorm.DB, version string) error {
	if db == nil {
		return errors.New("litellmops organization migration database is required")
	}
	if version != OrganizationMigrationID.String() {
		return errors.New("litellmops organization migration version mismatch")
	}
	if err := db.AutoMigrate(&OrgAuditRecord{}); err != nil {
		return fmt.Errorf("litellmops organization migration: audit table: %w", err)
	}
	seeds := []authorizationRouteSeed{
		{permission: PermissionOrgList, method: "GET", path: "/admin/api/litellmops/organizations", roles: []string{"admin", "litellmops-finance", "litellmops-readonly"}},
		{permission: PermissionOrgRead, method: "GET", path: "/admin/api/litellmops/organizations/:id", roles: []string{"admin", "litellmops-finance", "litellmops-readonly"}},
		{permission: PermissionUserRead, method: "GET", path: "/admin/api/litellmops/users/:id/recharges", roles: []string{"admin", "litellmops-finance", "litellmops-readonly"}},
		{permission: PermissionOrgWrite, method: "POST", path: "/admin/api/litellmops/organizations", roles: []string{"admin", "litellmops-finance"}},
		{permission: PermissionOrgWrite, method: "PATCH", path: "/admin/api/litellmops/organizations/:id", roles: []string{"admin", "litellmops-finance"}},
		{permission: PermissionOrgWrite, method: "DELETE", path: "/admin/api/litellmops/organizations/:id", roles: []string{"admin", "litellmops-finance"}},
		{permission: PermissionOrgWrite, method: "POST", path: "/admin/api/litellmops/organizations/:id/recharge", roles: []string{"admin", "litellmops-finance"}},
		{permission: PermissionOrgWrite, method: "POST", path: "/admin/api/litellmops/organizations/:id/members", roles: []string{"admin", "litellmops-finance"}},
		{permission: PermissionOrgWrite, method: "POST", path: "/admin/api/litellmops/organizations/:id/members/remove", roles: []string{"admin", "litellmops-finance"}},
		{permission: PermissionOrgWrite, method: "POST", path: "/admin/api/litellmops/organizations/:id/teams", roles: []string{"admin", "litellmops-finance"}},
		{permission: PermissionOrgWrite, method: "POST", path: "/admin/api/litellmops/teams/:teamId/recharge", roles: []string{"admin", "litellmops-finance"}},
		{permission: PermissionOrgWrite, method: "DELETE", path: "/admin/api/litellmops/teams/:teamId", roles: []string{"admin", "litellmops-finance"}},
	}
	for _, seed := range seeds {
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
				return fmt.Errorf("litellmops organization migration: read policy: %w", err)
			}
			if count > 0 {
				continue
			}
			if err := db.Create(&rule).Error; err != nil {
				return fmt.Errorf("litellmops organization migration: seed policy: %w", err)
			}
		}
	}
	if db.Migrator().HasTable("mss_boot_menus") {
		var parentID string
		_ = db.Raw("SELECT id FROM mss_boot_menus WHERE path = ? LIMIT 1", "/litellm-ops").Scan(&parentID).Error
		if parentID != "" {
			var existing int64
			_ = db.Raw("SELECT COUNT(*) FROM mss_boot_menus WHERE path = ?", "/litellm-ops/orgs").Scan(&existing).Error
			if existing == 0 {
				if err := db.Exec(
					`INSERT INTO mss_boot_menus (id, created_at, updated_at, parent_id, name, path, method, component, icon, target, header_render, footer_render, menu_render, menu_header_render, hide_children_in_menu, hide_in_menu, hide_in_breadcrumb, flat_menu, fixed_header, fix_siderbar, nav_theme, layout, header_theme, type, permission, status, sort) VALUES (?, ?, ?, ?, ?, ?, 'GET', ?, 'apartment', '', 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, '', '', '', 'MENU', ?, 'enabled', 40)`,
					newSnapshotID(), time.Now().UTC(), time.Now().UTC(), parentID, "menu.llops.orgs", "/litellm-ops/orgs", "./LitellmOps/Orgs", "/litellmops/orgs",
				).Error; err != nil {
					return fmt.Errorf("litellmops organization migration: menu: %w", err)
				}
			}
		}
	}
	return recordMigrationVersion(db, version)
}

var operationsAuthorizationSeeds = []authorizationRouteSeed{
	{permission: PermissionProductRead, method: "GET", path: "/admin/api/litellmops/sales/products", roles: []string{"admin", "litellmops-finance", "litellmops-readonly"}},
	{permission: PermissionProductWrite, method: "POST", path: "/admin/api/litellmops/sales/products", roles: []string{"admin", "litellmops-finance"}},
	{permission: PermissionProductWrite, method: "PATCH", path: "/admin/api/litellmops/sales/products/:id", roles: []string{"admin", "litellmops-finance"}},
	{permission: PermissionOrderRead, method: "GET", path: "/admin/api/litellmops/sales/orders", roles: []string{"admin", "litellmops-finance", "litellmops-readonly"}},
	{permission: PermissionOrderRead, method: "GET", path: "/admin/api/litellmops/sales/orders/:id", roles: []string{"admin", "litellmops-finance", "litellmops-readonly"}},
	{permission: PermissionOrderImport, method: "POST", path: "/admin/api/litellmops/sales/orders", roles: []string{"admin", "litellmops-finance"}},
	{permission: PermissionOrderImport, method: "POST", path: "/admin/api/litellmops/sales/orders/import", roles: []string{"admin", "litellmops-finance"}},
	{permission: PermissionOrderVerify, method: "POST", path: "/admin/api/litellmops/sales/orders/:id/verify", roles: []string{"admin", "litellmops-finance"}},
	{permission: PermissionOrderVerify, method: "POST", path: "/admin/api/litellmops/sales/orders/:id/match", roles: []string{"admin", "litellmops-finance"}},
	{permission: PermissionOrderApprove, method: "POST", path: "/admin/api/litellmops/sales/orders/:id/approve", roles: []string{"admin", "litellmops-finance"}},
	{permission: PermissionOrderExecute, method: "POST", path: "/admin/api/litellmops/sales/orders/:id/execute", roles: []string{"admin", "litellmops-finance"}},
	{permission: PermissionOrderReconcile, method: "POST", path: "/admin/api/litellmops/sales/orders/:id/reconcile", roles: []string{"admin", "litellmops-finance"}},
	{permission: PermissionOrderRefundReview, method: "POST", path: "/admin/api/litellmops/sales/orders/:id/refund-review", roles: []string{"admin", "litellmops-finance"}},
	{permission: PermissionUserWrite, method: "POST", path: "/admin/api/litellmops/users", roles: []string{"admin"}},
	{permission: PermissionUserWrite, method: "POST", path: "/admin/api/litellmops/users/invite", roles: []string{"admin"}},
	{permission: PermissionUserWrite, method: "PATCH", path: "/admin/api/litellmops/users/:id", roles: []string{"admin"}},
	{permission: PermissionUserWrite, method: "POST", path: "/admin/api/litellmops/users/:id/block", roles: []string{"admin"}},
	{permission: PermissionUserWrite, method: "POST", path: "/admin/api/litellmops/users/:id/unblock", roles: []string{"admin"}},
	{permission: PermissionUserWrite, method: "DELETE", path: "/admin/api/litellmops/users/:id", roles: []string{"admin"}},
	{permission: PermissionKeyIssue, method: "POST", path: "/admin/api/litellmops/keys", roles: []string{"admin"}},
	{permission: PermissionKeyIssue, method: "POST", path: "/admin/api/litellmops/keys/:id/rotate", roles: []string{"admin"}},
	{permission: PermissionKeyWrite, method: "PATCH", path: "/admin/api/litellmops/keys/:id", roles: []string{"admin"}},
	{permission: PermissionKeyWrite, method: "POST", path: "/admin/api/litellmops/keys/:id/reset-spend", roles: []string{"admin"}},
	{permission: PermissionKeyRevoke, method: "POST", path: "/admin/api/litellmops/keys/:id/block", roles: []string{"admin"}},
	{permission: PermissionKeyRevoke, method: "POST", path: "/admin/api/litellmops/keys/:id/unblock", roles: []string{"admin"}},
	{permission: PermissionKeyRevoke, method: "DELETE", path: "/admin/api/litellmops/keys/:id", roles: []string{"admin"}},
	{permission: PermissionGatewayRead, method: "GET", path: "/admin/api/litellmops/gateway/models", roles: []string{"admin", "litellmops-finance", "litellmops-readonly"}},
	{permission: PermissionGatewayRead, method: "GET", path: "/admin/api/litellmops/gateway/health", roles: []string{"admin", "litellmops-finance", "litellmops-readonly"}},
	{permission: PermissionGatewayWrite, method: "POST", path: "/admin/api/litellmops/gateway/models/:id/block", roles: []string{"admin"}},
	{permission: PermissionGatewayWrite, method: "POST", path: "/admin/api/litellmops/gateway/models/:id/unblock", roles: []string{"admin"}},
}

// migrateOperations is additive and portable across the three supported GORM
// dialects. It upgrades legacy recharge rows and creates the sales state store.
func migrateOperations(db *gorm.DB, version string) error {
	if db == nil {
		return errors.New("litellmops operations migration database is required")
	}
	if version != OperationsMigrationID.String() {
		return errors.New("litellmops operations migration version mismatch")
	}
	return db.Transaction(func(tx *gorm.DB) error {
		if err := upgradeLegacyRecharge(tx); err != nil {
			return err
		}
		if err := tx.AutoMigrate(&UserSnapshot{}, &KeySnapshot{}, &RechargeRecord{}, &UserLease{}, &OperationAttempt{}, &SalesProduct{}, &SalesOrder{}); err != nil {
			return fmt.Errorf("litellmops operations migration: schema: %w", err)
		}
		for _, seed := range operationsAuthorizationSeeds {
			for _, role := range seed.roles {
				rule := models.CasbinRule{PType: "p", V0: role, V1: adminpkg.APIAccessType.String(), V2: seed.path, V3: seed.method}
				var count int64
				if err := tx.Model(&models.CasbinRule{}).Where(
					"ptype = ? AND v0 = ? AND v1 = ? AND v2 = ? AND v3 = ?", rule.PType, rule.V0, rule.V1, rule.V2, rule.V3,
				).Count(&count).Error; err != nil {
					return fmt.Errorf("litellmops operations migration: read policy: %w", err)
				}
				if count == 0 {
					if err := tx.Create(&rule).Error; err != nil {
						return fmt.Errorf("litellmops operations migration: seed policy: %w", err)
					}
				}
			}
		}
		return recordMigrationVersion(tx, version)
	})
}

var managementFenceAuthorizationSeeds = []authorizationRouteSeed{
	{permission: PermissionRecharge, method: "POST", path: "/admin/api/litellmops/recharges/:id/reconcile", roles: []string{"admin", "litellmops-finance"}},
	{permission: PermissionManagementResolve, method: "GET", path: "/admin/api/litellmops/management/commands", roles: []string{"admin"}},
	{permission: PermissionManagementResolve, method: "POST", path: "/admin/api/litellmops/management/commands/:id/reconcile", roles: []string{"admin"}},
	{permission: PermissionManagementResolve, method: "POST", path: "/admin/api/litellmops/management/commands/:id/resolve", roles: []string{"admin"}},
}

func migrateManagementFence(db *gorm.DB, version string) error {
	if db == nil {
		return errors.New("litellmops management fence migration database is required")
	}
	if version != ManagementFenceMigrationID.String() {
		return errors.New("litellmops management fence migration version mismatch")
	}
	return db.Transaction(func(tx *gorm.DB) error {
		if err := tx.AutoMigrate(&RechargeRecord{}, &ManagementCommand{}); err != nil {
			return fmt.Errorf("litellmops management fence migration: schema: %w", err)
		}
		for _, seed := range managementFenceAuthorizationSeeds {
			for _, role := range seed.roles {
				rule := models.CasbinRule{PType: "p", V0: role, V1: adminpkg.APIAccessType.String(), V2: seed.path, V3: seed.method}
				var count int64
				if err := tx.Model(&models.CasbinRule{}).Where(
					"ptype = ? AND v0 = ? AND v1 = ? AND v2 = ? AND v3 = ?", rule.PType, rule.V0, rule.V1, rule.V2, rule.V3,
				).Count(&count).Error; err != nil {
					return fmt.Errorf("litellmops management fence migration: read policy: %w", err)
				}
				if count == 0 {
					if err := tx.Create(&rule).Error; err != nil {
						return fmt.Errorf("litellmops management fence migration: seed policy: %w", err)
					}
				}
			}
		}
		return recordMigrationVersion(tx, version)
	})
}

type legacyRechargeRow struct {
	ID             string
	CreatedAt      time.Time
	UserID         string
	Amount         float64
	BeforeBudget   float64
	AfterBudget    float64
	RaiseKeys      bool
	IdempotencyKey *string
	Status         string
}

// upgradeLegacyRecharge performs the data phase before GORM reifies the final
// constraints. This is required for a live legacy table whose idempotency key
// is nullable and whose completed rows predate integer amounts/payload hashes.
func upgradeLegacyRecharge(db *gorm.DB) error {
	if !db.Migrator().HasTable(&RechargeRecord{}) {
		return nil
	}
	for _, field := range []string{
		"UpdatedAt", "CompletedAt", "AmountUSDMicro", "BeforeBudgetUSDMicro", "TargetAfterUSDMicro",
		"PayloadHash", "Source", "SourceRef", "LastErrorCode", "UncertainSince", "LastObservedUSDMicro",
		"LastObservedAt", "Version",
	} {
		if db.Migrator().HasColumn(&RechargeRecord{}, field) {
			continue
		}
		if err := db.Migrator().AddColumn(&RechargeRecord{}, field); err != nil {
			return fmt.Errorf("litellmops operations migration: add recharge field %s: %w", field, err)
		}
	}
	rows := make([]legacyRechargeRow, 0)
	if err := db.Table((&RechargeRecord{}).TableName()).Select(
		"id, created_at, user_id, amount, before_budget, after_budget, raise_keys, idempotency_key, status",
	).Where("payload_hash IS NULL OR payload_hash = ? OR version IS NULL OR version = 0", "").Find(&rows).Error; err != nil {
		return fmt.Errorf("litellmops operations migration: read legacy recharge rows: %w", err)
	}
	for _, row := range rows {
		amount, err := usdToMicro(row.Amount)
		if err != nil {
			return fmt.Errorf("litellmops operations migration: invalid legacy amount for row %s", row.ID)
		}
		before, err := usdToMicro(row.BeforeBudget)
		if err != nil {
			return fmt.Errorf("litellmops operations migration: invalid legacy before budget for row %s", row.ID)
		}
		target, err := usdToMicro(row.AfterBudget)
		if err != nil {
			return fmt.Errorf("litellmops operations migration: invalid legacy target budget for row %s", row.ID)
		}
		idempotency := ""
		if row.IdempotencyKey != nil {
			idempotency = strings.TrimSpace(*row.IdempotencyKey)
		}
		if idempotency == "" {
			idempotency = "legacy:" + row.ID
		}
		values := map[string]any{
			"updated_at": row.CreatedAt, "amount_usd_micro": amount,
			"before_budget_usd_micro": before, "target_after_usd_micro": target,
			"idempotency_key": idempotency, "payload_hash": rechargePayloadHash(row.UserID, amount, row.RaiseKeys),
			"source": "legacy", "source_ref": row.ID, "version": 1,
		}
		if row.Status == RechargeCompleted {
			values["completed_at"] = row.CreatedAt
		}
		if err := db.Table((&RechargeRecord{}).TableName()).Where("id = ?", row.ID).Updates(values).Error; err != nil {
			return fmt.Errorf("litellmops operations migration: backfill legacy recharge row %s: %w", row.ID, err)
		}
	}
	type duplicate struct {
		UserID         string
		IdempotencyKey string
		Count          int64
	}
	var duplicates []duplicate
	if err := db.Table((&RechargeRecord{}).TableName()).Select("user_id, idempotency_key, COUNT(*) AS count").Group("user_id, idempotency_key").Having("COUNT(*) > 1").Find(&duplicates).Error; err != nil {
		return fmt.Errorf("litellmops operations migration: check recharge conflicts: %w", err)
	}
	if len(duplicates) != 0 {
		return errors.New("litellmops operations migration: duplicate legacy recharge idempotency keys require manual resolution")
	}
	return nil
}
