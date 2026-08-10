package system

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mss-boot-io/mss-boot-admin/admin/models"
	"github.com/mss-boot-io/mss-boot-admin/admin/pkg/schemahealth"
	configgormdb "github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/config/gormdb"
	migrationmodels "github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/migration/models"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

const (
	canonicalEmailMySQLDSNEnv      = "MSS_EMAIL_IDENTITY_TEST_MYSQL_DSN"
	canonicalEmailPostgresDSNEnv   = "MSS_EMAIL_IDENTITY_TEST_POSTGRES_DSN"
	canonicalEmailTestDatabaseName = "mss_email_identity_test"
	canonicalEmailIntegrationVer   = "20260810120000-integration"
	canonicalEmailIntegrationTable = "mss_boot_users"
	canonicalEmailMigrationTable   = "mss_boot_migration"
)

func TestCanonicalEmailIdentityMigrationMySQLIntegration(t *testing.T) {
	runCanonicalEmailIdentityIntegration(t, "mysql", canonicalEmailMySQLDSNEnv)
}

func TestCanonicalEmailIdentityMigrationPostgresIntegration(t *testing.T) {
	runCanonicalEmailIdentityIntegration(t, "postgres", canonicalEmailPostgresDSNEnv)
}

func runCanonicalEmailIdentityIntegration(t *testing.T, dialect, environment string) {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv(environment))
	if dsn == "" {
		t.Skip(environment + " is not set")
	}
	db := openCanonicalEmailIdentityIntegrationDB(t, dialect, dsn)

	t.Run("upgrade repeat and soft-delete reuse", func(t *testing.T) {
		resetCanonicalEmailIdentityIntegrationTables(t, db)
		createCanonicalEmailIdentityLegacyTables(t, db, dialect)
		if err := db.Exec(
			`INSERT INTO mss_boot_users (id, email, deleted_at) VALUES (?, ?, NULL), (?, ?, NULL), (?, ?, ?)`,
			"active", " Person@Example.COM ",
			"empty", "",
			"deleted", "person@example.com", time.Now().UTC(),
		).Error; err != nil {
			t.Fatal("insert canonical-email upgrade fixtures failed")
		}

		for attempt := 1; attempt <= 2; attempt++ {
			if err := migrateCanonicalEmailIdentities(db, canonicalEmailIntegrationVer); err != nil {
				t.Fatalf("%s canonical-email migration attempt %d failed: %v", dialect, attempt, err)
			}
		}
		if err := schemahealth.VerifyCanonicalEmailIdentity(
			t.Context(),
			db,
			schemahealth.CanonicalEmailDataInvariant,
		); err != nil {
			t.Fatalf("%s canonical-email index verification failed: %v", dialect, err)
		}
		var email string
		if err := db.Raw(
			"SELECT email FROM mss_boot_users WHERE id = ?",
			"active",
		).Scan(&email).Error; err != nil {
			t.Fatal("load canonical-email upgrade fixture failed")
		}
		if email != "person@example.com" {
			t.Fatalf("canonical email = %q, want canonical value", email)
		}
		assertCanonicalEmailMigrationVersionCount(
			t,
			db,
			canonicalEmailIntegrationVer,
			1,
		)

		duplicate := db.Exec(
			`INSERT INTO mss_boot_users (id, email, deleted_at) VALUES (?, ?, NULL)`,
			"duplicate",
			"PERSON@example.com",
		).Error
		if duplicate == nil {
			t.Fatal("canonical-email unique index accepted an active duplicate")
		}
		if err := db.Exec(
			"UPDATE mss_boot_users SET deleted_at = ? WHERE id = ?",
			time.Now().UTC(),
			"active",
		).Error; err != nil {
			t.Fatal("soft-delete canonical-email owner failed")
		}
		if err := db.Exec(
			`INSERT INTO mss_boot_users (id, email, deleted_at) VALUES (?, ?, NULL)`,
			"replacement",
			"person@example.com",
		).Error; err != nil {
			t.Fatal("canonical-email identity was not reusable after soft delete")
		}
	})

	if dialect == "postgres" {
		t.Run("ASCII identity fold ignores Turkish ICU column collation", func(t *testing.T) {
			collationDB := openCanonicalEmailIdentityIntegrationDB(t, dialect, dsn)
			createCanonicalEmailIdentityPostgresTurkishLegacyTables(t, collationDB)

			var localeFold string
			if err := collationDB.Raw(`SELECT LOWER('I' COLLATE "tr-x-icu")`).Scan(&localeFold).Error; err != nil {
				t.Fatal("inspect PostgreSQL Turkish ICU fold failed")
			}
			if localeFold == "i" {
				t.Fatalf("Turkish ICU fixture is not locale-sensitive: LOWER(I) = %q", localeFold)
			}
			if err := migrateCanonicalEmailIdentities(collationDB, canonicalEmailIntegrationVer); err != nil {
				t.Fatalf("migrate Turkish-collated canonical-email table failed: %v", err)
			}
			if err := collationDB.Exec(
				`INSERT INTO mss_boot_users (id, email, deleted_at) VALUES (?, ?, NULL)`,
				"ascii-lower-owner",
				"i@example.com",
			).Error; err != nil {
				t.Fatal("insert canonical ASCII owner failed")
			}
			if err := schemahealth.VerifyCanonicalEmailIdentity(
				t.Context(),
				collationDB,
				schemahealth.CanonicalEmailDataInvariant,
			); err != nil {
				t.Fatalf("C-collated PostgreSQL identity index was rejected: %v", err)
			}
			if err := collationDB.Exec(
				`INSERT INTO mss_boot_users (id, email, deleted_at) VALUES (?, ?, NULL)`,
				"ascii-upper-owner",
				"I@example.com",
			).Error; err == nil {
				t.Fatal("C-collated identity index accepted ASCII I/i double ownership")
			}
		})
	}

	if dialect == "mysql" {
		t.Run("ASCII identity fold remains deterministic under Turkish column collation", func(t *testing.T) {
			collationDB := openCanonicalEmailIdentityIntegrationDB(t, dialect, dsn)
			createCanonicalEmailIdentityMySQLTurkishLegacyTables(t, collationDB)

			var localeFold string
			if err := collationDB.Raw(
				"SELECT LOWER(CONVERT('I' USING utf8mb4) COLLATE utf8mb4_tr_0900_ai_ci)",
			).Scan(&localeFold).Error; err != nil {
				t.Fatal("inspect MySQL Turkish fold failed")
			}
			if localeFold != "i" {
				t.Fatalf("MySQL Turkish ASCII fold = %q, want deterministic i", localeFold)
			}
			if err := migrateCanonicalEmailIdentities(collationDB, canonicalEmailIntegrationVer); err != nil {
				t.Fatalf("migrate Turkish-collated canonical-email table failed: %v", err)
			}
			if err := collationDB.Exec(
				`INSERT INTO mss_boot_users (id, email, deleted_at) VALUES (?, ?, NULL)`,
				"ascii-lower-owner",
				"i@example.com",
			).Error; err != nil {
				t.Fatal("insert canonical ASCII owner failed")
			}
			if err := schemahealth.VerifyCanonicalEmailIdentity(
				t.Context(),
				collationDB,
				schemahealth.CanonicalEmailDataInvariant,
			); err != nil {
				t.Fatalf("ASCII-collated MySQL identity key was rejected: %v", err)
			}
			if err := collationDB.Exec(
				`INSERT INTO mss_boot_users (id, email, deleted_at) VALUES (?, ?, NULL)`,
				"ascii-upper-owner",
				"I@example.com",
			).Error; err == nil {
				t.Fatal("ASCII-collated identity key accepted ASCII I/i double ownership")
			}
		})
	}

	t.Run("preflight conflict leaves schema and data unchanged", func(t *testing.T) {
		resetCanonicalEmailIdentityIntegrationTables(t, db)
		createCanonicalEmailIdentityLegacyTables(t, db, dialect)
		if err := db.Exec(
			`INSERT INTO mss_boot_users (id, email, deleted_at) VALUES (?, ?, NULL), (?, ?, NULL)`,
			"first", "Person@example.com",
			"second", "person@example.com",
		).Error; err != nil {
			t.Fatal("insert canonical-email conflict fixtures failed")
		}

		err := migrateCanonicalEmailIdentities(db, canonicalEmailIntegrationVer)
		if !errors.Is(err, models.ErrEmailIdentityAmbiguous) {
			t.Fatalf("canonical-email conflict error = %v, want ambiguous identity", err)
		}
		if err != nil && strings.Contains(strings.ToLower(err.Error()), "person@example.com") {
			t.Fatal("canonical-email preflight error disclosed an identity")
		}
		var first string
		if err := db.Raw(
			"SELECT email FROM mss_boot_users WHERE id = ?",
			"first",
		).Scan(&first).Error; err != nil {
			t.Fatal("load canonical-email conflict fixture failed")
		}
		if first != "Person@example.com" {
			t.Fatalf("preflight failure mutated email to %q", first)
		}
		assertCanonicalEmailIndexAbsent(t, db, dialect)
		assertCanonicalEmailMigrationVersionCount(
			t,
			db,
			canonicalEmailIntegrationVer,
			0,
		)
	})

	t.Run("concurrent claims create exactly one owner", func(t *testing.T) {
		resetCanonicalEmailIdentityIntegrationTables(t, db)
		createCanonicalEmailIdentityLegacyTables(t, db, dialect)
		if err := migrateCanonicalEmailIdentities(db, canonicalEmailIntegrationVer); err != nil {
			t.Fatalf("prepare %s canonical-email index failed: %v", dialect, err)
		}

		start := make(chan struct{})
		errorsByClaim := make([]error, 2)
		var claims sync.WaitGroup
		claims.Add(2)
		for index, email := range []string{"Concurrent@Example.com", "concurrent@example.COM"} {
			go func(index int, email string) {
				defer claims.Done()
				<-start
				errorsByClaim[index] = db.Exec(
					`INSERT INTO mss_boot_users (id, email, deleted_at) VALUES (?, ?, NULL)`,
					fmt.Sprintf("claim-%d", index),
					email,
				).Error
			}(index, email)
		}
		close(start)
		claims.Wait()

		successes := 0
		for _, err := range errorsByClaim {
			if err == nil {
				successes++
			}
		}
		if successes != 1 {
			t.Fatalf("concurrent canonical-email successes = %d, want 1", successes)
		}
		var owners int64
		if err := db.Table(canonicalEmailIntegrationTable).
			Where("LOWER(TRIM(email)) = ? AND deleted_at IS NULL", "concurrent@example.com").
			Count(&owners).Error; err != nil {
			t.Fatal("count concurrent canonical-email owners failed")
		}
		if owners != 1 {
			t.Fatalf("concurrent canonical-email owners = %d, want 1", owners)
		}
	})
}

