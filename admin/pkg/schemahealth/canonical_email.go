package schemahealth

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/mss-boot-io/mss-boot-admin/admin/models"
	migrationmodels "github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/migration/models"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"gorm.io/plugin/dbresolver"
)

const (
	// CanonicalEmailIdentityMigrationVersion is the complete migration ID that
	// establishes the active canonical-email invariant.
	CanonicalEmailIdentityMigrationVersion = "20260810120000"
	// CanonicalEmailIdentityGeneratedColumn is the MySQL generated key used to
	// emulate a partial expression index.
	CanonicalEmailIdentityGeneratedColumn = "email_identity_key"
)

// CanonicalEmailVerificationScope selects one phase of the single canonical
// email verifier. Migrations use the narrower scopes before recording their
// version; the server uses RuntimeReadiness before mounting business routes.
type CanonicalEmailVerificationScope uint8

const (
	CanonicalEmailMySQLGeneratedColumn CanonicalEmailVerificationScope = iota + 1
	CanonicalEmailStorageInvariant
	CanonicalEmailDataInvariant
	CanonicalEmailRuntimeReadiness
)

// CanonicalEmailFailure identifies a fixed, non-sensitive readiness failure.
type CanonicalEmailFailure string

const (
	FailureInvalidInput            CanonicalEmailFailure = "invalid verifier input"
	FailureUnsupportedDatabase     CanonicalEmailFailure = "unsupported database"
	FailureInspection              CanonicalEmailFailure = "database inspection failed"
	FailureUserTableMissing        CanonicalEmailFailure = "user table is missing"
	FailureUserColumnsMissing      CanonicalEmailFailure = "required user columns are missing"
	FailureGeneratedColumnMissing  CanonicalEmailFailure = "generated identity column is missing"
	FailureGeneratedColumnWrong    CanonicalEmailFailure = "generated identity column is incompatible"
	FailureUniqueIndexMissing      CanonicalEmailFailure = "unique identity index is missing"
	FailureUniqueIndexWrong        CanonicalEmailFailure = "unique identity index is incompatible"
	FailureIdentityDataInvalid     CanonicalEmailFailure = "canonical identity data is invalid"
	FailureIdentityDataAmbiguous   CanonicalEmailFailure = "canonical identity data is ambiguous"
	FailureMigrationTableMissing   CanonicalEmailFailure = "migration table is missing"
	FailureMigrationVersionMissing CanonicalEmailFailure = "canonical identity migration is not recorded"
)

var ErrCanonicalEmailIdentityNotReady = errors.New("canonical email identity is not ready")

var canonicalEmailMySQLCharsetLiteral = regexp.MustCompile(`_[a-z0-9]+''`)

// CanonicalEmailError contains only an enumerated safe failure. Driver errors,
// SQL text, constraint names, record IDs, and email values are never retained.
type CanonicalEmailError struct {
	failure CanonicalEmailFailure
	causes  []error
}

func (e *CanonicalEmailError) Error() string {
	return fmt.Sprintf("%s: %s", ErrCanonicalEmailIdentityNotReady, e.failure)
}

func (e *CanonicalEmailError) Unwrap() []error {
	return append([]error{ErrCanonicalEmailIdentityNotReady}, e.causes...)
}

func (e *CanonicalEmailError) Failure() CanonicalEmailFailure {
	return e.failure
}

// CanonicalEmailFailureOf returns the safe reason code carried by err.
func CanonicalEmailFailureOf(err error) (CanonicalEmailFailure, bool) {
	var readinessErr *CanonicalEmailError
	if !errors.As(err, &readinessErr) {
		return "", false
	}
	return readinessErr.Failure(), true
}

func canonicalEmailFailure(failure CanonicalEmailFailure, causes ...error) error {
	return &CanonicalEmailError{failure: failure, causes: causes}
}

