package mgos

/*
 * @Author: lwnmengjing
 * @Date: 2022/3/11 16:03
 * @Last Modified by: lwnmengjing
 * @Last Modified time: 2022/3/11 16:03
 */

import (
	"fmt"
	"reflect"
	"regexp"
	"strings"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

const (
	// FromQueryTag tag标记
	FromQueryTag = "search"
)

var mongoFieldPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_.]*$`)

// ResolveSearchQuery 解析
/**
 * 	exact / iexact 等于
 * 	contains / icontains 包含
 *	gt / gte 大于 / 大于等于
 *	lt / lte 小于 / 小于等于
 *	startswith / istartswith 以…起始
 *	endswith / iendswith 以…结束
 *	in
 *	isnull
 *  order 排序		e.g. order[key]=desc     order[key]=asc
 */
func ResolveSearchQuery(q any, condition Condition) {
	qType := reflect.TypeOf(q)
	if qType == nil {
		return
	}
	if qType.Kind() == reflect.Ptr {
		qType = qType.Elem()
	}
	qValue := reflect.ValueOf(q)
	if qValue.Kind() == reflect.Ptr {
		if qValue.IsNil() {
			return
		}
		qValue = qValue.Elem()
	}
	if qType.Kind() != reflect.Struct || qValue.Kind() != reflect.Struct {
		return
	}

	for i := 0; i < qType.NumField(); i++ {
		tag, ok := qType.Field(i).Tag.Lookup(FromQueryTag)
		if !ok || tag == "-" {
			continue
		}
		if tag == "dlv" {
			ResolveSearchQuery(qValue.Field(i).Interface(), condition)
			continue
		}

		resolved := makeTag(tag)
		if !safeMongoField(resolved.Column) || qValue.Field(i).IsZero() {
			continue
		}
		value := qValue.Field(i).Interface()

		switch resolved.Type {
		case "exact":
			// $eq forces even document-shaped input to be interpreted as a literal value.
			condition.SetAnd(bson.M{resolved.Column: bson.M{"$eq": value}})
		case "iexact":
			condition.SetAnd(bson.M{resolved.Column: safeRegex(value, true, true, true)})
		case "contains":
			condition.SetAnd(bson.M{resolved.Column: safeRegex(value, false, false, false)})
		case "icontains":
			condition.SetAnd(bson.M{resolved.Column: safeRegex(value, false, false, true)})
		case "gt":
			condition.SetAnd(bson.M{resolved.Column: bson.M{"$gt": value}})
		case "gte":
			condition.SetAnd(bson.M{resolved.Column: bson.M{"$gte": value}})
		case "lt":
			condition.SetAnd(bson.M{resolved.Column: bson.M{"$lt": value}})
		case "lte":
			condition.SetAnd(bson.M{resolved.Column: bson.M{"$lte": value}})
		case "startswith":
			condition.SetAnd(bson.M{resolved.Column: safeRegex(value, true, false, false)})
		case "istartswith":
			condition.SetAnd(bson.M{resolved.Column: safeRegex(value, true, false, true)})
		case "endswith":
			condition.SetAnd(bson.M{resolved.Column: safeRegex(value, false, true, false)})
		case "iendswith":
			condition.SetAnd(bson.M{resolved.Column: safeRegex(value, false, true, true)})
		case "in":
			condition.SetAnd(bson.M{resolved.Column: bson.M{"$in": value}})
		case "isnull":
			condition.SetAnd(bson.M{resolved.Column: bson.M{"$in": []any{nil}, "$exists": true}})
		case "order":
			switch strings.ToLower(qValue.Field(i).String()) {
			case "asc":
				condition.SetOrder(resolved.Column, 1)
			case "desc":
				condition.SetOrder(resolved.Column, -1)
			}
		}
	}
}

func safeMongoField(column string) bool {
	return mongoFieldPattern.MatchString(column) && !strings.Contains(column, "..")
}

func safeRegex(value any, anchorStart, anchorEnd, caseInsensitive bool) primitive.Regex {
	pattern := regexp.QuoteMeta(fmt.Sprint(value))
	if anchorStart {
		pattern = "^" + pattern
	}
	if anchorEnd {
		pattern += "$"
	}
	options := ""
	if caseInsensitive {
		options = "i"
	}
	return primitive.Regex{Pattern: pattern, Options: options}
}
