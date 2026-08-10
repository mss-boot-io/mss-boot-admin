package resource_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mss-boot-io/mss-boot-admin/mss-boot/runtime/resource"
)

const diagnosticCanary = "runtime-resource-secret-canary-32fdbeef"

type eventLog struct {
	mu     sync.Mutex
	events []string
}

func (l *eventLog) add(event string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.events = append(l.events, event)
}

func (l *eventLog) take() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	result := append([]string(nil), l.events...)
	l.events = nil
	return result
}

type probeResource struct {
	name   string
	events *eventLog

	mu        sync.Mutex
	startErr  error
	readyErr  error
	healthErr error
	closeErr  error
	startFn   func(context.Context) error
	readyFn   func(context.Context) error
	healthFn  func(context.Context) error
	closeFn   func(context.Context) error

	startCalls  atomic.Int64
	readyCalls  atomic.Int64
	healthCalls atomic.Int64
	closeCalls  atomic.Int64
}

func newProbe(name string, events *eventLog) *probeResource {
	return &probeResource{name: name, events: events}
}

func (r *probeResource) Start(ctx context.Context) error {
	r.startCalls.Add(1)
	if r.events != nil {
		r.events.add("start:" + r.name)
	}
	r.mu.Lock()
	fn, result := r.startFn, r.startErr
	r.mu.Unlock()
	if fn != nil {
		return fn(ctx)
	}
	return result
}

func (r *probeResource) Ready(ctx context.Context) error {
	r.readyCalls.Add(1)
	if r.events != nil {
		r.events.add("ready:" + r.name)
	}
	r.mu.Lock()
	fn, result := r.readyFn, r.readyErr
	r.mu.Unlock()
	if fn != nil {
		return fn(ctx)
	}
	return result
}

func (r *probeResource) Health(ctx context.Context) error {
	r.healthCalls.Add(1)
	if r.events != nil {
		r.events.add("health:" + r.name)
	}
	r.mu.Lock()
	fn, result := r.healthFn, r.healthErr
	r.mu.Unlock()
	if fn != nil {
		return fn(ctx)
	}
	return result
}

func (r *probeResource) Close(ctx context.Context) error {
	r.closeCalls.Add(1)
	if r.events != nil {
		r.events.add("close:" + r.name)
	}
	r.mu.Lock()
	fn, result := r.closeFn, r.closeErr
	r.mu.Unlock()
	if fn != nil {
		return fn(ctx)
	}
	return result
}

func (r *probeResource) setClose(fn func(context.Context) error, result error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.closeFn = fn
	r.closeErr = result
}

type bareResource struct {
	calls atomic.Int64
}

func (r *bareResource) Start(context.Context) error {
	r.calls.Add(1)
	return nil
}

func (r *bareResource) Close(context.Context) error {
	r.calls.Add(1)
	return nil
}

type runningResource struct {
	*probeResource
	run func(context.Context) error
}

func (r *runningResource) Run(ctx context.Context) error {
	return r.run(ctx)
}

// controlledDeadlineContext makes deadline delivery deterministic without
// coupling lifecycle tests to wall-clock scheduling.
type controlledDeadlineContext struct {
	context.Context
	done chan struct{}
	once sync.Once
}

func newControlledDeadlineContext() *controlledDeadlineContext {
	return &controlledDeadlineContext{Context: context.Background(), done: make(chan struct{})}
}

func (c *controlledDeadlineContext) Deadline() (time.Time, bool) {
	return time.Now().Add(time.Hour), true
}

func (c *controlledDeadlineContext) Done() <-chan struct{} {
	return c.done
}

func (c *controlledDeadlineContext) Err() error {
	select {
	case <-c.done:
		return context.DeadlineExceeded
	default:
		return nil
	}
}

func (c *controlledDeadlineContext) expire() {
	c.once.Do(func() { close(c.done) })
}

// observedDoneContext reports when Close has begun waiting on an already
// active close generation. Its underlying Background context never expires.
type observedDoneContext struct {
	context.Context
	observed chan struct{}
	once     sync.Once
}

func newObservedDoneContext() *observedDoneContext {
	return &observedDoneContext{Context: context.Background(), observed: make(chan struct{})}
}

func (c *observedDoneContext) Done() <-chan struct{} {
	c.once.Do(func() { close(c.observed) })
	return c.Context.Done()
}

