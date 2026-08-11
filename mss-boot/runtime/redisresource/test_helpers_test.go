package redisresource

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"math/big"
	"sync"
	"testing"
	"time"

	runtimeconfig "github.com/mss-boot-io/mss-boot-admin/mss-boot/runtime/config"
)

type profileOptions struct {
	name       string
	mode       runtimeconfig.RedisMode
	endpoints  []string
	masterName string
	database   int
	username   string
	password   string
	tls        bool
	mutualTLS  bool
}

func normalizedProfile(t *testing.T, options profileOptions) runtimeconfig.ResourceProfile {
	t.Helper()
	if options.name == "" {
		options.name = "cache"
	}
	if options.mode == "" {
		options.mode = runtimeconfig.RedisStandalone
	}
	if len(options.endpoints) == 0 {
		options.endpoints = []string{"127.0.0.1:6379"}
	}
	redisConfig := &runtimeconfig.RedisConfig{
		Mode:     options.mode,
		Database: options.database,
	}
	switch options.mode {
	case runtimeconfig.RedisStandalone:
		redisConfig.Standalone = &runtimeconfig.RedisStandaloneConfig{Endpoint: options.endpoints[0]}
	case runtimeconfig.RedisSentinel:
		if options.masterName == "" {
			options.masterName = "primary"
		}
		redisConfig.Sentinel = &runtimeconfig.RedisSentinelConfig{Endpoints: options.endpoints, MasterName: options.masterName}
	case runtimeconfig.RedisCluster:
		redisConfig.Cluster = &runtimeconfig.RedisClusterConfig{Endpoints: options.endpoints}
	default:
		t.Fatalf("unsupported test mode %q", options.mode)
	}

	secrets := map[string]string{}
	if options.password == "" {
		redisConfig.Credentials = runtimeconfig.RedisCredentialsConfig{
			Kind:      runtimeconfig.RedisCredentialsAnonymous,
			Anonymous: &runtimeconfig.RedisAnonymousCredentialsConfig{},
		}
	} else {
		passwordRef := mustSecretRef(t, "env://TEST_REDIS_PASSWORD")
		password := runtimeconfig.RedisPasswordCredentialsConfig{PasswordRef: passwordRef}
		secrets[passwordRef.Reference()] = options.password
		if options.username != "" {
			usernameRef := mustSecretRef(t, "env://TEST_REDIS_USERNAME")
			password.UsernameRef = usernameRef
			secrets[usernameRef.Reference()] = options.username
		}
		redisConfig.Credentials = runtimeconfig.RedisCredentialsConfig{
			Kind:     runtimeconfig.RedisCredentialsPassword,
			Password: &password,
		}
	}

	if options.tls {
		certificatePEM, keyPEM := testCertificate(t)
		caRef := mustSecretRef(t, "env://TEST_REDIS_CA")
		secrets[caRef.Reference()] = certificatePEM
		tlsConfig := &runtimeconfig.RedisTLSConfig{
			MinVersion: "1.3",
			ServerName: "redis.example.test",
			CARef:      caRef,
		}
		if options.mutualTLS {
			certificateRef := mustSecretRef(t, "env://TEST_REDIS_CLIENT_CERT")
			keyRef := mustSecretRef(t, "env://TEST_REDIS_CLIENT_KEY")
			secrets[certificateRef.Reference()] = certificatePEM
			secrets[keyRef.Reference()] = keyPEM
			tlsConfig.ClientCertificateRef = certificateRef
			tlsConfig.ClientKeyRef = keyRef
		}
		redisConfig.TLS = tlsConfig
	}

	configuration := runtimeconfig.Config{Resources: map[string]runtimeconfig.ResourceConfig{
		options.name: {
			Provider: runtimeconfig.ProviderConfig{Kind: runtimeconfig.ProviderRedis, Redis: redisConfig},
		},
	}}
	resolver := runtimeconfig.SecretResolverFunc(func(_ context.Context, ref runtimeconfig.SecretRef) (string, error) {
		value, ok := secrets[ref.Reference()]
		if !ok {
			return "", errors.New("missing test secret")
		}
		return value, nil
	})
	snapshot, err := configuration.Normalize(context.Background(), resolver)
	if err != nil {
		t.Fatalf("normalize test profile: %v", err)
	}
	profile, ok := snapshot.Resource(options.name)
	if !ok {
		t.Fatal("normalized profile missing")
	}
	return profile
}

func mustSecretRef(t *testing.T, value string) runtimeconfig.SecretRef {
	t.Helper()
	ref, err := runtimeconfig.ParseSecretRef(value)
	if err != nil {
		t.Fatalf("parse secret ref: %v", err)
	}
	return ref
}

func mustScope(t *testing.T, resource *Resource, name string) *Scope {
	t.Helper()
	scope, err := resource.Scope(name)
	if err != nil {
		t.Fatalf("Scope(%q): %v", name, err)
	}
	return scope
}

func testCertificate(t *testing.T) (string, string) {
	t.Helper()
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate private key: %v", err)
	}
	now := time.Now()
	template := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "redis.example.test"},
		DNSNames:              []string{"redis.example.test"},
		NotBefore:             now.Add(-time.Minute),
		NotAfter:              now.Add(time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &privateKey.PublicKey, privateKey)
	if err != nil {
		t.Fatalf("create certificate: %v", err)
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		t.Fatalf("marshal private key: %v", err)
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})),
		string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER}))
}

