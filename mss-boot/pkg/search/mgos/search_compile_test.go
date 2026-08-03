package mgos

import (
	"reflect"
	"testing"

	"go.mongodb.org/mongo-driver/bson"
)

func TestCompileConditionReturnsTypedDetachedDocuments(t *testing.T) {
	type query struct {
		Name  string `search:"type:exact;column:name"`
		Order string `search:"type:order;column:created_at"`
	}

	filter, sort, err := CompileCondition(query{Name: "alice", Order: "desc"})
	if err != nil {
		t.Fatalf("CompileCondition() error = %v", err)
	}
	comparison, ok := filter["name"].(bson.M)
	if !ok {
		t.Fatalf("compiled filter = %#v, want typed bson.M comparison", filter)
	}
	if comparison["$eq"] != "alice" {
		t.Fatalf("compiled exact value = %#v", comparison["$eq"])
	}
	wantSort := bson.D{{Key: "created_at", Value: int32(-1)}}
	if !reflect.DeepEqual(sort, wantSort) {
		t.Fatalf("compiled sort = %#v, want %#v", sort, wantSort)
	}
}

func TestCompileConditionRejectsValuesThatCannotBeEncodedAsBSON(t *testing.T) {
	type query struct {
		Value any `search:"type:exact;column:value"`
	}

	if _, _, err := CompileCondition(query{Value: make(chan struct{})}); err == nil {
		t.Fatal("CompileCondition() accepted a value that cannot be encoded as BSON")
	}
}
