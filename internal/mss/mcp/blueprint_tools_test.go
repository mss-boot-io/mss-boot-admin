package mcp

import (
	"context"
	"testing"
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
