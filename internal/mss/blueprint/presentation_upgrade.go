package blueprint

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"regexp"
	"sort"
	"strings"

	"github.com/mss-boot-io/mss-boot-admin/internal/mss/spec"
	"gopkg.in/yaml.v3"
)

const (
	presentationPageInventoryPath    = ".mss/admin-presentation-page-inventory.yaml"
	presentationFrontendRegistryPath = "web/antd-v6/src/generated/core-presentation-registry.generated.ts"
	presentationSnapshotPath         = ".mss/admin-presentation-snapshot.json"
	presentationSnapshotVersion      = "mss.io/v1alpha1"
	presentationSnapshotKind         = "AdminPresentationSnapshot"

	presentationSnapshotAvailable   = "available"
	presentationSnapshotUnrecorded  = "unrecorded"
	presentationSnapshotMissing     = "missing"
	presentationSnapshotModified    = "modified"
	presentationSnapshotUnavailable = "unavailable"
)

var presentationFrontendRegistryEntryPattern = regexp.MustCompile(
	`(?m)^  "([^"\r\n]+)": \{\r?\n    definitionHash: "(sha256:[0-9a-f]{64})",\r?\n    definition: \{\r?\n      "pageKey": "([^"\r\n]+)",\r?\n      "definitionVersion": "([^"\r\n]+)",\r?\n      "definitionHash": "(sha256:[0-9a-f]{64})",`,
)

var presentationFrontendRegistryOuterKeyPattern = regexp.MustCompile(`(?m)^  "([^"\r\n]+)": \{$`)

// PresentationPageIdentity is the stable compatibility identity of one
// compiled Admin page. It deliberately contains no persisted profile data.
type PresentationPageIdentity struct {
	PageKey           string `json:"pageKey"`
	DefinitionVersion string `json:"definitionVersion"`
	DefinitionHash    string `json:"definitionHash"`
}

// PresentationUpgradeSnapshotSummary exposes the source-level identity facts
// available at one side of an Admin Distribution upgrade.
type PresentationUpgradeSnapshotSummary struct {
	State                           string                     `json:"state"`
	BackendFrontendInventoriesMatch bool                       `json:"backendFrontendInventoriesMatch"`
	Pages                           []PresentationPageIdentity `json:"pages"`
}

// PresentationUpgradeImpact reports only compiled capability identities. It
// never reads a downstream database and therefore never claims how many
// persisted profiles are affected.
type PresentationUpgradeImpact struct {
	From                     PresentationUpgradeSnapshotSummary `json:"from"`
	To                       PresentationUpgradeSnapshotSummary `json:"to"`
	ComparisonComplete       bool                               `json:"comparisonComplete"`
	AddedCapabilityIDs       []string                           `json:"addedCapabilityIds"`
	RemovedCapabilityIDs     []string                           `json:"removedCapabilityIds"`
	ChangedCapabilityIDs     []string                           `json:"changedCapabilityIds"`
	PotentiallyStalePageKeys []string                           `json:"potentiallyStalePageKeys"`
}

type presentationCapabilityIdentity struct {
	ID     string `json:"id"`
	SHA256 string `json:"sha256"`
}

type presentationPageSnapshot struct {
	PresentationPageIdentity
	Capabilities []presentationCapabilityIdentity `json:"capabilities"`
}

// presentationSnapshot is a value-free generated source snapshot kept in the
// normal managed Thin Host baseline. Its file hash is consequently covered by
// the existing Blueprint snapshot identity and three-way comparison.
type presentationSnapshot struct {
	APIVersion                      string                     `json:"apiVersion"`
	Kind                            string                     `json:"kind"`
	BackendInventorySHA256          string                     `json:"backendInventorySha256"`
	FrontendInventorySHA256         string                     `json:"frontendInventorySha256"`
	BackendFrontendInventoriesMatch bool                       `json:"backendFrontendInventoriesMatch"`
	Pages                           []presentationPageSnapshot `json:"pages"`
}

type generatedPresentationSnapshot struct {
	Generated string            `json:"$generated"`
	Sources   []string          `json:"sources"`
	Manifests []json.RawMessage `json:"manifests"`
}

type generatedPresentationPage struct {
	PageKey             string            `json:"pageKey"`
	DefinitionVersion   string            `json:"definitionVersion"`
	DefinitionHash      string            `json:"definitionHash"`
	Components          []json.RawMessage `json:"components"`
	Fields              []json.RawMessage `json:"fields"`
	DataSources         []json.RawMessage `json:"dataSources"`
	Actions             []json.RawMessage `json:"actions"`
	DefaultPresentation json.RawMessage   `json:"defaultPresentation"`
}

type presentationInventoryIdentity struct {
	BackendHash  string
	FrontendHash string
	SourcePath   string
}

