package spec

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
)

func TestPresentationCanonicalVersionTwoCrossLanguageGolden(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "presentation-canonical-v2.json"))
	if err != nil {
		t.Fatalf("read canonical golden: %v", err)
	}
	var fixture struct {
		Value     any    `json:"value"`
		Canonical string `json:"canonical"`
		SHA256    string `json:"sha256"`
	}
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatalf("parse canonical golden: %v", err)
	}
	var canonical bytes.Buffer
	if err := writePresentationCanonicalJSON(&canonical, fixture.Value); err != nil {
		t.Fatalf("write canonical golden: %v", err)
	}
	if canonical.String() != fixture.Canonical {
		t.Fatalf("canonical bytes = %q, want %q", canonical.String(), fixture.Canonical)
	}
	digest := sha256.Sum256(canonical.Bytes())
	if got := fmt.Sprintf("%x", digest); got != fixture.SHA256 {
		t.Fatalf("canonical sha256 = %s, want %s", got, fixture.SHA256)
	}
}

func TestPresentationPatternVersionTwoCrossLanguageSemantics(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "presentation-pattern-semantics-v2.json"))
	if err != nil {
		t.Fatalf("read pattern semantics fixture: %v", err)
	}
	var fixture struct {
		Accepted []struct {
			Pattern string   `json:"pattern"`
			Matches []string `json:"matches"`
			Rejects []string `json:"rejects"`
		} `json:"accepted"`
		Rejected []string `json:"rejected"`
	}
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatalf("parse pattern semantics fixture: %v", err)
	}
	for _, test := range fixture.Accepted {
		if !isPortablePresentationPattern(test.Pattern) {
			t.Errorf("portable pattern %q was rejected", test.Pattern)
			continue
		}
		compiled, err := regexp.Compile(test.Pattern)
		if err != nil {
			t.Errorf("compile portable pattern %q: %v", test.Pattern, err)
			continue
		}
		for _, value := range test.Matches {
			if !compiled.MatchString(value) {
				t.Errorf("pattern %q did not match %q", test.Pattern, value)
			}
		}
		for _, value := range test.Rejects {
			if compiled.MatchString(value) {
				t.Errorf("pattern %q unexpectedly matched %q", test.Pattern, value)
			}
		}
	}
	for _, pattern := range fixture.Rejected {
		if isPortablePresentationPattern(pattern) {
			t.Errorf("non-portable pattern %q was accepted", pattern)
		}
	}
}

func TestAdminModulePresentationOmissionIsBackwardCompatible(t *testing.T) {
	module := validModule()
	manifest, err := module.NormalizePresentation()
	if err != nil {
		t.Fatalf("NormalizePresentation() error = %v", err)
	}
	if manifest != nil {
		t.Fatalf("omitted presentation normalized to %#v, want nil", manifest)
	}
	data, err := module.YAML()
	if err != nil {
		t.Fatalf("YAML() error = %v", err)
	}
	if bytes.Contains(data, []byte("presentation:")) {
		t.Fatalf("omitted presentation was serialized:\n%s", data)
	}
}

func TestAdminModulePresentationNormalizesVersionTwoCompleteManifest(t *testing.T) {
	module := validPresentationModule()
	if issues := module.Validate(); len(issues) != 0 {
		t.Fatalf("Validate() issues = %#v", issues)
	}
	manifest, err := module.NormalizePresentation()
	if err != nil {
		t.Fatalf("NormalizePresentation() error = %v", err)
	}
	if manifest.PageKey != "purchase-order.list" || manifest.DefinitionVersion != "2" {
		t.Fatalf("manifest identity = %#v", manifest)
	}
	if len(manifest.DefinitionHash) != len("sha256:")+64 || !strings.HasPrefix(manifest.DefinitionHash, "sha256:") {
		t.Fatalf("definition hash = %q", manifest.DefinitionHash)
	}
	if len(manifest.DataSources) != 1 {
		t.Fatalf("data sources = %#v", manifest.DataSources)
	}
	dataSource := manifest.DataSources[0]
	if dataSource.ID != "purchase-order.list" || !slices.Equal(dataSource.PageSizeOptions, []int{20, 50, 100}) || dataSource.MaxPageSize != 100 || dataSource.MaxSortFields != 1 {
		t.Fatalf("normalized data source = %#v", dataSource)
	}
	for _, id := range []string{"id", "createdAt", "updatedAt"} {
		field := normalizedField(t, manifest, id)
		if !field.ReadOnly || !slices.Contains(field.Surfaces, "detail") {
			t.Fatalf("derived field %s = %#v", id, field)
		}
	}
	if got := normalizedField(t, manifest, "orderCode"); got.Format != "plain" || got.Nullable || got.ReadOnly || got.Validation.MinLength == nil || *got.Validation.MinLength != 2 {
		t.Fatalf("orderCode semantic facts = %#v", got)
	}
	canonical, err := manifest.CanonicalJSON()
	if err != nil {
		t.Fatalf("CanonicalJSON() error = %v", err)
	}
	if bytes.Contains(canonical, []byte("definitionHash")) || bytes.Contains(canonical, []byte("moduleName")) {
		t.Fatalf("canonical hash input contains excluded fields: %s", canonical)
	}
	if !bytes.HasPrefix(canonical, []byte(`{"actions":`)) {
		t.Fatalf("canonical JSON does not use fixed ASCII property ordering: %s", canonical)
	}
	if bytes.Contains(canonical, []byte(`\u003c`)) || !bytes.Contains(canonical, []byte("Purchase orders & <Orders>")) {
		t.Fatalf("canonical JSON escaped UTF-8 or HTML-only characters: %s", canonical)
	}
	separatorModule := validPresentationModule()
	separatorModule.Spec.Presentation.Title.EnUS = "Line\u2028Paragraph\u2029Literal\\u2028"
	separatorManifest, err := separatorModule.NormalizePresentation()
	if err != nil {
		t.Fatalf("NormalizePresentation(separator title) error = %v", err)
	}
	separatorCanonical, err := separatorManifest.CanonicalJSON()
	if err != nil {
		t.Fatalf("CanonicalJSON(separator title) error = %v", err)
	}
	expectedSeparator := []byte(`"en-US":"Line` + "\u2028" + `Paragraph` + "\u2029" + `Literal\\u2028"`)
	if !bytes.Contains(separatorCanonical, expectedSeparator) {
		t.Fatalf("canonical JSON does not match JSON.stringify separator escaping: %s", separatorCanonical)
	}

	changed := validPresentationModule()
	changed.Spec.Presentation.Title.EnUS = "Changed title"
	changedManifest, err := changed.NormalizePresentation()
	if err != nil {
		t.Fatalf("NormalizePresentation(changed) error = %v", err)
	}
	if changedManifest.DefinitionHash == manifest.DefinitionHash {
		t.Fatal("semantic title change did not change the version 2 hash")
	}

	validationChanged := validPresentationModule()
	validationChanged.Spec.Entity.Fields[0].Validation.MaxLength = intPointer(99)
	validationManifest, err := validationChanged.NormalizePresentation()
	if err != nil {
		t.Fatalf("NormalizePresentation(validation changed) error = %v", err)
	}
	if validationManifest.DefinitionHash == manifest.DefinitionHash {
		t.Fatal("field validation change did not change the version 2 hash")
	}

	limitCatalog, err := DefaultPresentationCatalog()
	if err != nil {
		t.Fatalf("DefaultPresentationCatalog() error = %v", err)
	}
	limitCatalog.Spec.DataSources[0].MaxPageSize = 150
	limitManifest, err := validPresentationModule().NormalizePresentationWithCatalog(limitCatalog)
	if err != nil {
		t.Fatalf("NormalizePresentationWithCatalog(limit changed) error = %v", err)
	}
	if limitManifest.DefinitionHash == manifest.DefinitionHash {
		t.Fatal("compiled data-source limit change did not change the version 2 hash")
	}
}