func TestGraphStartsTopologicallyAndClosesInReverse(t *testing.T) {
	t.Parallel()

	events := &eventLog{}
	db := newProbe("db", events)
	cache := newProbe("cache", events)
	api := newProbe("api", events)
	metrics := newProbe("metrics", events)
	graph, err := resource.Build(
		resource.Definition{Name: "api", Dependencies: []string{"db", "cache"}, Required: true, Resource: api},
		resource.Definition{Name: "metrics", Required: true, Resource: metrics},
		resource.Definition{Name: "cache", Dependencies: []string{"db"}, Required: true, Resource: cache},
		resource.Definition{Name: "db", Required: true, Resource: db},
	)
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	wantOrder := []string{"db", "cache", "api", "metrics"}
	if got := graph.ResourceNames(); !reflect.DeepEqual(got, wantOrder) {
		t.Fatalf("ResourceNames() = %v, want %v", got, wantOrder)
	}
	returnedNames := graph.ResourceNames()
	returnedNames[0] = "mutated"
	if got := graph.ResourceNames(); !reflect.DeepEqual(got, wantOrder) {
		t.Fatalf("ResourceNames() retained caller mutation: %v", got)
	}
	if got := fmt.Sprint(graph); got != "RuntimeResourceGraph{resources:4}" {
		t.Fatalf("String() = %q", got)
	}

	if err := graph.Start(context.Background()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	wantStart := []string{
		"start:db", "ready:db",
		"start:cache", "ready:cache",
		"start:api", "ready:api",
		"start:metrics", "ready:metrics",
	}
	if got := events.take(); !reflect.DeepEqual(got, wantStart) {
		t.Fatalf("startup events = %v, want %v", got, wantStart)
	}

	if err := graph.Health(context.Background()); err != nil {
		t.Fatalf("Health() error = %v", err)
	}
	if got := events.take(); !reflect.DeepEqual(got, []string{"health:db", "health:cache", "health:api", "health:metrics"}) {
		t.Fatalf("health events = %v", got)
	}
	if err := graph.Ready(context.Background()); err != nil {
		t.Fatalf("Ready() error = %v", err)
	}
	if got := events.take(); !reflect.DeepEqual(got, []string{"ready:db", "ready:cache", "ready:api", "ready:metrics"}) {
		t.Fatalf("ready events = %v", got)
	}

	if err := graph.Close(context.Background()); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if got := events.take(); !reflect.DeepEqual(got, []string{"close:metrics", "close:api", "close:cache", "close:db"}) {
		t.Fatalf("close events = %v", got)
	}
	if err := graph.Close(context.Background()); err != nil {
		t.Fatalf("repeated Close() error = %v", err)
	}
	if got := events.take(); len(got) != 0 {
		t.Fatalf("repeated Close() called resources: %v", got)
	}
}

func TestGraphPreflightRejectsInvalidMissingAndCyclicDefinitionsWithoutSideEffects(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		definitions func(*probeResource, *bareResource) []resource.Definition
	}{
		{
			name: "invalid name",
			definitions: func(probe *probeResource, _ *bareResource) []resource.Definition {
				return []resource.Definition{{Name: "SecretCanary", Required: true, Resource: probe}}
			},
		},
		{
			name: "missing dependency",
			definitions: func(probe *probeResource, _ *bareResource) []resource.Definition {
				return []resource.Definition{{Name: "api", Dependencies: []string{"database"}, Required: true, Resource: probe}}
			},
		},
		{
			name: "dependency cycle",
			definitions: func(probe *probeResource, _ *bareResource) []resource.Definition {
				return []resource.Definition{
					{Name: "one", Dependencies: []string{"two"}, Required: true, Resource: probe},
					{Name: "two", Dependencies: []string{"one"}, Required: true, Resource: newProbe("two", nil)},
				}
			},
		},
		{
			name: "duplicate resource ownership",
			definitions: func(probe *probeResource, _ *bareResource) []resource.Definition {
				return []resource.Definition{
					{Name: "one", Required: true, Resource: probe},
					{Name: "two", Required: true, Resource: probe},
				}
			},
		},
		{
			name: "duplicate dependency",
			definitions: func(probe *probeResource, _ *bareResource) []resource.Definition {
				return []resource.Definition{
					{Name: "one", Required: true, Resource: probe},
					{Name: "two", Dependencies: []string{"one", "one"}, Required: true, Resource: newProbe("two", nil)},
				}
			},
		},
		{
			name: "required readiness absent",
			definitions: func(_ *probeResource, bare *bareResource) []resource.Definition {
				return []resource.Definition{{Name: "bare", Required: true, Resource: bare}}
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			probe := newProbe("probe", nil)
			bare := &bareResource{}
			graph, err := resource.Build(test.definitions(probe, bare)...)
			if graph != nil || !errors.Is(err, resource.ErrInvalidGraph) {
				t.Fatalf("Build() = (%v, %v), want nil ErrInvalidGraph", graph, err)
			}
			if probe.startCalls.Load()+probe.readyCalls.Load()+probe.healthCalls.Load()+probe.closeCalls.Load() != 0 {
				t.Fatalf("preflight invoked probe resource")
			}
			if bare.calls.Load() != 0 {
				t.Fatalf("preflight invoked bare resource")
			}
			if strings.Contains(fmt.Sprint(err), diagnosticCanary) || strings.Contains(fmt.Sprint(err), "SecretCanary") {
				t.Fatalf("preflight diagnostic retained rejected input: %v", err)
			}
		})
	}
}