// CanonicalEmailWriter pins canonical-email migration and readiness work to
// the DBResolver source. Schema and identity checks must never accept a
// lagging replica as evidence that the writer is ready.
func CanonicalEmailWriter(db *gorm.DB) *gorm.DB {
	if db == nil {
		return nil
	}
	// Clauses returns a chain instance. Turn it back into a reusable session so
	// callers can safely perform more than one inspection without carrying SQL
	// or clauses from the preceding GORM statement.
	return db.Clauses(dbresolver.Write).Session(&gorm.Session{})
}

type canonicalEmailIndexMetadata struct {
	definition           string
	predicate            string
	expression           string
	collationName        string
	collationSchema      string
	dataType             string
	maximumLength        int64
	nullable             string
	extra                string
	generationExpression string
	unique               bool
	valid                bool
	ready                bool
	partial              bool
	keyColumns           int
	totalColumns         int
	indexColumns         []canonicalEmailIndexColumn
}

type canonicalEmailIndexColumn struct {
	name      string
	sequence  int
	nonUnique bool
	subPart   *int64
}

// VerifyCanonicalEmailIdentity is the only structural/data verifier used by
// canonical-email migrations and server startup. Every query runs with a
// discarded logger, is pinned to the DBResolver writer, and converts every
// database failure to a fixed error.
func VerifyCanonicalEmailIdentity(
	ctx context.Context,
	db *gorm.DB,
	scope CanonicalEmailVerificationScope,
) error {
	if ctx == nil || db == nil {
		return canonicalEmailFailure(FailureInvalidInput)
	}
	if err := ctx.Err(); err != nil {
		return canonicalEmailFailure(FailureInspection)
	}
	if scope < CanonicalEmailMySQLGeneratedColumn || scope > CanonicalEmailRuntimeReadiness {
		return canonicalEmailFailure(FailureInvalidInput)
	}

	dialect := db.Dialector.Name()
	if dialect != "sqlite" && dialect != "postgres" && dialect != "mysql" {
		return canonicalEmailFailure(FailureUnsupportedDatabase)
	}
	if scope == CanonicalEmailMySQLGeneratedColumn && dialect != "mysql" {
		return canonicalEmailFailure(FailureInvalidInput)
	}
	quiet := CanonicalEmailWriter(
		db.Session(&gorm.Session{Logger: logger.Discard}),
	).WithContext(ctx)
	if err := verifyCanonicalEmailUserStorage(quiet, dialect); err != nil {
		return err
	}

	if dialect == "mysql" {
		metadata, exists, err := inspectCanonicalEmailMySQLColumn(quiet)
		if err != nil {
			return err
		}
		if !exists {
			return canonicalEmailFailure(FailureGeneratedColumnMissing)
		}
		if !canonicalEmailMySQLColumnCompatible(metadata) {
			return canonicalEmailFailure(FailureGeneratedColumnWrong)
		}
		if scope == CanonicalEmailMySQLGeneratedColumn {
			return nil
		}
		metadata, exists, err = inspectCanonicalEmailMySQLIndex(quiet, metadata)
		if err != nil {
			return err
		}
		if !exists {
			return canonicalEmailFailure(FailureUniqueIndexMissing)
		}
		if !canonicalEmailMySQLIndexCompatible(metadata) {
			return canonicalEmailFailure(FailureUniqueIndexWrong)
		}
	} else {
		metadata, exists, err := inspectCanonicalEmailPartialIndex(quiet, dialect)
		if err != nil {
			return err
		}
		if !exists {
			return canonicalEmailFailure(FailureUniqueIndexMissing)
		}
		if !canonicalEmailPartialIndexCompatible(dialect, metadata) {
			return canonicalEmailFailure(FailureUniqueIndexWrong)
		}
	}

	if scope == CanonicalEmailStorageInvariant {
		return nil
	}
	if err := verifyCanonicalEmailData(quiet); err != nil {
		return err
	}
	if scope == CanonicalEmailDataInvariant {
		return nil
	}
	return verifyCanonicalEmailMigrationRecord(quiet)
}