func TestSupplierPresentationSourceNormalizesProductionEquivalentVersionTwoDefaults(t *testing.T) {
	root := findRepositoryRoot(t)
	module, err := LoadModule(filepath.Join(root, ".mss", "modules", "example-supplier.yaml"))
	if err != nil {
		t.Fatalf("LoadModule(Supplier) error = %v", err)
	}
	manifest, err := module.NormalizePresentation()
	if err != nil {
		t.Fatalf("NormalizePresentation(Supplier) error = %v", err)
	}
	if manifest == nil || manifest.PageKey != "supplier.list" || manifest.DefinitionVersion != "2" {
		t.Fatalf("Supplier manifest identity = %#v", manifest)
	}
	defaults := manifest.DefaultPresentation
	if defaults.List.Density != "large" || defaults.List.PageSize != 20 || len(defaults.List.DefaultSort) != 0 {
		t.Fatalf("Supplier list defaults = %#v", defaults.List)
	}
	searchFields := make([]string, 0, len(defaults.Search.Fields))
	for _, field := range defaults.Search.Fields {
		searchFields = append(searchFields, field.Field)
	}
	if !slices.Equal(searchFields, []string{"code", "country", "creditLevel", "enabled"}) || defaults.Search.CollapsedByDefault {
		t.Fatalf("Supplier search defaults = %#v", defaults.Search)
	}
	if defaults.Form.Columns != 1 || defaults.Detail.Columns != 1 {
		t.Fatalf("Supplier form/detail columns = %d/%d", defaults.Form.Columns, defaults.Detail.Columns)
	}
	detailFields := make([]string, 0, len(defaults.Detail.Fields))
	for _, field := range defaults.Detail.Fields {
		detailFields = append(detailFields, field.Field)
	}
	for _, derived := range []string{"id", "createdAt", "updatedAt"} {
		if !slices.Contains(detailFields, derived) {
			t.Fatalf("Supplier detail fields = %#v, missing %q", detailFields, derived)
		}
	}
	if len(defaults.Actions) < 2 || defaults.Actions[0].Action != "supplier.export" || defaults.Actions[1].Action != "supplier.create" {
		t.Fatalf("Supplier toolbar action order = %#v", defaults.Actions)
	}
	if !slices.Contains(normalizedField(t, manifest, "enabled").Components, "boolean-filter") {
		t.Fatalf("Supplier enabled components = %#v", normalizedField(t, manifest, "enabled").Components)
	}
	if got := normalizedField(t, manifest, "contactEmail").Label.EnUS; got != "Contact Email" {
		t.Fatalf("Supplier contactEmail English label = %q", got)
	}
	for _, action := range manifest.Actions {
		if !slices.Contains(action.RequiredPermissions, "/suppliers") {
			t.Fatalf("Supplier action %q permissions = %#v, missing module route", action.ID, action.RequiredPermissions)
		}
	}
	if got := defaults.Actions[0].Label.EnUS; got != "Export" {
		t.Fatalf("Supplier export English label = %q", got)
	}
}

func TestParseAdminModulePresentationRequiresExplicitLegalZeroValues(t *testing.T) {
	root := findRepositoryRoot(t)
	data, err := os.ReadFile(filepath.Join(root, ".mss", "modules", "example-supplier.yaml"))
	if err != nil {
		t.Fatalf("read Supplier module: %v", err)
	}
	tests := []struct {
		name string
		old  string
		new  string
		path string
	}{
		{
			name: "collapsedByDefault",
			old:  "      collapsedByDefault: false\n",
			new:  "",
			path: "spec.presentation.search.collapsedByDefault",
		},
		{
			name: "field order",
			old:  "        - { field: code, component: text, order: 10, width: 180 }\n",
			new:  "        - { field: code, component: text, width: 180 }\n",
			path: "spec.presentation.list.fields[0].order",
		},
		{
			name: "action order",
			old:  "      - action: export\n        label: { zh-CN: 导出, en-US: Export }\n        placement: toolbar\n        order: 10\n",
			new:  "      - action: export\n        label: { zh-CN: 导出, en-US: Export }\n        placement: toolbar\n",
			path: "spec.presentation.actions[0].order",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			mutated := replacePresentationTestYAML(t, data, test.old, test.new)
			_, err := ParseModule(mutated, "supplier.yaml")
			if !validationErrorHasIssue(err, test.path, "required") {
				t.Fatalf("ParseModule() error = %v, want required issue at %s", err, test.path)
			}
		})
	}

	explicitZero := replacePresentationTestYAML(
		t,
		data,
		"        - { field: code, component: text, order: 10, width: 180 }\n",
		"        - { field: code, component: text, order: 0, width: 180 }\n",
	)
	explicitZero = replacePresentationTestYAML(
		t,
		explicitZero,
		"      - action: export\n        label: { zh-CN: 导出, en-US: Export }\n        placement: toolbar\n        order: 10\n",
		"      - action: export\n        label: { zh-CN: 导出, en-US: Export }\n        placement: toolbar\n        order: 0\n",
	)
	if _, err := ParseModule(explicitZero, "supplier.yaml"); err != nil {
		t.Fatalf("ParseModule(explicit false/zero) error = %v", err)
	}
}

