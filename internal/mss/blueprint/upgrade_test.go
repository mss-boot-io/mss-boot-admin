package blueprint

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestUpgradeAppliesFoundationChangesAndPreservesCustomization(t *testing.T) {
	oldFoundation := writeBlueprintFixture(t)
	newFoundation := writeBlueprintFixture(t)
	prepareNewFoundation(t, newFoundation)
	applicationRoot := filepath.Join(t.TempDir(), "customer-admin")
	application := Application{
		Name:        "customer-admin",
		DisplayName: "Customer Administration",
		Module:      "github.com/acme/customer-admin",
		Repository:  "acme/customer-admin",
	}
	if _, err := Generate(context.Background(), Options{
		FoundationRoot: oldFoundation,
		Destination:    applicationRoot,
		Application:    application,
		Write:          true,
	}); err != nil {
		t.Fatalf("generate old application: %v", err)
	}

	customAgents := []byte("# Customer-specific Agent rules\n")
	if err := os.WriteFile(filepath.Join(applicationRoot, "AGENTS.md"), customAgents, 0o644); err != nil {
		t.Fatalf("customize AGENTS.md: %v", err)
	}
	if err := os.WriteFile(filepath.Join(applicationRoot, "business-only.txt"), []byte("preserve me\n"), 0o644); err != nil {
		t.Fatalf("write downstream-only file: %v", err)
	}

	plan, err := Upgrade(context.Background(), UpgradeOptions{
		ApplicationRoot: applicationRoot,
		FoundationRoot:  newFoundation,
		Application:     application,
	})
	if err != nil {
		t.Fatalf("plan foundation upgrade: %v", err)
	}
	if !plan.Success || !plan.DryRun {
		t.Fatalf("unexpected upgrade plan: %#v", plan)
	}
	assertUpgradeAction(t, plan, "AGENTS.md", ActionPreserve)
	assertUpgradeAction(t, plan, "admin/main.go", ActionUpdate)
	assertUpgradeAction(t, plan, "NEW.md", ActionCreate)
	assertUpgradeAction(t, plan, "web/antd-v6/public/fixture.bin", ActionDelete)

	applied, err := Upgrade(context.Background(), UpgradeOptions{
		ApplicationRoot: applicationRoot,
		FoundationRoot:  newFoundation,
		Application:     application,
		Write:           true,
	})
	if err != nil {
		t.Fatalf("apply foundation upgrade: %v", err)
	}
	if applied.DryRun || !applied.Success {
		t.Fatalf("unexpected applied plan: %#v", applied)
	}
	agents, err := os.ReadFile(filepath.Join(applicationRoot, "AGENTS.md"))
	if err != nil {
		t.Fatalf("read customized AGENTS.md: %v", err)
	}
	if string(agents) != string(customAgents) {
		t.Fatalf("customized AGENTS.md was overwritten: %q", agents)
	}
	assertContains(t, filepath.Join(applicationRoot, "admin/main.go"), "FoundationRevision = \"v2\"")
	assertContains(t, filepath.Join(applicationRoot, "NEW.md"), "new foundation capability")
	if _, err := os.Stat(filepath.Join(applicationRoot, "web", "antd", "public", "fixture.bin")); !os.IsNotExist(err) {
		t.Fatalf("obsolete unmodified file was not removed: %v", err)
	}
	assertContains(t, filepath.Join(applicationRoot, "business-only.txt"), "preserve me")

	manifest, err := ReadManifest(applicationRoot, "")
	if err != nil {
		t.Fatalf("read upgraded manifest: %v", err)
	}
	if manifest.Metadata.BlueprintVersion != "0.2.0" || manifest.Metadata.FoundationCommit != applied.ToFoundationCommit {
		t.Fatalf("manifest baseline was not updated: %#v", manifest.Metadata)
	}

	repeat, err := Upgrade(context.Background(), UpgradeOptions{
		ApplicationRoot: applicationRoot,
		FoundationRoot:  newFoundation,
		Application:     application,
	})
	if err != nil {
		t.Fatalf("repeat upgrade plan: %v", err)
	}
	if !repeat.Success {
		t.Fatalf("repeat upgrade is not conflict-free: %#v", repeat)
	}
	for _, change := range repeat.Changes {
		if change.Path == "AGENTS.md" {
			if change.Action != ActionPreserve {
				t.Fatalf("customized AGENTS.md should remain preserved, got %#v", change)
			}
			continue
		}
		if change.Action != ActionUnchanged {
			t.Fatalf("repeat upgrade contains unexpected action: %#v", change)
		}
	}
}

