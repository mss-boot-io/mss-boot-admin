package cache

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	"github.com/mss-boot-io/mss-boot-admin/mss-boot/runtime/redisresource"
)

const (
	baseGeneration = "base"
	wireVersion    = 1
)

// Result is the provider-neutral query result persisted by Derived. NotFound
// is distinct from an empty successful result, and RowsAffected is retained
// exactly across authority and cache paths.
type Result struct {
	Payload      []byte
	RowsAffected int64
	NotFound     bool
}

func (r Result) validate() error {
	if r.RowsAffected < 0 {
		return validationError(ErrInvalidResult, "rowsAffected", "must not be negative")
	}
	if r.NotFound && (r.RowsAffected != 0 || len(r.Payload) != 0) {
		return validationError(ErrInvalidResult, "notFound", "must have zero rows and no payload")
	}
	return nil
}

func cloneResult(result Result) Result {
	result.Payload = append([]byte(nil), result.Payload...)
	return result
}

// Source identifies the authoritative origin of an outcome.
type Source string

const (
	SourceCache     Source = "cache"
	SourceAuthority Source = "authority"
)

// Status explains the cache-side disposition without exposing Redis details.
type Status string

const (
	StatusHit               Status = "hit"
	StatusStored            Status = "stored"
	StatusProviderBypass    Status = "provider-bypass"
	StatusPayloadBypass     Status = "payload-bypass"
	StatusTransactionBypass Status = "transaction-bypass"
	StatusInvalidated       Status = "invalidated"
)

// Outcome always contains an independent payload copy.
type Outcome struct {
	Result Result
	Source Source
	Status Status
}

// Loader reconstructs one result from its declared database authority.
type Loader func(context.Context) (Result, error)

type wireEntry struct {
	Version      int    `json:"v"`
	Payload      []byte `json:"p,omitempty"`
	RowsAffected int64  `json:"r"`
	NotFound     bool   `json:"n,omitempty"`
}

type flight struct {
	ctx       context.Context
	cancel    context.CancelCauseFunc
	stopLife  func() bool
	done      chan struct{}
	waiters   int
	completed bool
	target    Target
	loader    Loader
	outcome   Outcome
	err       error
}

// Derived is a scoped, database-authoritative cache. It owns only local
// singleflight and invalidation-repair state; the supplied Scope retains all
// provider lifecycle ownership.
type Derived struct {
	scope  *redisresource.Scope
	policy Policy

	life   context.Context
	cancel context.CancelCauseFunc

	mu          sync.Mutex
	closed      bool
	drain       chan struct{}
	drainClosed bool
	active      int
	flights     map[string]*flight
	epochs      map[string]uint64
	pending     map[string]string
}

// NewDerived validates and copies policy without provider I/O.
func NewDerived(scope *redisresource.Scope, policy Policy) (*Derived, error) {
	if scope == nil {
		return nil, validationError(ErrInvalidPolicy, "scope", "is required")
	}
	if err := policy.validate(); err != nil {
		return nil, err
	}
	life, cancel := context.WithCancelCause(context.Background())
	return &Derived{
		scope:   scope,
		policy:  policy,
		life:    life,
		cancel:  cancel,
		drain:   make(chan struct{}),
		flights: make(map[string]*flight),
		epochs:  make(map[string]uint64),
		pending: make(map[string]string),
	}, nil
}

// Load returns a cache hit or reconstructs from loader. Redis misses,
// corruption, and provider failures never replace a successful loader result.
func (c *Derived) Load(ctx context.Context, target Target, loader Loader) (Outcome, error) {
	if ctx == nil {
		return Outcome{}, validationError(ErrInvalidTarget, "context", "is required")
	}
	if err := ctx.Err(); err != nil {
		return Outcome{}, err
	}
	if c == nil {
		return Outcome{}, ErrClosed
	}
	if loader == nil {
		return Outcome{}, validationError(ErrInvalidTarget, "loader", "is required")
	}
	if err := target.validate(); err != nil {
		return Outcome{}, err
	}
	datasetID, queryID := targetIDs(target)
	cacheFlight, err := c.acquireFlight(ctx, datasetID, queryID, target, loader)
	if err != nil {
		return Outcome{}, err
	}
	defer c.releaseWaiter(cacheFlight)

	select {
	case <-cacheFlight.done:
		return cloneOutcome(cacheFlight.outcome), cacheFlight.err
	case <-ctx.Done():
		return Outcome{}, ctx.Err()
	case <-c.life.Done():
		return Outcome{}, ErrClosed
	}
}

