package storage

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/bsm/redislock"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

const (
	PrefixKey = "__host"
)

var (
	// ErrInvalidConfiguration classifies an adapter failure caused by a local,
	// deterministic configuration defect. Callers must fail closed rather than
	// treating this class as an optional dependency outage.
	ErrInvalidConfiguration = errors.New("storage adapter configuration is invalid")
	// ErrDependencyUnavailable classifies a validated adapter profile that
	// could not reach or construct its external dependency.
	ErrDependencyUnavailable = errors.New("storage adapter dependency is unavailable")
)

// InvalidConfigurationError preserves the underlying validation error while
// giving composition roots a stable errors.Is classification.
type InvalidConfigurationError struct {
	Adapter string
	Err     error
}

func (e *InvalidConfigurationError) Error() string {
	if e == nil {
		return ErrInvalidConfiguration.Error()
	}
	if e.Adapter == "" {
		return fmt.Sprintf("%s: %v", ErrInvalidConfiguration, e.Err)
	}
	return fmt.Sprintf("%s %s: %v", e.Adapter, ErrInvalidConfiguration, e.Err)
}

func (e *InvalidConfigurationError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func (*InvalidConfigurationError) Is(target error) bool {
	return target == ErrInvalidConfiguration
}

// DependencyUnavailableError preserves the provider error while allowing an
// owner to degrade only this explicit optional-dependency failure class.
type DependencyUnavailableError struct {
	Adapter string
	Err     error
}

func (e *DependencyUnavailableError) Error() string {
	if e == nil {
		return ErrDependencyUnavailable.Error()
	}
	if e.Adapter == "" {
		return fmt.Sprintf("%s: %v", ErrDependencyUnavailable, e.Err)
	}
	return fmt.Sprintf("%s %s: %v", e.Adapter, ErrDependencyUnavailable, e.Err)
}

func (e *DependencyUnavailableError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func (*DependencyUnavailableError) Is(target error) bool {
	return target == ErrDependencyUnavailable
}

type AdapterCache interface {
	redis.UniversalClient
	Name() string
	String() string
	Initialize(*gorm.DB) error
	RemoveFromTag(ctx context.Context, tag string) error
}

type AdapterQueue interface {
	String() string
	Append(opts ...Option) error
	Register(opts ...Option)
	Run(context.Context)
	Shutdown()
}

// ManagedAdapterQueue is the additive lifecycle contract for queue adapters
// that can report startup and registration failures without terminating the
// process. AdapterQueue remains embedded so existing integrations continue to
// compile while owners migrate to the context-aware methods.
type ManagedAdapterQueue interface {
	AdapterQueue
	RegisterContext(context.Context, ...Option) error
	Start(context.Context) error
	Errors() <-chan error
	Close(context.Context) error
}

// ManagedAdapterQueueCloseState is an optional lifecycle capability used by
// owners to distinguish a completed close that returned diagnostics from a
// context timeout while close work is still in flight.
type ManagedAdapterQueueCloseState interface {
	CloseComplete() bool
}

type Messager interface {
	SetID(string)
	SetStream(string)
	SetValues(map[string]any)
	GetID() string
	GetStream() string
	GetValues() map[string]any
	GetPrefix() string
	SetPrefix(string)
	SetErrorCount(count int)
	GetErrorCount() int
	SetContext(ctx context.Context)
	GetContext() context.Context
}

type ConsumerFunc func(Messager) error

type AdapterLocker interface {
	String() string
	Lock(ctx context.Context, key string, ttl time.Duration, options *redislock.Options) (*redislock.Lock, error)
}
