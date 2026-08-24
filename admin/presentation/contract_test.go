package presentation

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseDocumentStrictShapeAndCanonicalDigest(t *testing.T) {
	raw := []byte(`{
  "kind":"AdminPagePresentation",
  "apiVersion":"mss.io/v1alpha1",
  "metadata":{"scope":{"kind":"application"},"definitionHash":"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","pageKey":"orders.list","name":"orders-default"},
  "spec":{"search":{"collapsedByDefault":false},"list":{"pageSize":20}}
}`)
	document, issues := ParseDocument(raw)
	require.Empty(t, issues)
	require.NotNil(t, document)
	require.False(t, *document.Profile.Spec.Search.CollapsedByDefault)
	require.Equal(t, sha256Digest(document.Canonical), document.Digest)
	require.Equal(t,
		`{"apiVersion":"mss.io/v1alpha1","kind":"AdminPagePresentation","metadata":{"definitionHash":"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","name":"orders-default","pageKey":"orders.list","scope":{"kind":"application"}},"spec":{"list":{"pageSize":20},"search":{"collapsedByDefault":false}}}`,
		string(document.Canonical),
	)
}

func TestParseDocumentRejectsUnsafeOrAmbiguousJSON(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		code string
	}{
		{
			name: "unknown executable property",
			raw:  validProfileJSON(`{"script":"alert(1)"}`),
			code: "invalid-document",
		},
		{
			name: "duplicate property",
			raw:  strings.Replace(validProfileJSON(`{"pageSize":20}`), `"kind":"AdminPagePresentation"`, `"kind":"AdminPagePresentation","kind":"AdminPagePresentation"`, 1),
			code: "invalid-json",
		},
		{
			name: "unexpected null",
			raw:  validProfileJSON(`{"title":null}`),
			code: "unexpected-null",
		},
		{
			name: "empty spec",
			raw:  validProfileJSON(`{}`),
			code: "empty-spec",
		},
		{
			name: "oversized",
			raw:  strings.Repeat(" ", MaxDocumentBytes+1),
			code: "document-too-large",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			document, issues := ParseDocument([]byte(test.raw))
			require.Nil(t, document)
			require.Contains(t, issueCodes(issues), test.code)
		})
	}
}

func TestParseDocumentBoundsConditionShapeAndValues(t *testing.T) {
	tooManyValues := make([]any, 51)
	for index := range tooManyValues {
		tooManyValues[index] = index
	}
	encoded, err := json.Marshal(map[string]any{
		"actions": []any{map[string]any{
			"action": "orders.read",
			"visibleWhen": map[string]any{
				"field": "status", "operator": "in", "value": tooManyValues,
			},
		}},
	})
	require.NoError(t, err)
	document, issues := ParseDocument([]byte(validProfileJSON(string(encoded))))
	require.Nil(t, document)
	require.Contains(t, issueCodes(issues), "condition-value-size")
}

func TestCapabilityRegistryValidatesHashDuplicatesAndProtectedPages(t *testing.T) {
	capability := validCapability(t)
	require.Empty(t, ValidateCapability(&capability))

	registry, err := NewRegistry(capability)
	require.NoError(t, err)
	require.ErrorIs(t, registry.Register(capability), ErrCapabilityAlreadyRegistered)

	protected := capability
	protected.PageKey = "authorization.roles"
	protected.DefinitionHash, err = ComputeDefinitionHash(&protected)
	require.NoError(t, err)
	require.Contains(t, issueCodes(ValidateCapability(&protected)), "protected-page")

	duplicate := capability
	duplicate.Fields = append(duplicate.Fields, duplicate.Fields[0])
	duplicate.DefinitionHash, err = ComputeDefinitionHash(&duplicate)
	require.NoError(t, err)
	require.Contains(t, issueCodes(ValidateCapability(&duplicate)), "duplicate-capability-id")

	badHash := capability
	badHash.DefinitionHash = "sha256:" + strings.Repeat("0", 64)
	require.Contains(t, issueCodes(ValidateCapability(&badHash)), "definition-hash-mismatch")
}

