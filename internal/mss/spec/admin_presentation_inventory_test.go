package spec

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"testing"
)

var adminRoutePathPropertyPattern = regexp.MustCompile(`(?m)\bpath:\s*["']([^"']+)["']`)

func TestAdminPresentationPageInventorySchemaIsValidClosedAndAligned(t *testing.T) {
	schema := readAdminPresentationPageInventorySchema(t)
	if schema["$schema"] != "https://json-schema.org/draft/2020-12/schema" {
		t.Fatalf("schema dialect = %#v", schema["$schema"])
	}
	if schema["$id"] != "https://mss-boot.io/schemas/admin-presentation-page-inventory.v1alpha1.json" {
		t.Fatalf("schema id = %#v", schema["$id"])
	}
	properties := jsonObject(t, schema["properties"], "properties")
	if jsonObject(t, properties["apiVersion"], "properties.apiVersion")["const"] != ModuleAPIVersion {
		t.Fatalf("schema apiVersion does not equal %s", ModuleAPIVersion)
	}
	if jsonObject(t, properties["kind"], "properties.kind")["const"] != AdminPresentationPageInventoryKind {
		t.Fatalf("schema kind does not equal %s", AdminPresentationPageInventoryKind)
	}
	definitions := jsonObject(t, schema["$defs"], "$defs")
	specification := jsonObject(t, definitions["spec"], "$defs.spec")
	for _, required := range []string{"objective", "coveragePolicy", "facetPolicies", "pages", "routes"} {
		if !slices.Contains(jsonStrings(t, specification["required"], "$defs.spec.required"), required) {
			t.Errorf("inventory spec does not require %q", required)
		}
	}
	page := jsonObject(t, definitions["page"], "$defs.page")
	for _, required := range []string{"id", "disposition", "eligibility", "routePatterns", "facetPolicy", "protectedCapabilities"} {
		if !slices.Contains(jsonStrings(t, page["required"], "$defs.page.required"), required) {
			t.Errorf("inventory page does not require %q", required)
		}
	}
	route := jsonObject(t, definitions["route"], "$defs.route")
	for _, required := range []string{"id", "path", "routeKind", "disposition", "pageIDs", "reason"} {
		if !slices.Contains(jsonStrings(t, route["required"], "$defs.route.required"), required) {
			t.Errorf("inventory route does not require %q", required)
		}
	}
	walkClosedObjectSchemas(t, schema, "$")
}

func TestAdminPresentationPageInventoryHasExactProductScope(t *testing.T) {
	repositoryRoot := adminPresentationRepositoryRoot(t)
	inventory := loadRepositoryAdminPresentationInventory(t, repositoryRoot)

	wantPageKeys := map[string]bool{
		"supplier.list":       true,
		"user.list":           true,
		"role.list":           true,
		"menu.list":           true,
		"department.list":     true,
		"post.list":           true,
		"task.list":           true,
		"notice.list":         true,
		"language.list":       true,
		"option.list":         true,
		"system-config.list":  true,
		"online-session.list": true,
		"log.login":           true,
		"log.audit":           true,
		"log.runtime":         true,
	}
	wantExcludedIDs := map[string]bool{
		"workplace":               true,
		"account-pages":           true,
		"auth-pages":              true,
		"app-config":              true,
		"presentation-governance": true,
		"exception-pages":         true,
		"redirects":               true,
	}
	wantLimitedFacets := []string{
		"title.localized",
		"list.columns",
		"list.labels",
		"list.order",
		"list.visibility",
		"list.width",
		"list.density",
		"list.pageSize",
		"search.fields",
		"search.labels",
		"search.order",
		"search.visibility",
		"search.collapsedByDefault",
	}
	if !slices.Equal(inventory.Spec.FacetPolicies.LimitedTable, wantLimitedFacets) {
		t.Fatalf("limited-table facets = %#v, want %#v", inventory.Spec.FacetPolicies.LimitedTable, wantLimitedFacets)
	}

	included := map[string]AdminPresentationPageInventoryPage{}
	excluded := map[string]AdminPresentationPageInventoryPage{}
	for _, page := range inventory.Spec.Pages {
		switch page.Disposition {
		case "included":
			included[page.PageKey] = page
		case "excluded":
			excluded[page.ID] = page
		}
	}
	if len(included) != 15 {
		t.Fatalf("included page count = %d, want 15", len(included))
	}
	if len(excluded) != 7 {
		t.Fatalf("excluded page count = %d, want 7", len(excluded))
	}
	for pageKey := range wantPageKeys {
		if _, exists := included[pageKey]; !exists {
			t.Errorf("included page key %q is missing", pageKey)
		}
	}
	for pageKey := range included {
		if !wantPageKeys[pageKey] {
			t.Errorf("unexpected included page key %q", pageKey)
		}
	}
	for pageID := range wantExcludedIDs {
		if _, exists := excluded[pageID]; !exists {
			t.Errorf("sensitive or non-management exclusion %q is missing", pageID)
		}
	}
	for pageID := range excluded {
		if !wantExcludedIDs[pageID] {
			t.Errorf("unexpected excluded page %q", pageID)
		}
	}

	supplier := included["supplier.list"]
	if supplier.SourceKind != "admin-module" || supplier.FacetPolicy != "full-table" || supplier.TargetSourcePath != ".mss/modules/example-supplier.yaml" {
		t.Fatalf("Supplier must remain the sole full AdminModule reference: %#v", supplier)
	}
	coreCount := 0
	for pageKey, page := range included {
		if pageKey == "supplier.list" {
			continue
		}
		coreCount++
		if page.SourceKind != "foundation-core" || page.FacetPolicy != "limited-table" {
			t.Errorf("core page %s escaped the limited Foundation boundary: %#v", pageKey, page)
			continue
		}
		sourcePath := filepath.Join(repositoryRoot, filepath.FromSlash(page.TargetSourcePath))
		data, err := os.ReadFile(sourcePath)
		if err != nil {
			t.Errorf("read core page source %s: %v", page.TargetSourcePath, err)
			continue
		}
		document, err := ParseCorePagePresentation(data, page.TargetSourcePath)
		if err != nil {
			t.Errorf("parse core page source %s: %v", page.TargetSourcePath, err)
			continue
		}
		if document.Spec.PageKey != page.PageKey || document.Spec.Binding != page.Binding {
			t.Errorf("core page source identity %s/%s, inventory %s/%s", document.Spec.PageKey, document.Spec.Binding, page.PageKey, page.Binding)
		}
	}
	if coreCount != 14 {
		t.Fatalf("Foundation core page count = %d, want 14", coreCount)
	}
}

