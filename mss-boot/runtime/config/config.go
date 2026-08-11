package config

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Config contains named runtime resources. Resource names are stable identity
// and are never inferred from map iteration order.
type Config struct {
	Resources map[string]ResourceConfig `yaml:"resources,omitempty" json:"resources,omitempty"`
}

type ProviderKind string

const ProviderRedis ProviderKind = "redis"

// ResourceConfig is a strict provider discriminator. The current additive
// checkpoint supports Redis only; future provider branches can be added
// without changing the existing branch.
type ResourceConfig struct {
	Provider ProviderConfig `yaml:"provider" json:"provider"`
}

type ProviderConfig struct {
	Kind  ProviderKind `yaml:"kind" json:"kind"`
	Redis *RedisConfig `yaml:"redis,omitempty" json:"redis,omitempty"`
}

type RedisMode string

const (
	RedisStandalone RedisMode = "standalone"
	RedisSentinel   RedisMode = "sentinel"
	RedisCluster    RedisMode = "cluster"
)

// RedisConfig uses both a mode discriminator and exactly one matching mode
// branch, preventing a stale field from silently changing deployment mode.
type RedisConfig struct {
	Mode       RedisMode              `yaml:"mode" json:"mode"`
	Standalone *RedisStandaloneConfig `yaml:"standalone,omitempty" json:"standalone,omitempty"`
	Sentinel   *RedisSentinelConfig   `yaml:"sentinel,omitempty" json:"sentinel,omitempty"`
	Cluster    *RedisClusterConfig    `yaml:"cluster,omitempty" json:"cluster,omitempty"`

	Database     int                    `yaml:"database,omitempty" json:"database,omitempty"`
	Credentials  RedisCredentialsConfig `yaml:"credentials" json:"credentials"`
	TLS          *RedisTLSConfig        `yaml:"tls,omitempty" json:"tls,omitempty"`
	DialTimeout  Duration               `yaml:"dialTimeout,omitempty" json:"dialTimeout,omitempty"`
	ReadTimeout  Duration               `yaml:"readTimeout,omitempty" json:"readTimeout,omitempty"`
	WriteTimeout Duration               `yaml:"writeTimeout,omitempty" json:"writeTimeout,omitempty"`
}

type RedisStandaloneConfig struct {
	Endpoint string `yaml:"endpoint" json:"endpoint"`
}

type RedisSentinelConfig struct {
	Endpoints  []string `yaml:"endpoints" json:"endpoints"`
	MasterName string   `yaml:"masterName" json:"masterName"`
}

type RedisClusterConfig struct {
	Endpoints []string `yaml:"endpoints" json:"endpoints"`
}

type RedisCredentialKind string

const (
	RedisCredentialsAnonymous RedisCredentialKind = "anonymous"
	RedisCredentialsPassword  RedisCredentialKind = "password"
)

// RedisCredentialsConfig makes authentication fallback impossible: callers
// must explicitly choose anonymous access or the password/ACL branch.
type RedisCredentialsConfig struct {
	Kind      RedisCredentialKind              `yaml:"kind" json:"kind"`
	Anonymous *RedisAnonymousCredentialsConfig `yaml:"anonymous,omitempty" json:"anonymous,omitempty"`
	Password  *RedisPasswordCredentialsConfig  `yaml:"password,omitempty" json:"password,omitempty"`
}

type RedisAnonymousCredentialsConfig struct{}

type RedisPasswordCredentialsConfig struct {
	UsernameRef SecretRef `yaml:"usernameRef,omitempty" json:"usernameRef,omitempty"`
	PasswordRef SecretRef `yaml:"passwordRef" json:"passwordRef"`
}