func inspectCanonicalEmailPartialIndex(
	db *gorm.DB,
	dialect string,
) (canonicalEmailIndexMetadata, bool, error) {
	switch dialect {
	case "sqlite":
		rows := make([]struct {
			Name    string `gorm:"column:name"`
			Unique  int    `gorm:"column:unique"`
			Partial int    `gorm:"column:partial"`
		}, 0)
		if err := db.Raw("PRAGMA index_list('mss_boot_users')").Scan(&rows).Error; err != nil {
			return canonicalEmailIndexMetadata{}, false, canonicalEmailFailure(FailureInspection)
		}
		var metadata canonicalEmailIndexMetadata
		found := false
		for _, row := range rows {
			if row.Name != models.EmailIdentityUniqueIndex {
				continue
			}
			if found {
				return canonicalEmailIndexMetadata{}, false, canonicalEmailFailure(FailureUniqueIndexWrong)
			}
			found = true
			metadata.unique = row.Unique == 1
			metadata.partial = row.Partial == 1
		}
		if !found {
			return canonicalEmailIndexMetadata{}, false, nil
		}
		var definition string
		result := db.Raw(
			"SELECT sql FROM sqlite_master WHERE type = 'index' AND name = ?",
			models.EmailIdentityUniqueIndex,
		).Scan(&definition)
		if result.Error != nil || result.RowsAffected != 1 {
			return canonicalEmailIndexMetadata{}, false, canonicalEmailFailure(FailureInspection)
		}
		metadata.definition = definition
		xinfo := make([]struct {
			CID int `gorm:"column:cid"`
			Key int `gorm:"column:key"`
		}, 0)
		if err := db.Raw("PRAGMA index_xinfo('" + models.EmailIdentityUniqueIndex + "')").
			Scan(&xinfo).Error; err != nil {
			return canonicalEmailIndexMetadata{}, false, canonicalEmailFailure(FailureInspection)
		}
		for _, row := range xinfo {
			if row.Key == 1 {
				metadata.keyColumns++
				if row.CID != -2 {
					metadata.definition = ""
				}
			}
		}
		return metadata, true, nil
	case "postgres":
		var row struct {
			Definition      string `gorm:"column:definition"`
			Predicate       string `gorm:"column:predicate"`
			Expression      string `gorm:"column:expression"`
			CollationName   string `gorm:"column:collation_name"`
			CollationSchema string `gorm:"column:collation_schema"`
			Unique          bool   `gorm:"column:is_unique"`
			Valid           bool   `gorm:"column:is_valid"`
			Ready           bool   `gorm:"column:is_ready"`
			KeyColumns      int    `gorm:"column:key_columns"`
			TotalColumns    int    `gorm:"column:total_columns"`
		}
		result := db.Raw(
			`SELECT pg_get_indexdef(index_class.oid) AS definition,
			        pg_get_expr(index_meta.indpred, index_meta.indrelid) AS predicate,
			        pg_get_expr(index_meta.indexprs, index_meta.indrelid) AS expression,
			        index_collation.collname AS collation_name,
			        collation_namespace.nspname AS collation_schema,
			        index_meta.indisunique AS is_unique,
			        index_meta.indisvalid AS is_valid,
			        index_meta.indisready AS is_ready,
			        index_meta.indnkeyatts AS key_columns,
			        index_meta.indnatts AS total_columns
			 FROM pg_class AS table_class
			 JOIN pg_namespace AS namespace ON namespace.oid = table_class.relnamespace
			 JOIN pg_index AS index_meta ON index_meta.indrelid = table_class.oid
			 JOIN pg_class AS index_class ON index_class.oid = index_meta.indexrelid
			 LEFT JOIN pg_collation AS index_collation
			   ON index_collation.oid = index_meta.indcollation[0]
			 LEFT JOIN pg_namespace AS collation_namespace
			   ON collation_namespace.oid = index_collation.collnamespace
			 WHERE namespace.nspname = current_schema()
			   AND table_class.relname = ? AND index_class.relname = ?`,
			(&models.User{}).TableName(),
			models.EmailIdentityUniqueIndex,
		).Scan(&row)
		if result.Error != nil {
			return canonicalEmailIndexMetadata{}, false, canonicalEmailFailure(FailureInspection)
		}
		if result.RowsAffected == 0 {
			return canonicalEmailIndexMetadata{}, false, nil
		}
		return canonicalEmailIndexMetadata{
			definition:      row.Definition,
			predicate:       row.Predicate,
			expression:      row.Expression,
			collationName:   row.CollationName,
			collationSchema: row.CollationSchema,
			unique:          row.Unique,
			valid:           row.Valid,
			ready:           row.Ready,
			keyColumns:      row.KeyColumns,
			totalColumns:    row.TotalColumns,
		}, true, nil
	default:
		return canonicalEmailIndexMetadata{}, false, canonicalEmailFailure(FailureUnsupportedDatabase)
	}
}

