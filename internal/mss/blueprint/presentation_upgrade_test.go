package blueprint

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/mss-boot-io/mss-boot-admin/internal/mss/spec"
)

func TestAdminDistributionUpgradeReportsPresentationImpactWithoutWritingDownstream(t *testing.T) {
	oldFoundation := writeThinHostBlueprintFixture(t)
	installPresentationFixture(t, oldFoundation, []presentationFixturePage{
		{sourceName: "role-list.yaml"},
		{sourceName: "user-list.yaml"},
	})
	newFoundation := writeThinHostBlueprintFixture(t)
	installPresentationFixture(t, newFoundation, []presentationFixturePage{
		{sourceName: "department-list.yaml"},
		{
			sourceName: "user-list.yaml",
			mutate: func(data string) string {
				updated := strings.Replace(data, "zh-CN: 用户管理", "zh-CN: 用户目录", 1)
				return strings.Replace(updated, "en-US: Users", "en-US: User directory", 1)
			},
		},
	})
	promoteThinHostDistribution(t, newFoundation, "v1.3.0", "v1.4.0")

	applicationRoot := filepath.Join(t.TempDir(), "presentation-admin")
	application := Application{
		Name:        "presentation-admin",
		DisplayName: "Presentation Administration",
		Module:      "github.com/acme/presentation-admin",
		Repository:  "acme/presentation-admin",
	}
	if _, err := Generate(context.Background(), Options{
		FoundationRoot: oldFoundation,
		Destination:    applicationRoot,
		Application:    application,
		Write:          true,
	}); err != nil {
		t.Fatalf("generate old presentation Thin Host: %v", err)
	}
	writeUpgradeFixtureFile(t, applicationRoot, "runtime.sqlite", "opaque downstream database bytes\n")
	before := snapshotUpgradeFixtureTree(t, applicationRoot)

	plan, err := Upgrade(context.Background(), UpgradeOptions{
		ApplicationRoot:              applicationRoot,
		FoundationRoot:               newFoundation,
		Application:                  application,
		RequestedDistributionVersion: "v1.4.0",
	})
	if err != nil {
		t.Fatalf("plan presentation-aware Admin Distribution upgrade: %v", err)
	}
	if !plan.Success || !plan.DryRun {
		t.Fatalf("presentation-aware plan = %#v", plan)
	}
	impact := plan.PresentationImpact
	if !impact.ComparisonComplete || impact.From.State != presentationSnapshotAvailable || impact.To.State != presentationSnapshotAvailable {
		t.Fatalf("presentation comparison state = %#v", impact)
	}
	if !impact.From.BackendFrontendInventoriesMatch || !impact.To.BackendFrontendInventoriesMatch {
		t.Fatalf("presentation inventory parity = %#v -> %#v", impact.From, impact.To)
	}
	if got, want := presentationPageKeys(impact.From.Pages), []string{"role.list", "user.list"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("old presentation page identities = %v, want %v", got, want)
	}
	if got, want := presentationPageKeys(impact.To.Pages), []string{"department.list", "user.list"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("new presentation page identities = %v, want %v", got, want)
	}
	if impact.From.Pages[1].DefinitionHash == impact.To.Pages[1].DefinitionHash {
		t.Fatalf("changed user.list definition hash was not reported: %#v", impact)
	}
	for _, expected := range []string{"department.list/field/name", "department.list/surface/list"} {
		if !containsUpgradeString(impact.AddedCapabilityIDs, expected) {
			t.Errorf("added presentation capabilities omit %s: %v", expected, impact.AddedCapabilityIDs)
		}
	}
	for _, expected := range []string{"role.list/field/name", "role.list/surface/list"} {
		if !containsUpgradeString(impact.RemovedCapabilityIDs, expected) {
			t.Errorf("removed presentation capabilities omit %s: %v", expected, impact.RemovedCapabilityIDs)
		}
	}
	for _, expected := range []string{"user.list/default/title", "user.list/defaults"} {
		if !containsUpgradeString(impact.ChangedCapabilityIDs, expected) {
			t.Errorf("changed presentation capabilities omit %s: %v", expected, impact.ChangedCapabilityIDs)
		}
	}
	if got, want := impact.PotentiallyStalePageKeys, []string{"department.list", "role.list", "user.list"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("potentially stale page keys = %v, want %v", got, want)
	}
	textPlan := plan.Text()
	for _, expected := range []string{
		"presentation upgrade impact:",
		"snapshots: available -> available (complete comparison: true)",
		"user.list@2/sha256:",
		"potentially stale page keys: department.list, role.list, user.list",
	} {
		if !strings.Contains(textPlan, expected) {
			t.Errorf("text upgrade plan omits %q:\n%s", expected, textPlan)
		}
	}
	planJSON, err := plan.JSON()
	if err != nil {
		t.Fatalf("marshal presentation-aware plan: %v", err)
	}
	var document map[string]json.RawMessage
	if err := json.Unmarshal(planJSON, &document); err != nil {
		t.Fatalf("decode presentation-aware plan: %v", err)
	}
	if len(document["presentationImpact"]) == 0 {
		t.Fatalf("JSON upgrade plan omitted presentationImpact: %s", planJSON)
	}
	after := snapshotUpgradeFixtureTree(t, applicationRoot)
	if !reflect.DeepEqual(after, before) {
		t.Fatalf("read-only presentation upgrade plan changed downstream files:\nbefore=%v\nafter=%v", before, after)
	}

	applied, err := Upgrade(context.Background(), UpgradeOptions{
		ApplicationRoot:              applicationRoot,
		FoundationRoot:               newFoundation,
		Application:                  application,
		RequestedDistributionVersion: "v1.4.0",
		Write:                        true,
	})
	if err != nil {
		t.Fatalf("apply presentation-aware Admin Distribution upgrade: %v", err)
	}
	if !applied.Success || applied.DryRun {
		t.Fatalf("applied presentation-aware plan = %#v", applied)
	}
	presentationData, err := os.ReadFile(filepath.Join(applicationRoot, filepath.FromSlash(presentationSnapshotPath)))
	if err != nil {
		t.Fatalf("read applied presentation snapshot: %v", err)
	}
	appliedPresentation, err := decodePresentationSnapshot(presentationData)
	if err != nil {
		t.Fatalf("decode applied presentation snapshot: %v", err)
	}
	if got, want := presentationPageKeys(presentationUpgradeSnapshotSummary(presentationSnapshotAvailable, appliedPresentation).Pages), presentationPageKeys(impact.To.Pages); !reflect.DeepEqual(got, want) {
		t.Fatalf("applied presentation pages = %v, want %v", got, want)
	}
	upgradedSnapshot, err := ReadSnapshot(applicationRoot, "")
	if err != nil {
		t.Fatalf("read upgraded Thin Host snapshot records: %v", err)
	}
	baseline, exists := upgradedSnapshot.Manifest.Files[presentationSnapshotPath]
	if !exists || baseline.SHA256 != digest(presentationData) || baseline.Size != int64(len(presentationData)) {
		t.Fatalf("presentation manifest baseline = %#v, data digest=%s size=%d", baseline, digest(presentationData), len(presentationData))
	}
	if got := string(upgradedSnapshot.Lock.Spec.Contracts["adminPresentationSnapshot"]); got != "v1alpha1" {
		t.Fatalf("upgraded lock Admin presentation contract = %q", got)
	}
	runtimeData, err := os.ReadFile(filepath.Join(applicationRoot, "runtime.sqlite"))
	if err != nil || string(runtimeData) != "opaque downstream database bytes\n" {
		t.Fatalf("apply accessed or changed opaque downstream database: data=%q error=%v", runtimeData, err)
	}

	repeat, err := Upgrade(context.Background(), UpgradeOptions{
		ApplicationRoot:              applicationRoot,
		FoundationRoot:               newFoundation,
		Application:                  application,
		RequestedDistributionVersion: "v1.4.0",
	})
	if err != nil {
		t.Fatalf("repeat presentation-aware upgrade plan: %v", err)
	}
	if !repeat.Success || !repeat.DryRun || len(repeat.ManagedHostChanges) != 0 ||
		!repeat.PresentationImpact.ComparisonComplete ||
		len(repeat.PresentationImpact.AddedCapabilityIDs) != 0 ||
		len(repeat.PresentationImpact.RemovedCapabilityIDs) != 0 ||
		len(repeat.PresentationImpact.ChangedCapabilityIDs) != 0 ||
		len(repeat.PresentationImpact.PotentiallyStalePageKeys) != 0 {
		t.Fatalf("repeat presentation-aware plan is not idempotent: %#v", repeat)
	}
}

