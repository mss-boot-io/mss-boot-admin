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

func TestParseDocumentAcceptsOpaqueRoleSubject(t *testing.T) {
	const roleID = "0123456789abcdef0123456789abcdef"
	raw := strings.Replace(
		validProfileJSON(`{"title":{"en-US":"Orders"}}`),
		`"scope":{"kind":"application"}`,
		`"scope":{"kind":"role","subject":"`+roleID+`"}`,
		1,
	)

	document, issues := ParseDocument([]byte(raw))
	require.Empty(t, issues)
	require.NotNil(t, document)
	require.Equal(t, ScopeRole, document.Profile.Metadata.Scope.Kind)
	require.Equal(t, roleID, *document.Profile.Metadata.Scope.Subject)
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
	_, err = NewRegistry(capability, capability)
	require.ErrorIs(t, err, ErrCapabilityAlreadyRegistered)
	require.NoError(t, registry.Freeze())
	require.ErrorIs(t, registry.Register(validCapability(t)), ErrRegistryFrozen)

	for _, pageKey := range []string{
		"account.users", "app-config.theme", "authorization.roles", "config.theme",
		"login.page", "presentation.governance", "system.settings",
	} {
		protected := capability
		protected.PageKey = pageKey
		protected.DefinitionHash, err = ComputeDefinitionHash(&protected)
		require.NoError(t, err)
		require.Contains(t, issueCodes(ValidateCapability(&protected)), "protected-page", pageKey)
	}

	overlong := capability
	overlong.PageKey = "a." + strings.Repeat("b", 119)
	overlong.DefinitionHash, err = ComputeDefinitionHash(&overlong)
	require.NoError(t, err)
	require.Contains(t, issueCodes(ValidateCapability(&overlong)), "invalid-identifier")

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
	  "form":{"fields":[{"field":"status","hidden":true,"visibleWhen":{"field":"status","operator":"eq","value":"open"}},{"field":"missing"}]},
  "actions":[{"action":"orders.missing"}]
}`)
	document, issues := ParseDocument([]byte(raw))
	require.Empty(t, issues)
	document.Profile.Metadata.DefinitionHash = "sha256:" + strings.Repeat("b", 64)
	issues = ValidateProfile(&capability, document.Profile)
	require.Subset(t, issueCodes(issues), []string{
		"definition-drift",
		"required-form-field-hidden",
		"required-form-field-conditional",
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

func TestVersionOneWireSerializationKeepsZeroValuedVersionTwoFactsCompatible(t *testing.T) {
	capability := validCapability(t)
	raw, err := json.Marshal(capability)
	require.NoError(t, err)
	require.Contains(t, string(raw), `"format":""`)
	require.Contains(t, string(raw), `"surfaceComponents":null`)
	require.Contains(t, string(raw), `"enumValues":null`)
	require.Contains(t, string(raw), `"validation":{}`)
}

func TestVersionOneActionlessCapabilityKeepsHistoricalDefinitionHash(t *testing.T) {
	label := text("状态", "Status")
	capability := CapabilityDefinition{
		PageKey:           "orders.list",
		DefinitionVersion: DefinitionVersionV1,
		Components:        []CapabilityComponent{{ID: "text"}},
		Fields: []CapabilityField{{
			ID: "status", Label: label, ValueType: "string",
			Surfaces: []Surface{SurfaceList}, Components: []string{"text"},
		}},
		DataSources: []CapabilityDataSource{{
			ID: "orders.list", RequiredPermissions: []string{"/orders"},
		}},
		Actions: nil,
		DefaultPresentation: CompletePresentation{
			Title: label, DataSource: "orders.list",
			List: CompleteListPresentation{
				Columns: []CompleteField{{Field: "status", Component: "text", Order: 0}},
				Density: "middle", PageSize: 20, DefaultSort: []Sort{},
			},
			Form:    CompleteFormPresentation{Columns: 1},
			Detail:  CompleteDetailPresentation{Columns: 1},
			Actions: nil,
		},
	}

	hash, err := ComputeDefinitionHash(&capability)
	require.NoError(t, err)
	require.Equal(t, "sha256:73ccff32c5e5e165826dd1aabbe99dbacea36d404a3388800b5fb38a7a4cd030", hash)
	capability.DefinitionHash = hash
	require.Empty(t, ValidateCapability(&capability))

	capability.Actions = []CapabilityAction{}
	emptyHash, err := ComputeDefinitionHash(&capability)
	require.NoError(t, err)
	require.NotEqual(t, hash, emptyHash)
}

func TestVersionOneDefinitionHashDistinguishesNilAndEmptyLegacySlices(t *testing.T) {
	base := CapabilityDefinition{PageKey: "orders.list", DefinitionVersion: DefinitionVersionV1}
	tests := []struct {
		name      string
		makeEmpty func(*CapabilityDefinition)
	}{
		{name: "fields", makeEmpty: func(capability *CapabilityDefinition) { capability.Fields = []CapabilityField{} }},
		{name: "data sources", makeEmpty: func(capability *CapabilityDefinition) { capability.DataSources = []CapabilityDataSource{} }},
		{name: "actions", makeEmpty: func(capability *CapabilityDefinition) { capability.Actions = []CapabilityAction{} }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			nilHash, err := ComputeDefinitionHash(&base)
			require.NoError(t, err)
			withEmpty := base
			test.makeEmpty(&withEmpty)
			emptyHash, err := ComputeDefinitionHash(&withEmpty)
			require.NoError(t, err)
			require.NotEqual(t, nilHash, emptyHash)
		})
	}
}

func TestVersionOneKeepsHistoricalQualifiedFieldIdentifiersCompatible(t *testing.T) {
	capability := validCapability(t)
	const legacyFieldID = "status-code"
	capability.Fields[0].ID = legacyFieldID
	capability.DefaultPresentation.List.Columns[0].Field = legacyFieldID
	capability.DefaultPresentation.List.DefaultSort[0].Field = legacyFieldID
	capability.DefaultPresentation.Search.Fields[0].Field = legacyFieldID
	capability.DefaultPresentation.Form.Fields[0].Field = legacyFieldID
	capability.DefaultPresentation.Detail.Fields[0].Field = legacyFieldID
	var err error
	capability.DefinitionHash, err = ComputeDefinitionHash(&capability)
	require.NoError(t, err)
	require.Empty(t, ValidateCapability(&capability))
}

func TestDefinitionHashVersionTwoCoversCompleteDefaultsAndUsesUTF8JSON(t *testing.T) {
	capability := validCapabilityV2(t)
	original := capability.DefinitionHash

	capability.DefaultPresentation.Title = text("订单 &<>", "Orders &<>")
	hash, err := ComputeDefinitionHash(&capability)
	require.NoError(t, err)
	require.NotEqual(t, original, hash)

	canonical, err := canonicalJSONV2(map[string]any{"text": "中文 &<>\u2028\u2029Literal\\u2028"})
	require.NoError(t, err)
	require.Equal(t, `{"text":"中文 &<>`+"\u2028\u2029"+`Literal\\u2028"}`, string(canonical))
}

func TestVersionTwoComponentSelectionUsesSurfaceBindings(t *testing.T) {
	capability := validCapabilityV2(t)
	document, issues := ParseDocument([]byte(validProfileJSON(`{
  "list":{"columns":[{"field":"status","component":"select"}]}
}`)))
	require.Empty(t, issues)
	document.Profile.Metadata.DefinitionHash = capability.DefinitionHash
	issues = ValidateProfile(&capability, document.Profile)
	require.Contains(t, issueCodes(issues), "unsupported-field-component")

	capability.DefaultPresentation.List.Columns[0].Component = "select"
	var err error
	capability.DefinitionHash, err = ComputeDefinitionHash(&capability)
	require.NoError(t, err)
	require.Contains(t, issueCodes(ValidateCapability(&capability)), "unsupported-field-component")
}

func TestVersionTwoRejectsNonPortableCompiledPattern(t *testing.T) {
	for _, pattern := range []string{
		`(?P<status>[A-Z]+)`, `^.$`, `^\s$`, `^\S$`, `^\u0041$`, `^\cA$`,
		`^[\b]$`, `^a{1001}$`, `^[]a]$`, `^[]$`, `^[^]$`, `^a{$`, `^a}$`, `^a]$`,
		`^\-$`, `^\_$`, `^\#$`,
	} {
		capability := validCapabilityV2(t)
		capability.Fields[0].Validation.Pattern = pattern
		var err error
		capability.DefinitionHash, err = ComputeDefinitionHash(&capability)
		require.NoError(t, err)
		require.Contains(t, issueCodes(ValidateCapability(&capability)), "non-portable-field-pattern", pattern)
	}
}

