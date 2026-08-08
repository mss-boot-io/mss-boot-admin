package system

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mss-boot-io/mss-boot-admin/admin/models"
	"github.com/mss-boot-io/mss-boot-admin/admin/pkg"
	configgormdb "github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/config/gormdb"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/enum"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

const (
	releaseUpgradeStateAssertEnv       = "MSS_RELEASE_UPGRADE_STATE_ASSERT"
	releaseUpgradeSQLiteDSNEnv         = "MSS_RELEASE_UPGRADE_SQLITE_DSN"
	releaseUpgradeMySQLDSNEnv          = "MSS_RELEASE_UPGRADE_MYSQL_DSN"
	releaseUpgradePostgresDSNEnv       = "MSS_RELEASE_UPGRADE_POSTGRES_DSN"
	releaseUpgradeTestDatabaseName     = "mss_release_upgrade_test"
	releaseUpgradeSQLiteDatabaseFile   = releaseUpgradeTestDatabaseName + ".sqlite"
	releaseUpgradeDatabaseCheckTimeout = 2 * time.Minute
)

// TestV07UpgradeStateIntegration is an opt-in, read-only assertion over the
// databases produced by tools/release/verify-v0.7-upgrade.sh. It deliberately
// opens the configured targets again after both candidate migration passes so
// the release evidence proves the connected database identity and durable
// security state instead of only proving that the migration commands exited.
func TestV07UpgradeStateIntegration(t *testing.T) {
	if strings.TrimSpace(os.Getenv(releaseUpgradeStateAssertEnv)) != "1" {
		t.Skip(releaseUpgradeStateAssertEnv + " is not set to 1")
	}

	for _, test := range releaseUpgradeDatabaseSpecs() {
		t.Run(test.dialect, func(t *testing.T) {
			dsn := strings.TrimSpace(os.Getenv(test.dsnEnv))
			if dsn == "" {
				t.Fatalf("%s is required when %s=1", test.dsnEnv, releaseUpgradeStateAssertEnv)
			}

			db := openReleaseUpgradeStateDatabase(t, test.dialect, dsn)
			assertReleaseUpgradeDatabaseIdentity(t, db, test.dialect)
			assertReleaseUpgradeRetiredDeveloperState(t, db)
			defaultRoleID := assertReleaseUpgradeRoleTopology(t, db)
			assertReleaseUpgradeRevisionInfrastructure(t, db, defaultRoleID)
		})
	}
}

// TestV07UpgradeDatabaseIdentityIntegration runs before the first baseline
// migration. This keeps the destructive rehearsal fail-closed even when a
// syntactically valid DSN would select a different database after connection.
func TestV07UpgradeDatabaseIdentityIntegration(t *testing.T) {
	if strings.TrimSpace(os.Getenv(releaseUpgradeStateAssertEnv)) != "1" {
		t.Skip(releaseUpgradeStateAssertEnv + " is not set to 1")
	}

	for _, test := range releaseUpgradeDatabaseSpecs() {
		t.Run(test.dialect, func(t *testing.T) {
			dsn := strings.TrimSpace(os.Getenv(test.dsnEnv))
			if dsn == "" {
				t.Fatalf("%s is required when %s=1", test.dsnEnv, releaseUpgradeStateAssertEnv)
			}
			db := openReleaseUpgradeStateDatabase(t, test.dialect, dsn)
			assertReleaseUpgradeDatabaseIdentity(t, db, test.dialect)
		})
	}
}

type releaseUpgradeDatabaseSpec struct {
	dialect string
	dsnEnv  string
}

func releaseUpgradeDatabaseSpecs() []releaseUpgradeDatabaseSpec {
	return []releaseUpgradeDatabaseSpec{
		{dialect: "sqlite", dsnEnv: releaseUpgradeSQLiteDSNEnv},
		{dialect: "mysql", dsnEnv: releaseUpgradeMySQLDSNEnv},
		{dialect: "postgres", dsnEnv: releaseUpgradePostgresDSNEnv},
	}
}

