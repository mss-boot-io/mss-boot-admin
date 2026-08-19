package blueprint

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestGeneratedWorkspaceUsesLocalNestedModuleBridge(t *testing.T) {
	root := writeBlueprintFixture(t)

	blueprintPath := filepath.Join(root, ".mss", "blueprints", "management-system.yaml")
	blueprintData, err := os.ReadFile(blueprintPath)
	if err != nil {
		t.Fatalf("read fixture Blueprint: %v", err)
	}
	blueprintText := string(blueprintData)
	blueprintText = strings.Replace(
		blueprintText,
		"textExtensions: [.go, .json, .md, .mod, .sum, .yaml, .yml]",
		"textExtensions: [.go, .json, .md, .mod, .sum, .work, .yaml, .yml]",
		1,
	)
	if err := os.WriteFile(blueprintPath, []byte(blueprintText), 0o644); err != nil {
		t.Fatalf("write fixture Blueprint: %v", err)
	}

	writeFixtureFile(t, root, "go.mod", `module github.com/mss-boot-io/mss-boot-admin

go 1.26.0
`)
	writeFixtureFile(t, root, "go.work", `go 1.26.0

use (
	.
	./admin
	./mss-boot
)

replace github.com/mss-boot-io/mss-boot-admin/mss-boot v1.0.0 => ./mss-boot
`)
	writeFixtureFile(t, root, "admin/go.mod", `module github.com/mss-boot-io/mss-boot-admin/admin

go 1.26.0

require github.com/mss-boot-io/mss-boot-admin/mss-boot v1.0.0

replace github.com/mss-boot-io/mss-boot-admin/mss-boot v1.0.0 => ../mss-boot
`)
	writeFixtureFile(t, root, "admin/main.go", `package main

import (
	"fmt"

	framework "github.com/mss-boot-io/mss-boot-admin/mss-boot"
)

func main() {
	fmt.Println(framework.Value)
}
`)
	writeFixtureFile(t, root, "mss-boot/go.mod", `module github.com/mss-boot-io/mss-boot-admin/mss-boot

go 1.26.0
`)
	writeFixtureFile(t, root, "mss-boot/value.go", `package mssboot

const Value = "local"
`)
	runGit(t, root, "add", "-A")
	runGit(t, root, "commit", "-m", "test: add nested workspace bridge")

	destination := filepath.Join(t.TempDir(), "workspace-admin")
	_, err = Generate(context.Background(), Options{
		FoundationRoot: root,
		Destination:    destination,
		Application: Application{
			Name:        "workspace-admin",
			DisplayName: "Workspace Administration",
			Module:      "github.com/acme/workspace-admin",
			Repository:  "acme/workspace-admin",
		},
		Write: true,
	})
	if err != nil {
		t.Fatalf("generate downstream workspace: %v", err)
	}

	assertContains(
		t,
		filepath.Join(destination, "go.work"),
		"replace github.com/acme/workspace-admin/mss-boot v1.0.0 => ./mss-boot",
	)
	assertContains(
		t,
		filepath.Join(destination, "mss-boot", "go.mod"),
		"module github.com/acme/workspace-admin/mss-boot",
	)
	assertContains(
		t,
		filepath.Join(destination, "admin", "go.mod"),
		"require github.com/acme/workspace-admin/mss-boot v1.0.0",
	)

	command := exec.Command("go", "list", "./...", "./admin/...", "./mss-boot/...")
	command.Dir = destination
	command.Env = workspaceTestEnvironment(os.Environ())
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("generated workspace requires a remote nested module: %v\n%s", err, output)
	}
}

func TestFoundationCompatibilityWorkflowPinsIndependentIdentityEvidence(t *testing.T) {
	path := filepath.Join("..", "..", "..", ".github", "workflows", "foundation-compatibility.yml")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read Foundation compatibility workflow: %v", err)
	}
	workflow := string(data)
	for _, required := range []string{
		"CURRENT_BLUEPRINT_VERSION",
		"NEXT_BLUEPRINT_VERSION",
		"NEXT_PROJECT_BASELINE_VERSION",
		"CURRENT_FOUNDATION_COMMIT",
		"NEXT_FOUNDATION_COMMIT",
		"internal/mss/buildinfo.Version=${generator_version}",
		"internal/mss/buildinfo.Commit=${foundation_commit}",
		"internal/mss/buildinfo.Commit=${next_commit}",
		"mss_get_blueprint_status",
		"mss_plan_foundation_upgrade",
		"snapshot:foundation",
		"status != mcp_status or status != doctor_status",
		"identities != applied.get(\"toIdentities\")",
		"project.foundationVersion was conflated with an independent runtime identity",
		"templates/application/.mss/project.yaml",
		"go test -shuffle=on -count=1 ./...",
		"go vet ./...",
		"templates/application/cmd/server/main.go",
		"internal/modules/customer-extension",
	} {
		if !strings.Contains(workflow, required) {
			t.Errorf("Foundation compatibility workflow is missing identity contract %q", required)
		}
	}
	for _, forbidden := range []string{
		`text.replace("version: 0.1.0", "version: 0.1.1-ci"`,
		`blueprint.get('version') != '`,
		`go run ./cmd/mss new app compatibility-admin`,
		`go run ./cmd/mss skills validate`,
		`go test ./internal/mss/...`,
		`${DOWNSTREAM}/admin/`,
		`${CONFLICT_FOUNDATION}/admin/main.go`,
		`foundationVersion: 0.1.1-ci`,
		`project = root / ".mss/project.yaml"`,
	} {
		if strings.Contains(workflow, forbidden) {
			t.Errorf("Foundation compatibility workflow retains coupled or untraceable fixture %q", forbidden)
		}
	}
}

func writeFixtureFile(t *testing.T, root, relative, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create fixture parent %s: %v", relative, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write fixture %s: %v", relative, err)
	}
}

func workspaceTestEnvironment(source []string) []string {
	result := make([]string, 0, len(source)+2)
	for _, entry := range source {
		if strings.HasPrefix(entry, "GOWORK=") || strings.HasPrefix(entry, "GOPROXY=") {
			continue
		}
		result = append(result, entry)
	}
	return append(result, "GOWORK=auto", "GOPROXY=off")
}
