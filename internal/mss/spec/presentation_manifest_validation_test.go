package spec

import (
	"strconv"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestValidateNormalizedPresentationManifestRejectsFinalDerivedViolations(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*NormalizedPresentationManifest)
		code   string
	}{
		{
			name:   "nil top-level collection",
			mutate: func(manifest *NormalizedPresentationManifest) { manifest.Components = nil },
			code:   "nil-collection",
		},
		{
			name: "overlong component id",
			mutate: func(manifest *NormalizedPresentationManifest) {
				manifest.Components[0].ID = strings.Repeat("component-", 14)
			},
			code: "invalid-identifier",
		},
		{
			name:   "protected page key",
			mutate: func(manifest *NormalizedPresentationManifest) { manifest.PageKey = "system.settings" },
			code:   "protected-page-key",
		},
		{
			name: "invalid field id",
			mutate: func(manifest *NormalizedPresentationManifest) {
				manifest.Fields[0].ID = strings.Repeat("a", 121)
			},
			code: "invalid-field-identifier",
		},
		{
			name: "empty explicit locale",
			mutate: func(manifest *NormalizedPresentationManifest) {
				manifest.DefaultPresentation.List.Columns[0].Placeholder = &PresentationLocalizedText{ZhCN: "提示", EnUS: ""}
			},
			code: "invalid-localized-text",
		},
		{
			name: "control character in localized text",
			mutate: func(manifest *NormalizedPresentationManifest) {
				manifest.Fields[0].Label.EnUS = "Order\x00code"
			},
			code: "invalid-localized-text",
		},
		{
			name: "overlong enum token",
			mutate: func(manifest *NormalizedPresentationManifest) {
				manifest.Fields[0].ValueType = "enum"
				manifest.Fields[0].EnumValues = []NormalizedPresentationEnumValue{{
					Value: strings.Repeat("a", 121),
					Label: PresentationLocalizedText{ZhCN: "测试", EnUS: "Test"},
				}}
			},
			code: "invalid-enum-value",
		},
		{
			name: "overlong permission",
			mutate: func(manifest *NormalizedPresentationManifest) {
				manifest.DataSources[0].RequiredPermissions[0] = "/" + strings.Repeat("a", 255)
			},
			code: "invalid-permission",
		},
		{
			name: "surface mapping mismatch",
			mutate: func(manifest *NormalizedPresentationManifest) {
				manifest.Fields[0].SurfaceComponents[0].Surface = "search"
			},
			code: "unexpected-surface-components",
		},
		{
			name: "non-decimal numeric bound",
			mutate: func(manifest *NormalizedPresentationManifest) {
				value := "0x1p2"
				manifest.Fields[0].Validation.Minimum = &value
			},
			code: "invalid-field-number-bound",
		},
		{
			name: "invalid precision and scale",
			mutate: func(manifest *NormalizedPresentationManifest) {
				precision, scale := 2, 3
				manifest.Fields[0].Validation.Precision = &precision
				manifest.Fields[0].Validation.Scale = &scale
			},
			code: "invalid-field-scale",
		},
		{
			name: "unsorted data source limits",
			mutate: func(manifest *NormalizedPresentationManifest) {
				manifest.DataSources[0].PageSizeOptions = []int{50, 20}
			},
			code: "unsorted-page-size-options",
		},
		{
			name: "negative maximum sort fields",
			mutate: func(manifest *NormalizedPresentationManifest) {
				manifest.DataSources[0].MaxSortFields = -1
			},
			code: "invalid-max-sort-fields",
		},
		{
			name: "nil complete default sort",
			mutate: func(manifest *NormalizedPresentationManifest) {
				manifest.DefaultPresentation.List.DefaultSort = nil
			},
			code: "nil-collection",
		},
		{
			name: "complete field width on wrong surface",
			mutate: func(manifest *NormalizedPresentationManifest) {
				width := 120
				manifest.DefaultPresentation.Form.Fields[0].Width = &width
			},
			code: "invalid-field-width-surface",
		},
		{
			name: "complete field span outside contract",
			mutate: func(manifest *NormalizedPresentationManifest) {
				span := 25
				manifest.DefaultPresentation.Detail.Fields[0].Span = &span
			},
			code: "invalid-field-span",
		},
		{
			name: "complete field order outside contract",
			mutate: func(manifest *NormalizedPresentationManifest) {
				manifest.DefaultPresentation.List.Columns[0].Order = 10001
			},
			code: "invalid-field-order",
		},
		{
			name: "complete default unknown field",
			mutate: func(manifest *NormalizedPresentationManifest) {
				manifest.DefaultPresentation.Search.Fields[0].Field = "unknownField"
			},
			code: "unknown-field",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			manifest, err := validPresentationModule().NormalizePresentation()
			if err != nil {
				t.Fatalf("NormalizePresentation(valid) error = %v", err)
			}
			test.mutate(manifest)
			issues := ValidateNormalizedPresentationManifest(manifest)
			if !issuesHaveCode(issues, test.code) {
				t.Fatalf("ValidateNormalizedPresentationManifest() issues = %#v, want code %q", issues, test.code)
			}
		})
	}

	if strconv.IntSize == 64 {
		manifest, err := validPresentationModule().NormalizePresentation()
		if err != nil {
			t.Fatalf("NormalizePresentation(valid) error = %v", err)
		}
		unsafeLength := int(normalizedPresentationMaxSafeInteger + 1)
		manifest.Fields[0].Validation.MaxLength = &unsafeLength
		if issues := ValidateNormalizedPresentationManifest(manifest); !issuesHaveCode(issues, "invalid-field-length") {
			t.Fatalf("unsafe integer issues = %#v", issues)
		}
	}
}

