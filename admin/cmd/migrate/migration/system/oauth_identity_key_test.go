package system

import (
	"database/sql"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mss-boot-io/mss-boot-admin/admin/models"
	"github.com/mss-boot-io/mss-boot-admin/admin/pkg"
	migrationmodels "github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/migration/models"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

const oauthIdentityKeyTestVersion = "20260806172000-test"

type oauthIdentityKeyTestRow struct {
	ID          string
	IdentityKey sql.NullString
	DeletedAt   sql.NullTime
}

func TestOAuthIdentityKeyMigrationBackfillsActiveRowsAndIsIdempotent(t *testing.T) {
	db := openOAuthIdentityKeyMigrationTestDB(t)
	createLegacyOAuthIdentityTable(t, db)
	fixtures := []struct {
		id        string
		provider  string
		openID    string
		unionID   string
		deletedAt any
	}{
		{id: "active-github", provider: " GitHub ", openID: " 42 "},
		{id: "active-lark", provider: "lark", unionID: " on_abc "},
		{id: "active-lark-case", provider: "lark", unionID: " on_ABC "},
		{id: "deleted-duplicate", provider: "github", openID: "42", deletedAt: "2026-08-01 00:00:00"},
		{id: "deleted-incomplete", provider: "lark", deletedAt: "2026-08-01 00:00:00"},
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
			t.Fatalf("insert legacy OAuth identity %q: %v", fixture.id, err)
		}
	}

	for attempt := 1; attempt <= 2; attempt++ {
		if err := migrateOAuthIdentityKeys(db, oauthIdentityKeyTestVersion); err != nil {
			t.Fatalf("migration attempt %d: %v", attempt, err)
		}
	}
	if !db.Migrator().HasColumn(&models.UserOAuth2{}, "IdentityKey") {
		t.Fatal("migration did not add identity_key")
	}
	if !db.Migrator().HasIndex(&models.UserOAuth2{}, oauthIdentityKeyUniqueIndex) {
		t.Fatal("migration did not add the identity key unique index")
	}

	rows := make([]oauthIdentityKeyTestRow, 0, len(fixtures))
	if err := db.Table((&models.UserOAuth2{}).TableName()).
		Select("id", "identity_key", "deleted_at").
		Order("id ASC").
		Scan(&rows).Error; err != nil {
		t.Fatalf("load migrated identities: %v", err)
	}
	wantKeys := map[string]string{
		"active-github":    "github:42",
		"active-lark":      "lark:on_abc",
		"active-lark-case": "lark:on_ABC",
	}
	for _, row := range rows {
		want, active := wantKeys[row.ID]
		if active {
			if !row.IdentityKey.Valid || row.IdentityKey.String != want {
				t.Fatalf("identity %s key = %#v, want %q", row.ID, row.IdentityKey, want)
			}
			continue
		}
		if row.IdentityKey.Valid {
			t.Fatalf("soft-deleted identity %s key = %#v, want NULL", row.ID, row.IdentityKey)
		}
	}
	var exactMatches int64
	if err := db.Table((&models.UserOAuth2{}).TableName()).
		Where("identity_key = ?", "lark:on_abc").
		Count(&exactMatches).Error; err != nil {
		t.Fatal(err)
	}
	if exactMatches != 1 {
		t.Fatalf("case-sensitive exact identity lookup matched %d rows, want 1", exactMatches)
	}

	if err := db.Exec(
		`INSERT INTO mss_boot_user_oauth2
		 (id, user_id, provider, open_id, identity_key)
		 VALUES (?, ?, ?, ?, ?)`,
		"duplicate-active", "other-user", "github", "42", "github:42",
	).Error; err == nil {
		t.Fatal("unique index accepted a duplicate active identity key")
	}
	for _, id := range []string{"deleted-null-1", "deleted-null-2"} {
		if err := db.Exec(
			`INSERT INTO mss_boot_user_oauth2
			 (id, user_id, provider, deleted_at, identity_key)
			 VALUES (?, ?, ?, ?, NULL)`,
			id, "deleted-user", "github", "2026-08-01 00:00:00",
		).Error; err != nil {
			t.Fatalf("unique index rejected nullable deleted identity %q: %v", id, err)
		}
	}
	assertOAuthIdentityMigrationVersionCount(t, db, oauthIdentityKeyTestVersion, 1)
}

