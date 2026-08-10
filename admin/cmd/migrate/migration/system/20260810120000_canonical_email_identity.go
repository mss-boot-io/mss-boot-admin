package system

import (
	"errors"
	"fmt"
	"strings"

	"github.com/mss-boot-io/mss-boot-admin/admin/models"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/migration"
	migrationmodels "github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/migration/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/logger"
)

const (
	canonicalEmailIdentityMigrationID migration.MigrationID = "20260810120000"
	canonicalEmailGeneratedColumn                           = "email_identity_key"
)

const canonicalEmailPartialIndexDDL = "CREATE UNIQUE INDEX IF NOT EXISTS " +
	models.EmailIdentityUniqueIndex +
	" ON mss_boot_users (LOWER(TRIM(email)))" +
	" WHERE deleted_at IS NULL AND TRIM(email) <> ''"

const canonicalEmailMySQLGeneratedColumnDDL = "ALTER TABLE `mss_boot_users` " +
	"ADD COLUMN `email_identity_key` VARBINARY(100) GENERATED ALWAYS AS (" +
	"CASE WHEN `deleted_at` IS NULL AND TRIM(`email`) <> '' " +
	"THEN CAST(LOWER(TRIM(`email`)) AS BINARY(100)) ELSE NULL END" +
	") STORED"

const canonicalEmailMySQLUniqueIndexDDL = "CREATE UNIQUE INDEX `" +
	models.EmailIdentityUniqueIndex + "` ON `mss_boot_users` (`email_identity_key`)"

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

type canonicalEmailIndexMetadata struct {
	definition           string
	dataType             string
	nullable             string
	extra                string
	generationExpression string
	indexColumns         []canonicalEmailIndexColumn
}

