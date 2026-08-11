// Package redisbridge is the sealed adapter between Runtime v2 Redis resources
// and runtime-owned capabilities that require fixed, same-slot Redis scripts.
//
// It is internal to runtime: application consumers cannot import it, select a
// physical key or hash tag, borrow the provider client, or submit arbitrary Lua.
package redisbridge

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"unicode/utf8"
)

var (
	ErrInvalidGroup  = errors.New("invalid Redis atomic group")
	ErrInvalidKey    = errors.New("invalid Redis atomic key")
	ErrCrossGroup    = errors.New("Redis atomic key crosses groups")
	ErrInvalidScript = errors.New("invalid fixed Redis script")
)

// AtomicGroup is an opaque server-owned same-slot capability. Its tag is
// derived, never selected or observed by the runtime capability consumer.
type AtomicGroup struct {
	tag   string
	valid bool
}

// NewAtomicGroup deterministically derives a group from a fixed runtime domain
// and opaque identity. The identity never appears in the physical key.
func NewAtomicGroup(domain string, identity []byte) (AtomicGroup, error) {
	if !validName(domain) || len(identity) == 0 || len(identity) > 512 {
		return AtomicGroup{}, ErrInvalidGroup
	}
	digest := sha256.New()
	_, _ = digest.Write([]byte("mss/runtime/redis-atomic-group/v1"))
	_, _ = digest.Write([]byte{0})
	_, _ = digest.Write([]byte(domain))
	_, _ = digest.Write([]byte{0})
	_, _ = digest.Write(identity)
	return AtomicGroup{
		tag:   base64.RawURLEncoding.EncodeToString(digest.Sum(nil)),
		valid: true,
	}, nil
}

func (g AtomicGroup) String() string {
	if !g.valid {
		return "RedisAtomicGroup<invalid>"
	}
	return "RedisAtomicGroup<opaque>"
}

func (g AtomicGroup) GoString() string { return g.String() }

// Key creates an opaque key capability within the group. The logical name is
// validated but neither it nor the eventual physical value is formattable.
func (g AtomicGroup) Key(logical string) (Key, error) {
	if !g.valid {
		return Key{}, ErrInvalidGroup
	}
	if !validName(logical) {
		return Key{}, ErrInvalidKey
	}
	return Key{tag: g.tag, logical: logical, valid: true}, nil
}

// Key is intentionally opaque outside this internal package.
type Key struct {
	tag     string
	logical string
	valid   bool
}

func (k Key) String() string {
	if !k.valid {
		return "RedisAtomicKey<invalid>"
	}
	return "RedisAtomicKey<opaque>"
}

func (k Key) GoString() string { return k.String() }

// Script is a sealed token for one source-compiled operation. There is no
// public constructor and no accessor for its Lua source.
type Script struct {
	id scriptID
}

func (s Script) String() string {
	if !s.id.valid() {
		return "RedisFixedScript<invalid>"
	}
	return "RedisFixedScript<opaque>"
}

func (s Script) GoString() string { return s.String() }

// Reply exposes only provider-neutral fixed-script result shapes.
type Reply struct {
	value any
}

func (r Reply) Text() (string, error) {
	value, ok := r.value.(string)
	if !ok {
		return "", errors.New("fixed Redis script returned an unexpected result")
	}
	return value, nil
}

func (r Reply) Strings() ([]string, error) {
	values, ok := r.value.([]any)
	if !ok {
		return nil, errors.New("fixed Redis script returned an unexpected result")
	}
	result := make([]string, len(values))
	for index, value := range values {
		if value == nil {
			continue
		}
		text, ok := value.(string)
		if !ok {
			return nil, errors.New("fixed Redis script returned an unexpected result")
		}
		result[index] = text
	}
	return result, nil
}

// Source is implemented by redisresource.Scope. Its internal-package method
// signature keeps the provider borrow inaccessible to application packages.
type Source interface {
	RedisBridgeUse(context.Context, func(Driver) error) error
}

// Driver is valid only during Source.RedisBridgeUse's structured callback.
type Driver interface {
	RedisBridgeRun(context.Context, Request) (Reply, error)
}

// Request is created only after group/key/script validation. Consumers never
// receive it; the resource-side Driver uses Execute to run the sealed source.
type Request struct {
	group  AtomicGroup
	keys   []Key
	script Script
	args   []any
}

func (Request) String() string               { return "RedisFixedRequest<opaque>" }
func (r Request) GoString() string           { return r.String() }
func (r Request) LogValue() slog.Value       { return slog.StringValue(r.String()) }
func (Request) MarshalJSON() ([]byte, error) { return []byte(`"RedisFixedRequest<opaque>"`), nil }
func (r Request) MarshalYAML() (any, error)  { return r.String(), nil }

// Executor is the narrow provider operation implemented by redisresource.
type Executor interface {
	RedisBridgeEvalFixed(context.Context, string, []string, ...any) (any, error)
}

// Execute materializes provider keys only inside the resource-side driver.
func (r Request) Execute(ctx context.Context, prefix string, executor Executor) (Reply, error) {
	if ctx == nil || executor == nil || prefix == "" || !r.group.valid || !r.script.id.valid() || len(r.keys) == 0 {
		return Reply{}, ErrInvalidScript
	}
	keys := make([]string, len(r.keys))
	for index, key := range r.keys {
		if !key.valid || key.tag != r.group.tag {
			return Reply{}, ErrCrossGroup
		}
		keys[index] = prefix + ":{" + r.group.tag + "}:" + key.logical
	}
	value, err := executor.RedisBridgeEvalFixed(ctx, scriptSource(r.script.id), keys, r.args...)
	if err != nil {
		return Reply{}, err
	}
	return Reply{value: value}, nil
}

// Lease is bound to exactly one AtomicGroup and expires with the source borrow.
type Lease interface {
	Run(context.Context, Script, []Key, ...any) (Reply, error)
}

type lease struct {
	group  AtomicGroup
	driver Driver
}

// Use borrows a group-bound lease from the source. Cross-group keys and an
// invalid script are rejected before the driver, and therefore before I/O.
func Use(ctx context.Context, source Source, group AtomicGroup, callback func(Lease) error) error {
	if ctx == nil || source == nil || !group.valid || callback == nil {
		return ErrInvalidGroup
	}
	return source.RedisBridgeUse(ctx, func(driver Driver) error {
		if driver == nil {
			return ErrInvalidGroup
		}
		return callback(&lease{group: group, driver: driver})
	})
}

func (l *lease) Run(ctx context.Context, script Script, keys []Key, args ...any) (Reply, error) {
	if l == nil || l.driver == nil || !l.group.valid || ctx == nil || !script.id.valid() || len(keys) == 0 {
		return Reply{}, ErrInvalidScript
	}
	copied := append([]Key(nil), keys...)
	for _, key := range copied {
		if !key.valid || key.tag != l.group.tag {
			return Reply{}, ErrCrossGroup
		}
	}
	return l.driver.RedisBridgeRun(ctx, Request{
		group:  l.group,
		keys:   copied,
		script: script,
		args:   append([]any(nil), args...),
	})
}

func validName(value string) bool {
	if value == "" || len(value) > 96 || !utf8.ValidString(value) || strings.TrimSpace(value) != value {
		return false
	}
	for _, character := range value {
		if character >= 'a' && character <= 'z' || character >= '0' && character <= '9' || character == '-' || character == '_' || character == '.' {
			continue
		}
		return false
	}
	return true
}

func (r Reply) String() string   { return "RedisFixedReply<opaque>" }
func (r Reply) GoString() string { return r.String() }

var _ fmt.Stringer = AtomicGroup{}
