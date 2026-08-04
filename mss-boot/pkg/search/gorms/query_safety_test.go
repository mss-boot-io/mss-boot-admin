package gorms

import "testing"

type paginationFields struct {
	Current  int64 `form:"current"`
	PageSize int64 `form:"pageSize"`
}

type safeSearchQuery struct {
	paginationFields
	Name    string   `search:"type:contains;column:name"`
	Missing *nestedSearch
	Range   []int64 `search:"type:between;column:created_at"`
	Null    bool    `search:"type:isnull;column:deleted_at"`
}

type nestedSearch struct {
	Status string `search:"type:exact;column:status"`
}

func TestResolveSearchQueryIgnoresNonStructEmbeddedFields(t *testing.T) {
	condition := &GormCondition{}
	query := safeSearchQuery{
		paginationFields: paginationFields{Current: 2, PageSize: 20},
		Name:             "alice",
	}

	ResolveSearchQuery(Mysql, query, condition)

	values, ok := condition.Where["`name` like ?"]
	if !ok {
		t.Fatalf("name condition was not generated: %#v", condition.Where)
	}
	if len(values) != 1 || values[0] != "%alice%" {
		t.Fatalf("unexpected name condition values: %#v", values)
	}
	if len(condition.Where) != 1 {
		t.Fatalf("pagination fields unexpectedly became search conditions: %#v", condition.Where)
	}
}

func TestResolveSearchQueryHandlesNilAndMalformedValuesWithoutPanic(t *testing.T) {
	condition := &GormCondition{}
	query := safeSearchQuery{
		Missing: nil,
		Range:   []int64{1},
	}

	ResolveSearchQuery(Mysql, query, condition)
	ResolveSearchQuery(Mysql, nil, condition)
	ResolveSearchQuery(Mysql, int64(42), condition)
	ResolveSearchQuery(Mysql, query, nil)

	if len(condition.Where) != 0 {
		t.Fatalf("malformed values unexpectedly generated conditions: %#v", condition.Where)
	}
}

func TestResolveSearchQueryBuildsSafeIsNullPredicate(t *testing.T) {
	condition := &GormCondition{}
	ResolveSearchQuery(Mysql, safeSearchQuery{Null: true}, condition)

	if _, ok := condition.Where["`deleted_at` is null"]; !ok {
		t.Fatalf("is-null condition was not generated: %#v", condition.Where)
	}
}
