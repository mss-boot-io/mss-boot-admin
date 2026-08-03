package mgos

import (
	"reflect"
	"testing"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestResolveSearchQueryWrapsExactValuesInEq(t *testing.T) {
	type query struct {
		Value any `search:"type:exact;column:name"`
	}
	condition := &Public{}
	operatorShapedValue := bson.M{"$ne": nil}

	ResolveSearchQuery(query{Value: operatorShapedValue}, condition)

	if len(condition.And) != 1 {
		t.Fatalf("condition count = %d, want 1", len(condition.And))
	}
	comparison, ok := condition.And[0]["name"].(bson.M)
	if !ok {
		t.Fatalf("exact comparison = %#v, want bson.M", condition.And[0]["name"])
	}
	if !reflect.DeepEqual(comparison["$eq"], operatorShapedValue) {
		t.Fatalf("$eq value = %#v, want %#v", comparison["$eq"], operatorShapedValue)
	}
}

func TestResolveSearchQueryEscapesRegexInput(t *testing.T) {
	type query struct {
		Value string `search:"type:icontains;column:name"`
	}
	condition := &Public{}

	ResolveSearchQuery(query{Value: `a.*(b)`}, condition)

	regex, ok := condition.And[0]["name"].(primitive.Regex)
	if !ok {
		t.Fatalf("regex value = %#v, want primitive.Regex", condition.And[0]["name"])
	}
	if regex.Pattern != `a\.\*\(b\)` {
		t.Fatalf("regex pattern = %q, want escaped literal", regex.Pattern)
	}
	if regex.Options != "i" {
		t.Fatalf("regex options = %q, want %q", regex.Options, "i")
	}
}

func TestResolveSearchQueryRejectsOperatorFieldNames(t *testing.T) {
	type query struct {
		Value string `search:"type:exact;column:$where"`
	}
	condition := &Public{}

	ResolveSearchQuery(query{Value: "return true"}, condition)

	if len(condition.And) != 0 {
		t.Fatalf("unsafe field generated conditions: %#v", condition.And)
	}
}

func TestResolveSearchQueryUsesStaticSortField(t *testing.T) {
	type query struct {
		Order string `search:"type:order;column:created_at"`
	}
	condition := &Public{}

	ResolveSearchQuery(query{Order: "DESC"}, condition)

	want := bson.D{{Key: "created_at", Value: int8(-1)}}
	if !reflect.DeepEqual(condition.Order, want) {
		t.Fatalf("sort = %#v, want %#v", condition.Order, want)
	}
}
