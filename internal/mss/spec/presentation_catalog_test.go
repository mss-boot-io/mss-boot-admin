package spec

import (
	"encoding/json"
	"slices"
	"strconv"
	"strings"
	"testing"

	distribution "github.com/mss-boot-io/mss-boot-admin"
)

func TestPresentationCatalogEmbeddedClosedInventoryAndLimits(t *testing.T) {
	embedded := distribution.EmbeddedFS()
	for _, path := range []string{
		".mss/admin-presentation-catalog.yaml",
		".mss/schemas/admin-presentation-catalog.schema.json",
	} {
		if _, err := embedded.ReadFile(path); err != nil {
			t.Fatalf("embedded Distribution is missing %s: %v", path, err)
		}
	}

	catalog, err := DefaultPresentationCatalog()
	if err != nil {
		t.Fatalf("DefaultPresentationCatalog() error = %v", err)
	}
	if catalog.Metadata.DefinitionVersion != PresentationDefinitionVersion {
		t.Fatalf("catalog definition version = %q", catalog.Metadata.DefinitionVersion)
	}
	componentIDs := make([]string, 0, len(catalog.Spec.Components))
	for _, component := range catalog.Spec.Components {
		componentIDs = append(componentIDs, component.ID)
	}
	for _, expected := range []string{
		"boolean", "boolean-filter", "copyable-code", "date-time", "email-input",
		"input", "select", "switch", "tag", "text",
	} {
		if !slices.Contains(componentIDs, expected) {
			t.Fatalf("catalog components = %#v, missing %q", componentIDs, expected)
		}
	}
	dataSource, ok := catalog.dataSource("list")
	if !ok {
		t.Fatal("catalog is missing list data source")
	}
	if !slices.Equal(dataSource.PageSizeOptions, []int{20, 50, 100}) || dataSource.MaxPageSize != 100 || dataSource.MaxSortFields != 1 {
		t.Fatalf("list limits = %#v", dataSource)
	}
	for _, test := range []struct {
		component string
		valueType string
		format    string
		readOnly  bool
		surface   string
	}{
		{component: "boolean-filter", valueType: "boolean", format: "plain", surface: "search"},
		{component: "copyable-code", valueType: "string", format: "identifier", readOnly: true, surface: "detail"},
		{component: "date-time", valueType: "date-time", format: "date-time", readOnly: true, surface: "detail"},
	} {
		if got := catalog.compatibleComponents(test.valueType, test.format, test.readOnly, test.surface); !slices.Contains(got, test.component) {
			t.Fatalf("compatible components for %#v = %#v", test, got)
		}
	}
}

func TestPresentationCatalogSchemaIsStrictAndCatalogParserRejectsUnknownFields(t *testing.T) {
	data, err := distribution.EmbeddedFS().ReadFile(".mss/schemas/admin-presentation-catalog.schema.json")
	if err != nil {
		t.Fatalf("read embedded catalog schema: %v", err)
	}
	var schema map[string]any
	if err := json.Unmarshal(data, &schema); err != nil {
		t.Fatalf("parse catalog schema: %v", err)
	}
	walkClosedObjectSchemas(t, schema, "$")

	catalogData, err := distribution.EmbeddedFS().ReadFile(".mss/admin-presentation-catalog.yaml")
	if err != nil {
		t.Fatalf("read embedded catalog: %v", err)
	}
	catalogData = append(catalogData, []byte("unknownExecutableContract: true\n")...)
	_, err = ParsePresentationCatalog(catalogData, "catalog.yaml")
	if err == nil || !strings.Contains(err.Error(), "field unknownExecutableContract not found") {
		t.Fatalf("ParsePresentationCatalog() error = %v, want strict unknown-field rejection", err)
	}
}

func TestPresentationCatalogSemanticValidationEnforcesSchemaIDLimit(t *testing.T) {
	for _, test := range []struct {
		name string
		edit func(*PresentationCatalog)
		code string
	}{
		{
			name: "component",
			edit: func(catalog *PresentationCatalog) {
				catalog.Spec.Components[0].ID = "a" + strings.Repeat("b", 64)
			},
			code: "invalid-component-id",
		},
		{
			name: "single character component",
			edit: func(catalog *PresentationCatalog) {
				catalog.Spec.Components[0].ID = "a"
			},
			code: "invalid-component-id",
		},
		{
			name: "local id",
			edit: func(catalog *PresentationCatalog) {
				catalog.Spec.DataSources[0].ID = "a" + strings.Repeat("b", 64)
			},
			code: "invalid-local-id",
		},
		{
			name: "single character local id",
			edit: func(catalog *PresentationCatalog) {
				catalog.Spec.DataSources[0].ID = "a"
			},
			code: "invalid-local-id",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			catalog, err := DefaultPresentationCatalog()
			if err != nil {
				t.Fatalf("DefaultPresentationCatalog() error = %v", err)
			}
			test.edit(catalog)
			for _, issue := range catalog.Validate() {
				if issue.Code == test.code {
					return
				}
			}
			t.Fatalf("Validate() accepted a catalog identifier beyond the schema limit")
		})
	}
}