func TestUpgradeDetectsConcurrentFoundationAndDownstreamChanges(t *testing.T) {
	oldFoundation := writeBlueprintFixture(t)
	newFoundation := writeBlueprintFixture(t)
	prepareNewFoundation(t, newFoundation)
	applicationRoot := filepath.Join(t.TempDir(), "conflict-admin")
	application := Application{
		Name:        "conflict-admin",
		DisplayName: "Conflict Administration",
		Module:      "github.com/acme/conflict-admin",
		Repository:  "acme/conflict-admin",
	}
	if _, err := Generate(context.Background(), Options{
		FoundationRoot: oldFoundation,
		Destination:    applicationRoot,
		Application:    application,
		Write:          true,
	}); err != nil {
		t.Fatalf("generate old application: %v", err)
	}
	if err := os.WriteFile(filepath.Join(applicationRoot, "admin/main.go"), []byte("package downstream\n"), 0o644); err != nil {
		t.Fatalf("customize main.go: %v", err)
	}

	plan, err := Upgrade(context.Background(), UpgradeOptions{
		ApplicationRoot: applicationRoot,
		FoundationRoot:  newFoundation,
		Application:     application,
		Write:           true,
	})
	if err == nil {
		t.Fatal("expected conflicting upgrade to fail")
	}
	if plan.Success {
		t.Fatalf("conflicting upgrade reported success: %#v", plan)
	}
	assertUpgradeAction(t, plan, "admin/main.go", ActionConflict)
	assertContains(t, filepath.Join(applicationRoot, "admin/main.go"), "package downstream")
}

func TestUpgradeDetectsCustomizedFileRemovedByFoundation(t *testing.T) {
	oldFoundation := writeBlueprintFixture(t)
	newFoundation := writeBlueprintFixture(t)
	prepareNewFoundation(t, newFoundation)
	applicationRoot := filepath.Join(t.TempDir(), "removed-admin")
	application := Application{
		Name:        "removed-admin",
		DisplayName: "Removed Administration",
		Module:      "github.com/acme/removed-admin",
		Repository:  "acme/removed-admin",
	}
	if _, err := Generate(context.Background(), Options{
		FoundationRoot: oldFoundation,
		Destination:    applicationRoot,
		Application:    application,
		Write:          true,
	}); err != nil {
		t.Fatalf("generate old application: %v", err)
	}
	binaryPath := filepath.Join(applicationRoot, "web", "antd", "public", "fixture.bin")
	if err := os.WriteFile(binaryPath, []byte{9, 9, 9}, 0o644); err != nil {
		t.Fatalf("customize removed binary: %v", err)
	}
	plan, err := Upgrade(context.Background(), UpgradeOptions{
		ApplicationRoot: applicationRoot,
		FoundationRoot:  newFoundation,
		Application:     application,
	})
	if err != nil {
		t.Fatalf("dry-run conflict plan should not return an execution error: %v", err)
	}
	if plan.Success {
		t.Fatal("customized removed file did not cause a conflict")
	}
	assertUpgradeAction(t, plan, "web/antd-v6/public/fixture.bin", ActionConflict)
}

