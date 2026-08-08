package spec

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestFeatureJSONSchemaIsValidJSON(t *testing.T) {
	_, sourceFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve feature test source path")
	}
	schemaPath := filepath.Clean(filepath.Join(
		filepath.Dir(sourceFile),
		"..", "..", "..", ".mss", "schemas", "feature.schema.json",
	))
	data, err := os.ReadFile(schemaPath)
	if err != nil {
		t.Fatalf("read FeatureSpec JSON schema: %v", err)
	}
	var schema any
	if err := json.Unmarshal(data, &schema); err != nil {
		t.Fatalf("FeatureSpec JSON schema is invalid JSON: %v", err)
	}
}

func TestLoadFeatureValidatesCompleteContract(t *testing.T) {
	path := writeFeatureFixture(t, validFeatureYAML)
	feature, err := LoadFeature(path)
	if err != nil {
		t.Fatalf("load valid feature: %v", err)
	}
	if feature.Metadata.Name != "supplier-onboarding" {
		t.Fatalf("feature name = %q", feature.Metadata.Name)
	}
	if len(feature.Spec.Requirements) != 1 || len(feature.Spec.Acceptance) != 1 {
		t.Fatalf("unexpected feature contract: %#v", feature.Spec)
	}
	summary := feature.Summary()
	if summary["requiredAcceptance"] != 1 {
		t.Fatalf("unexpected summary: %#v", summary)
	}
}

func TestFeatureRejectsUnknownActorAndRequirement(t *testing.T) {
	content := strings.Replace(validFeatureYAML, "actor: procurement", "actor: finance", 1)
	content = strings.Replace(content, "requirement: supplier-create", "requirement: missing-requirement", 1)
	path := writeFeatureFixture(t, content)
	_, err := LoadFeature(path)
	if err == nil {
		t.Fatal("expected invalid references to fail")
	}
	for _, expected := range []string{"actor \"finance\" is not declared", "requirement \"missing-requirement\" is not declared"} {
		if !strings.Contains(err.Error(), expected) {
			t.Fatalf("error %q does not contain %q", err, expected)
		}
	}
}

func TestFeatureRequiresAcceptanceForMustRequirement(t *testing.T) {
	content := strings.Replace(validFeatureYAML, "requirement: supplier-create", "requirement: \"\"", 1)
	path := writeFeatureFixture(t, content)
	_, err := LoadFeature(path)
	if err == nil || !strings.Contains(err.Error(), "needs at least one linked acceptance criterion") {
		t.Fatalf("expected missing acceptance linkage error, got %v", err)
	}
}

func TestFeatureRejectsUnsafeModuleSpecPath(t *testing.T) {
	content := strings.Replace(validFeatureYAML, "specPath: .mss/modules/supplier.yaml", "specPath: ../supplier.yaml", 1)
	path := writeFeatureFixture(t, content)
	_, err := LoadFeature(path)
	if err == nil || !strings.Contains(err.Error(), "specPath must be repository-relative") {
		t.Fatalf("expected unsafe module path error, got %v", err)
	}
}

func TestFeatureRejectsUnknownFields(t *testing.T) {
	content := strings.Replace(validFeatureYAML, "owner: procurement-platform", "owner: procurement-platform\n  invented: true", 1)
	path := writeFeatureFixture(t, content)
	_, err := LoadFeature(path)
	if err == nil || !strings.Contains(err.Error(), "field invented not found") {
		t.Fatalf("expected strict YAML error, got %v", err)
	}
}

func writeFeatureFixture(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "feature.yaml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write feature fixture: %v", err)
	}
	return path
}

const validFeatureYAML = `apiVersion: mss.io/v1alpha1
kind: Feature
metadata:
  name: supplier-onboarding
  displayName: Supplier onboarding
  owner: procurement-platform
spec:
  problem: Procurement needs a governed supplier registration workflow with backend authorization.
  goals:
    - Create supplier records safely.
  nonGoals: []
  actors:
    - id: procurement
      displayName: Procurement
  modules:
    - name: supplier
      operation: create
      specPath: .mss/modules/supplier.yaml
  requirements:
    - id: supplier-create
      title: Create supplier
      description: Procurement can create a valid supplier record.
      priority: must
      actor: procurement
      module: supplier
      permission: supplier:create
      rules:
        - Supplier code is unique.
  constraints:
    - id: backend-auth
      type: security
      statement: Every supplier mutation is authorized on the backend.
  acceptance:
    - id: supplier-create-test
      requirement: supplier-create
      statement: An authorized procurement user can create a supplier.
      level: integration
      required: true
      evidence:
        - type: test
          value: modules/supplier/tests/create_test.go
  risks:
    - id: permission-drift
      description: Backend and frontend permission identifiers may diverge.
      severity: high
      mitigation: Generate permission identifiers from the module specification.
  validation:
    changed: go run ./cmd/mss verify --changed
    all: go run ./cmd/mss verify --all
  rollout:
    strategy: phased
    migration: Apply additive migrations in a test environment first.
    rollback: Disable the menu and revert the feature commit.
`
