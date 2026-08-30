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

func TestLimitedTablePresentationRejectsConditionsThatRuntimeCannotEvaluate(t *testing.T) {
	limited := validCapabilityV2(t)
	limited.Actions = []CapabilityAction{}
	limited.DefaultPresentation.Form.Fields = []CompleteField{}
	limited.DefaultPresentation.Detail.Fields = []CompleteField{}
	limited.DefaultPresentation.Actions = []CompleteAction{}
	var err error
	limited.DefinitionHash, err = ComputeDefinitionHash(&limited)
	require.NoError(t, err)

	document, issues := ParseDocument([]byte(validProfileJSON(`{
  "list":{"columns":[{"field":"status","visibleWhen":{"field":"status","operator":"eq","value":"open"}}]},
  "search":{"fields":[{"field":"status","visibleWhen":{"field":"status","operator":"eq","value":"open"}}]}
}`)))
	require.Empty(t, issues)
	document.Profile.Metadata.DefinitionHash = limited.DefinitionHash
	issues = ValidateProfile(&limited, document.Profile)
	require.Equal(t, []string{"unsupported-limited-surface", "unsupported-limited-surface"}, issueCodes(issues))
	require.Equal(t, "spec.list.columns[0].visibleWhen", issues[0].Path)
	require.Equal(t, "spec.search.fields[0].visibleWhen", issues[1].Path)

	limited.DefaultPresentation.List.Columns[0].VisibleWhen = (*document.Profile.Spec.List.Columns)[0].VisibleWhen
	limited.DefinitionHash, err = ComputeDefinitionHash(&limited)
	require.NoError(t, err)
	require.Contains(t, issueCodes(ValidateCapability(&limited)), "unsupported-limited-condition")

	full := validCapabilityV2(t)
	document.Profile.Metadata.DefinitionHash = full.DefinitionHash
	require.NotContains(t, issueCodes(ValidateProfile(&full, document.Profile)), "unsupported-limited-surface")
}

func TestLimitedTablePresentationRejectsUnsupportedProfileSurfaces(t *testing.T) {
	limited := validCapabilityV2(t)
	limited.Actions = []CapabilityAction{}
	limited.DefaultPresentation.Form.Fields = []CompleteField{}
	limited.DefaultPresentation.Detail.Fields = []CompleteField{}
	limited.DefaultPresentation.Actions = []CompleteAction{}
	var err error
	limited.DefinitionHash, err = ComputeDefinitionHash(&limited)
	require.NoError(t, err)

	for _, testCase := range []struct {
		name     string
		spec     string
		wantPath string
		wantCode string
	}{
		{name: "data source", spec: `{"dataSource":"orders.list"}`, wantPath: "spec.dataSource", wantCode: "unsupported-limited-surface"},
		{name: "form", spec: `{"form":{"columns":1}}`, wantPath: "spec.form", wantCode: "unsupported-limited-surface"},
		{name: "detail", spec: `{"detail":{"columns":1}}`, wantPath: "spec.detail", wantCode: "unsupported-limited-surface"},
		{name: "actions", spec: `{"actions":[]}`, wantPath: "spec.actions", wantCode: "unsupported-limited-surface"},
		{name: "default sort", spec: `{"list":{"defaultSort":[{"field":"status","direction":"asc"}]}}`, wantPath: "spec.list.defaultSort", wantCode: "unsupported-limited-surface"},
		{name: "list component", spec: `{"list":{"columns":[{"field":"status","component":"text"}]}}`, wantPath: "spec.list.columns[0].component", wantCode: "unsupported-limited-surface"},
		{name: "list span", spec: `{"list":{"columns":[{"field":"status","span":1}]}}`, wantPath: "spec.list.columns[0].span", wantCode: "unsupported-limited-surface"},
		{name: "list placeholder", spec: `{"list":{"columns":[{"field":"status","placeholder":{"en-US":"Status"}}]}}`, wantPath: "spec.list.columns[0].placeholder", wantCode: "unsupported-limited-surface"},
		{name: "list help", spec: `{"list":{"columns":[{"field":"status","help":{"en-US":"Status help"}}]}}`, wantPath: "spec.list.columns[0].help", wantCode: "unsupported-limited-surface"},
		{name: "list condition with false value", spec: `{"list":{"columns":[{"field":"status","visibleWhen":{"field":"status","operator":"eq","value":false}}]}}`, wantPath: "spec.list.columns[0].visibleWhen", wantCode: "unsupported-limited-surface"},
		{name: "search component", spec: `{"search":{"fields":[{"field":"status","component":"select"}]}}`, wantPath: "spec.search.fields[0].component", wantCode: "unsupported-limited-surface"},
		{name: "search width", spec: `{"search":{"fields":[{"field":"status","width":120}]}}`, wantPath: "spec.search.fields[0].width", wantCode: "unsupported-limited-surface"},
		{name: "search span", spec: `{"search":{"fields":[{"field":"status","span":1}]}}`, wantPath: "spec.search.fields[0].span", wantCode: "unsupported-limited-surface"},
		{name: "search placeholder", spec: `{"search":{"fields":[{"field":"status","placeholder":{"en-US":"Status"}}]}}`, wantPath: "spec.search.fields[0].placeholder", wantCode: "unsupported-limited-surface"},
		{name: "search help", spec: `{"search":{"fields":[{"field":"status","help":{"en-US":"Status help"}}]}}`, wantPath: "spec.search.fields[0].help", wantCode: "unsupported-limited-surface"},
		{name: "search condition", spec: `{"search":{"fields":[{"field":"status","visibleWhen":{"field":"status","operator":"eq","value":"open"}}]}}`, wantPath: "spec.search.fields[0].visibleWhen", wantCode: "unsupported-limited-surface"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			document, parseIssues := ParseDocument([]byte(validProfileJSON(testCase.spec)))
			require.Empty(t, parseIssues)
			document.Profile.Metadata.DefinitionHash = limited.DefinitionHash

			issues := ValidateProfile(&limited, document.Profile)
			require.Equal(t, []string{testCase.wantCode}, issueCodes(issues))
			require.Equal(t, testCase.wantPath, issues[0].Path)

			full := validCapabilityV2(t)
			document.Profile.Metadata.DefinitionHash = full.DefinitionHash
			require.Empty(t, ValidateProfile(&full, document.Profile))
		})
	}
}

