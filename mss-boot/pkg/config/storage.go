package config

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// ProviderType identifies the only storage branches supported by the strict
// storage profile. S3-compatible providers use the S3 branch with an explicit
// endpoint and path-style setting.
type ProviderType string

const (
	Local ProviderType = "local"
	S3    ProviderType = "s3"
)

var (
	// ErrInvalidStorageConfiguration classifies configuration errors without
	// including secret values in the returned error.
	ErrInvalidStorageConfiguration = errors.New("invalid storage configuration")
	// ErrStorageHandleClosing is returned when work is submitted after shutdown
	// has started.
	ErrStorageHandleClosing = errors.New("storage handle is closing")
)

// Storage is a strict discriminated configuration. Exactly one branch must be
// present.
type Storage struct {
	Local *LocalStorageConfig `yaml:"local,omitempty" json:"local,omitempty"`
	S3    *S3StorageConfig    `yaml:"s3,omitempty" json:"s3,omitempty"`
}

type LocalStorageConfig struct {
	Root string `yaml:"root" json:"root"`
}

type S3StorageConfig struct {
	Endpoint     string              `yaml:"endpoint" json:"endpoint"`
	Region       string              `yaml:"region" json:"region"`
	Bucket       string              `yaml:"bucket" json:"bucket"`
	UsePathStyle bool                `yaml:"usePathStyle" json:"usePathStyle"`
	TLS          S3TLSConfig         `yaml:"tls,omitempty" json:"tls,omitempty"`
	Credentials  S3CredentialsConfig `yaml:"credentials" json:"credentials"`
}

// S3TLSConfig contains optional trust and mTLS secret references. A client
// certificate and key must always be configured together.
type S3TLSConfig struct {
	CARef                SecretRef `yaml:"caRef,omitempty" json:"caRef,omitempty"`
	ClientCertificateRef SecretRef `yaml:"clientCertificateRef,omitempty" json:"clientCertificateRef,omitempty"`
	ClientKeyRef         SecretRef `yaml:"clientKeyRef,omitempty" json:"clientKeyRef,omitempty"`
	AllowInsecureHTTP    bool      `yaml:"allowInsecureHTTP,omitempty" json:"allowInsecureHTTP,omitempty"`
}

// S3CredentialsConfig is itself a strict discriminated configuration. Exactly
// one credential source must be present.
type S3CredentialsConfig struct {
	DefaultChain *DefaultChainCredentials `yaml:"defaultChain,omitempty" json:"defaultChain,omitempty"`
	Static       *StaticCredentialRefs    `yaml:"static,omitempty" json:"static,omitempty"`
}

// DefaultChainCredentials makes use of the AWS credential chain explicit.
type DefaultChainCredentials struct{}

type StaticCredentialRefs struct {
	AccessKeyRef    SecretRef `yaml:"accessKeyRef" json:"accessKeyRef"`
	SecretKeyRef    SecretRef `yaml:"secretKeyRef" json:"secretKeyRef"`
	SessionTokenRef SecretRef `yaml:"sessionTokenRef,omitempty" json:"sessionTokenRef,omitempty"`
}

// SecretRef names secret material without embedding it in configuration.
// D1 supports environment references only.
type SecretRef string

// SecretResolver resolves a typed secret reference. Implementations must not
// include resolved values in errors.
type SecretResolver interface {
	Resolve(context.Context, SecretRef) (string, error)
}

// EnvSecretResolver resolves env://NAME references.
type EnvSecretResolver struct{}