func (c *Derived) acquireFlight(caller context.Context, datasetID, queryID string, target Target, loader Loader) (*flight, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil, ErrClosed
	}
	epoch := c.epochs[datasetID]
	flightID := fmt.Sprintf("%s/%s/%d", datasetID, queryID, epoch)
	if existing := c.flights[flightID]; existing != nil {
		existing.waiters++
		return existing, nil
	}
	// Keep the first caller's values for tenant-aware GORM hooks while making
	// cancellation a property of all waiters: one canceled waiter must not end
	// a load still awaited by another caller.
	flightContext, cancel := context.WithCancelCause(context.WithoutCancel(caller))
	stopLife := context.AfterFunc(c.life, func() { cancel(ErrClosed) })
	created := &flight{
		ctx:      flightContext,
		cancel:   cancel,
		stopLife: stopLife,
		done:     make(chan struct{}),
		waiters:  1,
		target:   target,
		loader:   loader,
	}
	c.flights[flightID] = created
	c.active++
	go c.runFlight(flightID, created, datasetID, queryID)
	return created, nil
}

func (c *Derived) runFlight(flightID string, current *flight, datasetID, queryID string) {
	defer func() {
		current.stopLife()
		current.cancel(context.Canceled)
		c.mu.Lock()
		current.completed = true
		if c.flights[flightID] == current {
			delete(c.flights, flightID)
		}
		c.active--
		if c.closed && c.active == 0 && !c.drainClosed {
			close(c.drain)
			c.drainClosed = true
		}
		close(current.done)
		c.mu.Unlock()
	}()
	current.outcome, current.err = c.execute(current.ctx, current.target, datasetID, queryID, current.loader)
}

func (c *Derived) releaseWaiter(current *flight) {
	c.mu.Lock()
	defer c.mu.Unlock()
	current.waiters--
	if current.waiters == 0 && !current.completed {
		current.cancel(context.Canceled)
	}
}

func (c *Derived) execute(ctx context.Context, target Target, datasetID, queryID string, loader Loader) (Outcome, error) {
	generation, providerOK, err := c.generation(ctx, datasetID)
	if err != nil {
		return Outcome{}, err
	}
	if !providerOK {
		return c.fromAuthority(ctx, loader, StatusProviderBypass)
	}

	result, hit, providerOK, err := c.read(ctx, datasetID, queryID, generation)
	if err != nil {
		return Outcome{}, err
	}
	if hit {
		return Outcome{Result: cloneResult(result), Source: SourceCache, Status: StatusHit}, nil
	}
	if !providerOK {
		return c.fromAuthority(ctx, loader, StatusProviderBypass)
	}

	result, err = loader(ctx)
	if err != nil {
		return Outcome{}, err
	}
	if err := result.validate(); err != nil {
		return Outcome{}, err
	}
	result = cloneResult(result)
	if len(result.Payload) > c.policy.MaxPayloadBytes {
		return Outcome{Result: result, Source: SourceAuthority, Status: StatusPayloadBypass}, nil
	}
	if err := c.write(ctx, datasetID, queryID, generation, result); err != nil {
		if ctx.Err() != nil {
			return Outcome{}, ctx.Err()
		}
		return Outcome{Result: result, Source: SourceAuthority, Status: StatusProviderBypass}, nil
	}
	return Outcome{Result: result, Source: SourceAuthority, Status: StatusStored}, nil
}

func (c *Derived) fromAuthority(ctx context.Context, loader Loader, status Status) (Outcome, error) {
	result, err := loader(ctx)
	if err != nil {
		return Outcome{}, err
	}
	if err := result.validate(); err != nil {
		return Outcome{}, err
	}
	if ctx.Err() != nil {
		return Outcome{}, ctx.Err()
	}
	return Outcome{Result: cloneResult(result), Source: SourceAuthority, Status: status}, nil
}

