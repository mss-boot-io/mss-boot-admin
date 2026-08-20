package doctor

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mss-boot-io/mss-boot-admin/internal/mss/blueprint"
	"github.com/mss-boot-io/mss-boot-admin/internal/mss/project"
	"gopkg.in/yaml.v3"
)

func TestParseComponentsReturnsStableDeduplicatedOrder(t *testing.T) {
	components, err := ParseComponents([]string{"agent", "docs", "agent", "backend"})
	if err != nil {
		t.Fatalf("parse components: %v", err)
	}
	want := []Component{ComponentBackend, ComponentDocs, ComponentAgent}
	if !reflect.DeepEqual(components, want) {
		t.Fatalf("components = %#v, want %#v", components, want)
	}
}

func TestParseComponentsAllOverridesSpecificSelections(t *testing.T) {
	components, err := ParseComponents([]string{"frontend", "all", "agent"})
	if err != nil {
		t.Fatalf("parse components: %v", err)
	}
	want := []Component{ComponentAll}
	if !reflect.DeepEqual(components, want) {
		t.Fatalf("components = %#v, want %#v", components, want)
	}
}

func TestParseComponentsRejectsUnknownComponent(t *testing.T) {
	if _, err := ParseComponents([]string{"database"}); err == nil {
		t.Fatal("expected unsupported component error")
	}
}

func TestRunAgentScopeDoesNotRequireFrontendOrDocsToolchains(t *testing.T) {
	root := t.TempDir()
	for _, relative := range []string{
		".mss/project.yaml",
		".mss/capabilities.yaml",
		".mss/commands.yaml",
	} {
		path := filepath.Join(root, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("create parent for %s: %v", relative, err)
		}
		if err := os.WriteFile(path, []byte("test\n"), 0o644); err != nil {
			t.Fatalf("write %s: %v", relative, err)
		}
	}
	writeDoctorSourceSentinel(t, root)

	projectContext := &project.Context{
		Root: root,
		Project: project.ProjectDocument{
			Metadata: project.Metadata{Name: "doctor-test", Repository: "acme/doctor-test"},
		},
	}
	report := Run(context.Background(), projectContext, WithComponents(ComponentAgent))

	if !reflect.DeepEqual(report.Components, []Component{ComponentAgent}) {
		t.Fatalf("report components = %#v", report.Components)
	}
	checks := make(map[string]Check, len(report.Checks))
	for _, check := range report.Checks {
		checks[check.ID] = check
	}
	for _, required := range []string{
		"contract:admin-distribution",
		"file:.mss/project.yaml",
		"file:.mss/capabilities.yaml",
		"file:.mss/commands.yaml",
		"tool:git",
		"tool:go",
		"snapshot:foundation",
	} {
		if _, ok := checks[required]; !ok {
			t.Errorf("required Agent check %q is missing", required)
		}
	}
	for _, excluded := range []string{
		"file:web/antd-v6/pnpm-lock.yaml",
		"file:docs/pnpm-lock.yaml",
		"tool:node",
		"tool:pnpm",
		"port:frontend-port",
	} {
		if _, ok := checks[excluded]; ok {
			t.Errorf("unrelated check %q must not be part of Agent scope", excluded)
		}
	}
	snapshot := checks["snapshot:foundation"]
	if snapshot.Status != StatusInfo || snapshot.Required || snapshot.Snapshot != nil {
		t.Fatalf("Foundation source snapshot check = %#v", snapshot)
	}
}

func TestAdminDistributionCheckPassesOnlyWhenSnapshotIsAligned(t *testing.T) {
	root := t.TempDir()
	distribution := doctorDistribution("v1.3.0")
	writeDoctorDistributionSourceSentinel(t, root, distribution)
	projectContext := &project.Context{
		Root: root,
		Project: project.ProjectDocument{
			Metadata: project.Metadata{Name: "doctor-test", Repository: "acme/doctor-test"},
			Spec: project.ProjectSpec{
				Distribution:     distribution,
				RepositoryLayout: map[string]string{"kind": "thin-host"},
			},
		},
	}

	check := adminDistributionCheck(projectContext)
	if check.Status != StatusPass || !check.Required {
		t.Fatalf("aligned Distribution check = %#v", check)
	}
	for _, expected := range []string{"mss-boot-admin@v1.3.0", "backend and frontend versions exactly match", "snapshot aligned"} {
		if !strings.Contains(check.Detail, expected) {
			t.Fatalf("aligned Distribution detail %q does not contain %q", check.Detail, expected)
		}
	}

	projectContext.Project.Spec.Distribution = doctorDistribution("v1.4.0")
	check = adminDistributionCheck(projectContext)
	if check.Status != StatusFail || !check.Required || !strings.Contains(check.Detail, "project pins") {
		t.Fatalf("mismatched Distribution check = %#v", check)
	}
}