func TestGraphStartupFailureRollsBackInReverseAndJoinsErrors(t *testing.T) {
	t.Parallel()

	events := &eventLog{}
	startFailure := errors.New("start failed: " + diagnosticCanary)
	closeOneFailure := errors.New("close one failed: " + diagnosticCanary)
	closeTwoFailure := errors.New("close two failed: " + diagnosticCanary)
	one := newProbe("one", events)
	two := newProbe("two", events)
	two.startErr = startFailure
	one.closeErr = closeOneFailure
	two.closeErr = closeTwoFailure

	graph, err := resource.Build(
		resource.Definition{Name: "one", Required: true, Resource: one},
		resource.Definition{Name: "two", Dependencies: []string{"one"}, Required: true, Resource: two},
	)
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	err = graph.Start(context.Background())
	for _, target := range []error{startFailure, closeOneFailure, closeTwoFailure} {
		if !errors.Is(err, target) {
			t.Fatalf("Start() error = %v, want errors.Is(%v)", err, target)
		}
	}
	if strings.Contains(fmt.Sprint(err), diagnosticCanary) {
		t.Fatalf("Start() exposed provider diagnostic: %v", err)
	}
	want := []string{"start:one", "ready:one", "start:two", "close:two", "close:one"}
	if got := events.take(); !reflect.DeepEqual(got, want) {
		t.Fatalf("startup failure events = %v, want %v", got, want)
	}

	one.setClose(nil, nil)
	two.setClose(nil, nil)
	if err := graph.Close(context.Background()); err != nil {
		t.Fatalf("Close() retry error = %v", err)
	}
	if got := events.take(); !reflect.DeepEqual(got, []string{"close:two", "close:one"}) {
		t.Fatalf("Close() retry events = %v", got)
	}
	if err := graph.Start(context.Background()); !errors.Is(err, resource.ErrStartRejected) {
		t.Fatalf("Start() after failed attempt error = %v, want ErrStartRejected", err)
	}
}

type pipeResource struct {
	mu              sync.Mutex
	reader          *os.File
	writer          *os.File
	openPipeHandles atomic.Int64
	closes          atomic.Int64
}

func (r *pipeResource) Start(ctx context.Context) error {
	reader, writer, err := os.Pipe()
	if err != nil {
		return err
	}
	r.mu.Lock()
	r.reader, r.writer = reader, writer
	r.mu.Unlock()
	r.openPipeHandles.Store(2)
	<-ctx.Done()
	return ctx.Err()
}

func (*pipeResource) Ready(context.Context) error { return nil }

func (r *pipeResource) Close(context.Context) error {
	r.closes.Add(1)
	r.mu.Lock()
	reader, writer := r.reader, r.writer
	r.mu.Unlock()
	var result error
	if reader != nil {
		result = errors.Join(result, reader.Close())
	}
	if writer != nil {
		result = errors.Join(result, writer.Close())
	}
	r.openPipeHandles.Store(0)
	return result
}