func TestLimitedTablePresentationAllowsOnlyWhitelistedListAndSearchProperties(t *testing.T) {
	limited := validCapabilityV2(t)
	limited.Actions = []CapabilityAction{}
	limited.DefaultPresentation.Form.Fields = []CompleteField{}
	limited.DefaultPresentation.Detail.Fields = []CompleteField{}
	limited.DefaultPresentation.Actions = []CompleteAction{}
	var err error
	limited.DefinitionHash, err = ComputeDefinitionHash(&limited)
	require.NoError(t, err)

	document, parseIssues := ParseDocument([]byte(validProfileJSON(`{
  "title":{"en-US":"Orders"},
  "list":{
    "columns":[{"field":"status","label":{"en-US":"Status"},"order":0,"hidden":false,"width":60}],
    "density":"compact",
    "pageSize":20
  },
  "search":{
    "fields":[{"field":"status","label":{"en-US":"Status"},"order":0,"hidden":false}],
    "collapsedByDefault":false
  }
}`)))
	require.Empty(t, parseIssues)
	document.Profile.Metadata.DefinitionHash = limited.DefinitionHash
	require.Empty(t, ValidateProfile(&limited, document.Profile))
}

func TestLimitedTablePresentationRejectsForbiddenPropertiesByPresence(t *testing.T) {
	limited := validCapabilityV2(t)
	limited.Actions = []CapabilityAction{}
	limited.DefaultPresentation.Form.Fields = []CompleteField{}
	limited.DefaultPresentation.Detail.Fields = []CompleteField{}
	limited.DefaultPresentation.Actions = []CompleteAction{}
	var err error
	limited.DefinitionHash, err = ComputeDefinitionHash(&limited)
	require.NoError(t, err)

	for _, testCase := range []struct {
		name     string
		mutate   func(*Profile)
		wantPath string
		wantCode string
	}{
		{
			name: "empty data source",
			mutate: func(profile *Profile) {
				empty := ""
				profile.Spec.DataSource = &empty
			},
			wantPath: "spec.dataSource", wantCode: "unsupported-limited-surface",
		},
		{
			name: "empty default sort",
			mutate: func(profile *Profile) {
				empty := []Sort{}
				profile.Spec.List = &ListPatch{DefaultSort: &empty}
			},
			wantPath: "spec.list.defaultSort", wantCode: "unsupported-limited-surface",
		},
		{
			name: "empty list component",
			mutate: func(profile *Profile) {
				empty := ""
				fields := []FieldPatch{{Field: "status", Component: &empty}}
				profile.Spec.List = &ListPatch{Columns: &fields}
			},
			wantPath: "spec.list.columns[0].component", wantCode: "unsupported-limited-surface",
		},
		{
			name: "zero list span",
			mutate: func(profile *Profile) {
				zero := 0
				fields := []FieldPatch{{Field: "status", Span: &zero}}
				profile.Spec.List = &ListPatch{Columns: &fields}
			},
			wantPath: "spec.list.columns[0].span", wantCode: "unsupported-limited-surface",
		},
		{
			name: "empty list placeholder",
			mutate: func(profile *Profile) {
				fields := []FieldPatch{{Field: "status", Placeholder: &LocalizedText{}}}
				profile.Spec.List = &ListPatch{Columns: &fields}
			},
			wantPath: "spec.list.columns[0].placeholder", wantCode: "unsupported-limited-surface",
		},
		{
			name: "empty list help",
			mutate: func(profile *Profile) {
				fields := []FieldPatch{{Field: "status", Help: &LocalizedText{}}}
				profile.Spec.List = &ListPatch{Columns: &fields}
			},
			wantPath: "spec.list.columns[0].help", wantCode: "unsupported-limited-surface",
		},
		{
			name: "empty list condition",
			mutate: func(profile *Profile) {
				fields := []FieldPatch{{Field: "status", VisibleWhen: &Condition{}}}
				profile.Spec.List = &ListPatch{Columns: &fields}
			},
			wantPath: "spec.list.columns[0].visibleWhen", wantCode: "unsupported-limited-surface",
		},
		{
			name: "empty search component",
			mutate: func(profile *Profile) {
				empty := ""
				fields := []FieldPatch{{Field: "status", Component: &empty}}
				profile.Spec.Search = &SearchPatch{Fields: &fields}
			},
			wantPath: "spec.search.fields[0].component", wantCode: "unsupported-limited-surface",
		},
		{
			name: "zero search width",
			mutate: func(profile *Profile) {
				zero := 0
				fields := []FieldPatch{{Field: "status", Width: &zero}}
				profile.Spec.Search = &SearchPatch{Fields: &fields}
			},
			wantPath: "spec.search.fields[0].width", wantCode: "unsupported-limited-surface",
		},
		{
			name: "zero search span",
			mutate: func(profile *Profile) {
				zero := 0
				fields := []FieldPatch{{Field: "status", Span: &zero}}
				profile.Spec.Search = &SearchPatch{Fields: &fields}
			},
			wantPath: "spec.search.fields[0].span", wantCode: "unsupported-limited-surface",
		},
		{
			name: "empty search placeholder",
			mutate: func(profile *Profile) {
				fields := []FieldPatch{{Field: "status", Placeholder: &LocalizedText{}}}
				profile.Spec.Search = &SearchPatch{Fields: &fields}
			},
			wantPath: "spec.search.fields[0].placeholder", wantCode: "unsupported-limited-surface",
		},
		{
			name: "empty search help",
			mutate: func(profile *Profile) {
				fields := []FieldPatch{{Field: "status", Help: &LocalizedText{}}}
				profile.Spec.Search = &SearchPatch{Fields: &fields}
			},
			wantPath: "spec.search.fields[0].help", wantCode: "unsupported-limited-surface",
		},
		{
			name: "empty search condition",
			mutate: func(profile *Profile) {
				fields := []FieldPatch{{Field: "status", VisibleWhen: &Condition{}}}
				profile.Spec.Search = &SearchPatch{Fields: &fields}
			},
			wantPath: "spec.search.fields[0].visibleWhen", wantCode: "unsupported-limited-surface",
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			document, parseIssues := ParseDocument([]byte(validProfileJSON(`{"title":{"en-US":"Orders"}}`)))
			require.Empty(t, parseIssues)
			document.Profile.Metadata.DefinitionHash = limited.DefinitionHash
			testCase.mutate(document.Profile)

			issues := ValidateProfile(&limited, document.Profile)
			found := false
			for _, current := range issues {
				if current.Code == testCase.wantCode && current.Path == testCase.wantPath {
					found = true
					break
				}
			}
			require.True(t, found, "issues = %#v", issues)
		})
	}
}

