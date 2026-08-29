package core

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/mss-boot-io/mss-boot-admin/admin/presentation"
)

func TestDefinitionsAreValidHashedAndDefensivelyCopied(t *testing.T) {
	definitions := Definitions()
	if len(definitions) != 1 {
		t.Fatalf("Definitions() count = %d, want 1", len(definitions))
	}
	definition := &definitions[0]
	if definition.PageKey != "user.list" || definition.DefinitionVersion != presentation.DefinitionVersionV2 {
		t.Fatalf("core definition identity = %q/%q", definition.PageKey, definition.DefinitionVersion)
	}
	if issues := presentation.ValidateCapability(definition); len(issues) > 0 {
		t.Fatalf("ValidateCapability(user.list) issues = %#v", issues)
	}
	hash, err := presentation.ComputeDefinitionHash(definition)
	if err != nil {
		t.Fatalf("ComputeDefinitionHash(user.list) error = %v", err)
	}
	if hash != definition.DefinitionHash {
		t.Fatalf("computed definition hash = %q, generated = %q", hash, definition.DefinitionHash)
	}
	if len(definition.DataSources) != 1 || definition.DataSources[0].MaxSortFields != 0 {
		t.Fatalf("user.list data source = %#v", definition.DataSources)
	}
	raw, err := json.Marshal(definition)
	if err != nil {
		t.Fatalf("json.Marshal(user.list) error = %v", err)
	}
	if !strings.Contains(string(raw), `"maxSortFields":0`) {
		t.Fatalf("zero sort limit disappeared from v2 wire identity: %s", raw)
	}

	definitions[0].PageKey = "mutated.list"
	definitions[0].Fields[0].Components[0] = "mutated-renderer"
	definitions[0].DataSources[0].RequiredPermissions[0] = "/mutated"
	fresh := Definitions()
	if fresh[0].PageKey != "user.list" || fresh[0].Fields[0].Components[0] == "mutated-renderer" ||
		fresh[0].DataSources[0].RequiredPermissions[0] != "/users" {
		t.Fatalf("Definitions() leaked caller mutation: %#v", fresh[0])
	}
}