func openCanonicalEmailIdentityIntegrationDB(
	t *testing.T,
	dialect,
	dsn string,
) *gorm.DB {
	t.Helper()
	dialector, ok := configgormdb.Opens[dialect]
	if !ok {
		t.Fatalf("canonical-email integration dialect %q is unavailable", dialect)
	}
	db, err := gorm.Open(dialector(dsn), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		t.Fatalf("open %s canonical-email integration database failed", dialect)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("obtain %s canonical-email integration connection failed", dialect)
	}
	sqlDB.SetMaxOpenConns(8)
	sqlDB.SetMaxIdleConns(4)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	db = db.WithContext(ctx)
	if err := sqlDB.PingContext(ctx); err != nil {
		cancel()
		_ = sqlDB.Close()
		t.Fatalf("ping %s canonical-email integration database failed", dialect)
	}
	var databaseName string
	query := "SELECT current_database()"
	if dialect == "mysql" {
		query = "SELECT DATABASE()"
	}
	if err := db.Raw(query).Scan(&databaseName).Error; err != nil {
		cancel()
		_ = sqlDB.Close()
		t.Fatalf("verify %s canonical-email integration database failed", dialect)
	}
	if databaseName != canonicalEmailTestDatabaseName {
		cancel()
		_ = sqlDB.Close()
		t.Fatalf(
			"refusing destructive %s canonical-email test outside allowlisted database %q",
			dialect,
			canonicalEmailTestDatabaseName,
		)
	}

	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cleanupCancel()
		if err := dropCanonicalEmailIdentityIntegrationTables(db.WithContext(cleanupCtx)); err != nil {
			t.Errorf("clean up %s canonical-email integration tables failed", dialect)
		}
		cancel()
		if err := sqlDB.Close(); err != nil {
			t.Errorf("close %s canonical-email integration database failed", dialect)
		}
	})
	resetCanonicalEmailIdentityIntegrationTables(t, db)
	return db
}