type presentationSourceReader func(path string) ([]byte, bool, error)

func validateNewApplicationPresentationSource(snapshot presentationSnapshot) error {
	if snapshot.APIVersion == "" || snapshot.BackendFrontendInventoriesMatch {
		return nil
	}
	return errors.New(
		"Admin presentation backend/frontend inventories do not match; refusing to generate a new application from a one-sided Distribution source",
	)
}

func loadPresentationSourceSnapshot(read presentationSourceReader) (presentationSnapshot, error) {
	inventoryData, inventoryExists, err := read(presentationPageInventoryPath)
	if err != nil {
		return presentationSnapshot{}, err
	}
	if !inventoryExists {
		return presentationSnapshot{}, nil
	}
	frontendData, frontendExists, err := read(presentationFrontendRegistryPath)
	if err != nil {
		return presentationSnapshot{}, err
	}
	if !frontendExists {
		return presentationSnapshot{}, fmt.Errorf("packaged Admin frontend presentation registry is missing: %s", presentationFrontendRegistryPath)
	}
	inventory, err := parsePresentationPageInventory(inventoryData)
	if err != nil {
		return presentationSnapshot{}, fmt.Errorf("read Admin presentation page inventory: %w", err)
	}
	pageKeys := make([]string, 0, len(inventory))
	for pageKey := range inventory {
		pageKeys = append(pageKeys, pageKey)
	}
	sort.Strings(pageKeys)
	rawManifests := make([]json.RawMessage, 0, len(pageKeys))
	sources := make([]string, 0, len(pageKeys))
	for _, pageKey := range pageKeys {
		identity := inventory[pageKey]
		data, exists, err := read(identity.SourcePath)
		if err != nil {
			return presentationSnapshot{}, fmt.Errorf("read core presentation source %s: %w", identity.SourcePath, err)
		}
		if !exists {
			return presentationSnapshot{}, fmt.Errorf("core presentation source is missing: %s", identity.SourcePath)
		}
		document, err := spec.ParseCorePagePresentation(data, identity.SourcePath)
		if err != nil {
			return presentationSnapshot{}, fmt.Errorf("parse core presentation source %s: %w", identity.SourcePath, err)
		}
		manifest, err := document.NormalizePresentation()
		if err != nil {
			return presentationSnapshot{}, fmt.Errorf("normalize core presentation source %s: %w", identity.SourcePath, err)
		}
		if manifest.PageKey != pageKey {
			return presentationSnapshot{}, fmt.Errorf("core presentation source %s produced page key %s, want %s", identity.SourcePath, manifest.PageKey, pageKey)
		}
		raw, err := json.Marshal(manifest)
		if err != nil {
			return presentationSnapshot{}, fmt.Errorf("marshal core presentation source %s: %w", identity.SourcePath, err)
		}
		rawManifests = append(rawManifests, raw)
		sources = append(sources, identity.SourcePath)
	}
	backendDocument, err := json.Marshal(generatedPresentationSnapshot{
		Generated: "mss Admin presentation upgrade snapshot",
		Sources:   sources,
		Manifests: rawManifests,
	})
	if err != nil {
		return presentationSnapshot{}, fmt.Errorf("marshal Admin presentation backend snapshot: %w", err)
	}
	pages, err := parseGeneratedPresentationPages(backendDocument)
	if err != nil {
		return presentationSnapshot{}, fmt.Errorf("read Admin presentation backend snapshot: %w", err)
	}
	backendHashes := make(map[string]string, len(pages))
	backendIdentities := make(map[string]PresentationPageIdentity, len(pages))
	for _, page := range pages {
		backendHashes[page.PageKey] = page.DefinitionHash
		backendIdentities[page.PageKey] = page.PresentationPageIdentity
	}
	frontendHashes := make(map[string]string, len(inventory))
	actualFrontend, err := parsePresentationFrontendRegistry(frontendData)
	if err != nil {
		return presentationSnapshot{}, fmt.Errorf("read packaged Admin frontend presentation registry: %w", err)
	}
	inventoryBackendHashes := make(map[string]string, len(inventory))
	inventoryFrontendHashes := make(map[string]string, len(inventory))
	for pageKey, identity := range inventory {
		inventoryBackendHashes[pageKey] = identity.BackendHash
		inventoryFrontendHashes[pageKey] = identity.FrontendHash
	}
	for pageKey, identity := range actualFrontend {
		frontendHashes[pageKey] = identity.DefinitionHash
	}
	if !equalPresentationHashMaps(backendHashes, inventoryBackendHashes) {
		return presentationSnapshot{}, errors.New("Admin presentation page inventory backend hashes contradict the compiled core page sources")
	}
	if !equalPresentationHashMaps(frontendHashes, inventoryFrontendHashes) {
		return presentationSnapshot{}, errors.New("Admin presentation page inventory frontend hashes contradict the packaged frontend registry")
	}
	backendDigest, err := presentationPageIdentityDigest(backendIdentities)
	if err != nil {
		return presentationSnapshot{}, err
	}
	frontendDigest, err := presentationPageIdentityDigest(actualFrontend)
	if err != nil {
		return presentationSnapshot{}, err
	}
	return presentationSnapshot{
		APIVersion:                      presentationSnapshotVersion,
		Kind:                            presentationSnapshotKind,
		BackendInventorySHA256:          backendDigest,
		FrontendInventorySHA256:         frontendDigest,
		BackendFrontendInventoriesMatch: backendDigest == frontendDigest,
		Pages:                           pages,
	}, nil
}

