package mgos

/*
 * @Author: lwnmengjing
 * @Date: 2022/3/11 15:52
 * @Last Modified by: lwnmengjing
 * @Last Modified time: 2022/3/11 15:52
 */

import (
	"strings"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Condition interface
type Condition interface {
	SetAnd(bson.M)
	SetOr(bson.M)
	SetOrder(string, int8)
}

// Public mongodb public
type Public struct {
	And   []bson.M
	Or    []bson.M
	Order bson.D
}

// SetAnd set and
func (e *Public) SetAnd(m bson.M) {
	if e.And == nil {
		e.And = make([]bson.M, 0)
	}
	e.And = append(e.And, m)
}

// SetOr set or
func (e *Public) SetOr(m bson.M) {
	if e.Or == nil {
		e.Or = make([]bson.M, 0)
	}
	e.Or = append(e.Or, m)
}

// SetOrder set order
func (e *Public) SetOrder(key string, t int8) {
	if e.Order == nil {
		e.Order = bson.D{}
	}
	e.Order = append(e.Order, primitive.E{Key: key, Value: t})
}

type resolveSearchTag struct {
	Type   string
	Column string
	Table  string
	On     []string
	Join   string
}

// makeTag 解析search的tag标签
func makeTag(tag string) *resolveSearchTag {
	r := &resolveSearchTag{}
	tags := strings.Split(tag, ";")
	var ts []string
	for _, t := range tags {
		ts = strings.Split(t, ":")
		if len(ts) == 0 {
			continue
		}
		switch ts[0] {
		case "type":
			if len(ts) > 1 {
				r.Type = ts[1]
			}
		case "column":
			if len(ts) > 1 {
				r.Column = ts[1]
			}
		case "table":
			if len(ts) > 1 {
				r.Table = ts[1]
			}
		case "on":
			if len(ts) > 1 {
				r.On = ts[1:]
			}
		case "join":
			if len(ts) > 1 {
				r.Join = ts[1]
			}
		}
	}
	return r
}
