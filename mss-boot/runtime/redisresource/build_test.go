package redisresource

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	runtimeconfig "github.com/mss-boot-io/mss-boot-admin/mss-boot/runtime/config"
	"github.com/redis/go-redis/v9"
)

func TestBuildIsPureAndStartConstructsOneOwnedClient(t *testing.T) {
	client := &fakeClient{}
	factory := &fakeFactory{client: client}
	profile := normalizedProfile(t, profileOptions{})

	resource, err := buildWithFactory(profile, factory)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if calls, _ := factory.snapshot(); calls != 0 {
		t.Fatalf("Build invoked client factory %d times", calls)
	}
	if got := resource.String(); got != `RedisRuntimeResource{name:"cache"}` {
		t.Fatalf("unexpected redacted String: %q", got)
	}

	if err := resource.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if calls, _ := factory.snapshot(); calls != 1 {
		t.Fatalf("Start factory calls = %d, want 1", calls)
	}
	if err := resource.Start(context.Background()); !errors.Is(err, ErrStartRejected) {
		t.Fatalf("second Start = %v, want ErrStartRejected", err)
	}
	if err := resource.Close(context.Background()); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if err := resource.Close(context.Background()); err != nil {
		t.Fatalf("second Close: %v", err)
	}
	if got := client.snapshot().closeCalls; got != 1 {
		t.Fatalf("owned client Close calls = %d, want 1", got)
	}
}

func TestBuildTopologyCredentialAndTLSMatrix(t *testing.T) {
	tests := []struct {
		name       string
		profile    profileOptions
		wantMode   runtimeconfig.RedisMode
		wantDB     int
		wantMaster string
		wantTLS    bool
	}{
		{
			name:     "standalone ACL",
			profile:  profileOptions{mode: runtimeconfig.RedisStandalone, endpoints: []string{"redis.internal:6379"}, database: 4, username: "acl-user", password: "top-secret"},
			wantMode: runtimeconfig.RedisStandalone,
			wantDB:   4,
		},
		{
			name:       "sentinel",
			profile:    profileOptions{mode: runtimeconfig.RedisSentinel, endpoints: []string{"sentinel-a:26379", "sentinel-b:26379"}, masterName: "orders-primary"},
			wantMode:   runtimeconfig.RedisSentinel,
			wantMaster: "orders-primary",
		},
		{
			name:     "cluster TLS and mTLS",
			profile:  profileOptions{mode: runtimeconfig.RedisCluster, endpoints: []string{"node-a:6379", "node-b:6379"}, tls: true, mutualTLS: true, username: "cluster-user", password: "cluster-secret"},
			wantMode: runtimeconfig.RedisCluster,
			wantTLS:  true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			factory := &fakeFactory{client: &fakeClient{}}
			resource, err := buildWithFactory(normalizedProfile(t, test.profile), factory)
			if err != nil {
				t.Fatalf("Build: %v", err)
			}
			if calls, _ := factory.snapshot(); calls != 0 {
				t.Fatalf("factory called during Build: %d", calls)
			}
			if err := resource.Start(context.Background()); err != nil {
				t.Fatalf("Start: %v", err)
			}
			defer func() {
				if err := resource.Close(context.Background()); err != nil {
					t.Errorf("Close: %v", err)
				}
			}()

			calls, specs := factory.snapshot()
			if calls != 1 || len(specs) != 1 {
				t.Fatalf("factory calls/specs = %d/%d, want 1/1", calls, len(specs))
			}
			spec := specs[0]
			if spec.mode != test.wantMode || spec.database != test.wantDB || spec.masterName != test.wantMaster {
				t.Fatalf("unexpected topology spec: mode=%q db=%d master=%q", spec.mode, spec.database, spec.masterName)
			}
			if !reflect.DeepEqual(spec.endpoints, test.profile.endpoints) {
				t.Fatalf("endpoints = %#v, want %#v", spec.endpoints, test.profile.endpoints)
			}
			if test.profile.password != "" && (spec.username != test.profile.username || spec.password != test.profile.password) {
				t.Fatal("ACL credentials were not transferred to the private client specification")
			}
			if (spec.tls != nil) != test.wantTLS {
				t.Fatalf("TLS present = %t, want %t", spec.tls != nil, test.wantTLS)
			}
			if test.wantTLS {
				if spec.tls.MinVersion != tls.VersionTLS13 || spec.tls.ServerName != "redis.example.test" || spec.tls.RootCAs == nil || len(spec.tls.Certificates) != 1 {
					t.Fatal("TLS/CA/mTLS settings were not transferred to the private client specification")
				}
			}
		})
	}
}

