package spec

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"regexp"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

const AdminPresentationPageInventoryKind = "AdminPresentationPageInventory"

var (
	adminPresentationPageKeyPattern = regexp.MustCompile(`^[a-z][a-z0-9-]*(?:\.[a-z][a-z0-9-]*)+$`)
	adminPresentationHashPattern    = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)
	adminPresentationGitSHAPattern  = regexp.MustCompile(`^[0-9a-f]{40}$`)
	adminPresentationRevision       = regexp.MustCompile(`^[1-9][0-9]*$`)
	adminPresentationRolloutWave    = regexp.MustCompile(`^wave-[0-5]$`)
	adminPresentationFacetPattern   = regexp.MustCompile(`^[a-z][a-zA-Z0-9]*(?:\.[a-z][a-zA-Z0-9]*)*$`)
)

// AdminPresentationPageInventory is the closed product-coverage contract for
// compiled Admin routes and their bounded runtime-presentation disposition.
type AdminPresentationPageInventory struct {
	APIVersion string                                      `yaml:"apiVersion" json:"apiVersion"`
	Kind       string                                      `yaml:"kind" json:"kind"`
	Metadata   AdminPresentationPageInventoryMetadata      `yaml:"metadata" json:"metadata"`
	Spec       AdminPresentationPageInventorySpecification `yaml:"spec" json:"spec"`
	SourcePath string                                      `yaml:"-" json:"sourcePath,omitempty"`
}

type AdminPresentationPageInventoryMetadata struct {
	Name           string   `yaml:"name" json:"name"`
	Revision       string   `yaml:"revision" json:"revision"`
	BaselineBranch string   `yaml:"baselineBranch" json:"baselineBranch"`
	BaselineCommit string   `yaml:"baselineCommit" json:"baselineCommit"`
	RouteSources   []string `yaml:"routeSources" json:"routeSources"`
}

type AdminPresentationPageInventorySpecification struct {
	Objective      string                                    `yaml:"objective" json:"objective"`
	CoveragePolicy AdminPresentationPageInventoryPolicy      `yaml:"coveragePolicy" json:"coveragePolicy"`
	FacetPolicies  AdminPresentationPageInventoryFacetPolicy `yaml:"facetPolicies" json:"facetPolicies"`
	Pages          []AdminPresentationPageInventoryPage      `yaml:"pages" json:"pages"`
	Routes         []AdminPresentationPageInventoryRoute     `yaml:"routes" json:"routes"`
}

type AdminPresentationPageInventoryPolicy struct {
	ClosureRule              string `yaml:"closureRule" json:"closureRule"`
	IncludedRule             string `yaml:"includedRule" json:"includedRule"`
	ExcludedRule             string `yaml:"excludedRule" json:"excludedRule"`
	PairedGeneratedRouteRule string `yaml:"pairedGeneratedRouteRule" json:"pairedGeneratedRouteRule"`
	DefaultAdoptionMode      string `yaml:"defaultAdoptionMode" json:"defaultAdoptionMode"`
	ActivationRule           string `yaml:"activationRule" json:"activationRule"`
}

type AdminPresentationPageInventoryFacetPolicy struct {
	FullTable    []string `yaml:"full-table" json:"full-table"`
	LimitedTable []string `yaml:"limited-table" json:"limited-table"`
}

