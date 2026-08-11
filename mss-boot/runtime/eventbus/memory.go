package eventbus

import (
	"context"

	runtimeresource "github.com/mss-boot-io/mss-boot-admin/mss-boot/runtime/resource"
)

// Memory is a process-local EventBus. Publish synchronously fans out to every
// subscriber registered when the notification is accepted.
type Memory[T any] struct {
	core *core[T]
}

// BuildMemory allocates process-local state only.
func BuildMemory[T any]() *Memory[T] {
	return &Memory[T]{core: newCore[T]()}
}

func (b *Memory[T]) Start(ctx context.Context) error {
	if b == nil {
		return operationError(OperationStart, ErrInvalidBus, ErrInvalidBus)
	}
	return b.core.start(ctx, nil)
}

func (b *Memory[T]) Subscribe(handler Handler[T]) (*Subscription, error) {
	if b == nil {
		return nil, operationError(OperationSubscribe, ErrInvalidBus, ErrInvalidBus)
	}
	return b.core.subscribe(handler)
}

func (b *Memory[T]) Publish(ctx context.Context, event Event[T]) error {
	if b == nil {
		return operationError(OperationPublish, ErrInvalidBus, ErrInvalidBus)
	}
	return b.core.deliver(ctx, event, deliveryNotification)
}

// Reconcile reloads the authoritative current revision and fans it out to
// subscribers that have not successfully observed it.
func (b *Memory[T]) Reconcile(ctx context.Context, reconciler *Reconciler[T]) error {
	if b == nil {
		return operationError(OperationReconcile, ErrInvalidBus, ErrInvalidBus)
	}
	release, err := b.core.beginActive(ctx, OperationReconcile)
	if err != nil {
		return err
	}
	defer release()
	if reconciler == nil {
		return operationError(OperationReconcile, ErrInvalidReconciler, ErrInvalidReconciler)
	}
	event, found, err := reconciler.Reconcile(ctx, b.core.last())
	if err != nil || !found {
		return err
	}
	return b.core.deliverActive(ctx, event, deliveryReconciliation)
}

func (b *Memory[T]) LastRevision() Revision {
	if b == nil {
		return 0
	}
	return b.core.last()
}

func (b *Memory[T]) Ready(ctx context.Context) error {
	if b == nil {
		return operationError(OperationReady, ErrInvalidBus, ErrInvalidBus)
	}
	return b.core.inspection(ctx, OperationReady)
}

func (b *Memory[T]) Health(ctx context.Context) error {
	if b == nil {
		return operationError(OperationHealth, ErrInvalidBus, ErrInvalidBus)
	}
	return b.core.inspection(ctx, OperationHealth)
}

func (b *Memory[T]) Close(ctx context.Context) error {
	if b == nil {
		return nil
	}
	return b.core.close(ctx)
}

var (
	_ EventBus[struct{}]               = (*Memory[struct{}])(nil)
	_ runtimeresource.Resource         = (*Memory[struct{}])(nil)
	_ runtimeresource.HealthChecker    = (*Memory[struct{}])(nil)
	_ runtimeresource.ReadinessChecker = (*Memory[struct{}])(nil)
)