type fakeFactory struct {
	mu      sync.Mutex
	calls   int
	specs   []clientSpec
	client  ownedClient
	newErr  error
	newWait <-chan struct{}
}

func (f *fakeFactory) New(spec clientSpec) (ownedClient, error) {
	if f.newWait != nil {
		<-f.newWait
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	f.specs = append(f.specs, spec.clone())
	if f.newErr != nil {
		return nil, f.newErr
	}
	if f.client == nil {
		f.client = &fakeClient{}
	}
	return f.client, nil
}

func (f *fakeFactory) snapshot() (int, []clientSpec) {
	f.mu.Lock()
	defer f.mu.Unlock()
	specs := make([]clientSpec, len(f.specs))
	for index := range f.specs {
		specs[index] = f.specs[index].clone()
	}
	return f.calls, specs
}

type fakeClient struct {
	mu sync.Mutex

	pingCalls         int
	closeCalls        int
	commandLog        []string
	deleteBatches     [][]string
	existsBatches     [][]string
	deleteCalls       int
	values            map[string][]byte
	closed            bool
	commandAfterClose bool

	pingErr      error
	closeErr     error
	getErr       error
	setErr       error
	deleteErr    error
	deleteFailAt int
	pingWait     <-chan struct{}
	getWait      <-chan struct{}
	getStart     chan<- struct{}
	setWait      <-chan struct{}
	setStart     chan<- struct{}
	closeWait    <-chan struct{}
	closeStart   chan<- struct{}
}

func (c *fakeClient) Ping(ctx context.Context) error {
	c.mu.Lock()
	c.pingCalls++
	wait := c.pingWait
	err := c.pingErr
	c.mu.Unlock()
	if wait != nil {
		select {
		case <-wait:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return err
}

func (c *fakeClient) Get(ctx context.Context, key string) ([]byte, bool, error) {
	c.mu.Lock()
	c.recordCommandLocked("get:" + key)
	wait := c.getWait
	started := c.getStart
	err := c.getErr
	c.mu.Unlock()
	if started != nil {
		select {
		case started <- struct{}{}:
		default:
		}
	}
	if wait != nil {
		select {
		case <-wait:
		case <-ctx.Done():
			return nil, false, ctx.Err()
		}
	}
	if err != nil {
		return nil, false, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	value, ok := c.values[key]
	return append([]byte(nil), value...), ok, nil
}

func (c *fakeClient) Set(ctx context.Context, key string, value []byte, _ time.Duration) error {
	c.mu.Lock()
	c.recordCommandLocked("set:" + key)
	wait := c.setWait
	started := c.setStart
	err := c.setErr
	c.mu.Unlock()
	if started != nil {
		select {
		case started <- struct{}{}:
		default:
		}
	}
	if wait != nil {
		select {
		case <-wait:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	if err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		c.commandAfterClose = true
	}
	if c.values == nil {
		c.values = map[string][]byte{}
	}
	c.values[key] = append([]byte(nil), value...)
	return nil
}

func (c *fakeClient) Delete(_ context.Context, keys ...string) (int64, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.deleteCalls++
	c.deleteBatches = append(c.deleteBatches, append([]string(nil), keys...))
	if c.deleteFailAt > 0 && c.deleteCalls == c.deleteFailAt {
		return 0, c.deleteErr
	}
	var count int64
	for _, key := range keys {
		c.recordCommandLocked("delete:" + key)
		if _, ok := c.values[key]; ok {
			delete(c.values, key)
			count++
		}
	}
	return count, nil
}

func (c *fakeClient) Exists(_ context.Context, keys ...string) (int64, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.existsBatches = append(c.existsBatches, append([]string(nil), keys...))
	var count int64
	for _, key := range keys {
		c.recordCommandLocked("exists:" + key)
		if _, ok := c.values[key]; ok {
			count++
		}
	}
	return count, nil
}

func (c *fakeClient) Close() error {
	c.mu.Lock()
	c.closeCalls++
	wait := c.closeWait
	started := c.closeStart
	err := c.closeErr
	c.mu.Unlock()
	if started != nil {
		select {
		case started <- struct{}{}:
		default:
		}
	}
	if wait != nil {
		<-wait
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closed = true
	return err
}

func (c *fakeClient) recordCommandLocked(command string) {
	if c.closed {
		c.commandAfterClose = true
	}
	c.commandLog = append(c.commandLog, command)
}

type fakeSnapshot struct {
	pingCalls         int
	closeCalls        int
	commandLog        []string
	deleteBatches     [][]string
	existsBatches     [][]string
	closed            bool
	commandAfterClose bool
}

func (c *fakeClient) snapshot() fakeSnapshot {
	c.mu.Lock()
	defer c.mu.Unlock()
	deleteBatches := make([][]string, len(c.deleteBatches))
	for index := range c.deleteBatches {
		deleteBatches[index] = append([]string(nil), c.deleteBatches[index]...)
	}
	existsBatches := make([][]string, len(c.existsBatches))
	for index := range c.existsBatches {
		existsBatches[index] = append([]string(nil), c.existsBatches[index]...)
	}
	return fakeSnapshot{
		pingCalls:         c.pingCalls,
		closeCalls:        c.closeCalls,
		commandLog:        append([]string(nil), c.commandLog...),
		deleteBatches:     deleteBatches,
		existsBatches:     existsBatches,
		closed:            c.closed,
		commandAfterClose: c.commandAfterClose,
	}
}