func TestAdminPresentationPageInventoryClosesCompiledRouteDeclarations(t *testing.T) {
	repositoryRoot := adminPresentationRepositoryRoot(t)
	inventory := loadRepositoryAdminPresentationInventory(t, repositoryRoot)
	wantRouteSources := []string{
		"web/antd-v6/package/core-routes.cjs",
		"web/antd-v6/config/routes.generated.ts",
		"web/antd-v6/src/generated/routes.ts",
	}
	if !slices.Equal(inventory.Metadata.RouteSources, wantRouteSources) {
		t.Fatalf("route sources = %#v, want %#v", inventory.Metadata.RouteSources, wantRouteSources)
	}

	coreSource := readRepositoryFile(t, repositoryRoot, wantRouteSources[0])
	generatedConfigSource := readRepositoryFile(t, repositoryRoot, wantRouteSources[1])
	generatedRegistrySource := readRepositoryFile(t, repositoryRoot, wantRouteSources[2])
	compiledCore := extractAdminRoutePaths(t, coreSource, "const coreRoutes = [")
	compiledFallback := extractAdminRoutePaths(t, coreSource, "const fallbackRoutes = [")
	compiledGenerated := extractAdminRoutePaths(t, generatedConfigSource, "export default [")
	packagedGenerated := extractAdminRoutePaths(t, generatedRegistrySource, "export default [")
	if !slices.Equal(compiledGenerated, packagedGenerated) {
		t.Fatalf("generated route projections differ: compiled=%#v packaged=%#v", compiledGenerated, packagedGenerated)
	}

	var inventoryCore, inventoryGenerated, inventoryFallback []string
	for _, route := range inventory.Spec.Routes {
		switch {
		case strings.HasPrefix(route.ID, "core-"):
			inventoryCore = append(inventoryCore, route.Path)
		case strings.HasPrefix(route.ID, "generated-"):
			inventoryGenerated = append(inventoryGenerated, route.Path)
		case strings.HasPrefix(route.ID, "fallback-"):
			inventoryFallback = append(inventoryFallback, route.Path)
		default:
			t.Errorf("route %q has no source-classifying ID prefix", route.ID)
		}
	}
	assertRouteDeclarationClosure(t, "core", inventoryCore, compiledCore)
	assertRouteDeclarationClosure(t, "generated", inventoryGenerated, compiledGenerated)
	assertRouteDeclarationClosure(t, "fallback", inventoryFallback, compiledFallback)
	if got, want := len(inventory.Spec.Routes), len(compiledCore)+len(compiledGenerated)+len(compiledFallback); got != want {
		t.Fatalf("route declaration count = %d, compiled count = %d", got, want)
	}
}

func TestAdminPresentationPageInventoryParserFailsClosed(t *testing.T) {
	repositoryRoot := adminPresentationRepositoryRoot(t)
	sourcePath := filepath.Join(repositoryRoot, ".mss", "admin-presentation-page-inventory.yaml")
	data, err := os.ReadFile(sourcePath)
	if err != nil {
		t.Fatalf("read page inventory: %v", err)
	}
	tests := []struct {
		name       string
		old        string
		new        string
		wantSubstr string
	}{
		{
			name:       "unknown field",
			old:        "  revision: \"1\"\n",
			new:        "  revision: \"1\"\n  unexpected: true\n",
			wantSubstr: "field unexpected not found",
		},
		{
			name:       "dangling page reference",
			old:        "pageIDs: [user-list]",
			new:        "pageIDs: [missing-user-list]",
			wantSubstr: "references unknown page missing-user-list",
		},
		{
			name:       "mismatched definition hashes",
			old:        "frontendHash: sha256:20979dbd25719ae69c07e383627849910d8a7fb8beb95b023362d56a4c4d72c7",
			new:        "frontendHash: sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			wantSubstr: "backend and frontend hashes must match",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			mutated := strings.Replace(string(data), test.old, test.new, 1)
			if mutated == string(data) {
				t.Fatalf("fixture mutation did not match %q", test.old)
			}
			_, err := ParseAdminPresentationPageInventory([]byte(mutated), "inventory.yaml")
			if err == nil || !strings.Contains(err.Error(), test.wantSubstr) {
				t.Fatalf("ParseAdminPresentationPageInventory() error = %v, want %q", err, test.wantSubstr)
			}
		})
	}
}

