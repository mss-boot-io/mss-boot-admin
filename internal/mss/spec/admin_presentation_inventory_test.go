package spec

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"testing"
)

var adminRoutePathPropertyPattern = regexp.MustCompile(`(?m)\bpath:\s*["']([^"']+)["']`)
var adminCoreRegistryIdentityPattern = regexp.MustCompile(`(?m)^  "([^"]+)": \{\n    definitionHash: "(sha256:[0-9a-f]{64})"`)

type generatedPresentationManifestFile struct {
	Sources   []string                        `json:"sources"`
	Manifests []generatedPresentationIdentity `json:"manifests"`
	Manifest  generatedPresentationIdentity   `json:"manifest"`
}

type generatedPresentationIdentity struct {
	PageKey        string `json:"pageKey"`
	DefinitionHash string `json:"definitionHash"`
}

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
	pageProperties := jsonObject(t, page["properties"], "$defs.page.properties")
	pageDispositions := jsonStrings(t, jsonObject(t, pageProperties["disposition"], "$defs.page.properties.disposition")["enum"], "$defs.page.properties.disposition.enum")
	for _, disposition := range []string{"included", "extension-example", "excluded"} {
		if !slices.Contains(pageDispositions, disposition) {
			t.Errorf("inventory page disposition does not allow %q", disposition)
		}
	}
	pageEligibility := jsonStrings(t, jsonObject(t, pageProperties["eligibility"], "$defs.page.properties.eligibility")["enum"], "$defs.page.properties.eligibility.enum")
	if !slices.Contains(pageEligibility, "external") {
		t.Error("inventory page eligibility does not allow external extensions")
	}
	route := jsonObject(t, definitions["route"], "$defs.route")
	for _, required := range []string{"id", "path", "routeKind", "disposition", "pageIDs", "reason"} {
		if !slices.Contains(jsonStrings(t, route["required"], "$defs.route.required"), required) {
			t.Errorf("inventory route does not require %q", required)
		}
	}
	routeProperties := jsonObject(t, route["properties"], "$defs.route.properties")
	routeDispositions := jsonStrings(t, jsonObject(t, routeProperties["disposition"], "$defs.route.properties.disposition")["enum"], "$defs.route.properties.disposition.enum")
	if !slices.Contains(routeDispositions, "extension-example") {
		t.Error("inventory route disposition does not allow extension-example")
	}
	walkClosedObjectSchemas(t, schema, "$")
}