func (EnvSecretResolver) Resolve(ctx context.Context, ref SecretRef) (string, error) {
	if ctx == nil {
		return "", storageConfigError("secretRef", "context is required")
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	name, err := envSecretName(ref)
	if err != nil {
		return "", err
	}
	value, ok := os.LookupEnv(name)
	if !ok || value == "" {
		return "", storageConfigError("secretRef", "referenced environment value is missing")
	}
	return value, nil
}

// StorageProfile is the immutable, normalized storage snapshot. Its fields are
// private so credentials cannot be read back by consumers.
type StorageProfile struct {
	provider     ProviderType
	localRoot    string
	endpoint     string
	region       string
	bucket       string
	usePathStyle bool

	credentialMode    storageCredentialMode
	staticCredentials func() (string, string, string)
	newTLSConfig      func() *tls.Config

	buildMu   sync.Mutex
	buildDone chan struct{}
	handle    *StorageHandle
}

type storageCredentialMode uint8

const (
	storageCredentialsDefaultChain storageCredentialMode = iota + 1
	storageCredentialsStatic
)

// Normalize validates and copies Storage without modifying caller-owned
// configuration. It performs all SecretRef resolution before a client can be
// constructed.
func (c Storage) Normalize(ctx context.Context, secrets SecretResolver) (*StorageProfile, error) {
	if ctx == nil {
		return nil, storageConfigError("storage", "context is required")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	branches := 0
	if c.Local != nil {
		branches++
	}
	if c.S3 != nil {
		branches++
	}
	if branches != 1 {
		return nil, storageConfigError("storage", "exactly one of local or s3 is required")
	}
	if c.Local != nil {
		return normalizeLocal(*c.Local)
	}
	return normalizeS3(ctx, *c.S3, secrets)
}

func normalizeLocal(c LocalStorageConfig) (*StorageProfile, error) {
	root := strings.TrimSpace(c.Root)
	if root == "" {
		return nil, storageConfigError("local.root", "is required")
	}
	if strings.IndexByte(root, 0) >= 0 || !filepath.IsAbs(root) {
		return nil, storageConfigError("local.root", "must be an absolute path")
	}
	return &StorageProfile{provider: Local, localRoot: filepath.Clean(root)}, nil
}

func normalizeS3(ctx context.Context, c S3StorageConfig, secrets SecretResolver) (*StorageProfile, error) {
	endpoint, endpointScheme, err := normalizeS3Endpoint(c.Endpoint)
	if err != nil {
		return nil, err
	}
	region := strings.TrimSpace(c.Region)
	if region == "" {
		return nil, storageConfigError("s3.region", "is required")
	}
	bucket := strings.TrimSpace(c.Bucket)
	if !validS3BucketName(bucket) {
		return nil, storageConfigError("s3.bucket", "must be a valid DNS-compatible bucket name")
	}

	profile := &StorageProfile{
		provider:     S3,
		endpoint:     endpoint,
		region:       region,
		bucket:       bucket,
		usePathStyle: c.UsePathStyle,
	}
	credentialBranches := 0
	if c.Credentials.DefaultChain != nil {
		credentialBranches++
		profile.credentialMode = storageCredentialsDefaultChain
	}
	if c.Credentials.Static != nil {
		credentialBranches++
		profile.credentialMode = storageCredentialsStatic
	}
	if credentialBranches != 1 {
		return nil, storageConfigError("s3.credentials", "exactly one of defaultChain or static is required")
	}
	if c.Credentials.Static != nil {
		if secrets == nil {
			return nil, storageConfigError("s3.credentials.static", "secret resolver is required")
		}
		accessKeyID, resolveErr := resolveRequiredSecret(ctx, secrets, "s3.credentials.static.accessKeyRef", c.Credentials.Static.AccessKeyRef)
		if resolveErr != nil {
			return nil, resolveErr
		}
		secretAccessKey, resolveErr := resolveRequiredSecret(ctx, secrets, "s3.credentials.static.secretKeyRef", c.Credentials.Static.SecretKeyRef)
		if resolveErr != nil {
			return nil, resolveErr
		}
		sessionToken := ""
		if c.Credentials.Static.SessionTokenRef != "" {
			sessionToken, resolveErr = resolveRequiredSecret(ctx, secrets, "s3.credentials.static.sessionTokenRef", c.Credentials.Static.SessionTokenRef)
			if resolveErr != nil {
				return nil, resolveErr
			}
		}
		// A function value formats as an address, not its captured material. Keep
		// resolved credentials out of the Profile's reflectable data fields.
		profile.staticCredentials = func() (string, string, string) {
			return accessKeyID, secretAccessKey, sessionToken
		}
	}

	tlsConfigured := c.TLS.CARef != "" || c.TLS.ClientCertificateRef != "" || c.TLS.ClientKeyRef != ""
	if endpointScheme == "http" && !c.TLS.AllowInsecureHTTP {
		return nil, storageConfigError("s3.endpoint", "http requires explicit tls.allowInsecureHTTP")
	}
	if endpointScheme == "https" && c.TLS.AllowInsecureHTTP {
		return nil, storageConfigError("s3.tls.allowInsecureHTTP", "is only valid for an http endpoint")
	}
	if tlsConfigured && endpointScheme != "https" {
		return nil, storageConfigError("s3.tls", "certificate settings require an https endpoint")
	}
	if (c.TLS.ClientCertificateRef == "") != (c.TLS.ClientKeyRef == "") {
		return nil, storageConfigError("s3.tls", "client certificate and key must be configured together")
	}
	if tlsConfigured && secrets == nil {
		return nil, storageConfigError("s3.tls", "secret resolver is required")
	}
	var rootCAs *x509.CertPool
	var certificates []tls.Certificate
	if c.TLS.CARef != "" {
		caPEM, resolveErr := resolveRequiredSecret(ctx, secrets, "s3.tls.caRef", c.TLS.CARef)
		if resolveErr != nil {
			return nil, resolveErr
		}
		rootCAs = x509.NewCertPool()
		if !rootCAs.AppendCertsFromPEM([]byte(caPEM)) {
			return nil, storageConfigError("s3.tls.caRef", "does not contain a valid PEM certificate")
		}
	}
	if c.TLS.ClientCertificateRef != "" {
		certificatePEM, resolveErr := resolveRequiredSecret(ctx, secrets, "s3.tls.clientCertificateRef", c.TLS.ClientCertificateRef)
		if resolveErr != nil {
			return nil, resolveErr
		}
		keyPEM, resolveErr := resolveRequiredSecret(ctx, secrets, "s3.tls.clientKeyRef", c.TLS.ClientKeyRef)
		if resolveErr != nil {
			return nil, resolveErr
		}
		certificate, pairErr := tls.X509KeyPair([]byte(certificatePEM), []byte(keyPEM))
		if pairErr != nil {
			return nil, storageConfigError("s3.tls", "client certificate and key are invalid")
		}
		certificates = []tls.Certificate{certificate}
	}
	if rootCAs != nil || len(certificates) != 0 {
		profile.newTLSConfig = func() *tls.Config {
			result := &tls.Config{MinVersion: tls.VersionTLS12}
			if rootCAs != nil {
				result.RootCAs = rootCAs.Clone()
			}
			result.Certificates = append([]tls.Certificate(nil), certificates...)
			return result
		}
	}
	return profile, nil
}

func normalizeS3Endpoint(raw string) (string, string, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return "", "", storageConfigError("s3.endpoint", "is required")
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Hostname() == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return "", "", storageConfigError("s3.endpoint", "must be an absolute http or https URL")
	}
	if parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", "", storageConfigError("s3.endpoint", "must not contain user info, query, or fragment")
	}
	if parsed.Path != "" && parsed.Path != "/" {
		return "", "", storageConfigError("s3.endpoint", "must not contain a path")
	}
	parsed.Path = ""
	return strings.TrimSuffix(parsed.String(), "/"), parsed.Scheme, nil
}

