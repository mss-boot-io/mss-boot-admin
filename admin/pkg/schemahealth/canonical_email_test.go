package schemahealth

import (
	"bytes"
	"errors"
	"log"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mss-boot-io/mss-boot-admin/admin/models"
	migrationmodels "github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/migration/models"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"gorm.io/plugin/dbresolver"
)

func TestVerifyCanonicalEmailIdentitySQLiteRuntimeReady(t *testing.T) {
	db := openCanonicalEmailSchemaTestDB(t, logger.Discard)
	createCanonicalEmailRuntimeSchema(t, db)

	if err := VerifyCanonicalEmailIdentity(
		t.Context(),
		db,
		CanonicalEmailRuntimeReadiness,
	); err != nil {
		t.Fatalf("runtime readiness rejected compatible schema: %v", err)
	}
}

func TestVerifyCanonicalEmailIdentityUsesDBResolverWriter(t *testing.T) {
	t.Run("writer ready while replica data is stale", func(t *testing.T) {
		writer, replica := openCanonicalEmailSchemaResolverTestDB(t)
		createCanonicalEmailRuntimeSchema(t, writer)
		createCanonicalEmailRuntimeSchema(t, replica)
		const staleReplicaEmail = " Stale.Replica@EXAMPLE.COM "
		if err := replica.Exec(
			"INSERT INTO mss_boot_users (id, email) VALUES (?, ?)",
			"stale-replica-row",
			staleReplicaEmail,
		).Error; err != nil {
			t.Fatal(err)
		}
		registerCanonicalEmailSchemaReplica(t, writer, replica)

		var ordinaryRead string
		if err := writer.Raw(
			"SELECT email FROM mss_boot_users WHERE id = ?",
			"stale-replica-row",
		).Scan(&ordinaryRead).Error; err != nil {
			t.Fatal(err)
		}
		if ordinaryRead != staleReplicaEmail {
			t.Fatalf("ordinary read = %q, want stale replica evidence", ordinaryRead)
		}
		if err := VerifyCanonicalEmailIdentity(
			t.Context(),
			writer,
			CanonicalEmailRuntimeReadiness,
		); err != nil {
			t.Fatalf("writer-ready schema rejected because replica is stale: %v", err)
		}
	})

	t.Run("writer marker missing while replica appears ready", func(t *testing.T) {
		writer, replica := openCanonicalEmailSchemaResolverTestDB(t)
		createCanonicalEmailRuntimeSchema(t, writer)
		createCanonicalEmailRuntimeSchema(t, replica)
		if err := writer.Where(
			"version = ?",
			CanonicalEmailIdentityMigrationVersion,
		).Delete(&migrationmodels.Migration{}).Error; err != nil {
			t.Fatal(err)
		}
		registerCanonicalEmailSchemaReplica(t, writer, replica)

		var ordinaryCount int64
		if err := writer.Model(&migrationmodels.Migration{}).
			Where("version = ?", CanonicalEmailIdentityMigrationVersion).
			Count(&ordinaryCount).Error; err != nil {
			t.Fatal(err)
		}
		if ordinaryCount != 1 {
			t.Fatalf("ordinary marker count = %d, want ready replica evidence", ordinaryCount)
		}
		err := VerifyCanonicalEmailIdentity(
			t.Context(),
			writer,
			CanonicalEmailRuntimeReadiness,
		)
		assertCanonicalEmailFailure(t, err, FailureMigrationVersionMissing)
	})
}

func TestVerifyCanonicalEmailIdentityRejectsMissingAndWrongSQLiteIndex(t *testing.T) {
	tests := []struct {
		name     string
		indexDDL string
		failure  CanonicalEmailFailure
	}{
		{
			name:    "missing",
			failure: FailureUniqueIndexMissing,
		},
		{
			name: "ordinary email uniqueness",
			indexDDL: "CREATE UNIQUE INDEX " + models.EmailIdentityUniqueIndex +
				" ON mss_boot_users (email)",
			failure: FailureUniqueIndexWrong,
		},
		{
			name: "wrong active predicate",
			indexDDL: "CREATE UNIQUE INDEX " + models.EmailIdentityUniqueIndex +
				" ON mss_boot_users (LOWER(TRIM(email)))" +
				" WHERE deleted_at IS NOT NULL AND TRIM(email) <> ''",
			failure: FailureUniqueIndexWrong,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			db := openCanonicalEmailSchemaTestDB(t, logger.Discard)
			createCanonicalEmailUserTable(t, db)
			if test.indexDDL != "" {
				if err := db.Exec(test.indexDDL).Error; err != nil {
					t.Fatal(err)
				}
			}

			err := VerifyCanonicalEmailIdentity(
				t.Context(),
				db,
				CanonicalEmailStorageInvariant,
			)
			assertCanonicalEmailFailure(t, err, test.failure)
		})
	}
}