func TestParseModuleRejectsDerivedManifestBoundaryViolationsBeforeGeneration(t *testing.T) {
	tests := []struct {
		name   string
		module func() *Module
		mutate func(*Module)
		code   string
	}{
		{
			name:   "overlong module qualified id",
			module: validPresentationModule,
			mutate: func(module *Module) { module.Metadata.Name = strings.Repeat("a", 115) },
			code:   "invalid-identifier",
		},
		{
			name:   "overlong lower camel field id",
			module: validPresentationModule,
			mutate: func(module *Module) {
				oldID := module.Spec.Entity.Fields[0].Name
				newID := "a" + strings.Repeat("b", 120)
				module.Spec.Entity.Fields[0].Name = newID
				for _, fields := range [][]PresentationFieldSource{
					module.Spec.Presentation.List.Fields,
					module.Spec.Presentation.Search.Fields,
					module.Spec.Presentation.Form.Fields,
					module.Spec.Presentation.Detail.Fields,
				} {
					for index := range fields {
						if fields[index].Field == oldID {
							fields[index].Field = newID
						}
					}
				}
			},
			code: "invalid-field-identifier",
		},
		{
			name:   "overlong field display name",
			module: validPresentationModule,
			mutate: func(module *Module) { module.Spec.Entity.Fields[0].DisplayName = strings.Repeat("界", 201) },
			code:   "invalid-localized-text",
		},
		{
			name:   "overlong English field display name",
			module: validPresentationModule,
			mutate: func(module *Module) { module.Spec.Entity.Fields[0].DisplayNameEn = strings.Repeat("a", 201) },
			code:   "invalid-localized-text",
		},
		{
			name:   "non portable English field display name",
			module: validPresentationModule,
			mutate: func(module *Module) { module.Spec.Entity.Fields[0].DisplayNameEn = "Order\x00code" },
			code:   "invalid-localized-text",
		},
		{
			name:   "overlong enum label",
			module: validEnumPresentationModule,
			mutate: func(module *Module) { module.Spec.Entity.Fields[0].EnumValues[0].Label = strings.Repeat("界", 201) },
			code:   "invalid-localized-text",
		},
		{
			name:   "overlong enum value",
			module: validEnumPresentationModule,
			mutate: func(module *Module) { module.Spec.Entity.Fields[0].EnumValues[0].Value = strings.Repeat("a", 121) },
			code:   "invalid-enum-value",
		},
		{
			name:   "overlong menu permission path",
			module: validPresentationModule,
			mutate: func(module *Module) { module.Spec.Menu.Path = "/" + strings.Repeat("a", 255) },
			code:   "invalid-permission",
		},
		{
			name:   "empty explicit placeholder locale",
			module: validPresentationModule,
			mutate: func(module *Module) {
				module.Spec.Presentation.List.Fields[0].Placeholder = &PresentationLocalizedText{ZhCN: "提示", EnUS: ""}
			},
			code: "invalid-localized-text",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			module := test.module()
			test.mutate(module)
			encoded, err := yaml.Marshal(module)
			if err != nil {
				t.Fatalf("yaml.Marshal() error = %v", err)
			}
			_, err = ParseModule(encoded, "boundary.yaml")
			if err == nil || !validationErrorHasCode(err, test.code) {
				t.Fatalf("ParseModule() error = %v, want code %q", err, test.code)
			}
		})
	}
}

