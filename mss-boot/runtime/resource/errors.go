package resource

import (
	"errors"
	"fmt"
)

var (
	// ErrInvalidGraph classifies an invalid resource declaration graph.
	ErrInvalidGraph = errors.New("invalid runtime resource graph")
	// ErrContextRequired is returned instead of silently substituting a
	// process-global context.
	ErrContextRequired = errors.New("runtime resource context is required")
	// ErrStartRejected classifies a repeated Start or a Start attempted after
	// shutdown has begun.
	ErrStartRejected = errors.New("runtime resource start rejected")
	// ErrNotStarted classifies an operation that requires a successful Start.
	ErrNotStarted = errors.New("runtime resource graph is not started")
	// ErrRunRejected classifies a repeated or concurrent Run call.
	ErrRunRejected = errors.New("runtime resource run rejected")
	// ErrRunStopped classifies a long-running Runner that returned nil before
	// cancellation or graph shutdown.
	ErrRunStopped = errors.New("runtime resource runner stopped before shutdown")
	// ErrClosing classifies work rejected because Close has begun. Close is an
	// irreversible state transition even when a caller deadline expires.
	ErrClosing = errors.New("runtime resource graph is closing")
)

// Operation identifies a fixed lifecycle boundary. Values are controlled by
// this package and are safe to include in diagnostics.
type Operation string

const (
	OperationStart    Operation = "start"
	OperationRun      Operation = "run"
	OperationHealth   Operation = "health"
	OperationReady    Operation = "ready"
	OperationRollback Operation = "rollback"
	OperationClose    Operation = "close"
)

// ValidationError describes graph structure without retaining rejected user
// input. Path contains only declaration indexes and canonical field names.
type ValidationError struct {
	Path   string
	Reason string
}

func (e *ValidationError) Error() string {
	if e == nil {
		return ErrInvalidGraph.Error()
	}
	return fmt.Sprintf("runtime resource graph %s: %s", e.Path, e.Reason)
}

func (e *ValidationError) Unwrap() error {
	return ErrInvalidGraph
}

func invalid(path, reason string) error {
	return &ValidationError{Path: path, Reason: reason}
}

// LifecycleError identifies the resource and operation that failed while
// keeping provider diagnostics out of the public error tree. Unwrap returns a
// redacted classifier: errors.Is remains useful, but callers cannot recover a
// provider error object or its potentially sensitive Error text with Unwrap or
// errors.As.
type LifecycleError struct {
	resource  string
	operation Operation
	cause     error
}

func (e *LifecycleError) Error() string {
	if e == nil {
		return "runtime resource lifecycle failed"
	}
	if e.resource == "" {
		return fmt.Sprintf("runtime resource graph %s failed", e.operation)
	}
	return fmt.Sprintf("runtime resource %q %s failed", e.resource, e.operation)
}

// Format keeps alternate fmt verbs on the same redacted surface as Error.
// In particular, %#v must not reveal the recursively wrapped provider error.
func (e *LifecycleError) Format(state fmt.State, _ rune) {
	_, _ = fmt.Fprint(state, e.Error())
}

func (e *LifecycleError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.cause
}

// ResourceName returns the validated resource name, or an empty string for a
// graph-level failure.
func (e *LifecycleError) ResourceName() string {
	if e == nil {
		return ""
	}
	return e.resource
}

// LifecycleOperation returns the fixed lifecycle operation that failed.
func (e *LifecycleError) LifecycleOperation() Operation {
	if e == nil {
		return ""
	}
	return e.operation
}

func lifecycleError(resourceName string, operation Operation, cause error) error {
	if cause == nil {
		return nil
	}
	return &LifecycleError{
		resource:  resourceName,
		operation: operation,
		cause:     redactCause(cause),
	}
}

// redactedCause deliberately has no Unwrap or As method. Is delegates to the
// private provider tree so callers can compare a sentinel they already own,
// while recursive inspection stops at this fixed diagnostic surface.
type redactedCause struct {
	cause error
}

func (e *redactedCause) Error() string {
	return "runtime resource provider operation failed"
}

func (e *redactedCause) Format(state fmt.State, _ rune) {
	_, _ = fmt.Fprint(state, e.Error())
}

func (e *redactedCause) Is(target error) bool {
	return e != nil && target != nil && errors.Is(e.cause, target)
}

func redactCause(cause error) error {
	if cause == nil {
		return nil
	}
	return &redactedCause{cause: cause}
}

func joinLifecycleContext(result error, operation Operation, contextErr error) error {
	if contextErr == nil || errors.Is(result, contextErr) {
		return result
	}
	return errors.Join(result, lifecycleError("", operation, contextErr))
}

// filteredRunCause keeps errors.Is classification for provider causes
// while preventing graph-owned peer cancellation from masquerading as caller
// cancellation. Error and Format remain redacted, and the provider object is
// never exposed through Unwrap or errors.As.
type filteredRunCause struct {
	cause      error
	contextErr error
}

func (e *filteredRunCause) Error() string {
	return "runtime resource runner cleanup failed"
}

func (e *filteredRunCause) Format(state fmt.State, _ rune) {
	_, _ = fmt.Fprint(state, e.Error())
}

func (e *filteredRunCause) Is(target error) bool {
	if target == nil || errors.Is(e.contextErr, target) {
		return false
	}
	return errors.Is(e.cause, target)
}

func nonContextRunCause(cause error, contextErr error) error {
	if cause == nil || contextErr == nil || !errors.Is(cause, contextErr) {
		return cause
	}
	if contextOnlyErrorTree(cause, contextErr) {
		return nil
	}
	return &filteredRunCause{cause: cause, contextErr: contextErr}
}

func contextOnlyErrorTree(current error, contextErr error) bool {
	if current == nil {
		return false
	}
	if current == contextErr {
		return true
	}
	if multi, ok := current.(interface{ Unwrap() []error }); ok {
		children := multi.Unwrap()
		if len(children) == 0 {
			return false
		}
		for _, child := range children {
			if !contextOnlyErrorTree(child, contextErr) {
				return false
			}
		}
		return true
	}
	if single, ok := current.(interface{ Unwrap() error }); ok {
		return contextOnlyErrorTree(single.Unwrap(), contextErr)
	}
	return false
}
