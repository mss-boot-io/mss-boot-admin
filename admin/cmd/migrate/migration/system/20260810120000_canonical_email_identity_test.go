package system

import (
	"bytes"
	"errors"
	"fmt"
	"log"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/mss-boot-io/mss-boot-admin/admin/models"
	migrationmodels "github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/migration/models"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

const canonicalEmailIdentityTestVersion = "20260810120000-test"

func TestCanonicalEmailIdentityMigrationSQLiteFreshAndRepeat(t *testing.T) {
	db := openCanonicalEmailMigrationSQLite(t)
	createLegacyCanonicalEmailUserTable(t, db)

	for attempt := 1; attempt <= 2; attempt++ {
		if err := migrateCanonicalEmailIdentities(db, canonicalEmailIdentityTestVersion); err != nil {
			t.Fatalf("migration attempt %d: %v", attempt, err)
		}
	}
	if err := verifyCanonicalEmailIndex(db, "sqlite"); err != nil {
		t.Fatalf("verify SQLite canonical-email index: %v", err)
	}
	assertCanonicalEmailMigrationVersionCount(t, db, canonicalEmailIdentityTestVersion, 1)

	for index := range 2 {
		if err := db.Exec(
			"INSERT INTO mss_boot_users (id, email) VALUES (?, '')",
			fmt.Sprintf("empty-%d", index),
		).Error; err != nil {
			t.Fatalf("empty email %d was rejected: %v", index, err)
		}
	}
}

func TestCanonicalEmailIdentityMigrationSQLiteUpgradeBackfillsAndScopesSoftDelete(t *testing.T) {
	db := openCanonicalEmailMigrationSQLite(t)
	createLegacyCanonicalEmailUserTable(t, db)
	fixtures := []struct {
		id        string
		email     any
		deletedAt any
	}{
		{id: "active-mixed", email: " Person@EXAMPLE.COM "},
		{id: "active-empty", email: ""},
		{id: "active-null", email: nil},
		{id: "deleted-duplicate", email: "PERSON@example.com", deletedAt: "2026-08-01 00:00:00"},
	}
	for _, fixture := range fixtures {
		if err := db.Exec(
			"INSERT INTO mss_boot_users (id, email, deleted_at) VALUES (?, ?, ?)",
			fixture.id,
			fixture.email,
			fixture.deletedAt,
		).Error; err != nil {
			t.Fatal(err)
		}
	}

	if err := migrateCanonicalEmailIdentities(db, canonicalEmailIdentityTestVersion); err != nil {
		t.Fatal(err)
	}
	var activeEmail string
	if err := db.Raw(
		"SELECT email FROM mss_boot_users WHERE id = ?",
		"active-mixed",
	).Scan(&activeEmail).Error; err != nil {
		t.Fatal(err)
	}
	if activeEmail != "person@example.com" {
		t.Fatalf("active email = %q, want canonical value", activeEmail)
	}
	var deletedEmail string
	if err := db.Raw(
		"SELECT email FROM mss_boot_users WHERE id = ?",
		"deleted-duplicate",
	).Scan(&deletedEmail).Error; err != nil {
		t.Fatal(err)
	}
	if deletedEmail != "PERSON@example.com" {
		t.Fatalf("soft-deleted email = %q, want preserved historical value", deletedEmail)
	}

	if err := db.Exec(
		"INSERT INTO mss_boot_users (id, email) VALUES (?, ?)",
		"raw-canonical-duplicate",
		" PERSON@EXAMPLE.COM ",
	).Error; err == nil {
		t.Fatal("partial expression index accepted a canonical duplicate")
	}
	if err := db.Exec(
		"INSERT INTO mss_boot_users (id, email, deleted_at) VALUES (?, ?, ?)",
		"second-deleted-duplicate",
		"person@example.com",
		"2026-08-02 00:00:00",
	).Error; err != nil {
		t.Fatalf("partial index rejected a soft-deleted duplicate: %v", err)
	}
	assertCanonicalEmailMigrationVersionCount(t, db, canonicalEmailIdentityTestVersion, 1)
}

func TestCanonicalEmailIdentityMigrationPreflightFailsWithoutMutationOrDisclosure(t *testing.T) {
	db := openCanonicalEmailMigrationSQLite(t)
	createLegacyCanonicalEmailUserTable(t, db)
	fixtures := []struct {
		id    string
		email string
	}{
		{id: "sensitive-invalid-id", email: "invalid-address-sentinel"},
		{id: "sensitive-conflict-a", email: " Conflict@EXAMPLE.COM "},
		{id: "sensitive-conflict-b", email: "conflict@example.com"},
	}
	for _, fixture := range fixtures {
		if err := db.Exec(
			"INSERT INTO mss_boot_users (id, email) VALUES (?, ?)",
			fixture.id,
			fixture.email,
		).Error; err != nil {
			t.Fatal(err)
		}
	}

	var logs bytes.Buffer
	observed := db.Session(&gorm.Session{Logger: logger.New(
		log.New(&logs, "", 0),
		logger.Config{LogLevel: logger.Info, Colorful: false},
	)})
	err := migrateCanonicalEmailIdentities(observed, canonicalEmailIdentityTestVersion)
	if err == nil || !errors.Is(err, models.ErrEmailIdentityInvalid) ||
		!errors.Is(err, models.ErrEmailIdentityAmbiguous) {
		t.Fatalf("preflight error = %v, want invalid and ambiguous identity types", err)
	}
	if err.Error() != "canonical email migration preflight failed: invalid=1 conflicts=1" {
		t.Fatalf("preflight error = %q", err)
	}
	combined := logs.String() + err.Error()
	for _, sensitive := range []string{
		"invalid-address-sentinel",
		"Conflict@EXAMPLE.COM",
		"conflict@example.com",
		"sensitive-invalid-id",
		"SELECT",
	} {
		if strings.Contains(combined, sensitive) {
			t.Fatalf("preflight disclosure contains %q: %s", sensitive, combined)
		}
	}

	var unchanged string
	if err := db.Raw(
		"SELECT email FROM mss_boot_users WHERE id = ?",
		"sensitive-conflict-a",
	).Scan(&unchanged).Error; err != nil {
		t.Fatal(err)
	}
	if unchanged != " Conflict@EXAMPLE.COM " {
		t.Fatalf("failed preflight changed email to %q", unchanged)
	}
	if canonicalEmailSQLiteIndexExists(t, db) {
		t.Fatal("failed preflight created the unique index")
	}
	assertCanonicalEmailMigrationVersionCount(t, db, canonicalEmailIdentityTestVersion, 0)
}

func TestCanonicalEmailIdentityBackfillCASRejectsConcurrentChangeWithoutDisclosure(t *testing.T) {
	db := openCanonicalEmailMigrationSQLite(t)
	createLegacyCanonicalEmailUserTable(t, db)
	const (
		rowID           = "sensitive-concurrent-backfill-id"
		originalEmail   = " Original@EXAMPLE.COM "
		concurrentEmail = "concurrent-owner@example.com"
	)
	if err := db.Exec(
		"INSERT INTO mss_boot_users (id, email) VALUES (?, ?)",
		rowID,
		originalEmail,
	).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(
		"INSERT INTO mss_boot_users (id, email) VALUES (?, NULL)",
		"nullable-email-row",
	).Error; err != nil {
		t.Fatal(err)
	}

	backfills, err := prepareCanonicalEmailBackfill(db.Session(&gorm.Session{Logger: logger.Discard}))
	if err != nil {
		t.Fatal(err)
	}
	if len(backfills) != 1 {
		t.Fatalf("backfill candidates = %d, want one non-NULL string candidate", len(backfills))
	}
	if err := db.Exec(
		"UPDATE mss_boot_users SET email = ? WHERE id = ?",
		concurrentEmail,
		rowID,
	).Error; err != nil {
		t.Fatal(err)
	}

	err = applyCanonicalEmailBackfill(
		db.Session(&gorm.Session{Logger: logger.Discard}),
		backfills,
	)
	if err == nil || err.Error() != "canonical email migration: backfill failed" {
		t.Fatalf("concurrent backfill error = %v, want fixed failure", err)
	}
	for _, sensitive := range []string{rowID, originalEmail, concurrentEmail} {
		if strings.Contains(err.Error(), sensitive) {
			t.Fatalf("concurrent backfill error disclosed identity data: %v", err)
		}
	}

	var persisted string
	if err := db.Raw(
		"SELECT email FROM mss_boot_users WHERE id = ?",
		rowID,
	).Scan(&persisted).Error; err != nil {
		t.Fatal(err)
	}
	if persisted != concurrentEmail {
		t.Fatalf("CAS backfill overwrote concurrent value; persisted email = %q", persisted)
	}
	var nullCount int64
	if err := db.Raw(
		"SELECT COUNT(*) FROM mss_boot_users WHERE id = ? AND email IS NULL",
		"nullable-email-row",
	).Scan(&nullCount).Error; err != nil {
		t.Fatal(err)
	}
	if nullCount != 1 {
		t.Fatalf("nullable email rows preserved = %d, want one", nullCount)
	}
}

func TestCanonicalEmailIdentityMigrationSQLiteConcurrentClaimsAndReuseAfterSoftDelete(t *testing.T) {
	db := openCanonicalEmailMigrationSQLite(t)
	createLegacyCanonicalEmailUserTable(t, db)
	if err := migrateCanonicalEmailIdentities(db, canonicalEmailIdentityTestVersion); err != nil {
		t.Fatal(err)
	}

	const contenders = 24
	start := make(chan struct{})
	var wait sync.WaitGroup
	var successes atomic.Int64
	for index := range contenders {
		wait.Add(1)
		go func(index int) {
			defer wait.Done()
			<-start
			email := " Concurrent@EXAMPLE.COM "
			if index%2 == 0 {
				email = "concurrent@example.com"
			}
			if err := db.Exec(
				"INSERT INTO mss_boot_users (id, email) VALUES (?, ?)",
				fmt.Sprintf("contender-%02d", index),
				email,
			).Error; err == nil {
				successes.Add(1)
			}
		}(index)
	}
	close(start)
	wait.Wait()

	var activeCount int64
	if err := db.Table((&models.User{}).TableName()).
		Where("deleted_at IS NULL AND LOWER(TRIM(email)) = ?", "concurrent@example.com").
		Count(&activeCount).Error; err != nil {
		t.Fatal(err)
	}
	if successes.Load() != 1 || activeCount != 1 {
		t.Fatalf("concurrent successes=%d active=%d, want exactly one", successes.Load(), activeCount)
	}
	if err := db.Exec(
		"UPDATE mss_boot_users SET deleted_at = ? WHERE deleted_at IS NULL AND LOWER(TRIM(email)) = ?",
		"2026-08-10 12:00:00",
		"concurrent@example.com",
	).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(
		"INSERT INTO mss_boot_users (id, email) VALUES (?, ?)",
		"replacement",
		"CONCURRENT@example.com",
	).Error; err != nil {
		t.Fatalf("soft-delete did not release canonical identity: %v", err)
	}
}

func TestCanonicalEmailIdentityDataVerifierRejectsPostBackfillDriftWithoutDisclosure(t *testing.T) {
	db := openCanonicalEmailMigrationSQLite(t)
	createLegacyCanonicalEmailUserTable(t, db)
	if err := migrateCanonicalEmailIdentities(db, canonicalEmailIdentityTestVersion); err != nil {
		t.Fatal(err)
	}
	const sensitiveEmail = " Drift@EXAMPLE.COM "
	if err := db.Exec(
		"INSERT INTO mss_boot_users (id, email) VALUES (?, ?)",
		"sensitive-drift-id",
		sensitiveEmail,
	).Error; err != nil {
		t.Fatal(err)
	}
	err := verifyCanonicalEmailData(db.Session(&gorm.Session{Logger: logger.Discard}))
	if !errors.Is(err, models.ErrEmailIdentityInvalid) ||
		!strings.Contains(err.Error(), "noncanonical=1") {
		t.Fatalf("data verification error = %v", err)
	}
	if strings.Contains(err.Error(), sensitiveEmail) ||
		strings.Contains(err.Error(), "sensitive-drift-id") {
		t.Fatalf("data verification disclosed identity: %v", err)
	}
}

func TestCanonicalEmailIdentityDDLAndMetadataContractsAreAuditable(t *testing.T) {
	for _, token := range []string{
		"CREATE UNIQUE INDEX IF NOT EXISTS " + models.EmailIdentityUniqueIndex,
		"LOWER(TRIM(email))",
		"deleted_at IS NULL",
		"TRIM(email) <> ''",
	} {
		if !strings.Contains(canonicalEmailPartialIndexDDL, token) {
			t.Fatalf("partial-index DDL is missing %q: %s", token, canonicalEmailPartialIndexDDL)
		}
	}
	for _, token := range []string{
		"VARBINARY(100)",
		"GENERATED ALWAYS AS",
		"deleted_at",
		"LOWER(TRIM(`email`))",
		"BINARY(100)",
		"STORED",
	} {
		if !strings.Contains(canonicalEmailMySQLGeneratedColumnDDL, token) {
			t.Fatalf("MySQL generated-column DDL is missing %q: %s", token, canonicalEmailMySQLGeneratedColumnDDL)
		}
	}

	partial := canonicalEmailIndexMetadata{definition: canonicalEmailPartialIndexDDL}
	for _, dialect := range []string{"sqlite", "postgres"} {
		if err := validateCanonicalEmailIndexMetadata(dialect, partial); err != nil {
			t.Fatalf("%s metadata rejected: %v", dialect, err)
		}
	}
	mysql := canonicalEmailIndexMetadata{
		dataType:             "varbinary",
		nullable:             "YES",
		extra:                "STORED GENERATED",
		generationExpression: "case when (`deleted_at` is null and trim(`email`) <> '') then cast(lower(trim(`email`)) as binary(100)) else NULL end",
		indexColumns: []canonicalEmailIndexColumn{{
			name:     canonicalEmailGeneratedColumn,
			sequence: 1,
		}},
	}
	if err := validateCanonicalEmailIndexMetadata("mysql", mysql); err != nil {
		t.Fatalf("MySQL metadata rejected: %v", err)
	}
	mysql.indexColumns[0].nonUnique = true
	if err := validateCanonicalEmailIndexMetadata("mysql", mysql); err == nil {
		t.Fatal("MySQL verifier accepted a non-unique index")
	}
	partial.definition = strings.ReplaceAll(
		partial.definition,
		"deleted_at IS NULL",
		"deleted_at IS NOT NULL",
	)
	if err := validateCanonicalEmailIndexMetadata("postgres", partial); err == nil {
		t.Fatal("PostgreSQL verifier accepted the wrong active-row predicate")
	}
}

func openCanonicalEmailMigrationSQLite(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := filepath.Join(t.TempDir(), "canonical-email.db") +
		"?_busy_timeout=10000&_journal_mode=WAL"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB.SetMaxOpenConns(16)
	t.Cleanup(func() { _ = sqlDB.Close() })
	return db
}

func createLegacyCanonicalEmailUserTable(t *testing.T, db *gorm.DB) {
	t.Helper()
	if err := db.Exec(`CREATE TABLE mss_boot_users (
		id TEXT PRIMARY KEY,
		email TEXT NULL,
		deleted_at DATETIME NULL
	)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&migrationmodels.Migration{}); err != nil {
		t.Fatal(err)
	}
}

func canonicalEmailSQLiteIndexExists(t *testing.T, db *gorm.DB) bool {
	t.Helper()
	var count int64
	if err := db.Raw(
		"SELECT COUNT(*) FROM sqlite_master WHERE type = 'index' AND name = ?",
		models.EmailIdentityUniqueIndex,
	).Scan(&count).Error; err != nil {
		t.Fatal(err)
	}
	return count != 0
}

func assertCanonicalEmailMigrationVersionCount(
	t *testing.T,
	db *gorm.DB,
	version string,
	want int64,
) {
	t.Helper()
	var count int64
	if err := db.Model(&migrationmodels.Migration{}).
		Where("version = ?", version).
		Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != want {
		t.Fatalf("canonical-email migration version count = %d, want %d", count, want)
	}
}
