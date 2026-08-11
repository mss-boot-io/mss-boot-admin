package eventbus

import (
	"context"
	"errors"
	"fmt"
)

var (
	ErrInvalidBus         = errors.New("invalid revision event bus")
	ErrInvalidOptions     = errors.New("invalid revision event bus options")
	ErrInvalidEvent       = errors.New("invalid revision event")
	ErrInvalidReconciler  = errors.New("invalid revision event reconciler")
	ErrContextRequired    = errors.New("revision event bus context is required")
	ErrStartRejected      = errors.New("revision event bus start rejected")
	ErrRunRejected        = errors.New("revision event bus run rejected")
	ErrNotStarted         = errors.New("revision event bus is not started")
	ErrClosing            = errors.New("revision event bus is closing")
	ErrDegraded           = errors.New("revision event bus is degraded")
	ErrProviderFailed     = errors.New("revision event provider failed")
	ErrPublishFailed      = errors.New("revision event publish failed")
	ErrPollFailed         = errors.New("revision event poll failed")
	ErrEncodeFailed       = errors.New("revision event encoding failed")
	ErrDecodeFailed       = errors.New("revision event decoding failed")
	ErrSubscriberFailed   = errors.New("revision event subscriber failed")
	ErrSubscriberPanicked = errors.New("revision event subscriber panicked")
	ErrReconcileFailed    = errors.New("revision event reconciliation failed")
	ErrReconcilerPanicked = errors.New("revision event reconciler panicked")
)

// Operation is a fixed diagnostic boundary and never contains provider,
// subscriber, or payload input.
type Operation string

const (
	OperationStart     Operation = "start"
	OperationRun       Operation = "run"
	OperationReady     Operation = "ready"
	OperationHealth    Operation = "health"
	OperationClose     Operation = "close"
	OperationSubscribe Operation = "subscribe"
	OperationPublish   Operation = "publish"
	OperationPoll      Operation = "poll"
	OperationReconcile Operation = "reconcile"
	OperationEncode    Operation = "encode"
	OperationDecode    Operation = "decode"
)

// ValidationError identifies an invalid public option without retaining its
// value or any provider data.
type ValidationError struct {
	Path   string
	Reason string
	class  error
}

func (e *ValidationError) Error() string {
	if e == nil {
		return ErrInvalidOptions.Error()
	}
	return fmt.Sprintf("revision event bus %s: %s", e.Path, e.Reason)
}

func (e *ValidationError) Unwrap() error {
	if e == nil || e.class == nil {
		return ErrInvalidOptions
	}
	return e.class
}

func invalid(path, reason string, class error) error {
	return &ValidationError{Path: path, Reason: reason, class: class}
}

// OperationError exposes only a controlled operation and fixed error
// classifications. Arbitrary provider, codec, source, and handler errors are
// never retained in the public unwrap chain.
type OperationError struct {
	operation Operation
	safe      error
}

func (e *OperationError) Error() string {
	if e == nil {
		return "revision event bus operation failed"
	}
	return fmt.Sprintf("revision event bus %s failed", e.operation)
}

func (e *OperationError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.safe
}

func (e *OperationError) Is(target error) bool {
	return e != nil && errors.Is(e.safe, target)
}

func (e *OperationError) Operation() Operation {
	if e == nil {
		return ""
	}
	return e.operation
}

func operationError(operation Operation, class, cause error) error {
	return &OperationError{
		operation: operation,
		safe:      errors.Join(class, safeClassifications(cause)),
	}
}

func safeClassifications(cause error) error {
	if cause == nil {
		return nil
	}
	controlled := []error{
		ErrInvalidBus,
		ErrInvalidOptions,
		ErrInvalidEvent,
		ErrInvalidReconciler,
		ErrContextRequired,
		ErrStartRejected,
		ErrRunRejected,
		ErrNotStarted,
		ErrClosing,
		ErrDegraded,
		ErrProviderFailed,
		ErrPublishFailed,
		ErrPollFailed,
		ErrEncodeFailed,
		ErrDecodeFailed,
		ErrSubscriberFailed,
		ErrSubscriberPanicked,
		ErrReconcileFailed,
		ErrReconcilerPanicked,
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
