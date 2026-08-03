package gorms

import (
	"fmt"
	"reflect"
	"strings"
)

const (
	// FromQueryTag tag标记
	FromQueryTag = "search"
	// Mysql 数据库标识
	Mysql = "mysql"
	// Postgres 数据库标识
	Postgres = "postgres"
	// Dm 数据库标识
	Dm = "dm"
)

// ResolveSearchQuery 解析
/**
 * 	exact / iexact 等于
 * 	contains / icontains 包含
 *	gt / gte 大于 / 大于等于
 *	lt / lte 小于 / 小于等于
 *	startswith / istartswith 以…起始
 *	endswith / iendswith 以…结束
 *	between 范围     e.g. receiveAt[]=2021-01-01&receiveAt[]=2021-01-02
 *	in
 *	isnull
 *  order 排序		e.g. order[key]=desc     order[key]=asc
 */
func ResolveSearchQuery(driver string, q any, condition Condition) {
	qType := reflect.TypeOf(q)
	qValue := reflect.ValueOf(q)
	if qType.Kind() == reflect.Ptr {
		qType = qType.Elem()
	}
	if qValue.Kind() == reflect.Ptr {
		qValue = qValue.Elem()
	}
	var t *resolveSearchTag

	for i := 0; i < qType.NumField(); i++ {
		tag, ok := qType.Field(i).Tag.Lookup(FromQueryTag)
		if !ok {
			// 递归调用
			ResolveSearchQuery(driver, qValue.Field(i).Interface(), condition)
			continue
		}
		switch tag {
		case "-":
			continue
		}
		t = makeTag(tag)
		if qValue.Field(i).IsZero() {
			continue
		}

		parseSQL(driver, t, condition, qValue, i)
	}
}

func parseSQL(driver string, searchTag *resolveSearchTag, condition Condition, qValue reflect.Value, i int) {
	var sep = "`"
	if driver == Postgres {
		sep = "\""
	}

	if driver == Dm {
		searchTag.Table = strings.ToUpper(searchTag.Table)
		searchTag.Column = strings.ToUpper(searchTag.Column)
	}
	iStr := ""
	if driver == Postgres {
		iStr = "i"
	}
	column := fmt.Sprintf("%s%s%s", sep, searchTag.Column, sep)
	if searchTag.Table != "" {
		searchTag.Table = fmt.Sprintf("%s%s%s.", sep, searchTag.Table, sep)
		column = searchTag.Table + column
	}
	switch searchTag.Type {
	case "left":
		// 左关联
		join := condition.SetJoinOn(searchTag.Type, fmt.Sprintf(
			"left join %s%s%s on %s%s%s.%s%s%s = %s%s%s.%s%s%s",
			sep,
			searchTag.Join,
			sep, sep,
			searchTag.Join,
			sep, sep,
			searchTag.On[0],
			sep, sep,
			searchTag.Table,
			sep, sep,
			searchTag.On[1],
			sep,
		))
		ResolveSearchQuery(driver, qValue.Field(i).Interface(), join)
	case "exact", "iexact":
		condition.SetWhere(fmt.Sprintf("%s = ?", column), []any{qValue.Field(i).Interface()})
	case "contains":
		condition.SetWhere(fmt.Sprintf("%s like ?", column), []any{"%" + qValue.Field(i).String() + "%"})
	case "icontains":
		condition.SetWhere(fmt.Sprintf("%s %slike ?", column, iStr), []any{"%" + qValue.Field(i).String() + "%"})
	case "gt":
		condition.SetWhere(fmt.Sprintf("%s > ?", column), []any{qValue.Field(i).Interface()})
	case "gte":
		condition.SetWhere(fmt.Sprintf("%s >= ?", column), []any{qValue.Field(i).Interface()})
	case "lt":
		condition.SetWhere(fmt.Sprintf("%s < ?", column), []any{qValue.Field(i).Interface()})
	case "lte":
		condition.SetWhere(fmt.Sprintf("%s <= ?", column), []any{qValue.Field(i).Interface()})
	case "startswith":
		condition.SetWhere(fmt.Sprintf("%s like ?", column), []any{qValue.Field(i).String() + "%"})
	case "istartswith":
		condition.SetWhere(fmt.Sprintf("%s %slike ?", column, iStr), []any{qValue.Field(i).String() + "%"})
	case "endswith":
		condition.SetWhere(fmt.Sprintf("%s like ?", column), []any{"%" + qValue.Field(i).String()})
	case "iendswith":
		condition.SetWhere(fmt.Sprintf("%s %slike ?", column, iStr), []any{"%" + qValue.Field(i).String()})
	case "in":
		condition.SetWhere(fmt.Sprintf("%s in (?)", column), []any{qValue.Field(i).Interface()})
	case "isnull":
		if !qValue.Field(i).IsZero() || !qValue.Field(i).IsNil() {
			condition.SetWhere(fmt.Sprintf("%s isnull", column), make([]any, 0))
		}
	case "between":
		condition.SetWhere(fmt.Sprintf("%s between ? and ?", column),
			[]any{qValue.Field(i).Index(0).Interface(), qValue.Field(i).Index(1).Interface()})
	case "order":
		switch strings.ToLower(qValue.Field(i).String()) {
		case "desc", "asc":
			condition.SetOrder(fmt.Sprintf("%s %s", column, qValue.Field(i).String()))
		}
	}
}
