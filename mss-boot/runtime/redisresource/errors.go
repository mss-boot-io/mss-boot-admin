package redisresource

import (
	"context"
	"errors"
	"fmt"
	"time"
)

var (
	ErrInvalidProfile     = errors.New("invalid Redis runtime profile")
	ErrContextRequired    = errors.New("Redis runtime context is required")
	ErrClientConstruction = errors.New("Redis runtime client construction failed")
	ErrUnavailable        = errors.New("Redis runtime resource unavailable")
	ErrStartRejected      = errors.New("Redis runtime resource start rejected")
	ErrNotStarted         = errors.New("Redis runtime resource is not started")
	ErrClosing            = errors.New("Redis runtime resource is closing")
	ErrCloseFailed        = errors.New("Redis runtime resource close failed")
	ErrUseRejected        = errors.New("Redis runtime resource use rejected")
	ErrDetachedCommand    = errors.New("Redis runtime detached lease command")
	ErrCommandFailed      = errors.New("Redis runtime command failed")
	ErrInvalidCommand     = errors.New("invalid Redis runtime command")
	ErrNotFound           = errors.New("Redis runtime key not found")
	ErrLeaseExpired       = errors.New("Redis runtime lease expired")
	ErrUnsafeKey          = errors.New("unsafe Redis logical key")
)

// Operation is a fixed diagnostic boundary. It never contains provider input.
type Operation string

const (
	OperationStart   Operation = "start"
	OperationReady   Operation = "ready"
	OperationHealth  Operation = "health"
	OperationClose   Operation = "close"
	OperationUse     Operation = "use"
	OperationScope   Operation = "scope"
	OperationGet     Operation = "get"
	OperationSet     Operation = "set"
	OperationDelete  Operation = "delete"
	OperationExists  Operation = "exists"
	OperationQualify Operation = "qualify-key"
)

// ValidationError describes profile structure without retaining a rejected
// endpoint, namespace, key, credential, certificate, or provider diagnostic.
type ValidationError struct {
	Path   string
	Reason string
	class  error
}

func (e *ValidationError) Error() string {
	if e == nil {
		return ErrInvalidProfile.Error()
	}
	return fmt.Sprintf("Redis runtime %s: %s", e.Path, e.Reason)
}

func (e *ValidationError) Format(state fmt.State, _ rune) {
	_, _ = fmt.Fprint(state, e.Error())
}

func (e *ValidationError) Unwrap() error {
	if e == nil || e.class == nil {
		return ErrInvalidProfile
	}
	return e.class
}

func invalid(path, reason string) error {
	return &ValidationError{Path: path, Reason: reason, class: ErrInvalidProfile}
}

func unsafeKey(reason string) error {
	return &ValidationError{Path: "logicalKey", Reason: reason, class: ErrUnsafeKey}
}

func invalidCommand(path, reason string) error {
	return &ValidationError{Path: path, Reason: reason, class: ErrInvalidCommand}
}

// LifecycleError exposes only a controlled operation. Unwrap and Is retain
// package sentinels plus cancellation/deadline classification. Provider and
// callback error objects are projected away because their text or concrete
// fields may contain connection, credential, or command details.
type LifecycleError struct {
	operation Operation
	safe      error
}

func (e *LifecycleError) Error() string {
	if e == nil {
		return "Redis runtime operation failed"
	}
	return fmt.Sprintf("Redis runtime %s failed", e.operation)
}

func (e *LifecycleError) Format(state fmt.State, _ rune) {
	_, _ = fmt.Fprint(state, e.Error())
}

func (e *LifecycleError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.safe
}

func (e *LifecycleError) Is(target error) bool {
	if e == nil {
		return false
	}
	return errors.Is(e.safe, target)
}

func (e *LifecycleError) LifecycleOperation() Operation {
	if e == nil {
		return ""
	}
	return e.operation
}

func lifecycleError(operation Operation, class, cause error) error {
	return &LifecycleError{
		operation: operation,
		safe:      errors.Join(class, safeClassifications(cause)),
	}
}

// safeClassifications projects an arbitrary provider/callback error onto the
// package's fixed public sentinels and the two standard context outcomes. The
// original object and its text never enter the public unwrap chain.
func safeClassifications(cause error) error {
	if cause == nil {
		return nil
	}
	controlled := []error{
		ErrInvalidProfile,
		ErrContextRequired,
		ErrClientConstruction,
		ErrUnavailable,
		ErrStartRejected,
		ErrNotStarted,
		ErrClosing,
		ErrCloseFailed,
		ErrUseRejected,
		ErrDetachedCommand,
		ErrCommandFailed,
		ErrInvalidCommand,
		ErrNotFound,
		ErrLeaseExpired,
		ErrUnsafeKey,
		context.Canceled,
		context.DeadlineExceeded,
	}
	result := make([]error, 0, 2)
	for _, classification := range controlled {
		if errors.Is(cause, classification) {
			result = append(result, classification)
		}
	}
	return errors.Join(result...)
}

func contextClassification(ctx context.Context) error {
	if ctx == nil {
		return nil
	}
	if cause := context.Cause(ctx); cause != nil {
		return cause
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if deadline, ok := ctx.Deadline(); ok && !time.Now().Before(deadline) {
		// A transport deadline can become observable immediately before the
		// context timer publishes Err. The caller's declared deadline remains
		// the stable public classification in that narrow race.
		return context.DeadlineExceeded
	}
	return nil
}