func TestParseAdminModulePresentationRejectsExplicitEmptyLocalizedText(t *testing.T) {
	root := findRepositoryRoot(t)
	data, err := os.ReadFile(filepath.Join(root, ".mss", "modules", "example-supplier.yaml"))
	if err != nil {
		t.Fatalf("read Supplier module: %v", err)
	}
	tests := []struct {
		name string
		old  string
		new  string
		path string
	}{
		{
			name: "title",
			old:  "      en-US: Suppliers\n",
			new:  "      en-US: \"\"\n",
			path: "spec.presentation.title.en-US",
		},
		{
			name: "field label",
			old:  "        - { field: code, component: text, order: 10, width: 180 }\n",
			new:  "        - { field: code, component: text, order: 10, width: 180, label: { zh-CN: 编码, en-US: \"\" } }\n",
			path: "spec.presentation.list.fields[0].label.en-US",
		},
		{
			name: "action label",
			old:  "        label: { zh-CN: 导出, en-US: Export }\n",
			new:  "        label: { zh-CN: 导出, en-US: \"\" }\n",
			path: "spec.presentation.actions[0].label.en-US",
		},
		{
			name: "placeholder",
			old:  "        - { field: code, component: text, order: 10, width: 180 }\n",
			new:  "        - { field: code, component: text, order: 10, width: 180, placeholder: { zh-CN: 提示, en-US: \"\" } }\n",
			path: "spec.presentation.list.fields[0].placeholder.en-US",
		},
		{
			name: "help",
			old:  "        - { field: code, component: text, order: 10, width: 180 }\n",
			new:  "        - { field: code, component: text, order: 10, width: 180, help: { zh-CN: 帮助, en-US: \"\" } }\n",
			path: "spec.presentation.list.fields[0].help.en-US",
		},
		{
			name: "confirm",
			old:  "          en-US: Delete this supplier?\n",
			new:  "          en-US: \"\"\n",
			path: "spec.presentation.actions[4].confirm.en-US",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			mutated := replacePresentationTestYAML(t, data, test.old, test.new)
			_, err := ParseModule(mutated, "supplier.yaml")
			if !validationErrorHasIssue(err, test.path, "localized-text-empty") {
				t.Fatalf("ParseModule() error = %v, want localized-text-empty issue at %s", err, test.path)
			}
		})
	}

	omittedLocale := replacePresentationTestYAML(
		t,
		data,
		"        label: { zh-CN: 导出, en-US: Export }\n",
		"        label: { zh-CN: 导出 }\n",
	)
	if _, err := ParseModule(omittedLocale, "supplier.yaml"); err != nil {
		t.Fatalf("ParseModule(omitted optional label locale) error = %v", err)
	}

	completeLocaleTests := []struct {
		name string
		old  string
		new  string
		path string
	}{
		{
			name: "title omission",
			old:  "      en-US: Suppliers\n",
			new:  "",
			path: "spec.presentation.title.en-US",
		},
		{
			name: "placeholder omission",
			old:  "        - { field: code, component: text, order: 10, width: 180 }\n",
			new:  "        - { field: code, component: text, order: 10, width: 180, placeholder: { zh-CN: 提示 } }\n",
			path: "spec.presentation.list.fields[0].placeholder.en-US",
		},
		{
			name: "help omission",
			old:  "        - { field: code, component: text, order: 10, width: 180 }\n",
			new:  "        - { field: code, component: text, order: 10, width: 180, help: { zh-CN: 帮助 } }\n",
			path: "spec.presentation.list.fields[0].help.en-US",
		},
		{
			name: "confirm omission",
			old:  "          en-US: Delete this supplier?\n",
			new:  "",
			path: "spec.presentation.actions[4].confirm.en-US",
		},
	}
	for _, test := range completeLocaleTests {
		t.Run(test.name, func(t *testing.T) {
			mutated := replacePresentationTestYAML(t, data, test.old, test.new)
			_, err := ParseModule(mutated, "supplier.yaml")
			if !validationErrorHasIssue(err, test.path, "required") {
				t.Fatalf("ParseModule() error = %v, want required issue at %s", err, test.path)
			}
		})
	}
}

func TestAdminModulePresentationLocalizedSchemaSeparatesOverridesFromCompleteText(t *testing.T) {
	root := findRepositoryRoot(t)
	data, err := os.ReadFile(filepath.Join(root, ".mss", "schemas", "admin-module.schema.json"))
	if err != nil {
		t.Fatalf("read AdminModule schema: %v", err)
	}
	var schema struct {
		Definitions map[string]map[string]any `json:"$defs"`
	}
	if err := json.Unmarshal(data, &schema); err != nil {
		t.Fatalf("parse AdminModule schema: %v", err)
	}
	complete := schema.Definitions["presentationLocalizedTextComplete"]
	required, _ := complete["required"].([]any)
	if !slices.Equal(required, []any{"zh-CN", "en-US"}) {
		t.Fatalf("complete localized required keys = %#v", required)
	}
	override := schema.Definitions["presentationLocalizedTextOverride"]
	if override["minProperties"] != float64(1) {
		t.Fatalf("localized override minProperties = %#v", override["minProperties"])
	}
	encoded := string(data)
	for _, reference := range []string{
		`"title": { "$ref": "#/$defs/presentationLocalizedTextComplete" }`,
		`"placeholder": { "$ref": "#/$defs/presentationLocalizedTextComplete" }`,
		`"help": { "$ref": "#/$defs/presentationLocalizedTextComplete" }`,
		`"confirm": { "$ref": "#/$defs/presentationLocalizedTextComplete" }`,
		`"label": { "$ref": "#/$defs/presentationLocalizedTextOverride" }`,
	} {
		if !strings.Contains(encoded, reference) {
			t.Errorf("AdminModule schema omitted localized reference %s", reference)
		}
	}
}

func TestAdminModulePresentationPartialLabelYAMLRoundTripPreservesOmission(t *testing.T) {
	root := findRepositoryRoot(t)
	data, err := os.ReadFile(filepath.Join(root, ".mss", "modules", "example-supplier.yaml"))
	if err != nil {
		t.Fatalf("read Supplier module: %v", err)
	}
	partial := replacePresentationTestYAML(
		t,
		data,
		"        - { field: code, component: text, order: 10, width: 180 }\n",
		"        - { field: code, component: text, order: 10, width: 180, label: { zh-CN: 编码 } }\n",
	)
	partial = replacePresentationTestYAML(
		t,
		partial,
		"        label: { zh-CN: 导出, en-US: Export }\n",
		"        label: { zh-CN: 导出 }\n",
	)
	module, err := ParseModule(partial, "supplier.yaml")
	if err != nil {
		t.Fatalf("ParseModule(partial labels) error = %v", err)
	}
	serialized, err := module.YAML()
	if err != nil {
		t.Fatalf("Module.YAML(partial labels) error = %v", err)
	}
	if bytes.Contains(serialized, []byte(`en-US: ""`)) {
		t.Fatalf("Module.YAML materialized an omitted label locale:\n%s", serialized)
	}
	if _, err := ParseModule(serialized, "supplier-round-trip.yaml"); err != nil {
		t.Fatalf("ParseModule(round-trip partial labels) error = %v\n%s", err, serialized)
	}
	jsonText, err := json.Marshal(PresentationLocalizedText{})
	if err != nil {
		t.Fatalf("marshal localized JSON: %v", err)
	}
	if string(jsonText) != `{"zh-CN":"","en-US":""}` {
		t.Fatalf("localized JSON shape changed with YAML omission support: %s", jsonText)
	}
}

