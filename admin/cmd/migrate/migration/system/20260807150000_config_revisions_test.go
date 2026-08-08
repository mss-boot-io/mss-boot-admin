package system

import (
	"database/sql"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/mss-boot-io/mss-boot-admin/admin/models"
	migrationmodels "github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/migration/models"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// migration.GetFilename retains the repository's historical 13-character
// version format from the timestamped filename.
const configRevisionsTestVersion = "2026080715000"

func TestConfigRevisionMigrationFreshSchemaAndIdempotency(t *testing.T) {
	db := openConfigRevisionMigrationTestDB(t)

	if err := migrateConfigRevisions(db, configRevisionsTestVersion); err != nil {
		t.Fatalf("run fresh config revision migration: %v", err)
	}
	assertConfigRevisionSchema(t, db)
	assertConfigRevisionMigrationVersionCount(t, db, 1)

	now := time.Now().UTC().Truncate(time.Second)
	if err := db.Exec(
		"INSERT INTO mss_boot_config_revisions (scope, owner_id, resource, updated_at) VALUES (?, ?, ?, ?)",
		"application",
		"",
		"theme",
		now,
	).Error; err != nil {
		t.Fatalf("insert revision using database default: %v", err)
	}
	row := &models.ConfigRevision{}
	if err := db.First(row, "scope = ? AND owner_id = ? AND resource = ?", "application", "", "theme").Error; err != nil {
		t.Fatalf("load default revision row: %v", err)
	}
	if row.Revision != 0 {
		t.Fatalf("default revision = %d, want 0", row.Revision)
	}

	if err := db.Model(&models.ConfigRevision{}).
		Where("scope = ? AND owner_id = ? AND resource = ?", "application", "", "theme").
		Updates(map[string]any{"revision": int64(7), "updated_at": now.Add(time.Minute)}).Error; err != nil {
		t.Fatalf("update revision sentinel: %v", err)
	}
	if err := migrateConfigRevisions(db, configRevisionsTestVersion); err != nil {
		t.Fatalf("rerun config revision migration: %v", err)
	}

	row = &models.ConfigRevision{}
	if err := db.First(row, "scope = ? AND owner_id = ? AND resource = ?", "application", "", "theme").Error; err != nil {
		t.Fatalf("load revision after rerun: %v", err)
	}
	if row.Revision != 7 {
		t.Fatalf("rerun changed revision to %d, want 7", row.Revision)
	}
	assertConfigRevisionSchema(t, db)
	assertConfigRevisionMigrationVersionCount(t, db, 1)
}

func TestConfigRevisionMigrationPreservesExistingConfigurationRows(t *testing.T) {
	db := openConfigRevisionMigrationTestDB(t)
	if err := db.Exec(`CREATE TABLE mss_boot_app_configs (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		"group" TEXT NOT NULL,
		value TEXT NOT NULL,
		auth BOOLEAN NOT NULL,
		updated_at DATETIME
	)`).Error; err != nil {
		t.Fatalf("create legacy app config table: %v", err)
	}
	if err := db.Exec(`CREATE TABLE mss_boot_user_configs (
		id TEXT PRIMARY KEY,
		user_id TEXT NOT NULL,
		name TEXT NOT NULL,
		"group" TEXT NOT NULL,
		value TEXT NOT NULL,
		updated_at DATETIME
	)`).Error; err != nil {
		t.Fatalf("create legacy user config table: %v", err)
	}

	now := time.Now().UTC().Truncate(time.Second)
	if err := db.Exec(
		`INSERT INTO mss_boot_app_configs (id, name, "group", value, auth, updated_at) VALUES (?, ?, ?, ?, ?, ?)`,
		"app-theme",
		"fixedHeader",
		"theme",
		"true",
		false,
		now,
	).Error; err != nil {
		t.Fatalf("insert existing app config: %v", err)
	}
	if err := db.Exec(
		`INSERT INTO mss_boot_user_configs (id, user_id, name, "group", value, updated_at) VALUES (?, ?, ?, ?, ?, ?)`,
		"user-theme",
		"user-1",
		"colorWeak",
		"theme",
		"true",
		now,
	).Error; err != nil {
		t.Fatalf("insert existing user config: %v", err)
	}

	beforeApp := readExistingAppConfigRows(t, db)
	beforeUser := readExistingUserConfigRows(t, db)
	beforeAppSchema := readSQLiteCreateSQL(t, db, "mss_boot_app_configs")
	beforeUserSchema := readSQLiteCreateSQL(t, db, "mss_boot_user_configs")

	if err := migrateConfigRevisions(db, configRevisionsTestVersion); err != nil {
		t.Fatalf("run upgrade config revision migration: %v", err)
	}

	if got := readExistingAppConfigRows(t, db); !reflect.DeepEqual(got, beforeApp) {
		t.Fatalf("app config rows changed:\n got: %#v\nwant: %#v", got, beforeApp)
	}
	if got := readExistingUserConfigRows(t, db); !reflect.DeepEqual(got, beforeUser) {
		t.Fatalf("user config rows changed:\n got: %#v\nwant: %#v", got, beforeUser)
	}
	if got := readSQLiteCreateSQL(t, db, "mss_boot_app_configs"); got != beforeAppSchema {
		t.Fatalf("app config schema changed:\n got: %s\nwant: %s", got, beforeAppSchema)
	}
	if got := readSQLiteCreateSQL(t, db, "mss_boot_user_configs"); got != beforeUserSchema {
		t.Fatalf("user config schema changed:\n got: %s\nwant: %s", got, beforeUserSchema)
	}
	assertConfigRevisionSchema(t, db)
	assertConfigRevisionMigrationVersionCount(t, db, 1)
}

func TestConfigRevisionCompositePrimaryKeySeparatesOwnersAndResources(t *testing.T) {
	db := openConfigRevisionMigrationTestDB(t)
	if err := migrateConfigRevisions(db, configRevisionsTestVersion); err != nil {
		t.Fatalf("run config revision migration: %v", err)
	}

	rows := []models.ConfigRevision{
		{Scope: "application", OwnerID: "", Resource: "theme", Revision: 1},
		{Scope: "application", OwnerID: "", Resource: "public-profile", Revision: 2},
		{Scope: "user", OwnerID: "user-1", Resource: "theme", Revision: 3},
		{Scope: "user", OwnerID: "user-2", Resource: "theme", Revision: 4},
	}
	if err := db.Create(&rows).Error; err != nil {
		t.Fatalf("create distinct composite keys: %v", err)
	}
	if err := db.Clauses(clause.OnConflict{DoNothing: true}).Create(&models.ConfigRevision{
		Scope: "user", OwnerID: "user-1", Resource: "theme", Revision: 99,
	}).Error; err != nil {
		t.Fatalf("attempt duplicate composite key: %v", err)
	}

	var count int64
	if err := db.Model(&models.ConfigRevision{}).Count(&count).Error; err != nil {
		t.Fatalf("count composite revision rows: %v", err)
	}
	if count != int64(len(rows)) {
		t.Fatalf("revision row count = %d, want %d", count, len(rows))
	}
	row := &models.ConfigRevision{}
	if err := db.First(row, "scope = ? AND owner_id = ? AND resource = ?", "user", "user-1", "theme").Error; err != nil {
		t.Fatalf("load duplicate-key sentinel: %v", err)
	}
	if row.Revision != 3 {
		t.Fatalf("duplicate composite key changed revision to %d, want 3", row.Revision)
	}
}

type sqliteConfigRevisionColumn struct {
	Name         string         `gorm:"column:name"`
	Type         string         `gorm:"column:type"`
	NotNull      int            `gorm:"column:notnull"`
	DefaultValue sql.NullString `gorm:"column:dflt_value"`
	PrimaryKey   int            `gorm:"column:pk"`
}

func assertConfigRevisionSchema(t *testing.T, db *gorm.DB) {
	t.Helper()
	if !db.Migrator().HasTable(&models.ConfigRevision{}) {
		t.Fatal("config revision table was not created")
	}
	var columns []sqliteConfigRevisionColumn
	if err := db.Raw("PRAGMA table_info('mss_boot_config_revisions')").Scan(&columns).Error; err != nil {
		t.Fatalf("inspect config revision schema: %v", err)
	}
	byName := make(map[string]sqliteConfigRevisionColumn, len(columns))
	for _, column := range columns {
		byName[column.Name] = column
	}
	wantPrimaryKeyOrder := map[string]int{
		"scope":    1,
		"owner_id": 2,
		"resource": 3,
	}
	for name, order := range wantPrimaryKeyOrder {
		column, ok := byName[name]
		if !ok {
			t.Fatalf("config revision schema missing column %q", name)
		}
		if column.PrimaryKey != order || column.NotNull != 1 {
			t.Fatalf("column %q primary-key order/not-null = %d/%d, want %d/1", name, column.PrimaryKey, column.NotNull, order)
		}
	}
	revision, ok := byName["revision"]
	if !ok {
		t.Fatal("config revision schema missing revision column")
	}
	if revision.PrimaryKey != 0 || revision.NotNull != 1 || !revision.DefaultValue.Valid || revision.DefaultValue.String != "0" {
		t.Fatalf("revision schema = %+v, want non-key NOT NULL DEFAULT 0", revision)
	}
	updatedAt, ok := byName["updated_at"]
	if !ok || updatedAt.PrimaryKey != 0 || updatedAt.NotNull != 1 {
		t.Fatalf("updated_at schema = %+v, want non-key NOT NULL", updatedAt)
	}
	for _, forbidden := range []string{"id", "created_at", "deleted_at"} {
		if _, exists := byName[forbidden]; exists {
			t.Fatalf("config revision schema contains forbidden lifecycle column %q", forbidden)
		}
	}
}

type existingAppConfigRow struct {
	ID        string
	Name      string
	Group     string
	Value     string
	Auth      bool
	UpdatedAt time.Time
}

type existingUserConfigRow struct {
	ID        string
	UserID    string
	Name      string
	Group     string
	Value     string
	UpdatedAt time.Time
}

func readExistingAppConfigRows(t *testing.T, db *gorm.DB) []existingAppConfigRow {
	t.Helper()
	var rows []existingAppConfigRow
	if err := db.Raw(`SELECT id, name, "group", value, auth, updated_at FROM mss_boot_app_configs ORDER BY id`).Scan(&rows).Error; err != nil {
		t.Fatalf("read existing app configs: %v", err)
	}
	return rows
}

func readExistingUserConfigRows(t *testing.T, db *gorm.DB) []existingUserConfigRow {
	t.Helper()
	var rows []existingUserConfigRow
	if err := db.Raw(`SELECT id, user_id, name, "group", value, updated_at FROM mss_boot_user_configs ORDER BY id`).Scan(&rows).Error; err != nil {
		t.Fatalf("read existing user configs: %v", err)
	}
	return rows
}

func readSQLiteCreateSQL(t *testing.T, db *gorm.DB, tableName string) string {
	t.Helper()
	var createSQL string
	if err := db.Raw("SELECT sql FROM sqlite_master WHERE type = 'table' AND name = ?", tableName).Scan(&createSQL).Error; err != nil {
		t.Fatalf("read schema for %s: %v", tableName, err)
	}
	return createSQL
}

func openConfigRevisionMigrationTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	databasePath := filepath.Join(t.TempDir(), "config-revision-migration.db")
	db, err := gorm.Open(sqlite.Open(databasePath), &gorm.Config{})
	if err != nil {
		t.Fatalf("open config revision migration database: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get config revision migration database: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	return db
}

func assertConfigRevisionMigrationVersionCount(t *testing.T, db *gorm.DB, want int64) {
	t.Helper()
	var count int64
	if err := db.Model(&migrationmodels.Migration{}).
		Where("version = ?", configRevisionsTestVersion).
		Count(&count).Error; err != nil {
		t.Fatalf("count config revision migration version: %v", err)
	}
	if count != want {
		t.Fatalf("config revision migration version count = %d, want %d", count, want)
	}
}