func TestAdminDistributionUpgradeConservativelyReportsUnrecordedPresentationBaseline(t *testing.T) {
	oldFoundation := writeThinHostBlueprintFixture(t)
	newFoundation := writeThinHostBlueprintFixture(t)
	installPresentationFixture(t, newFoundation, []presentationFixturePage{{sourceName: "user-list.yaml"}})
	promoteThinHostDistribution(t, newFoundation, "v1.3.0", "v1.4.0")
	applicationRoot := filepath.Join(t.TempDir(), "legacy-presentation-admin")
	application := Application{
		Name: "legacy-presentation-admin", Module: "github.com/acme/legacy-presentation-admin", Repository: "acme/legacy-presentation-admin",
	}
	if _, err := Generate(context.Background(), Options{
		FoundationRoot: oldFoundation, Destination: applicationRoot, Application: application, Write: true,
	}); err != nil {
		t.Fatalf("generate pre-presentation Thin Host: %v", err)
	}
	before := snapshotUpgradeFixtureTree(t, applicationRoot)
	plan, err := Upgrade(context.Background(), UpgradeOptions{
		ApplicationRoot:              applicationRoot,
		FoundationRoot:               newFoundation,
		Application:                  application,
		RequestedDistributionVersion: "v1.4.0",
	})
	if err != nil {
		t.Fatalf("plan first presentation-aware upgrade: %v", err)
	}
	impact := plan.PresentationImpact
	if impact.ComparisonComplete || impact.From.State != presentationSnapshotUnrecorded || impact.To.State != presentationSnapshotAvailable {
		t.Fatalf("unrecorded presentation comparison = %#v", impact)
	}
	if got, want := impact.PotentiallyStalePageKeys, []string{"user.list"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("conservative stale page keys = %v, want %v", got, want)
	}
	if len(impact.AddedCapabilityIDs) == 0 || len(impact.RemovedCapabilityIDs) != 0 || len(impact.ChangedCapabilityIDs) != 0 {
		t.Fatalf("unrecorded presentation capability transition = %#v", impact)
	}
	if after := snapshotUpgradeFixtureTree(t, applicationRoot); !reflect.DeepEqual(after, before) {
		t.Fatal("first presentation-aware dry run changed the downstream repository")
	}
}