func verifyCanonicalEmailUserStorage(db *gorm.DB, dialect string) error {
	exists, err := canonicalEmailTableExists(db, dialect, (&models.User{}).TableName())
	if err != nil {
		return err
	}
	if !exists {
		return canonicalEmailFailure(FailureUserTableMissing)
	}
	columns := make([]struct {
		Name string `gorm:"column:column_name"`
	}, 0)
	var result *gorm.DB
	switch dialect {
	case "sqlite":
		result = db.Raw("SELECT name AS column_name FROM pragma_table_info('mss_boot_users')").
			Scan(&columns)
	case "postgres":
		result = db.Raw(
			`SELECT column_name FROM information_schema.columns
			 WHERE table_schema = current_schema() AND table_name = ?`,
			(&models.User{}).TableName(),
		).Scan(&columns)
	case "mysql":
		result = db.Raw(
			`SELECT COLUMN_NAME AS column_name FROM information_schema.COLUMNS
			 WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ?`,
			(&models.User{}).TableName(),
		).Scan(&columns)
	default:
		return canonicalEmailFailure(FailureUnsupportedDatabase)
	}
	if result.Error != nil {
		return canonicalEmailFailure(FailureInspection)
	}
	foundEmail := false
	foundDeletedAt := false
	for _, column := range columns {
		switch strings.ToLower(column.Name) {
		case "email":
			foundEmail = true
		case "deleted_at":
			foundDeletedAt = true
		}
	}
	if !foundEmail || !foundDeletedAt {
		return canonicalEmailFailure(FailureUserColumnsMissing)
	}
	return nil
}

func canonicalEmailTableExists(db *gorm.DB, dialect, table string) (bool, error) {
	var count int64
	var result *gorm.DB
	switch dialect {
	case "sqlite":
		result = db.Raw(
			"SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?",
			table,
		).Scan(&count)
	case "postgres":
		result = db.Raw(
			`SELECT COUNT(*) FROM information_schema.tables
			 WHERE table_schema = current_schema() AND table_name = ?`,
			table,
		).Scan(&count)
	case "mysql":
		result = db.Raw(
			`SELECT COUNT(*) FROM information_schema.TABLES
			 WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ?`,
			table,
		).Scan(&count)
	default:
		return false, canonicalEmailFailure(FailureUnsupportedDatabase)
	}
	if result.Error != nil {
		return false, canonicalEmailFailure(FailureInspection)
	}
	return count == 1, nil
}