func TestRunThinHostScopesRequireOnlyHostBackendAndFrontendGlue(t *testing.T) {
	root := t.TempDir()
	distribution := doctorDistribution("v1.3.0")
	writeDoctorDistributionSourceSentinel(t, root, distribution)
	for _, relative := range []string{
		"go.mod",
		"cmd/server/main.go",
		"internal/modules/all/generated.go",
		"web/package.json",
		"web/tsconfig.json",
		"web/config/config.ts",
		"web/mss-admin.config.ts",
		"web/src/app.tsx",
		"web/src/access.ts",
	} {
		path := filepath.Join(root, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("create %s parent: %v", relative, err)
		}
		if err := os.WriteFile(path, []byte("test\n"), 0o644); err != nil {
			t.Fatalf("write %s: %v", relative, err)
		}
	}
	projectContext := &project.Context{
		Root: root,
		Project: project.ProjectDocument{
			Metadata: project.Metadata{Name: "doctor-test", Repository: "acme/doctor-test"},
			Spec: project.ProjectSpec{
				Distribution: distribution,
				RepositoryLayout: map[string]string{
					"kind":     "thin-host",
					"frontend": "web",
					"modules":  "internal/modules",
				},
			},
		},
	}
	report := Run(context.Background(), projectContext, WithComponents(ComponentBackend, ComponentFramework, ComponentFrontend))
	checks := checksByID(report.Checks)
	for _, required := range []string{
		"file:go.mod",
		"file:cmd/server/main.go",
		"file:internal/modules/all/generated.go",
		"file:web/package.json",
		"file:web/tsconfig.json",
		"file:web/config/config.ts",
		"file:web/mss-admin.config.ts",
		"file:web/src/app.tsx",
		"file:web/src/access.ts",
	} {
		if check, ok := checks[required]; !ok || check.Status != StatusPass {
			t.Errorf("Thin Host check %s = %#v", required, check)
		}
	}
	for _, forbidden := range []string{"file:admin/go.mod", "file:mss-boot/go.mod", "file:web/pnpm-lock.yaml"} {
		if _, ok := checks[forbidden]; ok {
			t.Errorf("Thin Host doctor unexpectedly requires %s", forbidden)
		}
	}
}

func TestRunFrontendScopeChecksTheOnlyConfiguredApplication(t *testing.T) {
	root := t.TempDir()
	for _, relative := range []string{
		"web/antd-v6/pnpm-lock.yaml",
	} {
		path := filepath.Join(root, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("create parent for %s: %v", relative, err)
		}
		if err := os.WriteFile(path, []byte("lockfileVersion: '9.0'\n"), 0o644); err != nil {
			t.Fatalf("write %s: %v", relative, err)
		}
	}
	projectContext := &project.Context{Root: root}
	projectContext.Project.Spec.RepositoryLayout = map[string]string{"frontend": "web/antd-v6"}
	projectContext.Project.Spec.Frontend.DefaultApplication = "antd-v6"
	projectContext.Project.Spec.Frontend.Applications = []project.FrontendApplicationSpec{
		{ID: "antd-v6", Path: "web/antd-v6", DevelopmentPort: 18124},
	}

	report := Run(context.Background(), projectContext, WithComponents(ComponentFrontend))
	checks := checksByID(report.Checks)
	for _, required := range []string{
		"file:web/antd-v6/pnpm-lock.yaml",
		"port:frontend-port",
	} {
		if _, ok := checks[required]; !ok {
			t.Errorf("configured frontend check %q is missing", required)
		}
	}
}

func TestRunAgentScopeRequiresValidGeneratedSnapshot(t *testing.T) {
	root := t.TempDir()
	writeDoctorAgentContracts(t, root)
	writeDoctorGeneratedSnapshot(t, root)
	projectContext := doctorProjectContext(root)

	report := Run(context.Background(), projectContext, WithComponents(ComponentAgent))
	checks := checksByID(report.Checks)
	snapshot := checks["snapshot:foundation"]
	if snapshot.Status != StatusPass || !snapshot.Required || snapshot.Snapshot == nil {
		t.Fatalf("generated snapshot check = %#v", snapshot)
	}
	if snapshot.Snapshot.Identities.Foundation.Version != "1.1.0" ||
		snapshot.Snapshot.Identities.Blueprint.Version != "0.2.1-ci" ||
		snapshot.Snapshot.Identities.Generator.Commit != strings.Repeat("b", 40) {
		t.Fatalf("doctor omitted independent identities: %#v", snapshot.Snapshot.Identities)
	}
}

