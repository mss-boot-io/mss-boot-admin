package blueprint

import (
	"bytes"
	"errors"
	"fmt"
	"io"

	"gopkg.in/yaml.v3"
)

// validateStrictYAMLDocument rejects YAML graph features before decoding a
// machine identity contract. Aliases and merge keys can make the effective
// field set depend on decoder-specific behavior, which is unsuitable for a
// signed or digested snapshot representation.
func validateStrictYAMLDocument(data []byte) error {
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	document := yaml.Node{}
	if err := decoder.Decode(&document); err != nil {
		return err
	}
	if len(document.Content) == 0 {
		return errors.New("empty YAML document")
	}
	var extra yaml.Node
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("multiple YAML documents are not supported")
		}
		return err
	}
	return validateStrictYAMLNode(&document, "$")
}

func validateStrictYAMLNode(node *yaml.Node, path string) error {
	if node.Kind == yaml.AliasNode || node.Anchor != "" {
		return fmt.Errorf("YAML anchors and aliases are not supported at %s", path)
	}
	switch node.Kind {
	case yaml.DocumentNode, yaml.SequenceNode:
		for index, child := range node.Content {
			if err := validateStrictYAMLNode(child, fmt.Sprintf("%s[%d]", path, index)); err != nil {
				return err
			}
		}
	case yaml.MappingNode:
		if len(node.Content)%2 != 0 {
			return fmt.Errorf("malformed YAML mapping at %s", path)
		}
		seen := make(map[string]bool, len(node.Content)/2)
		for index := 0; index < len(node.Content); index += 2 {
			key := node.Content[index]
			if key.Kind != yaml.ScalarNode || key.Tag == "!!merge" || key.Value == "<<" {
				return fmt.Errorf("YAML mapping keys and merges must be plain scalar fields at %s", path)
			}
			if seen[key.Value] {
				return fmt.Errorf("duplicate YAML key %q at %s", key.Value, path)
			}
			seen[key.Value] = true
			if err := validateStrictYAMLNode(node.Content[index+1], path+"."+key.Value); err != nil {
				return err
			}
		}
	}
	return nil
}
