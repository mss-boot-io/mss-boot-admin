package redisresource

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	runtimeresource "github.com/mss-boot-io/mss-boot-admin/mss-boot/runtime/resource"
)

func TestStartFailureCleansUpExactlyOnce(t *testing.T) {
	providerErr := errors.New("PING failed at private.redis with password=secret")
	client := &fakeClient{pingErr: providerErr}
	resource, err := buildWithFactory(normalizedProfile(t, profileOptions{}), &fakeFactory{client: client})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	err = resource.Start(context.Background())
	if !errors.Is(err, ErrUnavailable) || errors.Is(err, providerErr) {
		t.Fatalf("Start error = %v, want only sanitized classification", err)
	}
	assertRedacted(t, err, "private.redis", "password=secret")
	if got := client.snapshot().closeCalls; got != 1 {
		t.Fatalf("failed Start cleanup calls = %d, want 1", got)
	}
	if err := resource.Close(context.Background()); err != nil {
		t.Fatalf("Close after failed Start: %v", err)
	}
	if got := client.snapshot().closeCalls; got != 1 {
		t.Fatalf("Close retried client cleanup: calls=%d", got)
	}
	if err := resource.Ready(context.Background()); !errors.Is(err, ErrClosing) {
		t.Fatalf("Ready after failed Start = %v, want ErrClosing", err)
	}
}

func TestReadyHealthAndCallerCancellation(t *testing.T) {
	providerErr := errors.New("provider health detail secret")
	client := &fakeClient{}
	resource, err := buildWithFactory(normalizedProfile(t, profileOptions{}), &fakeFactory{client: client})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if err := resource.Ready(context.Background()); !errors.Is(err, ErrNotStarted) {
		t.Fatalf("Ready before Start = %v, want ErrNotStarted", err)
	}
	if err := resource.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() { _ = resource.Close(context.Background()) })

	if err := resource.Ready(context.Background()); err != nil {
		t.Fatalf("Ready: %v", err)
	}
	client.mu.Lock()
	client.pingErr = providerErr
	client.mu.Unlock()
	if err := resource.Health(context.Background()); !errors.Is(err, ErrUnavailable) || errors.Is(err, providerErr) {
		t.Fatalf("Health = %v, want only sanitized classification", err)
	} else {
		assertRedacted(t, err, "health detail secret")
	}

	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if err := resource.Ready(canceled); !errors.Is(err, context.Canceled) {
		t.Fatalf("Ready(canceled) = %v, want context.Canceled", err)
	}
	if err := resource.Health(nil); !errors.Is(err, ErrContextRequired) {
		t.Fatalf("Health(nil) = %v, want ErrContextRequired", err)
	}
}

func TestCloseDeadlineDrainsLeaseAndPermanentlyRejectsUse(t *testing.T) {
	client := &fakeClient{}
	resource, err := buildWithFactory(normalizedProfile(t, profileOptions{}), &fakeFactory{client: client})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if err := resource.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	scope := mustScope(t, resource, "consumer")

	entered := make(chan struct{})
	release := make(chan struct{})
	useDone := make(chan error, 1)
	go func() {
		useDone <- scope.Use(context.Background(), func(Lease) error {
			close(entered)
			<-release
			return nil
		})
	}()
	<-entered

	closeContext, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	if err := resource.Close(closeContext); !errors.Is(err, context.DeadlineExceeded) || !errors.Is(err, ErrCloseFailed) {
		t.Fatalf("Close deadline = %v, want deadline and ErrCloseFailed", err)
	}
	if got := client.snapshot().closeCalls; got != 0 {
		t.Fatalf("client closed while lease active: %d", got)
	}
	if err := scope.Use(context.Background(), func(Lease) error { return nil }); !errors.Is(err, ErrClosing) {
		t.Fatalf("Use after Close began = %v, want ErrClosing", err)
	}

	close(release)
	if err := <-useDone; err != nil {
		t.Fatalf("active Use: %v", err)
	}
	if err := resource.Close(context.Background()); err != nil {
		t.Fatalf("retry Close: %v", err)
	}
	if got := client.snapshot().closeCalls; got != 1 {
		t.Fatalf("client Close calls = %d, want 1", got)
	}
}

func TestCloseDeadlineBoundsProviderCloseAndRetryJoinsGeneration(t *testing.T) {
	providerErr := errors.New("provider close leaked password=secret endpoint=private.redis")
	releaseClose := make(chan struct{})
	closeStarted := make(chan struct{}, 1)
	client := &fakeClient{closeErr: providerErr, closeWait: releaseClose, closeStart: closeStarted}
	resource, err := buildWithFactory(normalizedProfile(t, profileOptions{}), &fakeFactory{client: client})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	scope := mustScope(t, resource, "consumer")
	if err := resource.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}

	deadline, cancel := context.WithTimeout(context.Background(), 15*time.Millisecond)
	defer cancel()
	err = resource.Close(deadline)
	if !errors.Is(err, context.DeadlineExceeded) || !errors.Is(err, ErrCloseFailed) {
		t.Fatalf("first Close = %v, want bounded deadline", err)
	}
	<-closeStarted
	if snapshot := client.snapshot(); snapshot.closeCalls != 1 || snapshot.closed {
		t.Fatalf("provider close generation before release: %#v", snapshot)
	}
	if err := scope.Use(context.Background(), func(Lease) error { return nil }); !errors.Is(err, ErrClosing) {
		t.Fatalf("Use during provider Close = %v, want ErrClosing", err)
	}

	retryDone := make(chan error, 1)
	go func() { retryDone <- resource.Close(context.Background()) }()
	select {
	case err := <-retryDone:
		t.Fatalf("retry returned before shared generation completed: %v", err)
	case <-time.After(10 * time.Millisecond):
	}
	if snapshot := client.snapshot(); snapshot.closeCalls != 1 {
		t.Fatalf("retry created another provider Close: %#v", snapshot)
	}
	close(releaseClose)
	err = <-retryDone
	if errors.Is(err, providerErr) || !errors.Is(err, ErrCloseFailed) {
		t.Fatalf("retry terminal error = %v, want shared sanitized error", err)
	}
	assertRedacted(t, err, "password=secret", "private.redis")
	err = resource.Close(context.Background())
	if errors.Is(err, providerErr) || !errors.Is(err, ErrCloseFailed) {
		t.Fatalf("cached terminal error = %v", err)
	}
	assertRedacted(t, err, "password=secret", "private.redis")
	if snapshot := client.snapshot(); snapshot.closeCalls != 1 || !snapshot.closed {
		t.Fatalf("terminal provider close state: %#v", snapshot)
	}
}

