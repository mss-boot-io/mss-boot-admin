package blueprint

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAdminDistributionUpgradePlansReadOnlyThenAppliesAndPreservesUnknownBusinessFiles(t *testing.T) {
	oldFoundation := writeThinHostBlueprintFixture(t)
	newFoundation := writeThinHostBlueprintFixture(t)
	promoteThinHostDistribution(t, newFoundation, "v1.3.0", "v1.4.0")
	applicationRoot := filepath.Join(t.TempDir(), "orders-admin")
	application := Application{
		Name:        "orders-admin",
		DisplayName: "Orders Administration",
		Module:      "github.com/acme/orders-admin",
		Repository:  "acme/orders-admin",
	}
	if _, err := Generate(context.Background(), Options{
		FoundationRoot: oldFoundation,
		Destination:    applicationRoot,
		Application:    application,
		Write:          true,
	}); err != nil {
		t.Fatalf("generate old Thin Host: %v", err)
	}
	writeUpgradeFixtureFile(t, applicationRoot, ".mss/modules/supplier.yaml", "apiVersion: mss.io/v1alpha1\nkind: AdminModule\n")
	writeUpgradeFixtureFile(t, applicationRoot, "internal/modules/supplier/custom.go", "package supplier\n\nconst Custom = true\n")
	goModPath := filepath.Join(applicationRoot, "go.mod")
	manifestPath := filepath.Join(applicationRoot, ".mss", "blueprint-manifest.json")
	goModBefore, err := os.ReadFile(goModPath)
	if err != nil {
		t.Fatalf("read old go.mod: %v", err)
	}
	manifestBefore, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("read old manifest: %v", err)
	}

	plan, err := Upgrade(context.Background(), UpgradeOptions{
		ApplicationRoot:              applicationRoot,
		FoundationRoot:               newFoundation,
		Application:                  application,
		RequestedDistributionVersion: "v1.4.0",
		PreservedBusinessPaths:       []string{"internal/modules", ".mss/modules"},
	})
	if err != nil {
		t.Fatalf("plan Admin Distribution upgrade: %v", err)
	}
	if !plan.Success || !plan.DryRun {
		t.Fatalf("Admin Distribution plan = %#v", plan)
	}
	if plan.FromDistribution.Version != "v1.3.0" || plan.ToDistribution.Version != "v1.4.0" {
		t.Fatalf("Distribution transition = %#v -> %#v", plan.FromDistribution, plan.ToDistribution)
	}
	if !plan.GoAdminModule.Changed || plan.GoAdminModule.FromVersion != "v1.3.0" || plan.GoAdminModule.ToVersion != "v1.4.0" {
		t.Fatalf("Go Admin transition = %#v", plan.GoAdminModule)
	}
	if !plan.AdminWebPackage.Changed || plan.AdminWebPackage.FromVersion != "1.3.0" || plan.AdminWebPackage.ToVersion != "1.4.0" {
		t.Fatalf("Admin Web transition = %#v", plan.AdminWebPackage)
	}
	if len(plan.ManagedHostChanges) == 0 || len(plan.Conflicts) != 0 ||
		!containsUpgradeString(plan.ModulesToRegenerate, "supplier") ||
		!containsUpgradeString(plan.PreservedFiles, "internal/modules/supplier/custom.go") ||
		len(plan.ValidationCommands) == 0 {
		t.Fatalf("Admin Distribution structured impact = %#v", plan)
	}
	if !strings.Contains(plan.Text(), "admin distribution: mss-boot-admin@v1.3.0 -> mss-boot-admin@v1.4.0") ||
		!strings.Contains(plan.Text(), "modules to regenerate: supplier") ||
		!strings.Contains(plan.Text(), "validation commands:") {
		t.Fatalf("Admin Distribution text plan omitted required fields:\n%s", plan.Text())
	}
	planJSON, err := plan.JSON()
	if err != nil {
		t.Fatalf("marshal Admin Distribution plan: %v", err)
	}
	var planDocument map[string]any
	if err := json.Unmarshal(planJSON, &planDocument); err != nil {
		t.Fatalf("decode Admin Distribution plan: %v", err)
	}
	for _, field := range []string{
		"fromDistribution",
		"toDistribution",
		"goAdminModule",
		"adminWebPackage",
		"managedHostChanges",
		"conflicts",
		"preservedFiles",
		"modulesToRegenerate",
		"validationCommands",
	} {
		if _, ok := planDocument[field]; !ok {
			t.Errorf("Admin Distribution plan JSON omitted %s: %s", field, planJSON)
		}
	}
	goModAfterPlan, _ := os.ReadFile(goModPath)
	manifestAfterPlan, _ := os.ReadFile(manifestPath)
	if !bytes.Equal(goModBefore, goModAfterPlan) || !bytes.Equal(manifestBefore, manifestAfterPlan) {
		t.Fatal("read-only Admin Distribution plan changed downstream files")
	}

	applied, err := Upgrade(context.Background(), UpgradeOptions{
		ApplicationRoot:              applicationRoot,
		FoundationRoot:               newFoundation,
		Application:                  application,
		RequestedDistributionVersion: "v1.4.0",
		PreservedBusinessPaths:       []string{"internal/modules", ".mss/modules"},
		Write:                        true,
	})
	if err != nil {
		t.Fatalf("apply Admin Distribution upgrade: %v", err)
	}
	if applied.DryRun || !applied.Success {
		t.Fatalf("applied Admin Distribution plan = %#v", applied)
	}
	assertContains(t, goModPath, "github.com/mss-boot-io/mss-boot-admin/admin v1.4.0")
	assertContains(t, filepath.Join(applicationRoot, "internal", "modules", "supplier", "custom.go"), "const Custom = true")
	snapshot, err := ReadSnapshot(applicationRoot, "")
	if err != nil {
		t.Fatalf("read upgraded Admin Distribution snapshot: %v", err)
	}
	if snapshot.Manifest.Distribution.Version != "v1.4.0" || snapshot.Lock.Spec.Distribution.Version != "v1.4.0" {
		t.Fatalf("upgraded snapshot Distribution = %#v / %#v", snapshot.Manifest.Distribution, snapshot.Lock.Spec.Distribution)
	}
	repeat, err := Upgrade(context.Background(), UpgradeOptions{
		ApplicationRoot:              applicationRoot,
		FoundationRoot:               newFoundation,
		Application:                  application,
		RequestedDistributionVersion: "v1.4.0",
		PreservedBusinessPaths:       []string{"internal/modules", ".mss/modules"},
	})
	if err != nil || !repeat.Success || len(repeat.ManagedHostChanges) != 0 || len(repeat.ModulesToRegenerate) != 0 {
		t.Fatalf("repeat Admin Distribution plan=%#v error=%v", repeat, err)
	}
}