func TestDefaultFactoryConstructsTopologySpecificGoRedisClients(t *testing.T) {
	tests := []struct {
		name        string
		profile     profileOptions
		wantMode    runtimeconfig.RedisMode
		wantCluster bool
	}{
		{name: "standalone", profile: profileOptions{mode: runtimeconfig.RedisStandalone}, wantMode: runtimeconfig.RedisStandalone},
		{name: "sentinel TLS", profile: profileOptions{mode: runtimeconfig.RedisSentinel, endpoints: []string{"sentinel-a:26379"}, tls: true}, wantMode: runtimeconfig.RedisSentinel},
		{name: "cluster TLS", profile: profileOptions{mode: runtimeconfig.RedisCluster, endpoints: []string{"node-a:6379", "node-b:6379"}, tls: true}, wantMode: runtimeconfig.RedisCluster, wantCluster: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			resource, err := Build(normalizedProfile(t, test.profile))
			if err != nil {
				t.Fatalf("Build: %v", err)
			}
			owned, err := (defaultClientFactory{}).New(resource.spec.clone())
			if err != nil {
				t.Fatalf("factory New: %v", err)
			}
			adapter, ok := owned.(*goRedisClient)
			if !ok {
				t.Fatalf("adapter = %T, want *goRedisClient", owned)
			}
			if adapter.mode != test.wantMode {
				t.Fatalf("adapter mode = %q, want %q", adapter.mode, test.wantMode)
			}
			if test.wantCluster {
				if _, ok := adapter.client.(*redis.ClusterClient); !ok {
					t.Fatalf("underlying client = %T, want *redis.ClusterClient", adapter.client)
				}
			} else if _, ok := adapter.client.(*redis.Client); !ok {
				t.Fatalf("underlying client = %T, want *redis.Client", adapter.client)
			}
			if err := owned.Close(); err != nil {
				t.Fatalf("factory client Close: %v", err)
			}
		})
	}

	if _, err := (defaultClientFactory{}).New(clientSpec{mode: runtimeconfig.RedisCluster, database: 1}); !errors.Is(err, ErrInvalidProfile) {
		t.Fatalf("cluster DB defense = %v, want ErrInvalidProfile", err)
	}
}

func TestDefaultFactoryEnablesContextTimeoutsForEveryTopology(t *testing.T) {
	standalone, err := Build(normalizedProfile(t, profileOptions{mode: runtimeconfig.RedisStandalone}))
	if err != nil {
		t.Fatalf("Build standalone: %v", err)
	}
	sentinel, err := Build(normalizedProfile(t, profileOptions{
		mode:       runtimeconfig.RedisSentinel,
		endpoints:  []string{"sentinel-a:26379"},
		masterName: "primary",
		username:   "data-user",
		password:   "data-secret",
	}))
	if err != nil {
		t.Fatalf("Build sentinel: %v", err)
	}
	cluster, err := Build(normalizedProfile(t, profileOptions{
		mode:      runtimeconfig.RedisCluster,
		endpoints: []string{"node-a:6379", "node-b:6379"},
	}))
	if err != nil {
		t.Fatalf("Build cluster: %v", err)
	}
	if !standaloneOptions(standalone.spec).ContextTimeoutEnabled {
		t.Fatal("standalone ContextTimeoutEnabled is false")
	}
	sentinelOptions := sentinelOptions(sentinel.spec)
	if !sentinelOptions.ContextTimeoutEnabled {
		t.Fatal("Sentinel ContextTimeoutEnabled is false")
	}
	if sentinelOptions.Username != "data-user" || sentinelOptions.Password != "data-secret" {
		t.Fatal("Sentinel data-plane ACL was not mapped")
	}
	if sentinelOptions.SentinelUsername != "" || sentinelOptions.SentinelPassword != "" {
		t.Fatal("Sentinel control-plane ACL was inferred without dedicated Runtime v2 references")
	}
	if !clusterOptions(cluster.spec).ContextTimeoutEnabled {
		t.Fatal("cluster ContextTimeoutEnabled is false")
	}
}

