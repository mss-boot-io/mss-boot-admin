package spec

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

// trackModulePresentationPresence preserves the distinction between a
// schema-required key that is explicitly set to its legal Go zero value and a
// key that was omitted from YAML. The public source structs intentionally keep
// their value fields for compatibility; omission is recorded only for parsed
// documents and therefore does not invalidate programmatically built specs.
func trackModulePresentationPresence(data []byte, module *Module) error {
	if module == nil {
		return nil
	}
	root, err := presentationYAMLRoot(data)
	if err != nil {
		return err
	}
	if module.Spec.Presentation != nil {
		if issues := validatePresentationModuleYAMLAncestors(root); len(issues) > 0 {
			return &ValidationError{Issues: issues}
		}
		if issues := presentationYAMLMappingKeyIssues("", root); len(issues) > 0 {
			return &ValidationError{Issues: issues}
		}
		if issues := presentationYAMLMergeKeyIssues("", root); len(issues) > 0 {
			return &ValidationError{Issues: issues}
		}
	}
	presentationNode := presentationYAMLMappingValue(
		presentationYAMLMappingValue(root, "spec"),
		"presentation",
	)
	if presentationNode == nil {
		return nil
	}
	if issues := presentationYAMLNullIssues("spec.presentation", presentationNode); len(issues) > 0 {
		return &ValidationError{Issues: issues}
	}
	if issues := validatePresentationYAMLStructure(presentationNode); len(issues) > 0 {
		return &ValidationError{Issues: issues}
	}
	if module.Spec.Presentation == nil {
		return nil
	}
	presentation := module.Spec.Presentation
	searchNode := presentationYAMLMappingValue(presentationNode, "search")
	presentation.Search.collapsedOmitted = presentationYAMLMappingValue(searchNode, "collapsedByDefault") == nil

	trackPresentationLocalizedTextPresence(
		presentationYAMLMappingValue(presentationNode, "title"),
		&presentation.Title,
	)
	trackPresentationFieldListPresence(
		presentationYAMLMappingValue(presentationNode, "list"),
		presentation.List.Fields,
	)
	trackPresentationFieldListPresence(searchNode, presentation.Search.Fields)
	trackPresentationFieldListPresence(
		presentationYAMLMappingValue(presentationNode, "form"),
		presentation.Form.Fields,
	)
	trackPresentationFieldListPresence(
		presentationYAMLMappingValue(presentationNode, "detail"),
		presentation.Detail.Fields,
	)
	trackPresentationActionListPresence(
		presentationYAMLMappingValue(presentationNode, "actions"),
		presentation.Actions,
	)
	return nil
}

func trackPresentationCatalogPresence(data []byte, catalog *PresentationCatalog) error {
	if catalog == nil {
		return nil
	}
	root, err := presentationYAMLRoot(data)
	if err != nil {
		return err
	}
	if issues := presentationYAMLMappingKeyIssues("", root); len(issues) > 0 {
		return &ValidationError{Issues: issues}
	}
	if issues := presentationYAMLMergeKeyIssues("", root); len(issues) > 0 {
		return &ValidationError{Issues: issues}
	}
	if issues := presentationYAMLNullIssues("", root); len(issues) > 0 {
		return &ValidationError{Issues: issues}
	}
	if issues := validatePresentationCatalogYAMLStructure(root); len(issues) > 0 {
		return &ValidationError{Issues: issues}
	}
	actionsNode := presentationYAMLMappingValue(
		presentationYAMLMappingValue(root, "spec"),
		"actions",
	)
	dataSourcesNode := presentationYAMLMappingValue(
		presentationYAMLMappingValue(root, "spec"),
		"dataSources",
	)
	dataSourceItems := presentationYAMLSequenceItems(dataSourcesNode)
	for index := range catalog.Spec.DataSources {
		if index >= len(dataSourceItems) {
			break
		}
		catalog.Spec.DataSources[index].maxSortFieldsOmitted =
			presentationYAMLMappingValue(dataSourceItems[index], "maxSortFields") == nil
	}
	items := presentationYAMLSequenceItems(actionsNode)
	for index := range catalog.Spec.Actions {
		if index >= len(items) {
			break
		}
		catalog.Spec.Actions[index].destructiveOmitted =
			presentationYAMLMappingValue(items[index], "destructive") == nil
	}
	return nil
}

func trackPresentationFieldListPresence(section *yaml.Node, fields []PresentationFieldSource) {
	items := presentationYAMLSequenceItems(presentationYAMLMappingValue(section, "fields"))
	for index := range fields {
		if index >= len(items) {
			break
		}
		item := items[index]
		fields[index].orderOmitted = presentationYAMLMappingValue(item, "order") == nil
		trackPresentationLocalizedTextPresence(presentationYAMLMappingValue(item, "label"), fields[index].Label)
		trackPresentationLocalizedTextPresence(presentationYAMLMappingValue(item, "placeholder"), fields[index].Placeholder)
		trackPresentationLocalizedTextPresence(presentationYAMLMappingValue(item, "help"), fields[index].Help)
	}
}

func trackPresentationActionListPresence(node *yaml.Node, actions []PresentationActionSource) {
	items := presentationYAMLSequenceItems(node)
	for index := range actions {
		if index >= len(items) {
			break
		}
		item := items[index]
		actions[index].orderOmitted = presentationYAMLMappingValue(item, "order") == nil
		trackPresentationLocalizedTextPresence(presentationYAMLMappingValue(item, "label"), actions[index].Label)
		trackPresentationLocalizedTextPresence(presentationYAMLMappingValue(item, "confirm"), actions[index].Confirm)
	}
}

