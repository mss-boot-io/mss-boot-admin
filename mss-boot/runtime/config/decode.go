package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"reflect"
	"strings"

	"gopkg.in/yaml.v3"
)

// Format selects the strict document decoder.
type Format string

const (
	FormatYAML Format = "yaml"
	FormatJSON Format = "json"
)

const maxDocumentBytes = 1 << 20

type keySchemaKind uint8

const (
	keySchemaScalar keySchemaKind = iota
	keySchemaStruct
	keySchemaMap
	keySchemaSlice
)

type keySchema struct {
	kind    keySchemaKind
	fields  map[string]*keySchema
	element *keySchema
}

var (
	configJSONKeySchema = buildKeySchema(reflect.TypeFor[Config](), "json")
	configYAMLKeySchema = buildKeySchema(reflect.TypeFor[Config](), "yaml")
)

// Decode reads one bounded YAML or JSON document, rejects unknown and
// duplicate fields, and never accepts trailing documents or values.
func Decode(reader io.Reader, format Format) (Config, error) {
	if reader == nil {
		return Config{}, invalid("document", "reader is required")
	}
	data, err := io.ReadAll(io.LimitReader(reader, maxDocumentBytes+1))
	if err != nil || len(data) == 0 || len(data) > maxDocumentBytes {
		return Config{}, invalid("document", "is empty, unreadable, or exceeds the size limit")
	}
	switch format {
	case FormatYAML:
		return decodeYAML(data)
	case FormatJSON:
		return decodeJSON(data)
	default:
		return Config{}, invalid("document", "format must be yaml or json")
	}
}

// DecodeYAML is a byte-slice convenience wrapper around Decode.
func DecodeYAML(data []byte) (Config, error) {
	return Decode(bytes.NewReader(data), FormatYAML)
}

// DecodeJSON is a byte-slice convenience wrapper around Decode.
func DecodeJSON(data []byte) (Config, error) {
	return Decode(bytes.NewReader(data), FormatJSON)
}

func decodeYAML(data []byte) (Config, error) {
	var document yaml.Node
	if err := yaml.Unmarshal(data, &document); err != nil || len(document.Content) != 1 || document.Content[0].Kind != yaml.MappingNode {
		return Config{}, invalid("document", "must contain one well-formed mapping")
	}
	if hasYAMLAliasOrMerge(document.Content[0]) {
		return Config{}, invalid("document", "aliases and merge keys are not supported")
	}
	if err := validateYAMLKeys(document.Content[0], configYAMLKeySchema); err != nil {
		return Config{}, err
	}

	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	var result Config
	if err := decoder.Decode(&result); err != nil {
		return Config{}, invalid("document", "contains an unknown field or invalid value")
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return Config{}, invalid("document", "must contain exactly one YAML document")
	}
	return result, nil
}

func hasYAMLAliasOrMerge(node *yaml.Node) bool {
	if node == nil {
		return false
	}
	if node.Kind == yaml.AliasNode || node.Tag == "!!merge" || node.Value == "<<" {
		return true
	}
	for _, child := range node.Content {
		if hasYAMLAliasOrMerge(child) {
			return true
		}
	}
	return false
}

func decodeJSON(data []byte) (Config, error) {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return Config{}, invalid("document", "must contain one JSON object")
	}
	if err := validateJSONKeys(data, configJSONKeySchema); err != nil {
		return Config{}, err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var result Config
	if err := decoder.Decode(&result); err != nil {
		return Config{}, invalid("document", "contains an unknown field or invalid value")
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return Config{}, invalid("document", "must contain exactly one JSON value")
	}
	return result, nil
}

func validateJSONKeys(data []byte, schema *keySchema) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := walkJSONValue(decoder, schema); err != nil {
		return invalid("document", "must be well-formed and use canonical non-duplicate fields")
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		return invalid("document", "must contain exactly one JSON value")
	}
	return nil
}