func TestUpgradeAcceptsLegacyManifestOnlyAsMigrationInput(t *testing.T) {
	oldFoundation := writeBlueprintFixture(t)
	newFoundation := writeBlueprintFixture(t)
	prepareNewFoundation(t, newFoundation)
	applicationRoot := filepath.Join(t.TempDir(), "legacy-admin")
	application := Application{
		Name:        "legacy-admin",
		DisplayName: "Legacy Administration",
		Module:      "github.com/acme/legacy-admin",
		Repository:  "acme/legacy-admin",
	}
	if _, err := Generate(context.Background(), Options{
		FoundationRoot: oldFoundation,
		Destination:    applicationRoot,
		Application:    application,
		Write:          true,
	}); err != nil {
		t.Fatalf("generate old application: %v", err)
	}
	manifest, err := ReadManifest(applicationRoot, "")
	if err != nil {
		t.Fatalf("read current manifest: %v", err)
	}
	manifest.APIVersion = legacyAPIVersion
	manifest.Identities = IdentitySet{}
	manifest.Records = ManifestRecords{}
	writeManifestAtPath(t, applicationRoot, ".mss/blueprint-manifest.json", manifest)
	if _, err := ReadManifest(applicationRoot, ""); err == nil {
		t.Fatal("legacy manifest was accepted by the current status reader")
	}
	if _, err := Upgrade(context.Background(), UpgradeOptions{
		ApplicationRoot: applicationRoot,
		FoundationRoot:  newFoundation,
		Application:     application,
		Write:           true,
	}); err != nil {
		t.Fatalf("upgrade legacy manifest input: %v", err)
	}
	snapshot, err := ReadSnapshot(applicationRoot, "")
	if err != nil {
		t.Fatalf("read migrated current snapshot: %v", err)
	}
	if snapshot.Manifest.APIVersion != snapshotAPIVersion {
		t.Fatalf("migrated manifest apiVersion = %q, want %q", snapshot.Manifest.APIVersion, snapshotAPIVersion)
	}
}

func prepareNewFoundation(t *testing.T, root string) {
	t.Helper()
	blueprintPath := filepath.Join(root, ".mss", "blueprints", "management-system.yaml")
	blueprintData, err := os.ReadFile(blueprintPath)
	if err != nil {
		t.Fatalf("read new blueprint: %v", err)
	}
	blueprintData = []byte(stringsReplaceAll(string(blueprintData), "version: 0.1.0", "version: 0.2.0"))
	if err := os.WriteFile(blueprintPath, blueprintData, 0o644); err != nil {
		t.Fatalf("write new blueprint: %v", err)
	}
	mainPath := filepath.Join(root, "admin/main.go")
	mainData, err := os.ReadFile(mainPath)
	if err != nil {
		t.Fatalf("read new main.go: %v", err)
	}
	mainData = append(mainData, []byte("\nconst FoundationRevision = \"v2\"\n")...)
	if err := os.WriteFile(mainPath, mainData, 0o644); err != nil {
		t.Fatalf("write new main.go: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "NEW.md"), []byte("new foundation capability\n"), 0o644); err != nil {
		t.Fatalf("write new file: %v", err)
	}
	if err := os.Remove(filepath.Join(root, "web", "antd", "public", "fixture.bin")); err != nil {
		t.Fatalf("remove obsolete fixture: %v", err)
	}
	runGit(t, root, "add", "-A")
	runGit(t, root, "commit", "-m", "foundation v2")
}

func assertUpgradeAction(t *testing.T, plan UpgradePlan, path string, action Action) {
	t.Helper()
	for _, change := range plan.Changes {
		if change.Path == path {
			if change.Action != action {
				t.Fatalf("upgrade action for %s = %s, want %s; detail=%s", path, change.Action, action, change.Detail)
			}
			return
		}
	}
	t.Fatalf("upgrade plan does not contain %s", path)
}

func stringsReplaceAll(value, old, replacement string) string {
	for {
		index := stringsIndex(value, old)
		if index < 0 {
			return value
		}
		value = value[:index] + replacement + value[index+len(old):]
	}
}

func stringsIndex(value, target string) int {
	for index := 0; index+len(target) <= len(value); index++ {
		if value[index:index+len(target)] == target {
			return index
		}
	}
	return -1
}