func TestAdminDistributionUpgradeRejectsRequestedVersionThatDoesNotMatchTarget(t *testing.T) {
	oldFoundation := writeThinHostBlueprintFixture(t)
	newFoundation := writeThinHostBlueprintFixture(t)
	promoteThinHostDistribution(t, newFoundation, "v1.3.0", "v1.4.0")
	applicationRoot := filepath.Join(t.TempDir(), "mismatch-admin")
	application := Application{Name: "mismatch-admin", Module: "github.com/acme/mismatch-admin", Repository: "acme/mismatch-admin"}
	if _, err := Generate(context.Background(), Options{FoundationRoot: oldFoundation, Destination: applicationRoot, Application: application, Write: true}); err != nil {
		t.Fatalf("generate old Thin Host: %v", err)
	}
	manifestPath := filepath.Join(applicationRoot, ".mss", "blueprint-manifest.json")
	before, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("read old manifest: %v", err)
	}
	_, err = Upgrade(context.Background(), UpgradeOptions{
		ApplicationRoot:              applicationRoot,
		FoundationRoot:               newFoundation,
		Application:                  application,
		RequestedDistributionVersion: "v1.5.0",
		Write:                        true,
	})
	if err == nil || !strings.Contains(err.Error(), "does not match target Foundation blueprint Distribution v1.4.0") {
		t.Fatalf("mismatched Admin Distribution error = %v", err)
	}
	after, _ := os.ReadFile(manifestPath)
	if !bytes.Equal(before, after) {
		t.Fatal("version-mismatched Admin Distribution upgrade changed the baseline")
	}
}

