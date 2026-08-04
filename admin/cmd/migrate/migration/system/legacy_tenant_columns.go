package system

import (
	"log/slog"
	"strings"

	"gorm.io/gorm"
)

type legacyTenantColumn struct {
	TableName  string `gorm:"column:table_name"`
	ColumnType string `gorm:"column:column_type"`
}

func relaxLegacyTenantColumns(tx *gorm.DB) error {
	query, ok := legacyTenantColumnsQuery(tx.Dialector.Name())
	if !ok {
		return nil
	}

	var columns []legacyTenantColumn
	if err := tx.Raw(query).Scan(&columns).Error; err != nil {
		return err
	}

	for i := range columns {
		sql, ok := relaxLegacyTenantColumnSQL(tx.Dialector.Name(), columns[i])
		if !ok {
			continue
		}
		if err := tx.Exec(sql).Error; err != nil {
			return err
		}
		slog.Info("relaxed legacy tenant_id NOT NULL constraint",
			"dialect", tx.Dialector.Name(),
			"table", columns[i].TableName)
	}
	return nil
}

func legacyTenantColumnsQuery(dialect string) (string, bool) {
	switch dialect {
	case "postgres":
		return `SELECT table_name, '' AS column_type
FROM information_schema.columns
WHERE table_schema = CURRENT_SCHEMA()
  AND column_name = 'tenant_id'
  AND is_nullable = 'NO'
ORDER BY table_name`, true
	case "mysql":
		return `SELECT table_name, column_type
FROM information_schema.columns
WHERE table_schema = DATABASE()
  AND column_name = 'tenant_id'
  AND is_nullable = 'NO'
ORDER BY table_name`, true
	default:
		return "", false
	}
}

func relaxLegacyTenantColumnSQL(dialect string, column legacyTenantColumn) (string, bool) {
	if column.TableName == "" {
		return "", false
	}
	switch dialect {
	case "postgres":
		return "ALTER TABLE " + quoteIdentifier(dialect, column.TableName) +
			" ALTER COLUMN " + quoteIdentifier(dialect, "tenant_id") + " DROP NOT NULL", true
	case "mysql":
		columnType := column.ColumnType
		if columnType == "" {
			columnType = "varchar(64)"
		}
		return "ALTER TABLE " + quoteIdentifier(dialect, column.TableName) +
			" MODIFY COLUMN " + quoteIdentifier(dialect, "tenant_id") + " " + columnType + " NULL", true
	default:
		return "", false
	}
}

func quoteIdentifier(dialect, identifier string) string {
	switch dialect {
	case "mysql":
		return "`" + strings.ReplaceAll(identifier, "`", "``") + "`"
	default:
		return `"` + strings.ReplaceAll(identifier, `"`, `""`) + `"`
	}
}