func TestGraphCancellationAndCloseDeadlineReleaseOwnership(t *testing.T) {
	t.Parallel()

	t.Run("startup cancellation closes owned pipe handles", func(t *testing.T) {
		pipes := &pipeResource{}
		graph, err := resource.Build(resource.Definition{Name: "pipes", Required: true, Resource: pipes})
		if err != nil {
			t.Fatalf("Build() error = %v", err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 25*time.Millisecond)
		defer cancel()
		err = graph.Start(ctx)
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("Start() error = %v, want DeadlineExceeded", err)
		}
		if got := pipes.openPipeHandles.Load(); got != 0 {
			t.Fatalf("tracked open pipe handles = %d, want zero", got)
		}
		if got := pipes.closes.Load(); got != 1 {
			t.Fatalf("Close calls after cancellation = %d, want one", got)
		}
	})

	t.Run("close honors caller deadline and remains retryable", func(t *testing.T) {
		closer := newProbe("closer", nil)
		closer.setClose(func(ctx context.Context) error {
			<-ctx.Done()
			return ctx.Err()
		}, nil)
		graph, err := resource.Build(resource.Definition{Name: "closer", Required: true, Resource: closer})
		if err != nil {
			t.Fatalf("Build() error = %v", err)
		}
		if err := graph.Start(context.Background()); err != nil {
			t.Fatalf("Start() error = %v", err)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 25*time.Millisecond)
		started := time.Now()
		err = graph.Close(ctx)
		cancel()
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("Close() error = %v, want DeadlineExceeded", err)
		}
		if elapsed := time.Since(started); elapsed > time.Second {
			t.Fatalf("Close() ignored caller deadline for %s", elapsed)
		}

		closer.setClose(nil, nil)
		if err := graph.Close(context.Background()); err != nil {
			t.Fatalf("Close() retry error = %v", err)
		}
		if got := closer.closeCalls.Load(); got != 2 {
			t.Fatalf("Close calls = %d, want two attempts", got)
		}
	})

	t.Run("cancelled close still rejects future start", func(t *testing.T) {
		probe := newProbe("probe", nil)
		graph, err := resource.Build(resource.Definition{Name: "probe", Required: true, Resource: probe})
		if err != nil {
			t.Fatalf("Build() error = %v", err)
		}
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if err := graph.Close(ctx); !errors.Is(err, context.Canceled) {
			t.Fatalf("Close(cancelled) error = %v, want context.Canceled", err)
		}
		if err := graph.Start(context.Background()); !errors.Is(err, resource.ErrStartRejected) {
			t.Fatalf("Start() after Close error = %v, want ErrStartRejected", err)
		}
		if got := probe.startCalls.Load() + probe.closeCalls.Load(); got != 0 {
			t.Fatalf("cancelled Close invoked resource lifecycle %d times", got)
		}
		if err := graph.Close(context.Background()); err != nil {
			t.Fatalf("Close() retry error = %v", err)
		}
	})
}

func TestGraphConcurrentRepeatedCloseIsIdempotentAndRejectsStart(t *testing.T) {
	t.Parallel()

	closer := newProbe("closer", nil)
	entered := make(chan struct{})
	release := make(chan struct{})
	var enteredOnce sync.Once
	closer.setClose(func(context.Context) error {
		enteredOnce.Do(func() { close(entered) })
		<-release
		return nil
	}, nil)
	graph, err := resource.Build(resource.Definition{Name: "closer", Required: true, Resource: closer})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if err := graph.Start(context.Background()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	const callers = 32
	results := make(chan error, callers)
	for range callers {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			results <- graph.Close(ctx)
		}()
	}
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("resource Close was not entered")
	}
	if err := graph.Start(context.Background()); !errors.Is(err, resource.ErrStartRejected) {
		t.Fatalf("Start() during Close error = %v, want ErrStartRejected", err)
	}
	close(release)
	for range callers {
		if err := <-results; err != nil {
			t.Fatalf("concurrent Close() error = %v", err)
		}
	}
	if got := closer.closeCalls.Load(); got != 1 {
		t.Fatalf("concurrent Close calls = %d, want one", got)
	}

	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if err := graph.Close(cancelled); err != nil {
		t.Fatalf("repeated Close() on closed graph error = %v", err)
	}
	if got := closer.closeCalls.Load(); got != 1 {
		t.Fatalf("repeated Close called resource %d times", got)
	}
}