func TestNormalizedPresentationEnglishDisplayNameUsesTrimmedFallback(t *testing.T) {
	module := validPresentationModule()
	module.Spec.Entity.Fields[0].DisplayNameEn = "   "
	module.Normalize()
	manifest, err := module.NormalizePresentation()
	if err != nil {
		t.Fatalf("NormalizePresentation() error = %v", err)
	}
	field := normalizedField(t, manifest, module.Spec.Entity.Fields[0].Name)
	if field.Label.EnUS != module.Spec.Entity.Fields[0].DisplayName || strings.TrimSpace(field.Label.EnUS) != field.Label.EnUS {
		t.Fatalf("normalized English label = %q, want trimmed displayName fallback %q", field.Label.EnUS, module.Spec.Entity.Fields[0].DisplayName)
	}
}

func validEnumPresentationModule() *Module {
	module := validPresentationModule()
	fieldID := module.Spec.Entity.Fields[0].Name
	module.Spec.Entity.Fields[0].Type = "enum"
	module.Spec.Entity.Fields[0].UI.Component = "select"
	module.Spec.Entity.Fields[0].EnumValues = []EnumValue{{Value: "normal", Label: "正常", LabelEn: "Normal", Color: "blue"}}
	for index := range module.Spec.Presentation.List.Fields {
		if module.Spec.Presentation.List.Fields[index].Field == fieldID {
			module.Spec.Presentation.List.Fields[index].Component = "tag"
		}
	}
	for index := range module.Spec.Presentation.Search.Fields {
		if module.Spec.Presentation.Search.Fields[index].Field == fieldID {
			module.Spec.Presentation.Search.Fields[index].Component = "select"
		}
	}
	for index := range module.Spec.Presentation.Form.Fields {
		if module.Spec.Presentation.Form.Fields[index].Field == fieldID {
			module.Spec.Presentation.Form.Fields[index].Component = "select"
		}
	}
	for index := range module.Spec.Presentation.Detail.Fields {
		if module.Spec.Presentation.Detail.Fields[index].Field == fieldID {
			module.Spec.Presentation.Detail.Fields[index].Component = "tag"
		}
	}
	module.Normalize()
	return module
}

func issuesHaveCode(issues []Issue, code string) bool {
	for _, issue := range issues {
		if issue.Code == code {
			return true
		}
	}
	return false
}

func TestNormalizedPresentationDecimalValidatorRejectsNonFiniteText(t *testing.T) {
	for _, value := range []string{"NaN", "Inf", "-Inf", "1e9999", "+1", "01", "0x1p2"} {
		t.Run(value, func(t *testing.T) {
			manifest, err := validPresentationModule().NormalizePresentation()
			if err != nil {
				t.Fatalf("NormalizePresentation(valid) error = %v", err)
			}
			manifest.Fields[0].Validation.Minimum = &value
			if issues := ValidateNormalizedPresentationManifest(manifest); !issuesHaveCode(issues, "invalid-field-number-bound") {
				t.Fatalf("minimum %q issues = %#v", value, issues)
			}
		})
	}
}