func TestAdminDistributionUpgradeReportsExplicitZeroPresentationStates(t *testing.T) {
	oldFoundation := writeThinHostBlueprintFixture(t)
	newFoundation := writeThinHostBlueprintFixture(t)
	promoteThinHostDistribution(t, newFoundation, "v1.3.0", "v1.4.0")
	applicationRoot := filepath.Join(t.TempDir(), "pre-presentation-admin")
	application := Application{
		Name: "pre-presentation-admin", Module: "github.com/acme/pre-presentation-admin", Repository: "acme/pre-presentation-admin",
	}
	if _, err := Generate(context.Background(), Options{
		FoundationRoot: oldFoundation, Destination: applicationRoot, Application: application, Write: true,
	}); err != nil {
		t.Fatalf("generate pre-presentation Thin Host: %v", err)
	}
	before := snapshotUpgradeFixtureTree(t, applicationRoot)
	plan, err := Upgrade(context.Background(), UpgradeOptions{
		ApplicationRoot:              applicationRoot,
		FoundationRoot:               newFoundation,
		Application:                  application,
		RequestedDistributionVersion: "v1.4.0",
	})
	if err != nil {
		t.Fatalf("plan pre-presentation upgrade: %v", err)
	}
	impact := plan.PresentationImpact
	if impact.ComparisonComplete || impact.From.State != presentationSnapshotUnrecorded || impact.To.State != presentationSnapshotUnavailable {
		t.Fatalf("zero presentation comparison = %#v", impact)
	}
	if impact.From.BackendFrontendInventoriesMatch || impact.To.BackendFrontendInventoriesMatch ||
		len(impact.From.Pages) != 0 || len(impact.To.Pages) != 0 ||
		len(impact.AddedCapabilityIDs) != 0 || len(impact.RemovedCapabilityIDs) != 0 ||
		len(impact.ChangedCapabilityIDs) != 0 || len(impact.PotentiallyStalePageKeys) != 0 {
		t.Fatalf("zero presentation impact contains invented facts: %#v", impact)
	}
	if got := plan.Text(); !strings.Contains(got, "snapshots: unrecorded -> unavailable (complete comparison: false)") ||
		!strings.Contains(got, "pages: none -> none") ||
		!strings.Contains(got, "potentially stale page keys: none") {
		t.Fatalf("zero presentation text is ambiguous:\n%s", got)
	}
	data, err := plan.JSON()
	if err != nil {
		t.Fatalf("marshal pre-presentation upgrade plan: %v", err)
	}
	for _, expected := range []string{
		`"state": "unrecorded"`,
		`"state": "unavailable"`,
		`"pages": []`,
		`"addedCapabilityIds": []`,
		`"potentiallyStalePageKeys": []`,
	} {
		if !bytes.Contains(data, []byte(expected)) {
			t.Errorf("zero presentation JSON omits %s:\n%s", expected, data)
		}
	}
	if after := snapshotUpgradeFixtureTree(t, applicationRoot); !reflect.DeepEqual(after, before) {
		t.Fatal("pre-presentation dry run changed the downstream repository")
	}
}