func TestParsePresentationCatalogRequiresExplicitDestructiveAndRejectsNull(t *testing.T) {
	data, err := distribution.EmbeddedFS().ReadFile(".mss/admin-presentation-catalog.yaml")
	if err != nil {
		t.Fatalf("read embedded catalog: %v", err)
	}
	tests := []struct {
		name string
		old  string
		new  string
		path string
		code string
	}{
		{
			name: "destructive omitted",
			old:  "    - id: create\n      apiOperation: create\n      permissionAction: create\n      placements: [toolbar]\n      destructive: false\n",
			new:  "    - id: create\n      apiOperation: create\n      permissionAction: create\n      placements: [toolbar]\n",
			path: "spec.actions[0].destructive",
			code: "required",
		},
		{
			name: "destructive null",
			old:  "    - id: create\n      apiOperation: create\n      permissionAction: create\n      placements: [toolbar]\n      destructive: false\n",
			new:  "    - id: create\n      apiOperation: create\n      permissionAction: create\n      placements: [toolbar]\n      destructive: null\n",
			path: "spec.actions[0].destructive",
			code: "null-not-allowed",
		},
		{
			name: "max page size omitted remains invalid",
			old:  "      maxPageSize: 100\n",
			new:  "",
			path: "spec.dataSources[0].maxPageSize",
			code: "max-page-size-out-of-range",
		},
		{
			name: "max sort fields omitted remains invalid",
			old:  "      maxSortFields: 1\n",
			new:  "",
			path: "spec.dataSources[0].maxSortFields",
			code: "max-sort-fields-out-of-range",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			mutated := replacePresentationTestYAML(t, data, test.old, test.new)
			_, err := ParsePresentationCatalog(mutated, "catalog.yaml")
			if !validationErrorHasIssue(err, test.path, test.code) {
				t.Fatalf("ParsePresentationCatalog() error = %v, want %s issue at %s", err, test.code, test.path)
			}
		})
	}
	if _, err := ParsePresentationCatalog(data, "catalog.yaml"); err != nil {
		t.Fatalf("ParsePresentationCatalog(explicit false) error = %v", err)
	}
}