func walkJSONValue(decoder *json.Decoder, schema *keySchema) error {
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	delimiter, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	switch delimiter {
	case '{':
		seen := make(map[string]struct{})
		for decoder.More() {
			keyToken, keyErr := decoder.Token()
			if keyErr != nil {
				return keyErr
			}
			key, ok := keyToken.(string)
			if !ok {
				return errors.New("JSON object key is not a string")
			}
			if _, exists := seen[key]; exists {
				return errors.New("duplicate JSON object key")
			}
			seen[key] = struct{}{}
			childSchema := (*keySchema)(nil)
			if schema != nil {
				switch schema.kind {
				case keySchemaStruct:
					var exists bool
					childSchema, exists = schema.fields[key]
					if !exists {
						return errors.New("unknown or non-canonical JSON object key")
					}
				case keySchemaMap:
					childSchema = schema.element
				}
			}
			if valueErr := walkJSONValue(decoder, childSchema); valueErr != nil {
				return valueErr
			}
		}
		closing, closingErr := decoder.Token()
		if closingErr != nil || closing != json.Delim('}') {
			return errors.New("invalid JSON object")
		}
	case '[':
		childSchema := (*keySchema)(nil)
		if schema != nil && schema.kind == keySchemaSlice {
			childSchema = schema.element
		}
		for decoder.More() {
			if valueErr := walkJSONValue(decoder, childSchema); valueErr != nil {
				return valueErr
			}
		}
		closing, closingErr := decoder.Token()
		if closingErr != nil || closing != json.Delim(']') {
			return errors.New("invalid JSON array")
		}
	default:
		return errors.New("unexpected JSON delimiter")
	}
	return nil
}

func validateYAMLKeys(node *yaml.Node, schema *keySchema) error {
	if node == nil {
		return nil
	}
	switch node.Kind {
	case yaml.MappingNode:
		seen := make(map[string]struct{}, len(node.Content)/2)
		for index := 0; index+1 < len(node.Content); index += 2 {
			key := node.Content[index]
			value := node.Content[index+1]
			if key.Kind != yaml.ScalarNode || key.Tag != "!!str" {
				return invalid("document", "must use canonical string field names")
			}
			if _, exists := seen[key.Value]; exists {
				return invalid("document", "must use non-duplicate fields")
			}
			seen[key.Value] = struct{}{}

			childSchema := (*keySchema)(nil)
			if schema != nil {
				switch schema.kind {
				case keySchemaStruct:
					var exists bool
					childSchema, exists = schema.fields[key.Value]
					if !exists {
						return invalid("document", "contains an unknown or non-canonical field")
					}
				case keySchemaMap:
					childSchema = schema.element
				}
			}
			if err := validateYAMLKeys(value, childSchema); err != nil {
				return err
			}
		}
	case yaml.SequenceNode:
		childSchema := (*keySchema)(nil)
		if schema != nil && schema.kind == keySchemaSlice {
			childSchema = schema.element
		}
		for _, child := range node.Content {
			if err := validateYAMLKeys(child, childSchema); err != nil {
				return err
			}
		}
	}
	return nil
}

func buildKeySchema(valueType reflect.Type, tagName string) *keySchema {
	for valueType.Kind() == reflect.Pointer {
		valueType = valueType.Elem()
	}
	switch valueType.Kind() {
	case reflect.Struct:
		fields := make(map[string]*keySchema)
		for index := range valueType.NumField() {
			field := valueType.Field(index)
			if !field.IsExported() {
				continue
			}
			name := strings.Split(field.Tag.Get(tagName), ",")[0]
			if name == "" || name == "-" {
				continue
			}
			fields[name] = buildKeySchema(field.Type, tagName)
		}
		return &keySchema{kind: keySchemaStruct, fields: fields}
	case reflect.Map:
		return &keySchema{kind: keySchemaMap, element: buildKeySchema(valueType.Elem(), tagName)}
	case reflect.Array, reflect.Slice:
		return &keySchema{kind: keySchemaSlice, element: buildKeySchema(valueType.Elem(), tagName)}
	default:
		return &keySchema{kind: keySchemaScalar}
	}
}
