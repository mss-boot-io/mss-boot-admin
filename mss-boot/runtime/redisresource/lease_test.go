package redisresource

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	runtimeconfig "github.com/mss-boot-io/mss-boot-admin/mss-boot/runtime/config"
	"github.com/redis/go-redis/v9"
)

func TestOneNamedResourceSharesOneClientAcrossIsolatedScopes(t *testing.T) {
	client := &fakeClient{}
	factory := &fakeFactory{client: client}
	resource, err := buildWithFactory(normalizedProfile(t, profileOptions{name: "cache"}), factory)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	sessions := mustScope(t, resource, "sessions")
	queries := mustScope(t, resource, "queries")
	if duplicate := mustScope(t, resource, "sessions"); duplicate != sessions {
		t.Fatal("repeated Scope did not return the same stable capability")
	}
	if calls, _ := factory.snapshot(); calls != 0 {
		t.Fatalf("Scope constructed a client: calls=%d", calls)
	}
	if err := resource.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if calls, _ := factory.snapshot(); calls != 1 {
		t.Fatalf("shared resource factory calls=%d, want 1", calls)
	}
	t.Cleanup(func() { _ = resource.Close(context.Background()) })

	var sessionKey Key
	var retained Lease
	if err := sessions.Use(context.Background(), func(lease Lease) error {
		retained = lease
		var err error
		sessionKey, err = lease.QualifyKey("user_42")
		if err != nil {
			return err
		}
		if strings.Contains(fmt.Sprintf("%#v", sessionKey), "user_42") {
			t.Fatal("opaque key formatting leaked the logical key")
		}
		return lease.Set(context.Background(), sessionKey, []byte("first"), time.Minute)
	}); err != nil {
		t.Fatalf("Use sessions: %v", err)
	}
	if _, err := retained.Get(context.Background(), sessionKey); !errors.Is(err, ErrLeaseExpired) {
		t.Fatalf("retained lease Get = %v, want ErrLeaseExpired", err)
	}

	if err := queries.Use(context.Background(), func(lease Lease) error {
		if _, err := lease.Get(context.Background(), sessionKey); !errors.Is(err, ErrUnsafeKey) {
			return fmt.Errorf("foreign key = %v, want ErrUnsafeKey", err)
		}
		queryKey, err := lease.QualifyKey("user_42")
		if err != nil {
			return err
		}
		return lease.Set(context.Background(), queryKey, []byte("second"), 0)
	}); err != nil {
		t.Fatalf("Use queries: %v", err)
	}

	for _, unsafe := range []string{"", "/absolute", "../escape", "a/../b", "namespace:key", "hash{tag}", "space key", "trailing/"} {
		if err := sessions.Use(context.Background(), func(lease Lease) error {
			_, err := lease.QualifyKey(unsafe)
			return err
		}); !errors.Is(err, ErrUnsafeKey) {
			t.Fatalf("unsafe key %q = %v, want ErrUnsafeKey", unsafe, err)
		}
	}

	log := client.snapshot().commandLog
	if !contains(log, "set:cache:sessions:user_42") || !contains(log, "set:cache:queries:user_42") {
		t.Fatalf("physical prefixes missing from command log: %#v", log)
	}
	if err := resource.Close(context.Background()); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if snapshot := client.snapshot(); snapshot.closeCalls != 1 {
		t.Fatalf("shared resource client Close calls=%d, want 1", snapshot.closeCalls)
	}
}

func TestDifferentNamedResourcesCannotSharePhysicalScopePrefix(t *testing.T) {
	firstClient := &fakeClient{}
	secondClient := &fakeClient{}
	first, err := buildWithFactory(normalizedProfile(t, profileOptions{name: "primary-cache"}), &fakeFactory{client: firstClient})
	if err != nil {
		t.Fatalf("Build first: %v", err)
	}
	second, err := buildWithFactory(normalizedProfile(t, profileOptions{name: "secondary-cache"}), &fakeFactory{client: secondClient})
	if err != nil {
		t.Fatalf("Build second: %v", err)
	}
	firstScope := mustScope(t, first, "sessions")
	secondScope := mustScope(t, second, "sessions")
	if err := first.Start(context.Background()); err != nil {
		t.Fatalf("Start first: %v", err)
	}
	if err := second.Start(context.Background()); err != nil {
		t.Fatalf("Start second: %v", err)
	}
	t.Cleanup(func() {
		_ = first.Close(context.Background())
		_ = second.Close(context.Background())
	})
	set := func(scope *Scope) error {
		return scope.Use(context.Background(), func(lease Lease) error {
			key, err := lease.QualifyKey("user_42")
			if err != nil {
				return err
			}
			return lease.Set(context.Background(), key, []byte("value"), 0)
		})
	}
	if err := set(firstScope); err != nil {
		t.Fatalf("first scope Set: %v", err)
	}
	if err := set(secondScope); err != nil {
		t.Fatalf("second scope Set: %v", err)
	}
	if got := firstClient.snapshot().commandLog; !contains(got, "set:primary-cache:sessions:user_42") {
		t.Fatalf("first physical prefix: %#v", got)
	}
	if got := secondClient.snapshot().commandLog; !contains(got, "set:secondary-cache:sessions:user_42") {
		t.Fatalf("second physical prefix: %#v", got)
	}
}

