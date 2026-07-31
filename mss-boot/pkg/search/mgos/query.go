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
	"strings"

	"go.mongodb.org/mongo-driver/bson"
)

const (
	// FromQueryTag tag标记
	FromQueryTag = "search"
)

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
	if qType.Kind() == reflect.Ptr {
		qType = qType.Elem()
	}
	qValue := reflect.ValueOf(q)
	if qValue.Kind() == reflect.Ptr {
		qValue = qValue.Elem()
	}
	var tag string
	var ok bool
	var t *resolveSearchTag
	for i := 0; i < qType.NumField(); i++ {
		tag, ok = qType.Field(i).Tag.Lookup(FromQueryTag)
		if !ok {
			continue
		}
		switch tag {
		case "-":
			continue
		case "dlv":
			// 递归调用
			ResolveSearchQuery(qValue.Field(i).Interface(), condition)
			continue
		}
		t = makeTag(tag)
		if qValue.Field(i).IsZero() {
			continue
		}
		// 解析
		switch t.Type {
		// case "left":
		// todo 左关联
		case "exact", "iexact":
			condition.SetAnd(bson.M{t.Column: qValue.Field(i).Interface()})
		case "contains", "icontains":
			condition.SetAnd(bson.M{t.Column: bson.M{"$regex": qValue.Field(i).Interface()}})
		case "gt":
			condition.SetAnd(bson.M{t.Column: bson.M{"$gt": qValue.Field(i).Interface()}})
		case "gte":
			condition.SetAnd(bson.M{t.Column: bson.M{"$gte": qValue.Field(i).Interface()}})
		case "lt":
			condition.SetAnd(bson.M{t.Column: bson.M{"$lt": qValue.Field(i).Interface()}})
		case "lte":
			condition.SetAnd(bson.M{t.Column: bson.M{"$lte": qValue.Field(i).Interface()}})
		case "startswith", "istartswith":
			condition.SetAnd(bson.M{t.Column: bson.M{"$regex": fmt.Sprintf("^%v.*", qValue.Field(i).Interface())}})
		case "endswith", "iendswith":
			condition.SetAnd(bson.M{t.Column: bson.M{"$regex": fmt.Sprintf("*.%v$", qValue.Field(i).Interface())}})
		case "in":
			condition.SetAnd(bson.M{t.Column: bson.M{"$in": qValue.Field(i).Interface()}})
		case "isnull":
			condition.SetAnd(bson.M{t.Column: bson.M{"$in": []any{nil}, "$exists": true}})
		case "order":
			switch strings.ToLower(qValue.Field(i).String()) {
			case "asc":
				condition.SetOrder(t.Column, 1)
			case "desc":
				condition.SetOrder(t.Column, -1)
			}
		}
	}
}