func inspectCanonicalEmailMySQLColumn(
	db *gorm.DB,
) (canonicalEmailIndexMetadata, bool, error) {
	var row struct {
		DataType             string `gorm:"column:DATA_TYPE"`
		MaximumLength        *int64 `gorm:"column:CHARACTER_MAXIMUM_LENGTH"`
		IsNullable           string `gorm:"column:IS_NULLABLE"`
		Extra                string `gorm:"column:EXTRA"`
		GenerationExpression string `gorm:"column:GENERATION_EXPRESSION"`
	}
	result := db.Raw(
		`SELECT DATA_TYPE, CHARACTER_MAXIMUM_LENGTH, IS_NULLABLE, EXTRA, GENERATION_EXPRESSION
		 FROM information_schema.COLUMNS
		 WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ? AND COLUMN_NAME = ?`,
		(&models.User{}).TableName(),
		CanonicalEmailIdentityGeneratedColumn,
	).Scan(&row)
	if result.Error != nil {
		return canonicalEmailIndexMetadata{}, false, canonicalEmailFailure(FailureInspection)
	}
	if result.RowsAffected == 0 {
		return canonicalEmailIndexMetadata{}, false, nil
	}
	var length int64
	if row.MaximumLength != nil {
		length = *row.MaximumLength
	}
	return canonicalEmailIndexMetadata{
		dataType:             row.DataType,
		maximumLength:        length,
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
		SubPart    *int64 `gorm:"column:SUB_PART"`
	}, 0, 1)
	result := db.Raw(
		`SELECT COLUMN_NAME, SEQ_IN_INDEX, NON_UNIQUE, SUB_PART
		 FROM information_schema.STATISTICS
		 WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ? AND INDEX_NAME = ?
		 ORDER BY SEQ_IN_INDEX`,
		(&models.User{}).TableName(),
		models.EmailIdentityUniqueIndex,
	).Scan(&rows)
	if result.Error != nil {
		return metadata, false, canonicalEmailFailure(FailureInspection)
	}
	metadata.indexColumns = metadata.indexColumns[:0]
	for _, row := range rows {
		metadata.indexColumns = append(metadata.indexColumns, canonicalEmailIndexColumn{
			name:      row.ColumnName,
			sequence:  row.SeqInIndex,
			nonUnique: row.NonUnique != 0,
			subPart:   row.SubPart,
		})
	}
	return metadata, len(rows) != 0, nil
}

func canonicalEmailPartialIndexCompatible(
	dialect string,
	metadata canonicalEmailIndexMetadata,
) bool {
	if !metadata.unique || metadata.keyColumns != 1 {
		return false
	}
	switch dialect {
	case "sqlite":
		if !metadata.partial {
			return false
		}
		actual := compactCanonicalEmailSQL(metadata.definition)
		actual = strings.Replace(actual, "createuniqueindexifnotexists", "createuniqueindex", 1)
		expected := compactCanonicalEmailSQL(
			"CREATE UNIQUE INDEX " + models.EmailIdentityUniqueIndex +
				" ON mss_boot_users (LOWER(TRIM(email)))" +
				" WHERE deleted_at IS NULL AND TRIM(email) <> ''",
		)
		return actual == expected
	case "postgres":
		if !metadata.valid || !metadata.ready || metadata.totalColumns != 1 ||
			metadata.collationName != "C" || metadata.collationSchema != "pg_catalog" {
			return false
		}
		expression := compactCanonicalEmailExpression(metadata.expression)
		predicate := compactCanonicalEmailExpression(metadata.predicate)
		validExpression := expression == "lowertrimemailcollatec" ||
			expression == "lowertrimbothfromemailcollatec" ||
			expression == "lowerbtrimemailcollatec"
		validPredicate := predicate == "deleted_atisnullandtrimemail<>''" ||
			predicate == "deleted_atisnullandtrimbothfromemail<>''" ||
			predicate == "deleted_atisnullandbtrimemail<>''"
		return validExpression && validPredicate
	default:
		return false
	}
}

