package system

import (
	"errors"
	"fmt"

	"github.com/mss-boot-io/mss-boot-admin/admin/models"
	"github.com/mss-boot-io/mss-boot-admin/admin/pkg/schemahealth"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/migration"
	migrationmodels "github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/migration/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/logger"
)

const (
	canonicalEmailIdentityMigrationID migration.MigrationID = migration.MigrationID(schemahealth.CanonicalEmailIdentityMigrationVersion)
)

const canonicalEmailPartialIndexDDL = "CREATE UNIQUE INDEX IF NOT EXISTS " +
	models.EmailIdentityUniqueIndex +
	" ON mss_boot_users (LOWER(TRIM(email)))" +
	" WHERE deleted_at IS NULL AND TRIM(email) <> ''"

const canonicalEmailMySQLGeneratedColumnDDL = "ALTER TABLE `mss_boot_users` " +
	"ADD COLUMN `" + schemahealth.CanonicalEmailIdentityGeneratedColumn +
	"` VARBINARY(100) GENERATED ALWAYS AS (" +
	"CASE WHEN `deleted_at` IS NULL AND TRIM(`email`) <> '' " +
	"THEN CAST(LOWER(TRIM(`email`)) AS BINARY(100)) ELSE NULL END" +
	") STORED"

const canonicalEmailMySQLUniqueIndexDDL = "CREATE UNIQUE INDEX `" +
	models.EmailIdentityUniqueIndex + "` ON `mss_boot_users` (`" +
	schemahealth.CanonicalEmailIdentityGeneratedColumn + "`)"

type canonicalEmailLegacyRow struct {
	ID    string  `gorm:"column:id"`
	Email *string `gorm:"column:email"`
}

type canonicalEmailBackfill struct {
	id        string
	original  string
	canonical string
}

type canonicalEmailPreflightError struct {
	invalidCount  int
	conflictCount int
}

func (e *canonicalEmailPreflightError) Error() string {
	return fmt.Sprintf(
		"canonical email migration preflight failed: invalid=%d conflicts=%d",
		e.invalidCount,
		e.conflictCount,
	)
}

func (e *canonicalEmailPreflightError) Unwrap() error {
	causes := make([]error, 0, 2)
	if e.invalidCount != 0 {
		causes = append(causes, models.ErrEmailIdentityInvalid)
	}
	if e.conflictCount != 0 {
		causes = append(causes, models.ErrEmailIdentityAmbiguous)
	}
	return errors.Join(causes...)
}

func init() {
	_ = migration.Migrate.Register(
		canonicalEmailIdentityMigrationID,
		_20260810120000CanonicalEmailIdentity,
	)
}

func _20260810120000CanonicalEmailIdentity(db *gorm.DB, version string) error {
	return migrateCanonicalEmailIdentities(db, version)
}

// migrateCanonicalEmailIdentities establishes one active, non-empty canonical
// email identity across all supported dialects. Existing data is read with a
// discarded logger and completely preflighted before any backfill or DDL.
func migrateCanonicalEmailIdentities(db *gorm.DB, version string) error {
	if db == nil {
		return errors.New("canonical email migration: database is nil")
	}
	dialect := db.Dialector.Name()
	switch dialect {
	case "sqlite", "postgres", "mysql":
	default:
		return fmt.Errorf("canonical email migration: unsupported database dialect %q", dialect)
	}

	quiet := db.Session(&gorm.Session{Logger: logger.Discard})
	if !quiet.Migrator().HasTable(&models.User{}) {
		return errors.New("canonical email migration: user table is missing")
	}
	if !quiet.Migrator().HasColumn(&models.User{}, "Email") ||
		!quiet.Migrator().HasColumn(&models.User{}, "DeletedAt") {
		return errors.New("canonical email migration: required user columns are missing")
	}

	if dialect == "mysql" {
		return migrateCanonicalEmailMySQL(quiet, version)
	}
	return quiet.Transaction(func(tx *gorm.DB) error {
		if dialect == "postgres" {
			if err := tx.Exec(
				`LOCK TABLE "mss_boot_users" IN SHARE ROW EXCLUSIVE MODE`,
			).Error; err != nil {
				return errors.New("canonical email migration: lock user table failed")
			}
		}
		backfills, err := prepareCanonicalEmailBackfill(tx)
		if err != nil {
			return err
		}
		if err := applyCanonicalEmailBackfill(tx, backfills); err != nil {
			return err
		}
		if err := tx.Exec(canonicalEmailPartialIndexDDL).Error; err != nil {
			return errors.New("canonical email migration: create unique index failed")
		}
		if err := schemahealth.VerifyCanonicalEmailIdentity(
			tx.Statement.Context,
			tx,
			schemahealth.CanonicalEmailDataInvariant,
		); err != nil {
			return err
		}
		return recordCanonicalEmailMigrationVersion(tx, version)
	})
}

