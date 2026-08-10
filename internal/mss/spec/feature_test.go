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
	if len(feature.Spec.Modules) != 1 || feature.Spec.Modules[0].Kind != FeatureModuleKindAdminModule {
		t.Fatalf("legacy module kind was not normalized: %#v", feature.Spec.Modules)
	}
	summary := feature.Summary()
	if summary["requiredAcceptance"] != 1 {
		t.Fatalf("unexpected summary: %#v", summary)
	}
	phases, ok := summary["acceptancePhases"].([]AcceptancePhaseSummary)
	if !ok || len(phases) != len(AcceptancePhases()) || phases[0].Phase != AcceptancePhaseCheckpoint || phases[0].Required != 1 {
		t.Fatalf("unexpected phase summary: %#v", summary["acceptancePhases"])
	}
}

func TestFeatureModuleContractKindValidation(t *testing.T) {
	t.Run("spec path defaults to admin module", func(t *testing.T) {
		feature, err := LoadFeature(writeFeatureFixture(t, validFeatureYAML))
		if err != nil {
			t.Fatalf("load legacy AdminModule Feature: %v", err)
		}
		if feature.Spec.Modules[0].Kind != FeatureModuleKindAdminModule {
			t.Fatalf("effective module kind = %q", feature.Spec.Modules[0].Kind)
		}
	})

	t.Run("missing spec path defaults to infrastructure", func(t *testing.T) {
		content := strings.Replace(validFeatureYAML, "      specPath: .mss/modules/supplier.yaml\n", "", 1)
		feature, err := LoadFeature(writeFeatureFixture(t, content))
		if err != nil {
			t.Fatalf("load legacy infrastructure Feature: %v", err)
		}
		if feature.Spec.Modules[0].Kind != FeatureModuleKindInfrastructure {
			t.Fatalf("effective module kind = %q", feature.Spec.Modules[0].Kind)
		}
	})

	t.Run("unsupported kind fails", func(t *testing.T) {
		content := strings.Replace(validFeatureYAML, "      operation: create", "      kind: service\n      operation: create", 1)
		_, err := LoadFeature(writeFeatureFixture(t, content))
		if err == nil || !strings.Contains(err.Error(), "modules[0].kind \"service\" is unsupported") {
			t.Fatalf("expected unsupported module kind error, got %v", err)
		}
	})
}

func TestFeatureAcceptancePhaseValidation(t *testing.T) {
	t.Run("missing defaults to checkpoint", func(t *testing.T) {
		content := strings.Replace(validFeatureYAML, "      phase: checkpoint\n", "", 1)
		feature, err := LoadFeature(writeFeatureFixture(t, content))
		if err != nil {
			t.Fatalf("load legacy feature without phase: %v", err)
		}
		if feature.Spec.Acceptance[0].Phase != AcceptancePhaseCheckpoint {
			t.Fatalf("effective phase = %q", feature.Spec.Acceptance[0].Phase)
		}
	})

	t.Run("blank defaults to checkpoint", func(t *testing.T) {
		content := strings.Replace(validFeatureYAML, "phase: checkpoint", "phase: '   '", 1)
		feature, err := LoadFeature(writeFeatureFixture(t, content))
		if err != nil {
			t.Fatalf("load legacy feature with blank phase: %v", err)
		}
		if feature.Spec.Acceptance[0].Phase != AcceptancePhaseCheckpoint {
			t.Fatalf("effective phase = %q", feature.Spec.Acceptance[0].Phase)
		}
	})

	t.Run("unsupported", func(t *testing.T) {
		content := strings.Replace(validFeatureYAML, "phase: checkpoint", "phase: release-day", 1)
		_, err := LoadFeature(writeFeatureFixture(t, content))
		if err == nil || !strings.Contains(err.Error(), "acceptance[0].phase \"release-day\" is unsupported") {
			t.Fatalf("expected unsupported phase error, got %v", err)
		}
	})

	t.Run("later required is phase local", func(t *testing.T) {
		feature, err := LoadFeature(writeFeatureFixture(t, validFeatureYAML))
		if err != nil {
			t.Fatalf("load feature: %v", err)
		}
		later := feature.Spec.Acceptance[0]
		later.ID = "publication-proof"
		later.Phase = AcceptancePhasePostPublication
		feature.Spec.Acceptance = append(feature.Spec.Acceptance, later)

		earlier := feature.AcceptanceForPhase(AcceptancePhaseCheckpoint)
		if len(earlier) != 1 || earlier[0].ID != "supplier-create-test" {
			t.Fatalf("checkpoint acceptance includes another phase: %#v", earlier)
		}
		summaries := feature.AcceptancePhaseSummaries()
		if summaries[0].Required != 1 || summaries[2].Required != 0 || summaries[len(summaries)-1].Required != 1 {
			t.Fatalf("required evidence was not phase local: %#v", summaries)
		}
	})
}

