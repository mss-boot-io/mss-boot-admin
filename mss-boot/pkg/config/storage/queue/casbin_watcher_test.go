package queue

import (
	"context"
	"errors"
	"testing"

	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/config/storage"
)

type watcherContextKey struct{}

type watcherManagedQueue struct {
	registerContext context.Context
	registerOptions *storage.Options
	registerErr     error
	registerCalls   int
}

func (q *watcherManagedQueue) String() string { return "watcher-managed-queue" }

func (q *watcherManagedQueue) Append(...storage.Option) error { return nil }

func (q *watcherManagedQueue) Register(opts ...storage.Option) {
	q.registerCalls++
	q.registerOptions = storage.SetOptions(opts...)
}

func (q *watcherManagedQueue) RegisterContext(ctx context.Context, opts ...storage.Option) error {
	q.registerCalls++
	q.registerContext = ctx
	q.registerOptions = storage.SetOptions(opts...)
	return q.registerErr
}

func (q *watcherManagedQueue) Run(context.Context) {}

func (q *watcherManagedQueue) Shutdown() {}

func (q *watcherManagedQueue) Start(context.Context) error { return nil }

func (q *watcherManagedQueue) Errors() <-chan error { return nil }

func (q *watcherManagedQueue) Close(context.Context) error { return nil }

func TestSampleWatcherManagedRegistrationUsesCallerContext(t *testing.T) {
	queue := &watcherManagedQueue{}
	watcher := NewSampleWatcher(queue)
	watcher.groupID = "node-a"
	ctx := context.WithValue(context.Background(), watcherContextKey{}, "caller")

	var callbackID string
	if err := watcher.SetUpdateCallbackContext(ctx, func(id string) { callbackID = id }); err != nil {
		t.Fatalf("SetUpdateCallbackContext: %v", err)
	}
	if queue.registerCalls != 1 {
		t.Fatalf("managed registration calls = %d, want 1", queue.registerCalls)
	}
	if got := queue.registerContext.Value(watcherContextKey{}); got != "caller" {
		t.Fatalf("registration context value = %v, want caller", got)
	}
	if queue.registerOptions == nil || queue.registerOptions.GroupID != "node-a" || queue.registerOptions.F == nil {
		t.Fatalf("registration options = %#v", queue.registerOptions)
	}

	self := &Message{}
	self.SetID("node-a")
	if err := queue.registerOptions.F(self); err != nil {
		t.Fatalf("self notification: %v", err)
	}
	if callbackID != "" {
		t.Fatalf("self notification invoked callback with %q", callbackID)
	}
	remote := &Message{}
	remote.SetID("node-b")
	if err := queue.registerOptions.F(remote); err != nil {
		t.Fatalf("remote notification: %v", err)
	}
	if callbackID != "node-b" {
		t.Fatalf("remote callback ID = %q, want node-b", callbackID)
	}
}

func TestSampleWatcherReturnsManagedRegistrationError(t *testing.T) {
	wantErr := errors.New("duplicate consumer")
	queue := &watcherManagedQueue{}
	watcher := NewSampleWatcher(queue)
	watcher.groupID = "node-a"
	firstCalls := 0
	if err := watcher.SetUpdateCallbackContext(context.Background(), func(string) { firstCalls++ }); err != nil {
		t.Fatalf("initial SetUpdateCallbackContext: %v", err)
	}
	queue.registerErr = wantErr

	err := watcher.SetUpdateCallbackContext(context.Background(), func(string) {
		t.Fatal("failed registration replaced the active callback")
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("SetUpdateCallbackContext error = %v, want %v", err, wantErr)
	}
	message := &Message{}
	message.SetID("node-b")
	if err := queue.registerOptions.F(message); err != nil {
		t.Fatalf("callback after failed registration: %v", err)
	}
	if firstCalls != 1 {
		t.Fatalf("active callback calls = %d, want 1", firstCalls)
	}
}

func TestSampleWatcherRejectsCanceledRegistrationContext(t *testing.T) {
	queue := &watcherManagedQueue{}
	watcher := NewSampleWatcher(queue)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := watcher.SetUpdateCallbackContext(ctx, func(string) {})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("SetUpdateCallbackContext error = %v, want context canceled", err)
	}
	if queue.registerCalls != 0 {
		t.Fatalf("managed registration calls = %d, want 0", queue.registerCalls)
	}
}

func TestSampleWatcherManagedLegacyRegistrationRequiresContext(t *testing.T) {
	queue := &watcherManagedQueue{}
	watcher := NewSampleWatcher(queue)

	if err := watcher.SetUpdateCallback(func(string) {}); err == nil {
		t.Fatal("legacy SetUpdateCallback accepted a managed queue without caller context")
	}
	if queue.registerCalls != 0 {
		t.Fatalf("managed registration calls = %d, want 0", queue.registerCalls)
	}
}

func TestSampleWatcherCasbinCallbackUpdateDoesNotRegisterDuplicateConsumer(t *testing.T) {
	queue := &watcherManagedQueue{}
	watcher := NewSampleWatcher(queue)
	watcher.groupID = "node-a"
	if err := watcher.SetUpdateCallbackContext(context.Background(), func(string) {
		t.Fatal("initial callback should be replaced by Casbin")
	}); err != nil {
		t.Fatalf("SetUpdateCallbackContext: %v", err)
	}

	var callbackID string
	if err := watcher.SetUpdateCallback(func(id string) { callbackID = id }); err != nil {
		t.Fatalf("Casbin SetUpdateCallback: %v", err)
	}
	if queue.registerCalls != 1 {
		t.Fatalf("managed registration calls = %d, want 1", queue.registerCalls)
	}
	message := &Message{}
	message.SetID("node-b")
	if err := queue.registerOptions.F(message); err != nil {
		t.Fatalf("updated callback: %v", err)
	}
	if callbackID != "node-b" {
		t.Fatalf("updated callback ID = %q, want node-b", callbackID)
	}
}