func TestRunAgentScopeFailsMalformedCurrentSnapshotWithoutSourceFallback(t *testing.T) {
	root := t.TempDir()
	writeDoctorAgentContracts(t, root)
	writeDoctorSourceSentinel(t, root)
	manifestPath := filepath.Join(root, ".mss", "blueprint-manifest.json")
	if err := os.WriteFile(manifestPath, []byte("{}\n"), 0o644); err != nil {
		t.Fatalf("write malformed current manifest: %v", err)
	}

	report := Run(context.Background(), doctorProjectContext(root), WithComponents(ComponentAgent))
	snapshot := checksByID(report.Checks)["snapshot:foundation"]
	if report.Ready || snapshot.Status != StatusFail || !snapshot.Required {
		t.Fatalf("malformed snapshot readiness = ready:%t check:%#v", report.Ready, snapshot)
	}
}

func writeDoctorAgentContracts(t *testing.T, root string) {
	t.Helper()
	for _, relative := range []string{".mss/project.yaml", ".mss/capabilities.yaml", ".mss/commands.yaml"} {
		path := filepath.Join(root, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("create parent for %s: %v", relative, err)
		}
		if err := os.WriteFile(path, []byte("test\n"), 0o644); err != nil {
			t.Fatalf("write %s: %v", relative, err)
		}
	}
}

func doctorProjectContext(root string) *project.Context {
	return &project.Context{
		Root: root,
		Project: project.ProjectDocument{
			Metadata: project.Metadata{
				Name:       "doctor-test",
				Repository: "acme/doctor-test",
			},
			Spec: project.ProjectSpec{
				// These deliberately differ from the snapshot root module and
				// Foundation identity; doctor must not compare either field.
				FoundationVersion: "0.1.99-unrelated",
				Backend: project.BackendSpec{
					Module: "github.com/acme/doctor-test/admin",
				},
			},
		},
	}
}

func checksByID(checks []Check) map[string]Check {
	result := make(map[string]Check, len(checks))
	for _, check := range checks {
		result[check.ID] = check
	}
	return result
}

func writeDoctorSourceSentinel(t *testing.T, root string) {
	t.Helper()
	path := filepath.Join(root, ".mss", "lock.yaml")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create source lock parent: %v", err)
	}
	data := `apiVersion: mss.io/v1alpha1
kind: FoundationLock
metadata:
  project: doctor-test
spec:
  foundation:
    repository: acme/doctor-test
    version: 0.1.0
    channel: development
  blueprint:
    name: management-system
    version: 0.1.0
  contracts:
    project: v1alpha1
  generatedBy:
    tool: mss
    version: 0.1.0-dev
  modules: {}
  upgrades: []
`
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatalf("write source lock: %v", err)
	}
}

func doctorDistribution(version string) project.DistributionSpec {
	core := strings.TrimPrefix(version, "v")
	return project.DistributionSpec{
		Name:    "mss-boot-admin",
		Version: version,
		Backend: project.DistributionBackendSpec{
			Module:  "github.com/mss-boot-io/mss-boot-admin/admin",
			Version: version,
		},
		Frontend: project.DistributionFrontendSpec{
			Package: "@mss-boot-io/admin-web",
			Version: core,
		},
	}
}

func writeDoctorDistributionSourceSentinel(t *testing.T, root string, distribution project.DistributionSpec) {
	t.Helper()
	path := filepath.Join(root, ".mss", "lock.yaml")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create source lock parent: %v", err)
	}
	data := fmt.Sprintf(`apiVersion: mss.io/v1alpha1
kind: FoundationLock
metadata:
  project: doctor-test
spec:
  distribution:
    name: %s
    version: %s
    backend:
      module: %s
      version: %s
    frontend:
      package: %q
      version: %s
  foundation:
    repository: acme/doctor-test
    version: 0.1.0
    channel: development
  blueprint:
    name: management-system
    version: 0.4.0
  contracts:
    project: v1alpha1
    adminDistribution: v1alpha1
  generatedBy:
    tool: mss
    version: 1.3.0-dev
  modules: {}
  upgrades: []
`, distribution.Name, distribution.Version, distribution.Backend.Module, distribution.Backend.Version, distribution.Frontend.Package, distribution.Frontend.Version)
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatalf("write Distribution source lock: %v", err)
	}
}