func TestVerifyCanonicalEmailIdentityRequiresExactMigrationVersion(t *testing.T) {
	db := openCanonicalEmailSchemaTestDB(t, logger.Discard)
	createCanonicalEmailUserTable(t, db)
	createCanonicalEmailSQLiteIndex(t, db)
	if err := db.AutoMigrate(&migrationmodels.Migration{}); err != nil {
		t.Fatal(err)
	}
	wrong := &migrationmodels.Migration{}
	wrong.SetVersion(CanonicalEmailIdentityMigrationVersion + "-truncated")
	if err := db.Create(wrong).Error; err != nil {
		t.Fatal(err)
	}

	err := VerifyCanonicalEmailIdentity(
		t.Context(),
		db,
		CanonicalEmailRuntimeReadiness,
	)
	assertCanonicalEmailFailure(t, err, FailureMigrationVersionMissing)
}

func TestVerifyCanonicalEmailIdentityRejectsSensitiveDataWithoutDisclosure(t *testing.T) {
	var logs bytes.Buffer
	observedLogger := logger.New(
		log.New(&logs, "", 0),
		logger.Config{LogLevel: logger.Info, Colorful: false},
	)
	db := openCanonicalEmailSchemaTestDB(t, observedLogger)
	createCanonicalEmailUserTable(t, db)
	createCanonicalEmailSQLiteIndex(t, db)
	const sensitive = " Sensitive.Invalid@EXAMPLE.COM "
	if err := db.Exec(
		"INSERT INTO mss_boot_users (id, email) VALUES (?, ?)",
		"sensitive-invalid-row",
		sensitive,
	).Error; err != nil {
		t.Fatal(err)
	}
	logs.Reset()

	err := VerifyCanonicalEmailIdentity(
		t.Context(),
		db,
		CanonicalEmailDataInvariant,
	)
	assertCanonicalEmailFailure(t, err, FailureIdentityDataInvalid)
	if !errors.Is(err, models.ErrEmailIdentityInvalid) {
		t.Fatalf("data error = %v, want invalid identity classification", err)
	}
	combined := logs.String() + err.Error()
	for _, forbidden := range []string{
		sensitive,
		"sensitive-invalid-row",
		"SELECT",
		models.EmailIdentityUniqueIndex,
	} {
		if strings.Contains(combined, forbidden) {
			t.Fatalf("readiness failure disclosed %q: %s", forbidden, combined)
		}
	}

	// Ambiguity is also detected independently of the index. This protects the
	// migration preflight and any future dialect implementation from assuming
	// metadata alone proves existing data is unambiguous.
	db = openCanonicalEmailSchemaTestDB(t, logger.Discard)
	createCanonicalEmailUserTable(t, db)
	for id, email := range map[string]string{
		"sensitive-owner-a": "owner@example.com",
		"sensitive-owner-b": "owner@example.com",
	} {
		if err := db.Exec(
			"INSERT INTO mss_boot_users (id, email) VALUES (?, ?)",
			id,
			email,
		).Error; err != nil {
			t.Fatal(err)
		}
	}
	err = verifyCanonicalEmailData(db.Session(&gorm.Session{Logger: logger.Discard}))
	assertCanonicalEmailFailure(t, err, FailureIdentityDataAmbiguous)
	if !errors.Is(err, models.ErrEmailIdentityAmbiguous) ||
		strings.Contains(err.Error(), "owner@example.com") {
		t.Fatalf("ambiguous data error is unsafe or untyped: %v", err)
	}
}

func TestVerifyCanonicalEmailIdentityDatabaseFailureIsFixedAndRedacted(t *testing.T) {
	db := openCanonicalEmailSchemaTestDB(t, logger.Discard)
	createCanonicalEmailRuntimeSchema(t, db)
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	if err := sqlDB.Close(); err != nil {
		t.Fatal(err)
	}

	err = VerifyCanonicalEmailIdentity(
		t.Context(),
		db,
		CanonicalEmailRuntimeReadiness,
	)
	assertCanonicalEmailFailure(t, err, FailureInspection)
	if err.Error() != "canonical email identity is not ready: database inspection failed" {
		t.Fatalf("database failure = %q, want fixed redacted error", err)
	}
}