func TestParsePresentationCatalogRejectsSchemaScalarCoercion(t *testing.T) {
	data, err := distribution.EmbeddedFS().ReadFile(".mss/admin-presentation-catalog.yaml")
	if err != nil {
		t.Fatalf("read embedded catalog: %v", err)
	}
	tests := []struct {
		name string
		old  string
		new  string
		path string
	}{
		{name: "api version boolean", old: "apiVersion: mss.io/v1alpha1\n", new: "apiVersion: true\n", path: "apiVersion"},
		{name: "kind integer", old: "kind: AdminPresentationCatalog\n", new: "kind: 123\n", path: "kind"},
		{name: "metadata name boolean", old: "  name: admin-presentation-v2\n", new: "  name: true\n", path: "metadata.name"},
		{name: "definition version integer", old: "  definitionVersion: \"2\"\n", new: "  definitionVersion: 2\n", path: "metadata.definitionVersion"},
		{
			name: "component id boolean",
			old:  "    - id: boolean\n      valueTypes: [boolean]\n",
			new:  "    - id: true\n      valueTypes: [boolean]\n",
			path: "spec.components[0].id",
		},
		{
			name: "value type boolean",
			old:  "    - id: boolean\n      valueTypes: [boolean]\n      formats: [plain]\n",
			new:  "    - id: boolean\n      valueTypes: [true]\n      formats: [plain]\n",
			path: "spec.components[0].valueTypes[0]",
		},
		{
			name: "format integer",
			old:  "    - id: boolean\n      valueTypes: [boolean]\n      formats: [plain]\n",
			new:  "    - id: boolean\n      valueTypes: [boolean]\n      formats: [123]\n",
			path: "spec.components[0].formats[0]",
		},
		{
			name: "surface integer",
			old:  "    - id: boolean\n      valueTypes: [boolean]\n      formats: [plain]\n      surfaces: [list, detail]\n",
			new:  "    - id: boolean\n      valueTypes: [boolean]\n      formats: [plain]\n      surfaces: [123, detail]\n",
			path: "spec.components[0].surfaces[0]",
		},
		{
			name: "read only boolean",
			old:  "    - id: boolean\n      valueTypes: [boolean]\n      formats: [plain]\n      surfaces: [list, detail]\n      readOnly: any\n",
			new:  "    - id: boolean\n      valueTypes: [boolean]\n      formats: [plain]\n      surfaces: [list, detail]\n      readOnly: true\n",
			path: "spec.components[0].readOnly",
		},
		{
			name: "data source id boolean",
			old:  "    - id: list\n      apiOperation: list\n",
			new:  "    - id: true\n      apiOperation: list\n",
			path: "spec.dataSources[0].id",
		},
		{name: "page size option float", old: "      pageSizeOptions: [20, 50, 100]\n", new: "      pageSizeOptions: [20.0, 50, 100]\n", path: "spec.dataSources[0].pageSizeOptions[0]"},
		{name: "max page size float", old: "      maxPageSize: 100\n", new: "      maxPageSize: 100.0\n", path: "spec.dataSources[0].maxPageSize"},
		{name: "max sort fields float", old: "      maxSortFields: 1\n", new: "      maxSortFields: 1.0\n", path: "spec.dataSources[0].maxSortFields"},
		{
			name: "action id boolean",
			old:  "    - id: create\n      apiOperation: create\n",
			new:  "    - id: true\n      apiOperation: create\n",
			path: "spec.actions[0].id",
		},
		{
			name: "placement integer",
			old:  "    - id: create\n      apiOperation: create\n      permissionAction: create\n      placements: [toolbar]\n",
			new:  "    - id: create\n      apiOperation: create\n      permissionAction: create\n      placements: [123]\n",
			path: "spec.actions[0].placements[0]",
		},
		{
			name: "destructive string",
			old:  "    - id: create\n      apiOperation: create\n      permissionAction: create\n      placements: [toolbar]\n      destructive: false\n",
			new:  "    - id: create\n      apiOperation: create\n      permissionAction: create\n      placements: [toolbar]\n      destructive: yes\n",
			path: "spec.actions[0].destructive",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			mutated := replacePresentationTestYAML(t, data, test.old, test.new)
			_, err := ParsePresentationCatalog(mutated, "catalog.yaml")
			if !validationErrorHasIssue(err, test.path, "yaml-type-mismatch") {
				t.Fatalf("ParsePresentationCatalog() error = %v, want yaml-type-mismatch issue at %s", err, test.path)
			}
		})
	}

	aliased := replacePresentationTestYAML(t, data, "      maxPageSize: 100\n", "      maxPageSize: &catalogInteger 100\n")
	aliased = replacePresentationTestYAML(
		t,
		aliased,
		"    - id: create\n      apiOperation: create\n",
		"    - id: *catalogInteger\n      apiOperation: create\n",
	)
	_, err = ParsePresentationCatalog(aliased, "catalog.yaml")
	if !validationErrorHasIssue(err, "spec.actions[0].id", "yaml-type-mismatch") {
		t.Fatalf("ParsePresentationCatalog(aliased integer id) error = %v, want yaml-type-mismatch", err)
	}
}

func TestParsePresentationCatalogRejectsTokenWhitespace(t *testing.T) {
	data, err := distribution.EmbeddedFS().ReadFile(".mss/admin-presentation-catalog.yaml")
	if err != nil {
		t.Fatalf("read embedded catalog: %v", err)
	}
	tests := []struct {
		name string
		old  string
		new  string
		path string
	}{
		{name: "api version", old: "apiVersion: mss.io/v1alpha1\n", new: "apiVersion: \" mss.io/v1alpha1 \"\n", path: "apiVersion"},
		{name: "kind", old: "kind: AdminPresentationCatalog\n", new: "kind: \" AdminPresentationCatalog \"\n", path: "kind"},
		{name: "metadata name", old: "  name: admin-presentation-v2\n", new: "  name: \" admin-presentation-v2 \"\n", path: "metadata.name"},
		{name: "definition version", old: "  definitionVersion: \"2\"\n", new: "  definitionVersion: \" 2 \"\n", path: "metadata.definitionVersion"},
		{
			name: "component id",
			old:  "    - id: boolean\n      valueTypes: [boolean]\n",
			new:  "    - id: \" boolean \"\n      valueTypes: [boolean]\n",
			path: "spec.components[0].id",
		},
		{
			name: "value type",
			old:  "    - id: boolean\n      valueTypes: [boolean]\n      formats: [plain]\n",
			new:  "    - id: boolean\n      valueTypes: [\" boolean \"]\n      formats: [plain]\n",
			path: "spec.components[0].valueTypes[0]",
		},
		{
			name: "format",
			old:  "    - id: boolean\n      valueTypes: [boolean]\n      formats: [plain]\n",
			new:  "    - id: boolean\n      valueTypes: [boolean]\n      formats: [\" plain \"]\n",
			path: "spec.components[0].formats[0]",
		},
		{
			name: "surface",
			old:  "    - id: boolean\n      valueTypes: [boolean]\n      formats: [plain]\n      surfaces: [list, detail]\n",
			new:  "    - id: boolean\n      valueTypes: [boolean]\n      formats: [plain]\n      surfaces: [\" list \", detail]\n",
			path: "spec.components[0].surfaces[0]",
		},
		{
			name: "read only",
			old:  "    - id: boolean\n      valueTypes: [boolean]\n      formats: [plain]\n      surfaces: [list, detail]\n      readOnly: any\n",
			new:  "    - id: boolean\n      valueTypes: [boolean]\n      formats: [plain]\n      surfaces: [list, detail]\n      readOnly: \" any \"\n",
			path: "spec.components[0].readOnly",
		},
		{
			name: "data source operation",
			old:  "    - id: list\n      apiOperation: list\n",
			new:  "    - id: list\n      apiOperation: \" list \"\n",
			path: "spec.dataSources[0].apiOperation",
		},
		{
			name: "action placement",
			old:  "    - id: create\n      apiOperation: create\n      permissionAction: create\n      placements: [toolbar]\n",
			new:  "    - id: create\n      apiOperation: create\n      permissionAction: create\n      placements: [\" toolbar \"]\n",
			path: "spec.actions[0].placements[0]",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			mutated := replacePresentationTestYAML(t, data, test.old, test.new)
			_, err := ParsePresentationCatalog(mutated, "catalog.yaml")
			if !validationErrorHasIssue(err, test.path, "yaml-token-whitespace") {
				t.Fatalf("ParsePresentationCatalog() error = %v, want yaml-token-whitespace issue at %s", err, test.path)
			}
		})
	}
}