type presentationFixturePage struct {
	sourceName string
	mutate     func(string) string
}

func installPresentationFixture(t *testing.T, foundationRoot string, fixtures []presentationFixturePage) {
	t.Helper()
	_, testFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve presentation upgrade test source path")
	}
	repositoryRoot := filepath.Clean(filepath.Join(filepath.Dir(testFile), "..", "..", ".."))
	type pageIdentity struct {
		pageKey    string
		version    string
		sourcePath string
		hash       string
	}
	identities := make([]pageIdentity, 0, len(fixtures))
	for _, fixture := range fixtures {
		sourcePath := filepath.ToSlash(filepath.Join(".mss", "core-pages", fixture.sourceName))
		data, err := os.ReadFile(filepath.Join(repositoryRoot, filepath.FromSlash(sourcePath)))
		if err != nil {
			t.Fatalf("read repository presentation fixture %s: %v", sourcePath, err)
		}
		if fixture.mutate != nil {
			data = []byte(fixture.mutate(string(data)))
		}
		document, err := spec.ParseCorePagePresentation(data, sourcePath)
		if err != nil {
			t.Fatalf("parse presentation fixture %s: %v", sourcePath, err)
		}
		manifest, err := document.NormalizePresentation()
		if err != nil {
			t.Fatalf("normalize presentation fixture %s: %v", sourcePath, err)
		}
		writeFixtureFile(t, foundationRoot, sourcePath, string(data))
		identities = append(identities, pageIdentity{
			pageKey: manifest.PageKey, version: manifest.DefinitionVersion, sourcePath: sourcePath, hash: manifest.DefinitionHash,
		})
	}
	sort.SliceStable(identities, func(i, j int) bool { return identities[i].pageKey < identities[j].pageKey })
	var inventory strings.Builder
	inventory.WriteString("apiVersion: mss.io/v1alpha1\nkind: AdminPresentationPageInventory\nspec:\n  pages:\n")
	for _, identity := range identities {
		fmt.Fprintf(
			&inventory,
			"    - disposition: included\n      pageKey: %s\n      sourcePath: %s\n      definitionIdentity:\n        state: matching\n        backendHash: %s\n        frontendHash: %s\n",
			identity.pageKey,
			identity.sourcePath,
			identity.hash,
			identity.hash,
		)
	}
	writeFixtureFile(t, foundationRoot, presentationPageInventoryPath, inventory.String())
	var frontend strings.Builder
	frontend.WriteString("// Code generated by presentation upgrade test. DO NOT EDIT.\n\nexport const corePresentationInventory = [\n")
	for _, identity := range identities {
		fmt.Fprintf(&frontend, "  %q,\n", identity.pageKey)
	}
	frontend.WriteString("] as const;\n\nexport const corePresentationRegistry = {\n")
	for _, identity := range identities {
		fmt.Fprintf(
			&frontend,
			"  %q: {\n    definitionHash: %q,\n    definition: {\n      \"pageKey\": %q,\n      \"definitionVersion\": %q,\n      \"definitionHash\": %q,\n    } as const,\n  },\n",
			identity.pageKey,
			identity.hash,
			identity.pageKey,
			identity.version,
			identity.hash,
		)
	}
	frontend.WriteString("} as const;\n")
	writeFixtureFile(t, foundationRoot, presentationFrontendRegistryPath, frontend.String())
	runGit(t, foundationRoot, "add", "-A")
	runGit(t, foundationRoot, "commit", "-m", "test: add Admin presentation upgrade source")
}

