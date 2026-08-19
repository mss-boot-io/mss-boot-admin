package app

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mss-boot-io/mss-boot-admin/internal/mss/blueprint"
	"github.com/mss-boot-io/mss-boot-admin/internal/mss/project"
)

func TestUpgradeApplicationDefersRootModuleIdentityToManifest(t *testing.T) {
	context := &project.Context{
		Project: project.ProjectDocument{
			Metadata: project.Metadata{
				Name:        "customer-admin",
				DisplayName: "Customer Administration",
				Repository:  "acme/customer-admin",
			},
			Spec: project.ProjectSpec{
				Backend: project.BackendSpec{
					Module: "github.com/acme/customer-admin/admin",
				},
			},
		},
	}

	application := upgradeApplication(context)
	if application.Name != "customer-admin" ||
		application.DisplayName != "Customer Administration" ||
		application.Repository != "acme/customer-admin" {
		t.Fatalf("upgrade application = %#v", application)
	}
	if application.Module != "" {
		t.Fatalf("upgrade must not use nested backend module as root identity: %q", application.Module)
	}
}

func TestUpgradeAdminCommandDefaultsToPlanAndRequiresApplyConfirmation(t *testing.T) {
	rootOverride := ""
	command := newUpgradeAdminCommand(&rootOverride)
	if flag := command.Flags().Lookup("apply"); flag == nil || flag.DefValue != "false" {
		t.Fatalf("Admin Distribution apply flag = %#v", flag)
	}
	command.SetArgs([]string{"v1.4.0", "--foundation", t.TempDir(), "--apply"})
	command.SilenceUsage = true
	command.SilenceErrors = true
	if err := command.Execute(); err == nil || !strings.Contains(err.Error(), "--yes is required with --apply") {
		t.Fatalf("unconfirmed Admin Distribution apply error = %v", err)
	}

	command = newUpgradeAdminCommand(&rootOverride)
	command.SetArgs([]string{"v1.4.0", "--foundation", t.TempDir(), "--yes"})
	command.SilenceUsage = true
	command.SilenceErrors = true
	if err := command.Execute(); err == nil || !strings.Contains(err.Error(), "--yes is only valid together with --apply") {
		t.Fatalf("plan with stray confirmation error = %v", err)
	}
}

func TestUpgradePlanInputsUseProjectLayoutAndCanonicalCommands(t *testing.T) {
	context := &project.Context{
		Project: project.ProjectDocument{Spec: project.ProjectSpec{
			RepositoryLayout: map[string]string{"specifications": "contracts"},
		}},
		Commands: project.CommandCatalog{Spec: project.CommandCatalogSpec{Commands: map[string]project.Command{
			"backend-test":   {Command: "GOWORK=off go test ./..."},
			"frontend-build": {Command: "corepack pnpm --dir web build"},
			"verify":         {Command: "make verify"},
		}}},
	}
	if got := upgradeModuleSpecificationsPath(context); got != filepath.ToSlash("contracts/modules") {
		t.Fatalf("module specification path = %q", got)
	}
	commands := upgradeValidationCommands(context)
	for _, expected := range []string{"mss doctor --strict --format json", "GOWORK=off go test ./...", "corepack pnpm --dir web build", "make verify"} {
		if !containsAppString(commands, expected) {
			t.Errorf("validation commands %q omit %q", commands, expected)
		}
	}
	paths := upgradePreservedBusinessPaths(context)
	for _, expected := range []string{"contracts/modules", "contracts/features"} {
		if !containsAppString(paths, expected) {
			t.Errorf("preserved business paths %q omit %q", paths, expected)
		}
	}
}

func TestUpgradeApplicationHandlesNilContext(t *testing.T) {
	if application := upgradeApplication(nil); application.Name != "" || application.Module != "" {
		t.Fatalf("nil project context produced %#v", application)
	}
}

func TestWriteUpgradeStatusPreservesFlatJSONAndReportsFourIdentities(t *testing.T) {
	status := blueprint.SnapshotStatus{
		ManifestMetadata: blueprint.ManifestMetadata{
			Project:              "customer-admin",
			Module:               "github.com/acme/customer-admin",
			Repository:           "acme/customer-admin",
			Blueprint:            "management-system",
			BlueprintVersion:     "0.2.1-ci",
			FoundationRepository: "mss-boot-io/mss-boot-admin",
			FoundationCommit:     strings.Repeat("a", 40),
			GeneratorVersion:     "1.1.0",
			GeneratorCommit:      strings.Repeat("b", 40),
		},
		Distribution: project.DistributionSpec{
			Name:     "mss-boot-admin",
			Version:  "v1.3.0",
			Backend:  project.DistributionBackendSpec{Module: "github.com/mss-boot-io/mss-boot-admin/admin", Version: "v1.3.0"},
			Frontend: project.DistributionFrontendSpec{Package: "@mss-boot-io/admin-web", Version: "1.3.0"},
		},
		Identities: blueprint.IdentitySet{
			Foundation: blueprint.FoundationIdentity{Repository: "mss-boot-io/mss-boot-admin", Version: "1.1.0", Commit: strings.Repeat("a", 40)},
			Blueprint:  blueprint.BlueprintIdentity{Name: "management-system", Version: "0.2.1-ci", SHA256: strings.Repeat("c", 64)},
			Generator:  blueprint.GeneratorIdentity{Tool: "mss", Version: "1.1.0", Commit: strings.Repeat("b", 40)},
			Snapshot:   blueprint.DownstreamSnapshotIdentity{Project: "customer-admin", Module: "github.com/acme/customer-admin", Repository: "acme/customer-admin", SHA256: strings.Repeat("d", 64)},
		},
		Records: blueprint.ManifestRecords{
			SnapshotRecordPaths: blueprint.SnapshotRecordPaths{LockPath: ".mss/lock.yaml", ManifestPath: ".mss/blueprint-manifest.json"},
			LockSHA256:          strings.Repeat("e", 64),
		},
	}

	var output bytes.Buffer
	if err := writeUpgradeStatus(&output, status, "json"); err != nil {
		t.Fatalf("writeUpgradeStatus(JSON) error = %v", err)
	}
	var document map[string]any
	if err := json.Unmarshal(output.Bytes(), &document); err != nil {
		t.Fatalf("decode status JSON: %v", err)
	}
	if document["project"] != "customer-admin" || document["blueprintVersion"] != "0.2.1-ci" {
		t.Fatalf("flat compatibility fields = %#v", document)
	}
	if _, ok := document["identities"].(map[string]any); !ok {
		t.Fatalf("identities = %#v", document["identities"])
	}
	if _, ok := document["records"].(map[string]any); !ok {
		t.Fatalf("records = %#v", document["records"])
	}
	if distribution, ok := document["distribution"].(map[string]any); !ok || distribution["version"] != "v1.3.0" {
		t.Fatalf("status Distribution = %#v", document["distribution"])
	}

	output.Reset()
	if err := writeUpgradeStatus(&output, status, "text"); err != nil {
		t.Fatalf("writeUpgradeStatus(text) error = %v", err)
	}
	for _, expected := range []string{
		"admin distribution: mss-boot-admin@v1.3.0",
		"blueprint: management-system@0.2.1-ci sha256 ",
		"foundation: mss-boot-io/mss-boot-admin@1.1.0 commit ",
		"generator: mss@1.1.0 commit ",
		"snapshot: " + strings.Repeat("d", 64),
		"lock: .mss/lock.yaml sha256 ",
	} {
		if !strings.Contains(output.String(), expected) {
			t.Errorf("text status is missing %q:\n%s", expected, output.String())
		}
	}
}

func containsAppString(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}