func migrateCanonicalEmailMySQL(db *gorm.DB, version string) error {
	backfills, err := prepareCanonicalEmailBackfill(db)
	if err != nil {
		return err
	}
	if err := db.Transaction(func(tx *gorm.DB) error {
		return applyCanonicalEmailBackfill(tx, backfills)
	}); err != nil {
		return err
	}

	err = schemahealth.VerifyCanonicalEmailIdentity(
		db.Statement.Context,
		db,
		schemahealth.CanonicalEmailMySQLGeneratedColumn,
	)
	if failure, ok := schemahealth.CanonicalEmailFailureOf(err); ok &&
		failure == schemahealth.FailureGeneratedColumnMissing {
		if err := db.Exec(canonicalEmailMySQLGeneratedColumnDDL).Error; err != nil {
			return errors.New("canonical email migration: create MySQL generated column failed")
		}
	} else if err != nil {
		return err
	}

	err = schemahealth.VerifyCanonicalEmailIdentity(
		db.Statement.Context,
		db,
		schemahealth.CanonicalEmailStorageInvariant,
	)
	if failure, ok := schemahealth.CanonicalEmailFailureOf(err); ok &&
		failure == schemahealth.FailureUniqueIndexMissing {
		if err := db.Exec(canonicalEmailMySQLUniqueIndexDDL).Error; err != nil {
			return errors.New("canonical email migration: create MySQL unique index failed")
		}
	} else if err != nil {
		return err
	}
	if err := schemahealth.VerifyCanonicalEmailIdentity(
		db.Statement.Context,
		db,
		schemahealth.CanonicalEmailDataInvariant,
	); err != nil {
		return err
	}
	return recordCanonicalEmailMigrationVersion(db, version)
}

func prepareCanonicalEmailBackfill(db *gorm.DB) ([]canonicalEmailBackfill, error) {
	rows := make([]canonicalEmailLegacyRow, 0)
	if err := db.Table((&models.User{}).TableName()).
		Select("id", "email").
		Where("deleted_at IS NULL").
		Order("id ASC").
		Scan(&rows).Error; err != nil {
		return nil, errors.New("canonical email migration: preflight read failed")
	}

	backfills := make([]canonicalEmailBackfill, 0, len(rows))
	identityCounts := make(map[string]int, len(rows))
	invalidCount := 0
	for _, row := range rows {
		if row.Email == nil || *row.Email == "" {
			continue
		}
		canonical, err := models.CanonicalEmailIdentity(*row.Email)
		if err != nil {
			invalidCount++
			continue
		}
		identityCounts[canonical]++
		if canonical != *row.Email {
			backfills = append(backfills, canonicalEmailBackfill{
				id:        row.ID,
				original:  *row.Email,
				canonical: canonical,
			})
		}
	}
	conflictCount := 0
	for _, count := range identityCounts {
		if count > 1 {
			conflictCount++
		}
	}
	if invalidCount != 0 || conflictCount != 0 {
		return nil, &canonicalEmailPreflightError{
			invalidCount:  invalidCount,
			conflictCount: conflictCount,
		}
	}
	return backfills, nil
}

func applyCanonicalEmailBackfill(db *gorm.DB, backfills []canonicalEmailBackfill) error {
	for _, backfill := range backfills {
		result := db.Table((&models.User{}).TableName()).
			Where(
				"id = ? AND deleted_at IS NULL AND email = ?",
				backfill.id,
				backfill.original,
			).
			UpdateColumn("email", backfill.canonical)
		if result.Error != nil || result.RowsAffected != 1 {
			return errors.New("canonical email migration: backfill failed")
		}
	}
	return nil
}

func recordCanonicalEmailMigrationVersion(db *gorm.DB, version string) error {
	row := &migrationmodels.Migration{}
	row.SetVersion(version)
	if err := db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "version"}},
		DoNothing: true,
	}).Create(row).Error; err != nil {
		return errors.New("canonical email migration: record version failed")
	}
	return nil
}