func TestParseAdminModulePresentationRejectsCoercedLocalizedScalars(t *testing.T) {
	root := findRepositoryRoot(t)
	data, err := os.ReadFile(filepath.Join(root, ".mss", "modules", "example-supplier.yaml"))
	if err != nil {
		t.Fatalf("read Supplier module: %v", err)
	}
	tests := []struct {
		name string
		old  string
		new  string
		path string
	}{
		{
			name: "title",
			old:  "      en-US: Suppliers\n",
			new:  "      en-US: 123\n",
			path: "spec.presentation.title.en-US",
		},
		{
			name: "field label",
			old:  "        - { field: code, component: text, order: 10, width: 180 }\n",
			new:  "        - { field: code, component: text, order: 10, width: 180, label: { zh-CN: 编码, en-US: 123 } }\n",
			path: "spec.presentation.list.fields[0].label.en-US",
		},
		{
			name: "action label",
			old:  "        label: { zh-CN: 导出, en-US: Export }\n",
			new:  "        label: { zh-CN: 导出, en-US: 123 }\n",
			path: "spec.presentation.actions[0].label.en-US",
		},
		{
			name: "placeholder",
			old:  "        - { field: code, component: text, order: 10, width: 180 }\n",
			new:  "        - { field: code, component: text, order: 10, width: 180, placeholder: { zh-CN: 提示, en-US: 123 } }\n",
			path: "spec.presentation.list.fields[0].placeholder.en-US",
		},
		{
			name: "help",
			old:  "        - { field: code, component: text, order: 10, width: 180 }\n",
			new:  "        - { field: code, component: text, order: 10, width: 180, help: { zh-CN: 帮助, en-US: 123 } }\n",
			path: "spec.presentation.list.fields[0].help.en-US",
		},
		{
			name: "confirm",
			old:  "          en-US: Delete this supplier?\n",
			new:  "          en-US: 123\n",
			path: "spec.presentation.actions[4].confirm.en-US",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			mutated := replacePresentationTestYAML(t, data, test.old, test.new)
			_, err := ParseModule(mutated, "supplier.yaml")
			if !validationErrorHasIssue(err, test.path, "yaml-type-mismatch") {
				t.Fatalf("ParseModule() error = %v, want yaml-type-mismatch issue at %s", err, test.path)
			}
		})
	}

	aliased := replacePresentationTestYAML(t, data, "      pageSize: 20\n", "      pageSize: &localizedNumber 20\n")
	aliased = replacePresentationTestYAML(
		t,
		aliased,
		"        - { field: code, component: text, order: 10, width: 180 }\n",
		"        - { field: code, component: text, order: 10, width: 180, label: { zh-CN: 编码, en-US: *localizedNumber } }\n",
	)
	_, err = ParseModule(aliased, "supplier.yaml")
	if !validationErrorHasIssue(err, "spec.presentation.list.fields[0].label.en-US", "yaml-type-mismatch") {
		t.Fatalf("ParseModule(aliased numeric locale) error = %v, want yaml-type-mismatch", err)
	}
}

func TestParseAdminModulePresentationRejectsSchemaScalarCoercion(t *testing.T) {
	root := findRepositoryRoot(t)
	data, err := os.ReadFile(filepath.Join(root, ".mss", "modules", "example-supplier.yaml"))
	if err != nil {
		t.Fatalf("read Supplier module: %v", err)
	}
	tests := []struct {
		name string
		old  string
		new  string
		path string
	}{
		{name: "page key boolean", old: "    pageKey: supplier.list\n", new: "    pageKey: true\n", path: "spec.presentation.pageKey"},
		{name: "definition version integer", old: "    definitionVersion: \"2\"\n", new: "    definitionVersion: 2\n", path: "spec.presentation.definitionVersion"},
		{name: "data source integer", old: "    dataSource: list\n", new: "    dataSource: 1\n", path: "spec.presentation.dataSource"},
		{name: "density boolean", old: "      density: large\n", new: "      density: true\n", path: "spec.presentation.list.density"},
		{name: "page size float", old: "      pageSize: 20\n", new: "      pageSize: 20.0\n", path: "spec.presentation.list.pageSize"},
		{name: "collapsed string", old: "      collapsedByDefault: false\n", new: "      collapsedByDefault: yes\n", path: "spec.presentation.search.collapsedByDefault"},
		{
			name: "field id boolean",
			old:  "        - { field: code, component: text, order: 10, width: 180 }\n",
			new:  "        - { field: true, component: text, order: 10, width: 180 }\n",
			path: "spec.presentation.list.fields[0].field",
		},
		{
			name: "component integer",
			old:  "        - { field: code, component: text, order: 10, width: 180 }\n",
			new:  "        - { field: code, component: 123, order: 10, width: 180 }\n",
			path: "spec.presentation.list.fields[0].component",
		},
		{
			name: "allowed component integer",
			old:  "allowedComponents: [copyable-code]",
			new:  "allowedComponents: [123]",
			path: "spec.presentation.detail.fields[0].allowedComponents[0]",
		},
		{
			name: "field order float",
			old:  "        - { field: code, component: text, order: 10, width: 180 }\n",
			new:  "        - { field: code, component: text, order: 10.0, width: 180 }\n",
			path: "spec.presentation.list.fields[0].order",
		},
		{
			name: "field hidden string",
			old:  "        - { field: code, component: text, order: 10, width: 180 }\n",
			new:  "        - { field: code, component: text, order: 10, width: 180, hidden: yes }\n",
			path: "spec.presentation.list.fields[0].hidden",
		},
		{
			name: "field width float",
			old:  "        - { field: code, component: text, order: 10, width: 180 }\n",
			new:  "        - { field: code, component: text, order: 10, width: 180.0 }\n",
			path: "spec.presentation.list.fields[0].width",
		},
		{
			name: "form columns float",
			old:  "    form:\n      columns: 1\n",
			new:  "    form:\n      columns: 1.0\n",
			path: "spec.presentation.form.columns",
		},
		{
			name: "sort field integer",
			old:  "      defaultSort: []\n",
			new:  "      defaultSort: [{ field: 123, direction: asc }]\n",
			path: "spec.presentation.list.defaultSort[0].field",
		},
		{
			name: "sort direction integer",
			old:  "      defaultSort: []\n",
			new:  "      defaultSort: [{ field: code, direction: 123 }]\n",
			path: "spec.presentation.list.defaultSort[0].direction",
		},
		{
			name: "action id boolean",
			old:  "      - action: export\n",
			new:  "      - action: true\n",
			path: "spec.presentation.actions[0].action",
		},
		{
			name: "action placement boolean",
			old:  "      - action: export\n        label: { zh-CN: 导出, en-US: Export }\n        placement: toolbar\n",
			new:  "      - action: export\n        label: { zh-CN: 导出, en-US: Export }\n        placement: true\n",
			path: "spec.presentation.actions[0].placement",
		},
		{
			name: "action order float",
			old:  "      - action: export\n        label: { zh-CN: 导出, en-US: Export }\n        placement: toolbar\n        order: 10\n",
			new:  "      - action: export\n        label: { zh-CN: 导出, en-US: Export }\n        placement: toolbar\n        order: 10.0\n",
			path: "spec.presentation.actions[0].order",
		},
		{
			name: "action hidden string",
			old:  "      - action: export\n        label: { zh-CN: 导出, en-US: Export }\n        placement: toolbar\n        order: 10\n",
			new:  "      - action: export\n        label: { zh-CN: 导出, en-US: Export }\n        placement: toolbar\n        order: 10\n        hidden: yes\n",
			path: "spec.presentation.actions[0].hidden",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			mutated := replacePresentationTestYAML(t, data, test.old, test.new)
			_, err := ParseModule(mutated, "supplier.yaml")
			if !validationErrorHasIssue(err, test.path, "yaml-type-mismatch") {
				t.Fatalf("ParseModule() error = %v, want yaml-type-mismatch issue at %s", err, test.path)
			}
		})
	}
}

