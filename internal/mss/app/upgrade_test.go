package app

import (
	"bytes"
	"encoding/json"
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

	output.Reset()
	if err := writeUpgradeStatus(&output, status, "text"); err != nil {
		t.Fatalf("writeUpgradeStatus(text) error = %v", err)
	}
	for _, expected := range []string{
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
