package eventbus

import (
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestMemoryFansOutToCurrentSubscribersAndRejectsOldRevisions(t *testing.T) {
	bus := BuildMemory[string]()
	var mu sync.Mutex
	received := map[string][]Revision{}
	subscribe := func(name string) *Subscription {
		t.Helper()
		subscription, err := bus.Subscribe(func(_ context.Context, event Event[string]) error {
			mu.Lock()
			received[name] = append(received[name], event.Revision)
			mu.Unlock()
			return nil
		})
		if err != nil {
			t.Fatalf("Subscribe(%s): %v", name, err)
		}
		return subscription
	}

	first := subscribe("first")
	_ = subscribe("second")
	if err := bus.Publish(context.Background(), Event[string]{Revision: 1, Payload: "before-start"}); !errors.Is(err, ErrNotStarted) {
		t.Fatalf("Publish before Start error = %v, want ErrNotStarted", err)
	}
	if err := bus.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if err := bus.Publish(context.Background(), Event[string]{Revision: 1, Payload: "one"}); err != nil {
		t.Fatalf("Publish revision 1: %v", err)
	}

	first.Cancel()
	_ = subscribe("third")
	for _, event := range []Event[string]{
		{Revision: 1, Payload: "duplicate"},
		{Revision: 3, Payload: "three"},
		{Revision: 2, Payload: "out-of-order"},
	} {
		if err := bus.Publish(context.Background(), event); err != nil {
			t.Fatalf("Publish revision %d: %v", event.Revision, err)
		}
	}

	mu.Lock()
	defer mu.Unlock()
	want := map[string][]Revision{
		"first":  {1},
		"second": {1, 3},
		"third":  {3},
	}
	for name, revisions := range want {
		if got := received[name]; !equalRevisions(got, revisions) {
			t.Fatalf("%s revisions = %v, want %v", name, got, revisions)
		}
	}
	if got := bus.LastRevision(); got != 3 {
		t.Fatalf("LastRevision = %d, want 3", got)
	}
	if err := bus.Health(context.Background()); err != nil {
		t.Fatalf("Health: %v", err)
	}
}

func TestMemoryPanicIsolationAndAuthoritativeReconciliation(t *testing.T) {
	bus := BuildMemory[string]()
	if err := bus.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}

	source := &mutableSource[string]{event: Event[string]{Revision: 7, Payload: "seven"}, found: true}
	reconciler, err := BuildReconciler[string](source)
	if err != nil {
		t.Fatalf("BuildReconciler: %v", err)
	}
	var firstCalls atomic.Int32
	var secondCalls atomic.Int32
	if _, err := bus.Subscribe(func(_ context.Context, _ Event[string]) error {
		if firstCalls.Add(1) == 1 {
			panic("subscriber-private-panic")
		}
		return nil
	}); err != nil {
		t.Fatalf("Subscribe first: %v", err)
	}
	if _, err := bus.Subscribe(func(_ context.Context, _ Event[string]) error {
		secondCalls.Add(1)
		return nil
	}); err != nil {
		t.Fatalf("Subscribe second: %v", err)
	}

	err = bus.Publish(context.Background(), Event[string]{Revision: 7, Payload: "seven"})
	if !errors.Is(err, ErrSubscriberPanicked) {
		t.Fatalf("Publish error = %v, want ErrSubscriberPanicked", err)
	}
	if strings.Contains(err.Error(), "subscriber-private-panic") {
		t.Fatalf("panic text leaked: %v", err)
	}
	if firstCalls.Load() != 1 || secondCalls.Load() != 1 {
		t.Fatalf("calls after panic = %d,%d, want 1,1", firstCalls.Load(), secondCalls.Load())
	}
	if err := bus.Health(context.Background()); !errors.Is(err, ErrDegraded) {
		t.Fatalf("Health error = %v, want ErrDegraded", err)
	}

	if err := bus.Reconcile(context.Background(), reconciler); err != nil {
		t.Fatalf("Reconcile failed subscriber: %v", err)
	}
	if firstCalls.Load() != 2 || secondCalls.Load() != 1 {
		t.Fatalf("calls after retrying lagging subscriber = %d,%d, want 2,1", firstCalls.Load(), secondCalls.Load())
	}
	if err := bus.Health(context.Background()); err != nil {
		t.Fatalf("Health after reconciliation: %v", err)
	}

	var lateCalls atomic.Int32
	if _, err := bus.Subscribe(func(_ context.Context, _ Event[string]) error {
		lateCalls.Add(1)
		return nil
	}); err != nil {
		t.Fatalf("Subscribe late: %v", err)
	}
	if err := bus.Reconcile(context.Background(), reconciler); err != nil {
		t.Fatalf("Reconcile late subscriber: %v", err)
	}
	if lateCalls.Load() != 1 {
		t.Fatalf("late subscriber calls = %d, want 1", lateCalls.Load())
	}

	source.set(Event[string]{Revision: 8, Payload: "committed-before-publish"}, true, nil)
	if err := bus.Reconcile(context.Background(), reconciler); err != nil {
		t.Fatalf("Reconcile commit-before-publish: %v", err)
	}
	if got := bus.LastRevision(); got != 8 {
		t.Fatalf("LastRevision = %d, want 8", got)
	}
	if firstCalls.Load() != 3 || secondCalls.Load() != 2 || lateCalls.Load() != 2 {
		t.Fatalf("calls after revision 8 = %d,%d,%d, want 3,2,2", firstCalls.Load(), secondCalls.Load(), lateCalls.Load())
	}

	source.set(Event[string]{Revision: 6, Payload: "old-authority"}, true, nil)
	if err := bus.Reconcile(context.Background(), reconciler); err != nil {
		t.Fatalf("Reconcile old authoritative revision: %v", err)
	}
	if got := bus.LastRevision(); got != 8 {
		t.Fatalf("LastRevision after old authority = %d, want 8", got)
	}
}

