package gorms

import "strings"

// Condition interface
type Condition interface {
	SetWhere(k string, v []any)
	SetOr(k string, v []any)
	SetOrder(k string)
	SetJoinOn(t, on string) Condition
}

// GormCondition gorm condition
type GormCondition struct {
	// GormPublic gorm public
	GormPublic
	// Join gorm join
	Join []*GormJoin
}

// GormPublic gorm public
type GormPublic struct {
	// Where and condition
	Where map[string][]any
	// Order order
	Order []string
	// Or condition
	Or map[string][]any
}

// GormJoin gorm join
type GormJoin struct {
	// Type join type
	Type string
	// JoinOn Table join table
	JoinOn string
	// GormPublic On join on
	GormPublic
}

// SetJoinOn set join on
func (e *GormJoin) SetJoinOn(t, on string) Condition {
	return nil
}

// SetWhere set where
func (e *GormPublic) SetWhere(k string, v []any) {
	if e.Where == nil {
		e.Where = make(map[string][]any)
	}
	e.Where[k] = v
}

// SetOr set or condition
func (e *GormPublic) SetOr(k string, v []any) {
	if e.Or == nil {
		e.Or = make(map[string][]any)
	}
	e.Or[k] = v
}

// SetOrder set order
func (e *GormPublic) SetOrder(k string) {
	if e.Order == nil {
		e.Order = make([]string, 0)
	}
	e.Order = append(e.Order, k)
}

// SetJoinOn set join on
func (e *GormCondition) SetJoinOn(t, on string) Condition {
	if e.Join == nil {
		e.Join = make([]*GormJoin, 0)
	}
	join := &GormJoin{
		Type:       t,
		JoinOn:     on,
		GormPublic: GormPublic{},
	}
	e.Join = append(e.Join, join)
	return join
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