func TestRegistryReturnsImmutableDefinitionClones(t *testing.T) {
	capability := validCapability(t)
	registry := MustNewRegistry(capability)
	first, ok := registry.Lookup(capability.PageKey)
	require.True(t, ok)
	first.Fields[0].ID = "mutated"
	first.Components[0].ID = "mutated"

	second, ok := registry.Lookup(capability.PageKey)
	require.True(t, ok)
	require.Equal(t, "status", second.Fields[0].ID)
	require.Equal(t, "text", second.Components[0].ID)
}

func TestSemanticValidationRejectsDriftUnknownReferencesAndRequiredHide(t *testing.T) {
	capability := validCapability(t)
	raw := validProfileJSON(`{
  "dataSource":"orders.unknown",
  "form":{"fields":[{"field":"status","hidden":true},{"field":"missing"}]},
  "actions":[{"action":"orders.missing"}]
}`)
	document, issues := ParseDocument([]byte(raw))
	require.Empty(t, issues)
	document.Profile.Metadata.DefinitionHash = "sha256:" + strings.Repeat("b", 64)
	issues = ValidateProfile(&capability, document.Profile)
	require.Subset(t, issueCodes(issues), []string{
		"definition-drift",
		"required-form-field-hidden",
		"unknown-action",
		"unknown-data-source",
		"unknown-field",
	})
}

func TestDefinitionHashCoversCompatibilityButNotDefaultPresentation(t *testing.T) {
	capability := validCapability(t)
	original := capability.DefinitionHash
	capability.DefaultPresentation.Title = text("Changed", "Changed")
	hash, err := ComputeDefinitionHash(&capability)
	require.NoError(t, err)
	require.Equal(t, original, hash)

	capability.Fields[0].Sortable = false
	hash, err = ComputeDefinitionHash(&capability)
	require.NoError(t, err)
	require.NotEqual(t, original, hash)
}

func validCapability(t *testing.T) CapabilityDefinition {
	t.Helper()
	capability := CapabilityDefinition{
		PageKey:           "orders.list",
		DefinitionVersion: DefinitionVersion,
		Components: []CapabilityComponent{
			{ID: "text"}, {ID: "select"},
		},
		Fields: []CapabilityField{
			{
				ID: "status", Label: text("状态", "Status"), ValueType: "enum", Required: true,
				Sortable: true, Filterable: true,
				Surfaces:   []Surface{SurfaceList, SurfaceSearch, SurfaceForm, SurfaceDetail},
				Components: []string{"text", "select"},
			},
		},
		DataSources: []CapabilityDataSource{
			{ID: "orders.list", RequiredPermissions: []string{"/orders", "/orders/permissions/list"}},
		},
		Actions: []CapabilityAction{
			{ID: "orders.read", RequiredPermissions: []string{"/orders/permissions/read"}, Placements: []ActionPlacement{PlacementRow}},
		},
		DefaultPresentation: CompletePresentation{
			Title: text("订单", "Orders"), DataSource: "orders.list",
			List: CompleteListPresentation{
				Columns: []CompleteField{{Field: "status", Component: "text", Order: 10}},
				Density: "middle", PageSize: 20,
				DefaultSort: []Sort{{Field: "status", Direction: "asc"}},
			},
			Search:  CompleteSearchPresentation{Fields: []CompleteField{{Field: "status", Component: "select", Order: 10}}},
			Form:    CompleteFormPresentation{Fields: []CompleteField{{Field: "status", Component: "select", Order: 10}}, Columns: 1},
			Detail:  CompleteDetailPresentation{Fields: []CompleteField{{Field: "status", Component: "text", Order: 10}}, Columns: 1},
			Actions: []CompleteAction{{Action: "orders.read", Placement: PlacementRow, Order: 10}},
		},
	}
	var err error
	capability.DefinitionHash, err = ComputeDefinitionHash(&capability)
	require.NoError(t, err)
	return capability
}

func validProfileJSON(spec string) string {
	return `{
  "apiVersion":"mss.io/v1alpha1",
  "kind":"AdminPagePresentation",
  "metadata":{
    "name":"orders-default",
    "pageKey":"orders.list",
    "definitionHash":"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
    "scope":{"kind":"application"}
  },
  "spec":` + spec + `
}`
}

func text(zhCN, enUS string) LocalizedText {
	return LocalizedText{ZhCN: &zhCN, EnUS: &enUS}
}

func issueCodes(issues []Issue) []string {
	result := make([]string, 0, len(issues))
	for _, current := range issues {
		result = append(result, current.Code)
	}
	return result
}