// RedisTLSConfig enables server-authenticated TLS. ClientCertificateRef and
// ClientKeyRef form an inseparable mTLS pair. InsecureSkipVerify is
// intentionally absent from the schema.
type RedisTLSConfig struct {
	MinVersion           string    `yaml:"minVersion,omitempty" json:"minVersion,omitempty"`
	ServerName           string    `yaml:"serverName,omitempty" json:"serverName,omitempty"`
	CARef                SecretRef `yaml:"caRef,omitempty" json:"caRef,omitempty"`
	ClientCertificateRef SecretRef `yaml:"clientCertificateRef,omitempty" json:"clientCertificateRef,omitempty"`
	ClientKeyRef         SecretRef `yaml:"clientKeyRef,omitempty" json:"clientKeyRef,omitempty"`
}

const (
	defaultDialTimeout  = 5 * time.Second
	defaultReadTimeout  = 3 * time.Second
	defaultWriteTimeout = 3 * time.Second
)

type validatedResource struct {
	name  string
	redis validatedRedis
}

type validatedRedis struct {
	mode         RedisMode
	endpoints    []string
	masterName   string
	database     int
	dialTimeout  time.Duration
	readTimeout  time.Duration
	writeTimeout time.Duration
	credentials  validatedCredentials
	tls          *validatedTLS
}

type validatedCredentials struct {
	kind        RedisCredentialKind
	usernameRef SecretRef
	passwordRef SecretRef
}

type validatedTLS struct {
	minVersion           uint16
	serverName           string
	caRef                SecretRef
	clientCertificateRef SecretRef
	clientKeyRef         SecretRef
}