func TestAdminPresentationPageInventoryHasExactProductScope(t *testing.T) {
	repositoryRoot := adminPresentationRepositoryRoot(t)
	inventory := loadRepositoryAdminPresentationInventory(t, repositoryRoot)

	wantPageKeys := map[string]bool{
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
	extensions := map[string]AdminPresentationPageInventoryPage{}
	excluded := map[string]AdminPresentationPageInventoryPage{}
	for _, page := range inventory.Spec.Pages {
		switch page.Disposition {
		case "included":
			included[page.PageKey] = page
		case "extension-example":
			extensions[page.PageKey] = page
		case "excluded":
			excluded[page.ID] = page
		}
	}
	if len(included) != 14 {
		t.Fatalf("included page count = %d, want 14", len(included))
	}
	if len(extensions) != 1 {
		t.Fatalf("extension-example page count = %d, want 1", len(extensions))
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

	supplier := extensions["supplier.list"]
	if supplier.Disposition != "extension-example" || supplier.Eligibility != "external" || supplier.SourceKind != "admin-module" || supplier.FacetPolicy != "full-table" || supplier.TargetSourcePath != ".mss/modules/example-supplier.yaml" {
		t.Fatalf("Supplier must remain the sole external full AdminModule example: %#v", supplier)
	}
	if supplier.ImplementationState != "generated" || supplier.AdoptionState != "not-applicable" || supplier.AcceptanceState != "not-applicable" || supplier.RolloutWave != "excluded" {
		t.Fatalf("Supplier extension lifecycle must remain outside built-in adoption and acceptance: %#v", supplier)
	}
	coreCount := 0
	for pageKey, page := range included {
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

func TestAdminPresentationPageInventoryMatchesGeneratedDefinitionsAndConsumers(t *testing.T) {
	repositoryRoot := adminPresentationRepositoryRoot(t)
	inventory := loadRepositoryAdminPresentationInventory(t, repositoryRoot)

	var coreManifest generatedPresentationManifestFile
	if err := json.Unmarshal(readRepositoryFile(t, repositoryRoot, "admin/presentation/core/manifest.generated.json"), &coreManifest); err != nil {
		t.Fatalf("parse generated core presentation manifest: %v", err)
	}
	if len(coreManifest.Sources) != 14 || len(coreManifest.Manifests) != 14 {
		t.Fatalf("generated core identity count = %d sources/%d manifests, want 14/14", len(coreManifest.Sources), len(coreManifest.Manifests))
	}
	coreHashes := make(map[string]string, len(coreManifest.Manifests))
	for index, manifest := range coreManifest.Manifests {
		coreHashes[manifest.PageKey] = manifest.DefinitionHash
		if got, want := coreManifest.Sources[index], coreSourcePathForPageKey(manifest.PageKey); got != want {
			t.Errorf("core source[%d] = %q, want %q", index, got, want)
		}
	}

	frontendHashes := map[string]string{}
	frontendSource := readRepositoryFile(t, repositoryRoot, "web/antd-v6/src/generated/core-presentation-registry.generated.ts")
	for _, match := range adminCoreRegistryIdentityPattern.FindAllSubmatch(frontendSource, -1) {
		frontendHashes[string(match[1])] = string(match[2])
	}
	if len(frontendHashes) != 14 {
		t.Fatalf("frontend core registry identity count = %d, want 14", len(frontendHashes))
	}
	supplierFrontendSource := readRepositoryFile(t, repositoryRoot, "web/antd-v6/src/generated/presentation-registry.generated.ts")
	supplierFrontendMatches := adminCoreRegistryIdentityPattern.FindAllSubmatch(supplierFrontendSource, -1)
	if len(supplierFrontendMatches) != 1 || string(supplierFrontendMatches[0][1]) != "supplier.list" {
		t.Fatalf("frontend Supplier registry identities = %#v", supplierFrontendMatches)
	}
	supplierFrontendHash := string(supplierFrontendMatches[0][2])

	var supplierManifest generatedPresentationManifestFile
	if err := json.Unmarshal(readRepositoryFile(t, repositoryRoot, "admin/modules/supplier/presentation_manifest.generated.json"), &supplierManifest); err != nil {
		t.Fatalf("parse generated Supplier presentation manifest: %v", err)
	}
	if supplierManifest.Manifest.PageKey != "supplier.list" {
		t.Fatalf("Supplier generated page key = %q", supplierManifest.Manifest.PageKey)
	}

	seen := map[string]bool{}
	for _, page := range inventory.Spec.Pages {
		if page.Disposition != "included" {
			continue
		}
		seen[page.PageKey] = true
		if page.ImplementationState != "generated" || page.DefinitionIdentity.State != "matching" {
			t.Errorf("%s lifecycle = %s/%s, want generated/matching", page.PageKey, page.ImplementationState, page.DefinitionIdentity.State)
		}
		if page.SourcePath != page.TargetSourcePath {
			t.Errorf("%s source identity = %q/%q", page.PageKey, page.SourcePath, page.TargetSourcePath)
		}
		if _, err := os.Stat(filepath.Join(repositoryRoot, filepath.FromSlash(page.RuntimeConsumer))); err != nil {
			t.Errorf("%s runtime consumer %q: %v", page.PageKey, page.RuntimeConsumer, err)
		}
		wantHash := coreHashes[page.PageKey]
		if frontendHashes[page.PageKey] != wantHash {
			t.Errorf("%s generated backend/frontend identities = %q/%q", page.PageKey, wantHash, frontendHashes[page.PageKey])
		}
		if wantHash == "" || page.DefinitionIdentity.BackendHash != wantHash || page.DefinitionIdentity.FrontendHash != wantHash {
			t.Errorf("%s inventory identity = %q/%q, generated = %q", page.PageKey, page.DefinitionIdentity.BackendHash, page.DefinitionIdentity.FrontendHash, wantHash)
		}
	}
	if len(seen) != 14 {
		t.Fatalf("generated included page identity count = %d, want 14", len(seen))
	}

	var supplier AdminPresentationPageInventoryPage
	for _, page := range inventory.Spec.Pages {
		if page.Disposition == "extension-example" {
			if supplier.PageKey != "" {
				t.Fatalf("multiple extension examples found: %q and %q", supplier.PageKey, page.PageKey)
			}
			supplier = page
		}
	}
	if supplier.PageKey != "supplier.list" {
		t.Fatalf("extension example page key = %q, want supplier.list", supplier.PageKey)
	}
	if supplier.ImplementationState != "generated" || supplier.DefinitionIdentity.State != "matching" {
		t.Fatalf("Supplier extension lifecycle = %s/%s, want generated/matching", supplier.ImplementationState, supplier.DefinitionIdentity.State)
	}
	if supplier.SourcePath != supplier.TargetSourcePath {
		t.Fatalf("Supplier extension source identity = %q/%q", supplier.SourcePath, supplier.TargetSourcePath)
	}
	if _, err := os.Stat(filepath.Join(repositoryRoot, filepath.FromSlash(supplier.RuntimeConsumer))); err != nil {
		t.Fatalf("Supplier extension runtime consumer %q: %v", supplier.RuntimeConsumer, err)
	}
	wantSupplierHash := supplierManifest.Manifest.DefinitionHash
	if supplierFrontendHash != wantSupplierHash {
		t.Errorf("Supplier generated backend/frontend identities = %q/%q", wantSupplierHash, supplierFrontendHash)
	}
	if wantSupplierHash == "" || supplier.DefinitionIdentity.BackendHash != wantSupplierHash || supplier.DefinitionIdentity.FrontendHash != wantSupplierHash {
		t.Errorf("Supplier extension inventory identity = %q/%q, generated = %q", supplier.DefinitionIdentity.BackendHash, supplier.DefinitionIdentity.FrontendHash, wantSupplierHash)
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

	wantSourceHashes := map[string]string{
		"web/antd-v6/package/core-routes.cjs":    "16c05447567ec784f98219cda20578c0c331036c977fffb0806eb44f00dca69b",
		"web/antd-v6/config/routes.generated.ts": "e6b670e37ea6133400fbf8d7283f7242e8baed4ab799512158064111af9726b9",
		"web/antd-v6/src/generated/routes.ts":    "bc6d132bb48fa1af81522b0ad58c08d2a9c32ebe81f16ede807be91f6d87899a",
	}
	for sourcePath, wantHash := range wantSourceHashes {
		sum := sha256.Sum256(readRepositoryFile(t, repositoryRoot, sourcePath))
		if gotHash := fmt.Sprintf("%x", sum); gotHash != wantHash {
			t.Errorf("compiled route declaration identity changed for %s: got %s, want %s", sourcePath, gotHash, wantHash)
		}
	}

	var routeIdentities strings.Builder
	for _, route := range inventory.Spec.Routes {
		fmt.Fprintf(&routeIdentities, "%s|%s|%s|%s|%s\n", route.ID, route.Path, route.RouteKind, route.Disposition, strings.Join(route.PageIDs, ","))
	}
	routeIdentityHash := fmt.Sprintf("%x", sha256.Sum256([]byte(routeIdentities.String())))
	const wantRouteIdentityHash = "d3c4d7a512ea420b78ad85228404a574200d0676de29bcbd88f9a2ea844e6d57"
	if routeIdentityHash != wantRouteIdentityHash {
		t.Fatalf("route classification identity changed: got %s, want %s", routeIdentityHash, wantRouteIdentityHash)
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
			old:        "  revision: \"2\"\n",
			new:        "  revision: \"2\"\n  unexpected: true\n",
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
		{
			name:       "generated source identity drift",
			old:        "sourcePath: .mss/core-pages/role-list.yaml",
			new:        "sourcePath: .mss/core-pages/user-list.yaml",
			wantSubstr: "sourcePath must equal targetSourcePath for generated pages",
		},
		{
			name:       "generated runtime consumer missing",
			old:        "runtimeConsumer: web/antd-v6/src/pages/Language/index.tsx",
			new:        "runtimeConsumer: ''",
			wantSubstr: "runtimeConsumer is unsafe or missing for a generated page",
		},
		{
			name:       "active page lacks acceptance",
			old:        "adoptionState: active\n      acceptanceState: passed",
			new:        "adoptionState: active\n      acceptanceState: pending",
			wantSubstr: "active adoption requires passed acceptance",
		},
		{
			name:       "extension example cannot be active",
			old:        "      adoptionState: not-applicable\n      acceptanceState: not-applicable\n      rolloutWave: excluded\n",
			new:        "      adoptionState: active\n      acceptanceState: not-applicable\n      rolloutWave: excluded\n",
			wantSubstr: "adoptionState must equal not-applicable for extension-example pages",
		},
		{
			name:       "extension example cannot pass built-in acceptance",
			old:        "      adoptionState: not-applicable\n      acceptanceState: not-applicable\n      rolloutWave: excluded\n",
			new:        "      adoptionState: not-applicable\n      acceptanceState: passed\n      rolloutWave: excluded\n",
			wantSubstr: "acceptanceState must equal not-applicable for extension-example pages",
		},
		{
			name:       "extension route cannot be classified as included",
			old:        "    - id: generated-suppliers\n      path: /suppliers\n      routeKind: page\n      disposition: extension-example\n",
			new:        "    - id: generated-suppliers\n      path: /suppliers\n      routeKind: page\n      disposition: included\n",
			wantSubstr: "disposition must equal extension-example for extension-example page supplier-list",
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

func coreSourcePathForPageKey(pageKey string) string {
	return ".mss/core-pages/" + strings.ReplaceAll(pageKey, ".", "-") + ".yaml"
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
	if document.Summary["includedPages"] != 14 || document.Summary["extensionPages"] != 1 || document.Summary["excludedPages"] != 7 {
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
