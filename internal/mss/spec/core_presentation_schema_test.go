package spec

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestAdminCorePagePresentationSchemaIsClosedAndAuthorityFree(t *testing.T) {
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve core presentation schema test path")
	}
	path := filepath.Clean(filepath.Join(filepath.Dir(currentFile), "..", "..", "..", ".mss", "schemas", "admin-core-page-presentation.schema.json"))
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read core page presentation schema: %v", err)
	}
	var schema map[string]any
	if err := json.Unmarshal(data, &schema); err != nil {
		t.Fatalf("parse core page presentation schema: %v", err)
	}
	if schema["$id"] != "https://mss-boot.io/schemas/admin-core-page-presentation.v1alpha1.json" {
		t.Fatalf("schema id = %#v", schema["$id"])
	}
	forbidden := map[string]bool{
		"permission": true, "permissions": true, "route": true, "url": true,
		"method": true, "import": true, "guard": true, "handler": true,
		"operation": true, "expression": true, "script": true, "html": true,
	}
	walkPresentationSchema(t, schema, "$", forbidden)
}