func TestVersionTwoAcceptsUnicodeSafePortablePatterns(t *testing.T) {
	for _, pattern := range []string{
		`^[A-Z0-9_-]+$`, `^[^A]$`, `^[^\n]$`, `^[\t\n\f\r ]$`, `^[\-]$`, `^(\{|\}|\])$`,
	} {
		capability := validCapabilityV2(t)
		capability.Fields[0].Validation.Pattern = pattern
		var err error
		capability.DefinitionHash, err = ComputeDefinitionHash(&capability)
		require.NoError(t, err)
		require.NotContains(t, issueCodes(ValidateCapability(&capability)), "non-portable-field-pattern", pattern)
	}
}

func TestVersionTwoCapabilityEnforcesCompiledPaginationAndSortLimits(t *testing.T) {
	capability := validCapabilityV2(t)
	document, issues := ParseDocument([]byte(validProfileJSON(`{
  "list":{"pageSize":200,"defaultSort":[
    {"field":"status","direction":"asc"},
    {"field":"status","direction":"desc"}
  ]}
}`)))
	require.Empty(t, issues)
	document.Profile.Metadata.DefinitionHash = capability.DefinitionHash

	issues = ValidateProfile(&capability, document.Profile)
	require.Subset(t, issueCodes(issues), []string{
		"page-size-exceeds-data-source-limit",
		"unsupported-page-size",
		"too-many-data-source-sort-fields",
	})

	unsupported := capability
	unsupported.DefinitionVersion = "3"
	issues = ValidateCapability(&unsupported)
	require.Contains(t, issueCodes(issues), "unsupported-definition-version")

	zeroSortLimit := validCapabilityV2(t)
	zeroSortLimit.DataSources[0].MaxSortFields = 0
	hash, err := ComputeDefinitionHash(&zeroSortLimit)
	require.NoError(t, err)
	zeroSortLimit.DefinitionHash = hash
	require.NotContains(t, issueCodes(ValidateCapability(&zeroSortLimit)), "invalid-max-sort-fields")
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

func validCapabilityV2(t *testing.T) CapabilityDefinition {
	t.Helper()
	capability := validCapability(t)
	capability.DefinitionVersion = DefinitionVersionV2
	capability.Fields[0].Format = "plain"
	capability.Fields[0].Searchable = true
	capability.Fields[0].SurfaceComponents = []CapabilitySurfaceComponents{
		{Surface: SurfaceList, Components: []string{"text"}},
		{Surface: SurfaceSearch, Components: []string{"select"}},
		{Surface: SurfaceForm, Components: []string{"select"}},
		{Surface: SurfaceDetail, Components: []string{"text"}},
	}
	capability.Fields[0].EnumValues = []CapabilityEnumValue{
		{Value: "open", Label: text("开放", "Open"), Color: "green"},
		{Value: "closed", Label: text("关闭", "Closed"), Color: "red"},
	}
	capability.DataSources[0].PageSizeOptions = []int{20, 50, 100}
	capability.DataSources[0].MaxPageSize = 100
	capability.DataSources[0].MaxSortFields = 1
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