func TestProfileFieldReferencesAcceptVersionTwoLowerCamelIdentifiers(t *testing.T) {
	capability := validCapabilityV2(t)
	fieldTemplate := capability.Fields[0]
	capability.Fields = make([]CapabilityField, 0, 5)
	for _, fieldSpec := range []struct {
		id        string
		surface   Surface
		component string
	}{
		{id: "contactEmail", surface: SurfaceList, component: "text"},
		{id: "contactName", surface: SurfaceSearch, component: "select"},
		{id: "creditLevel", surface: SurfaceForm, component: "select"},
		{id: "createdAt", surface: SurfaceDetail, component: "text"},
		{id: "updatedAt", surface: SurfaceList, component: "text"},
	} {
		field := fieldTemplate
		field.ID = fieldSpec.id
		field.Surfaces = []Surface{fieldSpec.surface}
		field.Components = []string{fieldSpec.component}
		field.SurfaceComponents = []CapabilitySurfaceComponents{{
			Surface: fieldSpec.surface, Components: []string{fieldSpec.component},
		}}
		field.Searchable = fieldSpec.surface == SurfaceSearch
		field.Sortable = fieldSpec.id == "updatedAt"
		field.Filterable = false
		capability.Fields = append(capability.Fields, field)
	}
	listColumn := capability.DefaultPresentation.List.Columns[0]
	listColumn.Field = "contactEmail"
	updatedAtColumn := listColumn
	updatedAtColumn.Field = "updatedAt"
	updatedAtColumn.Order = 20
	capability.DefaultPresentation.List.Columns = []CompleteField{listColumn, updatedAtColumn}
	capability.DefaultPresentation.List.DefaultSort[0].Field = "updatedAt"
	capability.DefaultPresentation.Search.Fields[0].Field = "contactName"
	capability.DefaultPresentation.Form.Fields[0].Field = "creditLevel"
	capability.DefaultPresentation.Detail.Fields[0].Field = "createdAt"
	var err error
	capability.DefinitionHash, err = ComputeDefinitionHash(&capability)
	require.NoError(t, err)
	require.Empty(t, ValidateCapability(&capability))

	document, parseIssues := ParseDocument([]byte(validProfileJSON(`{
  "dataSource":"orders.list",
  "list":{
    "columns":[{"field":"contactEmail","component":"text","visibleWhen":{"field":"contactName","operator":"eq","value":"open"}}],
    "defaultSort":[{"field":"updatedAt","direction":"asc"}]
  },
  "search":{"fields":[{"field":"contactName","component":"select"}]},
  "form":{"fields":[{"field":"creditLevel","component":"select"}]},
  "detail":{"fields":[{"field":"createdAt","component":"text"}]},
  "actions":[{"action":"orders.read","placement":"row"}]
}`)))
	require.Empty(t, parseIssues)
	document.Profile.Metadata.DefinitionHash = capability.DefinitionHash
	require.Empty(t, ValidateProfile(&capability, document.Profile))

	unknownDocument, parseIssues := ParseDocument([]byte(validProfileJSON(`{
  "list":{"columns":[{"field":"unknownField"}]}
}`)))
	require.Empty(t, parseIssues)
	unknownDocument.Profile.Metadata.DefinitionHash = capability.DefinitionHash
	semanticIssues := ValidateProfile(&capability, unknownDocument.Profile)
	require.Contains(t, issueCodes(semanticIssues), "unknown-field")
	require.NotContains(t, issueCodes(semanticIssues), "invalid-identifier")
}

