package core

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/mss-boot-io/mss-boot-admin/admin/presentation"
)

func TestDefinitionsAreValidHashedAndDefensivelyCopied(t *testing.T) {
	definitions := Definitions()
	wantPageKeys := []string{
		"department.list", "language.list", "log.audit", "log.login", "log.runtime", "menu.list", "notice.list",
		"online-session.list", "option.list", "post.list", "role.list", "system-config.list", "task.list", "user.list",
	}
	if len(definitions) != len(wantPageKeys) {
		t.Fatalf("Definitions() count = %d, want %d", len(definitions), len(wantPageKeys))
	}
	for index := range definitions {
		definition := &definitions[index]
		if definition.PageKey != wantPageKeys[index] || definition.DefinitionVersion != presentation.DefinitionVersionV2 {
			t.Fatalf("core definition %d identity = %q/%q", index, definition.PageKey, definition.DefinitionVersion)
		}
		if issues := presentation.ValidateCapability(definition); len(issues) > 0 {
			t.Fatalf("ValidateCapability(%s) issues = %#v", definition.PageKey, issues)
		}
		hash, err := presentation.ComputeDefinitionHash(definition)
		if err != nil {
			t.Fatalf("ComputeDefinitionHash(%s) error = %v", definition.PageKey, err)
		}
		if hash != definition.DefinitionHash {
			t.Fatalf("%s computed definition hash = %q, generated = %q", definition.PageKey, hash, definition.DefinitionHash)
		}
		if len(definition.DataSources) != 1 || definition.DataSources[0].MaxSortFields != 0 ||
			!slices.Equal(definition.DataSources[0].PageSizeOptions, []int{20, 50, 100}) {
			t.Fatalf("%s data source = %#v", definition.PageKey, definition.DataSources)
		}
		if len(definition.Actions) != 0 || len(definition.DefaultPresentation.Form.Fields) != 0 ||
			len(definition.DefaultPresentation.Detail.Fields) != 0 || len(definition.DefaultPresentation.Actions) != 0 {
			t.Fatalf("%s exposed form, detail, or actions", definition.PageKey)
		}
		raw, err := json.Marshal(definition)
		if err != nil {
			t.Fatalf("json.Marshal(%s) error = %v", definition.PageKey, err)
		}
		if !strings.Contains(string(raw), `"maxSortFields":0`) {
			t.Fatalf("%s zero sort limit disappeared from v2 wire identity: %s", definition.PageKey, raw)
		}
		for _, field := range definition.Fields {
			lower := strings.ToLower(field.ID)
			for _, forbidden := range []string{"password", "credential", "secret", "token", "sessionid", "userid", "roleid"} {
				if strings.Contains(lower, forbidden) {
					t.Errorf("%s exposes sensitive field %q", definition.PageKey, field.ID)
				}
			}
		}
	}

	userIndex := slices.Index(wantPageKeys, "user.list")
	definitions[userIndex].PageKey = "mutated.list"
	definitions[userIndex].Fields[0].Components[0] = "mutated-renderer"
	definitions[userIndex].DataSources[0].RequiredPermissions[0] = "/mutated"
	fresh := Definitions()
	if fresh[userIndex].PageKey != "user.list" || fresh[userIndex].Fields[0].Components[0] == "mutated-renderer" ||
		fresh[userIndex].DataSources[0].RequiredPermissions[0] != "/users" {
		t.Fatalf("Definitions() leaked caller mutation: %#v", fresh[userIndex])
	}
}