func TestReleaseUpgradeStateAssertionsSQLite(t *testing.T) {
	dsn := filepath.Join(t.TempDir(), releaseUpgradeSQLiteDatabaseFile)
	db, err := gorm.Open(configgormdb.Opens["sqlite"](dsn), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		t.Fatalf("open SQLite release upgrade assertion fixture: %T", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("obtain SQLite release upgrade assertion fixture connection: %T", err)
	}
	t.Cleanup(func() {
		if err := sqlDB.Close(); err != nil {
			t.Errorf("close SQLite release upgrade assertion fixture: %T", err)
		}
	})

	if err := db.AutoMigrate(
		&models.Role{},
		&models.Menu{},
		&models.API{},
		&models.CasbinRule{},
		&models.ConfigRevision{},
	); err != nil {
		t.Fatalf("migrate SQLite release upgrade assertion fixture: %T", err)
	}

	rootRole := &models.Role{Name: "admin", Status: enum.Enabled}
	defaultRole := &models.Role{Name: "user", Status: enum.Enabled}
	if err := db.Create(rootRole).Error; err != nil {
		t.Fatalf("create root release upgrade assertion fixture: %T", err)
	}
	if err := db.Create(defaultRole).Error; err != nil {
		t.Fatalf("create default release upgrade assertion fixture: %T", err)
	}
	if err := db.Table(rootRole.TableName()).Where("id = ?", rootRole.ID).
		Updates(map[string]any{"root": true, "default": false}).Error; err != nil {
		t.Fatalf("mark root release upgrade assertion fixture: %T", err)
	}
	if err := db.Table(defaultRole.TableName()).Where("id = ?", defaultRole.ID).
		Updates(map[string]any{"root": false, "default": true}).Error; err != nil {
		t.Fatalf("mark default release upgrade assertion fixture: %T", err)
	}
	for _, revision := range []models.ConfigRevision{
		{
			Scope: leastPrivilegeAuthorizationScopeGlobal, Resource: leastPrivilegeAuthorizationResource,
			Revision: 1, UpdatedAt: time.Now(),
		},
		{
			Scope: leastPrivilegeAuthorizationScopeRole, OwnerID: defaultRole.ID,
			Resource: leastPrivilegeAuthorizationResource, Revision: 1, UpdatedAt: time.Now(),
		},
	} {
		if err := db.Create(&revision).Error; err != nil {
			t.Fatalf("create release upgrade revision assertion fixture: %T", err)
		}
	}

	assertReleaseUpgradeDatabaseIdentity(t, db, "sqlite")
	assertReleaseUpgradeRetiredDeveloperState(t, db)
	gotDefaultRoleID := assertReleaseUpgradeRoleTopology(t, db)
	if gotDefaultRoleID != defaultRole.ID {
		t.Fatalf("release upgrade default role id = %q, want %q", gotDefaultRoleID, defaultRole.ID)
	}
	assertReleaseUpgradeRevisionInfrastructure(t, db, gotDefaultRoleID)
}

func TestIsReleaseUpgradeRetiredMenuPath(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{path: "/develop", want: true},
		{path: "/model", want: true},
		{path: "/generator", want: true},
		{path: "/field/legacy", want: true},
		{path: "/virtual/legacy", want: true},
		{path: "/custom", want: false},
		{path: "/development-dashboard", want: false},
	}
	for _, test := range tests {
		t.Run(test.path, func(t *testing.T) {
			if got := isReleaseUpgradeRetiredMenuPath(test.path); got != test.want {
				t.Fatalf("isReleaseUpgradeRetiredMenuPath(%q) = %t, want %t", test.path, got, test.want)
			}
		})
	}
}

