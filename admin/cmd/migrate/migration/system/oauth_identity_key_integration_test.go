package system

import (
	"os"
	"strings"
	"testing"

	"github.com/mss-boot-io/mss-boot-admin/admin/models"
	"gorm.io/gorm"
)

const oauthIdentityKeyIntegrationVersion = "20260806172000-integration"

func TestOAuthIdentityKeyMigrationMySQLIntegration(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv(patMigrationMySQLDSNEnv))
	if dsn == "" {
		t.Skip(patMigrationMySQLDSNEnv + " is not set")
	}
	db := openPATMigrationIntegrationDB(t, "mysql", dsn)
	runOAuthIdentityKeyMigrationIntegrationContract(t, db, "mysql")
}

func TestOAuthIdentityKeyMigrationPostgresIntegration(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv(patMigrationPostgresDSNEnv))
	if dsn == "" {
		t.Skip(patMigrationPostgresDSNEnv + " is not set")
	}
	db := openPATMigrationIntegrationDB(t, "postgres", dsn)
	runOAuthIdentityKeyMigrationIntegrationContract(t, db, "postgres")
}

func runOAuthIdentityKeyMigrationIntegrationContract(t *testing.T, db *gorm.DB, dialect string) {
	t.Helper()
	t.Cleanup(func() {
		if err := dropOAuthIdentityKeyIntegrationTables(db); err != nil {
			t.Errorf("clean up %s OAuth identity migration tables failed: %v", dialect, err)
		}
	})
	resetOAuthIdentityKeyIntegrationTables(t, db)
	createLegacyOAuthIdentityIntegrationTable(t, db)

	fixtures := []struct {
		id        string
		provider  string
		openID    string
		unionID   string
		deletedAt any
	}{
		{id: "github-active", provider: "github", openID: "42"},
		{id: "lark-lower", provider: "lark", unionID: "on_abc"},
		{id: "lark-upper", provider: "lark", unionID: "on_ABC"},
		{id: "deleted-duplicate", provider: "lark", unionID: "on_abc", deletedAt: "2026-08-01 00:00:00"},
		{id: "deleted-incomplete", provider: "github", deletedAt: "2026-08-01 00:00:00"},
	}
	for _, fixture := range fixtures {
		if err := db.Exec(
			`INSERT INTO mss_boot_user_oauth2
			 (id, user_id, provider, open_id, union_id, deleted_at)
			 VALUES (?, ?, ?, ?, ?, ?)`,
			fixture.id,
			"user-"+fixture.id,
			fixture.provider,
			fixture.openID,
			fixture.unionID,
			fixture.deletedAt,
		).Error; err != nil {
			t.Fatalf("insert %s legacy OAuth identity %q failed: %v", dialect, fixture.id, err)
		}
	}

	for attempt := 1; attempt <= 2; attempt++ {
		if err := migrateOAuthIdentityKeys(db, oauthIdentityKeyIntegrationVersion); err != nil {
			t.Fatalf("%s OAuth identity migration attempt %d failed: %v", dialect, attempt, err)
		}
	}
	if !db.Migrator().HasColumn(&models.UserOAuth2{}, "IdentityKey") ||
		!db.Migrator().HasIndex(&models.UserOAuth2{}, oauthIdentityKeyUniqueIndex) {
		t.Fatalf("%s OAuth identity migration did not establish the schema contract", dialect)
	}
	if dialect == "mysql" {
		var collation string
		if err := db.Raw(
			`SELECT COLLATION_NAME
			 FROM information_schema.COLUMNS
			 WHERE TABLE_SCHEMA = DATABASE()
			   AND TABLE_NAME = ?
			   AND COLUMN_NAME = ?`,
			(&models.UserOAuth2{}).TableName(),
			"identity_key",
		).Scan(&collation).Error; err != nil {
			t.Fatalf("inspect MySQL OAuth identity collation failed: %v", err)
		}
		if collation != "utf8mb4_bin" {
			t.Fatalf("MySQL OAuth identity collation = %q, want utf8mb4_bin", collation)
		}
	}

	for key, want := range map[string]int64{
		"github:42":   1,
		"lark:on_abc": 1,
		"lark:on_ABC": 1,
	} {
		var count int64
		if err := db.Table((&models.UserOAuth2{}).TableName()).
			Where("identity_key = ?", key).
			Count(&count).Error; err != nil {
			t.Fatalf("query %s identity %q failed: %v", dialect, key, err)
		}
		if count != want {
			t.Fatalf("%s exact identity %q matched %d rows, want %d", dialect, key, count, want)
		}
	}
	var deletedWithKey int64
	if err := db.Table((&models.UserOAuth2{}).TableName()).
		Where("deleted_at IS NOT NULL AND identity_key IS NOT NULL").
		Count(&deletedWithKey).Error; err != nil {
		t.Fatal(err)
	}
	if deletedWithKey != 0 {
		t.Fatalf("%s migration retained %d soft-deleted identity keys", dialect, deletedWithKey)
	}

	if err := db.Exec(
		`INSERT INTO mss_boot_user_oauth2
		 (id, user_id, provider, union_id, identity_key)
		 VALUES (?, ?, ?, ?, ?)`,
		"duplicate-active", "other-user", "lark", "on_abc", "lark:on_abc",
	).Error; err == nil {
		t.Fatalf("%s unique index accepted an exact duplicate OAuth identity", dialect)
	}
	for _, id := range []string{"deleted-null-a", "deleted-null-b"} {
		if err := db.Exec(
			`INSERT INTO mss_boot_user_oauth2
			 (id, user_id, provider, deleted_at, identity_key)
			 VALUES (?, ?, ?, ?, NULL)`,
			id, "deleted-user", "lark", "2026-08-01 00:00:00",
		).Error; err != nil {
			t.Fatalf("%s unique index rejected nullable deleted row %q: %v", dialect, id, err)
		}
	}
	assertOAuthIdentityMigrationVersionCount(t, db, oauthIdentityKeyIntegrationVersion, 1)

	resetOAuthIdentityKeyIntegrationTables(t, db)
	createLegacyOAuthIdentityIntegrationTable(t, db)
	for _, id := range []string{"duplicate-a", "duplicate-b"} {
		if err := db.Exec(
			`INSERT INTO mss_boot_user_oauth2
			 (id, user_id, provider, open_id)
			 VALUES (?, ?, ?, ?)`,
			id, "user-"+id, "github", "42",
		).Error; err != nil {
			t.Fatal(err)
		}
	}
	err := migrateOAuthIdentityKeys(db, oauthIdentityKeyIntegrationVersion+"-duplicate")
	if err == nil || !strings.Contains(err.Error(), "duplicate active") {
		t.Fatalf("%s duplicate history migration error = %v", dialect, err)
	}
	assertOAuthIdentityMigrationVersionCount(t, db, oauthIdentityKeyIntegrationVersion+"-duplicate", 0)

	resetOAuthIdentityKeyIntegrationTables(t, db)
	for attempt := 1; attempt <= 2; attempt++ {
		if err := migrateOAuthIdentityKeys(db, oauthIdentityKeyIntegrationVersion+"-fresh"); err != nil {
			t.Fatalf("%s fresh OAuth identity migration attempt %d failed: %v", dialect, attempt, err)
		}
	}
	if !db.Migrator().HasTable(&models.UserOAuth2{}) ||
		!db.Migrator().HasColumn(&models.UserOAuth2{}, "IdentityKey") ||
		!db.Migrator().HasIndex(&models.UserOAuth2{}, oauthIdentityKeyUniqueIndex) {
		t.Fatalf("%s fresh OAuth identity migration did not establish the schema contract", dialect)
	}
	for _, fixture := range []struct {
		userID  string
		unionID string
	}{
		{userID: "fresh-lower", unionID: "on_fresh"},
		{userID: "fresh-upper", unionID: "on_FRESH"},
	} {
		identity := &models.UserOAuth2{
			UserID:   fixture.userID,
			Provider: "lark",
			UnionID:  fixture.unionID,
		}
		if err := db.Create(identity).Error; err != nil {
			t.Fatalf("%s fresh schema rejected case-distinct identity %q: %v", dialect, fixture.unionID, err)
		}
	}
	for _, key := range []string{"lark:on_fresh", "lark:on_FRESH"} {
		var count int64
		if err := db.Model(&models.UserOAuth2{}).Where("identity_key = ?", key).Count(&count).Error; err != nil {
			t.Fatal(err)
		}
		if count != 1 {
			t.Fatalf("%s fresh exact lookup %q matched %d rows, want 1", dialect, key, count)
		}
	}
	assertOAuthIdentityMigrationVersionCount(t, db, oauthIdentityKeyIntegrationVersion+"-fresh", 1)
}