// Normalize validates the entire graph before resolving any SecretRef, then
// copies all values into an immutable snapshot. It never mutates Config and
// never constructs a client, opens a network connection, or starts a goroutine.
func (c Config) Normalize(ctx context.Context, resolver SecretResolver) (*Snapshot, error) {
	if ctx == nil {
		return nil, invalid("runtime", "context is required")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	names := make([]string, 0, len(c.Resources))
	for name := range c.Resources {
		names = append(names, name)
	}
	sort.Strings(names)

	validated := make([]validatedResource, 0, len(names))
	requiresResolver := false
	for _, name := range names {
		path := "resources." + name
		if !validResourceName(name) {
			return nil, invalid("resources", "contains a non-canonical resource name")
		}
		resource, needsSecrets, err := validateResource(path, c.Resources[name])
		if err != nil {
			return nil, err
		}
		validated = append(validated, validatedResource{name: name, redis: resource})
		requiresResolver = requiresResolver || needsSecrets
	}
	if requiresResolver && resolver == nil {
		return nil, invalid("resources", "a secret resolver is required")
	}

	profiles := make(map[string]ResourceProfile, len(validated))
	for _, resource := range validated {
		profile, err := resolveResource(ctx, resolver, resource)
		if err != nil {
			return nil, err
		}
		profiles[resource.name] = profile
	}
	return &Snapshot{names: append([]string(nil), names...), resources: profiles}, nil
}

func validateResource(path string, resource ResourceConfig) (validatedRedis, bool, error) {
	branches := 0
	if resource.Provider.Redis != nil {
		branches++
	}
	if branches != 1 {
		return validatedRedis{}, false, invalid(path+".provider", "exactly one provider branch is required")
	}
	if resource.Provider.Kind != ProviderRedis {
		return validatedRedis{}, false, invalid(path+".provider.kind", "is unknown or does not match its branch")
	}
	return validateRedis(path+".provider.redis", *resource.Provider.Redis)
}

func validateRedis(path string, redis RedisConfig) (validatedRedis, bool, error) {
	branches := 0
	if redis.Standalone != nil {
		branches++
	}
	if redis.Sentinel != nil {
		branches++
	}
	if redis.Cluster != nil {
		branches++
	}
	if branches != 1 {
		return validatedRedis{}, false, invalid(path, "exactly one Redis mode branch is required")
	}

	result := validatedRedis{
		mode:         redis.Mode,
		database:     redis.Database,
		dialTimeout:  durationOrDefault(redis.DialTimeout, defaultDialTimeout),
		readTimeout:  durationOrDefault(redis.ReadTimeout, defaultReadTimeout),
		writeTimeout: durationOrDefault(redis.WriteTimeout, defaultWriteTimeout),
	}
	if result.database < 0 {
		return validatedRedis{}, false, invalid(path+".database", "must not be negative")
	}

	var endpoints []string
	switch redis.Mode {
	case RedisStandalone:
		if redis.Standalone == nil || redis.Sentinel != nil || redis.Cluster != nil {
			return validatedRedis{}, false, invalid(path+".mode", "does not match the standalone branch")
		}
		endpoints = []string{redis.Standalone.Endpoint}
	case RedisSentinel:
		if redis.Sentinel == nil || redis.Standalone != nil || redis.Cluster != nil {
			return validatedRedis{}, false, invalid(path+".mode", "does not match the sentinel branch")
		}
		if !validSentinelMasterName(redis.Sentinel.MasterName) {
			return validatedRedis{}, false, invalid(path+".sentinel.masterName", "is required and must be canonical")
		}
		result.masterName = redis.Sentinel.MasterName
		endpoints = redis.Sentinel.Endpoints
	case RedisCluster:
		if redis.Cluster == nil || redis.Standalone != nil || redis.Sentinel != nil {
			return validatedRedis{}, false, invalid(path+".mode", "does not match the cluster branch")
		}
		if redis.Database != 0 {
			return validatedRedis{}, false, invalid(path+".database", "must be zero in cluster mode")
		}
		endpoints = redis.Cluster.Endpoints
	default:
		return validatedRedis{}, false, invalid(path+".mode", "is unknown")
	}

	normalizedEndpoints, err := normalizeEndpoints(path, endpoints)
	if err != nil {
		return validatedRedis{}, false, err
	}
	result.endpoints = normalizedEndpoints

	credentials, needsCredentialSecrets, err := validateCredentials(path+".credentials", redis.Credentials)
	if err != nil {
		return validatedRedis{}, false, err
	}
	result.credentials = credentials
	tlsProfile, needsTLSSecrets, err := validateTLS(path+".tls", redis.TLS)
	if err != nil {
		return validatedRedis{}, false, err
	}
	result.tls = tlsProfile
	return result, needsCredentialSecrets || needsTLSSecrets, nil
}

func durationOrDefault(value Duration, fallback time.Duration) time.Duration {
	if parsed, ok := value.Value(); ok {
		return parsed
	}
	return fallback
}

func validResourceName(value string) bool {
	if value == "" || len(value) > 63 || value[0] < 'a' || value[0] > 'z' {
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

func validSentinelMasterName(value string) bool {
	if value == "" || len(value) > 128 || strings.TrimSpace(value) != value {
		return false
	}
	for index := range len(value) {
		character := value[index]
		if character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z' || character >= '0' && character <= '9' || character == '-' || character == '_' || character == '.' {
			continue
		}
		return false
	}
	return true
}

func normalizeEndpoints(path string, endpoints []string) ([]string, error) {
	if len(endpoints) == 0 {
		return nil, invalid(path+".endpoints", "must contain at least one endpoint")
	}
	result := make([]string, 0, len(endpoints))
	seen := make(map[string]struct{}, len(endpoints))
	for _, endpoint := range endpoints {
		normalized, err := normalizeEndpoint(endpoint)
		if err != nil {
			return nil, invalid(path+".endpoints", "contains an invalid host:port endpoint")
		}
		if _, exists := seen[normalized]; exists {
			return nil, invalid(path+".endpoints", "contains a duplicate endpoint")
		}
		seen[normalized] = struct{}{}
		result = append(result, normalized)
	}
	return result, nil
}

func normalizeEndpoint(value string) (string, error) {
	if value == "" || strings.TrimSpace(value) != value || strings.ContainsAny(value, "/?#@\x00") {
		return "", fmt.Errorf("invalid endpoint")
	}
	host, portText, err := net.SplitHostPort(value)
	if err != nil || host == "" {
		return "", fmt.Errorf("invalid endpoint")
	}
	port, err := strconv.Atoi(portText)
	if err != nil || port < 1 || port > 65535 {
		return "", fmt.Errorf("invalid endpoint")
	}
	if parsedIP := net.ParseIP(host); parsedIP != nil {
		if parsedIP.IsUnspecified() {
			return "", fmt.Errorf("invalid endpoint")
		}
		host = parsedIP.String()
	} else {
		host = strings.ToLower(host)
		if !validDNSName(host) {
			return "", fmt.Errorf("invalid endpoint")
		}
	}
	return net.JoinHostPort(host, strconv.Itoa(port)), nil
}

func validDNSName(value string) bool {
	if len(value) > 253 || strings.HasSuffix(value, ".") {
		return false
	}
	labels := strings.Split(value, ".")
	for _, label := range labels {
		if label == "" || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for index := range len(label) {
			character := label[index]
			if character >= 'a' && character <= 'z' || character >= '0' && character <= '9' || character == '-' {
				continue
			}
			return false
		}
	}
	return true
}

func validateCredentials(path string, credentials RedisCredentialsConfig) (validatedCredentials, bool, error) {
	branches := 0
	if credentials.Anonymous != nil {
		branches++
	}
	if credentials.Password != nil {
		branches++
	}
	if branches != 1 {
		return validatedCredentials{}, false, invalid(path, "exactly one credential branch is required")
	}
	switch credentials.Kind {
	case RedisCredentialsAnonymous:
		if credentials.Anonymous == nil || credentials.Password != nil {
			return validatedCredentials{}, false, invalid(path+".kind", "does not match the anonymous branch")
		}
		return validatedCredentials{kind: credentials.Kind}, false, nil
	case RedisCredentialsPassword:
		if credentials.Password == nil || credentials.Anonymous != nil {
			return validatedCredentials{}, false, invalid(path+".kind", "does not match the password branch")
		}
		if !credentials.Password.PasswordRef.valid() {
			return validatedCredentials{}, false, invalid(path+".password.passwordRef", "is required")
		}
		if credentials.Password.UsernameRef != (SecretRef{}) && !credentials.Password.UsernameRef.valid() {
			return validatedCredentials{}, false, invalid(path+".password.usernameRef", "is invalid")
		}
		return validatedCredentials{
			kind:        credentials.Kind,
			usernameRef: credentials.Password.UsernameRef,
			passwordRef: credentials.Password.PasswordRef,
		}, true, nil
	default:
		return validatedCredentials{}, false, invalid(path+".kind", "is unknown")
	}
}

func validateTLS(path string, value *RedisTLSConfig) (*validatedTLS, bool, error) {
	if value == nil {
		return nil, false, nil
	}
	minVersion := uint16(tls.VersionTLS12)
	switch value.MinVersion {
	case "", "1.2":
	case "1.3":
		minVersion = uint16(tls.VersionTLS13)
	default:
		return nil, false, invalid(path+".minVersion", "must be 1.2 or 1.3")
	}
	if value.ServerName != "" && !validTLSServerName(value.ServerName) {
		return nil, false, invalid(path+".serverName", "must be a valid DNS name or IP address")
	}
	if (value.ClientCertificateRef == (SecretRef{})) != (value.ClientKeyRef == (SecretRef{})) {
		return nil, false, invalid(path, "client certificate and key references must be configured together")
	}
	for field, ref := range map[string]SecretRef{
		"caRef":                value.CARef,
		"clientCertificateRef": value.ClientCertificateRef,
		"clientKeyRef":         value.ClientKeyRef,
	} {
		if ref != (SecretRef{}) && !ref.valid() {
			return nil, false, invalid(path+"."+field, "is invalid")
		}
	}
	needsSecrets := value.CARef != (SecretRef{}) || value.ClientCertificateRef != (SecretRef{})
	return &validatedTLS{
		minVersion:           minVersion,
		serverName:           strings.ToLower(value.ServerName),
		caRef:                value.CARef,
		clientCertificateRef: value.ClientCertificateRef,
		clientKeyRef:         value.ClientKeyRef,
	}, needsSecrets, nil
}

func validTLSServerName(value string) bool {
	if strings.TrimSpace(value) != value || strings.ContainsAny(value, ":/?#@\x00") {
		return false
	}
	return net.ParseIP(value) != nil || validDNSName(strings.ToLower(value))
}

func resolveResource(ctx context.Context, resolver SecretResolver, resource validatedResource) (ResourceProfile, error) {
	path := "resources." + resource.name + ".provider.redis"
	redisProfile := RedisProfile{
		mode:         resource.redis.mode,
		endpoints:    append([]string(nil), resource.redis.endpoints...),
		masterName:   resource.redis.masterName,
		database:     resource.redis.database,
		dialTimeout:  resource.redis.dialTimeout,
		readTimeout:  resource.redis.readTimeout,
		writeTimeout: resource.redis.writeTimeout,
		credentials: RedisCredentialsProfile{
			kind: resource.redis.credentials.kind,
		},
	}
	if resource.redis.credentials.kind == RedisCredentialsPassword {
		if resource.redis.credentials.usernameRef != (SecretRef{}) {
			username, err := resolveSecret(ctx, resolver, path+".credentials.password.usernameRef", resource.redis.credentials.usernameRef)
			if err != nil {
				return ResourceProfile{}, err
			}
			redisProfile.credentials.username = username
		}
		password, err := resolveSecret(ctx, resolver, path+".credentials.password.passwordRef", resource.redis.credentials.passwordRef)
		if err != nil {
			return ResourceProfile{}, err
		}
		redisProfile.credentials.password = password
	}
	if resource.redis.tls != nil {
		tlsProfile := RedisTLSProfile{
			minVersion: resource.redis.tls.minVersion,
			serverName: resource.redis.tls.serverName,
		}
		if resource.redis.tls.caRef != (SecretRef{}) {
			ca, err := resolveSecret(ctx, resolver, path+".tls.caRef", resource.redis.tls.caRef)
			if err != nil {
				return ResourceProfile{}, err
			}
			pool := x509.NewCertPool()
			if !pool.AppendCertsFromPEM([]byte(ca.Reveal())) {
				return ResourceProfile{}, &SecretResolutionError{
					Path:        path + ".tls.caRef",
					Source:      ca.Source(),
					Fingerprint: ca.Fingerprint(),
					Reason:      "resolved value is not a valid PEM certificate",
				}
			}
			tlsProfile.ca = ca
		}
		if resource.redis.tls.clientCertificateRef != (SecretRef{}) {
			certificate, err := resolveSecret(ctx, resolver, path+".tls.clientCertificateRef", resource.redis.tls.clientCertificateRef)
			if err != nil {
				return ResourceProfile{}, err
			}
			key, err := resolveSecret(ctx, resolver, path+".tls.clientKeyRef", resource.redis.tls.clientKeyRef)
			if err != nil {
				return ResourceProfile{}, err
			}
			if _, pairErr := tls.X509KeyPair([]byte(certificate.Reveal()), []byte(key.Reveal())); pairErr != nil {
				return ResourceProfile{}, &SecretResolutionError{
					Path:        path + ".tls",
					Source:      certificate.Source(),
					Fingerprint: certificate.Fingerprint(),
					Reason:      "resolved client certificate and key are invalid",
				}
			}
			tlsProfile.clientCertificate = certificate
			tlsProfile.clientKey = key
		}
		redisProfile.tls = &tlsProfile
	}
	return ResourceProfile{name: resource.name, provider: ProviderRedis, redis: &redisProfile}, nil
}