func TestClusterPortableMultiKeyCommandsArePartitionedBySlot(t *testing.T) {
	client := &fakeClient{}
	providerErr := errors.New("cluster delete failed at secret-node")
	resource, err := buildWithFactory(normalizedProfile(t, profileOptions{
		name:      "cluster-cache",
		mode:      runtimeconfig.RedisCluster,
		endpoints: []string{"node-a:6379", "node-b:6379"},
	}), &fakeFactory{client: client})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	scope := mustScope(t, resource, "sessions")
	if err := resource.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() { _ = resource.Close(context.Background()) })
	if err := scope.Use(context.Background(), func(lease Lease) error {
		keys := make([]Key, 3)
		for index, logical := range []string{"user-a", "user-b", "user-c"} {
			key, err := lease.QualifyKey(logical)
			if err != nil {
				return err
			}
			keys[index] = key
			if err := lease.Set(context.Background(), key, []byte("value"), 0); err != nil {
				return err
			}
		}
		if count, err := lease.Exists(context.Background(), keys...); err != nil || count != 3 {
			return fmt.Errorf("Exists = %d, %v", count, err)
		}
		client.mu.Lock()
		client.deleteFailAt = 2
		client.deleteErr = providerErr
		client.mu.Unlock()
		if count, err := lease.Delete(context.Background(), keys...); count != 1 || !errors.Is(err, ErrCommandFailed) || errors.Is(err, providerErr) {
			return fmt.Errorf("fail-fast Delete = %d, %v; want sanitized partial count 1", count, err)
		}
		return nil
	}); err != nil {
		t.Fatalf("Use: %v", err)
	}
	snapshot := client.snapshot()
	for operation, batches := range map[string][][]string{
		"Exists": snapshot.existsBatches,
		"Delete": snapshot.deleteBatches,
	} {
		wantBatches := 3
		if operation == "Delete" {
			wantBatches = 2
		}
		if len(batches) != wantBatches {
			t.Fatalf("%s batches = %#v, want %d fail-fast single-key calls", operation, batches, wantBatches)
		}
		for _, batch := range batches {
			if len(batch) != 1 || strings.ContainsAny(batch[0], "{}") {
				t.Fatalf("%s batch = %#v, want one untagged qualified key", operation, batch)
			}
		}
	}
}