func TestOAuthIdentityKeyMigrationRejectsHistoricalDuplicatesBeforeBackfill(t *testing.T) {
	db := openOAuthIdentityKeyMigrationTestDB(t)
	createLegacyOAuthIdentityTable(t, db)
	for _, fixture := range []struct {
		id       string
		provider string
		openID   string
	}{
		{id: "binding-a", provider: "github", openID: "42"},
		{id: "binding-b", provider: " GITHUB ", openID: " 42 "},
	} {
		if err := db.Exec(
			`INSERT INTO mss_boot_user_oauth2 (id, user_id, provider, open_id)
			 VALUES (?, ?, ?, ?)`,
			fixture.id, "user-"+fixture.id, fixture.provider, fixture.openID,
		).Error; err != nil {
			t.Fatal(err)
		}
	}

	err := migrateOAuthIdentityKeys(db, oauthIdentityKeyTestVersion)
	if err == nil || !strings.Contains(err.Error(), "duplicate active") ||
		!strings.Contains(err.Error(), "binding-a") || !strings.Contains(err.Error(), "binding-b") {
		t.Fatalf("duplicate migration error = %v", err)
	}
	var populated int64
	if countErr := db.Table((&models.UserOAuth2{}).TableName()).
		Where("identity_key IS NOT NULL").
		Count(&populated).Error; countErr != nil {
		t.Fatal(countErr)
	}
	if populated != 0 {
		t.Fatalf("duplicate migration backfilled %d rows before failing", populated)
	}
	assertOAuthIdentityMigrationVersionCount(t, db, oauthIdentityKeyTestVersion, 0)
}

func TestOAuthIdentityKeyMigrationRejectsIncompleteActiveIdentity(t *testing.T) {
	db := openOAuthIdentityKeyMigrationTestDB(t)
	createLegacyOAuthIdentityTable(t, db)
	if err := db.Exec(
		`INSERT INTO mss_boot_user_oauth2 (id, user_id, provider, open_id, union_id)
		 VALUES (?, ?, ?, ?, ?)`,
		"missing-union", "user-lark", "lark", "wrong-field", "",
	).Error; err != nil {
		t.Fatal(err)
	}

	err := migrateOAuthIdentityKeys(db, oauthIdentityKeyTestVersion)
	if err == nil || !strings.Contains(err.Error(), "active row missing-union is invalid") {
		t.Fatalf("incomplete migration error = %v", err)
	}
	assertOAuthIdentityMigrationVersionCount(t, db, oauthIdentityKeyTestVersion, 0)
}

func TestOAuthIdentityKeyMigrationFreshSchema(t *testing.T) {
	db := openOAuthIdentityKeyMigrationTestDB(t)
	for attempt := 1; attempt <= 2; attempt++ {
		if err := migrateOAuthIdentityKeys(db, oauthIdentityKeyTestVersion); err != nil {
			t.Fatalf("fresh migration attempt %d: %v", attempt, err)
		}
	}
	if !db.Migrator().HasTable(&models.UserOAuth2{}) ||
		!db.Migrator().HasColumn(&models.UserOAuth2{}, "IdentityKey") ||
		!db.Migrator().HasIndex(&models.UserOAuth2{}, oauthIdentityKeyUniqueIndex) {
		t.Fatal("fresh migration did not create the OAuth identity key contract")
	}

	identity := &models.UserOAuth2{
		UserID:   "fresh-user",
		Provider: pkg.GithubLoginProvider,
		OpenID:   "42",
	}
	if err := db.Create(identity).Error; err != nil {
		t.Fatalf("create identity on fresh schema: %v", err)
	}
	if identity.IdentityKey == nil || *identity.IdentityKey != "github:42" {
		t.Fatalf("fresh identity key = %#v", identity.IdentityKey)
	}
	assertOAuthIdentityMigrationVersionCount(t, db, oauthIdentityKeyTestVersion, 1)
}

func createLegacyOAuthIdentityTable(t *testing.T, db *gorm.DB) {
	t.Helper()
	if err := db.Exec(`CREATE TABLE mss_boot_user_oauth2 (
		id TEXT PRIMARY KEY,
		user_id TEXT,
		provider TEXT,
		open_id TEXT,
		union_id TEXT,
		created_at DATETIME,
		updated_at DATETIME,
		deleted_at DATETIME
	)`).Error; err != nil {
		t.Fatalf("create legacy OAuth identity table: %v", err)
	}
}

func openOAuthIdentityKeyMigrationTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	databasePath := filepath.Join(t.TempDir(), "oauth-identity-key.db")
	db, err := gorm.Open(sqlite.Open(databasePath), &gorm.Config{})
	if err != nil {
		t.Fatalf("open OAuth identity migration database: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	return db
}

func assertOAuthIdentityMigrationVersionCount(t *testing.T, db *gorm.DB, version string, want int64) {
	t.Helper()
	var count int64
	if err := db.Model(&migrationmodels.Migration{}).
		Where("version = ?", version).
		Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != want {
		t.Fatalf("OAuth identity migration version count = %d, want %d", count, want)
	}
}