func openReleaseUpgradeStateDatabase(t *testing.T, dialect, dsn string) *gorm.DB {
	t.Helper()
	dialector, ok := configgormdb.Opens[dialect]
	if !ok {
		t.Fatalf("release upgrade dialect %q is unavailable", dialect)
	}

	db, err := gorm.Open(dialector(dsn), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		t.Fatalf("open %s release upgrade database: %T", dialect, err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("obtain %s release upgrade connection: %T", dialect, err)
	}
	sqlDB.SetMaxOpenConns(2)
	sqlDB.SetMaxIdleConns(1)

	ctx, cancel := context.WithTimeout(context.Background(), releaseUpgradeDatabaseCheckTimeout)
	db = db.WithContext(ctx)
	if err := sqlDB.PingContext(ctx); err != nil {
		cancel()
		_ = sqlDB.Close()
		t.Fatalf("ping %s release upgrade database: %T", dialect, err)
	}
	t.Cleanup(func() {
		cancel()
		if err := sqlDB.Close(); err != nil {
			t.Errorf("close %s release upgrade database: %T", dialect, err)
		}
	})
	return db
}

func assertReleaseUpgradeDatabaseIdentity(t *testing.T, db *gorm.DB, dialect string) {
	t.Helper()
	if dialect == "sqlite" {
		var databases []struct {
			Name string `gorm:"column:name"`
			File string `gorm:"column:file"`
		}
		if err := db.Raw("PRAGMA database_list").Scan(&databases).Error; err != nil {
			t.Fatalf("inspect SQLite release upgrade database identity: %T", err)
		}
		for _, database := range databases {
			if database.Name == "main" && filepath.Base(filepath.Clean(database.File)) == releaseUpgradeSQLiteDatabaseFile {
				return
			}
		}
		t.Fatalf("refusing release upgrade assertion outside SQLite database %q", releaseUpgradeSQLiteDatabaseFile)
	}

	query := "SELECT current_database()"
	if dialect == "mysql" {
		query = "SELECT DATABASE()"
	}
	var databaseName string
	if err := db.Raw(query).Scan(&databaseName).Error; err != nil {
		t.Fatalf("inspect %s release upgrade database identity: %T", dialect, err)
	}
	if databaseName != releaseUpgradeTestDatabaseName {
		t.Fatalf(
			"refusing release upgrade assertion outside the allowlisted %s database %q",
			dialect,
			releaseUpgradeTestDatabaseName,
		)
	}
}

func assertReleaseUpgradeRetiredDeveloperState(t *testing.T, db *gorm.DB) {
	t.Helper()
	var menus []models.Menu
	if err := db.Find(&menus).Error; err != nil {
		t.Fatalf("load active release upgrade menus: %T", err)
	}
	for _, menu := range menus {
		if isReleaseUpgradeRetiredMenuPath(menu.Path) {
			t.Fatalf("retired developer menu remains active: %q", menu.Path)
		}
	}

	retiredAPIPaths := make(map[string]struct{})
	for _, route := range retiredRuntimeRoutes {
		retiredAPIPaths[route.Path] = struct{}{}
		retiredAPIPaths[normalizeRouteInventoryPath(route.Path)] = struct{}{}
	}
	var policies []models.CasbinRule
	if err := db.Where("ptype = ?", "p").Find(&policies).Error; err != nil {
		t.Fatalf("load release upgrade authorization policies: %T", err)
	}
	for _, policy := range policies {
		if isReleaseUpgradeRetiredMenuPath(policy.V2) {
			t.Fatalf("retired developer menu policy remains active: %s %s", policy.V1, policy.V2)
		}
		if policy.V1 != pkg.APIAccessType.String() {
			continue
		}
		if _, retired := retiredAPIPaths[policy.V2]; retired {
			t.Fatalf("retired developer API policy remains active: %s %s", normalizedMethod(policy.V3), policy.V2)
		}
	}

	var APIs []models.API
	if err := db.Find(&APIs).Error; err != nil {
		t.Fatalf("load active release upgrade API inventory: %T", err)
	}
	for _, API := range APIs {
		if _, retired := retiredAPIPaths[API.Path]; retired {
			t.Fatalf("retired developer API inventory remains active: %s %s", normalizedMethod(API.Method), API.Path)
		}
	}
}

func isReleaseUpgradeRetiredMenuPath(path string) bool {
	return path == "/develop" || isRetiredRuntimeProductPath(path)
}

func assertReleaseUpgradeRoleTopology(t *testing.T, db *gorm.DB) string {
	t.Helper()
	var roots int64
	if err := db.Model(&models.Role{}).
		Where(map[string]any{"root": true, "status": enum.Enabled}).
		Count(&roots).Error; err != nil {
		t.Fatalf("count active release upgrade root roles: %T", err)
	}
	if roots < 1 {
		t.Fatal("release upgrade has no active root role")
	}

	var defaults []models.Role
	if err := db.Where(map[string]any{"default": true, "status": enum.Enabled}).
		Order("id ASC").
		Find(&defaults).Error; err != nil {
		t.Fatalf("load active release upgrade default roles: %T", err)
	}
	if len(defaults) != 1 {
		t.Fatalf("active release upgrade default roles = %d, want exactly 1", len(defaults))
	}
	if defaults[0].Root {
		t.Fatal("active release upgrade default role retains root authority")
	}
	return defaults[0].ID
}

func assertReleaseUpgradeRevisionInfrastructure(t *testing.T, db *gorm.DB, defaultRoleID string) {
	t.Helper()
	if !db.Migrator().HasTable(&models.ConfigRevision{}) {
		t.Fatal("release upgrade is missing config revision infrastructure")
	}
	for _, column := range []string{"scope", "owner_id", "resource", "revision", "updated_at"} {
		if !db.Migrator().HasColumn(&models.ConfigRevision{}, column) {
			t.Fatalf("release upgrade config revision infrastructure is missing column %q", column)
		}
	}

	assertReleaseUpgradeAuthorizationRevision(t, db, leastPrivilegeAuthorizationScopeGlobal, "")
	assertReleaseUpgradeAuthorizationRevision(t, db, leastPrivilegeAuthorizationScopeRole, defaultRoleID)
}

func assertReleaseUpgradeAuthorizationRevision(t *testing.T, db *gorm.DB, scope, ownerID string) {
	t.Helper()
	var revision models.ConfigRevision
	if err := db.Where(
		"scope = ? AND owner_id = ? AND resource = ?",
		scope,
		ownerID,
		leastPrivilegeAuthorizationResource,
	).Take(&revision).Error; err != nil {
		t.Fatalf("load release upgrade authorization revision %s/%s: %T", scope, ownerID, err)
	}
	if revision.Revision < 1 {
		t.Fatalf("release upgrade authorization revision %s/%s = %d, want at least 1", scope, ownerID, revision.Revision)
	}
}
