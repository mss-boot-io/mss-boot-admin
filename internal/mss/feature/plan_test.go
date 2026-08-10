package feature

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mss-boot-io/mss-boot-admin/internal/mss/spec"
)

func TestBuildExampleSupplierFeature(t *testing.T) {
	root := repositoryRoot(t)
	plan, err := Build(Options{
		Root:        root,
		FeaturePath: ".mss/features/example-supplier-onboarding.yaml",
	})
	if err != nil {
		t.Fatalf("build supplier Feature plan: %v", err)
	}
	if !plan.Success {
		t.Fatalf("Feature plan is not successful: %#v", plan.Issues)
	}
	if plan.Feature.Name != "supplier-onboarding" {
		t.Fatalf("feature name = %q", plan.Feature.Name)
	}
	if len(plan.Modules) != 1 || plan.Modules[0].Name != "supplier" {
		t.Fatalf("unexpected module plan: %#v", plan.Modules)
	}
	if plan.Modules[0].Kind != spec.FeatureModuleKindAdminModule {
		t.Fatalf("supplier module kind = %q", plan.Modules[0].Kind)
	}
	if !plan.Modules[0].SpecValid || !plan.Modules[0].GenerationDryRun || plan.Modules[0].GeneratedOutputs < 12 {
		t.Fatalf("supplier module was not completely planned: %#v", plan.Modules[0])
	}
	if len(plan.Requirements) < 4 || len(plan.Acceptance) < 6 {
		t.Fatalf("Feature plan is incomplete: requirements=%d acceptance=%d", len(plan.Requirements), len(plan.Acceptance))
	}
	for _, requirement := range plan.Requirements {
		if requirement.Priority == "must" && len(requirement.Acceptance) == 0 {
			t.Fatalf("must requirement %s has no acceptance evidence", requirement.ID)
		}
	}
	if plan.Rollout.Strategy != "phased" || plan.Validation.Changed == "" || plan.Validation.All == "" {
		t.Fatalf("rollout or validation is incomplete: %#v %#v", plan.Rollout, plan.Validation)
	}
}

func TestPlanInfrastructureFeatureWithoutAdminModuleSpec(t *testing.T) {
	root := repositoryRoot(t)
	plan, err := Build(Options{
		Root:        root,
		FeaturePath: ".mss/features/foundation-v1-1-0-release.yaml",
	})
	if err != nil {
		t.Fatalf("build infrastructure Feature plan: %v", err)
	}
	if !plan.Success || len(plan.Modules) != 3 {
		t.Fatalf("infrastructure Feature plan = %#v", plan)
	}
	for _, module := range plan.Modules {
		if module.Kind != spec.FeatureModuleKindInfrastructure || module.SpecValid || module.GeneratedOutputs != 0 || module.Issue != "" {
			t.Fatalf("infrastructure module attempted AdminModule generation: %#v", module)
		}
	}

	module := buildModulePlan(root, spec.FeatureModule{
		Name:      "release-evidence",
		Kind:      spec.FeatureModuleKindInfrastructure,
		Operation: "extend",
		SpecPath:  ".mss/modules/does-not-exist.yaml",
	})
	if module.Issue != "" || module.SpecValid || module.GeneratedOutputs != 0 {
		t.Fatalf("infrastructure module loaded its ignored AdminModule specPath: %#v", module)
	}
}

func TestPlanAdminModuleMissingSpecPathFails(t *testing.T) {
	for _, operation := range []string{"create", "extend"} {
		t.Run(operation, func(t *testing.T) {
			plan := buildModulePlan(repositoryRoot(t), spec.FeatureModule{
				Name:      "supplier",
				Kind:      spec.FeatureModuleKindAdminModule,
				Operation: operation,
			})
			if plan.Issue != "admin-module create/extend operations require specPath" {
				t.Fatalf("missing AdminModule specPath issue = %q", plan.Issue)
			}
		})
	}
}

func TestPlanGroupsAcceptanceByPhase(t *testing.T) {
	criteria := []AcceptancePlan{
		{ID: "checkpoint-test", Phase: spec.AcceptancePhaseCheckpoint, Required: true},
		{ID: "framework-test", Phase: spec.AcceptancePhasePreFramework, Required: true},
		{ID: "publication-test", Phase: spec.AcceptancePhasePostPublication, Required: true},
		{ID: "publication-note", Phase: spec.AcceptancePhasePostPublication, Required: false},
	}
	groups := groupAcceptanceByPhase(criteria)
	if len(groups) != len(spec.AcceptancePhases()) {
		t.Fatalf("phase groups = %#v", groups)
	}
	if groups[2].Phase != spec.AcceptancePhasePreFramework || groups[2].Required != 1 || len(groups[2].Criteria) != 1 {
		t.Fatalf("pre-framework group includes another phase: %#v", groups[2])
	}
	if groups[4].Phase != spec.AcceptancePhasePostPublication || groups[4].Acceptance != 2 || groups[4].Required != 1 {
		t.Fatalf("post-publication group = %#v", groups[4])
	}

	plan := Plan{Acceptance: criteria, AcceptancePhases: groups}
	text := plan.Text()
	if !strings.Contains(text, "acceptance.pre-framework: 1 (required=1)") ||
		!strings.Contains(text, "acceptance.post-publication: 2 (required=1)") {
		t.Fatalf("phase aggregation missing from text plan:\n%s", text)
	}
}

func repositoryRoot(t *testing.T) string {
	t.Helper()
	current, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(current, ".mss", "project.yaml")); err == nil {
			return current
		}
		parent := filepath.Dir(current)
		if parent == current {
			t.Fatal("repository root was not found")
		}
		current = parent
	}
}