func TestParseAdminModulePresentationRejectsTokenWhitespace(t *testing.T) {
	root := findRepositoryRoot(t)
	data, err := os.ReadFile(filepath.Join(root, ".mss", "modules", "example-supplier.yaml"))
	if err != nil {
		t.Fatalf("read Supplier module: %v", err)
	}
	tests := []struct {
		name string
		old  string
		new  string
		path string
	}{
		{name: "page key", old: "    pageKey: supplier.list\n", new: "    pageKey: \" supplier.list \"\n", path: "spec.presentation.pageKey"},
		{name: "definition version", old: "    definitionVersion: \"2\"\n", new: "    definitionVersion: \" 2 \"\n", path: "spec.presentation.definitionVersion"},
		{name: "data source", old: "    dataSource: list\n", new: "    dataSource: \" list \"\n", path: "spec.presentation.dataSource"},
		{name: "density", old: "      density: large\n", new: "      density: \" large \"\n", path: "spec.presentation.list.density"},
		{
			name: "field",
			old:  "        - { field: code, component: text, order: 10, width: 180 }\n",
			new:  "        - { field: \" code \", component: text, order: 10, width: 180 }\n",
			path: "spec.presentation.list.fields[0].field",
		},
		{
			name: "component",
			old:  "        - { field: code, component: text, order: 10, width: 180 }\n",
			new:  "        - { field: code, component: \" text \", order: 10, width: 180 }\n",
			path: "spec.presentation.list.fields[0].component",
		},
		{
			name: "allowed component",
			old:  "allowedComponents: [copyable-code]",
			new:  "allowedComponents: [\" copyable-code \"]",
			path: "spec.presentation.detail.fields[0].allowedComponents[0]",
		},
		{
			name: "sort direction",
			old:  "      defaultSort: []\n",
			new:  "      defaultSort: [{ field: code, direction: \" asc \" }]\n",
			path: "spec.presentation.list.defaultSort[0].direction",
		},
		{
			name: "action",
			old:  "      - action: export\n",
			new:  "      - action: \" export \"\n",
			path: "spec.presentation.actions[0].action",
		},
		{
			name: "placement",
			old:  "      - action: export\n        label: { zh-CN: 导出, en-US: Export }\n        placement: toolbar\n",
			new:  "      - action: export\n        label: { zh-CN: 导出, en-US: Export }\n        placement: \" toolbar \"\n",
			path: "spec.presentation.actions[0].placement",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			mutated := replacePresentationTestYAML(t, data, test.old, test.new)
			_, err := ParseModule(mutated, "supplier.yaml")
			if !validationErrorHasIssue(err, test.path, "yaml-token-whitespace") {
				t.Fatalf("ParseModule() error = %v, want yaml-token-whitespace issue at %s", err, test.path)
			}
		})
	}
}

func TestParseAdminModulePresentationRejectsMergeKeysAndCustomTags(t *testing.T) {
	root := findRepositoryRoot(t)
	data, err := os.ReadFile(filepath.Join(root, ".mss", "modules", "example-supplier.yaml"))
	if err != nil {
		t.Fatalf("read Supplier module: %v", err)
	}
	tests := []struct {
		name string
		old  string
		new  string
		path string
		code string
	}{
		{name: "spec merge", old: "spec:\n", new: "spec:\n  <<: {}\n", path: "spec.<<", code: "yaml-merge-key-forbidden"},
		{name: "document root custom mapping tag", old: "apiVersion:", new: "!evil\napiVersion:", path: "", code: "yaml-type-mismatch"},
		{name: "spec custom mapping tag", old: "spec:\n", new: "spec: !evil\n", path: "spec", code: "yaml-type-mismatch"},
		{name: "root custom key tag", old: "apiVersion:", new: "!evil apiVersion:", path: "apiVersion", code: "yaml-key-type-mismatch"},
		{name: "nested custom key tag", old: "  presentation:\n", new: "  !evil presentation:\n", path: "spec.presentation", code: "yaml-key-type-mismatch"},
		{
			name: "field merge",
			old:  "        - { field: code, component: text, order: 10, width: 180 }\n",
			new:  "        - { <<: { width: 180 }, field: code, component: text, order: 10 }\n",
			path: "spec.presentation.list.fields[0].<<",
			code: "yaml-merge-key-forbidden",
		},
		{
			name: "sequence item custom key tag",
			old:  "        - { field: code, component: text, order: 10, width: 180 }\n",
			new:  "        - { !evil field: code, component: text, order: 10, width: 180 }\n",
			path: "spec.presentation.list.fields[0].field",
			code: "yaml-key-type-mismatch",
		},
		{
			name: "locale custom key tag",
			old:  "        label: { zh-CN: 导出, en-US: Export }\n",
			new:  "        label: { zh-CN: 导出, !evil en-US: Export }\n",
			path: "spec.presentation.actions[0].label.en-US",
			code: "yaml-key-type-mismatch",
		},
		{
			name: "boolean property custom key tag",
			old:  "        - { field: code, component: text, order: 10, width: 180 }\n",
			new:  "        - { field: code, component: text, order: 10, width: 180, !evil hidden: false }\n",
			path: "spec.presentation.list.fields[0].hidden",
			code: "yaml-key-type-mismatch",
		},
		{name: "presentation custom mapping tag", old: "  presentation:\n", new: "  presentation: !evil\n", path: "spec.presentation", code: "yaml-type-mismatch"},
		{name: "actions custom sequence tag", old: "    actions:\n", new: "    actions: !evil\n", path: "spec.presentation.actions", code: "yaml-type-mismatch"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			mutated := replacePresentationTestYAML(t, data, test.old, test.new)
			_, err := ParseModule(mutated, "supplier.yaml")
			if !validationErrorHasIssue(err, test.path, test.code) {
				t.Fatalf("ParseModule() error = %v, want %s issue at %s", err, test.code, test.path)
			}
		})
	}
}