func TestScopeUseContextBoundsBackgroundCommand(t *testing.T) {
	neverRespond := make(chan struct{})
	client := &fakeClient{getWait: neverRespond}
	resource, err := buildWithFactory(normalizedProfile(t, profileOptions{}), &fakeFactory{client: client})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if err := resource.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	scope := mustScope(t, resource, "consumer")
	useContext, cancel := context.WithTimeout(context.Background(), 15*time.Millisecond)
	defer cancel()
	err = scope.Use(useContext, func(lease Lease) error {
		key, err := lease.QualifyKey("bounded")
		if err != nil {
			return err
		}
		_, err = lease.Get(context.Background(), key)
		return err
	})
	if !errors.Is(err, context.DeadlineExceeded) || !errors.Is(err, ErrUseRejected) || !errors.Is(err, ErrCommandFailed) {
		t.Fatalf("Use/background Get = %v, want inherited Use deadline", err)
	}
	if err := resource.Close(context.Background()); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

func TestScopeUseCancelsAndRejectsDetachedCommandAtCallbackReturn(t *testing.T) {
	neverRespond := make(chan struct{})
	commandStarted := make(chan struct{}, 1)
	client := &fakeClient{setWait: neverRespond, setStart: commandStarted}
	resource, err := buildWithFactory(normalizedProfile(t, profileOptions{}), &fakeFactory{client: client})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if err := resource.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	scope := mustScope(t, resource, "consumer")
	commandDone := make(chan error, 1)
	err = scope.Use(context.Background(), func(lease Lease) error {
		key, err := lease.QualifyKey("detached")
		if err != nil {
			return err
		}
		go func() {
			commandDone <- lease.Set(context.Background(), key, []byte("value"), 0)
		}()
		<-commandStarted
		return nil
	})
	if !errors.Is(err, ErrUseRejected) || !errors.Is(err, ErrDetachedCommand) {
		t.Fatalf("Use detached command = %v, want structured callback rejection", err)
	}
	if commandErr := <-commandDone; !errors.Is(commandErr, ErrLeaseExpired) || !errors.Is(commandErr, ErrCommandFailed) {
		t.Fatalf("detached command = %v, want lease cancellation", commandErr)
	}
	if err := resource.Close(context.Background()); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if snapshot := client.snapshot(); snapshot.closeCalls != 1 || snapshot.commandAfterClose {
		t.Fatalf("client lifecycle after rejected detached command: %#v", snapshot)
	}
}

func TestLeaseCommandsUseCallerContextAndRedactProviderErrors(t *testing.T) {
	providerErr := errors.New("GET exposed password=hunter2 endpoint=private.redis")
	client := &fakeClient{getErr: providerErr}
	resource, err := buildWithFactory(normalizedProfile(t, profileOptions{}), &fakeFactory{client: client})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if err := resource.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	scope := mustScope(t, resource, "consumer")
	t.Cleanup(func() { _ = resource.Close(context.Background()) })

	err = scope.Use(context.Background(), func(lease Lease) error {
		key, err := lease.QualifyKey("safe")
		if err != nil {
			return err
		}
		_, err = lease.Get(context.Background(), key)
		return err
	})
	if errors.Is(err, providerErr) || !errors.Is(err, ErrCommandFailed) || !errors.Is(err, ErrUseRejected) {
		t.Fatalf("Use/Get error = %v, want retained classifications", err)
	}
	assertRedacted(t, err, "hunter2", "private.redis")

	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	err = scope.Use(context.Background(), func(lease Lease) error {
		key, err := lease.QualifyKey("safe")
		if err != nil {
			return err
		}
		return lease.Set(canceled, key, []byte("secret-value"), 0)
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled Set = %v, want context.Canceled", err)
	}
}

func TestStandaloneDefaultClientAgainstMiniredis(t *testing.T) {
	server := miniredis.RunT(t)
	resource, err := Build(normalizedProfile(t, profileOptions{endpoints: []string{server.Addr()}, name: "integration-cache"}))
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if err := resource.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	scope := mustScope(t, resource, "users")
	t.Cleanup(func() {
		if err := resource.Close(context.Background()); err != nil {
			t.Errorf("Close: %v", err)
		}
	})

	if err := scope.Use(context.Background(), func(lease Lease) error {
		key, err := lease.QualifyKey("42")
		if err != nil {
			return err
		}
		if err := lease.Set(context.Background(), key, []byte("payload"), time.Minute); err != nil {
			return err
		}
		value, err := lease.Get(context.Background(), key)
		if err != nil {
			return err
		}
		if string(value) != "payload" {
			return fmt.Errorf("Get = %q, want payload", value)
		}
		count, err := lease.Exists(context.Background(), key)
		if err != nil || count != 1 {
			return fmt.Errorf("Exists = %d, %v", count, err)
		}
		count, err = lease.Delete(context.Background(), key)
		if err != nil || count != 1 {
			return fmt.Errorf("Delete = %d, %v", count, err)
		}
		_, err = lease.Get(context.Background(), key)
		if !errors.Is(err, ErrNotFound) || errors.Is(err, ErrCommandFailed) || errors.Is(err, redis.Nil) {
			return fmt.Errorf("missing Get = %v, want provider-neutral ErrNotFound only", err)
		}
		return nil
	}); err != nil {
		t.Fatalf("Use: %v", err)
	}
	if server.Exists("users/42") {
		t.Fatal("unqualified key exists in provider")
	}
	if server.Exists("integration-cache:users:42") {
		t.Fatal("Delete did not remove qualified key")
	}
}

func TestGetMissingReturnsProviderNeutralNotFound(t *testing.T) {
	server := miniredis.RunT(t)
	resource, err := Build(normalizedProfile(t, profileOptions{endpoints: []string{server.Addr()}}))
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	scope := mustScope(t, resource, "missing")
	if err := resource.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() { _ = resource.Close(context.Background()) })
	if err := scope.Use(context.Background(), func(lease Lease) error {
		key, err := lease.QualifyKey("key")
		if err != nil {
			return err
		}
		_, err = lease.Get(context.Background(), key)
		if !errors.Is(err, ErrNotFound) {
			return fmt.Errorf("Get = %v, want ErrNotFound", err)
		}
		if errors.Is(err, ErrCommandFailed) || errors.Is(err, redis.Nil) {
			return fmt.Errorf("Get = %v, leaked provider/command classification", err)
		}
		return nil
	}); err != nil {
		t.Fatalf("Use: %v", err)
	}
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