type canonicalEmailIndexColumn struct {
	name      string
	sequence  int
	nonUnique bool
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
		if err := verifyCanonicalEmailIndex(tx, dialect); err != nil {
			return err
		}
		if err := verifyCanonicalEmailData(tx); err != nil {
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

	metadata, columnExists, err := inspectCanonicalEmailMySQLColumn(db)
	if err != nil {
		return err
	}
	if columnExists {
		if err := validateCanonicalEmailIndexMetadata("mysql-column", metadata); err != nil {
			return err
		}
	} else if err := db.Exec(canonicalEmailMySQLGeneratedColumnDDL).Error; err != nil {
		return errors.New("canonical email migration: create MySQL generated column failed")
	}

	metadata, indexExists, err := inspectCanonicalEmailMySQLIndex(db, metadata)
	if err != nil {
		return err
	}
	if indexExists {
		if err := validateCanonicalEmailIndexMetadata("mysql", metadata); err != nil {
			return err
		}
	} else if err := db.Exec(canonicalEmailMySQLUniqueIndexDDL).Error; err != nil {
		return errors.New("canonical email migration: create MySQL unique index failed")
	}
	if err := verifyCanonicalEmailIndex(db, "mysql"); err != nil {
		return err
	}
	if err := verifyCanonicalEmailData(db); err != nil {
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

func verifyCanonicalEmailData(db *gorm.DB) error {
	backfills, err := prepareCanonicalEmailBackfill(db)
	if err != nil {
		return err
	}
	if len(backfills) != 0 {
		return fmt.Errorf(
			"canonical email migration: data verification failed: noncanonical=%d: %w",
			len(backfills),
			models.ErrEmailIdentityInvalid,
		)
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

func verifyCanonicalEmailIndex(db *gorm.DB, dialect string) error {
	var metadata canonicalEmailIndexMetadata
	switch dialect {
	case "sqlite":
		var definition string
		result := db.Raw(
			"SELECT sql FROM sqlite_master WHERE type = 'index' AND name = ?",
			models.EmailIdentityUniqueIndex,
		).Scan(&definition)
		if result.Error != nil || result.RowsAffected != 1 {
			return errors.New("canonical email migration: inspect SQLite index failed")
		}
		metadata.definition = definition
	case "postgres":
		var definition string
		result := db.Raw(
			`SELECT indexdef FROM pg_indexes
			 WHERE schemaname = current_schema() AND tablename = ? AND indexname = ?`,
			(&models.User{}).TableName(),
			models.EmailIdentityUniqueIndex,
		).Scan(&definition)
		if result.Error != nil || result.RowsAffected != 1 {
			return errors.New("canonical email migration: inspect PostgreSQL index failed")
		}
		metadata.definition = definition
	case "mysql":
		var err error
		var exists bool
		metadata, exists, err = inspectCanonicalEmailMySQLColumn(db)
		if err != nil || !exists {
			return errors.New("canonical email migration: inspect MySQL generated column failed")
		}
		metadata, exists, err = inspectCanonicalEmailMySQLIndex(db, metadata)
		if err != nil || !exists {
			return errors.New("canonical email migration: inspect MySQL unique index failed")
		}
	default:
		return fmt.Errorf("canonical email migration: unsupported verifier dialect %q", dialect)
	}
	return validateCanonicalEmailIndexMetadata(dialect, metadata)
}

func inspectCanonicalEmailMySQLColumn(
	db *gorm.DB,
) (canonicalEmailIndexMetadata, bool, error) {
	var row struct {
		DataType             string `gorm:"column:DATA_TYPE"`
		IsNullable           string `gorm:"column:IS_NULLABLE"`
		Extra                string `gorm:"column:EXTRA"`
		GenerationExpression string `gorm:"column:GENERATION_EXPRESSION"`
	}
	result := db.Raw(
		`SELECT DATA_TYPE, IS_NULLABLE, EXTRA, GENERATION_EXPRESSION
		 FROM information_schema.COLUMNS
		 WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ? AND COLUMN_NAME = ?`,
		(&models.User{}).TableName(),
		canonicalEmailGeneratedColumn,
	).Scan(&row)
	if result.Error != nil {
		return canonicalEmailIndexMetadata{}, false,
			errors.New("canonical email migration: inspect MySQL generated column failed")
	}
	if result.RowsAffected == 0 {
		return canonicalEmailIndexMetadata{}, false, nil
	}
	return canonicalEmailIndexMetadata{
		dataType:             row.DataType,
		nullable:             row.IsNullable,
		extra:                row.Extra,
		generationExpression: row.GenerationExpression,
	}, true, nil
}

func inspectCanonicalEmailMySQLIndex(
	db *gorm.DB,
	metadata canonicalEmailIndexMetadata,
) (canonicalEmailIndexMetadata, bool, error) {
	rows := make([]struct {
		ColumnName string `gorm:"column:COLUMN_NAME"`
		SeqInIndex int    `gorm:"column:SEQ_IN_INDEX"`
		NonUnique  int    `gorm:"column:NON_UNIQUE"`
	}, 0, 1)
	result := db.Raw(
		`SELECT COLUMN_NAME, SEQ_IN_INDEX, NON_UNIQUE
		 FROM information_schema.STATISTICS
		 WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ? AND INDEX_NAME = ?
		 ORDER BY SEQ_IN_INDEX`,
		(&models.User{}).TableName(),
		models.EmailIdentityUniqueIndex,
	).Scan(&rows)
	if result.Error != nil {
		return metadata, false,
			errors.New("canonical email migration: inspect MySQL unique index failed")
	}
	metadata.indexColumns = metadata.indexColumns[:0]
	for _, row := range rows {
		metadata.indexColumns = append(metadata.indexColumns, canonicalEmailIndexColumn{
			name:      row.ColumnName,
			sequence:  row.SeqInIndex,
			nonUnique: row.NonUnique != 0,
		})
	}
	return metadata, len(rows) != 0, nil
}

func validateCanonicalEmailIndexMetadata(
	dialect string,
	metadata canonicalEmailIndexMetadata,
) error {
	switch dialect {
	case "sqlite", "postgres":
		definition := normalizeEmailIndexDefinition(metadata.definition)
		required := []string{
			"createuniqueindex",
			strings.ToLower(models.EmailIdentityUniqueIndex),
			"lower(",
			"trim(",
			"email",
			"where",
			"deleted_atisnull",
			"<>''",
		}
		for _, token := range required {
			if !strings.Contains(definition, token) {
				return fmt.Errorf(
					"canonical email migration: %s index metadata is incompatible",
					dialect,
				)
			}
		}
		if strings.Contains(definition, "deleted_atisnotnull") {
			return fmt.Errorf(
				"canonical email migration: %s index metadata is incompatible",
				dialect,
			)
		}
		return nil
	case "mysql-column", "mysql":
		generation := normalizeEmailIndexDefinition(metadata.generationExpression)
		if !strings.EqualFold(metadata.dataType, "varbinary") ||
			!strings.EqualFold(metadata.nullable, "YES") ||
			!strings.Contains(strings.ToLower(metadata.extra), "stored generated") ||
			!strings.Contains(generation, "deleted_atisnull") ||
			!strings.Contains(generation, "lower(") ||
			!strings.Contains(generation, "trim(") ||
			!strings.Contains(generation, "email") ||
			!strings.Contains(generation, "<>") {
			return errors.New("canonical email migration: MySQL generated column metadata is incompatible")
		}
		if dialect == "mysql-column" {
			return nil
		}
		if len(metadata.indexColumns) != 1 ||
			metadata.indexColumns[0].name != canonicalEmailGeneratedColumn ||
			metadata.indexColumns[0].sequence != 1 ||
			metadata.indexColumns[0].nonUnique {
			return errors.New("canonical email migration: MySQL unique index metadata is incompatible")
		}
		return nil
	default:
		return fmt.Errorf("canonical email migration: unsupported metadata dialect %q", dialect)
	}
}

func normalizeEmailIndexDefinition(value string) string {
	replacer := strings.NewReplacer(
		" ", "",
		"\t", "",
		"\n", "",
		"\r", "",
		"`", "",
		"\"", "",
	)
	return strings.ToLower(replacer.Replace(value))
}