func TestParseAdminModulePresentationRejectsExplicitNull(t *testing.T) {
	root := findRepositoryRoot(t)
	data, err := os.ReadFile(filepath.Join(root, ".mss", "modules", "example-supplier.yaml"))
	if err != nil {
		t.Fatalf("read Supplier module: %v", err)
	}
	tests := []struct {
		name string
		old  string
		new  string
		path string
	}{
		{
			name: "field label",
			old:  "        - { field: code, component: text, order: 10, width: 180 }\n",
			new:  "        - { field: code, component: text, order: 10, width: 180, label: null }\n",
			path: "spec.presentation.list.fields[0].label",
		},
		{
			name: "allowed components",
			old:  "allowedComponents: [copyable-code]",
			new:  "allowedComponents: null",
			path: "spec.presentation.detail.fields[0].allowedComponents",
		},
		{
			name: "hidden",
			old:  "        - { field: code, component: text, order: 10, width: 180 }\n",
			new:  "        - { field: code, component: text, order: 10, width: 180, hidden: null }\n",
			path: "spec.presentation.list.fields[0].hidden",
		},
		{
			name: "width",
			old:  "        - { field: code, component: text, order: 10, width: 180 }\n",
			new:  "        - { field: code, component: text, order: 10, width: null }\n",
			path: "spec.presentation.list.fields[0].width",
		},
		{
			name: "action label",
			old:  "label: { zh-CN: 导出, en-US: Export }",
			new:  "label: null",
			path: "spec.presentation.actions[0].label",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			mutated := replacePresentationTestYAML(t, data, test.old, test.new)
			_, err := ParseModule(mutated, "supplier.yaml")
			if !validationErrorHasIssue(err, test.path, "null-not-allowed") {
				t.Fatalf("ParseModule() error = %v, want null-not-allowed issue at %s", err, test.path)
			}
		})
	}

	text := string(data)
	presentationStart := strings.Index(text, "  presentation:\n")
	presentationEnd := strings.Index(text[presentationStart:], "\n  events:")
	if presentationStart < 0 || presentationEnd < 0 {
		t.Fatal("Supplier fixture is missing the presentation block boundary")
	}
	presentationEnd += presentationStart
	nullPresentation := []byte(text[:presentationStart] + "  presentation: null\n" + text[presentationEnd+1:])
	_, err = ParseModule(nullPresentation, "supplier.yaml")
	if !validationErrorHasIssue(err, "spec.presentation", "null-not-allowed") {
		t.Fatalf("ParseModule(explicit null presentation) error = %v, want null-not-allowed", err)
	}
}

func TestAdminModulePresentationRejectsUnsafeOrIncompatibleSource(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Module)
		code   string
	}{
		{
			name:   "version one",
			mutate: func(module *Module) { module.Spec.Presentation.DefinitionVersion = "1" },
			code:   "unsupported-definition-version",
		},
		{
			name:   "qualified data source",
			mutate: func(module *Module) { module.Spec.Presentation.DataSource = "purchase-order.list" },
			code:   "invalid-local-data-source-reference",
		},
		{
			name:   "qualified action",
			mutate: func(module *Module) { module.Spec.Presentation.Actions[0].Action = "purchase-order.create" },
			code:   "invalid-local-action-reference",
		},
		{
			name:   "protected page",
			mutate: func(module *Module) { module.Spec.Presentation.PageKey = "presentation.config" },
			code:   "protected-page-key",
		},
		{
			name:   "overlong page key",
			mutate: func(module *Module) { module.Spec.Presentation.PageKey = "a." + strings.Repeat("b", 119) },
			code:   "page-key-too-long",
		},
		{
			name:   "unknown component",
			mutate: func(module *Module) { module.Spec.Presentation.List.Fields[0].Component = "remote-widget" },
			code:   "unknown-component",
		},
		{
			name:   "component surface",
			mutate: func(module *Module) { module.Spec.Presentation.List.Fields[0].Component = "input" },
			code:   "component-incompatible",
		},
		{
			name:   "required form hidden",
			mutate: func(module *Module) { module.Spec.Presentation.Form.Fields[0].Hidden = true },
			code:   "required-form-field-hidden",
		},
		{
			name:   "width outside profile contract",
			mutate: func(module *Module) { module.Spec.Presentation.List.Fields[0].Width = intPointer(50) },
			code:   "width-out-of-range",
		},
		{
			name:   "precision outside capability contract",
			mutate: func(module *Module) { module.Spec.Entity.Fields[0].Validation.Precision = intPointer(39) },
			code:   "presentation-precision-out-of-range",
		},
		{
			name:   "non portable regular expression",
			mutate: func(module *Module) { module.Spec.Entity.Fields[0].Validation.Pattern = `(?P<code>[A-Z]+)` },
			code:   "presentation-non-portable-pattern",
		},
		{
			name: "non finite numeric bound",
			mutate: func(module *Module) {
				value := math.Inf(1)
				module.Spec.Entity.Fields[0].Validation.Maximum = &value
			},
			code: "presentation-non-finite-bound",
		},
		{
			name:   "unsupported page size",
			mutate: func(module *Module) { module.Spec.Presentation.List.PageSize = 25 },
			code:   "unsupported-page-size",
		},
		{
			name: "unsupported sort field",
			mutate: func(module *Module) {
				module.Spec.Entity.Fields[0].Sortable = false
				module.Spec.Presentation.List.DefaultSort = []PresentationSortSource{{Field: "orderCode", Direction: "asc"}}
			},
			code: "unsupported-sort-field",
		},
		{
			name: "too many sorts",
			mutate: func(module *Module) {
				module.Spec.Presentation.List.DefaultSort = []PresentationSortSource{
					{Field: "orderCode", Direction: "asc"},
					{Field: "orderCode", Direction: "desc"},
				}
			},
			code: "too-many-sort-fields",
		},
		{
			name:   "action placement",
			mutate: func(module *Module) { module.Spec.Presentation.Actions[0].Placement = "row" },
			code:   "action-placement-incompatible",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			module := validPresentationModule()
			test.mutate(module)
			_, err := module.NormalizePresentation()
			if err == nil || !validationErrorHasCode(err, test.code) {
				t.Fatalf("NormalizePresentation() error = %v, want code %q", err, test.code)
			}
		})
	}
	if strconv.IntSize == 64 {
		module := validPresentationModule()
		unsafeLength := int(int64(1 << 53))
		module.Spec.Entity.Fields[0].Validation.MaxLength = &unsafeLength
		_, err := module.NormalizePresentation()
		if err == nil || !validationErrorHasCode(err, "presentation-unsafe-integer") {
			t.Fatalf("NormalizePresentation(unsafe length) error = %v, want presentation-unsafe-integer", err)
		}
	}
}

