package resource

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	maxResources           = 1024
	defaultRollbackTimeout = 5 * time.Second
)

// Starter synchronously acquires a resource. It must honor the caller context
// and must not detach long-running work; long-running work belongs in Runner.
type Starter interface {
	Start(context.Context) error
}

// Runner owns long-running work. Run must block until its work completes or
// its context is cancelled, and it must wait for every goroutine it starts
// before returning.
type Runner interface {
	Run(context.Context) error
}

// HealthChecker reports current provider health without changing lifecycle
// ownership and must honor the caller context.
type HealthChecker interface {
	Health(context.Context) error
}

// ReadinessChecker reports whether a started resource can serve dependents and
// must honor the caller context.
type ReadinessChecker interface {
	Ready(context.Context) error
}

// Closer releases an acquired resource. Implementations must be idempotent,
// must honor the caller context, and must not detach cleanup work.
type Closer interface {
	Close(context.Context) error
}

// Resource is the minimum owned lifecycle. Close must be safe after any Start
// invocation, including a Start that acquired only part of its state. Optional
// Run, Health, and Ready behavior is discovered through the narrow interfaces
// above.
type Resource interface {
	Starter
	Closer
}

// Definition declares one named resource and its dependency names. Required
// means Start must establish readiness synchronously; Build rejects a required
// resource that does not implement ReadinessChecker.
type Definition struct {
	Name         string
	Dependencies []string
	Required     bool
	Resource     Resource
}

type node struct {
	name         string
	dependencies []string
	required     bool
	resource     Resource
	runner       Runner
	health       HealthChecker
	readiness    ReadinessChecker

	// acquired and closed are protected by Graph.mu. A resource becomes
	// acquired immediately before Start is invoked, so a partially failing
	// Start is always included in rollback.
	acquired bool
	closed   bool
}

type resourceIdentity struct {
	typeName reflect.Type
	pointer  uintptr
}

// Graph is an immutable declaration graph plus a concurrency-safe, one-way
// lifecycle state machine.
type Graph struct {
	mu    sync.Mutex
	nodes []node
	names []string

	startAttempted bool
	startSucceeded bool
	startDone      chan struct{}
	startCancel    context.CancelFunc

	runAttempted bool
	runDone      chan struct{}
	runCancel    context.CancelFunc

	closeRequested bool
	closed         bool
	activeClose    *closeGeneration

	inspections     int
	inspectionsDone chan struct{}
}

// Build validates and deterministically sorts definitions without invoking
// any resource method. The returned graph owns private copies of all names and
// dependency slices.
func Build(definitions ...Definition) (*Graph, error) {
	if len(definitions) > maxResources {
		return nil, invalid("definitions", "exceeds the resource limit")
	}

	byName := make(map[string]Definition, len(definitions))
	definitionIndex := make(map[string]int, len(definitions))
	resourceOwner := make(map[resourceIdentity]int, len(definitions))
	for index, definition := range definitions {
		path := fmt.Sprintf("definitions[%d]", index)
		if !validName(definition.Name) {
			return nil, invalid(path+".name", "must be a canonical resource name")
		}
		if _, exists := byName[definition.Name]; exists {
			return nil, invalid(path+".name", "duplicates another resource name")
		}
		if nilInterface(definition.Resource) {
			return nil, invalid(path+".resource", "is required")
		}
		if identity, stable := stableResourceIdentity(definition.Resource); stable {
			if _, exists := resourceOwner[identity]; exists {
				return nil, invalid(path+".resource", "is already owned by another resource name")
			}
			resourceOwner[identity] = index
		}
		if definition.Required {
			readiness, ok := definition.Resource.(ReadinessChecker)
			if !ok || nilInterface(readiness) {
				return nil, invalid(path+".resource", "required resources must implement readiness")
			}
		}

		dependencies := append([]string(nil), definition.Dependencies...)
		seen := make(map[string]struct{}, len(dependencies))
		for dependencyIndex, dependency := range dependencies {
			dependencyPath := fmt.Sprintf("%s.dependencies[%d]", path, dependencyIndex)
			if !validName(dependency) {
				return nil, invalid(dependencyPath, "must be a canonical resource name")
			}
			if dependency == definition.Name {
				return nil, invalid(dependencyPath, "must not reference the resource itself")
			}
			if _, exists := seen[dependency]; exists {
				return nil, invalid(dependencyPath, "duplicates another dependency")
			}
			seen[dependency] = struct{}{}
		}
		sort.Strings(dependencies)
		definition.Dependencies = dependencies
		byName[definition.Name] = definition
		definitionIndex[definition.Name] = index
	}

	validationNames := make([]string, 0, len(byName))
	for name := range byName {
		validationNames = append(validationNames, name)
	}
	sort.Strings(validationNames)
	for _, name := range validationNames {
		definition := byName[name]
		for dependencyIndex, dependency := range definition.Dependencies {
			if _, exists := byName[dependency]; !exists {
				path := fmt.Sprintf("definitions[%d].dependencies[%d]", definitionIndex[name], dependencyIndex)
				return nil, invalid(path, "references a missing resource")
			}
		}
	}

	order, err := topologicalOrder(byName)
	if err != nil {
		return nil, err
	}

	nodes := make([]node, 0, len(order))
	for _, name := range order {
		definition := byName[name]
		current := node{
			name:         definition.Name,
			dependencies: append([]string(nil), definition.Dependencies...),
			required:     definition.Required,
			resource:     definition.Resource,
		}
		current.runner, _ = definition.Resource.(Runner)
		current.health, _ = definition.Resource.(HealthChecker)
		current.readiness, _ = definition.Resource.(ReadinessChecker)
		nodes = append(nodes, current)
	}

	return &Graph{nodes: nodes, names: append([]string(nil), order...)}, nil
}

