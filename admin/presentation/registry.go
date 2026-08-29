package presentation

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"sync"
)

var (
	ErrCapabilityAlreadyRegistered = errors.New("presentation capability is already registered")
	ErrRegistryFrozen              = errors.New("presentation capability registry is frozen")
)

// Registry contains only trusted definitions compiled into the server. It is
// deliberately empty in P1 production; generated business modules begin
// registration in P2 after their own compatibility review.
type Registry struct {
	mu          sync.RWMutex
	definitions map[string]CapabilityDefinition
	frozen      bool
}

func NewRegistry(definitions ...CapabilityDefinition) (*Registry, error) {
	registry := &Registry{definitions: make(map[string]CapabilityDefinition, len(definitions))}
	for index := range definitions {
		if err := registry.register(definitions[index]); err != nil {
			return nil, err
		}
	}
	return registry, nil
}

func MustNewRegistry(definitions ...CapabilityDefinition) *Registry {
	registry, err := NewRegistry(definitions...)
	if err != nil {
		panic(err)
	}
	return registry
}

// NewFrozenRegistry creates a fully validated registry that cannot be mutated.
// NewRegistry deliberately retains its P1 builder semantics for compatibility;
// application composition must publish only a frozen registry.
func NewFrozenRegistry(definitions ...CapabilityDefinition) (*Registry, error) {
	registry, err := NewRegistry(definitions...)
	if err != nil {
		return nil, err
	}
	if err = registry.Freeze(); err != nil {
		return nil, err
	}
	return registry, nil
}

func MustNewFrozenRegistry(definitions ...CapabilityDefinition) *Registry {
	registry, err := NewFrozenRegistry(definitions...)
	if err != nil {
		panic(err)
	}
	return registry
}

func (registry *Registry) Register(definition CapabilityDefinition) error {
	if registry == nil {
		return errors.New("presentation registry is nil")
	}
	registry.mu.Lock()
	defer registry.mu.Unlock()
	if registry.frozen {
		return ErrRegistryFrozen
	}
	return registry.registerLocked(definition)
}

// Freeze prevents all later mutation. Application composition freezes the
// registry before any route is mounted; NewRegistry returns an already-frozen
// registry for the common one-shot construction path.
func (registry *Registry) Freeze() error {
	if registry == nil {
		return errors.New("presentation registry is nil")
	}
	registry.mu.Lock()
	defer registry.mu.Unlock()
	registry.frozen = true
	return nil
}

func (registry *Registry) register(definition CapabilityDefinition) error {
	registry.mu.Lock()
	defer registry.mu.Unlock()
	return registry.registerLocked(definition)
}

func (registry *Registry) registerLocked(definition CapabilityDefinition) error {
	if issues := ValidateCapability(&definition); len(issues) > 0 {
		return fmt.Errorf("invalid presentation capability %q: %s", definition.PageKey, formatIssues(issues))
	}
	clone, err := cloneCapability(definition)
	if err != nil {
		return fmt.Errorf("clone presentation capability %q: %w", definition.PageKey, err)
	}
	if registry.definitions == nil {
		registry.definitions = make(map[string]CapabilityDefinition)
	}
	if _, exists := registry.definitions[definition.PageKey]; exists {
		return fmt.Errorf("%w: %s", ErrCapabilityAlreadyRegistered, definition.PageKey)
	}
	registry.definitions[definition.PageKey] = clone
	return nil
}

func (registry *Registry) Lookup(pageKey string) (*CapabilityDefinition, bool) {
	if registry == nil {
		return nil, false
	}
	registry.mu.RLock()
	definition, ok := registry.definitions[pageKey]
	registry.mu.RUnlock()
	if !ok {
		return nil, false
	}
	clone, err := cloneCapability(definition)
	if err != nil {
		return nil, false
	}
	return &clone, true
}

func (registry *Registry) List() []CapabilityDefinition {
	if registry == nil {
		return []CapabilityDefinition{}
	}
	registry.mu.RLock()
	keys := make([]string, 0, len(registry.definitions))
	for key := range registry.definitions {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	result := make([]CapabilityDefinition, 0, len(keys))
	for _, key := range keys {
		clone, err := cloneCapability(registry.definitions[key])
		if err == nil {
			result = append(result, clone)
		}
	}
	registry.mu.RUnlock()
	return result
}

func (registry *Registry) Validate(profile *Profile) []Issue {
	if profile == nil {
		return ValidateProfile(nil, nil)
	}
	definition, ok := registry.Lookup(profile.Metadata.PageKey)
	if !ok {
		return ValidateProfile(nil, profile)
	}
	return ValidateProfile(definition, profile)
}

func cloneCapability(value CapabilityDefinition) (CapabilityDefinition, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return CapabilityDefinition{}, err
	}
	result := CapabilityDefinition{}
	if err = json.Unmarshal(raw, &result); err != nil {
		return CapabilityDefinition{}, err
	}
	return result, nil
}

func formatIssues(issues []Issue) string {
	if len(issues) == 0 {
		return ""
	}
	result := ""
	for index := range issues {
		if index > 0 {
			result += "; "
		}
		result += issues[index].Path + ": " + issues[index].Code
	}
	return result
}

// DefaultRegistry preserves the P1 source-compatibility surface. Production
// application paths inject their own frozen registry and must not fall back to
// this empty value.
var DefaultRegistry = MustNewFrozenRegistry()
