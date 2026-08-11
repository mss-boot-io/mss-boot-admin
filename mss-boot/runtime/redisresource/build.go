package redisresource

import (
	"crypto/tls"
	"crypto/x509"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	runtimeconfig "github.com/mss-boot-io/mss-boot-admin/mss-boot/runtime/config"
	runtimeresource "github.com/mss-boot-io/mss-boot-admin/mss-boot/runtime/resource"
)

const (
	maxScopeNameBytes  = 63
	maxLogicalKeyBytes = 512
)

type clientSpec struct {
	mode         runtimeconfig.RedisMode
	endpoints    []string
	masterName   string
	database     int
	dialTimeout  time.Duration
	readTimeout  time.Duration
	writeTimeout time.Duration
	username     string
	password     string
	tls          *tls.Config
}

func (s clientSpec) clone() clientSpec {
	result := s
	result.endpoints = append([]string(nil), s.endpoints...)
	if s.tls != nil {
		result.tls = s.tls.Clone()
	}
	return result
}

// Resource owns one delayed Redis client and implements runtime/resource's
// Start, Ready, Health, and Close contracts.
type Resource struct {
	name    string
	spec    clientSpec
	factory clientFactory

	scopeMu sync.Mutex
	scopes  map[string]*Scope

	state lifecycleState
}

// Build validates and privately copies a normalized Redis profile. It does
// not invoke the client factory and has no network or goroutine side effects.
func Build(profile runtimeconfig.ResourceProfile) (*Resource, error) {
	return buildWithFactory(profile, defaultClientFactory{})
}

func buildWithFactory(profile runtimeconfig.ResourceProfile, factory clientFactory) (*Resource, error) {
	if profile.Provider() != runtimeconfig.ProviderRedis {
		return nil, invalid("profile.provider", "must be redis")
	}
	redisProfile, ok := profile.Redis()
	if !ok {
		return nil, invalid("profile.redis", "is required")
	}
	if !validScopeName(profile.Name()) {
		return nil, invalid("profile.name", "must be canonical")
	}
	if factory == nil {
		return nil, invalid("clientFactory", "is required")
	}

	spec, err := makeClientSpec(redisProfile)
	if err != nil {
		return nil, err
	}
	return &Resource{
		name:    profile.Name(),
		spec:    spec,
		factory: factory,
		scopes:  make(map[string]*Scope),
		state:   newLifecycleState(),
	}, nil
}

func makeClientSpec(profile runtimeconfig.RedisProfile) (clientSpec, error) {
	result := clientSpec{
		mode:         profile.Mode(),
		endpoints:    profile.Endpoints(),
		masterName:   profile.SentinelMasterName(),
		database:     profile.Database(),
		dialTimeout:  profile.DialTimeout(),
		readTimeout:  profile.ReadTimeout(),
		writeTimeout: profile.WriteTimeout(),
	}
	if result.dialTimeout <= 0 || result.readTimeout <= 0 || result.writeTimeout <= 0 {
		return clientSpec{}, invalid("profile.redis.timeouts", "must be positive")
	}
	for _, endpoint := range result.endpoints {
		if endpoint == "" {
			return clientSpec{}, invalid("profile.redis.endpoints", "contains an empty endpoint")
		}
	}
	switch result.mode {
	case runtimeconfig.RedisStandalone:
		if len(result.endpoints) != 1 || result.masterName != "" {
			return clientSpec{}, invalid("profile.redis.standalone", "has inconsistent topology")
		}
	case runtimeconfig.RedisSentinel:
		if len(result.endpoints) == 0 || result.masterName == "" {
			return clientSpec{}, invalid("profile.redis.sentinel", "has inconsistent topology")
		}
	case runtimeconfig.RedisCluster:
		if len(result.endpoints) == 0 || result.masterName != "" {
			return clientSpec{}, invalid("profile.redis.cluster", "has inconsistent topology")
		}
		// Defense in depth. Normalize already enforces this, but a cluster
		// client must never silently accept a database selection.
		if result.database != 0 {
			return clientSpec{}, invalid("profile.redis.database", "must be zero in cluster mode")
		}
	default:
		return clientSpec{}, invalid("profile.redis.mode", "is unsupported")
	}

	credentials := profile.Credentials()
	switch credentials.Kind() {
	case runtimeconfig.RedisCredentialsAnonymous:
		if _, ok := credentials.Username(); ok {
			return clientSpec{}, invalid("profile.redis.credentials", "anonymous credentials contain a username")
		}
		if _, ok := credentials.Password(); ok {
			return clientSpec{}, invalid("profile.redis.credentials", "anonymous credentials contain a password")
		}
	case runtimeconfig.RedisCredentialsPassword:
		if username, ok := credentials.Username(); ok {
			result.username = username.Reveal()
		}
		password, ok := credentials.Password()
		if !ok || password.Reveal() == "" {
			return clientSpec{}, invalid("profile.redis.credentials.password", "is required")
		}
		result.password = password.Reveal()
	default:
		return clientSpec{}, invalid("profile.redis.credentials.kind", "is unsupported")
	}

	if tlsProfile, ok := profile.TLS(); ok {
		tlsConfig, err := makeTLSConfig(tlsProfile)
		if err != nil {
			return clientSpec{}, err
		}
		result.tls = tlsConfig
	}
	return result, nil
}

