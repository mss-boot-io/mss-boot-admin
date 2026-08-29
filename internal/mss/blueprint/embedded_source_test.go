package blueprint

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mss-boot-io/mss-boot-admin/internal/mss/buildinfo"
)

func TestGenerateEmbeddedWorksOutsideGitAndIsIdempotent(t *testing.T) {
	setEmbeddedReleaseBuild(t)
	foundation, err := loadEmbeddedFoundation("")
	if err != nil {
		t.Fatalf("loadEmbeddedFoundation() error = %v", err)
	}
	expectedTemplateSources := map[string]string{
		"go.mod":                            "templates/application/go.mod.tmpl",
		"cmd/server/main.go":                "templates/application/cmd/server/main.go.tmpl",
		"internal/modules/all/generated.go": "templates/application/internal/modules/all/generated.go.tmpl",
	}
	var embeddedGoSum []byte
	for _, file := range foundation.Files {
		if file.OutputPath == "go.sum" {
			embeddedGoSum = append([]byte(nil), file.Data...)
		}
		expectedSource, ok := expectedTemplateSources[file.OutputPath]
		if !ok {
			continue
		}
		if file.SourcePath != expectedSource {
			t.Fatalf("embedded %s source = %q, want %q", file.OutputPath, file.SourcePath, expectedSource)
		}
		delete(expectedTemplateSources, file.OutputPath)
	}
	if len(expectedTemplateSources) != 0 {
		t.Fatalf("embedded template outputs missing = %v", expectedTemplateSources)
	}
	if len(embeddedGoSum) == 0 {
		t.Fatal("embedded template does not contain go.sum")
	}
	working := t.TempDir()
	registry := embeddedFrontendRegistry(t)
	destination := filepath.Join(working, "standalone-admin")
	options := Options{
		Destination: destination,
		Application: Application{
			Name:        "standalone-admin",
			DisplayName: "Standalone Administration",
			Module:      "github.com/acme/standalone-admin",
			Repository:  "acme/standalone-admin",
		},
		FrontendRegistryURL: registry,
	}

	plan, err := GenerateEmbedded(context.Background(), working, options)
	if err != nil {
		t.Fatalf("GenerateEmbedded(dry-run) error = %v", err)
	}
	if !plan.DryRun || !plan.Success || plan.Identities.Foundation.Version != "1.3.7" || plan.Identities.Foundation.Commit != strings.Repeat("a", 40) {
		t.Fatalf("embedded dry-run plan = %#v", plan)
	}
	if _, err := os.Stat(destination); !os.IsNotExist(err) {
		t.Fatalf("dry-run created destination: %v", err)
	}

	options.Write = true
	options.InitializeGit = true
	written, err := GenerateEmbedded(context.Background(), working, options)
	if err != nil {
		t.Fatalf("GenerateEmbedded(write) error = %v", err)
	}
	if written.DryRun || !written.Success {
		t.Fatalf("embedded write plan = %#v", written)
	}
	for _, relative := range []string{".git/HEAD", ".mss/project.yaml", ".mss/lock.yaml", ".mss/blueprint-manifest.json", "go.mod", "web/package.json"} {
		if info, err := os.Stat(filepath.Join(destination, filepath.FromSlash(relative))); err != nil || !info.Mode().IsRegular() {
			t.Fatalf("generated %s = info:%v err:%v", relative, info, err)
		}
	}
	dockerfile, err := os.ReadFile(filepath.Join(destination, "Dockerfile"))
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"web/pnpm-lock.yaml", "web/.npmrc", "pnpm@10.34.5 install --frozen-lockfile"} {
		if !strings.Contains(string(dockerfile), expected) {
			t.Fatalf("generated Dockerfile does not contain %q", expected)
		}
	}
	if strings.Contains(string(dockerfile), "CONFIG_PROVIDER=fs") {
		t.Fatal("generated Dockerfile bakes the development-only FS configuration provider into the runtime image")
	}
	workflow, err := os.ReadFile(filepath.Join(destination, ".github", "workflows", "ci.yml"))
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		"releases/download/v1.3.7/install-mss.sh",
		"--version v1.3.7",
		"mss doctor --strict",
		"mss verify --all",
	} {
		if !strings.Contains(string(workflow), expected) {
			t.Fatalf("generated Thin Host CI does not contain %q", expected)
		}
	}
	if strings.Contains(string(workflow), "go run ./cmd/mss") {
		t.Fatal("generated Thin Host CI uses a nonexistent checkout-local cmd/mss")
	}
	gitignore, err := os.ReadFile(filepath.Join(destination, ".gitignore"))
	if err != nil {
		t.Fatal(err)
	}
	for _, generatedPath := range []string{
		"logs/",
		"web/.umi/",
		"web/.umi-production/",
		"web/src/.umi/",
		"web/src/.umi-production/",
	} {
		if !strings.Contains(string(gitignore), generatedPath+"\n") {
			t.Fatalf("generated Thin Host .gitignore does not exclude %q", generatedPath)
		}
	}
	contractsWorkflow := strings.Split(string(workflow), "\n  backend:")[0]
	corepackIndex := strings.Index(contractsWorkflow, "corepack enable")
	installIndex := strings.Index(contractsWorkflow, "corepack pnpm@10.34.5 --dir web install --frozen-lockfile")
	doctorIndex := strings.Index(contractsWorkflow, "mss doctor --strict")
	verifyIndex := strings.Index(contractsWorkflow, "mss verify --all")
	if corepackIndex < 0 || installIndex < 0 || doctorIndex < 0 || verifyIndex < 0 ||
		corepackIndex > installIndex || installIndex > doctorIndex || doctorIndex > verifyIndex {
		t.Fatal("generated Thin Host contracts CI does not install frontend dependencies before doctor and verify")
	}
	lockData, err := os.ReadFile(filepath.Join(destination, "web", "pnpm-lock.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{frontendIntegrityToken, frontendTarballToken, "NODE_AUTH_TOKEN", "_authToken"} {
		if strings.Contains(string(lockData), forbidden) {
			t.Fatalf("generated frontend lock contains forbidden value %q", forbidden)
		}
	}
	if !strings.Contains(string(lockData), "resolution: {tarball: http://127.0.0.1:") ||
		!strings.Contains(string(lockData), "/artifacts/frozen-admin-web-1.3.7.tgz, integrity: sha512-") {
		t.Fatalf("generated frontend lock did not resolve the exact tarball URL and integrity")
	}
	goModuleData, err := os.ReadFile(filepath.Join(destination, "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(goModuleData), "github.com/mss-boot-io/mss-boot-admin/mss-boot v1.3.7 // indirect") {
		t.Fatal("generated Go module does not keep the exact Framework pin in tidy readonly form")
	}
	for _, forbidden := range []string{"\nreplace ", "\nexclude ", "file:", "127.0.0.1", "localhost", "__MSS_"} {
		if strings.Contains(string(goModuleData), forbidden) {
			t.Fatalf("generated Go module contains forbidden contributor residue %q", forbidden)
		}
	}
	goSumData, err := os.ReadFile(filepath.Join(destination, "go.sum"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(goSumData, embeddedGoSum) {
		t.Fatal("generated Go checksum lock differs from the embedded template")
	}
	developmentData, err := os.ReadFile(filepath.Join(destination, ".mss", "dev.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(developmentData), "CONFIG_PROVIDER: fs") {
		t.Fatal("generated development topology does not select embedded Admin configuration")
	}
	if !strings.Contains(string(developmentData), "command: [go, run, ./cmd/server, server]") {
		t.Fatal("generated development topology does not start the Admin server subcommand")
	}
	projectData, err := os.ReadFile(filepath.Join(destination, ".mss", "project.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(projectData), "admin: go run ./cmd/server server") {
		t.Fatal("generated project entrypoint does not start the Admin server subcommand")
	}

	repeated, err := GenerateEmbedded(context.Background(), working, options)
	if err != nil {
		t.Fatalf("GenerateEmbedded(repeat) error = %v", err)
	}
	for _, change := range repeated.Changes {
		if change.Action != ActionUnchanged {
			t.Fatalf("repeat change = %#v, want unchanged", change)
		}
	}
}

func TestGenerateEmbeddedRejectsIncompleteBuildProvenance(t *testing.T) {
	originalVersion, originalCommit, originalTimestamp := buildinfo.Version, buildinfo.Commit, buildinfo.Timestamp
	t.Cleanup(func() {
		buildinfo.Version, buildinfo.Commit, buildinfo.Timestamp = originalVersion, originalCommit, originalTimestamp
	})
	buildinfo.Version, buildinfo.Commit, buildinfo.Timestamp = "v1.3.7", "deadbeef", "2026-08-25T12:34:56Z"
	_, err := GenerateEmbedded(context.Background(), t.TempDir(), Options{
		Application: Application{Name: "invalid", DisplayName: "Invalid", Module: "github.com/acme/invalid", Repository: "acme/invalid"},
	})
	if err == nil || !strings.Contains(err.Error(), "official release-built mss") {
		t.Fatalf("GenerateEmbedded(incomplete provenance) error = %v", err)
	}
}

func TestUpgradeEmbeddedUsesMatchingSourceAndPreservesBusinessFiles(t *testing.T) {
	setEmbeddedReleaseBuild(t)
	working := t.TempDir()
	registry := embeddedFrontendRegistry(t)
	applicationRoot := filepath.Join(working, "upgrade-admin")
	application := Application{
		Name:        "upgrade-admin",
		DisplayName: "Upgrade Administration",
		Module:      "github.com/acme/upgrade-admin",
		Repository:  "acme/upgrade-admin",
	}
	if _, err := GenerateEmbedded(context.Background(), working, Options{Destination: applicationRoot, Application: application, FrontendRegistryURL: registry, Write: true}); err != nil {
		t.Fatalf("GenerateEmbedded() error = %v", err)
	}
	businessPath := filepath.Join(applicationRoot, "internal", "modules", "custom", "owned.go")
	if err := os.MkdirAll(filepath.Dir(businessPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(businessPath, []byte("package custom\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	options := UpgradeOptions{
		ApplicationRoot:              applicationRoot,
		RequestedDistributionVersion: "v1.3.7",
		PreservedBusinessPaths:       []string{"internal/modules"},
		FrontendRegistryURL:          registry,
	}
	plan, err := UpgradeEmbedded(context.Background(), options)
	if err != nil {
		t.Fatalf("UpgradeEmbedded(plan) error = %v", err)
	}
	if !plan.DryRun || !plan.Success || plan.FoundationRoot != "embedded://mss/1.3.7" {
		t.Fatalf("embedded upgrade plan = %#v", plan)
	}
	if !containsEmbeddedString(plan.PreservedFiles, "internal/modules/custom/owned.go") {
		t.Fatalf("embedded upgrade preserved files = %q", plan.PreservedFiles)
	}

	options.Write = true
	applied, err := UpgradeEmbedded(context.Background(), options)
	if err != nil {
		t.Fatalf("UpgradeEmbedded(apply) error = %v", err)
	}
	if applied.DryRun {
		t.Fatalf("embedded apply remained dry-run: %#v", applied)
	}
	if data, err := os.ReadFile(businessPath); err != nil || string(data) != "package custom\n" {
		t.Fatalf("business file after embedded apply = %q err=%v", data, err)
	}
}

func containsEmbeddedString(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}

func setEmbeddedReleaseBuild(t *testing.T) {
	t.Helper()
	originalVersion, originalCommit, originalTimestamp := buildinfo.Version, buildinfo.Commit, buildinfo.Timestamp
	t.Cleanup(func() {
		buildinfo.Version, buildinfo.Commit, buildinfo.Timestamp = originalVersion, originalCommit, originalTimestamp
	})
	buildinfo.Version = "v1.3.7"
	buildinfo.Commit = strings.Repeat("a", 40)
	buildinfo.Timestamp = "2026-08-25T12:34:56Z"
}

func embeddedFrontendRegistry(t *testing.T) string {
	t.Helper()
	integrity := testFrontendIntegrity("embedded-admin-web-candidate")
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(
			writer,
			`{"name":"@mss-boot-io/admin-web","version":"1.3.7","dist":{"integrity":%q,"tarball":%q}}`,
			integrity,
			"http://"+request.Host+"/artifacts/frozen-admin-web-1.3.7.tgz",
		)
	}))
	t.Cleanup(server.Close)
	return server.URL
}
