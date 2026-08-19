package generator

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mss-boot-io/mss-boot-admin/internal/mss/project"
)

func TestGenerateUsesThinHostProjectLayoutWithoutCopiedTemplates(t *testing.T) {
	root := t.TempDir()
	document := thinHostProjectDocument()
	module := generatorTestModule()
	module.SourcePath = ".mss/modules/example-supplier.yaml"

	plan, err := Generate(module, Options{Root: root, Write: true, Project: &document})
	if err != nil {
		t.Fatalf("Generate(thin host) error = %v", err)
	}
	if plan.LayoutKind != layoutThinHost {
		t.Fatalf("layout kind = %q, want %q", plan.LayoutKind, layoutThinHost)
	}
	for _, required := range []string{
		"internal/modules/supplier/model_generated.go",
		"internal/modules/supplier/module_generated.go",
		"internal/modules/all/generated.go",
		"web/config/business-routes.generated.ts",
		"web/src/generated/modules/supplier/SupplierPage.tsx",
		"web/src/generated/locales/en-US.ts",
		"web/src/pages/generated/Supplier/index.tsx",
	} {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(required))); err != nil {
			t.Errorf("generated thin-host output %s: %v", required, err)
		}
	}
	for _, forbidden := range []string{"admin", "mss-boot", "web/antd-v6", "templates"} {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(forbidden))); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("thin-host generation created forbidden path %s: %v", forbidden, err)
		}
	}

	assertGeneratedContains(t, root, "internal/modules/all/generated.go", `module0 "github.com/acme/orders-admin/internal/modules/supplier"`)
	assertGeneratedContains(t, root, "internal/modules/supplier/module_generated.go", `"github.com/mss-boot-io/mss-boot-admin/admin/business"`)
	assertGeneratedContains(t, root, "web/src/generated/modules/supplier/SupplierPage.tsx", `from '@mss-boot-io/admin-web/runtime'`)
	assertGeneratedContains(t, root, "web/src/generated/modules/supplier/contract.ts", `from '@mss-boot-io/admin-web/runtime'`)

	second, err := Generate(module, Options{Root: root, Check: true, Project: &document})
	if err != nil {
		t.Fatalf("Generate(thin host check) error = %v", err)
	}
	for _, change := range second.Changes {
		if change.Action != ActionUnchanged {
			t.Fatalf("thin-host second generation changed %s: %s", change.Path, change.Action)
		}
	}
	custom := filepath.Join(root, "internal", "modules", "supplier", "custom.go")
	if err := os.WriteFile(custom, []byte("package supplier\n\nconst Custom = true\n"), 0o644); err != nil {
		t.Fatalf("write custom business file: %v", err)
	}
	if _, err := Generate(module, Options{Root: root, Write: true, Project: &document}); err != nil {
		t.Fatalf("regenerate with custom business file: %v", err)
	}
	data, err := os.ReadFile(custom)
	if err != nil || !strings.Contains(string(data), "const Custom = true") {
		t.Fatalf("custom business file was not preserved: %q, %v", data, err)
	}
}

func TestGenerateRejectsEscapingThinHostLayoutBeforeWriting(t *testing.T) {
	root := t.TempDir()
	document := thinHostProjectDocument()
	document.Spec.RepositoryLayout["modules"] = "../outside"
	module := generatorTestModule()
	module.SourcePath = ".mss/modules/example-supplier.yaml"
	plan, err := Generate(module, Options{Root: root, Write: true, Project: &document})
	if err == nil || !strings.Contains(err.Error(), "confined") {
		t.Fatalf("Generate(escaping layout) plan=%#v error=%v", plan, err)
	}
	entries, readErr := os.ReadDir(root)
	if readErr != nil {
		t.Fatalf("read target root: %v", readErr)
	}
	if len(entries) != 0 {
		t.Fatalf("escaping layout wrote entries before validation: %#v", entries)
	}
}

func thinHostProjectDocument() project.ProjectDocument {
	return project.ProjectDocument{
		APIVersion: "mss.io/v1alpha1",
		Kind:       "Project",
		Metadata:   project.Metadata{Name: "orders-admin", Repository: "acme/orders-admin"},
		Spec: project.ProjectSpec{
			Distribution: project.DistributionSpec{
				Name:     "mss-boot-admin",
				Version:  "v1.3.0",
				Backend:  project.DistributionBackendSpec{Module: "github.com/mss-boot-io/mss-boot-admin/admin", Version: "v1.3.0"},
				Frontend: project.DistributionFrontendSpec{Package: "@mss-boot-io/admin-web", Version: "1.3.0"},
			},
			RepositoryLayout: map[string]string{
				"kind":           "thin-host",
				"backend":        ".",
				"frontend":       "web",
				"modules":        "internal/modules",
				"generated":      "web/src/generated",
				"businessRoutes": "web/config/business-routes.generated.ts",
				"specifications": ".mss",
				"documentation":  "docs",
			},
			Backend: project.BackendSpec{
				Module:          "github.com/acme/orders-admin",
				FrameworkModule: "github.com/mss-boot-io/mss-boot-admin/mss-boot",
			},
		},
	}
}

func assertGeneratedContains(t *testing.T, root, relative, expected string) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(relative)))
	if err != nil {
		t.Fatalf("read generated %s: %v", relative, err)
	}
	if !strings.Contains(string(data), expected) {
		t.Fatalf("generated %s does not contain %q:\n%s", relative, expected, data)
	}
}