func parsePresentationFrontendRegistry(data []byte) (map[string]PresentationPageIdentity, error) {
	content := strings.ReplaceAll(string(data), "\r\n", "\n")
	const inventoryStart = "export const corePresentationInventory = [\n"
	const inventoryEnd = "\n] as const;"
	inventoryOffset := strings.Index(content, inventoryStart)
	if inventoryOffset < 0 {
		return nil, errors.New("frontend registry omits corePresentationInventory")
	}
	inventoryBodyStart := inventoryOffset + len(inventoryStart)
	inventoryBodyEnd := strings.Index(content[inventoryBodyStart:], inventoryEnd)
	if inventoryBodyEnd < 0 {
		return nil, errors.New("frontend registry has an unterminated corePresentationInventory")
	}
	inventoryBody := content[inventoryBodyStart : inventoryBodyStart+inventoryBodyEnd]
	inventoryKeys := make(map[string]bool)
	for index, line := range strings.Split(inventoryBody, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if !strings.HasSuffix(line, ",") {
			return nil, fmt.Errorf("frontend presentation inventory line %d is not a generated JSON string entry", index+1)
		}
		var pageKey string
		if err := json.Unmarshal([]byte(strings.TrimSuffix(line, ",")), &pageKey); err != nil {
			return nil, fmt.Errorf("parse frontend presentation inventory line %d: %w", index+1, err)
		}
		pageKey = strings.TrimSpace(pageKey)
		if pageKey == "" {
			return nil, fmt.Errorf("frontend presentation inventory line %d has an empty page key", index+1)
		}
		if inventoryKeys[pageKey] {
			return nil, fmt.Errorf("frontend presentation inventory duplicates page key %s", pageKey)
		}
		inventoryKeys[pageKey] = true
	}
	if len(inventoryKeys) == 0 {
		return nil, errors.New("frontend presentation inventory contains no page keys")
	}

	const registryStart = "export const corePresentationRegistry = {\n"
	registryOffset := strings.Index(content, registryStart)
	if registryOffset < 0 {
		return nil, errors.New("frontend registry omits corePresentationRegistry")
	}
	registryBodyStart := registryOffset + len(registryStart)
	registryBodyEnd := strings.LastIndex(content[registryBodyStart:], "\n} as const;")
	if registryBodyEnd < 0 {
		return nil, errors.New("frontend registry has an unterminated corePresentationRegistry")
	}
	registryBody := content[registryBodyStart : registryBodyStart+registryBodyEnd]
	outerMatches := presentationFrontendRegistryOuterKeyPattern.FindAllStringSubmatch(registryBody, -1)
	entryMatches := presentationFrontendRegistryEntryPattern.FindAllStringSubmatch(registryBody, -1)
	if len(outerMatches) != len(entryMatches) {
		return nil, errors.New("frontend presentation registry contains an entry without a complete generated identity")
	}
	identities := make(map[string]PresentationPageIdentity, len(entryMatches))
	for _, match := range entryMatches {
		outerPageKey := strings.TrimSpace(match[1])
		outerHash := strings.TrimSpace(match[2])
		pageKey := strings.TrimSpace(match[3])
		version := strings.TrimSpace(match[4])
		definitionHash := strings.TrimSpace(match[5])
		if outerPageKey == "" || outerPageKey != pageKey {
			return nil, fmt.Errorf("frontend presentation registry entry %q contradicts its page identity %q", outerPageKey, pageKey)
		}
		if version == "" || !validPresentationDefinitionHash(outerHash) || outerHash != definitionHash {
			return nil, fmt.Errorf("frontend presentation registry entry %s has an invalid or contradictory identity", pageKey)
		}
		if _, exists := identities[pageKey]; exists {
			return nil, fmt.Errorf("frontend presentation registry duplicates page key %s", pageKey)
		}
		identities[pageKey] = PresentationPageIdentity{
			PageKey: pageKey, DefinitionVersion: version, DefinitionHash: definitionHash,
		}
	}
	if len(identities) != len(inventoryKeys) {
		return nil, errors.New("frontend presentation registry page keys contradict corePresentationInventory")
	}
	for pageKey := range inventoryKeys {
		if _, exists := identities[pageKey]; !exists {
			return nil, fmt.Errorf("frontend presentation registry omits inventory page key %s", pageKey)
		}
	}
	return identities, nil
}