type upgradeFixtureFingerprint struct {
	Mode   fs.FileMode
	SHA256 string
	Size   int64
}

func snapshotUpgradeFixtureTree(t *testing.T, root string) map[string]upgradeFixtureFingerprint {
	t.Helper()
	result := make(map[string]upgradeFixtureFingerprint)
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("fixture contains non-regular file %s", path)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		result[filepath.ToSlash(relative)] = upgradeFixtureFingerprint{Mode: info.Mode().Perm(), SHA256: digest(data), Size: int64(len(data))}
		return nil
	})
	if err != nil {
		t.Fatalf("snapshot downstream fixture tree: %v", err)
	}
	return result
}

func presentationPageKeys(pages []PresentationPageIdentity) []string {
	result := make([]string, 0, len(pages))
	for _, page := range pages {
		result = append(result, page.PageKey)
	}
	return result
}

func TestPresentationSnapshotJSONRejectsUnknownFieldsAndDuplicateCapabilityIDs(t *testing.T) {
	data := []byte(`{
  "apiVersion": "mss.io/v1alpha1",
  "kind": "AdminPresentationSnapshot",
  "backendInventorySha256": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
  "frontendInventorySha256": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
  "backendFrontendInventoriesMatch": true,
  "pages": [{
    "pageKey": "user.list",
    "definitionVersion": "2",
    "definitionHash": "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
    "capabilities": [
      {"id": "user.list/default/title", "sha256": "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"},
      {"id": "user.list/default/title", "sha256": "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"}
    ]
  }]
}`)
	if _, err := decodePresentationSnapshot(data); err == nil || !strings.Contains(err.Error(), "invalid or unsorted capability") {
		t.Fatalf("duplicate capability identities error = %v", err)
	}
	unknown := bytes.Replace(data, []byte(`"kind": "AdminPresentationSnapshot"`), []byte(`"kind": "AdminPresentationSnapshot", "database": "forbidden"`), 1)
	if _, err := decodePresentationSnapshot(unknown); err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("unknown presentation snapshot field error = %v", err)
	}
}