type AdminPresentationPageInventoryPage struct {
	ID                    string                                     `yaml:"id" json:"id"`
	DisplayName           map[string]string                          `yaml:"displayName" json:"displayName"`
	Disposition           string                                     `yaml:"disposition" json:"disposition"`
	Eligibility           string                                     `yaml:"eligibility" json:"eligibility"`
	PageKey               string                                     `yaml:"pageKey,omitempty" json:"pageKey,omitempty"`
	RoutePatterns         []string                                   `yaml:"routePatterns" json:"routePatterns"`
	SourceKind            string                                     `yaml:"sourceKind" json:"sourceKind"`
	SourcePath            string                                     `yaml:"sourcePath,omitempty" json:"sourcePath,omitempty"`
	TargetSourcePath      string                                     `yaml:"targetSourcePath,omitempty" json:"targetSourcePath,omitempty"`
	Binding               string                                     `yaml:"binding,omitempty" json:"binding,omitempty"`
	PageKind              string                                     `yaml:"pageKind" json:"pageKind"`
	DataClass             string                                     `yaml:"dataClass" json:"dataClass"`
	RootOnly              bool                                       `yaml:"rootOnly,omitempty" json:"rootOnly,omitempty"`
	RequiredPermissions   []string                                   `yaml:"requiredPermissions,omitempty" json:"requiredPermissions,omitempty"`
	ImplementationState   string                                     `yaml:"implementationState" json:"implementationState"`
	AdoptionState         string                                     `yaml:"adoptionState" json:"adoptionState"`
	AcceptanceState       string                                     `yaml:"acceptanceState" json:"acceptanceState"`
	RolloutWave           string                                     `yaml:"rolloutWave" json:"rolloutWave"`
	FacetPolicy           string                                     `yaml:"facetPolicy" json:"facetPolicy"`
	ProtectedCapabilities []string                                   `yaml:"protectedCapabilities" json:"protectedCapabilities"`
	RuntimeConsumer       string                                     `yaml:"runtimeConsumer,omitempty" json:"runtimeConsumer,omitempty"`
	DefinitionIdentity    AdminPresentationPageInventoryDefinitionID `yaml:"definitionIdentity,omitempty" json:"definitionIdentity,omitempty"`
	ExclusionReason       string                                     `yaml:"exclusionReason,omitempty" json:"exclusionReason,omitempty"`
	Notes                 string                                     `yaml:"notes,omitempty" json:"notes,omitempty"`
}

type AdminPresentationPageInventoryDefinitionID struct {
	State        string `yaml:"state,omitempty" json:"state,omitempty"`
	BackendHash  string `yaml:"backendHash,omitempty" json:"backendHash,omitempty"`
	FrontendHash string `yaml:"frontendHash,omitempty" json:"frontendHash,omitempty"`
}

type AdminPresentationPageInventoryRoute struct {
	ID          string   `yaml:"id" json:"id"`
	Path        string   `yaml:"path" json:"path"`
	RouteKind   string   `yaml:"routeKind" json:"routeKind"`
	Disposition string   `yaml:"disposition" json:"disposition"`
	PageIDs     []string `yaml:"pageIDs" json:"pageIDs"`
	Reason      string   `yaml:"reason" json:"reason"`
}

// LoadAdminPresentationPageInventory reads and validates one page inventory.
func LoadAdminPresentationPageInventory(sourcePath string) (*AdminPresentationPageInventory, error) {
	data, err := os.ReadFile(sourcePath)
	if err != nil {
		return nil, fmt.Errorf("read Admin presentation page inventory: %w", err)
	}
	return ParseAdminPresentationPageInventory(data, sourcePath)
}

// ParseAdminPresentationPageInventory strictly decodes one inventory. Unknown
// keys and additional YAML documents fail before semantic validation.
func ParseAdminPresentationPageInventory(data []byte, sourcePath string) (*AdminPresentationPageInventory, error) {
	inventory := &AdminPresentationPageInventory{SourcePath: sourcePath}
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(inventory); err != nil {
		return nil, fmt.Errorf("parse Admin presentation page inventory %s: %w", sourcePath, err)
	}
	var additionalDocument yaml.Node
	if err := decoder.Decode(&additionalDocument); err == nil {
		return nil, fmt.Errorf("parse Admin presentation page inventory %s: multiple YAML documents are not allowed", sourcePath)
	} else if !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("parse Admin presentation page inventory %s: %w", sourcePath, err)
	}
	if err := inventory.Validate(); err != nil {
		return nil, err
	}
	return inventory, nil
}

