package spec

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// DocumentHeader is the common identity shared by project specifications.
type DocumentHeader struct {
	APIVersion string `yaml:"apiVersion" json:"apiVersion"`
	Kind       string `yaml:"kind" json:"kind"`
}

// ValidatedDocument is a stable CLI and MCP result independent of specification kind.
type ValidatedDocument struct {
	Path       string         `json:"path"`
	APIVersion string         `json:"apiVersion"`
	Kind       string         `json:"kind"`
	Name       string         `json:"name"`
	Summary    map[string]any `json:"summary"`
	Document   any            `json:"document"`
}

// InspectHeader reads only the common identity fields without accepting unknown content.
func InspectHeader(path string) (DocumentHeader, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return DocumentHeader{}, fmt.Errorf("read specification header: %w", err)
	}
	header := DocumentHeader{}
	if err := yaml.Unmarshal(data, &header); err != nil {
		return DocumentHeader{}, fmt.Errorf("parse specification header: %w", err)
	}
	return header, nil
}

// ValidateFile dispatches to the strict semantic validator selected by kind.
func ValidateFile(path string) (*ValidatedDocument, error) {
	header, err := InspectHeader(path)
	if err != nil {
		return nil, err
	}
	cleanPath := filepath.ToSlash(path)
	switch header.Kind {
	case AdminPresentationPageInventoryKind:
		inventory, loadErr := LoadAdminPresentationPageInventory(path)
		if loadErr != nil {
			return nil, loadErr
		}
		inventory.SourcePath = cleanPath
		return &ValidatedDocument{
			Path:       cleanPath,
			APIVersion: inventory.APIVersion,
			Kind:       inventory.Kind,
			Name:       inventory.Metadata.Name,
			Summary:    inventory.Summary(),
			Document:   inventory,
		}, nil
	case CorePagePresentationKind:
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil, fmt.Errorf("read core page presentation: %w", readErr)
		}
		document, parseErr := ParseCorePagePresentation(data, cleanPath)
		if parseErr != nil {
			return nil, parseErr
		}
		return &ValidatedDocument{
			Path:       cleanPath,
			APIVersion: document.APIVersion,
			Kind:       document.Kind,
			Name:       document.Metadata.Name,
			Summary: map[string]any{
				"binding": document.Spec.Binding,
				"pageKey": document.Spec.PageKey,
				"version": document.Spec.DefinitionVersion,
			},
			Document: document,
		}, nil
	case "AdminModule":
		module, err := LoadModule(path)
		if err != nil {
			return nil, err
		}
		module.SourcePath = cleanPath
		return &ValidatedDocument{
			Path:       cleanPath,
			APIVersion: module.APIVersion,
			Kind:       module.Kind,
			Name:       module.Metadata.Name,
			Summary: map[string]any{
				"displayName": module.Metadata.DisplayName,
				"fields":      len(module.Spec.Entity.Fields),
				"permissions": len(module.Spec.Permissions),
				"menu":        module.Spec.Menu.Path,
				"tests":       module.Spec.Tests,
			},
			Document: module,
		}, nil
	case "Feature":
		feature, err := LoadFeature(path)
		if err != nil {
			return nil, err
		}
		feature.SourcePath = cleanPath
		return &ValidatedDocument{
			Path:       cleanPath,
			APIVersion: feature.APIVersion,
			Kind:       feature.Kind,
			Name:       feature.Metadata.Name,
			Summary:    feature.Summary(),
			Document:   feature,
		}, nil
	case "":
		return nil, fmt.Errorf("specification %s has no kind", cleanPath)
	default:
		return nil, fmt.Errorf("unsupported specification kind %q in %s", strings.TrimSpace(header.Kind), cleanPath)
	}
}