func validS3BucketName(bucket string) bool {
	if len(bucket) < 3 || len(bucket) > 63 || net.ParseIP(bucket) != nil ||
		strings.Contains(bucket, "..") || strings.Contains(bucket, ".-") || strings.Contains(bucket, "-.") {
		return false
	}
	if !asciiLowerOrDigit(bucket[0]) || !asciiLowerOrDigit(bucket[len(bucket)-1]) {
		return false
	}
	for i := range len(bucket) {
		c := bucket[i]
		if asciiLowerOrDigit(c) || c == '-' || c == '.' {
			continue
		}
		return false
	}
	return true
}

func asciiLowerOrDigit(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= '0' && c <= '9'
}

func resolveRequiredSecret(ctx context.Context, secrets SecretResolver, field string, ref SecretRef) (string, error) {
	if _, err := envSecretName(ref); err != nil {
		return "", storageConfigError(field, "must be a valid env:// reference")
	}
	value, err := secrets.Resolve(ctx, ref)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return "", err
		}
		return "", storageConfigError(field, "could not be resolved")
	}
	if value == "" {
		return "", storageConfigError(field, "resolved to an empty value")
	}
	return value, nil
}

func envSecretName(ref SecretRef) (string, error) {
	const prefix = "env://"
	value := string(ref)
	if !strings.HasPrefix(value, prefix) {
		return "", ErrInvalidStorageConfiguration
	}
	name := strings.TrimPrefix(value, prefix)
	if name == "" {
		return "", ErrInvalidStorageConfiguration
	}
	for i := range len(name) {
		c := name[i]
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c == '_' || (i > 0 && c >= '0' && c <= '9') {
			continue
		}
		return "", ErrInvalidStorageConfiguration
	}
	return name, nil
}