// Validate enforces schema-level invariants that require cross-record checks.
func (inventory *AdminPresentationPageInventory) Validate() error {
	problems := make([]string, 0)
	add := func(problem string) {
		problems = append(problems, problem)
	}
	if inventory.APIVersion != ModuleAPIVersion {
		add("apiVersion must equal " + ModuleAPIVersion)
	}
	if inventory.Kind != AdminPresentationPageInventoryKind {
		add("kind must equal " + AdminPresentationPageInventoryKind)
	}
	if !moduleNamePattern.MatchString(inventory.Metadata.Name) {
		add("metadata.name must be lower-case kebab-case")
	}
	if !adminPresentationRevision.MatchString(inventory.Metadata.Revision) {
		add("metadata.revision must be a positive integer string")
	}
	if strings.TrimSpace(inventory.Metadata.BaselineBranch) == "" {
		add("metadata.baselineBranch is required")
	}
	if !adminPresentationGitSHAPattern.MatchString(inventory.Metadata.BaselineCommit) {
		add("metadata.baselineCommit must be a full lower-case Git SHA")
	}
	validateUniqueRepositoryPaths("metadata.routeSources", inventory.Metadata.RouteSources, add)

	policy := inventory.Spec.CoveragePolicy
	for name, value := range map[string]string{
		"closureRule":              policy.ClosureRule,
		"includedRule":             policy.IncludedRule,
		"excludedRule":             policy.ExcludedRule,
		"pairedGeneratedRouteRule": policy.PairedGeneratedRouteRule,
		"activationRule":           policy.ActivationRule,
	} {
		if strings.TrimSpace(value) == "" {
			add("spec.coveragePolicy." + name + " is required")
		}
	}
	if policy.DefaultAdoptionMode != "disabled" {
		add("spec.coveragePolicy.defaultAdoptionMode must equal disabled")
	}
	if strings.TrimSpace(inventory.Spec.Objective) == "" {
		add("spec.objective is required")
	}
	validateFacetList("spec.facetPolicies.full-table", inventory.Spec.FacetPolicies.FullTable, add)
	validateFacetList("spec.facetPolicies.limited-table", inventory.Spec.FacetPolicies.LimitedTable, add)
	fullFacets := stringSet(inventory.Spec.FacetPolicies.FullTable)
	for _, facet := range inventory.Spec.FacetPolicies.LimitedTable {
		if !fullFacets[facet] {
			add("spec.facetPolicies.limited-table facet " + facet + " is not allowed by full-table")
		}
	}

	pageIDs := make(map[string]AdminPresentationPageInventoryPage, len(inventory.Spec.Pages))
	pageKeys := make(map[string]string, len(inventory.Spec.Pages))
	for index, page := range inventory.Spec.Pages {
		location := "spec.pages[" + strconv.Itoa(index) + "]"
		validateInventoryPage(location, page, add)
		if _, exists := pageIDs[page.ID]; exists {
			add(location + ".id must be unique")
		}
		pageIDs[page.ID] = page
		if page.PageKey != "" {
			if previous, exists := pageKeys[page.PageKey]; exists {
				add(location + ".pageKey duplicates page " + previous)
			}
			pageKeys[page.PageKey] = page.ID
		}
	}
	if len(inventory.Spec.Pages) == 0 {
		add("spec.pages must contain at least one page")
	}

	routeIDs := make(map[string]bool, len(inventory.Spec.Routes))
	coveredPatterns := make(map[string]map[string]bool, len(pageIDs))
	for index, route := range inventory.Spec.Routes {
		location := "spec.routes[" + strconv.Itoa(index) + "]"
		validateInventoryRoute(location, route, add)
		if routeIDs[route.ID] {
			add(location + ".id must be unique")
		}
		routeIDs[route.ID] = true
		seenPageIDs := map[string]bool{}
		for _, pageID := range route.PageIDs {
			if seenPageIDs[pageID] {
				add(location + ".pageIDs must be unique")
			}
			seenPageIDs[pageID] = true
			page, exists := pageIDs[pageID]
			if !exists {
				add(location + ".pageIDs references unknown page " + pageID)
				continue
			}
			if !contains(page.RoutePatterns, route.Path) {
				add(location + ".path is not declared by page " + pageID)
			}
			if coveredPatterns[pageID] == nil {
				coveredPatterns[pageID] = map[string]bool{}
			}
			coveredPatterns[pageID][route.Path] = true
		}
	}
	if len(inventory.Spec.Routes) == 0 {
		add("spec.routes must contain at least one route")
	}
	for pageID, page := range pageIDs {
		for _, routePattern := range page.RoutePatterns {
			if !coveredPatterns[pageID][routePattern] {
				add("spec.pages page " + pageID + " route pattern " + routePattern + " has no matching route declaration")
			}
		}
	}
	if len(problems) > 0 {
		return validationError{Problems: problems}
	}
	return nil
}