func TestAdminModulePresentationRejectsDuplicatePageKeysAcrossModules(t *testing.T) {
	first := validPresentationModule()
	second := validPresentationModule()
	second.Metadata.Name = "another-module"
	issues := ValidateUniquePresentationPageKeys([]*Module{first, nil, second})
	if len(issues) != 1 || issues[0].Code != "duplicate-page-key" || issues[0].Path != "modules[2].spec.presentation.pageKey" {
		t.Fatalf("ValidateUniquePresentationPageKeys() = %#v", issues)
	}
}

func TestNormalizedPresentationFieldsPreserveCompatibilityFacts(t *testing.T) {
	minimum, maximum := 1.25, 99.5
	field := normalizedEntityPresentationField(FieldSpec{
		Name: "creditLevel", DisplayName: "信用等级", Type: "enum",
		Required: true, Nullable: false, Immutable: true,
		Searchable: true, Sortable: true, Filterable: true,
		Validation: ValidationSpec{
			MinLength: intPointer(1), MaxLength: intPointer(20),
			Minimum: &minimum, Maximum: &maximum, Pattern: "^[a-z]+$",
			Precision: intPointer(4), Scale: intPointer(2),
		},
		EnumValues: []EnumValue{
			{Value: "preferred", Label: "优先", LabelEn: "Preferred", Color: "green"},
			{Value: "normal", Label: "正常", LabelEn: "Normal", Color: "blue"},
		},
	})
	if field.ValueType != "enum" || field.Format != "plain" || !field.Required || field.Nullable || !field.ReadOnly || !field.Searchable || !field.Sortable || !field.Filterable {
		t.Fatalf("normalized compatibility facts = %#v", field)
	}
	if len(field.EnumValues) != 2 || field.EnumValues[0].Label.EnUS != "Preferred" {
		t.Fatalf("normalized enum values = %#v", field.EnumValues)
	}
	if field.Validation.Minimum == nil || *field.Validation.Minimum != "1.25" || field.Validation.Maximum == nil || *field.Validation.Maximum != "99.5" || field.Validation.Pattern != "^[a-z]+$" {
		t.Fatalf("normalized validation = %#v", field.Validation)
	}

	email := normalizedEntityPresentationField(FieldSpec{
		Name: "contactEmail", DisplayName: "联系邮箱", Type: "string", Nullable: true,
		Validation: ValidationSpec{Format: "email", MaxLength: intPointer(254)},
	})
	if email.ValueType != "string" || email.Format != "email" || !email.Nullable || email.Validation.MaxLength == nil || *email.Validation.MaxLength != 254 {
		t.Fatalf("normalized email facts = %#v", email)
	}
}

func TestAdminModulePresentationKeepsSelectedSearchOnlyField(t *testing.T) {
	module := validPresentationModule()
	visible := false
	module.Spec.Entity.Fields = append(module.Spec.Entity.Fields, FieldSpec{
		Name: "externalReference", Column: "external_reference", GoName: "ExternalReference",
		DisplayName: "外部引用", DisplayNameEn: "External Reference", Type: "string",
		Searchable: true, List: &visible, Form: &visible, Detail: &visible,
	})
	module.Spec.Presentation.Search.Fields = append(module.Spec.Presentation.Search.Fields,
		PresentationFieldSource{Field: "externalReference", Component: "input", Order: 20})
	module.Normalize()

	manifest, err := module.NormalizePresentation()
	if err != nil {
		t.Fatalf("NormalizePresentation(search-only field) error = %v", err)
	}
	field := normalizedField(t, manifest, "externalReference")
	if !slices.Equal(field.Surfaces, []string{"search"}) {
		t.Fatalf("search-only field surfaces = %#v", field.Surfaces)
	}
}

func TestAdminModulePresentationSupportsReadOnlyModuleWithoutActions(t *testing.T) {
	module := validPresentationModule()
	module.Spec.API.Operations = []string{"list"}
	permissions := make([]Permission, 0, 1)
	for _, permission := range module.Spec.Permissions {
		if permission.Action == "list" {
			permissions = append(permissions, permission)
		}
	}
	module.Spec.Permissions = permissions
	module.Spec.Presentation.Actions = []PresentationActionSource{}

	manifest, err := module.NormalizePresentation()
	if err != nil {
		t.Fatalf("NormalizePresentation(read-only module) error = %v", err)
	}
	if manifest.Actions == nil || len(manifest.Actions) != 0 {
		t.Fatalf("read-only capability actions = %#v, want a non-nil empty collection", manifest.Actions)
	}
	if manifest.DefaultPresentation.Actions == nil || len(manifest.DefaultPresentation.Actions) != 0 {
		t.Fatalf("read-only default actions = %#v, want a non-nil empty collection", manifest.DefaultPresentation.Actions)
	}
}

func TestAdminModulePresentationRequiresExplicitActionsCollection(t *testing.T) {
	module := validPresentationModule()
	module.Spec.API.Operations = []string{"list"}
	permissions := make([]Permission, 0, 1)
	for _, permission := range module.Spec.Permissions {
		if permission.Action == "list" {
			permissions = append(permissions, permission)
		}
	}
	module.Spec.Permissions = permissions
	module.Spec.Presentation.Actions = nil

	_, err := module.NormalizePresentation()
	validation, ok := err.(*ValidationError)
	if !ok {
		t.Fatalf("NormalizePresentation(omitted actions) error = %v, want ValidationError", err)
	}
	for _, issue := range validation.Issues {
		if issue.Path == "spec.presentation.actions" && issue.Code == "required" {
			return
		}
	}
	t.Fatalf("NormalizePresentation(omitted actions) issues = %#v", validation.Issues)
}