func writeDoctorGeneratedSnapshot(t *testing.T, root string) {
	t.Helper()
	files := map[string]blueprint.ManifestFile{
		"AGENTS.md": {SHA256: strings.Repeat("f", 64), Mode: 0o644, Size: 7},
	}
	identities := blueprint.IdentitySet{
		Foundation: blueprint.FoundationIdentity{
			Repository: "mss-boot-io/mss-boot-admin",
			Version:    "1.1.0",
			Commit:     strings.Repeat("a", 40),
			Timestamp:  "2026-08-10T12:00:00Z",
			Channel:    "candidate",
			Source:     ".mss/release-policy.yaml",
		},
		Blueprint: blueprint.BlueprintIdentity{
			Name:    "management-system",
			Version: "0.2.1-ci",
			SHA256:  strings.Repeat("c", 64),
		},
		Generator: blueprint.GeneratorIdentity{
			Tool:    "mss",
			Version: "1.1.0",
			Commit:  strings.Repeat("b", 40),
		},
		Snapshot: blueprint.DownstreamSnapshotIdentity{
			Project:    "doctor-test",
			Module:     "github.com/acme/doctor-test",
			Repository: "acme/doctor-test",
		},
	}
	identities.Snapshot.SHA256 = doctorSnapshotDigest(t, identities, files)
	records := blueprint.SnapshotRecordPaths{
		LockPath:     ".mss/lock.yaml",
		ManifestPath: ".mss/blueprint-manifest.json",
	}
	lock := blueprint.FoundationLock{
		APIVersion: "mss.io/v1alpha2",
		Kind:       "FoundationLock",
		Metadata:   blueprint.FoundationLockMetadata{Project: "doctor-test"},
		Spec: blueprint.FoundationLockSpec{
			Identities: identities,
			Records:    records,
			Contracts:  map[string]string{"project": "v1alpha1"},
			Modules:    map[string]any{},
			Upgrades:   []any{},
		},
	}
	lockData, err := yaml.Marshal(lock)
	if err != nil {
		t.Fatalf("marshal generated lock: %v", err)
	}
	lockSum := sha256.Sum256(lockData)
	manifest := blueprint.Manifest{
		APIVersion: "mss.io/v1alpha2",
		Kind:       "BlueprintManifest",
		Metadata: blueprint.ManifestMetadata{
			Project:              identities.Snapshot.Project,
			Module:               identities.Snapshot.Module,
			Repository:           identities.Snapshot.Repository,
			Blueprint:            identities.Blueprint.Name,
			BlueprintVersion:     identities.Blueprint.Version,
			FoundationRepository: identities.Foundation.Repository,
			FoundationCommit:     identities.Foundation.Commit,
			FoundationTimestamp:  identities.Foundation.Timestamp,
			GeneratorVersion:     identities.Generator.Version,
			GeneratorCommit:      identities.Generator.Commit,
		},
		Identities: identities,
		Records: blueprint.ManifestRecords{
			SnapshotRecordPaths: records,
			LockSHA256:          hex.EncodeToString(lockSum[:]),
		},
		Files: files,
	}
	manifestData, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		t.Fatalf("marshal generated manifest: %v", err)
	}
	manifestData = append(manifestData, '\n')
	for relative, data := range map[string][]byte{
		records.LockPath:     lockData,
		records.ManifestPath: manifestData,
	} {
		path := filepath.Join(root, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("create snapshot parent: %v", err)
		}
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatalf("write snapshot record %s: %v", relative, err)
		}
	}
}

func doctorSnapshotDigest(t *testing.T, identities blueprint.IdentitySet, files map[string]blueprint.ManifestFile) string {
	t.Helper()
	type application struct {
		Project    string `json:"project"`
		Module     string `json:"module"`
		Repository string `json:"repository"`
	}
	type file struct {
		Path   string `json:"path"`
		SHA256 string `json:"sha256"`
		Mode   uint32 `json:"mode"`
		Size   int64  `json:"size"`
	}
	type input struct {
		Application application                  `json:"application"`
		Foundation  blueprint.FoundationIdentity `json:"foundation"`
		Blueprint   blueprint.BlueprintIdentity  `json:"blueprint"`
		Generator   blueprint.GeneratorIdentity  `json:"generator"`
		Files       []file                       `json:"files"`
	}
	value := input{
		Application: application{
			Project:    identities.Snapshot.Project,
			Module:     identities.Snapshot.Module,
			Repository: identities.Snapshot.Repository,
		},
		Foundation: identities.Foundation,
		Blueprint:  identities.Blueprint,
		Generator:  identities.Generator,
		Files: []file{{
			Path:   "AGENTS.md",
			SHA256: files["AGENTS.md"].SHA256,
			Mode:   uint32(fs.FileMode(files["AGENTS.md"].Mode).Perm()),
			Size:   files["AGENTS.md"].Size,
		}},
	}
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal snapshot digest input: %v", err)
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