func topologicalOrder(definitions map[string]Definition) ([]string, error) {
	indegree := make(map[string]int, len(definitions))
	dependents := make(map[string][]string, len(definitions))
	for name, definition := range definitions {
		indegree[name] = len(definition.Dependencies)
		for _, dependency := range definition.Dependencies {
			dependents[dependency] = append(dependents[dependency], name)
		}
	}
	for dependency := range dependents {
		sort.Strings(dependents[dependency])
	}

	ready := make([]string, 0, len(definitions))
	for name, count := range indegree {
		if count == 0 {
			ready = append(ready, name)
		}
	}
	sort.Strings(ready)

	order := make([]string, 0, len(definitions))
	for len(ready) > 0 {
		name := ready[0]
		ready = ready[1:]
		order = append(order, name)
		for _, dependent := range dependents[name] {
			indegree[dependent]--
			if indegree[dependent] == 0 {
				ready = append(ready, dependent)
				sort.Strings(ready)
			}
		}
	}
	if len(order) != len(definitions) {
		return nil, invalid("definitions", "contains a dependency cycle")
	}
	return order, nil
}

func validName(value string) bool {
	if value == "" || len(value) > 63 || value[0] < 'a' || value[0] > 'z' {
		return false
	}
	for index := range len(value) {
		character := value[index]
		if character >= 'a' && character <= 'z' || character >= '0' && character <= '9' || index > 0 && (character == '-' || character == '.') {
			continue
		}
		return false
	}
	return value[len(value)-1] != '-' && value[len(value)-1] != '.' && !strings.Contains(value, "..")
}

func nilInterface(value any) bool {
	if value == nil {
		return true
	}
	reflected := reflect.ValueOf(value)
	switch reflected.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return reflected.IsNil()
	default:
		return false
	}
}

func stableResourceIdentity(value Resource) (resourceIdentity, bool) {
	reflected := reflect.ValueOf(value)
	if reflected.Kind() != reflect.Pointer {
		return resourceIdentity{}, false
	}
	return resourceIdentity{typeName: reflected.Type(), pointer: reflected.Pointer()}, true
}

// ResourceNames returns the deterministic topological order. The returned
// slice is independent from the graph.
func (g *Graph) ResourceNames() []string {
	if g == nil {
		return nil
	}
	return append([]string(nil), g.names...)
}

func (g *Graph) String() string {
	if g == nil {
		return "RuntimeResourceGraph<nil>"
	}
	return fmt.Sprintf("RuntimeResourceGraph{resources:%d}", len(g.names))
}