func TestValidateFileDispatchesAdminPresentationPageInventory(t *testing.T) {
	repositoryRoot := adminPresentationRepositoryRoot(t)
	path := filepath.Join(repositoryRoot, ".mss", "admin-presentation-page-inventory.yaml")
	document, err := ValidateFile(path)
	if err != nil {
		t.Fatalf("ValidateFile(page inventory) error = %v", err)
	}
	if document.Kind != AdminPresentationPageInventoryKind || document.Name != "admin-presentation-pages" {
		t.Fatalf("validated inventory identity = %q/%q", document.Kind, document.Name)
	}
	if document.Summary["includedPages"] != 15 || document.Summary["excludedPages"] != 7 {
		t.Fatalf("validated inventory summary = %#v", document.Summary)
	}
}

func readAdminPresentationPageInventorySchema(t *testing.T) map[string]any {
	t.Helper()
	repositoryRoot := adminPresentationRepositoryRoot(t)
	data := readRepositoryFile(t, repositoryRoot, ".mss/schemas/admin-presentation-page-inventory.schema.json")
	var schema map[string]any
	if err := json.Unmarshal(data, &schema); err != nil {
		t.Fatalf("parse Admin presentation page inventory schema: %v", err)
	}
	return schema
}

func loadRepositoryAdminPresentationInventory(t *testing.T, repositoryRoot string) *AdminPresentationPageInventory {
	t.Helper()
	path := filepath.Join(repositoryRoot, ".mss", "admin-presentation-page-inventory.yaml")
	inventory, err := LoadAdminPresentationPageInventory(path)
	if err != nil {
		t.Fatalf("load Admin presentation page inventory: %v", err)
	}
	return inventory
}

func adminPresentationRepositoryRoot(t *testing.T) string {
	t.Helper()
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve Admin presentation inventory test path")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(currentFile), "..", "..", ".."))
}

func readRepositoryFile(t *testing.T, repositoryRoot, relativePath string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(repositoryRoot, filepath.FromSlash(relativePath)))
	if err != nil {
		t.Fatalf("read %s: %v", relativePath, err)
	}
	return data
}

func extractAdminRoutePaths(t *testing.T, source []byte, declaration string) []string {
	t.Helper()
	start := bytes.Index(source, []byte(declaration))
	if start < 0 {
		t.Fatalf("route declaration %q is missing", declaration)
	}
	arrayStart := start + len(declaration) - 1
	arrayEnd := matchingJSArrayEnd(t, source, arrayStart)
	matches := adminRoutePathPropertyPattern.FindAllSubmatch(source[arrayStart:arrayEnd+1], -1)
	paths := make([]string, 0, len(matches))
	for _, match := range matches {
		paths = append(paths, string(match[1]))
	}
	if len(paths) == 0 {
		t.Fatalf("route declaration %q contains no paths", declaration)
	}
	return paths
}

func matchingJSArrayEnd(t *testing.T, source []byte, start int) int {
	t.Helper()
	depth := 0
	var quote byte
	escaped := false
	lineComment := false
	blockComment := false
	for index := start; index < len(source); index++ {
		current := source[index]
		next := byte(0)
		if index+1 < len(source) {
			next = source[index+1]
		}
		if lineComment {
			if current == '\n' {
				lineComment = false
			}
			continue
		}
		if blockComment {
			if current == '*' && next == '/' {
				blockComment = false
				index++
			}
			continue
		}
		if quote != 0 {
			if escaped {
				escaped = false
				continue
			}
			if current == '\\' {
				escaped = true
				continue
			}
			if current == quote {
				quote = 0
			}
			continue
		}
		if current == '/' && next == '/' {
			lineComment = true
			index++
			continue
		}
		if current == '/' && next == '*' {
			blockComment = true
			index++
			continue
		}
		if current == '\'' || current == '"' || current == '`' {
			quote = current
			continue
		}
		switch current {
		case '[':
			depth++
		case ']':
			depth--
			if depth == 0 {
				return index
			}
		}
	}
	t.Fatalf("route array starting at byte %d is not closed", start)
	return -1
}

func assertRouteDeclarationClosure(t *testing.T, source string, inventory, compiled []string) {
	t.Helper()
	if !slices.Equal(inventory, compiled) {
		t.Fatalf("%s route closure differs:\ninventory: %#v\ncompiled:  %#v", source, inventory, compiled)
	}
}