// Summary returns stable inventory counts for CLI and MCP callers.
func (inventory *AdminPresentationPageInventory) Summary() map[string]any {
	included := 0
	excluded := 0
	for _, page := range inventory.Spec.Pages {
		if page.Disposition == "included" {
			included++
		} else if page.Disposition == "excluded" {
			excluded++
		}
	}
	return map[string]any{
		"name":          inventory.Metadata.Name,
		"revision":      inventory.Metadata.Revision,
		"pages":         len(inventory.Spec.Pages),
		"includedPages": included,
		"excludedPages": excluded,
		"routes":        len(inventory.Spec.Routes),
	}
}

func validateInventoryPage(location string, page AdminPresentationPageInventoryPage, add func(string)) {
	if !moduleNamePattern.MatchString(page.ID) {
		add(location + ".id must be lower-case kebab-case")
	}
	if strings.TrimSpace(page.DisplayName["zh-CN"]) == "" || strings.TrimSpace(page.DisplayName["en-US"]) == "" || len(page.DisplayName) != 2 {
		add(location + ".displayName must contain exactly zh-CN and en-US")
	}
	validateUniqueRoutePatterns(location+".routePatterns", page.RoutePatterns, add)
	if !contains([]string{"business", "identity", "authorization", "operations", "configuration", "security", "navigation", "public"}, page.DataClass) {
		add(location + ".dataClass is unsupported")
	}
	if !contains([]string{"table", "dashboard", "settings", "public", "governance", "exception", "redirect"}, page.PageKind) {
		add(location + ".pageKind is unsupported")
	}
	if len(page.ProtectedCapabilities) == 0 {
		add(location + ".protectedCapabilities must contain at least one boundary")
	}
	if page.Disposition == "included" {
		if !contains([]string{"full", "limited"}, page.Eligibility) {
			add(location + ".eligibility must be full or limited")
		}
		if !adminPresentationPageKeyPattern.MatchString(page.PageKey) {
			add(location + ".pageKey is invalid")
		}
		if !contains([]string{"admin-module", "foundation-core"}, page.SourceKind) {
			add(location + ".sourceKind must be admin-module or foundation-core")
		}
		if !safeInventoryRepositoryPath(page.TargetSourcePath) {
			add(location + ".targetSourcePath is unsafe or missing")
		}
		if page.SourcePath != "" && !safeInventoryRepositoryPath(page.SourcePath) {
			add(location + ".sourcePath is unsafe")
		}
		if !adminPresentationPageKeyPattern.MatchString(page.Binding) {
			add(location + ".binding is invalid")
		}
		if page.PageKind != "table" {
			add(location + ".pageKind must be table for included pages")
		}
		if page.RequiredPermissions == nil {
			add(location + ".requiredPermissions must be explicit")
		}
		if !contains([]string{"planned", "generated"}, page.ImplementationState) {
			add(location + ".implementationState must be planned or generated")
		}
		if !contains([]string{"disabled", "shadow", "active"}, page.AdoptionState) {
			add(location + ".adoptionState is unsupported")
		}
		if !contains([]string{"pending", "passed"}, page.AcceptanceState) {
			add(location + ".acceptanceState is unsupported")
		}
		if page.RolloutWave == "excluded" || !adminPresentationRolloutWave.MatchString(page.RolloutWave) {
			add(location + ".rolloutWave must be wave-0 through wave-5")
		}
		wantFacetPolicy := "limited-table"
		if page.Eligibility == "full" {
			wantFacetPolicy = "full-table"
		}
		if page.FacetPolicy != wantFacetPolicy {
			add(location + ".facetPolicy must equal " + wantFacetPolicy)
		}
		validateDefinitionIdentity(location+".definitionIdentity", page.DefinitionIdentity, add)
		if page.ExclusionReason != "" {
			add(location + ".exclusionReason is only valid for excluded pages")
		}
		return
	}
	if page.Disposition != "excluded" {
		add(location + ".disposition must be included or excluded")
		return
	}
	if !contains([]string{"protected", "non-management"}, page.Eligibility) {
		add(location + ".eligibility must be protected or non-management")
	}
	if page.SourceKind != "not-applicable" {
		add(location + ".sourceKind must be not-applicable")
	}
	if page.ImplementationState != "excluded" || page.AdoptionState != "not-applicable" || page.AcceptanceState != "not-applicable" || page.RolloutWave != "excluded" || page.FacetPolicy != "none" {
		add(location + " excluded lifecycle and facet states are inconsistent")
	}
	if strings.TrimSpace(page.ExclusionReason) == "" {
		add(location + ".exclusionReason is required")
	}
	if page.PageKey != "" || page.SourcePath != "" || page.TargetSourcePath != "" || page.Binding != "" || page.RuntimeConsumer != "" || page.DefinitionIdentity.State != "" || page.RequiredPermissions != nil {
		add(location + " excluded pages cannot expose a presentation identity, source, consumer, or permission list")
	}
}