// authorityBypass runs a non-shared authority operation under this adapter's
// close boundary. QueryCache uses it for active transactions so Close still
// cancels and drains work even though no shared provider command is allowed.
func (c *Derived) authorityBypass(ctx context.Context, loader Loader, status Status) (Outcome, error) {
	operationContext, finish, err := c.beginDirect(ctx)
	if err != nil {
		return Outcome{}, err
	}
	defer finish()
	outcome, err := c.fromAuthority(operationContext, loader, status)
	if operationErr := directContextError(ctx, operationContext); operationErr != nil {
		return Outcome{}, operationErr
	}
	return outcome, err
}

func (c *Derived) generation(ctx context.Context, datasetID string) (string, bool, error) {
	if pending := c.pendingGeneration(datasetID); pending != "" {
		if err := c.setGeneration(ctx, datasetID, pending); err != nil {
			if ctx.Err() != nil {
				return "", false, ctx.Err()
			}
			return "", false, nil
		}
		c.clearPending(datasetID, pending)
	}

	generation := baseGeneration
	err := c.scope.Use(ctx, func(lease redisresource.Lease) error {
		key, err := lease.QualifyKey(c.generationKey(datasetID))
		if err != nil {
			return err
		}
		value, err := lease.Get(ctx, key)
		if errors.Is(err, redisresource.ErrNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		candidate := string(value)
		if !validGeneration(candidate) {
			return redisresource.ErrCommandFailed
		}
		generation = candidate
		return nil
	})
	if err != nil {
		if ctx.Err() != nil {
			return "", false, ctx.Err()
		}
		return "", false, nil
	}
	return generation, true, nil
}

func (c *Derived) read(ctx context.Context, datasetID, queryID, generation string) (Result, bool, bool, error) {
	var encoded []byte
	err := c.scope.Use(ctx, func(lease redisresource.Lease) error {
		key, err := lease.QualifyKey(c.entryKey(datasetID, queryID, generation))
		if err != nil {
			return err
		}
		encoded, err = lease.Get(ctx, key)
		if errors.Is(err, redisresource.ErrNotFound) {
			encoded = nil
			return nil
		}
		return err
	})
	if err != nil {
		if ctx.Err() != nil {
			return Result{}, false, false, ctx.Err()
		}
		return Result{}, false, false, nil
	}
	if encoded == nil {
		return Result{}, false, true, nil
	}
	if len(encoded) > c.policy.MaxPayloadBytes*2+256 {
		return Result{}, false, true, nil
	}
	var entry wireEntry
	if err := json.Unmarshal(encoded, &entry); err != nil || entry.Version != wireVersion {
		return Result{}, false, true, nil
	}
	result := Result{Payload: entry.Payload, RowsAffected: entry.RowsAffected, NotFound: entry.NotFound}
	if len(result.Payload) > c.policy.MaxPayloadBytes || result.validate() != nil {
		return Result{}, false, true, nil
	}
	return cloneResult(result), true, true, nil
}

func (c *Derived) write(ctx context.Context, datasetID, queryID, generation string, result Result) error {
	encoded, err := json.Marshal(wireEntry{
		Version:      wireVersion,
		Payload:      result.Payload,
		RowsAffected: result.RowsAffected,
		NotFound:     result.NotFound,
	})
	if err != nil {
		return err
	}
	return c.scope.Use(ctx, func(lease redisresource.Lease) error {
		key, err := lease.QualifyKey(c.entryKey(datasetID, queryID, generation))
		if err != nil {
			return err
		}
		return lease.Set(ctx, key, encoded, c.policy.TTL)
	})
}

// Invalidate switches the dataset generation. Old entries become unreachable
// immediately after a successful provider write and expire by TTL. A provider
// failure is reported as StatusProviderBypass with a nil error so callers do
// not reinterpret an already committed database write as failed. This cache
// instance retains the pending generation and repairs it on its next read.
func (c *Derived) Invalidate(ctx context.Context, dataset Dataset) (Status, error) {
	if ctx == nil {
		return "", validationError(ErrInvalidTarget, "context", "is required")
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if c == nil {
		return "", ErrClosed
	}
	if err := dataset.validate(); err != nil {
		return "", err
	}
	operationContext, finish, err := c.beginDirect(ctx)
	if err != nil {
		return "", err
	}
	defer finish()
	tokenBytes := make([]byte, 16)
	if _, err := rand.Read(tokenBytes); err != nil {
		return "", errors.New("derived cache generation failed")
	}
	token := hex.EncodeToString(tokenBytes)
	datasetID := datasetIdentity(dataset)

	c.mu.Lock()
	c.epochs[datasetID]++
	c.pending[datasetID] = token
	c.mu.Unlock()

	if err := c.setGeneration(operationContext, datasetID, token); err != nil {
		if operationErr := directContextError(ctx, operationContext); operationErr != nil {
			return "", operationErr
		}
		return StatusProviderBypass, nil
	}
	c.clearPending(datasetID, token)
	return StatusInvalidated, nil
}

func (c *Derived) beginDirect(ctx context.Context) (context.Context, func(), error) {
	if c == nil {
		return nil, nil, ErrClosed
	}
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil, nil, ErrClosed
	}
	c.active++
	c.mu.Unlock()

	operationContext, cancel := context.WithCancelCause(ctx)
	stopLife := context.AfterFunc(c.life, func() { cancel(ErrClosed) })
	var once sync.Once
	finish := func() {
		once.Do(func() {
			stopLife()
			cancel(context.Canceled)
			c.mu.Lock()
			c.active--
			if c.closed && c.active == 0 && !c.drainClosed {
				close(c.drain)
				c.drainClosed = true
			}
			c.mu.Unlock()
		})
	}
	return operationContext, finish, nil
}

func directContextError(caller, operation context.Context) error {
	if errors.Is(context.Cause(operation), ErrClosed) {
		return ErrClosed
	}
	if caller != nil && caller.Err() != nil {
		return caller.Err()
	}
	return nil
}

func (c *Derived) setGeneration(ctx context.Context, datasetID, generation string) error {
	return c.scope.Use(ctx, func(lease redisresource.Lease) error {
		key, err := lease.QualifyKey(c.generationKey(datasetID))
		if err != nil {
			return err
		}
		return lease.Set(ctx, key, []byte(generation), 0)
	})
}

func (c *Derived) pendingGeneration(datasetID string) string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.pending[datasetID]
}

