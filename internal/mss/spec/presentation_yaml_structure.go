package spec

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

type presentationYAMLValidator func(string, *yaml.Node, *[]Issue)

func validatePresentationModuleYAMLAncestors(node *yaml.Node) []Issue {
	issues := make([]Issue, 0)
	if !presentationYAMLRequireMapping("", node, &issues) {
		return issues
	}
	specNode := presentationYAMLMappingValue(node, "spec")
	presentationYAMLRequireMapping("spec", specNode, &issues)
	return issues
}

func validatePresentationYAMLStructure(node *yaml.Node) []Issue {
	issues := make([]Issue, 0)
	const path = "spec.presentation"
	if !presentationYAMLRequireMapping(path, node, &issues) {
		return issues
	}
	presentationYAMLValidateProperty(path, node, "pageKey", presentationYAMLToken, &issues)
	presentationYAMLValidateProperty(path, node, "definitionVersion", presentationYAMLToken, &issues)
	presentationYAMLValidateProperty(path, node, "title", presentationYAMLLocalizedText, &issues)
	presentationYAMLValidateProperty(path, node, "dataSource", presentationYAMLToken, &issues)
	presentationYAMLValidateProperty(path, node, "list", presentationYAMLList, &issues)
	presentationYAMLValidateProperty(path, node, "search", presentationYAMLSearch, &issues)
	presentationYAMLValidateProperty(path, node, "form", presentationYAMLForm, &issues)
	presentationYAMLValidateProperty(path, node, "detail", presentationYAMLDetail, &issues)
	presentationYAMLValidateProperty(path, node, "actions", presentationYAMLActions, &issues)
	return issues
}

func validatePresentationCatalogYAMLStructure(node *yaml.Node) []Issue {
	issues := make([]Issue, 0)
	if !presentationYAMLRequireMapping("", node, &issues) {
		return issues
	}
	presentationYAMLValidateProperty("", node, "apiVersion", presentationYAMLToken, &issues)
	presentationYAMLValidateProperty("", node, "kind", presentationYAMLToken, &issues)
	presentationYAMLValidateProperty("", node, "metadata", presentationYAMLCatalogMetadata, &issues)
	presentationYAMLValidateProperty("", node, "spec", presentationYAMLCatalogSpec, &issues)
	return issues
}

func presentationYAMLList(path string, node *yaml.Node, issues *[]Issue) {
	if !presentationYAMLRequireMapping(path, node, issues) {
		return
	}
	presentationYAMLValidateProperty(path, node, "density", presentationYAMLToken, issues)
	presentationYAMLValidateProperty(path, node, "pageSize", presentationYAMLInteger, issues)
	presentationYAMLValidateProperty(path, node, "defaultSort", presentationYAMLSorts, issues)
	presentationYAMLValidateProperty(path, node, "fields", presentationYAMLFields, issues)
}

func presentationYAMLSearch(path string, node *yaml.Node, issues *[]Issue) {
	if !presentationYAMLRequireMapping(path, node, issues) {
		return
	}
	presentationYAMLValidateProperty(path, node, "collapsedByDefault", presentationYAMLBoolean, issues)
	presentationYAMLValidateProperty(path, node, "fields", presentationYAMLFields, issues)
}

func presentationYAMLForm(path string, node *yaml.Node, issues *[]Issue) {
	if !presentationYAMLRequireMapping(path, node, issues) {
		return
	}
	presentationYAMLValidateProperty(path, node, "columns", presentationYAMLInteger, issues)
	presentationYAMLValidateProperty(path, node, "fields", presentationYAMLFields, issues)
}

func presentationYAMLDetail(path string, node *yaml.Node, issues *[]Issue) {
	presentationYAMLForm(path, node, issues)
}

func presentationYAMLFields(path string, node *yaml.Node, issues *[]Issue) {
	items, ok := presentationYAMLRequireSequence(path, node, issues)
	if !ok {
		return
	}
	for index, item := range items {
		presentationYAMLField(fmt.Sprintf("%s[%d]", path, index), item, issues)
	}
}

func presentationYAMLField(path string, node *yaml.Node, issues *[]Issue) {
	if !presentationYAMLRequireMapping(path, node, issues) {
		return
	}
	presentationYAMLValidateProperty(path, node, "field", presentationYAMLToken, issues)
	presentationYAMLValidateProperty(path, node, "label", presentationYAMLLocalizedText, issues)
	presentationYAMLValidateProperty(path, node, "component", presentationYAMLToken, issues)
	presentationYAMLValidateProperty(path, node, "allowedComponents", presentationYAMLTokenSequence, issues)
	presentationYAMLValidateProperty(path, node, "order", presentationYAMLInteger, issues)
	presentationYAMLValidateProperty(path, node, "hidden", presentationYAMLBoolean, issues)
	presentationYAMLValidateProperty(path, node, "width", presentationYAMLInteger, issues)
	presentationYAMLValidateProperty(path, node, "span", presentationYAMLInteger, issues)
	presentationYAMLValidateProperty(path, node, "placeholder", presentationYAMLLocalizedText, issues)
	presentationYAMLValidateProperty(path, node, "help", presentationYAMLLocalizedText, issues)
}