func TestCanonicalEmailMetadataCompatibilityRejectsWrongPostgresAndMySQLShapes(t *testing.T) {
	postgres := canonicalEmailIndexMetadata{
		predicate:       "((deleted_at IS NULL) AND (TRIM(BOTH FROM email) <> ''::text))",
		expression:      `lower((TRIM(BOTH FROM email) COLLATE "C"))`,
		collationName:   "C",
		collationSchema: "pg_catalog",
		unique:          true,
		valid:           true,
		ready:           true,
		keyColumns:      1,
		totalColumns:    1,
	}
	if !canonicalEmailPartialIndexCompatible("postgres", postgres) {
		t.Fatal("compatible PostgreSQL expression index metadata was rejected")
	}
	postgres.predicate = "deleted_at IS NOT NULL AND TRIM(email) <> ''"
	if canonicalEmailPartialIndexCompatible("postgres", postgres) {
		t.Fatal("wrong PostgreSQL predicate was accepted")
	}
	postgres.predicate = "deleted_at IS NULL AND TRIM(email) <> ''"
	postgres.collationName = "tr-x-icu"
	postgres.collationSchema = "pg_catalog"
	if canonicalEmailPartialIndexCompatible("postgres", postgres) {
		t.Fatal("locale-sensitive PostgreSQL collation was accepted")
	}

	mysql := canonicalEmailIndexMetadata{
		dataType:             "varbinary",
		maximumLength:        100,
		nullable:             "YES",
		extra:                "STORED GENERATED",
		generationExpression: "(case when ((`deleted_at` is null) and (trim(`email`) <> _latin1\\'\\')) then cast(lower((convert(trim(`email`) using ascii) collate ascii_general_ci)) as char(100) charset binary) else NULL end)",
		indexColumns: []canonicalEmailIndexColumn{{
			name:     CanonicalEmailIdentityGeneratedColumn,
			sequence: 1,
		}},
	}
	if !canonicalEmailMySQLColumnCompatible(mysql) ||
		!canonicalEmailMySQLIndexCompatible(mysql) {
		t.Fatal("compatible MySQL generated-column index metadata was rejected")
	}
	mysql.maximumLength = 99
	if canonicalEmailMySQLColumnCompatible(mysql) {
		t.Fatal("wrong MySQL generated-column width was accepted")
	}
	mysql.maximumLength = 100
	mysql.generationExpression = "case when deleted_at is null and trim(email) <> '' then cast(lower(trim(email)) as binary(100)) else null end"
	if canonicalEmailMySQLColumnCompatible(mysql) {
		t.Fatal("locale-sensitive MySQL generated expression was accepted")
	}
	mysql.generationExpression = "case when deleted_at is null and trim(email) <> '' then cast(lower(convert(trim(email) using ascii) collate ascii_general_ci) as binary(100)) else null end"
	mysql.indexColumns[0].nonUnique = true
	if canonicalEmailMySQLIndexCompatible(mysql) {
		t.Fatal("non-unique MySQL identity index was accepted")
	}
}

func openCanonicalEmailSchemaTestDB(t *testing.T, databaseLogger logger.Interface) *gorm.DB {
	t.Helper()
	dsn := filepath.Join(t.TempDir(), "canonical-email-schema.db") + "?_busy_timeout=5000"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: databaseLogger})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	return db
}

func openCanonicalEmailSchemaResolverTestDB(t *testing.T) (*gorm.DB, *gorm.DB) {
	t.Helper()
	directory := t.TempDir()
	open := func(name string) *gorm.DB {
		dsn := filepath.Join(directory, name) + "?_busy_timeout=5000"
		db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Discard})
		if err != nil {
			t.Fatal(err)
		}
		sqlDB, err := db.DB()
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = sqlDB.Close() })
		return db
	}
	return open("writer.db"), open("replica.db")
}

func registerCanonicalEmailSchemaReplica(t *testing.T, writer, replica *gorm.DB) {
	t.Helper()
	replicaSQL, err := replica.DB()
	if err != nil {
		t.Fatal(err)
	}
	if err := writer.Use(dbresolver.Register(dbresolver.Config{
		Replicas: []gorm.Dialector{sqlite.Dialector{Conn: replicaSQL}},
	})); err != nil {
		t.Fatal(err)
	}
}

func createCanonicalEmailRuntimeSchema(t *testing.T, db *gorm.DB) {
	t.Helper()
	createCanonicalEmailUserTable(t, db)
	createCanonicalEmailSQLiteIndex(t, db)
	if err := db.AutoMigrate(&migrationmodels.Migration{}); err != nil {
		t.Fatal(err)
	}
	version := &migrationmodels.Migration{}
	version.SetVersion(CanonicalEmailIdentityMigrationVersion)
	if err := db.Create(version).Error; err != nil {
		t.Fatal(err)
	}
}

func createCanonicalEmailUserTable(t *testing.T, db *gorm.DB) {
	t.Helper()
	if err := db.Exec(`CREATE TABLE mss_boot_users (
		id TEXT PRIMARY KEY,
		email TEXT NULL,
		deleted_at DATETIME NULL
	)`).Error; err != nil {
		t.Fatal(err)
	}
}

func createCanonicalEmailSQLiteIndex(t *testing.T, db *gorm.DB) {
	t.Helper()
	if err := db.Exec(
		"CREATE UNIQUE INDEX " + models.EmailIdentityUniqueIndex +
			" ON mss_boot_users (LOWER(TRIM(email)))" +
			" WHERE deleted_at IS NULL AND TRIM(email) <> ''",
	).Error; err != nil {
		t.Fatal(err)
	}
}

func assertCanonicalEmailFailure(
	t *testing.T,
	err error,
	want CanonicalEmailFailure,
) {
	t.Helper()
	if !errors.Is(err, ErrCanonicalEmailIdentityNotReady) {
		t.Fatalf("readiness error = %v, want ErrCanonicalEmailIdentityNotReady", err)
	}
	got, ok := CanonicalEmailFailureOf(err)
	if !ok || got != want {
		t.Fatalf("readiness failure = %q, %v; want %q", got, ok, want)
	}
}