func TestAdminModulePresentationExplicitEmptyAllowedComponentsUsesCatalogDefaults(t *testing.T) {
	module := validPresentationModule()
	module.Spec.Presentation.Detail.Fields[1].AllowedComponents = []string{}

	manifest, err := module.NormalizePresentation()
	if err != nil {
		t.Fatalf("NormalizePresentation(empty allowedComponents) error = %v", err)
	}
	field := normalizedField(t, manifest, "id")
	if len(field.Components) == 0 || !slices.Contains(field.Components, "copyable-code") {
		t.Fatalf("normalized id components = %#v, want compatible catalog defaults", field.Components)
	}
}

func TestAdminModulePresentationRejectsFieldReferencesBeyondSchemaLimit(t *testing.T) {
	module := validPresentationModule()
	module.Spec.Presentation.List.Fields[0].Field = "a" + strings.Repeat("b", 64)

	_, err := module.NormalizePresentation()
	if !validationErrorHasCode(err, "invalid-local-field-reference") {
		t.Fatalf("NormalizePresentation(long field reference) error = %v", err)
	}
}

func TestAdminModulePresentationSchemaIsClosedDataOnlyAndOptional(t *testing.T) {
	root := findRepositoryRoot(t)
	data, err := os.ReadFile(filepath.Join(root, ".mss", "schemas", "admin-module.schema.json"))
	if err != nil {
		t.Fatalf("read AdminModule schema: %v", err)
	}
	var schema map[string]any
	if err := json.Unmarshal(data, &schema); err != nil {
		t.Fatalf("parse AdminModule schema: %v", err)
	}
	definitions := jsonObject(t, schema["$defs"], "$defs")
	specification := jsonObject(t, definitions["spec"], "$defs.spec")
	required := jsonStrings(t, specification["required"], "$defs.spec.required")
	if slices.Contains(required, "presentation") {
		t.Fatal("spec.presentation must remain optional")
	}
	properties := jsonObject(t, specification["properties"], "$defs.spec.properties")
	if _, exists := properties["presentation"]; !exists {
		t.Fatal("AdminModule schema omits spec.presentation")
	}
	presentation := jsonObject(t, definitions["presentation"], "$defs.presentation")
	presentationProperties := jsonObject(t, presentation["properties"], "$defs.presentation.properties")
	actions := jsonObject(t, presentationProperties["actions"], "$defs.presentation.properties.actions")
	if _, exists := actions["minItems"]; exists {
		t.Fatal("complete read-only presentations must allow an explicit empty actions collection")
	}
	presentationField := jsonObject(t, definitions["presentationField"], "$defs.presentationField")
	presentationFieldProperties := jsonObject(t, presentationField["properties"], "$defs.presentationField.properties")
	allowedComponents := jsonObject(t, presentationFieldProperties["allowedComponents"], "$defs.presentationField.properties.allowedComponents")
	if _, exists := allowedComponents["minItems"]; exists {
		t.Fatal("explicit empty allowedComponents must retain omitted/default catalog semantics")
	}
	walkClosedObjectSchemas(t, definitions["presentation"], "$defs.presentation")
	encoded, err := json.Marshal(definitions["presentation"])
	if err != nil {
		t.Fatalf("marshal presentation schema: %v", err)
	}
	for _, forbidden := range []string{
		`"route"`, `"url"`, `"method"`, `"headers"`, `"permission"`,
		`"script"`, `"html"`, `"sql"`, `"import"`, `"handler"`,
	} {
		if bytes.Contains(encoded, []byte(forbidden)) {
			t.Fatalf("presentation source schema exposes forbidden authority or executable property %s", forbidden)
		}
	}
}

func validPresentationModule() *Module {
	module := validModule()
	module.Spec.Entity.Fields[0].Sortable = true
	module.Spec.Entity.Fields[0].Validation.MinLength = intPointer(2)
	module.Spec.Presentation = &PresentationSource{
		PageKey: "purchase-order.list", DefinitionVersion: "2",
		Title:      PresentationLocalizedText{ZhCN: "采购订单 & <列表>", EnUS: "Purchase orders & <Orders>"},
		DataSource: "list",
		List: PresentationListSource{
			Density: "large", PageSize: 20, DefaultSort: []PresentationSortSource{},
			Fields: []PresentationFieldSource{{Field: "orderCode", Component: "text", Order: 10, Width: intPointer(180)}},
		},
		Search: PresentationSearchSource{
			CollapsedByDefault: false,
			Fields:             []PresentationFieldSource{{Field: "orderCode", Component: "input", Order: 10}},
		},
		Form: PresentationFormSource{
			Columns: 1,
			Fields:  []PresentationFieldSource{{Field: "orderCode", Component: "input", Order: 10, Span: intPointer(24)}},
		},
		Detail: PresentationDetailSource{
			Columns: 1,
			Fields: []PresentationFieldSource{
				{Field: "orderCode", Component: "text", Order: 10, Span: intPointer(24)},
				{Field: "id", Component: "copyable-code", AllowedComponents: []string{"copyable-code"}, Order: 20, Span: intPointer(24)},
				{Field: "createdAt", Component: "date-time", Order: 30, Span: intPointer(24)},
				{Field: "updatedAt", Component: "date-time", Order: 40, Span: intPointer(24)},
			},
		},
		Actions: []PresentationActionSource{
			{Action: "create", Placement: "toolbar", Order: 10},
			{Action: "export", Placement: "toolbar", Order: 20},
			{Action: "read", Placement: "row", Order: 30},
			{Action: "update", Placement: "row", Order: 40},
			{
				Action: "delete", Placement: "row", Order: 50,
				Confirm: &PresentationLocalizedText{ZhCN: "确认删除？", EnUS: "Delete this order?"},
			},
		},
	}
	module.Normalize()
	return module
}

func validationErrorHasCode(err error, code string) bool {
	validation, ok := err.(*ValidationError)
	if !ok {
		return false
	}
	for _, issue := range validation.Issues {
		if issue.Code == code {
			return true
		}
	}
	return false
}

func validationErrorHasIssue(err error, path, code string) bool {
	validation, ok := err.(*ValidationError)
	if !ok {
		return false
	}
	for _, issue := range validation.Issues {
		if issue.Path == path && issue.Code == code {
			return true
		}
	}
	return false
}

func replacePresentationTestYAML(t *testing.T, data []byte, old, replacement string) []byte {
	t.Helper()
	if strings.Count(string(data), old) != 1 {
		t.Fatalf("fixture occurrence count for %q = %d, want 1", old, strings.Count(string(data), old))
	}
	return []byte(strings.Replace(string(data), old, replacement, 1))
}

func normalizedField(t *testing.T, manifest *NormalizedPresentationManifest, id string) NormalizedPresentationField {
	t.Helper()
	for _, field := range manifest.Fields {
		if field.ID == id {
			return field
		}
	}
	t.Fatalf("normalized manifest is missing field %q", id)
	return NormalizedPresentationField{}
}