func resetCanonicalEmailIdentityIntegrationTables(t *testing.T, db *gorm.DB) {
	t.Helper()
	if err := dropCanonicalEmailIdentityIntegrationTables(db); err != nil {
		t.Fatal("reset canonical-email integration tables failed")
	}
}

func dropCanonicalEmailIdentityIntegrationTables(db *gorm.DB) error {
	userErr := db.Exec("DROP TABLE IF EXISTS " + canonicalEmailIntegrationTable).Error
	migrationErr := db.Exec("DROP TABLE IF EXISTS " + canonicalEmailMigrationTable).Error
	return errors.Join(userErr, migrationErr)
}

func createCanonicalEmailIdentityLegacyTables(t *testing.T, db *gorm.DB, dialect string) {
	t.Helper()
	deletedAtType := "TIMESTAMP NULL"
	if dialect == "mysql" {
		deletedAtType = "DATETIME(3) NULL"
	}
	statement := fmt.Sprintf(`CREATE TABLE mss_boot_users (
		id VARCHAR(64) PRIMARY KEY,
		email VARCHAR(100) NULL,
		deleted_at %s
	)`, deletedAtType)
	if err := db.Exec(statement).Error; err != nil {
		t.Fatal("create canonical-email legacy user table failed")
	}
	if err := db.AutoMigrate(&migrationmodels.Migration{}); err != nil {
		t.Fatal("create canonical-email migration table failed")
	}
}

