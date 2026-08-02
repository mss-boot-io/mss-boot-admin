package feature

import (
	"os"
	"path/filepath"
	"testing"
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