func parseGeneratedPresentationPages(data []byte) ([]presentationPageSnapshot, error) {
	if err := rejectDuplicateJSONKeys(data); err != nil {
		return nil, fmt.Errorf("parse generated presentation snapshot: %w", err)
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	document := generatedPresentationSnapshot{}
	if err := decoder.Decode(&document); err != nil {
		return nil, fmt.Errorf("parse generated presentation snapshot: %w", err)
	}
	if err := ensureJSONEOF(decoder); err != nil {
		return nil, fmt.Errorf("parse generated presentation snapshot: %w", err)
	}
	if len(document.Manifests) == 0 {
		return nil, errors.New("generated presentation snapshot contains no page manifests")
	}
	pages := make([]presentationPageSnapshot, 0, len(document.Manifests))
	seenPages := make(map[string]bool, len(document.Manifests))
	for index, raw := range document.Manifests {
		page := generatedPresentationPage{}
		if err := json.Unmarshal(raw, &page); err != nil {
			return nil, fmt.Errorf("parse generated presentation page %d: %w", index, err)
		}
		page.PageKey = strings.TrimSpace(page.PageKey)
		page.DefinitionVersion = strings.TrimSpace(page.DefinitionVersion)
		page.DefinitionHash = strings.TrimSpace(page.DefinitionHash)
		if page.PageKey == "" || page.DefinitionVersion == "" || !validPresentationDefinitionHash(page.DefinitionHash) {
			return nil, fmt.Errorf("generated presentation page %d has an invalid identity", index)
		}
		if seenPages[page.PageKey] {
			return nil, fmt.Errorf("generated presentation snapshot duplicates page key %s", page.PageKey)
		}
		seenPages[page.PageKey] = true
		capabilities, err := presentationCapabilityIdentities(page)
		if err != nil {
			return nil, fmt.Errorf("read generated presentation page %s: %w", page.PageKey, err)
		}
		pages = append(pages, presentationPageSnapshot{
			PresentationPageIdentity: PresentationPageIdentity{
				PageKey: page.PageKey, DefinitionVersion: page.DefinitionVersion, DefinitionHash: page.DefinitionHash,
			},
			Capabilities: capabilities,
		})
	}
	sort.SliceStable(pages, func(i, j int) bool { return pages[i].PageKey < pages[j].PageKey })
	return pages, nil
}

func presentationCapabilityIdentities(page generatedPresentationPage) ([]presentationCapabilityIdentity, error) {
	capacity, err := presentationCapabilityCapacity(
		len(page.Components),
		len(page.Fields),
		len(page.DataSources),
		len(page.Actions),
	)
	if err != nil {
		return nil, fmt.Errorf("count capabilities for page %s: %w", page.PageKey, err)
	}
	capabilities := make([]presentationCapabilityIdentity, 0, capacity)
	seen := map[string]bool{}
	appendCollection := func(kind string, values []json.RawMessage) error {
		for index, raw := range values {
			var identity struct {
				ID string `json:"id"`
			}
			if err := json.Unmarshal(raw, &identity); err != nil {
				return fmt.Errorf("parse %s capability %d: %w", kind, index, err)
			}
			identity.ID = strings.TrimSpace(identity.ID)
			if identity.ID == "" {
				return fmt.Errorf("%s capability %d has no id", kind, index)
			}
			id := page.PageKey + "/" + kind + "/" + identity.ID
			if seen[id] {
				return fmt.Errorf("duplicate capability id %s", id)
			}
			hash, err := canonicalPresentationJSONDigest(raw)
			if err != nil {
				return fmt.Errorf("hash capability %s: %w", id, err)
			}
			seen[id] = true
			capabilities = append(capabilities, presentationCapabilityIdentity{ID: id, SHA256: hash})
		}
		return nil
	}
	for _, collection := range []struct {
		kind   string
		values []json.RawMessage
	}{
		{kind: "component", values: page.Components},
		{kind: "field", values: page.Fields},
		{kind: "data-source", values: page.DataSources},
		{kind: "action", values: page.Actions},
	} {
		if err := appendCollection(collection.kind, collection.values); err != nil {
			return nil, err
		}
	}
	if len(page.DefaultPresentation) == 0 || bytes.Equal(bytes.TrimSpace(page.DefaultPresentation), []byte("null")) {
		return nil, errors.New("default presentation is required")
	}
	var defaults map[string]json.RawMessage
	if err := json.Unmarshal(page.DefaultPresentation, &defaults); err != nil {
		return nil, fmt.Errorf("parse default presentation: %w", err)
	}
	for _, name := range []string{"title", "dataSource"} {
		raw, exists := defaults[name]
		if !exists {
			return nil, fmt.Errorf("default presentation omits %s", name)
		}
		id := page.PageKey + "/default/" + presentationCapabilityToken(name)
		hash, err := canonicalPresentationJSONDigest(raw)
		if err != nil {
			return nil, fmt.Errorf("hash capability %s: %w", id, err)
		}
		seen[id] = true
		capabilities = append(capabilities, presentationCapabilityIdentity{ID: id, SHA256: hash})
	}
	for _, name := range []string{"list", "search", "form", "detail", "actions"} {
		raw, exists := defaults[name]
		if !exists {
			return nil, fmt.Errorf("default presentation omits %s surface", name)
		}
		id := page.PageKey + "/surface/" + name
		hash, err := canonicalPresentationJSONDigest(raw)
		if err != nil {
			return nil, fmt.Errorf("hash capability %s: %w", id, err)
		}
		seen[id] = true
		capabilities = append(capabilities, presentationCapabilityIdentity{ID: id, SHA256: hash})
	}
	defaultHash, err := canonicalPresentationJSONDigest(page.DefaultPresentation)
	if err != nil {
		return nil, fmt.Errorf("hash complete default presentation: %w", err)
	}
	capabilities = append(capabilities, presentationCapabilityIdentity{ID: page.PageKey + "/defaults", SHA256: defaultHash})
	sort.SliceStable(capabilities, func(i, j int) bool { return capabilities[i].ID < capabilities[j].ID })
	return capabilities, nil
}

func presentationCapabilityCapacity(collectionSizes ...int) (int, error) {
	capacity := 8
	for _, size := range collectionSizes {
		if size < 0 || size > math.MaxInt-capacity {
			return 0, errors.New("presentation capability count overflows int")
		}
		capacity += size
	}
	return capacity, nil
}

func presentationCapabilityToken(value string) string {
	var builder strings.Builder
	for index, current := range value {
		if current >= 'A' && current <= 'Z' {
			if index > 0 {
				builder.WriteByte('-')
			}
			builder.WriteByte(byte(current - 'A' + 'a'))
			continue
		}
		builder.WriteRune(current)
	}
	return builder.String()
}

func canonicalPresentationJSONDigest(raw []byte) (string, error) {
	if err := rejectDuplicateJSONKeys(raw); err != nil {
		return "", err
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return "", err
	}
	if err := ensureJSONEOF(decoder); err != nil {
		return "", err
	}
	canonical, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return digest(canonical), nil
}

func parsePresentationPageInventory(data []byte) (map[string]presentationInventoryIdentity, error) {
	if err := validateStrictYAMLDocument(data); err != nil {
		return nil, err
	}
	var document yaml.Node
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	if err := decoder.Decode(&document); err != nil {
		return nil, err
	}
	var extra yaml.Node
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return nil, errors.New("multiple YAML documents are not supported")
		}
		return nil, err
	}
	root, err := presentationYAMLDocumentMapping(&document)
	if err != nil {
		return nil, err
	}
	if scalar, ok := presentationYAMLScalar(presentationYAMLMappingValue(root, "apiVersion")); !ok || scalar != "mss.io/v1alpha1" {
		return nil, errors.New("page inventory apiVersion must equal mss.io/v1alpha1")
	}
	if scalar, ok := presentationYAMLScalar(presentationYAMLMappingValue(root, "kind")); !ok || scalar != "AdminPresentationPageInventory" {
		return nil, errors.New("page inventory kind must equal AdminPresentationPageInventory")
	}
	spec := presentationYAMLMappingValue(root, "spec")
	pages := presentationYAMLMappingValue(spec, "pages")
	if pages == nil || pages.Kind != yaml.SequenceNode {
		return nil, errors.New("page inventory spec.pages must be a sequence")
	}
	result := make(map[string]presentationInventoryIdentity)
	for index, page := range pages.Content {
		if page.Kind != yaml.MappingNode {
			return nil, fmt.Errorf("page inventory entry %d must be a mapping", index)
		}
		disposition, ok := presentationYAMLScalar(presentationYAMLMappingValue(page, "disposition"))
		if !ok {
			return nil, fmt.Errorf("page inventory entry %d disposition must be a string", index)
		}
		if disposition != "included" {
			continue
		}
		pageKey, ok := presentationYAMLScalar(presentationYAMLMappingValue(page, "pageKey"))
		if !ok || strings.TrimSpace(pageKey) == "" {
			return nil, fmt.Errorf("included page inventory entry %d has no page key", index)
		}
		sourcePath, ok := presentationYAMLScalar(presentationYAMLMappingValue(page, "sourcePath"))
		if !ok || !safeRelativePath(sourcePath) || sourcePath != normalizedPath(sourcePath) || !strings.HasPrefix(sourcePath, ".mss/core-pages/") {
			return nil, fmt.Errorf("included page inventory entry %s has an invalid core source path", pageKey)
		}
		identity := presentationYAMLMappingValue(page, "definitionIdentity")
		backendHash, backendOK := presentationYAMLScalar(presentationYAMLMappingValue(identity, "backendHash"))
		frontendHash, frontendOK := presentationYAMLScalar(presentationYAMLMappingValue(identity, "frontendHash"))
		if !backendOK || !frontendOK || !validPresentationDefinitionHash(backendHash) || !validPresentationDefinitionHash(frontendHash) {
			return nil, fmt.Errorf("included page inventory entry %s has invalid backend/frontend hashes", pageKey)
		}
		if _, exists := result[pageKey]; exists {
			return nil, fmt.Errorf("page inventory duplicates included page key %s", pageKey)
		}
		result[pageKey] = presentationInventoryIdentity{BackendHash: backendHash, FrontendHash: frontendHash, SourcePath: sourcePath}
	}
	if len(result) == 0 {
		return nil, errors.New("page inventory contains no included presentation pages")
	}
	return result, nil
}