func TestParsePresentationCatalogRejectsMergeKeysAndCustomTags(t *testing.T) {
	data, err := distribution.EmbeddedFS().ReadFile(".mss/admin-presentation-catalog.yaml")
	if err != nil {
		t.Fatalf("read embedded catalog: %v", err)
	}
	tests := []struct {
		name string
		old  string
		new  string
		path string
		code string
	}{
		{name: "root merge", old: "apiVersion: mss.io/v1alpha1\n", new: "<<: {}\napiVersion: mss.io/v1alpha1\n", path: "<<", code: "yaml-merge-key-forbidden"},
		{
			name: "component merge",
			old:  "    - id: boolean\n      valueTypes: [boolean]\n      formats: [plain]\n      surfaces: [list, detail]\n      readOnly: any\n",
			new:  "    - <<: { readOnly: any }\n      id: boolean\n      valueTypes: [boolean]\n      formats: [plain]\n      surfaces: [list, detail]\n",
			path: "spec.components[0].<<",
			code: "yaml-merge-key-forbidden",
		},
		{name: "root custom mapping tag", old: "apiVersion: mss.io/v1alpha1\n", new: "!evil\napiVersion: mss.io/v1alpha1\n", path: "", code: "yaml-type-mismatch"},
		{name: "components custom sequence tag", old: "  components:\n", new: "  components: !evil\n", path: "spec.components", code: "yaml-type-mismatch"},
		{name: "root custom key tag", old: "apiVersion: mss.io/v1alpha1\n", new: "!evil apiVersion: mss.io/v1alpha1\n", path: "apiVersion", code: "yaml-key-type-mismatch"},
		{name: "nested custom key tag", old: "  name: admin-presentation-v2\n", new: "  !evil name: admin-presentation-v2\n", path: "metadata.name", code: "yaml-key-type-mismatch"},
		{name: "sequence item custom key tag", old: "    - id: boolean\n", new: "    - !evil id: boolean\n", path: "spec.components[0].id", code: "yaml-key-type-mismatch"},
		{
			name: "boolean property custom key tag",
			old:  "    - id: create\n      apiOperation: create\n      permissionAction: create\n      placements: [toolbar]\n      destructive: false\n",
			new:  "    - id: create\n      apiOperation: create\n      permissionAction: create\n      placements: [toolbar]\n      !evil destructive: false\n",
			path: "spec.actions[0].destructive",
			code: "yaml-key-type-mismatch",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			mutated := replacePresentationTestYAML(t, data, test.old, test.new)
			_, err := ParsePresentationCatalog(mutated, "catalog.yaml")
			if !validationErrorHasIssue(err, test.path, test.code) {
				t.Fatalf("ParsePresentationCatalog() error = %v, want %s issue at %s", err, test.code, test.path)
			}
		})
	}
}

func walkClosedObjectSchemas(t *testing.T, value any, path string) {
	t.Helper()
	switch current := value.(type) {
	case map[string]any:
		if current["type"] == "object" && current["additionalProperties"] != false {
			t.Errorf("object schema %s is not closed", path)
		}
		for key, child := range current {
			walkClosedObjectSchemas(t, child, path+"."+key)
		}
	case []any:
		for index, child := range current {
			walkClosedObjectSchemas(t, child, path+"["+strconv.Itoa(index)+"]")
		}
	}
}
