package redisresource

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/mss-boot-io/mss-boot-admin/mss-boot/runtime/internal/redisbridge"
)

func TestRedisBridgeRejectsCrossGroupBeforeProviderIO(t *testing.T) {
	client := &atomicFakeClient{fakeClient: &fakeClient{}}
	resource, scope := startedAtomicResource(t, client)
	t.Cleanup(func() { _ = resource.Close(context.Background()) })
	one, _ := redisbridge.NewAtomicGroup("challenge-subject", []byte("one"))
	two, _ := redisbridge.NewAtomicGroup("challenge-subject", []byte("two"))
	oneKey, _ := one.Key("state")
	twoKey, _ := two.Key("state")

	if err := redisbridge.Use(context.Background(), scope, one, func(lease redisbridge.Lease) error {
		_, runErr := lease.Run(context.Background(), redisbridge.ChallengeBeginScript(), []redisbridge.Key{oneKey, twoKey})
		if !errors.Is(runErr, redisbridge.ErrCrossGroup) {
			t.Fatalf("Run error = %v, want ErrCrossGroup", runErr)
		}
		return nil
	}); err != nil {
		t.Fatalf("Use: %v", err)
	}
	if got := client.evalCalls(); got != 0 {
		t.Fatalf("cross-group request reached provider %d times", got)
	}
}

func TestRedisBridgeLeaseCancelsAndDrainsDetachedCommand(t *testing.T) {
	started := make(chan struct{}, 1)
	client := &atomicFakeClient{fakeClient: &fakeClient{}, evalStarted: started, waitForContext: true}
	resource, scope := startedAtomicResource(t, client)
	t.Cleanup(func() { _ = resource.Close(context.Background()) })
	group, _ := redisbridge.NewAtomicGroup("challenge-subject", []byte("detached"))
	key, _ := group.Key("state")
	commandDone := make(chan error, 1)

	err := redisbridge.Use(context.Background(), scope, group, func(lease redisbridge.Lease) error {
		go func() {
			_, runErr := lease.Run(context.Background(), redisbridge.ChallengeReadVerifierScript(), []redisbridge.Key{key})
			commandDone <- runErr
		}()
		<-started
		return nil
	})
	if !errors.Is(err, ErrDetachedCommand) || !errors.Is(err, ErrUseRejected) {
		t.Fatalf("Use error = %v, want detached/use rejected", err)
	}
	if runErr := <-commandDone; !errors.Is(runErr, ErrLeaseExpired) || !errors.Is(runErr, ErrCommandFailed) {
		t.Fatalf("detached Run error = %v, want lease-expired/command-failed", runErr)
	}
}

func TestRedisBridgeContextAndProviderErrorsAreRedacted(t *testing.T) {
	providerErr := &providerSecretError{secret: "password=hunter2 endpoint=private.redis"}
	client := &atomicFakeClient{fakeClient: &fakeClient{}, evalErr: providerErr}
	resource, scope := startedAtomicResource(t, client)
	t.Cleanup(func() { _ = resource.Close(context.Background()) })
	group, _ := redisbridge.NewAtomicGroup("challenge-subject", []byte("redaction"))
	key, _ := group.Key("state")

	err := redisbridge.Use(context.Background(), scope, group, func(lease redisbridge.Lease) error {
		_, runErr := lease.Run(context.Background(), redisbridge.ChallengeReadVerifierScript(), []redisbridge.Key{key})
		return runErr
	})
	if !errors.Is(err, ErrCommandFailed) || !errors.Is(err, ErrUseRejected) || errors.Is(err, providerErr) {
		t.Fatalf("provider Run error = %v, want controlled classifications", err)
	}
	var leaked *providerSecretError
	if errors.As(err, &leaked) {
		t.Fatal("public error chain exposed provider object")
	}
	assertRedacted(t, err, "hunter2", "private.redis")

	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	err = redisbridge.Use(context.Background(), scope, group, func(lease redisbridge.Lease) error {
		_, runErr := lease.Run(canceled, redisbridge.ChallengeReadVerifierScript(), []redisbridge.Key{key})
		return runErr
	})
	if !errors.Is(err, context.Canceled) || !errors.Is(err, ErrUseRejected) {
		t.Fatalf("canceled Run error = %v, want canceled/use rejected", err)
	}
}

func startedAtomicResource(t *testing.T, client *atomicFakeClient) (*Resource, *Scope) {
	t.Helper()
	resource, err := buildWithFactory(normalizedProfile(t, profileOptions{}), &fakeFactory{client: client})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if err = resource.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	return resource, mustScope(t, resource, "challenge")
}

type atomicFakeClient struct {
	*fakeClient

	atomicMu       sync.Mutex
	atomicCalls    int
	evalErr        error
	evalStarted    chan<- struct{}
	waitForContext bool
}

func (c *atomicFakeClient) EvalFixed(ctx context.Context, _ string, _ []string, _ ...any) (any, error) {
	c.atomicMu.Lock()
	c.atomicCalls++
	started := c.evalStarted
	waitForContext := c.waitForContext
	err := c.evalErr
	c.atomicMu.Unlock()
	if started != nil {
		select {
		case started <- struct{}{}:
		default:
		}
	}
	if waitForContext {
		<-ctx.Done()
		return nil, context.Cause(ctx)
	}
	if err != nil {
		return nil, err
	}
	return []any{nil, nil, nil}, nil
}

func (c *atomicFakeClient) evalCalls() int {
	c.atomicMu.Lock()
	defer c.atomicMu.Unlock()
	return c.atomicCalls
}