func presentationYAMLSorts(path string, node *yaml.Node, issues *[]Issue) {
	items, ok := presentationYAMLRequireSequence(path, node, issues)
	if !ok {
		return
	}
	for index, item := range items {
		itemPath := fmt.Sprintf("%s[%d]", path, index)
		if !presentationYAMLRequireMapping(itemPath, item, issues) {
			continue
		}
		presentationYAMLValidateProperty(itemPath, item, "field", presentationYAMLToken, issues)
		presentationYAMLValidateProperty(itemPath, item, "direction", presentationYAMLToken, issues)
	}
}

func presentationYAMLActions(path string, node *yaml.Node, issues *[]Issue) {
	items, ok := presentationYAMLRequireSequence(path, node, issues)
	if !ok {
		return
	}
	for index, item := range items {
		itemPath := fmt.Sprintf("%s[%d]", path, index)
		if !presentationYAMLRequireMapping(itemPath, item, issues) {
			continue
		}
		presentationYAMLValidateProperty(itemPath, item, "action", presentationYAMLToken, issues)
		presentationYAMLValidateProperty(itemPath, item, "label", presentationYAMLLocalizedText, issues)
		presentationYAMLValidateProperty(itemPath, item, "placement", presentationYAMLToken, issues)
		presentationYAMLValidateProperty(itemPath, item, "order", presentationYAMLInteger, issues)
		presentationYAMLValidateProperty(itemPath, item, "hidden", presentationYAMLBoolean, issues)
		presentationYAMLValidateProperty(itemPath, item, "confirm", presentationYAMLLocalizedText, issues)
	}
}

func presentationYAMLLocalizedText(path string, node *yaml.Node, issues *[]Issue) {
	if !presentationYAMLRequireMapping(path, node, issues) {
		return
	}
	presentationYAMLValidateProperty(path, node, "zh-CN", presentationYAMLString, issues)
	presentationYAMLValidateProperty(path, node, "en-US", presentationYAMLString, issues)
}

func presentationYAMLCatalogMetadata(path string, node *yaml.Node, issues *[]Issue) {
	if !presentationYAMLRequireMapping(path, node, issues) {
		return
	}
	presentationYAMLValidateProperty(path, node, "name", presentationYAMLToken, issues)
	presentationYAMLValidateProperty(path, node, "definitionVersion", presentationYAMLToken, issues)
}

func presentationYAMLCatalogSpec(path string, node *yaml.Node, issues *[]Issue) {
	if !presentationYAMLRequireMapping(path, node, issues) {
		return
	}
	presentationYAMLValidateProperty(path, node, "components", presentationYAMLCatalogComponents, issues)
	presentationYAMLValidateProperty(path, node, "dataSources", presentationYAMLCatalogDataSources, issues)
	presentationYAMLValidateProperty(path, node, "actions", presentationYAMLCatalogActions, issues)
}

func presentationYAMLCatalogComponents(path string, node *yaml.Node, issues *[]Issue) {
	items, ok := presentationYAMLRequireSequence(path, node, issues)
	if !ok {
		return
	}
	for index, item := range items {
		itemPath := fmt.Sprintf("%s[%d]", path, index)
		if !presentationYAMLRequireMapping(itemPath, item, issues) {
			continue
		}
		presentationYAMLValidateProperty(itemPath, item, "id", presentationYAMLToken, issues)
		presentationYAMLValidateProperty(itemPath, item, "valueTypes", presentationYAMLTokenSequence, issues)
		presentationYAMLValidateProperty(itemPath, item, "formats", presentationYAMLTokenSequence, issues)
		presentationYAMLValidateProperty(itemPath, item, "surfaces", presentationYAMLTokenSequence, issues)
		presentationYAMLValidateProperty(itemPath, item, "readOnly", presentationYAMLToken, issues)
	}
}

func presentationYAMLCatalogDataSources(path string, node *yaml.Node, issues *[]Issue) {
	items, ok := presentationYAMLRequireSequence(path, node, issues)
	if !ok {
		return
	}
	for index, item := range items {
		itemPath := fmt.Sprintf("%s[%d]", path, index)
		if !presentationYAMLRequireMapping(itemPath, item, issues) {
			continue
		}
		presentationYAMLValidateProperty(itemPath, item, "id", presentationYAMLToken, issues)
		presentationYAMLValidateProperty(itemPath, item, "apiOperation", presentationYAMLToken, issues)
		presentationYAMLValidateProperty(itemPath, item, "permissionAction", presentationYAMLToken, issues)
		presentationYAMLValidateProperty(itemPath, item, "pageSizeOptions", presentationYAMLIntegerSequence, issues)
		presentationYAMLValidateProperty(itemPath, item, "maxPageSize", presentationYAMLInteger, issues)
		presentationYAMLValidateProperty(itemPath, item, "maxSortFields", presentationYAMLInteger, issues)
	}
}

