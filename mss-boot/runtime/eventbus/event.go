package eventbus

import (
	"context"
	"encoding/json"
)

// Revision is an authoritative, monotonically increasing state revision.
// Zero is reserved for "no revision observed" and is never publishable.
type Revision uint64

// Event is a typed latest-state notification. Payload is a reload hint; the
// authoritative source remains the source of truth.
type Event[T any] struct {
	Revision Revision
	Payload  T
}

// Handler receives a best-effort notification. Returning an error or
// panicking degrades health but does not prevent other current subscribers
// from receiving the event.
type Handler[T any] func(context.Context, Event[T]) error

// EventBus is the common typed notification contract implemented by Memory
// and Redis. Provider-specific reconciliation scheduling remains explicit.
type EventBus[T any] interface {
	Start(context.Context) error
	Subscribe(Handler[T]) (*Subscription, error)
	Publish(context.Context, Event[T]) error
	LastRevision() Revision
	Ready(context.Context) error
	Health(context.Context) error
	Close(context.Context) error
}

// Codec controls the typed payload stored by the Redis provider.
type Codec[T any] interface {
	Marshal(T) ([]byte, error)
	Unmarshal([]byte) (T, error)
}

// JSONCodec is the default deterministic payload codec.
type JSONCodec[T any] struct{}

func (JSONCodec[T]) Marshal(value T) ([]byte, error) { return json.Marshal(value) }

func (JSONCodec[T]) Unmarshal(data []byte) (result T, err error) {
	err = json.Unmarshal(data, &result)
	return result, err
}

// AuthoritativeSource returns the current committed event. found=false means
// that the authoritative state has no revision yet.
type AuthoritativeSource[T any] interface {
	Latest(context.Context) (event Event[T], found bool, err error)
}

// AuthoritativeSourceFunc adapts a function to AuthoritativeSource.
type AuthoritativeSourceFunc[T any] func(context.Context) (Event[T], bool, error)

func (f AuthoritativeSourceFunc[T]) Latest(ctx context.Context) (Event[T], bool, error) {
	return f(ctx)
}

// Reconciler is a domain-neutral bridge to authoritative revision state.
// It owns no goroutine and performs work only when Reconcile is called.
type Reconciler[T any] struct {
	source AuthoritativeSource[T]
}

// BuildReconciler validates and retains an authoritative source without
// calling it.
func BuildReconciler[T any](source AuthoritativeSource[T]) (*Reconciler[T], error) {
	if source == nil {
		return nil, invalid("source", "is required", ErrInvalidReconciler)
	}
	return &Reconciler[T]{source: source}, nil
}

// Reconcile reads the authoritative latest event. A revision older than the
// bus's highest observed revision is ignored; an equal revision remains
// eligible so subscribers that joined late or previously failed can reload.
func (r *Reconciler[T]) Reconcile(ctx context.Context, observed Revision) (event Event[T], found bool, err error) {
	if ctx == nil {
		return event, false, operationError(OperationReconcile, ErrContextRequired, ErrContextRequired)
	}
	if contextErr := ctx.Err(); contextErr != nil {
		return event, false, operationError(OperationReconcile, ErrReconcileFailed, contextErr)
	}
	if r == nil || r.source == nil {
		return event, false, operationError(OperationReconcile, ErrInvalidReconciler, ErrInvalidReconciler)
	}

	panicked := true
	defer func() {
		if recover() != nil || panicked {
			event = Event[T]{}
			found = false
			err = operationError(OperationReconcile, ErrReconcilerPanicked, ErrReconcilerPanicked)
		}
	}()
	event, found, sourceErr := r.source.Latest(ctx)
	panicked = false
	if sourceErr != nil {
		return Event[T]{}, false, operationError(OperationReconcile, ErrReconcileFailed, sourceErr)
	}
	if contextErr := ctx.Err(); contextErr != nil {
		return Event[T]{}, false, operationError(OperationReconcile, ErrReconcileFailed, contextErr)
	}
	if !found {
		return Event[T]{}, false, nil
	}
	if event.Revision == 0 {
		return Event[T]{}, false, operationError(OperationReconcile, ErrReconcileFailed, ErrInvalidEvent)
	}
	if event.Revision < observed {
		return Event[T]{}, false, nil
	}
	return event, true, nil
}
