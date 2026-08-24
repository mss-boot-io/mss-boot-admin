package spec

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"testing"
)

func TestAdminPagePresentationSchemaIsVersionedClosedAndDataOnly(t *testing.T) {
	schema := readAdminPagePresentationSchema(t)
	if schema["$schema"] != "https://json-schema.org/draft/2020-12/schema" {
		t.Fatalf("unexpected JSON Schema dialect: %#v", schema["$schema"])
	}
	if schema["$id"] != "https://mss-boot.io/schemas/admin-page-presentation.v1alpha1.json" {
		t.Fatalf("unexpected schema id: %#v", schema["$id"])
	}
	properties := jsonObject(t, schema["properties"], "properties")
	if jsonObject(t, properties["apiVersion"], "properties.apiVersion")["const"] != "mss.io/v1alpha1" {
		t.Fatal("apiVersion must be fixed to mss.io/v1alpha1")
	}
	if jsonObject(t, properties["kind"], "properties.kind")["const"] != "AdminPagePresentation" {
		t.Fatal("kind must be fixed to AdminPagePresentation")
	}

	forbidden := map[string]bool{
		"permission": true, "permissions": true, "route": true, "url": true,
		"method": true, "headers": true, "html": true, "script": true,
		"expression": true, "sql": true, "import": true, "componentImport": true,
		"plugin": true, "template": true,
	}
	walkPresentationSchema(t, schema, "$", forbidden)
}

func TestAdminPagePresentationSchemaBindsProfilesToCompiledCapabilities(t *testing.T) {
	schema := readAdminPagePresentationSchema(t)
	definitions := jsonObject(t, schema["$defs"], "$defs")
	metadata := jsonObject(t, definitions["metadata"], "$defs.metadata")
	required := jsonStrings(t, metadata["required"], "$defs.metadata.required")
	for _, field := range []string{"name", "pageKey", "definitionHash", "scope"} {
		if !slices.Contains(required, field) {
			t.Fatalf("metadata does not require %q: %#v", field, required)
		}
	}
	hash := jsonObject(t, definitions["definitionHash"], "$defs.definitionHash")
	if hash["pattern"] != "^sha256:[0-9a-f]{64}$" {
		t.Fatalf("definition hash is not an exact SHA-256 identity: %#v", hash["pattern"])
	}

	specification := jsonObject(t, definitions["spec"], "$defs.spec")
	specificationProperties := jsonObject(t, specification["properties"], "$defs.spec.properties")
	for _, reference := range []string{"dataSource", "list", "search", "form", "detail", "actions"} {
		if _, exists := specificationProperties[reference]; !exists {
			t.Fatalf("presentation spec is missing %q", reference)
		}
	}

	fieldPatch := jsonObject(t, definitions["fieldPatch"], "$defs.fieldPatch")
	fieldProperties := jsonObject(t, fieldPatch["properties"], "$defs.fieldPatch.properties")
	if _, exists := fieldProperties["field"]; !exists {
		t.Fatal("field patches must use a stable field reference")
	}
	if _, exists := fieldProperties["component"]; !exists {
		t.Fatal("field patches must use a registered component reference")
	}
	actionPatch := jsonObject(t, definitions["actionPatch"], "$defs.actionPatch")
	actionProperties := jsonObject(t, actionPatch["properties"], "$defs.actionPatch.properties")
	if _, exists := actionProperties["action"]; !exists {
		t.Fatal("action patches must use a stable action reference")
	}
}

func TestAdminPagePresentationConditionDSLIsBoundedAndNonExecutable(t *testing.T) {
	schema := readAdminPagePresentationSchema(t)
	definitions := jsonObject(t, schema["$defs"], "$defs")
	predicate := jsonObject(t, definitions["predicate"], "$defs.predicate")
	properties := jsonObject(t, predicate["properties"], "$defs.predicate.properties")
	operator := jsonObject(t, properties["operator"], "$defs.predicate.properties.operator")
	operators := jsonStrings(t, operator["enum"], "$defs.predicate.properties.operator.enum")
	expected := []string{"eq", "neq", "in", "not-in", "exists", "not-exists", "gt", "gte", "lt", "lte"}
	if !slices.Equal(operators, expected) {
		t.Fatalf("unexpected condition operators: %#v", operators)
	}

	for _, groupName := range []string{"allCondition", "anyCondition"} {
		group := jsonObject(t, definitions[groupName], "$defs."+groupName)
		groupProperties := jsonObject(t, group["properties"], "$defs."+groupName+".properties")
		key := "all"
		if groupName == "anyCondition" {
			key = "any"
		}
		items := jsonObject(t, groupProperties[key], "$defs."+groupName+".properties."+key)
		if items["maxItems"] != float64(20) {
			t.Fatalf("%s does not bound group size: %#v", groupName, items["maxItems"])
		}
	}
}

func walkPresentationSchema(t *testing.T, value any, path string, forbidden map[string]bool) {
	t.Helper()
	switch current := value.(type) {
	case map[string]any:
		if current["type"] == "object" && current["additionalProperties"] != false {
			t.Errorf("object schema %s is not closed with additionalProperties=false", path)
		}
		if rawProperties, exists := current["properties"]; exists {
			properties := jsonObject(t, rawProperties, path+".properties")
			for name := range properties {
				if forbidden[name] {
					t.Errorf("schema exposes forbidden executable or authority property %s.%s", path, name)
				}
			}
		}
		for key, nested := range current {
			walkPresentationSchema(t, nested, fmt.Sprintf("%s.%s", path, key), forbidden)
		}
	case []any:
		for index, nested := range current {
			walkPresentationSchema(t, nested, fmt.Sprintf("%s[%d]", path, index), forbidden)
		}
	}
}

func readAdminPagePresentationSchema(t *testing.T) map[string]any {
	t.Helper()
	_, sourceFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve presentation schema test source path")
	}
	path := filepath.Clean(filepath.Join(
		filepath.Dir(sourceFile), "..", "..", "..", ".mss", "schemas",
		"admin-page-presentation.schema.json",
	))
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read Admin page presentation JSON Schema: %v", err)
	}
	var schema map[string]any
	if err := json.Unmarshal(data, &schema); err != nil {
		t.Fatalf("parse Admin page presentation JSON Schema: %v", err)
	}
	return schema
}

func jsonObject(t *testing.T, value any, path string) map[string]any {
	t.Helper()
	object, ok := value.(map[string]any)
	if !ok {
		t.Fatalf("%s is not an object: %#v", path, value)
	}
	return object
}

func jsonStrings(t *testing.T, value any, path string) []string {
	t.Helper()
	items, ok := value.([]any)
	if !ok {
		t.Fatalf("%s is not an array: %#v", path, value)
	}
	values := make([]string, 0, len(items))
	for index, item := range items {
		text, ok := item.(string)
		if !ok {
			t.Fatalf("%s[%d] is not a string: %#v", path, index, item)
		}
		values = append(values, text)
	}
	return values
}