func TestBuildRejectsNonRedisAndScopeRejectsUnsafeNames(t *testing.T) {
	if _, err := Build(runtimeconfig.ResourceProfile{}); !errors.Is(err, ErrInvalidProfile) {
		t.Fatalf("zero profile = %v, want ErrInvalidProfile", err)
	}
	profile := normalizedProfile(t, profileOptions{})
	resource, err := Build(profile)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	for _, scopeName := range []string{"", "UPPER", "prefix:escape", "a..b", "-start"} {
		if _, err := resource.Scope(scopeName); !errors.Is(err, ErrInvalidCommand) {
			t.Fatalf("scope %q = %v, want ErrInvalidCommand", scopeName, err)
		}
	}
}

func TestErrorsAndFormattingRedactProfileAndProviderData(t *testing.T) {
	const secret = "redis://acl-user:super-secret@private.example:6379"
	providerErr := &providerSecretError{secret: secret}
	factory := &fakeFactory{newErr: providerErr}
	resource, err := buildWithFactory(normalizedProfile(t, profileOptions{password: "super-secret", username: "acl-user"}), factory)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	err = resource.Start(context.Background())
	if !errors.Is(err, ErrClientConstruction) || errors.Is(err, providerErr) {
		t.Fatalf("Start error classification = %v", err)
	}
	var leaked *providerSecretError
	if errors.As(err, &leaked) {
		t.Fatal("public error chain exposed provider error object")
	}
	assertRedacted(t, err, secret, "super-secret", "private.example")
	assertRedacted(t, resource, "super-secret", "acl-user", "127.0.0.1:6379")
}

type providerSecretError struct{ secret string }

func (e *providerSecretError) Error() string { return "provider secret: " + e.secret }

func assertRedacted(t *testing.T, value any, forbidden ...string) {
	t.Helper()
	values := []any{value}
	if err, ok := value.(error); ok {
		values = append(values, unwrapTree(err, 0)...)
	}
	formatted := make([]string, 0, len(values)*4)
	for _, current := range values {
		formatted = append(formatted,
			fmt.Sprint(current),
			fmt.Sprintf("%v", current),
			fmt.Sprintf("%+v", current),
			fmt.Sprintf("%#v", current),
		)
	}
	for _, current := range formatted {
		for _, forbiddenValue := range forbidden {
			if strings.Contains(current, forbiddenValue) {
				t.Fatalf("formatted diagnostic %q leaked %q", current, forbiddenValue)
			}
		}
	}
}

func unwrapTree(err error, depth int) []any {
	if err == nil || depth >= 64 {
		return nil
	}
	result := []any{}
	if multiple, ok := err.(interface{ Unwrap() []error }); ok {
		for _, child := range multiple.Unwrap() {
			if child == nil {
				continue
			}
			result = append(result, child)
			result = append(result, unwrapTree(child, depth+1)...)
		}
		return result
	}
	if single, ok := err.(interface{ Unwrap() error }); ok {
		child := single.Unwrap()
		if child != nil {
			result = append(result, child)
			result = append(result, unwrapTree(child, depth+1)...)
		}
	}
	return result
}