func TestFeatureJSONSchemaDefinesOptionalAcceptancePhaseEnum(t *testing.T) {
	schema := readFeatureSchema(t)
	definitions := schema["$defs"].(map[string]any)
	acceptance := definitions["acceptance"].(map[string]any)
	required := acceptance["required"].([]any)
	if containsJSONText(required, "phase") {
		t.Fatalf("legacy Feature contracts must not require phase: %#v", required)
	}
	properties := acceptance["properties"].(map[string]any)
	phase := properties["phase"].(map[string]any)
	if phase["default"] != string(AcceptancePhaseCheckpoint) {
		t.Fatalf("legacy phase default = %#v", phase["default"])
	}
	values := phase["enum"].([]any)
	if len(values) != len(AcceptancePhases()) {
		t.Fatalf("phase enum = %#v", values)
	}
	for _, expected := range AcceptancePhases() {
		if !containsJSONText(values, string(expected)) {
			t.Fatalf("phase enum does not contain %q: %#v", expected, values)
		}
	}
}

func TestFeatureJSONSchemaDefinesFeatureModuleKindEnum(t *testing.T) {
	schema := readFeatureSchema(t)
	definitions := schema["$defs"].(map[string]any)
	module := definitions["module"].(map[string]any)
	required := module["required"].([]any)
	if containsJSONText(required, "kind") {
		t.Fatalf("legacy Feature modules must not require kind: %#v", required)
	}
	properties := module["properties"].(map[string]any)
	kind := properties["kind"].(map[string]any)
	values := kind["enum"].([]any)
	expected := []FeatureModuleKind{FeatureModuleKindAdminModule, FeatureModuleKindInfrastructure}
	if len(values) != len(expected) {
		t.Fatalf("module kind enum = %#v", values)
	}
	for _, value := range expected {
		if !containsJSONText(values, string(value)) {
			t.Fatalf("module kind enum does not contain %q: %#v", value, values)
		}
	}
}

func TestRenderFeatureTemplateSetsCheckpointPhase(t *testing.T) {
	data, err := RenderFeatureTemplate("supplier-review", "Supplier review", "procurement")
	if err != nil {
		t.Fatalf("render Feature template: %v", err)
	}
	featurePath := writeFeatureFixture(t, string(data))
	feature, err := LoadFeature(featurePath)
	if err != nil {
		t.Fatalf("load rendered Feature template: %v", err)
	}
	if len(feature.Spec.Acceptance) != 1 || feature.Spec.Acceptance[0].Phase != AcceptancePhaseCheckpoint {
		t.Fatalf("rendered acceptance phase = %#v", feature.Spec.Acceptance)
	}
}

func TestRenderFeatureTemplateSetsAdminModuleKind(t *testing.T) {
	data, err := RenderFeatureTemplate("supplier-review", "Supplier review", "procurement")
	if err != nil {
		t.Fatalf("render Feature template: %v", err)
	}
	feature, err := LoadFeature(writeFeatureFixture(t, string(data)))
	if err != nil {
		t.Fatalf("load rendered Feature template: %v", err)
	}
	if len(feature.Spec.Modules) != 1 || feature.Spec.Modules[0].Kind != FeatureModuleKindAdminModule {
		t.Fatalf("rendered module kind = %#v", feature.Spec.Modules)
	}
	if !strings.Contains(string(data), "kind: admin-module") {
		t.Fatalf("rendered Feature does not make module kind explicit:\n%s", data)
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

func readFeatureSchema(t *testing.T) map[string]any {
	t.Helper()
	_, sourceFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve feature test source path")
	}
	data, err := os.ReadFile(filepath.Clean(filepath.Join(
		filepath.Dir(sourceFile), "..", "..", "..", ".mss", "schemas", "feature.schema.json",
	)))
	if err != nil {
		t.Fatalf("read FeatureSpec JSON schema: %v", err)
	}
	var schema map[string]any
	if err := json.Unmarshal(data, &schema); err != nil {
		t.Fatalf("FeatureSpec JSON schema is invalid JSON: %v", err)
	}
	return schema
}

func containsJSONText(values []any, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
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
      phase: checkpoint
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
