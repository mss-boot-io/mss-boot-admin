package generator

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"go/format"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/mss-boot-io/mss-boot-admin/internal/mss/spec"
)

const corePresentationTemplateRevision = "1.3.7-core-presentation-v2.1"

type corePresentationRegistryEntry struct {
	Source     string
	Document   *spec.CorePagePresentation
	Projection presentationProjection
}

type corePresentationSnapshot struct {
	Generated string                   `json:"$generated"`
	Sources   []string                 `json:"sources"`
	Manifests []presentationProjection `json:"manifests"`
}

func corePresentationOutputGroupPaths(layout targetLayout) []string {
	return []string{
		filepath.ToSlash(filepath.Join(filepath.FromSlash(layout.BackendDir), "presentation", "core", "definitions_generated.go")),
		filepath.ToSlash(filepath.Join(filepath.FromSlash(layout.BackendDir), "presentation", "core", "manifest.generated.json")),
		filepath.ToSlash(filepath.Join(filepath.FromSlash(layout.GeneratedDir), "core-presentation-registry.generated.ts")),
	}
}

func validateCorePresentationOutputGroup(repository *os.Root, layout targetLayout, enabled bool) error {
	if layout.Kind != layoutFoundation || !enabled {
		return nil
	}
	return validateManagedOutputGroup(repository, "core-presentation", corePresentationOutputGroupPaths(layout))
}

func renderCorePresentationOutputs(repository *os.Root, layout targetLayout) ([]output, error) {
	// Thin Hosts consume Foundation core definitions from the pinned Admin and
	// Admin Web distributions. They must never copy core source or generated
	// Foundation output into application-owned paths.
	if layout.Kind != layoutFoundation {
		return nil, nil
	}
	pattern := filepath.ToSlash(filepath.Join(filepath.FromSlash(layout.SpecificationsDir), "core-pages", "*.yaml"))
	matches, err := fs.Glob(repository.FS(), pattern)
	if err != nil {
		return nil, fmt.Errorf("discover core page presentation sources: %w", err)
	}
	if len(matches) == 0 {
		return nil, nil
	}
	sort.Strings(matches)
	entries := make([]corePresentationRegistryEntry, 0, len(matches))
	pageKeys := make(map[string]string, len(matches))
	for _, match := range matches {
		source := filepath.ToSlash(match)
		data, readErr := repository.ReadFile(filepath.FromSlash(source))
		if readErr != nil {
			return nil, fmt.Errorf("read core page presentation %s: %w", source, readErr)
		}
		document, parseErr := spec.ParseCorePagePresentation(data, source)
		if parseErr != nil {
			return nil, parseErr
		}
		manifest, normalizeErr := document.NormalizePresentation()
		if normalizeErr != nil {
			return nil, fmt.Errorf("normalize core page presentation %s: %w", source, normalizeErr)
		}
		raw, marshalErr := encodePresentationJSON(manifest, "")
		if marshalErr != nil {
			return nil, fmt.Errorf("marshal core page presentation %s: %w", source, marshalErr)
		}
		var projection presentationProjection
		if unmarshalErr := json.Unmarshal(raw, &projection); unmarshalErr != nil {
			return nil, fmt.Errorf("project core page presentation %s: %w", source, unmarshalErr)
		}
		canonical, canonicalErr := manifest.CanonicalJSON()
		if canonicalErr != nil {
			return nil, fmt.Errorf("canonicalize core page presentation %s: %w", source, canonicalErr)
		}
		projectedCanonical, projectionErr := canonicalPresentationProjection(&projection)
		if projectionErr != nil {
			return nil, fmt.Errorf("canonicalize core page projection %s: %w", source, projectionErr)
		}
		if !bytes.Equal(canonical, projectedCanonical) {
			return nil, fmt.Errorf("core page projection %s diverges from its canonical manifest", source)
		}
		digest := sha256.Sum256(projectedCanonical)
		wantHash := "sha256:" + hex.EncodeToString(digest[:])
		if projection.DefinitionHash != wantHash {
			return nil, fmt.Errorf("core page projection %s hash = %s, want %s", source, projection.DefinitionHash, wantHash)
		}
		if previous, duplicate := pageKeys[projection.PageKey]; duplicate {
			return nil, fmt.Errorf("duplicate core presentation page key %s in %s and %s", projection.PageKey, previous, source)
		}
		pageKeys[projection.PageKey] = source
		entries = append(entries, corePresentationRegistryEntry{Source: source, Document: document, Projection: projection})
	}
	sort.SliceStable(entries, func(i, j int) bool {
		return entries[i].Projection.PageKey < entries[j].Projection.PageKey
	})
	return renderCorePresentationRegistry(entries, layout)
}

