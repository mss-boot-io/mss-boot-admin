package system

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/mss-boot-io/mss-boot-admin/admin/models"
	configgormdb "github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/config/gormdb"
	migrationmodels "github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/migration/models"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

const (
	patMigrationMySQLDSNEnv     = "MSS_PAT_TEST_MYSQL_DSN"
	patMigrationPostgresDSNEnv  = "MSS_PAT_TEST_POSTGRES_DSN"
	patMigrationTestDatabaseTag = "mss_pat_test"
	patMigrationIntegrationVer  = "2026080617999"
)

func TestUserAuthTokenHashMigrationMySQLIntegration(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv(patMigrationMySQLDSNEnv))
	if dsn == "" {
		t.Skip(patMigrationMySQLDSNEnv + " is not set")
	}

	db := openPATMigrationIntegrationDB(t, "mysql", dsn)
	runPATMigrationIntegrationContract(t, db, "DATETIME")
	t.Run("partial DDL restart", func(t *testing.T) {
		resetPATMigrationIntegrationTables(t, db)
		createLegacyPATTable(t, db, "DATETIME")
		if err := db.Exec(
			"ALTER TABLE mss_boot_user_auth_token ADD COLUMN token_hash VARCHAR(80) NOT NULL DEFAULT ''",
		).Error; err != nil {
			t.Fatal("create partial MySQL PAT schema failed")
		}

		rawToken := "mysql-partial-ddl-token"
		staleHash := models.HashUserAuthToken("stale-partial-ddl-token")
		if err := db.Exec(
			`INSERT INTO mss_boot_user_auth_token
				(id, user_id, token, token_hash, expired_at, revoked, created_at, updated_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			"partial-ddl",
			"integration-user",
			rawToken,
			staleHash,
			time.Now().UTC().Add(time.Hour).Truncate(time.Second),
			false,
			time.Now().UTC().Truncate(time.Second),
			time.Now().UTC().Truncate(time.Second),
		).Error; err != nil {
			t.Fatal("insert partial MySQL PAT fixture failed")
		}

		for attempt := 1; attempt <= 2; attempt++ {
			if err := migrateUserAuthTokenHashes(db, patMigrationIntegrationVer); err != nil {
				t.Fatalf("partial MySQL migration attempt %d failed", attempt)
			}
		}

		row := &models.UserAuthToken{}
		if err := db.First(row, "id = ?", "partial-ddl").Error; err != nil {
			t.Fatal("load partial MySQL PAT fixture failed")
		}
		if row.LegacyToken != "" {
			t.Fatal("partial MySQL migration retained plaintext")
		}
		wantHash := models.HashUserAuthToken(rawToken)
		if row.TokenHash != wantHash {
			t.Fatal("partial MySQL migration did not recompute the digest from plaintext")
		}
		if row.Fingerprint != models.UserAuthTokenFingerprint(wantHash) {
			t.Fatal("partial MySQL migration stored an invalid fingerprint")
		}
		if row.Revoked {
			t.Fatal("partial MySQL migration revoked a recoverable token")
		}
		assertPATMigrationNoPlaintext(t, db)
		assertPATMigrationVersionCount(t, db, patMigrationIntegrationVer, 1)
	})
}

func TestUserAuthTokenHashMigrationPostgresIntegration(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv(patMigrationPostgresDSNEnv))
	if dsn == "" {
		t.Skip(patMigrationPostgresDSNEnv + " is not set")
	}

	db := openPATMigrationIntegrationDB(t, "postgres", dsn)
	runPATMigrationIntegrationContract(t, db, "TIMESTAMPTZ")
}

func openPATMigrationIntegrationDB(t *testing.T, dialect, dsn string) *gorm.DB {
	t.Helper()
	dialector, ok := configgormdb.Opens[dialect]
	if !ok {
		t.Fatalf("PAT migration integration dialect %q is unavailable", dialect)
	}

	db, err := gorm.Open(dialector(dsn), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		t.Fatalf("open %s PAT migration integration database failed", dialect)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("obtain %s PAT migration integration connection failed", dialect)
	}
	sqlDB.SetMaxOpenConns(4)
	sqlDB.SetMaxIdleConns(2)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	db = db.WithContext(ctx)
	if err := sqlDB.PingContext(ctx); err != nil {
		cancel()
		_ = sqlDB.Close()
		t.Fatalf("ping %s PAT migration integration database failed", dialect)
	}

	var databaseName string
	databaseNameQuery := "SELECT current_database()"
	if dialect == "mysql" {
		databaseNameQuery = "SELECT DATABASE()"
	}
	if err := db.Raw(databaseNameQuery).Scan(&databaseName).Error; err != nil {
		cancel()
		_ = sqlDB.Close()
		t.Fatalf("verify %s PAT migration integration database failed", dialect)
	}
	if !strings.Contains(strings.ToLower(strings.TrimSpace(databaseName)), patMigrationTestDatabaseTag) {
		cancel()
		_ = sqlDB.Close()
		t.Fatalf("refusing destructive %s PAT migration test outside a marked test database", dialect)
	}

	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cleanupCancel()
		cleanupDB := db.WithContext(cleanupCtx)
		if err := dropPATMigrationIntegrationTables(cleanupDB); err != nil {
			t.Errorf("clean up %s PAT migration integration tables failed", dialect)
		}
		cancel()
		if err := sqlDB.Close(); err != nil {
			t.Errorf("close %s PAT migration integration database failed", dialect)
		}
	})
	resetPATMigrationIntegrationTables(t, db)
	return db
}

func runPATMigrationIntegrationContract(t *testing.T, db *gorm.DB, timestampType string) {
	t.Helper()
	t.Run("fresh and idempotent", func(t *testing.T) {
		resetPATMigrationIntegrationTables(t, db)
		for attempt := 1; attempt <= 2; attempt++ {
			if err := migrateUserAuthTokenHashes(db, patMigrationIntegrationVer); err != nil {
				t.Fatalf("fresh PAT migration attempt %d failed", attempt)
			}
		}
		if !db.Migrator().HasTable(&models.UserAuthToken{}) {
			t.Fatal("fresh PAT migration did not create the token table")
		}
		for _, column := range []string{"token", "token_hash", "fingerprint"} {
			if !db.Migrator().HasColumn(&models.UserAuthToken{}, column) {
				t.Fatalf("fresh PAT migration is missing column %q", column)
			}
		}
		assertPATMigrationNoPlaintext(t, db)
		assertPATMigrationVersionCount(t, db, patMigrationIntegrationVer, 1)
	})

	t.Run("legacy rows and restart", func(t *testing.T) {
		resetPATMigrationIntegrationTables(t, db)
		createLegacyPATTable(t, db, timestampType)

		now := time.Now().UTC().Truncate(time.Second)
		fixtures := []struct {
			id        string
			rawToken  string
			expiredAt time.Time
			revoked   bool
		}{
			{id: "active", rawToken: "integration-active-token", expiredAt: now.Add(24 * time.Hour)},
			{id: "revoked", rawToken: "integration-revoked-token", expiredAt: now.Add(24 * time.Hour), revoked: true},
			{id: "expired", rawToken: "integration-expired-token", expiredAt: now.Add(-time.Hour)},
			{id: "empty", rawToken: "", expiredAt: now.Add(24 * time.Hour)},
		}
		for _, fixture := range fixtures {
			if err := db.Exec(
				`INSERT INTO mss_boot_user_auth_token
					(id, user_id, token, expired_at, revoked, created_at, updated_at)
				 VALUES (?, ?, ?, ?, ?, ?, ?)`,
				fixture.id,
				"integration-user",
				fixture.rawToken,
				fixture.expiredAt,
				fixture.revoked,
				now,
				now,
			).Error; err != nil {
				t.Fatalf("insert legacy PAT fixture %q failed", fixture.id)
			}
		}

		for attempt := 1; attempt <= 2; attempt++ {
			if err := migrateUserAuthTokenHashes(db, patMigrationIntegrationVer); err != nil {
				t.Fatalf("legacy PAT migration attempt %d failed", attempt)
			}
		}

		assertPATMigrationNoPlaintext(t, db)
		assertPATMigrationVersionCount(t, db, patMigrationIntegrationVer, 1)
		for _, fixture := range fixtures {
			row := &models.UserAuthToken{}
			if err := db.First(row, "id = ?", fixture.id).Error; err != nil {
				t.Fatalf("load migrated PAT fixture %q failed", fixture.id)
			}
			if row.LegacyToken != "" {
				t.Fatalf("migrated PAT fixture %q retained plaintext", fixture.id)
			}
			if fixture.rawToken == "" {
				if !row.Revoked || row.TokenHash != "" || row.Fingerprint != "" {
					t.Fatalf("empty PAT fixture %q did not fail closed", fixture.id)
				}
				continue
			}

			wantHash := models.HashUserAuthToken(fixture.rawToken)
			if row.TokenHash != wantHash {
				t.Fatalf("migrated PAT fixture %q has an invalid digest", fixture.id)
			}
			if row.Fingerprint != models.UserAuthTokenFingerprint(wantHash) {
				t.Fatalf("migrated PAT fixture %q has an invalid fingerprint", fixture.id)
			}
			if row.Revoked != fixture.revoked {
				t.Fatalf("migrated PAT fixture %q changed its revoked state", fixture.id)
			}
			if delta := row.ExpiredAt.Sub(fixture.expiredAt); delta < -time.Second || delta > time.Second {
				t.Fatalf("migrated PAT fixture %q changed its expiry", fixture.id)
			}
		}
	})
}

func createLegacyPATTable(t *testing.T, db *gorm.DB, timestampType string) {
	t.Helper()
	statement := fmt.Sprintf(`CREATE TABLE mss_boot_user_auth_token (
		id VARCHAR(64) PRIMARY KEY,
		user_id VARCHAR(64) NOT NULL,
		token TEXT NULL,
		expired_at %s NULL,
		revoked BOOLEAN NOT NULL DEFAULT FALSE,
		created_at %s NULL,
		updated_at %s NULL,
		deleted_at %s NULL
	)`, timestampType, timestampType, timestampType, timestampType)
	if err := db.Exec(statement).Error; err != nil {
		t.Fatal("create legacy PAT integration table failed")
	}
}

func resetPATMigrationIntegrationTables(t *testing.T, db *gorm.DB) {
	t.Helper()
	if err := dropPATMigrationIntegrationTables(db); err != nil {
		t.Fatal("reset PAT migration integration tables failed")
	}
}

func dropPATMigrationIntegrationTables(db *gorm.DB) error {
	if err := db.Exec("DROP TABLE IF EXISTS mss_boot_user_auth_token").Error; err != nil {
		return fmt.Errorf("drop PAT table: %w", err)
	}
	if err := db.Exec("DROP TABLE IF EXISTS mss_boot_migration").Error; err != nil {
		return fmt.Errorf("drop migration table: %w", err)
	}
	return nil
}

func assertPATMigrationNoPlaintext(t *testing.T, db *gorm.DB) {
	t.Helper()
	var count int64
	if err := db.Table((&models.UserAuthToken{}).TableName()).
		Where("COALESCE(token, '') <> ''").
		Count(&count).Error; err != nil {
		t.Fatal("count plaintext PAT values failed")
	}
	if count != 0 {
		t.Fatalf("plaintext PAT value count = %d, want 0", count)
	}
}

func assertPATMigrationVersionCount(t *testing.T, db *gorm.DB, version string, want int64) {
	t.Helper()
	var count int64
	if err := db.Model(&migrationmodels.Migration{}).
		Where("version = ?", version).
		Count(&count).Error; err != nil {
		t.Fatal("count PAT migration version rows failed")
	}
	if count != want {
		t.Fatalf("PAT migration version row count = %d, want %d", count, want)
	}
}
