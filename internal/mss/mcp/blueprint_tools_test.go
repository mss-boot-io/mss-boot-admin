package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mss-boot-io/mss-boot-admin/internal/mss/blueprint"
	"github.com/mss-boot-io/mss-boot-admin/internal/mss/buildinfo"
	"github.com/mss-boot-io/mss-boot-admin/internal/mss/project"
)

func TestBlueprintToolDefinitionsAreComplete(t *testing.T) {
	definitions := blueprintToolDefinitions()
	if len(definitions) != 4 {
		t.Fatalf("blueprint tool count = %d, want 4", len(definitions))
	}
	seen := make(map[string]bool, len(definitions))
	for _, definition := range definitions {
		if definition.Name == "" {
			t.Fatal("blueprint tool has an empty name")
		}
		if len(definition.InputSchema) == 0 {
			t.Fatalf("tool %s has no input schema", definition.Name)
		}
		if seen[definition.Name] {
			t.Fatalf("duplicate blueprint tool %s", definition.Name)
		}
		seen[definition.Name] = true
	}
	for _, expected := range []string{
		"mss_plan_application",
		"mss_get_blueprint_status",
		"mss_plan_foundation_upgrade",
		"mss_apply_foundation_upgrade",
	} {
		if !seen[expected] {
			t.Fatalf("missing blueprint tool %s", expected)
		}
	}
	for _, definition := range definitions {
		required, _ := definition.InputSchema["required"].([]string)
		switch definition.Name {
		case "mss_plan_foundation_upgrade":
			if containsString(required, "foundationRoot") {
				t.Fatalf("%s still requires a Foundation checkout: %v", definition.Name, required)
			}
		case "mss_apply_foundation_upgrade":
			if containsString(required, "foundationRoot") || !containsString(required, "confirm") {
				t.Fatalf("%s required fields = %v, want only confirmation", definition.Name, required)
			}
		}
	}
}

func TestApplyFoundationUpgradeRequiresConfirmation(t *testing.T) {
	server := &Server{Root: t.TempDir()}
	result, known := server.callBlueprintTool(context.Background(), "mss_apply_foundation_upgrade", map[string]any{
		"foundationRoot": t.TempDir(),
	})
	if !known {
		t.Fatal("apply foundation upgrade tool was not recognized")
	}
	if !result.IsError {
		t.Fatalf("unconfirmed upgrade did not return an error: %#v", result)
	}
}

func TestAllToolDefinitionsIncludeBlueprintTools(t *testing.T) {
	definitions := append([]Tool(nil), tools()...)
	seen := make(map[string]bool, len(definitions))
	for _, definition := range definitions {
		seen[definition.Name] = true
	}
	for _, expected := range []string{
		"mss_plan_application",
		"mss_get_blueprint_status",
		"mss_plan_foundation_upgrade",
		"mss_apply_foundation_upgrade",
	} {
		if !seen[expected] {
			t.Fatalf("tools() does not include %s", expected)
		}
	}
}

func TestFoundationUpgradeApplicationDefersRootModuleToSnapshot(t *testing.T) {
	projectContext := &project.Context{
		Project: project.ProjectDocument{
			Metadata: project.Metadata{
				Name:        "customer-admin",
				DisplayName: "Customer Administration",
				Repository:  "acme/customer-admin",
			},
			Spec: project.ProjectSpec{
				FoundationVersion: "0.1.99-unrelated",
				Backend: project.BackendSpec{
					Module: "github.com/acme/customer-admin/admin",
				},
			},
		},
	}

	application := foundationUpgradeApplication(projectContext)
	if application.Name != "customer-admin" || application.Repository != "acme/customer-admin" {
		t.Fatalf("upgrade application = %#v", application)
	}
	if application.Module != "" {
		t.Fatalf("nested backend module escaped into root snapshot identity: %q", application.Module)
	}
	if nilApplication := foundationUpgradeApplication(nil); nilApplication.Name != "" || nilApplication.Module != "" {
		t.Fatalf("nil project context produced %#v", nilApplication)
	}
}