func createCanonicalEmailIdentityPostgresTurkishLegacyTables(t *testing.T, db *gorm.DB) {
	t.Helper()
	var available int64
	if err := db.Raw(
		`SELECT COUNT(*) FROM pg_collation WHERE collname = 'tr-x-icu'`,
	).Scan(&available).Error; err != nil {
		t.Fatal("inspect PostgreSQL Turkish ICU collation failed")
	}
	if available == 0 {
		t.Fatal("required PostgreSQL Turkish ICU collation tr-x-icu is unavailable")
	}
	if err := db.Exec(`CREATE TABLE mss_boot_users (
		id VARCHAR(64) PRIMARY KEY,
		email VARCHAR(100) COLLATE "tr-x-icu" NULL,
		deleted_at TIMESTAMP NULL
	)`).Error; err != nil {
		t.Fatal("create Turkish-collated canonical-email legacy user table failed")
	}
	if err := db.AutoMigrate(&migrationmodels.Migration{}); err != nil {
		t.Fatal("create canonical-email migration table failed")
	}
}

func createCanonicalEmailIdentityMySQLTurkishLegacyTables(t *testing.T, db *gorm.DB) {
	t.Helper()
	var available int64
	if err := db.Raw(
		`SELECT COUNT(*) FROM information_schema.COLLATIONS
		 WHERE COLLATION_NAME = 'utf8mb4_tr_0900_ai_ci'`,
	).Scan(&available).Error; err != nil {
		t.Fatal("inspect MySQL Turkish collation failed")
	}
	if available == 0 {
		t.Fatal("required MySQL Turkish collation utf8mb4_tr_0900_ai_ci is unavailable")
	}
	if err := db.Exec(`CREATE TABLE mss_boot_users (
		id VARCHAR(64) PRIMARY KEY,
		email VARCHAR(100) CHARACTER SET utf8mb4 COLLATE utf8mb4_tr_0900_ai_ci NULL,
		deleted_at DATETIME(3) NULL
	)`).Error; err != nil {
		t.Fatal("create Turkish-collated canonical-email legacy user table failed")
	}
	if err := db.AutoMigrate(&migrationmodels.Migration{}); err != nil {
		t.Fatal("create canonical-email migration table failed")
	}
}

func assertCanonicalEmailIndexAbsent(t *testing.T, db *gorm.DB, dialect string) {
	t.Helper()
	var count int64
	switch dialect {
	case "postgres":
		if err := db.Raw(
			`SELECT COUNT(*) FROM pg_indexes
			 WHERE schemaname = current_schema() AND tablename = ? AND indexname = ?`,
			canonicalEmailIntegrationTable,
			models.EmailIdentityUniqueIndex,
		).Scan(&count).Error; err != nil {
			t.Fatal("inspect absent PostgreSQL canonical-email index failed")
		}
	case "mysql":
		if err := db.Raw(
			`SELECT COUNT(*) FROM information_schema.STATISTICS
			 WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ? AND INDEX_NAME = ?`,
			canonicalEmailIntegrationTable,
			models.EmailIdentityUniqueIndex,
		).Scan(&count).Error; err != nil {
			t.Fatal("inspect absent MySQL canonical-email index failed")
		}
	default:
		t.Fatalf("unsupported canonical-email integration dialect %q", dialect)
	}
	if count != 0 {
		t.Fatalf("%s canonical-email index exists after preflight failure", dialect)
	}
}