func createLegacyOAuthIdentityIntegrationTable(t *testing.T, db *gorm.DB) {
	t.Helper()
	if err := db.Exec(`CREATE TABLE mss_boot_user_oauth2 (
		id VARCHAR(64) PRIMARY KEY,
		user_id VARCHAR(64) NULL,
		provider VARCHAR(20) NULL,
		open_id VARCHAR(64) NULL,
		union_id VARCHAR(64) NULL,
		created_at TIMESTAMP NULL,
		updated_at TIMESTAMP NULL,
		deleted_at TIMESTAMP NULL
	)`).Error; err != nil {
		t.Fatal("create legacy OAuth identity integration table failed")
	}
}

func resetOAuthIdentityKeyIntegrationTables(t *testing.T, db *gorm.DB) {
	t.Helper()
	if err := dropOAuthIdentityKeyIntegrationTables(db); err != nil {
		t.Fatal("reset OAuth identity integration tables failed")
	}
}

func dropOAuthIdentityKeyIntegrationTables(db *gorm.DB) error {
	if err := db.Exec("DROP TABLE IF EXISTS mss_boot_user_oauth2").Error; err != nil {
		return err
	}
	if err := db.Exec("DROP TABLE IF EXISTS mss_boot_users").Error; err != nil {
		return err
	}
	if err := db.Exec("DROP TABLE IF EXISTS mss_boot_roles").Error; err != nil {
		return err
	}
	if err := db.Exec("DROP TABLE IF EXISTS mss_boot_migration").Error; err != nil {
		return err
	}
	return nil
}
