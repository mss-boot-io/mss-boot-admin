package gorms

import (
	"fmt"
	"reflect"
	"strings"
)

const (
	// FromQueryTag marks fields that participate in generated search clauses.
	FromQueryTag = "search"
	// Mysql identifies the MySQL SQL dialect.
	Mysql = "mysql"
	// Postgres identifies the PostgreSQL SQL dialect.
	Postgres = "postgres"
	// Dm identifies the DM SQL dialect.
	Dm = "dm"
)

// ResolveSearchQuery translates search-tagged fields into Condition calls.
//
// Fields without a search tag are traversed only when they are nested structs.
// Invalid, nil, or unsupported values are ignored instead of panicking because
// this function is used on request DTOs that commonly embed pagination and
// other non-search fields.
func ResolveSearchQuery(driver string, query any, condition Condition) {
	if query == nil || condition == nil {
		return
	}

	queryValue, ok := indirectValue(reflect.ValueOf(query))
	if !ok || queryValue.Kind() != reflect.Struct {
		return
	}
	queryType := queryValue.Type()

	for i := 0; i < queryType.NumField(); i++ {
		structField := queryType.Field(i)
		fieldValue := queryValue.Field(i)
		if structField.PkgPath != "" && !structField.Anonymous {
			// Unexported fields cannot be converted to interface values safely.
			continue
		}

		tag, tagged := structField.Tag.Lookup(FromQueryTag)
		if !tagged {
			if fieldValue.CanInterface() && isNestedStruct(fieldValue) {
				ResolveSearchQuery(driver, fieldValue.Interface(), condition)
			}
			continue
		}
		if tag == "-" || !fieldValue.CanInterface() || fieldValue.IsZero() {
			continue
		}

		parseSQL(driver, makeTag(tag), condition, fieldValue)
	}
}

func parseSQL(driver string, searchTag *resolveSearchTag, condition Condition, fieldValue reflect.Value) {
	if searchTag == nil || condition == nil || searchTag.Type == "" {
		return
	}

	separator := "`"
	if driver == Postgres {
		separator = "\""
	}
	if driver == Dm {
		searchTag.Table = strings.ToUpper(searchTag.Table)
		searchTag.Column = strings.ToUpper(searchTag.Column)
	}
	if searchTag.Column == "" && searchTag.Type != "left" {
		return
	}

	insensitivePrefix := ""
	if driver == Postgres {
		insensitivePrefix = "i"
	}
	column := fmt.Sprintf("%s%s%s", separator, searchTag.Column, separator)
	if searchTag.Table != "" {
		column = fmt.Sprintf("%s%s%s.%s", separator, searchTag.Table, separator, column)
	}

	switch searchTag.Type {
	case "left":
		if searchTag.Join == "" || searchTag.Table == "" || len(searchTag.On) < 2 {
			return
		}
		join := condition.SetJoinOn(searchTag.Type, fmt.Sprintf(
			"left join %s%s%s on %s%s%s.%s%s%s = %s%s%s.%s%s%s",
			separator,
			searchTag.Join,
			separator,
			separator,
			searchTag.Join,
			separator,
			separator,
			searchTag.On[0],
			separator,
			separator,
			searchTag.Table,
			separator,
			separator,
			searchTag.On[1],
			separator,
		))
		if join != nil && fieldValue.CanInterface() {
			ResolveSearchQuery(driver, fieldValue.Interface(), join)
		}
	case "exact", "iexact":
		condition.SetWhere(fmt.Sprintf("%s = ?", column), []any{fieldValue.Interface()})
	case "contains":
		if fieldValue.Kind() == reflect.String {
			condition.SetWhere(fmt.Sprintf("%s like ?", column), []any{"%" + fieldValue.String() + "%"})
		}
	case "icontains":
		if fieldValue.Kind() == reflect.String {
			condition.SetWhere(fmt.Sprintf("%s %slike ?", column, insensitivePrefix), []any{"%" + fieldValue.String() + "%"})
		}
	case "gt":
		condition.SetWhere(fmt.Sprintf("%s > ?", column), []any{fieldValue.Interface()})
	case "gte":
		condition.SetWhere(fmt.Sprintf("%s >= ?", column), []any{fieldValue.Interface()})
	case "lt":
		condition.SetWhere(fmt.Sprintf("%s < ?", column), []any{fieldValue.Interface()})
	case "lte":
		condition.SetWhere(fmt.Sprintf("%s <= ?", column), []any{fieldValue.Interface()})
	case "startswith":
		if fieldValue.Kind() == reflect.String {
			condition.SetWhere(fmt.Sprintf("%s like ?", column), []any{fieldValue.String() + "%"})
		}
	case "istartswith":
		if fieldValue.Kind() == reflect.String {
			condition.SetWhere(fmt.Sprintf("%s %slike ?", column, insensitivePrefix), []any{fieldValue.String() + "%"})
		}
	case "endswith":
		if fieldValue.Kind() == reflect.String {
			condition.SetWhere(fmt.Sprintf("%s like ?", column), []any{"%" + fieldValue.String()})
		}
	case "iendswith":
		if fieldValue.Kind() == reflect.String {
			condition.SetWhere(fmt.Sprintf("%s %slike ?", column, insensitivePrefix), []any{"%" + fieldValue.String()})
		}
	case "in":
		if fieldValue.Kind() == reflect.Slice || fieldValue.Kind() == reflect.Array {
			condition.SetWhere(fmt.Sprintf("%s in (?)", column), []any{fieldValue.Interface()})
		}
	case "isnull":
		// Zero values were filtered by ResolveSearchQuery. A non-zero flag means
		// the caller explicitly requested the NULL predicate.
		condition.SetWhere(fmt.Sprintf("%s is null", column), nil)
	case "between":
		if (fieldValue.Kind() == reflect.Slice || fieldValue.Kind() == reflect.Array) && fieldValue.Len() >= 2 {
			condition.SetWhere(
				fmt.Sprintf("%s between ? and ?", column),
				[]any{fieldValue.Index(0).Interface(), fieldValue.Index(1).Interface()},
			)
		}
	case "order":
		if fieldValue.Kind() != reflect.String {
			return
		}
		direction := strings.ToLower(fieldValue.String())
		if direction == "desc" || direction == "asc" {
			condition.SetOrder(fmt.Sprintf("%s %s", column, direction))
		}
	}
}

func indirectValue(value reflect.Value) (reflect.Value, bool) {
	for value.IsValid() && (value.Kind() == reflect.Pointer || value.Kind() == reflect.Interface) {
		if value.IsNil() {
			return reflect.Value{}, false
		}
		value = value.Elem()
	}
	return value, value.IsValid()
}

func isNestedStruct(value reflect.Value) bool {
	value, ok := indirectValue(value)
	return ok && value.Kind() == reflect.Struct
}
