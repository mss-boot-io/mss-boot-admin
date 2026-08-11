package eventbus

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	runtimeconfig "github.com/mss-boot-io/mss-boot-admin/mss-boot/runtime/config"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/runtime/redisresource"
)

type redisTestPayload struct {
	Name string `json:"name"`
}

func TestRedisBuildIsPureAndPollingFansOutLatestRevision(t *testing.T) {
	server := miniredis.RunT(t)
	resource, scope := buildTestRedis(t, server.Addr())
	source := &mutableSource[redisTestPayload]{event: Event[redisTestPayload]{Revision: 1, Payload: redisTestPayload{Name: "one"}}, found: true}
	reconciler, err := BuildReconciler[redisTestPayload](source)
	if err != nil {
		t.Fatalf("BuildReconciler: %v", err)
	}
	commandsBefore := server.CommandCount()
	first, err := BuildRedis(scope, RedisOptions[redisTestPayload]{
		Reconciler:   reconciler,
		PollInterval: 5 * time.Millisecond,
		PollTimeout:  100 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("BuildRedis first: %v", err)
	}
	second, err := BuildRedis(scope, RedisOptions[redisTestPayload]{
		Reconciler:   reconciler,
		PollInterval: 5 * time.Millisecond,
		PollTimeout:  100 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("BuildRedis second: %v", err)
	}
	if got := server.CommandCount(); got != commandsBefore {
		t.Fatalf("BuildRedis command count = %d, want %d", got, commandsBefore)
	}
	if got := source.callCount(); got != 0 {
		t.Fatalf("BuildRedis called authoritative source %d times", got)
	}

	if err := resource.Start(context.Background()); err != nil {
		t.Fatalf("resource Start: %v", err)
	}
	t.Cleanup(func() { _ = resource.Close(context.Background()) })
	for name, bus := range map[string]*Redis[redisTestPayload]{"first": first, "second": second} {
		if err := bus.Start(context.Background()); err != nil {
			t.Fatalf("%s Start: %v", name, err)
		}
		t.Cleanup(func() { _ = bus.Close(context.Background()) })
	}

	var mu sync.Mutex
	var received []Event[redisTestPayload]
	if _, err := second.Subscribe(func(_ context.Context, event Event[redisTestPayload]) error {
		mu.Lock()
		received = append(received, event)
		mu.Unlock()
		return nil
	}); err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	if err := first.Publish(context.Background(), source.event); err != nil {
		t.Fatalf("Publish revision 1: %v", err)
	}
	if err := second.PollOnce(context.Background()); err != nil {
		t.Fatalf("PollOnce revision 1: %v", err)
	}
	for _, event := range []Event[redisTestPayload]{
		{Revision: 1, Payload: redisTestPayload{Name: "duplicate"}},
		{Revision: 1, Payload: redisTestPayload{Name: "duplicate-again"}},
	} {
		if err := first.Publish(context.Background(), event); err != nil {
			t.Fatalf("Publish duplicate: %v", err)
		}
	}
	if err := second.PollOnce(context.Background()); err != nil {
		t.Fatalf("PollOnce duplicate: %v", err)
	}
	source.set(Event[redisTestPayload]{Revision: 3, Payload: redisTestPayload{Name: "three"}}, true, nil)
	if err := first.Publish(context.Background(), Event[redisTestPayload]{Revision: 3, Payload: redisTestPayload{Name: "three"}}); err != nil {
		t.Fatalf("Publish revision 3: %v", err)
	}
	if err := second.PollOnce(context.Background()); err != nil {
		t.Fatalf("PollOnce revision 3: %v", err)
	}
	if err := first.Publish(context.Background(), Event[redisTestPayload]{Revision: 2, Payload: redisTestPayload{Name: "out-of-order"}}); err != nil {
		t.Fatalf("Publish out-of-order revision 2: %v", err)
	}
	if err := second.PollOnce(context.Background()); err != nil {
		t.Fatalf("PollOnce after out-of-order publish: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(received) != 2 || received[0].Revision != 1 || received[0].Payload.Name != "one" || received[1].Revision != 3 || received[1].Payload.Name != "three" {
		t.Fatalf("received = %#v, want revisions 1 and 3", received)
	}
	if first.LastRevision() != 3 || second.LastRevision() != 3 {
		t.Fatalf("last revisions = %d,%d, want 3,3", first.LastRevision(), second.LastRevision())
	}
}

func TestRedisDisconnectDegradesWhileAuthorityRepairsCommitBeforePublish(t *testing.T) {
	server := miniredis.RunT(t)
	address := server.Addr()
	resource, scope := buildTestRedis(t, address)
	source := &mutableSource[redisTestPayload]{event: Event[redisTestPayload]{Revision: 1, Payload: redisTestPayload{Name: "one"}}, found: true}
	reconciler, err := BuildReconciler[redisTestPayload](source)
	if err != nil {
		t.Fatalf("BuildReconciler: %v", err)
	}
	bus, err := BuildRedis(scope, RedisOptions[redisTestPayload]{
		Reconciler:   reconciler,
		PollInterval: 5 * time.Millisecond,
		PollTimeout:  40 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("BuildRedis: %v", err)
	}
	if err := resource.Start(context.Background()); err != nil {
		t.Fatalf("resource Start: %v", err)
	}
	t.Cleanup(func() { _ = resource.Close(context.Background()) })
	if err := bus.Start(context.Background()); err != nil {
		t.Fatalf("bus Start: %v", err)
	}
	t.Cleanup(func() { _ = bus.Close(context.Background()) })

	var mu sync.Mutex
	var revisions []Revision
	if _, err := bus.Subscribe(func(_ context.Context, event Event[redisTestPayload]) error {
		mu.Lock()
		revisions = append(revisions, event.Revision)
		mu.Unlock()
		return nil
	}); err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	if err := bus.Reconcile(context.Background()); err != nil {
		t.Fatalf("initial Reconcile: %v", err)
	}

	server.Close()
	source.set(Event[redisTestPayload]{Revision: 2, Payload: redisTestPayload{Name: "committed"}}, true, nil)
	started := time.Now()
	err = bus.PollOnce(context.Background())
	if !errors.Is(err, ErrProviderFailed) {
		t.Fatalf("PollOnce during outage error = %v, want ErrProviderFailed", err)
	}
	if time.Since(started) > 500*time.Millisecond {
		t.Fatalf("bounded PollOnce took %s", time.Since(started))
	}
	if strings.Contains(err.Error(), address) {
		t.Fatalf("provider address leaked: %v", err)
	}
	if got := bus.LastRevision(); got != 2 {
		t.Fatalf("LastRevision after outage reconciliation = %d, want 2", got)
	}
	if err := bus.Health(context.Background()); !errors.Is(err, ErrDegraded) {
		t.Fatalf("Health during outage = %v, want ErrDegraded", err)
	}

	if err := server.Restart(); err != nil {
		t.Fatalf("Restart miniredis: %v", err)
	}
	if err := bus.PollOnce(context.Background()); err != nil {
		t.Fatalf("PollOnce after recovery: %v", err)
	}
	if err := bus.Health(context.Background()); err != nil {
		t.Fatalf("Health after recovery: %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	if !equalRevisions(revisions, []Revision{1, 2}) {
		t.Fatalf("revisions = %v, want [1 2]", revisions)
	}
}

func TestRedisRunOwnsPollingAndCloseLeavesSharedResourceOpen(t *testing.T) {
	server := miniredis.RunT(t)
	resource, scope := buildTestRedis(t, server.Addr())
	source := &mutableSource[redisTestPayload]{event: Event[redisTestPayload]{Revision: 4, Payload: redisTestPayload{Name: "four"}}, found: true}
	reconciler, err := BuildReconciler[redisTestPayload](source)
	if err != nil {
		t.Fatalf("BuildReconciler: %v", err)
	}
	bus, err := BuildRedis(scope, RedisOptions[redisTestPayload]{
		Reconciler:   reconciler,
		PollInterval: 5 * time.Millisecond,
		PollTimeout:  100 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("BuildRedis: %v", err)
	}
	if err := resource.Start(context.Background()); err != nil {
		t.Fatalf("resource Start: %v", err)
	}
	if err := bus.Start(context.Background()); err != nil {
		t.Fatalf("bus Start: %v", err)
	}

	delivered := make(chan Revision, 1)
	if _, err := bus.Subscribe(func(_ context.Context, event Event[redisTestPayload]) error {
		select {
		case delivered <- event.Revision:
		default:
		}
		return nil
	}); err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	runDone := make(chan error, 1)
	go func() { runDone <- bus.Run(context.Background()) }()
	select {
	case revision := <-delivered:
		if revision != 4 {
			t.Fatalf("delivered revision = %d, want 4", revision)
		}
	case <-time.After(time.Second):
		t.Fatal("Run did not reconcile initial revision")
	}
	if err := bus.Close(context.Background()); err != nil {
		t.Fatalf("bus Close: %v", err)
	}
	select {
	case err := <-runDone:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Run error = %v, want context cancellation", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Run did not stop after Close")
	}
	if err := resource.Health(context.Background()); err != nil {
		t.Fatalf("shared Redis resource was closed by bus: %v", err)
	}
	if err := resource.Close(context.Background()); err != nil {
		t.Fatalf("resource Close: %v", err)
	}
}

func buildTestRedis(t *testing.T, endpoint string) (*redisresource.Resource, *redisresource.Scope) {
	t.Helper()
	configuration := runtimeconfig.Config{Resources: map[string]runtimeconfig.ResourceConfig{
		"event-bus": {
			Provider: runtimeconfig.ProviderConfig{
				Kind: runtimeconfig.ProviderRedis,
				Redis: &runtimeconfig.RedisConfig{
					Mode:       runtimeconfig.RedisStandalone,
					Standalone: &runtimeconfig.RedisStandaloneConfig{Endpoint: endpoint},
					Credentials: runtimeconfig.RedisCredentialsConfig{
						Kind:      runtimeconfig.RedisCredentialsAnonymous,
						Anonymous: &runtimeconfig.RedisAnonymousCredentialsConfig{},
					},
				},
			},
		},
	}}
	snapshot, err := configuration.Normalize(context.Background(), nil)
	if err != nil {
		t.Fatalf("Normalize Redis profile: %v", err)
	}
	profile, ok := snapshot.Resource("event-bus")
	if !ok {
		t.Fatal("normalized Redis profile missing")
	}
	resource, err := redisresource.Build(profile)
	if err != nil {
		t.Fatalf("redisresource.Build: %v", err)
	}
	scope, err := resource.Scope("replicas")
	if err != nil {
		t.Fatalf("Scope: %v", err)
	}
	return resource, scope
}