func TestAdminDistributionUpgradeConflictsFailClosedAndKeepUnknownBusinessFiles(t *testing.T) {
	oldFoundation := writeThinHostBlueprintFixture(t)
	newFoundation := writeThinHostBlueprintFixture(t)
	promoteThinHostDistribution(t, newFoundation, "v1.3.0", "v1.4.0")
	applicationRoot := filepath.Join(t.TempDir(), "conflict-thin-admin")
	application := Application{Name: "conflict-thin-admin", Module: "github.com/acme/conflict-thin-admin", Repository: "acme/conflict-thin-admin"}
	if _, err := Generate(context.Background(), Options{FoundationRoot: oldFoundation, Destination: applicationRoot, Application: application, Write: true}); err != nil {
		t.Fatalf("generate old Thin Host: %v", err)
	}
	goModPath := filepath.Join(applicationRoot, "go.mod")
	goMod, err := os.ReadFile(goModPath)
	if err != nil {
		t.Fatalf("read old go.mod: %v", err)
	}
	goMod = append(goMod, []byte("\n// downstream dependency customization\n")...)
	if err := os.WriteFile(goModPath, goMod, 0o644); err != nil {
		t.Fatalf("customize go.mod: %v", err)
	}
	writeUpgradeFixtureFile(t, applicationRoot, "internal/modules/order/custom.go", "package order\n\nconst Keep = true\n")
	manifestPath := filepath.Join(applicationRoot, ".mss", "blueprint-manifest.json")
	manifestBefore, _ := os.ReadFile(manifestPath)

	plan, err := Upgrade(context.Background(), UpgradeOptions{
		ApplicationRoot:              applicationRoot,
		FoundationRoot:               newFoundation,
		Application:                  application,
		RequestedDistributionVersion: "v1.4.0",
		PreservedBusinessPaths:       []string{"internal/modules"},
		Write:                        true,
	})
	if err == nil || plan.Success || len(plan.Conflicts) == 0 {
		t.Fatalf("conflicting Admin Distribution apply plan=%#v error=%v", plan, err)
	}
	assertUpgradeAction(t, plan, "go.mod", ActionConflict)
	assertContains(t, goModPath, "downstream dependency customization")
	assertContains(t, filepath.Join(applicationRoot, "internal", "modules", "order", "custom.go"), "const Keep = true")
	if !containsUpgradeString(plan.PreservedFiles, "internal/modules/order/custom.go") {
		t.Fatalf("conflict plan omitted preserved business file: %#v", plan.PreservedFiles)
	}
	manifestAfter, _ := os.ReadFile(manifestPath)
	if !bytes.Equal(manifestBefore, manifestAfter) {
		t.Fatal("conflicting Admin Distribution upgrade updated the snapshot baseline")
	}
}

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
	if _, err := os.Stat(filepath.Join(applicationRoot, "web", "antd-v6", "public", "fixture.bin")); !os.IsNotExist(err) {
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
	binaryPath := filepath.Join(applicationRoot, "web", "antd-v6", "public", "fixture.bin")
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
	if err := os.Remove(filepath.Join(root, "web", "antd-v6", "public", "fixture.bin")); err != nil {
		t.Fatalf("remove obsolete fixture: %v", err)
	}
	runGit(t, root, "add", "-A")
	runGit(t, root, "commit", "-m", "foundation v2")
}

func promoteThinHostDistribution(t *testing.T, root, fromVersion, toVersion string) {
	t.Helper()
	for _, relative := range []string{
		".mss/blueprints/management-system.yaml",
		".mss/release-policy.yaml",
	} {
		path := filepath.Join(root, filepath.FromSlash(relative))
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", relative, err)
		}
		data = []byte(strings.ReplaceAll(string(data), fromVersion, toVersion))
		data = []byte(strings.ReplaceAll(
			string(data),
			strings.TrimPrefix(fromVersion, "v"),
			strings.TrimPrefix(toVersion, "v"),
		))
		if relative == ".mss/blueprints/management-system.yaml" {
			data = []byte(strings.Replace(string(data), "version: 0.4.0", "version: 0.5.0", 1))
		}
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatalf("write %s: %v", relative, err)
		}
	}
	runGit(t, root, "add", "-A")
	runGit(t, root, "commit", "-m", "upgrade Admin Distribution to "+toVersion)
}

func writeUpgradeFixtureFile(t *testing.T, root, relative, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create %s parent: %v", relative, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", relative, err)
	}
}

func containsUpgradeString(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
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