func TestResourceRejectsScopeCreationAfterCloseBegins(t *testing.T) {
	releaseClose := make(chan struct{})
	closeStarted := make(chan struct{}, 1)
	client := &fakeClient{closeWait: releaseClose, closeStart: closeStarted}
	resource, err := buildWithFactory(normalizedProfile(t, profileOptions{}), &fakeFactory{client: client})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	existing := mustScope(t, resource, "existing")
	if err := resource.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	closeDone := make(chan error, 1)
	go func() { closeDone <- resource.Close(context.Background()) }()
	<-closeStarted
	if _, err := resource.Scope("late"); !errors.Is(err, ErrClosing) {
		t.Fatalf("new Scope during Close = %v, want ErrClosing", err)
	}
	if duplicate, err := resource.Scope("existing"); !errors.Is(err, ErrClosing) || duplicate != nil {
		t.Fatalf("existing Scope lookup during Close = %#v, %v; want rejection", duplicate, err)
	}
	if err := existing.Use(context.Background(), func(Lease) error { return nil }); !errors.Is(err, ErrClosing) {
		t.Fatalf("existing Scope Use during Close = %v, want ErrClosing", err)
	}
	close(releaseClose)
	if err := <-closeDone; err != nil {
		t.Fatalf("Close: %v", err)
	}
}

func TestConcurrentCloseAndUseNeverCommandsAfterClientClose(t *testing.T) {
	client := &fakeClient{}
	resource, err := buildWithFactory(normalizedProfile(t, profileOptions{}), &fakeFactory{client: client})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if err := resource.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	scope := mustScope(t, resource, "workers")

	const workers = 64
	start := make(chan struct{})
	var wait sync.WaitGroup
	wait.Add(workers)
	for index := 0; index < workers; index++ {
		go func(index int) {
			defer wait.Done()
			<-start
			err := scope.Use(context.Background(), func(lease Lease) error {
				key, err := lease.QualifyKey(fmt.Sprintf("worker/%d", index))
				if err != nil {
					return err
				}
				return lease.Set(context.Background(), key, []byte("value"), 0)
			})
			if err != nil && !errors.Is(err, ErrClosing) {
				t.Errorf("Use: %v", err)
			}
		}(index)
	}
	close(start)

	closeErrors := make(chan error, 8)
	for range 8 {
		go func() { closeErrors <- resource.Close(context.Background()) }()
	}
	wait.Wait()
	for range 8 {
		if err := <-closeErrors; err != nil {
			t.Errorf("concurrent Close: %v", err)
		}
	}
	snapshot := client.snapshot()
	if snapshot.closeCalls != 1 {
		t.Fatalf("client Close calls = %d, want 1", snapshot.closeCalls)
	}
	if snapshot.commandAfterClose {
		t.Fatal("a leased command reached the client after Close")
	}
}

func TestResourceGraphIntegration(t *testing.T) {
	client := &fakeClient{}
	redisResource, err := buildWithFactory(normalizedProfile(t, profileOptions{}), &fakeFactory{client: client})
	if err != nil {
		t.Fatalf("Build redis resource: %v", err)
	}
	_ = mustScope(t, redisResource, "sessions")
	_ = mustScope(t, redisResource, "queries")
	graph, err := runtimeresource.Build(redisResource.Definition(true))
	if err != nil {
		t.Fatalf("Build graph: %v", err)
	}
	if names := graph.ResourceNames(); len(names) != 1 || names[0] != "cache" {
		t.Fatalf("graph resources = %#v, want one named Redis resource", names)
	}
	if err := graph.Start(context.Background()); err != nil {
		t.Fatalf("graph Start: %v", err)
	}
	if err := graph.Health(context.Background()); err != nil {
		t.Fatalf("graph Health: %v", err)
	}
	if err := graph.Ready(context.Background()); err != nil {
		t.Fatalf("graph Ready: %v", err)
	}
	if err := graph.Close(context.Background()); err != nil {
		t.Fatalf("graph Close: %v", err)
	}
	snapshot := client.snapshot()
	if snapshot.pingCalls != 4 { // Start, required gate, Health, Ready.
		t.Fatalf("PING calls = %d, want 4", snapshot.pingCalls)
	}
	if snapshot.closeCalls != 1 {
		t.Fatalf("Close calls = %d, want 1", snapshot.closeCalls)
	}
}