func TestMemoryCloseWaitsForAcceptedDeliveryAndHonorsDeadline(t *testing.T) {
	bus := BuildMemory[struct{}]()
	started := make(chan struct{})
	releaseHandler := make(chan struct{})
	if _, err := bus.Subscribe(func(ctx context.Context, _ Event[struct{}]) error {
		close(started)
		select {
		case <-releaseHandler:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}); err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	if err := bus.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	publishDone := make(chan error, 1)
	go func() {
		publishDone <- bus.Publish(context.Background(), Event[struct{}]{Revision: 1})
	}()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("handler did not start")
	}

	closeCtx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	err := bus.Close(closeCtx)
	cancel()
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("first Close error = %v, want deadline", err)
	}
	close(releaseHandler)
	if err := <-publishDone; err != nil {
		t.Fatalf("Publish: %v", err)
	}
	if err := bus.Close(context.Background()); err != nil {
		t.Fatalf("second Close: %v", err)
	}
	if err := bus.Close(context.Background()); err != nil {
		t.Fatalf("idempotent Close: %v", err)
	}
	if err := bus.Publish(context.Background(), Event[struct{}]{Revision: 2}); !errors.Is(err, ErrClosing) {
		t.Fatalf("Publish after Close error = %v, want ErrClosing", err)
	}
}

func TestReconcilerRedactsSourceFailureAndPanic(t *testing.T) {
	for name, source := range map[string]AuthoritativeSource[string]{
		"error": AuthoritativeSourceFunc[string](func(context.Context) (Event[string], bool, error) {
			return Event[string]{}, false, errors.New("database-secret-dsn")
		}),
		"panic": AuthoritativeSourceFunc[string](func(context.Context) (Event[string], bool, error) {
			panic("source-private-panic")
		}),
	} {
		t.Run(name, func(t *testing.T) {
			reconciler, err := BuildReconciler(source)
			if err != nil {
				t.Fatalf("BuildReconciler: %v", err)
			}
			_, _, err = reconciler.Reconcile(context.Background(), 0)
			if !errors.Is(err, ErrReconcileFailed) && !errors.Is(err, ErrReconcilerPanicked) {
				t.Fatalf("Reconcile error = %v, want controlled classification", err)
			}
			if strings.Contains(err.Error(), "database-secret-dsn") || strings.Contains(err.Error(), "source-private-panic") {
				t.Fatalf("source detail leaked: %v", err)
			}
		})
	}
}

func equalRevisions(left, right []Revision) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

type mutableSource[T any] struct {
	mu    sync.Mutex
	event Event[T]
	found bool
	err   error
	calls int
}

func (s *mutableSource[T]) Latest(context.Context) (Event[T], bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls++
	return s.event, s.found, s.err
}

func (s *mutableSource[T]) set(event Event[T], found bool, err error) {
	s.mu.Lock()
	s.event = event
	s.found = found
	s.err = err
	s.mu.Unlock()
}

func (s *mutableSource[T]) callCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.calls
}