func storageConfigError(field, reason string) error {
	return &StorageConfigurationError{Field: field, Reason: reason}
}

// StorageConfigurationError identifies a bad field without retaining its value.
type StorageConfigurationError struct {
	Field  string
	Reason string
}

func (e *StorageConfigurationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Reason)
}

func (e *StorageConfigurationError) Unwrap() error {
	return ErrInvalidStorageConfiguration
}

func (p *StorageProfile) Provider() ProviderType {
	if p == nil {
		return ""
	}
	return p.provider
}

// String and GoString deliberately expose only non-secret profile identity.
// They prevent ordinary structured/debug formatting from reflecting captured
// credential or TLS material.
func (p *StorageProfile) String() string {
	if p == nil {
		return "StorageProfile<nil>"
	}
	return fmt.Sprintf("StorageProfile{provider:%q}", p.provider)
}

func (p *StorageProfile) GoString() string {
	return p.String()
}

// LocalRoot exposes only the non-secret local root required by the Admin local
// writer. S3 profiles return false.
func (p *StorageProfile) LocalRoot() (string, bool) {
	if p == nil || p.provider != Local {
		return "", false
	}
	return p.localRoot, true
}

// Build creates the single handle owned by this profile. Repeated or concurrent
// successful calls return the same handle. A failed or canceled construction
// does not poison the immutable profile and may be retried.
func (p *StorageProfile) Build(ctx context.Context) (*StorageHandle, error) {
	if p == nil {
		return nil, storageConfigError("storageProfile", "is required")
	}
	if ctx == nil {
		return nil, storageConfigError("storageProfile", "context is required")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	for {
		p.buildMu.Lock()
		if p.handle != nil {
			handle := p.handle
			p.buildMu.Unlock()
			return handle, nil
		}
		if p.buildDone != nil {
			done := p.buildDone
			p.buildMu.Unlock()
			select {
			case <-done:
				continue
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
		done := make(chan struct{})
		p.buildDone = done
		p.buildMu.Unlock()

		handle, err := p.build(ctx)
		p.buildMu.Lock()
		if err == nil {
			p.handle = handle
		}
		p.buildDone = nil
		close(done)
		p.buildMu.Unlock()
		return handle, err
	}
}

func (p *StorageProfile) build(ctx context.Context) (*StorageHandle, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if p.provider == Local {
		return newStorageHandle(p, nil, nil), nil
	}
	if p.provider != S3 {
		return nil, storageConfigError("storageProfile.provider", "is unsupported")
	}

	ownedTransport := newStorageHTTPTransport()
	tlsConfig := &tls.Config{MinVersion: tls.VersionTLS12}
	if p.newTLSConfig != nil {
		tlsConfig = p.newTLSConfig()
	}
	ownedTransport.TLSClientConfig = tlsConfig
	httpClient := &http.Client{Transport: ownedTransport}

	var awsConfig aws.Config
	closeTransports := ownedTransport.CloseIdleConnections
	if p.credentialMode == storageCredentialsDefaultChain {
		credentialTransport := newStorageHTTPTransport()
		credentialHTTPClient := &http.Client{Transport: credentialTransport}
		var err error
		awsConfig, err = awsconfig.LoadDefaultConfig(
			ctx,
			awsconfig.WithRegion(p.region),
			awsconfig.WithHTTPClient(credentialHTTPClient),
		)
		if err != nil {
			ownedTransport.CloseIdleConnections()
			credentialTransport.CloseIdleConnections()
			return nil, fmt.Errorf("load explicit S3 default credential chain: %w", err)
		}
		// Credential providers such as WebIdentity may contact public STS. Keep
		// their transport independent from the S3 endpoint's private CA or mTLS
		// identity, then scope the owned transport to object requests only.
		awsConfig.HTTPClient = httpClient
		closeTransports = func() {
			ownedTransport.CloseIdleConnections()
			credentialTransport.CloseIdleConnections()
		}
	} else {
		if p.staticCredentials == nil {
			ownedTransport.CloseIdleConnections()
			return nil, storageConfigError("storageProfile.credentials", "static material is unavailable")
		}
		accessKeyID, secretAccessKey, sessionToken := p.staticCredentials()
		awsConfig = aws.Config{
			Region: p.region,
			Credentials: aws.NewCredentialsCache(credentials.NewStaticCredentialsProvider(
				accessKeyID,
				secretAccessKey,
				sessionToken,
			)),
			HTTPClient: httpClient,
		}
	}
	client := s3.NewFromConfig(awsConfig, func(options *s3.Options) {
		options.BaseEndpoint = aws.String(p.endpoint)
		options.UsePathStyle = p.usePathStyle
	})
	if client == nil {
		closeTransports()
		return nil, errors.New("construct S3 client: client is nil")
	}
	return newStorageHandle(p, client, closeTransports), nil
}

func newStorageHTTPTransport() *http.Transport {
	return &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: time.Second,
	}
}

// StorageHandle owns one profile client and its HTTP transport.
type StorageHandle struct {
	profile *StorageProfile
	client  *s3.Client

	mu          sync.Mutex
	closing     bool
	active      int
	drained     chan struct{}
	drainedOnce sync.Once
	closeIdle   func()
	closeOnce   sync.Once
}

func newStorageHandle(profile *StorageProfile, client *s3.Client, closeIdle func()) *StorageHandle {
	return &StorageHandle{profile: profile, client: client, closeIdle: closeIdle}
}

// Use leases the handle for one bounded operation. Local profiles receive a nil
// S3 client. The callback must not retain either callback argument.
func (h *StorageHandle) Use(ctx context.Context, operation func(*StorageProfile, *s3.Client) error) error {
	if h == nil {
		return errors.New("storage handle is required")
	}
	if ctx == nil {
		return errors.New("storage operation context is required")
	}
	if operation == nil {
		return errors.New("storage operation is required")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	h.mu.Lock()
	if h.closing {
		h.mu.Unlock()
		return ErrStorageHandleClosing
	}
	h.active++
	h.mu.Unlock()
	defer h.release()
	return operation(h.profile, h.client)
}

func (h *StorageHandle) release() {
	h.mu.Lock()
	h.active--
	shouldDrain := h.closing && h.active == 0
	h.mu.Unlock()
	if shouldDrain {
		h.drainedOnce.Do(func() { close(h.drained) })
		// Close may already have returned on its deadline. The final lease is
		// therefore responsible for retiring idle connections as well as waking
		// any Close retry.
		h.closeOwnedTransport()
	}
}

// Close rejects new leases, waits for existing leases to drain, then closes the
// owned transport once. A context timeout leaves the handle closing; a later
// Close call can finish the same shutdown.
func (h *StorageHandle) Close(ctx context.Context) error {
	if h == nil {
		return nil
	}
	if ctx == nil {
		return errors.New("storage close context is required")
	}
	h.mu.Lock()
	if !h.closing {
		h.closing = true
		h.drained = make(chan struct{})
		if h.active == 0 {
			h.drainedOnce.Do(func() { close(h.drained) })
		}
	}
	drained := h.drained
	h.mu.Unlock()

	select {
	case <-drained:
		h.closeOwnedTransport()
		return nil
	default:
	}
	select {
	case <-drained:
		h.closeOwnedTransport()
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (h *StorageHandle) closeOwnedTransport() {
	h.closeOnce.Do(func() {
		if h.closeIdle != nil {
			h.closeIdle()
		}
	})
}