func TestProfileFieldReferencesRejectInvalidIdentifiersAtEverySurface(t *testing.T) {
	_, issues := ParseDocument([]byte(validProfileJSON(`{
  "list":{
    "columns":[
      {"field":"ContactEmail"},
      {"field":"status","visibleWhen":{"field":"contact:email","operator":"eq","value":"open"}}
    ],
    "defaultSort":[{"field":"contact@email","direction":"asc"}]
  },
  "search":{"fields":[{"field":"contact_name"}]},
  "form":{"fields":[{"field":"contact/name"}]},
  "detail":{"fields":[{"field":"contact name"}]}
}`)))
	require.Len(t, issues, 6)
	paths := make([]string, 0, len(issues))
	for _, current := range issues {
		require.Equal(t, "invalid-identifier", current.Code)
		paths = append(paths, current.Path)
	}
	require.ElementsMatch(t, []string{
		"spec.list.columns[0].field",
		"spec.list.columns[1].visibleWhen.field",
		"spec.list.defaultSort[0].field",
		"spec.search.fields[0].field",
		"spec.form.fields[0].field",
		"spec.detail.fields[0].field",
	}, paths)
}

func TestProfileCamelCaseFieldSupportDoesNotRelaxOtherIdentifiers(t *testing.T) {
	raw := validProfileJSON(`{
  "dataSource":"Orders.list",
  "list":{"columns":[{"field":"contactEmail","component":"TextInput"}]},
  "actions":[{"action":"orders.Read"}]
}`)
	raw = strings.Replace(raw, `"pageKey":"orders.list"`, `"pageKey":"Orders.list"`, 1)
	_, issues := ParseDocument([]byte(raw))
	require.Len(t, issues, 4)
	paths := make([]string, 0, len(issues))
	for _, current := range issues {
		require.Equal(t, "invalid-identifier", current.Code)
		paths = append(paths, current.Path)
	}
	require.ElementsMatch(t, []string{
		"metadata.pageKey",
		"spec.dataSource",
		"spec.list.columns[0].component",
		"spec.actions[0].action",
	}, paths)
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