func presentationYAMLDocumentMapping(document *yaml.Node) (*yaml.Node, error) {
	if document == nil || document.Kind != yaml.DocumentNode || len(document.Content) != 1 || document.Content[0].Kind != yaml.MappingNode {
		return nil, errors.New("page inventory must be one YAML mapping document")
	}
	return document.Content[0], nil
}

func presentationYAMLMappingValue(mapping *yaml.Node, key string) *yaml.Node {
	if mapping == nil || mapping.Kind != yaml.MappingNode {
		return nil
	}
	for index := 0; index+1 < len(mapping.Content); index += 2 {
		if mapping.Content[index].Value == key {
			return mapping.Content[index+1]
		}
	}
	return nil
}

func presentationYAMLScalar(node *yaml.Node) (string, bool) {
	if node == nil || node.Kind != yaml.ScalarNode || node.Tag != "!!str" {
		return "", false
	}
	return strings.TrimSpace(node.Value), true
}

func presentationPageIdentityDigest(values map[string]PresentationPageIdentity) (string, error) {
	keys := make([]string, 0, len(values))
	for pageKey := range values {
		keys = append(keys, pageKey)
	}
	sort.Strings(keys)
	identities := make([]PresentationPageIdentity, 0, len(keys))
	for _, pageKey := range keys {
		identity := values[pageKey]
		if identity.PageKey != pageKey || identity.DefinitionVersion == "" || !validPresentationDefinitionHash(identity.DefinitionHash) {
			return "", fmt.Errorf("invalid presentation page identity for %s", pageKey)
		}
		identities = append(identities, identity)
	}
	data, err := json.Marshal(identities)
	if err != nil {
		return "", err
	}
	return digest(data), nil
}