func TestPresentationSourceSnapshotComparesPackagedFrontendIdentity(t *testing.T) {
	foundationRoot := writeThinHostBlueprintFixture(t)
	installPresentationFixture(t, foundationRoot, []presentationFixturePage{{sourceName: "user-list.yaml"}})
	snapshot, err := loadPresentationSourceSnapshot(func(path string) ([]byte, bool, error) {
		data, readErr := os.ReadFile(filepath.Join(foundationRoot, filepath.FromSlash(path)))
		if os.IsNotExist(readErr) {
			return nil, false, nil
		}
		if readErr != nil {
			return nil, false, readErr
		}
		if path == presentationFrontendRegistryPath {
			data = bytes.Replace(data, []byte(`"definitionVersion": "2"`), []byte(`"definitionVersion": "999"`), 1)
		}
		return data, true, nil
	})
	if err != nil {
		t.Fatalf("load source snapshot with packaged frontend version drift: %v", err)
	}
	if snapshot.BackendFrontendInventoriesMatch || snapshot.BackendInventorySHA256 == snapshot.FrontendInventorySHA256 {
		t.Fatalf("packaged frontend version drift was trusted as matching: %#v", snapshot)
	}
}

func TestAdminDistributionUpgradeFailsClosedOnPackagedFrontendIdentityDrift(t *testing.T) {
	oldFoundation := writeThinHostBlueprintFixture(t)
	newFoundation := writeThinHostBlueprintFixture(t)
	installPresentationFixture(t, newFoundation, []presentationFixturePage{{sourceName: "user-list.yaml"}})
	frontendPath := filepath.Join(newFoundation, filepath.FromSlash(presentationFrontendRegistryPath))
	frontendData, err := os.ReadFile(frontendPath)
	if err != nil {
		t.Fatalf("read packaged frontend fixture: %v", err)
	}
	frontendData = bytes.Replace(frontendData, []byte(`"definitionVersion": "2"`), []byte(`"definitionVersion": "999"`), 1)
	writeFixtureFile(t, newFoundation, presentationFrontendRegistryPath, string(frontendData))
	runGit(t, newFoundation, "add", presentationFrontendRegistryPath)
	runGit(t, newFoundation, "commit", "-m", "test: drift packaged frontend presentation identity")
	promoteThinHostDistribution(t, newFoundation, "v1.3.0", "v1.4.0")

	applicationRoot := filepath.Join(t.TempDir(), "frontend-drift-admin")
	application := Application{
		Name: "frontend-drift-admin", Module: "github.com/acme/frontend-drift-admin", Repository: "acme/frontend-drift-admin",
	}
	if _, err := Generate(context.Background(), Options{
		FoundationRoot: oldFoundation, Destination: applicationRoot, Application: application, Write: true,
	}); err != nil {
		t.Fatalf("generate pre-presentation Thin Host: %v", err)
	}
	before := snapshotUpgradeFixtureTree(t, applicationRoot)
	plan, err := Upgrade(context.Background(), UpgradeOptions{
		ApplicationRoot:              applicationRoot,
		FoundationRoot:               newFoundation,
		Application:                  application,
		RequestedDistributionVersion: "v1.4.0",
	})
	if err != nil {
		t.Fatalf("plan drifted packaged frontend upgrade: %v", err)
	}
	if plan.Success || plan.PresentationImpact.To.State != presentationSnapshotAvailable || plan.PresentationImpact.To.BackendFrontendInventoriesMatch {
		t.Fatalf("packaged frontend identity drift did not fail the plan closed: %#v", plan)
	}
	if after := snapshotUpgradeFixtureTree(t, applicationRoot); !reflect.DeepEqual(after, before) {
		t.Fatal("failed-closed packaged frontend plan changed downstream files")
	}
}
