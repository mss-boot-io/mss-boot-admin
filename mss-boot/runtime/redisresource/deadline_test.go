package redisresource

import (
	"context"
	"errors"
	"net"
	"sync"
	"testing"
	"time"

	runtimeconfig "github.com/mss-boot-io/mss-boot-admin/mss-boot/runtime/config"
	"github.com/redis/go-redis/v9"
	"github.com/redis/go-redis/v9/maintnotifications"
)

func TestRealStalledPipeHonorsCallerDeadlines(t *testing.T) {
	t.Run("Start", func(t *testing.T) {
		client, unblock := newStalledPipeClient(t)
		resource, err := buildWithFactory(normalizedProfile(t, profileOptions{}), &fakeFactory{client: client})
		if err != nil {
			t.Fatalf("Build: %v", err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
		defer cancel()
		started := time.Now()
		err = resource.Start(ctx)
		assertBoundedDeadline(t, "Start", err, time.Since(started))
		unblock()
		_ = resource.Close(context.Background())
	})

	t.Run("started operations", func(t *testing.T) {
		client, unblock := newStalledPipeClient(t)
		resource, err := buildWithFactory(normalizedProfile(t, profileOptions{}), &fakeFactory{client: client})
		if err != nil {
			t.Fatalf("Build: %v", err)
		}
		scope := mustScope(t, resource, "deadline")
		markStartedForProviderTest(resource, client)
		t.Cleanup(func() {
			unblock()
			_ = resource.Close(context.Background())
		})

		for _, operation := range []string{"Ready", "Health", "Get", "Set"} {
			t.Run(operation, func(t *testing.T) {
				ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
				defer cancel()
				started := time.Now()
				switch operation {
				case "Ready":
					err = resource.Ready(ctx)
				case "Health":
					err = resource.Health(ctx)
				case "Get":
					err = scope.Use(context.Background(), func(lease Lease) error {
						key, qualifyErr := lease.QualifyKey("key")
						if qualifyErr != nil {
							return qualifyErr
						}
						_, getErr := lease.Get(ctx, key)
						return getErr
					})
				case "Set":
					err = scope.Use(context.Background(), func(lease Lease) error {
						key, qualifyErr := lease.QualifyKey("key")
						if qualifyErr != nil {
							return qualifyErr
						}
						return lease.Set(ctx, key, []byte("value"), 0)
					})
				}
				assertBoundedDeadline(t, operation, err, time.Since(started))
			})
		}
	})
}

func assertBoundedDeadline(t *testing.T, operation string, err error, elapsed time.Duration) {
	t.Helper()
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("%s = %v, want context.DeadlineExceeded", operation, err)
	}
	if elapsed >= 250*time.Millisecond {
		t.Fatalf("%s took %s; caller deadline did not bound the stalled transport", operation, elapsed)
	}
}

func markStartedForProviderTest(resource *Resource, client ownedClient) {
	done := make(chan struct{})
	close(done)
	resource.state.mu.Lock()
	resource.state.startAttempted = true
	resource.state.startDone = done
	resource.state.started = true
	resource.state.client = client
	resource.state.mu.Unlock()
}

func newStalledPipeClient(t *testing.T) (ownedClient, func()) {
	t.Helper()
	var mu sync.Mutex
	servers := []net.Conn{}
	client := redis.NewClient(&redis.Options{
		Addr:                  "stalled-pipe",
		Protocol:              2,
		MaxRetries:            -1,
		DialTimeout:           500 * time.Millisecond,
		ReadTimeout:           500 * time.Millisecond,
		WriteTimeout:          500 * time.Millisecond,
		ContextTimeoutEnabled: true,
		DisableIdentity:       true,
		MaintNotificationsConfig: &maintnotifications.Config{
			Mode: maintnotifications.ModeDisabled,
		},
		Dialer: func(context.Context, string, string) (net.Conn, error) {
			consumer, server := net.Pipe()
			mu.Lock()
			servers = append(servers, server)
			mu.Unlock()
			return consumer, nil
		},
	})
	unblock := func() {
		mu.Lock()
		defer mu.Unlock()
		for _, server := range servers {
			_ = server.Close()
		}
		servers = nil
	}
	t.Cleanup(func() {
		unblock()
		_ = client.Close()
	})
	return &goRedisClient{client: client, mode: runtimeconfig.RedisStandalone}, unblock
}
