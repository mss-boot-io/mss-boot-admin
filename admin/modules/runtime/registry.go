package runtime

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"

	"gorm.io/gorm"
)

// Permission declares one backend-enforced module action.
type Permission struct {
	Code        string
	DisplayName string
	Description string
}

// Menu declares the default navigation entry for a module.
type Menu struct {
	Path          string
	DisplayName   string
	DisplayNameEn string
	Icon          string
	Parent        string
	Order         int
	Hidden        bool
}

// Descriptor is the runtime contract registered by one vertical module.
type Descriptor struct {
	Name        string
	DisplayName string
	Description string
	Version     string
	Model       any
	Migrate     func(*gorm.DB) error
	Permissions []Permission
	Menu        Menu
}

var registry = struct {
	sync.RWMutex
	modules map[string]Descriptor
}{
	modules: make(map[string]Descriptor),
}

// Register adds one module descriptor. It returns an error for invalid or duplicate modules.
func Register(descriptor Descriptor) error {
	descriptor.Name = strings.TrimSpace(descriptor.Name)
	descriptor.DisplayName = strings.TrimSpace(descriptor.DisplayName)
	if descriptor.Name == "" {
		return errors.New("module name is required")
	}
	if descriptor.DisplayName == "" {
		return fmt.Errorf("module %s display name is required", descriptor.Name)
	}
	if descriptor.Model == nil && descriptor.Migrate == nil {
		return fmt.Errorf("module %s must define a model or explicit compatibility migration", descriptor.Name)
	}

	registry.Lock()
	defer registry.Unlock()
	if _, exists := registry.modules[descriptor.Name]; exists {
		return fmt.Errorf("module %s is already registered", descriptor.Name)
	}
	registry.modules[descriptor.Name] = cloneDescriptor(descriptor)
	return nil
}

// MustRegister registers a descriptor during package initialization.
func MustRegister(descriptor Descriptor) {
	if err := Register(descriptor); err != nil {
		panic(err)
	}
}

// Get returns a copy of one registered module descriptor.
func Get(name string) (Descriptor, bool) {
	registry.RLock()
	defer registry.RUnlock()
	descriptor, exists := registry.modules[name]
	if !exists {
		return Descriptor{}, false
	}
	return cloneDescriptor(descriptor), true
}

// All returns registered descriptors in stable name order.
func All() []Descriptor {
	registry.RLock()
	defer registry.RUnlock()
	result := make([]Descriptor, 0, len(registry.modules))
	for _, descriptor := range registry.modules {
		result = append(result, cloneDescriptor(descriptor))
	}
	sort.SliceStable(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})
	return result
}

// Migrate invokes only explicitly registered compatibility hooks. A model is
// metadata, never authorization to infer production DDL with AutoMigrate.
// New generated modules register typed forward migrations with the Admin
// migration runner and leave Descriptor.Migrate nil.
func Migrate(db *gorm.DB) error {
	if db == nil {
		return errors.New("module migration database is nil")
	}
	for _, descriptor := range All() {
		if descriptor.Migrate != nil {
			if err := descriptor.Migrate(db); err != nil {
				return fmt.Errorf("migrate module %s: %w", descriptor.Name, err)
			}
		}
	}
	return nil
}

// ResetForTest clears the registry. It is intentionally exported only for isolated tests.
func ResetForTest() {
	registry.Lock()
	defer registry.Unlock()
	registry.modules = make(map[string]Descriptor)
}

func cloneDescriptor(descriptor Descriptor) Descriptor {
	clone := descriptor
	clone.Permissions = append([]Permission(nil), descriptor.Permissions...)
	return clone
}