func TestServePlansEmbeddedApplicationFromEmptyWorkingDirectory(t *testing.T) {
	setMCPReleaseBuild(t)
	working := t.TempDir()
	requests := []string{
		`{"jsonrpc":"2.0","id":"plan","method":"tools/call","params":{"name":"mss_plan_application","arguments":{"name":"empty-admin","displayName":"Empty Administration","module":"github.com/acme/empty-admin","repository":"acme/empty-admin"}}}`,
		`{"jsonrpc":"2.0","id":"project","method":"tools/call","params":{"name":"mss_get_project_context","arguments":{}}}`,
	}
	var output bytes.Buffer
	server := &Server{Root: working, ContributorFrontendRegistryURL: mcpFrontendRegistry(t)}
	if err := server.Serve(context.Background(), bytes.NewBufferString(joinLines(requests)), &output); err != nil {
		t.Fatalf("serve MCP requests from empty directory: %v", err)
	}
	responses := decodeResponses(t, output.Bytes())
	if len(responses) != 2 {
		t.Fatalf("response count = %d, want 2; output=%s", len(responses), output.String())
	}

	plan := structuredResult[blueprint.Plan](t, responses[0])
	if !plan.DryRun || !plan.Success {
		t.Fatalf("embedded application plan = %#v", plan)
	}
	if plan.Destination != filepath.Join(working, ".mss", "output", "empty-admin") {
		t.Fatalf("plan destination = %q", plan.Destination)
	}
	if plan.Identities.Foundation.Version != "1.3.3" || plan.Identities.Foundation.Commit != strings.Repeat("a", 40) {
		t.Fatalf("embedded release identity = %#v", plan.Identities.Foundation)
	}
	entries, err := os.ReadDir(working)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("read-only plan wrote into empty directory: %#v", entries)
	}
	projectCall := objectResult(t, responses[1])
	if projectCall["isError"] != true {
		t.Fatalf("project-only tool did not fail closed: %#v", projectCall)
	}
}

func TestApplicationPlanRejectsDestinationsOutsideWorkingRoot(t *testing.T) {
	root := t.TempDir()
	server := &Server{Root: root}
	for _, destination := range []string{"../outside", filepath.Join(string(filepath.Separator), "outside")} {
		result, known := server.callBlueprintTool(context.Background(), "mss_plan_application", map[string]any{
			"name":        "safe-admin",
			"module":      "github.com/acme/safe-admin",
			"destination": destination,
		})
		if !known || !result.IsError || !strings.Contains(result.Content[0].Text, "working root") {
			t.Fatalf("destination %q = known:%v result:%#v", destination, known, result)
		}
	}
	inside := filepath.Join(root, "inside")
	resolved, err := resolveApplicationPlanDestination(root, inside)
	if err != nil || resolved != inside {
		t.Fatalf("absolute destination inside working root = %q err=%v", resolved, err)
	}
}

