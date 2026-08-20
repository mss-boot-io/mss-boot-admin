package project

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestDistributionSpecRequiresOneSemanticVersionCore(t *testing.T) {
	valid := DistributionSpec{
		Name:    "mss-boot-admin",
		Version: "v1.3.0",
		Backend: DistributionBackendSpec{
			Module:  "github.com/mss-boot-io/mss-boot-admin/admin",
			Version: "v1.3.0",
		},
		Frontend: DistributionFrontendSpec{
			Package: "@mss-boot-io/admin-web",
			Version: "1.3.0",
		},
	}
	if problems := valid.Validate(); len(problems) != 0 {
		t.Fatalf("valid distribution problems = %#v", problems)
	}
	prerelease := valid
	prerelease.Version = "v1.3.0-rc.1"
	prerelease.Backend.Version = "v1.3.0-rc.1"
	prerelease.Frontend.Version = "1.3.0-rc.1"
	if problems := prerelease.Validate(); len(problems) != 0 {
		t.Fatalf("valid prerelease distribution problems = %#v", problems)
	}

	tests := []struct {
		name   string
		mutate func(*DistributionSpec)
		want   string
	}{
		{name: "backend mismatch", mutate: func(value *DistributionSpec) { value.Backend.Version = "v1.2.9" }, want: "backend version"},
		{name: "frontend mismatch", mutate: func(value *DistributionSpec) { value.Frontend.Version = "1.4.0" }, want: "frontend version"},
		{name: "frontend prefix", mutate: func(value *DistributionSpec) { value.Frontend.Version = "v1.3.0" }, want: "unprefixed"},
		{name: "product prefix", mutate: func(value *DistributionSpec) { value.Version = "1.3.0" }, want: "v-prefixed"},
		{name: "prerelease mismatch", mutate: func(value *DistributionSpec) { value.Frontend.Version = "1.3.0-rc.1" }, want: "frontend version"},
		{name: "numeric prerelease leading zero", mutate: func(value *DistributionSpec) { value.Version = "v1.3.0-rc.01" }, want: "semantic version"},
		{name: "build metadata", mutate: func(value *DistributionSpec) { value.Version = "v1.3.0+preview" }, want: "semantic version"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			candidate := valid
			test.mutate(&candidate)
			problems := candidate.Validate()
			if len(problems) == 0 || !strings.Contains(strings.Join(problems, "; "), test.want) {
				t.Fatalf("Validate() problems = %#v, want %q", problems, test.want)
			}
		})
	}
}

func TestContextValidatesExplicitThinHostLayout(t *testing.T) {
	context := &Context{
		Root: t.TempDir(),
		Project: ProjectDocument{
			APIVersion: "mss.io/v1alpha1",
			Kind:       "Project",
			Metadata:   Metadata{Name: "orders-admin"},
			Spec: ProjectSpec{
				Distribution: DistributionSpec{
					Name:     "mss-boot-admin",
					Version:  "v1.3.0",
					Backend:  DistributionBackendSpec{Module: "github.com/mss-boot-io/mss-boot-admin/admin", Version: "v1.3.0"},
					Frontend: DistributionFrontendSpec{Package: "@mss-boot-io/admin-web", Version: "1.3.0"},
				},
				RepositoryLayout: map[string]string{
					"kind":           "thin-host",
					"backend":        ".",
					"frontend":       "web",
					"modules":        "internal/modules",
					"generated":      "web/src/generated",
					"businessRoutes": "web/config/business-routes.generated.ts",
					"specifications": ".mss",
				},
				Backend:  BackendSpec{Module: "github.com/acme/orders-admin"},
				Frontend: FrontendSpec{DefaultApplication: "admin-web", Applications: []FrontendApplicationSpec{{ID: "admin-web", Path: "web"}}},
			},
		},
		Capabilities: CapabilityCatalog{APIVersion: "mss.io/v1alpha1", Kind: "CapabilityCatalog"},
		Commands: CommandCatalog{APIVersion: "mss.io/v1alpha1", Kind: "CommandCatalog", Spec: CommandCatalogSpec{Commands: map[string]Command{
			"verify": {Command: "make verify", Category: "verification"},
		}}},
	}
	if err := context.Validate(); err != nil {
		t.Fatalf("Validate(thin host) error = %v", err)
	}
	context.Project.Spec.RepositoryLayout["modules"] = "../outside"
	if err := context.Validate(); err == nil || !strings.Contains(err.Error(), "escapes repository root") {
		t.Fatalf("Validate(escaping thin host) error = %v", err)
	}
}

func TestDistributionProjectAndLockSchemasAreValidJSON(t *testing.T) {
	_, sourceFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve project test source path")
	}
	schemaDirectory := filepath.Clean(filepath.Join(filepath.Dir(sourceFile), "..", "..", "..", ".mss", "schemas"))
	for _, name := range []string{"admin-distribution.schema.json", "project.schema.json", "foundation-lock.schema.json"} {
		data, err := os.ReadFile(filepath.Join(schemaDirectory, name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		var document map[string]any
		if err := json.Unmarshal(data, &document); err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		if document["$schema"] != "https://json-schema.org/draft/2020-12/schema" {
			t.Errorf("%s draft = %#v", name, document["$schema"])
		}
	}
}