func makeTLSConfig(profile runtimeconfig.RedisTLSProfile) (*tls.Config, error) {
	if profile.MinVersion() != tls.VersionTLS12 && profile.MinVersion() != tls.VersionTLS13 {
		return nil, invalid("profile.redis.tls.minVersion", "must be TLS 1.2 or TLS 1.3")
	}
	result := &tls.Config{
		MinVersion: profile.MinVersion(),
		ServerName: profile.ServerName(),
	}
	if ca, ok := profile.CA(); ok {
		roots := x509.NewCertPool()
		if !roots.AppendCertsFromPEM([]byte(ca.Reveal())) {
			return nil, invalid("profile.redis.tls.ca", "is not a valid PEM certificate bundle")
		}
		result.RootCAs = roots
	}
	certificate, hasCertificate := profile.ClientCertificate()
	key, hasKey := profile.ClientKey()
	if hasCertificate != hasKey {
		return nil, invalid("profile.redis.tls.clientAuthentication", "requires both certificate and key")
	}
	if hasCertificate {
		pair, err := tls.X509KeyPair([]byte(certificate.Reveal()), []byte(key.Reveal()))
		if err != nil {
			return nil, invalid("profile.redis.tls.clientAuthentication", "contains an invalid certificate or key")
		}
		result.Certificates = []tls.Certificate{pair}
	}
	return result, nil
}

func validScopeName(value string) bool {
	if value == "" || len(value) > maxScopeNameBytes || value[0] < 'a' || value[0] > 'z' {
		return false
	}
	for index := range len(value) {
		character := value[index]
		if character >= 'a' && character <= 'z' || character >= '0' && character <= '9' || index > 0 && (character == '-' || character == '.') {
			continue
		}
		return false
	}
	return value[len(value)-1] != '-' && value[len(value)-1] != '.' && !strings.Contains(value, "..")
}

func validLogicalKey(value string) bool {
	if value == "" || len(value) > maxLogicalKeyBytes || !utf8.ValidString(value) || strings.TrimSpace(value) != value {
		return false
	}
	if strings.HasPrefix(value, "/") || strings.HasSuffix(value, "/") || strings.Contains(value, "//") || strings.ContainsAny(value, ":{}\\") {
		return false
	}
	for _, segment := range strings.Split(value, "/") {
		if segment == "" || segment == "." || segment == ".." {
			return false
		}
		for _, character := range segment {
			if character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z' || character >= '0' && character <= '9' || character == '-' || character == '_' || character == '.' {
				continue
			}
			return false
		}
	}
	return true
}

// Definition adapts the resource to a runtime/resource graph without exposing
// its client. Dependency names are copied by runtime/resource.Build.
func (r *Resource) Definition(required bool, dependencies ...string) runtimeresource.Definition {
	if r == nil {
		return runtimeresource.Definition{}
	}
	return runtimeresource.Definition{
		Name:         r.name,
		Dependencies: append([]string(nil), dependencies...),
		Required:     required,
		Resource:     r,
	}
}

func (r *Resource) String() string {
	if r == nil {
		return "RedisRuntimeResource<nil>"
	}
	return "RedisRuntimeResource{name:\"" + r.name + "\"}"
}

func (r *Resource) GoString() string { return r.String() }

var (
	_ runtimeresource.Resource         = (*Resource)(nil)
	_ runtimeresource.HealthChecker    = (*Resource)(nil)
	_ runtimeresource.ReadinessChecker = (*Resource)(nil)
)