func TestEmbeddedUpgradeLifecyclePreservesThinHostBusinessFiles(t *testing.T) {
	setMCPReleaseBuild(t)
	working := t.TempDir()
	registry := mcpFrontendRegistry(t)
	applicationRoot := filepath.Join(working, "upgrade-admin")
	application := blueprint.Application{
		Name:        "upgrade-admin",
		DisplayName: "Upgrade Administration",
		Module:      "github.com/acme/upgrade-admin",
		Repository:  "acme/upgrade-admin",
	}
	if _, err := blueprint.GenerateEmbedded(context.Background(), working, blueprint.Options{
		Destination:         applicationRoot,
		Application:         application,
		FrontendRegistryURL: registry,
		Write:               true,
	}); err != nil {
		t.Fatalf("generate embedded Thin Host: %v", err)
	}
	businessPath := filepath.Join(applicationRoot, "internal", "modules", "custom", "owned.go")
	if err := os.MkdirAll(filepath.Dir(businessPath), 0o755); err != nil {
		t.Fatal(err)
	}
	const businessContent = "package custom\n"
	if err := os.WriteFile(businessPath, []byte(businessContent), 0o644); err != nil {
		t.Fatal(err)
	}

	requests := []string{
		`{"jsonrpc":"2.0","id":"dry-run","method":"tools/call","params":{"name":"mss_plan_foundation_upgrade","arguments":{}}}`,
		`{"jsonrpc":"2.0","id":"apply","method":"tools/call","params":{"name":"mss_apply_foundation_upgrade","arguments":{"confirm":true}}}`,
		`{"jsonrpc":"2.0","id":"no-op","method":"tools/call","params":{"name":"mss_plan_foundation_upgrade","arguments":{}}}`,
	}
	var output bytes.Buffer
	server := &Server{Root: applicationRoot, ContributorFrontendRegistryURL: registry}
	if err := server.Serve(context.Background(), bytes.NewBufferString(joinLines(requests)), &output); err != nil {
		t.Fatalf("serve embedded upgrade lifecycle: %v", err)
	}
	responses := decodeResponses(t, output.Bytes())
	if len(responses) != 3 {
		t.Fatalf("response count = %d, want 3; output=%s", len(responses), output.String())
	}

	dryRun := structuredResult[blueprint.UpgradePlan](t, responses[0])
	if !dryRun.DryRun || !dryRun.Success || dryRun.FoundationRoot != "embedded://mss/1.3.3" {
		t.Fatalf("embedded upgrade dry-run = %#v", dryRun)
	}
	if dryRun.ToDistribution.Version != "v1.3.3" || dryRun.ToIdentities.Foundation.Commit != strings.Repeat("a", 40) {
		t.Fatalf("upgrade target is not bound to release build identity: %#v", dryRun.ToIdentities)
	}
	if !containsString(dryRun.PreservedFiles, "internal/modules/custom/owned.go") {
		t.Fatalf("business file missing from preserved set: %q", dryRun.PreservedFiles)
	}

	applied := structuredResult[blueprint.UpgradePlan](t, responses[1])
	if applied.DryRun || !applied.Success {
		t.Fatalf("embedded upgrade apply = %#v", applied)
	}
	data, err := os.ReadFile(businessPath)
	if err != nil || string(data) != businessContent {
		t.Fatalf("business file after apply = %q err=%v", data, err)
	}

	noOp := structuredResult[blueprint.UpgradePlan](t, responses[2])
	if !noOp.DryRun || !noOp.Success {
		t.Fatalf("post-apply no-op plan = %#v", noOp)
	}
	for _, change := range noOp.Changes {
		if change.Action != blueprint.ActionUnchanged {
			t.Fatalf("post-apply change = %#v, want unchanged", change)
		}
	}
	if !containsString(noOp.PreservedFiles, "internal/modules/custom/owned.go") {
		t.Fatalf("post-apply business preservation = %q", noOp.PreservedFiles)
	}
}

func TestEmbeddedUpgradeStillRequiresExplicitConfirmation(t *testing.T) {
	server := &Server{Root: t.TempDir()}
	result, known := server.callBlueprintTool(context.Background(), "mss_apply_foundation_upgrade", map[string]any{})
	if !known || !result.IsError || !strings.Contains(result.Content[0].Text, "confirm=true") {
		t.Fatalf("unconfirmed embedded upgrade = known:%v result:%#v", known, result)
	}
}

func setMCPReleaseBuild(t *testing.T) {
	t.Helper()
	originalVersion, originalCommit, originalTimestamp := buildinfo.Version, buildinfo.Commit, buildinfo.Timestamp
	t.Cleanup(func() {
		buildinfo.Version, buildinfo.Commit, buildinfo.Timestamp = originalVersion, originalCommit, originalTimestamp
	})
	buildinfo.Version = "v1.3.3"
	buildinfo.Commit = strings.Repeat("a", 40)
	buildinfo.Timestamp = "2026-08-25T12:34:56Z"
}

func mcpFrontendRegistry(t *testing.T) string {
	t.Helper()
	integrity := "sha512-" + strings.Repeat("A", 86) + "=="
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(writer, `{"name":"@mss-boot-io/admin-web","version":"1.3.3","dist":{"integrity":%q}}`, integrity)
	}))
	t.Cleanup(server.Close)
	return server.URL
}

func structuredResult[T any](t *testing.T, response rpcResponse) T {
	t.Helper()
	result := objectResult(t, response)
	if result["isError"] == true {
		t.Fatalf("tool call failed: %#v", result)
	}
	data, err := json.Marshal(result["structuredContent"])
	if err != nil {
		t.Fatalf("marshal structuredContent: %v", err)
	}
	var value T
	if err := json.Unmarshal(data, &value); err != nil {
		t.Fatalf("decode structuredContent: %v", err)
	}
	return value
}

func containsString(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}