func renderCorePresentationRegistry(entries []corePresentationRegistryEntry, layout targetLayout) ([]output, error) {
	sources := make([]string, 0, len(entries))
	projections := make([]presentationProjection, 0, len(entries))
	for _, entry := range entries {
		sources = append(sources, entry.Source)
		projections = append(projections, entry.Projection)
	}
	sourceLabel := strings.Join(sources, ", ")
	definitionsJSON, err := encodePresentationJSON(projections, "")
	if err != nil {
		return nil, fmt.Errorf("marshal core presentation definitions: %w", err)
	}

	var backend strings.Builder
	fmt.Fprintf(&backend, "// Code generated by mss core presentation template %s from %s. DO NOT EDIT.\n\n", corePresentationTemplateRevision, sourceLabel)
	backend.WriteString("// Package core exposes the Foundation-owned presentation capabilities for handwritten core pages.\n")
	backend.WriteString("package core\n\n")
	backend.WriteString("import (\n\t\"encoding/json\"\n\n")
	fmt.Fprintf(&backend, "\t%q\n", layout.AdminModule+"/presentation")
	backend.WriteString(")\n\n")
	fmt.Fprintf(&backend, "const definitionsJSON = %s\n\n", strconv.Quote(string(definitionsJSON)))
	backend.WriteString("// Definitions returns a fresh deep copy so callers cannot mutate the generated Foundation inventory.\n")
	backend.WriteString("func Definitions() []presentation.CapabilityDefinition {\n")
	backend.WriteString("\tvar definitions []presentation.CapabilityDefinition\n")
	backend.WriteString("\tif err := json.Unmarshal([]byte(definitionsJSON), &definitions); err != nil {\n")
	backend.WriteString("\t\tpanic(\"decode generated Foundation core presentation definitions: \" + err.Error())\n\t}\n")
	backend.WriteString("\treturn definitions\n}\n")
	formattedBackend, err := format.Source([]byte(backend.String()))
	if err != nil {
		return nil, fmt.Errorf("format generated core presentation definitions: %w", err)
	}

	var frontend strings.Builder
	fmt.Fprintf(&frontend, "// Code generated by mss core presentation template %s from %s. DO NOT EDIT.\n\n", corePresentationTemplateRevision, sourceLabel)
	frontend.WriteString("export const corePresentationInventory = [\n")
	for _, entry := range entries {
		fmt.Fprintf(&frontend, "  %s,\n", strconv.Quote(entry.Projection.PageKey))
	}
	frontend.WriteString("] as const;\n\n")
	frontend.WriteString("export const corePresentationRegistry = {\n")
	for _, entry := range entries {
		pretty, marshalErr := encodePresentationJSON(entry.Projection, "  ")
		if marshalErr != nil {
			return nil, fmt.Errorf("marshal TypeScript core presentation %s: %w", entry.Projection.PageKey, marshalErr)
		}
		fmt.Fprintf(&frontend, "  %s: {\n", strconv.Quote(entry.Projection.PageKey))
		fmt.Fprintf(&frontend, "    definitionHash: %s,\n", strconv.Quote(entry.Projection.DefinitionHash))
		frontend.WriteString("    definition: ")
		frontend.WriteString(indentFollowingLines(string(pretty), "    "))
		frontend.WriteString(" as const,\n")
		frontend.WriteString("  },\n")
	}
	frontend.WriteString("} as const;\n")

	snapshot := corePresentationSnapshot{
		Generated: "Code generated by mss core presentation template " + corePresentationTemplateRevision + " from " + sourceLabel + ". DO NOT EDIT.",
		Sources:   append([]string(nil), sources...), Manifests: projections,
	}
	snapshotJSON, err := encodePresentationJSON(snapshot, "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal core presentation snapshot: %w", err)
	}
	snapshotJSON = append(snapshotJSON, '\n')
	source := strings.Join(sources, ",")
	paths := corePresentationOutputGroupPaths(layout)
	return []output{
		{path: paths[0], content: normalizeNewline(formattedBackend), managed: true, source: source, fileMode: 0o644},
		{path: paths[1], content: normalizeNewline(snapshotJSON), managed: true, source: source, fileMode: 0o644},
		{path: paths[2], content: normalizeNewline([]byte(frontend.String())), managed: true, source: source, fileMode: 0o644},
	}, nil
}

func indentFollowingLines(value, indent string) string {
	return strings.ReplaceAll(value, "\n", "\n"+indent)
}