func TestGraphConcurrentCloseSharesFailedGenerationBeforeRetry(t *testing.T) {
	for _, test := range []struct {
		name      string
		transient bool
	}{
		{name: "permanent failure"},
		{name: "transient failure", transient: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			closeFailure := errors.New("close failed: " + diagnosticCanary)
			closer := newProbe("closer", nil)
			entered := make(chan struct{})
			release := make(chan struct{})
			closer.setClose(func(context.Context) error {
				attempt := closer.closeCalls.Load()
				if attempt == 1 {
					close(entered)
					<-release
				}
				if !test.transient || attempt == 1 {
					return closeFailure
				}
				return nil
			}, nil)

			graph, err := resource.Build(resource.Definition{Name: "closer", Required: true, Resource: closer})
			if err != nil {
				t.Fatalf("Build() error = %v", err)
			}
			if err := graph.Start(context.Background()); err != nil {
				t.Fatalf("Start() error = %v", err)
			}

			const waiters = 16
			results := make(chan error, waiters+1)
			go func() { results <- graph.Close(context.Background()) }()
			select {
			case <-entered:
			case <-time.After(time.Second):
				t.Fatal("first provider Close was not entered")
			}

			for range waiters {
				waiterCtx := newObservedDoneContext()
				go func() { results <- graph.Close(waiterCtx) }()
				select {
				case <-waiterCtx.observed:
				case <-time.After(time.Second):
					t.Fatal("concurrent Close did not join the active generation")
				}
			}
			close(release)

			for range waiters + 1 {
				err := <-results
				if !errors.Is(err, closeFailure) {
					t.Fatalf("concurrent Close() error = %v, want shared failure", err)
				}
				if strings.Contains(fmt.Sprint(err), diagnosticCanary) {
					t.Fatalf("Close() exposed provider diagnostic: %v", err)
				}
			}
			if got := closer.closeCalls.Load(); got != 1 {
				t.Fatalf("provider Close calls in one generation = %d, want one", got)
			}

			err = graph.Close(context.Background())
			if test.transient {
				if err != nil {
					t.Fatalf("later Close() retry error = %v", err)
				}
			} else if !errors.Is(err, closeFailure) {
				t.Fatalf("later Close() retry error = %v, want permanent failure", err)
			}
			if got := closer.closeCalls.Load(); got != 2 {
				t.Fatalf("provider Close calls after explicit retry = %d, want two", got)
			}
			if test.transient {
				if err := graph.Close(context.Background()); err != nil {
					t.Fatalf("Close() after successful retry error = %v", err)
				}
				if got := closer.closeCalls.Load(); got != 2 {
					t.Fatalf("closed provider was invoked again: %d calls", got)
				}
			}
		})
	}
}

func TestGraphReadinessFailureRollsBackBeforeDependentStart(t *testing.T) {
	t.Parallel()

	events := &eventLog{}
	readinessFailure := errors.New("readiness failed: " + diagnosticCanary)
	one := newProbe("one", events)
	two := newProbe("two", events)
	three := newProbe("three", events)
	two.readyErr = readinessFailure
	graph, err := resource.Build(
		resource.Definition{Name: "three", Dependencies: []string{"two"}, Required: true, Resource: three},
		resource.Definition{Name: "two", Dependencies: []string{"one"}, Required: true, Resource: two},
		resource.Definition{Name: "one", Required: true, Resource: one},
	)
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	err = graph.Start(context.Background())
	if !errors.Is(err, readinessFailure) {
		t.Fatalf("Start() error = %v, want readiness failure", err)
	}
	if strings.Contains(fmt.Sprint(err), diagnosticCanary) {
		t.Fatalf("Start() exposed readiness diagnostic: %v", err)
	}
	want := []string{"start:one", "ready:one", "start:two", "ready:two", "close:two", "close:one"}
	if got := events.take(); !reflect.DeepEqual(got, want) {
		t.Fatalf("readiness failure events = %v, want %v", got, want)
	}
	if got := three.startCalls.Load(); got != 0 {
		t.Fatalf("dependent Start calls = %d, want zero", got)
	}
	if err := graph.Ready(context.Background()); !errors.Is(err, resource.ErrNotStarted) {
		t.Fatalf("Ready() after failed Start error = %v, want ErrNotStarted", err)
	}
}