func (c *Derived) clearPending(datasetID, generation string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.pending[datasetID] == generation {
		delete(c.pending, datasetID)
	}
}

func validGeneration(value string) bool {
	if value == baseGeneration {
		return true
	}
	if len(value) != 32 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func (c *Derived) generationKey(datasetID string) string {
	return c.policy.Namespace + "/generation/" + datasetID
}

func (c *Derived) entryKey(datasetID, queryID, generation string) string {
	return c.policy.Namespace + "/entry/" + datasetID + "/" + generation + "/" + queryID
}

func targetIDs(target Target) (string, string) {
	return datasetIdentity(target.dataset()), digest(target.QueryIdentity)
}

func datasetIdentity(dataset Dataset) string {
	return digest(dataset.Datasource + "\x00" + dataset.Table)
}

func digest(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func cloneOutcome(outcome Outcome) Outcome {
	outcome.Result = cloneResult(outcome.Result)
	return outcome
}

// Close cancels and drains local flights. It deliberately does not close or
// otherwise mutate the supplied Redis Scope or its owning Resource.
func (c *Derived) Close(ctx context.Context) error {
	if c == nil {
		return nil
	}
	if ctx == nil {
		return validationError(ErrInvalidTarget, "context", "is required")
	}
	c.mu.Lock()
	if !c.closed {
		c.closed = true
		c.cancel(ErrClosed)
		if c.active == 0 && !c.drainClosed {
			close(c.drain)
			c.drainClosed = true
		}
	}
	drain := c.drain
	c.mu.Unlock()
	select {
	case <-drain:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