// Start acquires resources in topological order. A required resource's Ready
// check completes before any dependent Start is invoked. On cancellation or
// failure, every resource whose Start was invoked is closed in reverse order
// under an independent bounded rollback context.
func (g *Graph) Start(ctx context.Context) (result error) {
	if g == nil {
		return ErrInvalidGraph
	}
	if ctx == nil {
		return ErrContextRequired
	}
	if err := ctx.Err(); err != nil {
		return lifecycleError("", OperationStart, err)
	}

	startCtx, cancel, err := g.beginStart(ctx)
	if err != nil {
		return err
	}
	succeeded := false
	defer func() {
		cancel()
		g.finishStart(succeeded)
	}()

	acquired := make([]int, 0, len(g.nodes))
	for index := range g.nodes {
		if err := startCtx.Err(); err != nil {
			result = g.rollbackStartup(ctx, acquired, lifecycleError("", OperationStart, err))
			return result
		}

		g.mu.Lock()
		g.nodes[index].acquired = true
		g.mu.Unlock()
		acquired = append(acquired, index)

		if err := g.nodes[index].resource.Start(startCtx); err != nil {
			startFailure := lifecycleError(g.nodes[index].name, OperationStart, err)
			startFailure = joinLifecycleContext(startFailure, OperationStart, ctx.Err())
			result = g.rollbackStartup(ctx, acquired, startFailure)
			return result
		}
		if g.nodes[index].required {
			if err := g.nodes[index].readiness.Ready(startCtx); err != nil {
				readinessFailure := lifecycleError(g.nodes[index].name, OperationReady, err)
				readinessFailure = joinLifecycleContext(readinessFailure, OperationReady, ctx.Err())
				result = g.rollbackStartup(ctx, acquired, readinessFailure)
				return result
			}
		}
	}

	if err := startCtx.Err(); err != nil {
		result = g.rollbackStartup(ctx, acquired, lifecycleError("", OperationStart, err))
		return result
	}
	g.mu.Lock()
	closing := g.closeRequested
	if !closing {
		g.startSucceeded = true
	}
	g.mu.Unlock()
	if closing {
		result = g.rollbackStartup(ctx, acquired, lifecycleError("", OperationStart, ErrClosing))
		return result
	}
	succeeded = true
	return nil
}

func (g *Graph) beginStart(ctx context.Context) (context.Context, context.CancelFunc, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.startAttempted || g.closeRequested || g.closed {
		return nil, nil, ErrStartRejected
	}
	g.startAttempted = true
	g.startDone = make(chan struct{})
	startCtx, cancel := context.WithCancel(ctx)
	g.startCancel = cancel
	return startCtx, cancel, nil
}

func (g *Graph) finishStart(succeeded bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.startSucceeded = succeeded
	g.startCancel = nil
	close(g.startDone)
}

func (g *Graph) rollbackStartup(parent context.Context, acquired []int, original error) error {
	rollbackCtx, cancel := context.WithTimeout(context.WithoutCancel(parent), defaultRollbackTimeout)
	defer cancel()
	return errors.Join(original, g.closeIndexes(rollbackCtx, acquired, OperationRollback))
}

func (g *Graph) beginInspection() error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if !g.startSucceeded {
		return ErrNotStarted
	}
	if g.closeRequested || g.closed {
		return ErrClosing
	}
	if g.inspections == 0 {
		g.inspectionsDone = make(chan struct{})
	}
	g.inspections++
	return nil
}

func (g *Graph) finishInspection() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.inspections--
	if g.inspections == 0 {
		close(g.inspectionsDone)
	}
}

func (g *Graph) waitForInspections(ctx context.Context) error {
	g.mu.Lock()
	if g.inspections == 0 {
		g.mu.Unlock()
		return nil
	}
	done := g.inspectionsDone
	g.mu.Unlock()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return lifecycleError("", OperationClose, ctx.Err())
	}
}

func waitForLifecycle(ctx context.Context, done <-chan struct{}) error {
	if done == nil {
		return nil
	}
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return lifecycleError("", OperationClose, ctx.Err())
	}
}