func presentationYAMLCatalogActions(path string, node *yaml.Node, issues *[]Issue) {
	items, ok := presentationYAMLRequireSequence(path, node, issues)
	if !ok {
		return
	}
	for index, item := range items {
		itemPath := fmt.Sprintf("%s[%d]", path, index)
		if !presentationYAMLRequireMapping(itemPath, item, issues) {
			continue
		}
		presentationYAMLValidateProperty(itemPath, item, "id", presentationYAMLToken, issues)
		presentationYAMLValidateProperty(itemPath, item, "apiOperation", presentationYAMLToken, issues)
		presentationYAMLValidateProperty(itemPath, item, "permissionAction", presentationYAMLToken, issues)
		presentationYAMLValidateProperty(itemPath, item, "placements", presentationYAMLTokenSequence, issues)
		presentationYAMLValidateProperty(itemPath, item, "destructive", presentationYAMLBoolean, issues)
	}
}

func presentationYAMLTokenSequence(path string, node *yaml.Node, issues *[]Issue) {
	items, ok := presentationYAMLRequireSequence(path, node, issues)
	if !ok {
		return
	}
	for index, item := range items {
		presentationYAMLToken(fmt.Sprintf("%s[%d]", path, index), item, issues)
	}
}

func presentationYAMLIntegerSequence(path string, node *yaml.Node, issues *[]Issue) {
	items, ok := presentationYAMLRequireSequence(path, node, issues)
	if !ok {
		return
	}
	for index, item := range items {
		presentationYAMLInteger(fmt.Sprintf("%s[%d]", path, index), item, issues)
	}
}

func presentationYAMLString(path string, node *yaml.Node, issues *[]Issue) {
	presentationYAMLRequireScalar(path, node, "!!str", "string", issues)
}

func presentationYAMLToken(path string, node *yaml.Node, issues *[]Issue) {
	if !presentationYAMLRequireScalar(path, node, "!!str", "string", issues) {
		return
	}
	node = presentationYAMLDereference(node)
	if node.Value != strings.TrimSpace(node.Value) {
		*issues = append(*issues, Issue{
			Path: path, Code: "yaml-token-whitespace",
			Message: "schema token strings must not contain leading or trailing whitespace",
		})
	}
}

func presentationYAMLBoolean(path string, node *yaml.Node, issues *[]Issue) {
	presentationYAMLRequireScalar(path, node, "!!bool", "boolean", issues)
}

func presentationYAMLInteger(path string, node *yaml.Node, issues *[]Issue) {
	presentationYAMLRequireScalar(path, node, "!!int", "integer", issues)
}

func presentationYAMLRequireScalar(path string, node *yaml.Node, tag, expected string, issues *[]Issue) bool {
	node = presentationYAMLDereference(node)
	if node != nil && node.Kind == yaml.ScalarNode && node.ShortTag() == tag {
		return true
	}
	presentationYAMLAddTypeIssue(path, expected, issues)
	return false
}

func presentationYAMLRequireMapping(path string, node *yaml.Node, issues *[]Issue) bool {
	node = presentationYAMLDereference(node)
	if node != nil && node.Kind == yaml.MappingNode && node.ShortTag() == "!!map" {
		return true
	}
	presentationYAMLAddTypeIssue(path, "mapping", issues)
	return false
}

func presentationYAMLRequireSequence(path string, node *yaml.Node, issues *[]Issue) ([]*yaml.Node, bool) {
	node = presentationYAMLDereference(node)
	if node == nil || node.Kind != yaml.SequenceNode || node.ShortTag() != "!!seq" {
		presentationYAMLAddTypeIssue(path, "sequence", issues)
		return nil, false
	}
	return presentationYAMLSequenceItems(node), true
}

func presentationYAMLValidateProperty(
	path string,
	mapping *yaml.Node,
	key string,
	validator presentationYAMLValidator,
	issues *[]Issue,
) {
	if node := presentationYAMLMappingValue(mapping, key); node != nil {
		validator(presentationYAMLPath(path, key), node, issues)
	}
}

func presentationYAMLPath(parent, child string) string {
	if parent == "" {
		return child
	}
	return parent + "." + child
}

func presentationYAMLAddTypeIssue(path, expected string, issues *[]Issue) {
	*issues = append(*issues, Issue{
		Path: path, Code: "yaml-type-mismatch",
		Message: "must be an explicit YAML " + expected + " matching the schema type",
	})
}