func validateDefinitionIdentity(location string, identity AdminPresentationPageInventoryDefinitionID, add func(string)) {
	switch identity.State {
	case "matching":
		if !adminPresentationHashPattern.MatchString(identity.BackendHash) || !adminPresentationHashPattern.MatchString(identity.FrontendHash) {
			add(location + " matching state requires two SHA-256 hashes")
		} else if identity.BackendHash != identity.FrontendHash {
			add(location + " backend and frontend hashes must match")
		}
	case "planned":
		if identity.BackendHash != "" || identity.FrontendHash != "" {
			add(location + " planned state cannot claim generated hashes")
		}
	default:
		add(location + ".state must be matching or planned")
	}
}

func validateInventoryRoute(location string, route AdminPresentationPageInventoryRoute, add func(string)) {
	if !moduleNamePattern.MatchString(route.ID) {
		add(location + ".id must be lower-case kebab-case")
	}
	if !strings.HasPrefix(route.Path, "/") {
		add(location + ".path must be absolute")
	}
	if !contains([]string{"layout", "page", "subroute", "redirect", "fallback"}, route.RouteKind) {
		add(location + ".routeKind is unsupported")
	}
	if !contains([]string{"included", "excluded", "redirect", "exception"}, route.Disposition) {
		add(location + ".disposition is unsupported")
	}
	if len(route.PageIDs) == 0 {
		add(location + ".pageIDs must contain at least one page")
	}
	if strings.TrimSpace(route.Reason) == "" {
		add(location + ".reason is required")
	}
}

func validateFacetList(location string, facets []string, add func(string)) {
	if len(facets) == 0 {
		add(location + " must contain at least one facet")
		return
	}
	seen := map[string]bool{}
	for _, facet := range facets {
		if !adminPresentationFacetPattern.MatchString(facet) {
			add(location + " contains invalid facet " + facet)
		}
		if seen[facet] {
			add(location + " contains duplicate facet " + facet)
		}
		seen[facet] = true
	}
}

func validateUniqueRepositoryPaths(location string, values []string, add func(string)) {
	if len(values) == 0 {
		add(location + " must contain at least one path")
	}
	seen := map[string]bool{}
	for _, value := range values {
		if !safeInventoryRepositoryPath(value) {
			add(location + " contains unsafe path " + value)
		}
		if seen[value] {
			add(location + " contains duplicate path " + value)
		}
		seen[value] = true
	}
}

func validateUniqueRoutePatterns(location string, values []string, add func(string)) {
	if len(values) == 0 {
		add(location + " must contain at least one route pattern")
	}
	seen := map[string]bool{}
	for _, value := range values {
		if !strings.HasPrefix(value, "/") {
			add(location + " contains non-absolute pattern " + value)
		}
		if seen[value] {
			add(location + " contains duplicate pattern " + value)
		}
		seen[value] = true
	}
}

func safeInventoryRepositoryPath(value string) bool {
	if value == "" || strings.HasPrefix(value, "/") || strings.Contains(value, "\\") {
		return false
	}
	clean := path.Clean(value)
	return clean != "." && clean != ".." && !strings.HasPrefix(clean, "../")
}

func stringSet(values []string) map[string]bool {
	set := make(map[string]bool, len(values))
	for _, value := range values {
		set[value] = true
	}
	return set
}
