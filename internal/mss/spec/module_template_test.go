package spec

import (
	"bytes"
	"testing"
)

func TestRenderModuleTemplateProducesCompleteV6Starter(t *testing.T) {
	data, err := RenderModuleTemplate("purchase-order", "Purchase order")
	if err != nil {
		t.Fatalf("RenderModuleTemplate() error = %v", err)
	}
	module, err := ParseModule(data, "starter.yaml")
	if err != nil {
		t.Fatalf("ParseModule(starter) error = %v", err)
	}

	for _, operation := range []string{"list", "get", "create", "update", "delete", "export"} {
		if !contains(module.Spec.API.Operations, operation) {
			t.Errorf("starter operations = %#v, missing %q", module.Spec.API.Operations, operation)
		}
	}
	if _, ok := module.Permission("export"); !ok {
		t.Error("starter is missing the export permission")
	}
	if !module.Spec.UI.List || !module.Spec.UI.Form || !module.Spec.UI.Detail || !module.Spec.UI.Export {
		t.Fatalf("starter UI = %#v, want complete initial Ant Design 6 surfaces", module.Spec.UI)
	}
	marker, ok := module.Field("name")
	if !ok || !marker.Unique || !marker.Searchable {
		t.Fatalf("starter E2E marker = %#v, exists=%t", marker, ok)
	}
	if module.Spec.Generation.MigrationID == "" || module.Spec.Generation.AuthorizationMigrationID == "" ||
		module.Spec.Generation.MigrationID == module.Spec.Generation.AuthorizationMigrationID {
		t.Fatalf("starter migration identities = %#v", module.Spec.Generation)
	}

	repeated, err := RenderModuleTemplate("purchase-order", "Purchase order")
	if err != nil {
		t.Fatalf("RenderModuleTemplate(repeated) error = %v", err)
	}
	if !bytes.Equal(data, repeated) {
		t.Fatal("starter output is not deterministic for the same module")
	}
	otherData, err := RenderModuleTemplate("customer", "Customer")
	if err != nil {
		t.Fatalf("RenderModuleTemplate(other) error = %v", err)
	}
	other, err := ParseModule(otherData, "other.yaml")
	if err != nil {
		t.Fatalf("ParseModule(other starter) error = %v", err)
	}
	if module.Spec.Generation.MigrationID == other.Spec.Generation.MigrationID ||
		module.Spec.Generation.AuthorizationMigrationID == other.Spec.Generation.AuthorizationMigrationID {
		t.Fatalf("different starters reused migration identities: %#v and %#v", module.Spec.Generation, other.Spec.Generation)
	}
}