func trackPresentationLocalizedTextPresence(node *yaml.Node, text *PresentationLocalizedText) {
	if node == nil || text == nil {
		return
	}
	text.presenceTracked = true
	text.zhCNPresent = presentationYAMLMappingValue(node, "zh-CN") != nil
	text.enUSPresent = presentationYAMLMappingValue(node, "en-US") != nil
}

func presentationYAMLRoot(data []byte) (*yaml.Node, error) {
	var document yaml.Node
	if err := yaml.Unmarshal(data, &document); err != nil {
		return nil, fmt.Errorf("decode presentation YAML presence: %w", err)
	}
	if document.Kind != yaml.DocumentNode || len(document.Content) != 1 {
		return nil, fmt.Errorf("decode presentation YAML presence: expected one document root")
	}
	root := presentationYAMLDereference(document.Content[0])
	if root == nil || root.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("decode presentation YAML presence: document root must be a mapping")
	}
	return root, nil
}

func presentationYAMLMappingValue(node *yaml.Node, key string) *yaml.Node {
	node = presentationYAMLDereference(node)
	if node == nil || node.Kind != yaml.MappingNode {
		return nil
	}
	for index := 0; index+1 < len(node.Content); index += 2 {
		if node.Content[index].Value == key {
			return presentationYAMLDereference(node.Content[index+1])
		}
	}
	return nil
}

func presentationYAMLSequenceItems(node *yaml.Node) []*yaml.Node {
	node = presentationYAMLDereference(node)
	if node == nil || node.Kind != yaml.SequenceNode {
		return nil
	}
	items := make([]*yaml.Node, 0, len(node.Content))
	for _, item := range node.Content {
		items = append(items, presentationYAMLDereference(item))
	}
	return items
}

func presentationYAMLDereference(node *yaml.Node) *yaml.Node {
	for node != nil && node.Kind == yaml.AliasNode {
		node = node.Alias
	}
	return node
}

func presentationYAMLNullIssues(path string, node *yaml.Node) []Issue {
	issues := make([]Issue, 0)
	var walk func(string, *yaml.Node)
	walk = func(currentPath string, current *yaml.Node) {
		current = presentationYAMLDereference(current)
		if current == nil {
			return
		}
		if current.Kind == yaml.ScalarNode &&
			(current.Tag == "!!null" || current.Tag == "tag:yaml.org,2002:null") {
			issues = append(issues, Issue{
				Path: currentPath, Code: "null-not-allowed",
				Message: "explicit null is not allowed; omit optional properties or provide the declared value type",
			})
			return
		}
		switch current.Kind {
		case yaml.MappingNode:
			for index := 0; index+1 < len(current.Content); index += 2 {
				childPath := current.Content[index].Value
				if currentPath != "" {
					childPath = currentPath + "." + childPath
				}
				walk(childPath, current.Content[index+1])
			}
		case yaml.SequenceNode:
			for index, child := range current.Content {
				walk(fmt.Sprintf("%s[%d]", currentPath, index), child)
			}
		}
	}
	walk(path, node)
	return issues
}

func presentationYAMLMergeKeyIssues(path string, node *yaml.Node) []Issue {
	issues := make([]Issue, 0)
	var walk func(string, *yaml.Node)
	walk = func(currentPath string, current *yaml.Node) {
		current = presentationYAMLDereference(current)
		if current == nil {
			return
		}
		switch current.Kind {
		case yaml.MappingNode:
			for index := 0; index+1 < len(current.Content); index += 2 {
				key := current.Content[index]
				childPath := presentationYAMLPath(currentPath, key.Value)
				if key.Value == "<<" || key.ShortTag() == "!!merge" {
					issues = append(issues, Issue{
						Path: childPath, Code: "yaml-merge-key-forbidden",
						Message: "YAML merge keys are not allowed in strict presentation contracts",
					})
					continue
				}
				walk(childPath, current.Content[index+1])
			}
		case yaml.SequenceNode:
			for index, child := range current.Content {
				walk(fmt.Sprintf("%s[%d]", currentPath, index), child)
			}
		}
	}
	walk(path, node)
	return issues
}

func presentationYAMLMappingKeyIssues(path string, node *yaml.Node) []Issue {
	issues := make([]Issue, 0)
	var walk func(string, *yaml.Node)
	walk = func(currentPath string, current *yaml.Node) {
		current = presentationYAMLDereference(current)
		if current == nil {
			return
		}
		switch current.Kind {
		case yaml.MappingNode:
			for index := 0; index+1 < len(current.Content); index += 2 {
				key := current.Content[index]
				keyName := key.Value
				if keyName == "" {
					keyName = "<key>"
				}
				childPath := presentationYAMLPath(currentPath, keyName)
				isMerge := key.Value == "<<" || key.ShortTag() == "!!merge"
				if !isMerge && (key.Kind != yaml.ScalarNode || key.ShortTag() != "!!str") {
					issues = append(issues, Issue{
						Path: childPath, Code: "yaml-key-type-mismatch",
						Message: "mapping keys must be explicit standard YAML strings",
					})
				}
				walk(childPath, current.Content[index+1])
			}
		case yaml.SequenceNode:
			for index, child := range current.Content {
				walk(fmt.Sprintf("%s[%d]", currentPath, index), child)
			}
		}
	}
	walk(path, node)
	return issues
}
