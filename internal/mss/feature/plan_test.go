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

func TestBuildFoundationV110GeneratorBlueprintFeature(t *testing.T) {
	root := repositoryRoot(t)
	plan, err := Build(Options{
		Root:        root,
		FeaturePath: ".mss/features/foundation-v1-1-0-generator-blueprint.yaml",
	})
	if err != nil {
		t.Fatalf("build v1.1.0 Generator/Blueprint Feature plan: %v", err)
	}
	if !plan.Success {
		t.Fatalf("Generator/Blueprint Feature plan is not successful: %#v", plan.Issues)
	}
	if len(plan.Modules) != 7 {
		t.Fatalf("Generator/Blueprint module plans = %d, want 7", len(plan.Modules))
	}

	adminModules := 0
	for _, module := range plan.Modules {
		if module.Kind == spec.FeatureModuleKindInfrastructure {
			if module.SpecValid || module.GeneratedOutputs != 0 || module.Issue != "" {
				t.Fatalf("infrastructure module attempted AdminModule generation: %#v", module)
			}
			continue
		}
		adminModules++
		if module.Name != "supplier" || module.SpecName != "supplier" || module.SpecPath != ".mss/modules/example-supplier.yaml" {
			t.Fatalf("unexpected flagship AdminModule plan: %#v", module)
		}
		if !module.SpecValid || !module.GenerationDryRun || module.GeneratedOutputs < 12 || module.Issue != "" {
			t.Fatalf("flagship supplier module was not completely planned: %#v", module)
		}
	}
	if adminModules != 1 {
		t.Fatalf("flagship AdminModule plans = %d, want exactly one supplier", adminModules)
	}

	foundDeterministicRequirement := false
	for _, requirement := range plan.Requirements {
		if requirement.ID == "keep-generation-deterministic" && requirement.Module != "supplier" {
			t.Fatalf("golden module requirement points to %q, want supplier", requirement.Module)
		}
		if requirement.ID == "keep-generation-deterministic" {
			foundDeterministicRequirement = true
		}
	}
	if !foundDeterministicRequirement {
		t.Fatal("Generator/Blueprint Feature plan is missing keep-generation-deterministic")
	}
}

func TestCanonicalVerticalModulePathContracts(t *testing.T) {
	root := repositoryRoot(t)
	paths := []string{
		"AGENTS.md",
		".mss/project.yaml",
	}
	featurePaths, err := filepath.Glob(filepath.Join(root, ".mss", "features", "*.yaml"))
	if err != nil {
		t.Fatalf("list Feature contracts: %v", err)
	}
	for _, path := range featurePaths {
		relative, err := filepath.Rel(root, path)
		if err != nil {
			t.Fatalf("make Feature path relative: %v", err)
		}
		paths = append(paths, filepath.ToSlash(relative))
	}

	for _, relative := range paths {
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(relative)))
		if err != nil {
			t.Fatalf("read canonical-path contract %s: %v", relative, err)
		}
		for lineNumber, line := range strings.Split(string(data), "\n") {
			searchFrom := 0
			for {
				offset := strings.Index(line[searchFrom:], "modules/")
				if offset < 0 {
					break
				}
				index := searchFrom + offset
				if index == 0 || line[index-1] != '/' {
					t.Fatalf("%s:%d contains non-canonical root module path: %s", relative, lineNumber+1, strings.TrimSpace(line))
				}
				searchFrom = index + len("modules/")
			}
		}
	}

	agentContract, err := os.ReadFile(filepath.Join(root, "AGENTS.md"))
	if err != nil {
		t.Fatalf("read AGENTS.md: %v", err)
	}
	if !strings.Contains(string(agentContract), "`admin/modules/<name>/`") {
		t.Fatal("AGENTS.md does not declare admin/modules/<name>/ as the vertical-module path")
	}
	projectContract, err := os.ReadFile(filepath.Join(root, ".mss", "project.yaml"))
	if err != nil {
		t.Fatalf("read project contract: %v", err)
	}
	if !strings.Contains(string(projectContract), "modules: admin/modules") {
		t.Fatal(".mss/project.yaml does not declare admin/modules as repositoryLayout.modules")
	}
	if _, err := os.Stat(filepath.Join(root, "modules")); err == nil {
		t.Fatal("root modules directory conflicts with canonical admin/modules ownership")
	} else if !os.IsNotExist(err) {
		t.Fatalf("inspect root modules directory: %v", err)
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