func canonicalEmailMySQLColumnCompatible(metadata canonicalEmailIndexMetadata) bool {
	if !strings.EqualFold(metadata.dataType, "varbinary") || metadata.maximumLength != 100 ||
		!strings.EqualFold(metadata.nullable, "YES") ||
		!strings.Contains(strings.ToLower(metadata.extra), "stored generated") {
		return false
	}
	generation := compactCanonicalEmailExpression(metadata.generationExpression)
	return generation == "casewhendeleted_atisnullandtrimemail<>''thencastlowerconverttrimemailusingasciicollateascii_general_ciasbinary100elsenullend" ||
		generation == "casewhendeleted_atisnullandtrimemail<>''thencastlowerconverttrimemailusingasciicollateascii_general_ciaschar100charsetbinaryelsenullend"
}

func canonicalEmailMySQLIndexCompatible(metadata canonicalEmailIndexMetadata) bool {
	if len(metadata.indexColumns) != 1 {
		return false
	}
	column := metadata.indexColumns[0]
	return column.name == CanonicalEmailIdentityGeneratedColumn &&
		column.sequence == 1 && !column.nonUnique && column.subPart == nil
}

func compactCanonicalEmailSQL(value string) string {
	replacer := strings.NewReplacer(
		" ", "",
		"\t", "",
		"\n", "",
		"\r", "",
		"`", "",
		"\"", "",
		"\\", "",
	)
	return strings.ToLower(replacer.Replace(value))
}

func compactCanonicalEmailExpression(value string) string {
	value = compactCanonicalEmailSQL(value)
	value = canonicalEmailMySQLCharsetLiteral.ReplaceAllString(value, "''")
	replacer := strings.NewReplacer(
		"(", "",
		")", "",
		"::text", "",
		"_utf8mb4", "",
		"_utf8mb3", "",
		"_utf8", "",
		"_latin1", "",
		"_ascii", "",
	)
	return replacer.Replace(value)
}

func verifyCanonicalEmailData(db *gorm.DB) error {
	rows := make([]struct {
		Email *string `gorm:"column:email"`
	}, 0)
	if err := db.Table((&models.User{}).TableName()).
		Select("email").
		Where("deleted_at IS NULL").
		Scan(&rows).Error; err != nil {
		return canonicalEmailFailure(FailureInspection)
	}

	identities := make(map[string]struct{}, len(rows))
	invalid := false
	ambiguous := false
	for _, row := range rows {
		if row.Email == nil || *row.Email == "" {
			continue
		}
		canonical, err := models.CanonicalEmailIdentity(*row.Email)
		if err != nil || canonical != *row.Email {
			invalid = true
			continue
		}
		if _, exists := identities[canonical]; exists {
			ambiguous = true
		}
		identities[canonical] = struct{}{}
	}
	if invalid && ambiguous {
		return canonicalEmailFailure(
			FailureIdentityDataInvalid,
			models.ErrEmailIdentityInvalid,
			models.ErrEmailIdentityAmbiguous,
		)
	}
	if invalid {
		return canonicalEmailFailure(FailureIdentityDataInvalid, models.ErrEmailIdentityInvalid)
	}
	if ambiguous {
		return canonicalEmailFailure(FailureIdentityDataAmbiguous, models.ErrEmailIdentityAmbiguous)
	}
	return nil
}

func verifyCanonicalEmailMigrationRecord(db *gorm.DB) error {
	dialect := db.Dialector.Name()
	exists, err := canonicalEmailTableExists(
		db,
		dialect,
		(&migrationmodels.Migration{}).TableName(),
	)
	if err != nil {
		return err
	}
	if !exists {
		return canonicalEmailFailure(FailureMigrationTableMissing)
	}
	var count int64
	result := db.Model(&migrationmodels.Migration{}).
		Where("version = ?", CanonicalEmailIdentityMigrationVersion).
		Count(&count)
	if result.Error != nil {
		return canonicalEmailFailure(FailureInspection)
	}
	if count != 1 {
		return canonicalEmailFailure(FailureMigrationVersionMissing)
	}
	return nil
}