func equalPresentationHashMaps(left, right map[string]string) bool {
	if len(left) != len(right) {
		return false
	}
	for key, value := range left {
		if right[key] != value {
			return false
		}
	}
	return true
}

func validPresentationDefinitionHash(value string) bool {
	return strings.HasPrefix(value, "sha256:") && sha256Pattern.MatchString(strings.TrimPrefix(value, "sha256:"))
}

func renderPresentationSnapshot(snapshot presentationSnapshot) ([]byte, error) {
	if err := validatePresentationSnapshot(snapshot); err != nil {
		return nil, err
	}
	data, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

func decodePresentationSnapshot(data []byte) (presentationSnapshot, error) {
	if err := rejectDuplicateJSONKeys(data); err != nil {
		return presentationSnapshot{}, fmt.Errorf("parse Admin presentation snapshot: %w", err)
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	snapshot := presentationSnapshot{}
	if err := decoder.Decode(&snapshot); err != nil {
		return presentationSnapshot{}, fmt.Errorf("parse Admin presentation snapshot: %w", err)
	}
	if err := ensureJSONEOF(decoder); err != nil {
		return presentationSnapshot{}, fmt.Errorf("parse Admin presentation snapshot: %w", err)
	}
	if err := validatePresentationSnapshot(snapshot); err != nil {
		return presentationSnapshot{}, err
	}
	return snapshot, nil
}

func validatePresentationSnapshot(snapshot presentationSnapshot) error {
	if snapshot.APIVersion != presentationSnapshotVersion || snapshot.Kind != presentationSnapshotKind {
		return errors.New("unsupported Admin presentation snapshot identity")
	}
	if !sha256Pattern.MatchString(snapshot.BackendInventorySHA256) || !sha256Pattern.MatchString(snapshot.FrontendInventorySHA256) {
		return errors.New("Admin presentation inventory digests must be SHA-256 digests")
	}
	if len(snapshot.Pages) == 0 {
		return errors.New("Admin presentation snapshot pages must not be empty")
	}
	previousPage := ""
	pageIdentities := make(map[string]PresentationPageIdentity, len(snapshot.Pages))
	for _, page := range snapshot.Pages {
		if page.PageKey == "" || page.PageKey <= previousPage || page.DefinitionVersion == "" || !validPresentationDefinitionHash(page.DefinitionHash) {
			return errors.New("Admin presentation snapshot pages must be uniquely sorted with valid identities")
		}
		previousPage = page.PageKey
		pageIdentities[page.PageKey] = page.PresentationPageIdentity
		previousCapability := ""
		if len(page.Capabilities) == 0 {
			return fmt.Errorf("Admin presentation snapshot page %s has no capabilities", page.PageKey)
		}
		for _, capability := range page.Capabilities {
			if capability.ID == "" || capability.ID <= previousCapability || !strings.HasPrefix(capability.ID, page.PageKey+"/") || !sha256Pattern.MatchString(capability.SHA256) {
				return fmt.Errorf("Admin presentation snapshot page %s has invalid or unsorted capability identities", page.PageKey)
			}
			previousCapability = capability.ID
		}
	}
	computedBackendDigest, err := presentationPageIdentityDigest(pageIdentities)
	if err != nil {
		return err
	}
	if computedBackendDigest != snapshot.BackendInventorySHA256 {
		return errors.New("Admin presentation backend inventory digest contradicts its page identities")
	}
	if snapshot.BackendFrontendInventoriesMatch != (snapshot.BackendInventorySHA256 == snapshot.FrontendInventorySHA256) {
		return errors.New("Admin presentation inventory parity contradicts its backend/frontend digests")
	}
	return nil
}

func buildPresentationUpgradeImpact(applicationRoot string, oldManifest Manifest, desired map[string]desiredFile) (PresentationUpgradeImpact, error) {
	fromState, fromSnapshot, err := readCurrentPresentationSnapshot(applicationRoot, oldManifest)
	if err != nil {
		return PresentationUpgradeImpact{}, err
	}
	toState := presentationSnapshotUnavailable
	toSnapshot := presentationSnapshot{}
	if file, exists := desired[presentationSnapshotPath]; exists {
		toSnapshot, err = decodePresentationSnapshot(file.Data)
		if err != nil {
			return PresentationUpgradeImpact{}, fmt.Errorf("read target Admin presentation snapshot: %w", err)
		}
		toState = presentationSnapshotAvailable
	}
	impact := PresentationUpgradeImpact{
		From:                     presentationUpgradeSnapshotSummary(fromState, fromSnapshot),
		To:                       presentationUpgradeSnapshotSummary(toState, toSnapshot),
		ComparisonComplete:       fromState == presentationSnapshotAvailable && toState == presentationSnapshotAvailable,
		AddedCapabilityIDs:       make([]string, 0),
		RemovedCapabilityIDs:     make([]string, 0),
		ChangedCapabilityIDs:     make([]string, 0),
		PotentiallyStalePageKeys: make([]string, 0),
	}
	fromCapabilities := presentationCapabilityHashMap(fromSnapshot)
	toCapabilities := presentationCapabilityHashMap(toSnapshot)
	for id, hash := range toCapabilities {
		fromHash, exists := fromCapabilities[id]
		switch {
		case !exists:
			impact.AddedCapabilityIDs = append(impact.AddedCapabilityIDs, id)
		case fromHash != hash:
			impact.ChangedCapabilityIDs = append(impact.ChangedCapabilityIDs, id)
		}
	}
	for id := range fromCapabilities {
		if _, exists := toCapabilities[id]; !exists {
			impact.RemovedCapabilityIDs = append(impact.RemovedCapabilityIDs, id)
		}
	}
	fromPages := presentationPageHashMap(fromSnapshot)
	toPages := presentationPageHashMap(toSnapshot)
	stale := map[string]bool{}
	if !impact.ComparisonComplete {
		for pageKey := range fromPages {
			stale[pageKey] = true
		}
		for pageKey := range toPages {
			stale[pageKey] = true
		}
	} else {
		for pageKey, fromIdentity := range fromPages {
			toIdentity, exists := toPages[pageKey]
			if !exists || fromIdentity != toIdentity {
				stale[pageKey] = true
			}
		}
		// A newly compiled page may collide with a dormant downstream profile
		// left by an older Distribution. Without reading the downstream DB the
		// source-only plan cannot prove otherwise, so it reports the page key.
		for pageKey := range toPages {
			if _, exists := fromPages[pageKey]; !exists {
				stale[pageKey] = true
			}
		}
	}
	for pageKey := range stale {
		impact.PotentiallyStalePageKeys = append(impact.PotentiallyStalePageKeys, pageKey)
	}
	sort.Strings(impact.AddedCapabilityIDs)
	sort.Strings(impact.RemovedCapabilityIDs)
	sort.Strings(impact.ChangedCapabilityIDs)
	sort.Strings(impact.PotentiallyStalePageKeys)
	return impact, nil
}

func readCurrentPresentationSnapshot(applicationRoot string, oldManifest Manifest) (string, presentationSnapshot, error) {
	baseline, recorded := oldManifest.Files[presentationSnapshotPath]
	if !recorded {
		return presentationSnapshotUnrecorded, presentationSnapshot{}, nil
	}
	data, exists, err := readManagedFile(applicationRoot, presentationSnapshotPath)
	if err != nil {
		return "", presentationSnapshot{}, fmt.Errorf("read current Admin presentation snapshot: %w", err)
	}
	if !exists {
		return presentationSnapshotMissing, presentationSnapshot{}, nil
	}
	if digest(data) != baseline.SHA256 {
		return presentationSnapshotModified, presentationSnapshot{}, nil
	}
	snapshot, err := decodePresentationSnapshot(data)
	if err != nil {
		return "", presentationSnapshot{}, fmt.Errorf("read current Admin presentation snapshot: %w", err)
	}
	return presentationSnapshotAvailable, snapshot, nil
}

func presentationUpgradeSnapshotSummary(state string, snapshot presentationSnapshot) PresentationUpgradeSnapshotSummary {
	pages := make([]PresentationPageIdentity, 0, len(snapshot.Pages))
	for _, page := range snapshot.Pages {
		pages = append(pages, page.PresentationPageIdentity)
	}
	return PresentationUpgradeSnapshotSummary{
		State: state, BackendFrontendInventoriesMatch: snapshot.BackendFrontendInventoriesMatch, Pages: pages,
	}
}

func presentationCapabilityHashMap(snapshot presentationSnapshot) map[string]string {
	result := make(map[string]string)
	for _, page := range snapshot.Pages {
		for _, capability := range page.Capabilities {
			result[capability.ID] = capability.SHA256
		}
	}
	return result
}

func presentationPageHashMap(snapshot presentationSnapshot) map[string]string {
	result := make(map[string]string, len(snapshot.Pages))
	for _, page := range snapshot.Pages {
		result[page.PageKey] = page.DefinitionVersion + "\x00" + page.DefinitionHash
	}
	return result
}

func presentationImpactText(impact PresentationUpgradeImpact) string {
	var builder strings.Builder
	builder.WriteString("presentation upgrade impact:\n")
	fmt.Fprintf(&builder, "- snapshots: %s -> %s (complete comparison: %t)\n", impact.From.State, impact.To.State, impact.ComparisonComplete)
	fmt.Fprintf(
		&builder,
		"- backend/frontend inventories match: %t -> %t\n",
		impact.From.BackendFrontendInventoriesMatch,
		impact.To.BackendFrontendInventoriesMatch,
	)
	fmt.Fprintf(&builder, "- pages: %s -> %s\n", presentationPageIdentityText(impact.From.Pages), presentationPageIdentityText(impact.To.Pages))
	fmt.Fprintf(&builder, "- added capability ids: %s\n", presentationStringListText(impact.AddedCapabilityIDs))
	fmt.Fprintf(&builder, "- removed capability ids: %s\n", presentationStringListText(impact.RemovedCapabilityIDs))
	fmt.Fprintf(&builder, "- changed capability ids: %s\n", presentationStringListText(impact.ChangedCapabilityIDs))
	fmt.Fprintf(&builder, "- potentially stale page keys: %s\n", presentationStringListText(impact.PotentiallyStalePageKeys))
	return builder.String()
}

func presentationPageIdentityText(pages []PresentationPageIdentity) string {
	if len(pages) == 0 {
		return "none"
	}
	values := make([]string, 0, len(pages))
	for _, page := range pages {
		values = append(values, page.PageKey+"@"+page.DefinitionVersion+"/"+page.DefinitionHash)
	}
	return strings.Join(values, ", ")
}

func presentationStringListText(values []string) string {
	if len(values) == 0 {
		return "none"
	}
	return strings.Join(values, ", ")
}