func TestGraphHealthAndReadyJoinRedactedDiagnostics(t *testing.T) {
	t.Parallel()

	healthOne := errors.New("health one: " + diagnosticCanary)
	healthTwo := errors.New("health two: " + diagnosticCanary)
	readyOne := errors.New("ready one: " + diagnosticCanary)
	readyTwo := errors.New("ready two: " + diagnosticCanary)
	one := newProbe("one", nil)
	two := newProbe("two", nil)
	one.healthErr, two.healthErr = healthOne, healthTwo
	one.readyErr, two.readyErr = readyOne, readyTwo
	graph, err := resource.Build(
		resource.Definition{Name: "two", Dependencies: []string{"one"}, Resource: two},
		resource.Definition{Name: "one", Resource: one},
	)
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if err := graph.Start(context.Background()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	healthErr := graph.Health(context.Background())
	for _, target := range []error{healthOne, healthTwo} {
		if !errors.Is(healthErr, target) {
			t.Fatalf("Health() error = %v, want errors.Is(%v)", healthErr, target)
		}
	}
	if strings.Contains(fmt.Sprint(healthErr), diagnosticCanary) || strings.Contains(fmt.Sprintf("%#v", healthErr), diagnosticCanary) {
		t.Fatalf("Health() exposed provider diagnostics: %v", healthErr)
	}

	readyErr := graph.Ready(context.Background())
	for _, target := range []error{readyOne, readyTwo} {
		if !errors.Is(readyErr, target) {
			t.Fatalf("Ready() error = %v, want errors.Is(%v)", readyErr, target)
		}
	}
	if strings.Contains(fmt.Sprint(readyErr), diagnosticCanary) {
		t.Fatalf("Ready() exposed provider diagnostics: %v", readyErr)
	}
	var lifecycleErr *resource.LifecycleError
	if !errors.As(readyErr, &lifecycleErr) || lifecycleErr.ResourceName() != "one" || lifecycleErr.LifecycleOperation() != resource.OperationReady {
		t.Fatalf("Ready() lifecycle error = %#v", lifecycleErr)
	}
	if err := graph.Close(context.Background()); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func TestGraphRunCancelsPeersAndJoinsSanitizedDiagnostics(t *testing.T) {
	t.Parallel()

	t.Run("early successful exit is unhealthy", func(t *testing.T) {
		worker := &runningResource{probeResource: newProbe("worker", nil)}
		worker.run = func(context.Context) error { return nil }
		graph, err := resource.Build(resource.Definition{Name: "worker", Required: true, Resource: worker})
		if err != nil {
			t.Fatalf("Build() error = %v", err)
		}
		if err := graph.Start(context.Background()); err != nil {
			t.Fatalf("Start() error = %v", err)
		}
		if err := graph.Run(context.Background()); !errors.Is(err, resource.ErrRunStopped) {
			t.Fatalf("Run() error = %v, want ErrRunStopped", err)
		}
		if err := graph.Close(context.Background()); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
	})

	t.Run("worker failure cancels and joins peers", func(t *testing.T) {
		runFailure := errors.New("runner failed: " + diagnosticCanary)
		slowStarted := make(chan struct{})
		var active atomic.Int64
		slow := &runningResource{probeResource: newProbe("slow", nil)}
		slow.run = func(ctx context.Context) error {
			active.Add(1)
			close(slowStarted)
			defer active.Add(-1)
			<-ctx.Done()
			return ctx.Err()
		}
		fast := &runningResource{probeResource: newProbe("fast", nil)}
		fast.run = func(context.Context) error {
			<-slowStarted
			return runFailure
		}
		graph, err := resource.Build(
			resource.Definition{Name: "slow", Required: true, Resource: slow},
			resource.Definition{Name: "fast", Required: true, Resource: fast},
		)
		if err != nil {
			t.Fatalf("Build() error = %v", err)
		}
		if err := graph.Start(context.Background()); err != nil {
			t.Fatalf("Start() error = %v", err)
		}
		err = graph.Run(context.Background())
		if !errors.Is(err, runFailure) {
			t.Fatalf("Run() error = %v, want runner failure", err)
		}
		if strings.Contains(fmt.Sprint(err), diagnosticCanary) {
			t.Fatalf("Run() exposed provider diagnostic: %v", err)
		}
		if got := active.Load(); got != 0 {
			t.Fatalf("active runners after Run = %d, want zero", got)
		}
		if err := graph.Run(context.Background()); !errors.Is(err, resource.ErrRunRejected) {
			t.Fatalf("repeated Run() error = %v, want ErrRunRejected", err)
		}
		if err := graph.Close(context.Background()); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
	})

	t.Run("peer cancellation retains independent cleanup failure", func(t *testing.T) {
		runFailure := errors.New("runner failed: " + diagnosticCanary)
		cleanupFailure := errors.New("runner cleanup failed: " + diagnosticCanary)
		peerStarted := make(chan struct{})
		peer := &runningResource{probeResource: newProbe("peer", nil)}
		peer.run = func(ctx context.Context) error {
			close(peerStarted)
			<-ctx.Done()
			return errors.Join(ctx.Err(), cleanupFailure)
		}
		failed := &runningResource{probeResource: newProbe("failed", nil)}
		failed.run = func(context.Context) error {
			<-peerStarted
			return runFailure
		}
		graph, err := resource.Build(
			resource.Definition{Name: "peer", Required: true, Resource: peer},
			resource.Definition{Name: "failed", Required: true, Resource: failed},
		)
		if err != nil {
			t.Fatalf("Build() error = %v", err)
		}
		if err := graph.Start(context.Background()); err != nil {
			t.Fatalf("Start() error = %v", err)
		}

		err = graph.Run(context.Background())
		for _, target := range []error{runFailure, cleanupFailure} {
			if !errors.Is(err, target) {
				t.Fatalf("Run() error = %v, want errors.Is(%v)", err, target)
			}
		}
		if errors.Is(err, context.Canceled) {
			t.Fatalf("Run() classified graph-owned peer cancellation as caller cancellation: %v", err)
		}
		if strings.Contains(fmt.Sprint(err), diagnosticCanary) || strings.Contains(fmt.Sprintf("%#v", err), diagnosticCanary) {
			t.Fatalf("Run() exposed provider diagnostic: %v", err)
		}
		if err := graph.Close(context.Background()); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
	})

	t.Run("caller cancellation is returned after worker exit", func(t *testing.T) {
		started := make(chan struct{})
		var active atomic.Int64
		worker := &runningResource{probeResource: newProbe("worker", nil)}
		worker.run = func(ctx context.Context) error {
			active.Add(1)
			close(started)
			defer active.Add(-1)
			<-ctx.Done()
			return ctx.Err()
		}
		graph, err := resource.Build(resource.Definition{Name: "worker", Required: true, Resource: worker})
		if err != nil {
			t.Fatalf("Build() error = %v", err)
		}
		if err := graph.Start(context.Background()); err != nil {
			t.Fatalf("Start() error = %v", err)
		}
		ctx, cancel := context.WithCancel(context.Background())
		result := make(chan error, 1)
		go func() { result <- graph.Run(ctx) }()
		select {
		case <-started:
		case <-time.After(time.Second):
			t.Fatal("runner did not start")
		}
		cancel()
		select {
		case err := <-result:
			if !errors.Is(err, context.Canceled) {
				t.Fatalf("Run() error = %v, want context.Canceled", err)
			}
		case <-time.After(time.Second):
			t.Fatal("Run() did not wait for cancellation")
		}
		if got := active.Load(); got != 0 {
			t.Fatalf("active runners after cancellation = %d, want zero", got)
		}
		if err := graph.Close(context.Background()); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
	})
}

func TestGraphCloseCancelsRunBeforeReverseResourceClose(t *testing.T) {
	t.Parallel()

	started := make(chan struct{}, 2)
	var active atomic.Int64
	events := &eventLog{}
	newRunner := func(name string) *runningResource {
		current := &runningResource{probeResource: newProbe(name, nil)}
		current.run = func(ctx context.Context) error {
			active.Add(1)
			started <- struct{}{}
			defer active.Add(-1)
			<-ctx.Done()
			return ctx.Err()
		}
		current.setClose(func(context.Context) error {
			if got := active.Load(); got != 0 {
				return fmt.Errorf("close observed %d active runners: %s", got, diagnosticCanary)
			}
			events.add("close:" + name)
			return nil
		}, nil)
		return current
	}
	one := newRunner("one")
	two := newRunner("two")
	graph, err := resource.Build(
		resource.Definition{Name: "two", Dependencies: []string{"one"}, Required: true, Resource: two},
		resource.Definition{Name: "one", Required: true, Resource: one},
	)
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if err := graph.Start(context.Background()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	runResult := make(chan error, 1)
	go func() { runResult <- graph.Run(context.Background()) }()
	for range 2 {
		select {
		case <-started:
		case <-time.After(time.Second):
			t.Fatal("runners did not start")
		}
	}
	if err := graph.Close(context.Background()); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	select {
	case err := <-runResult:
		if err != nil {
			t.Fatalf("Run() during owned Close error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Close() returned before Run() completed")
	}
	if got := active.Load(); got != 0 {
		t.Fatalf("active runners after Close = %d, want zero", got)
	}
	if got := events.take(); !reflect.DeepEqual(got, []string{"close:two", "close:one"}) {
		t.Fatalf("close events = %v, want reverse topological order", got)
	}
}

func TestGraphProviderErrorsPreserveExpiredCallerDeadline(t *testing.T) {
	t.Run("Start", func(t *testing.T) {
		providerFailure := errors.New("start after deadline: " + diagnosticCanary)
		entered := make(chan struct{})
		probe := newProbe("probe", nil)
		probe.startFn = func(ctx context.Context) error {
			close(entered)
			<-ctx.Done()
			return providerFailure
		}
		graph, err := resource.Build(resource.Definition{Name: "probe", Required: true, Resource: probe})
		if err != nil {
			t.Fatalf("Build() error = %v", err)
		}
		ctx := newControlledDeadlineContext()
		result := make(chan error, 1)
		go func() { result <- graph.Start(ctx) }()
		<-entered
		ctx.expire()
		err = <-result
		for _, target := range []error{providerFailure, context.DeadlineExceeded} {
			if !errors.Is(err, target) {
				t.Fatalf("Start() error = %v, want errors.Is(%v)", err, target)
			}
		}
		if strings.Contains(fmt.Sprint(err), diagnosticCanary) {
			t.Fatalf("Start() exposed provider diagnostic: %v", err)
		}
	})

	t.Run("required Ready", func(t *testing.T) {
		providerFailure := errors.New("ready after deadline: " + diagnosticCanary)
		entered := make(chan struct{})
		probe := newProbe("probe", nil)
		probe.readyFn = func(ctx context.Context) error {
			close(entered)
			<-ctx.Done()
			return providerFailure
		}
		graph, err := resource.Build(resource.Definition{Name: "probe", Required: true, Resource: probe})
		if err != nil {
			t.Fatalf("Build() error = %v", err)
		}
		ctx := newControlledDeadlineContext()
		result := make(chan error, 1)
		go func() { result <- graph.Start(ctx) }()
		<-entered
		ctx.expire()
		err = <-result
		for _, target := range []error{providerFailure, context.DeadlineExceeded} {
			if !errors.Is(err, target) {
				t.Fatalf("Start() readiness error = %v, want errors.Is(%v)", err, target)
			}
		}
		if strings.Contains(fmt.Sprint(err), diagnosticCanary) {
			t.Fatalf("Start() readiness exposed provider diagnostic: %v", err)
		}
	})

	t.Run("Close", func(t *testing.T) {
		providerFailure := errors.New("close after deadline: " + diagnosticCanary)
		entered := make(chan struct{})
		probe := newProbe("probe", nil)
		probe.closeFn = func(ctx context.Context) error {
			close(entered)
			<-ctx.Done()
			return providerFailure
		}
		graph, err := resource.Build(resource.Definition{Name: "probe", Required: true, Resource: probe})
		if err != nil {
			t.Fatalf("Build() error = %v", err)
		}
		if err := graph.Start(context.Background()); err != nil {
			t.Fatalf("Start() error = %v", err)
		}
		ctx := newControlledDeadlineContext()
		result := make(chan error, 1)
		go func() { result <- graph.Close(ctx) }()
		<-entered
		ctx.expire()
		err = <-result
		for _, target := range []error{providerFailure, context.DeadlineExceeded} {
			if !errors.Is(err, target) {
				t.Fatalf("Close() error = %v, want errors.Is(%v)", err, target)
			}
		}
		if strings.Contains(fmt.Sprint(err), diagnosticCanary) {
			t.Fatalf("Close() exposed provider diagnostic: %v", err)
		}
	})
}
