package response

import (
	"reflect"
	"sync"

	"github.com/gin-gonic/gin/binding"
)

const (
	uri uint8 = iota
	jsonBody
	xmlBody
	yamlBody
	formBody
	queryValues
)

var constructor = &bindConstructor{}

type bindConstructor struct {
	cache map[reflect.Type][]uint8
	mux   sync.RWMutex
}

// GetBindingForGin returns a deterministic, cached binding plan for d.
func (e *bindConstructor) GetBindingForGin(d any) []binding.Binding {
	typeOf := indirectType(reflect.TypeOf(d))
	if typeOf == nil || typeOf.Kind() != reflect.Struct {
		return nil
	}
	bindingIDs, ok := e.getBinding(typeOf)
	if !ok {
		bindingIDs = resolveBindingIDs(typeOf)
		e.setBinding(typeOf, bindingIDs)
	}

	bindings := make([]binding.Binding, 0, len(bindingIDs))
	for _, bindingID := range bindingIDs {
		switch bindingID {
		case uri:
			bindings = append(bindings, nil)
		case queryValues:
			bindings = append(bindings, binding.Query)
		case formBody:
			bindings = append(bindings, binding.Form)
		case jsonBody:
			bindings = append(bindings, binding.JSON)
		case xmlBody:
			bindings = append(bindings, binding.XML)
		case yamlBody:
			bindings = append(bindings, binding.YAML)
		}
	}
	return bindings
}

func resolveBindingIDs(root reflect.Type) []uint8 {
	found := make(map[uint8]bool, 6)
	visited := make(map[reflect.Type]bool)
	inspectBindingTags(root, found, visited)

	// URI and query values are applied before a request body so validation can
	// evaluate the fully populated DTO. Body formats have a stable precedence.
	order := []uint8{uri, queryValues, formBody, jsonBody, xmlBody, yamlBody}
	bindings := make([]uint8, 0, len(found))
	for _, bindingID := range order {
		if found[bindingID] {
			bindings = append(bindings, bindingID)
		}
	}
	return bindings
}

func inspectBindingTags(typeOf reflect.Type, found map[uint8]bool, visited map[reflect.Type]bool) {
	typeOf = indirectType(typeOf)
	if typeOf == nil || typeOf.Kind() != reflect.Struct || visited[typeOf] {
		return
	}
	visited[typeOf] = true
	defer delete(visited, typeOf)

	for i := 0; i < typeOf.NumField(); i++ {
		field := typeOf.Field(i)
		if field.PkgPath != "" && !field.Anonymous {
			continue
		}
		markTag(field, "uri", uri, found)
		markTag(field, "query", queryValues, found)
		markTag(field, "form", formBody, found)
		markTag(field, "json", jsonBody, found)
		markTag(field, "xml", xmlBody, found)
		markTag(field, "yaml", yamlBody, found)

		nested := indirectType(field.Type)
		if nested != nil && nested.Kind() == reflect.Struct {
			inspectBindingTags(nested, found, visited)
		}
	}
}

func markTag(field reflect.StructField, key string, bindingID uint8, found map[uint8]bool) {
	value, ok := field.Tag.Lookup(key)
	if !ok || value == "-" {
		return
	}
	found[bindingID] = true
}

func indirectType(typeOf reflect.Type) reflect.Type {
	for typeOf != nil && (typeOf.Kind() == reflect.Ptr || typeOf.Kind() == reflect.Slice || typeOf.Kind() == reflect.Array) {
		typeOf = typeOf.Elem()
	}
	return typeOf
}

func (e *bindConstructor) getBinding(typeOf reflect.Type) ([]uint8, bool) {
	e.mux.RLock()
	defer e.mux.RUnlock()
	if e.cache == nil {
		return nil, false
	}
	bindings, ok := e.cache[typeOf]
	if !ok {
		return nil, false
	}
	return append([]uint8{}, bindings...), true
}

func (e *bindConstructor) setBinding(typeOf reflect.Type, bindings []uint8) {
	e.mux.Lock()
	defer e.mux.Unlock()
	if e.cache == nil {
		e.cache = make(map[reflect.Type][]uint8)
	}
	e.cache[typeOf] = append([]uint8(nil), bindings...)
}
