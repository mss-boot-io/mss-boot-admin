package config

import (
	"context"
	"crypto/tls"
	"fmt"
	"time"
)

// Snapshot is the immutable result of Normalize. All maps, slices, and secret
// values are private; accessors return copies.
type Snapshot struct {
	names     []string
	resources map[string]ResourceProfile
}

func (s *Snapshot) ResourceNames() []string {
	if s == nil {
		return nil
	}
	return append([]string(nil), s.names...)
}

func (s *Snapshot) Resource(name string) (ResourceProfile, bool) {
	if s == nil {
		return ResourceProfile{}, false
	}
	resource, ok := s.resources[name]
	return cloneResource(resource), ok
}

func (s *Snapshot) String() string {
	if s == nil {
		return "RuntimeConfigSnapshot<nil>"
	}
	return fmt.Sprintf("RuntimeConfigSnapshot{resources:%d}", len(s.names))
}

func (s *Snapshot) GoString() string {
	return s.String()
}

// Build creates a pure immutable construction plan. It performs only bounded
// in-memory copies: no provider constructor, DNS lookup, network connection,
// file open, environment lookup, or goroutine is invoked here.
func (s *Snapshot) Build(ctx context.Context) (*Plan, error) {
	if s == nil {
		return nil, invalid("snapshot", "is required")
	}
	if ctx == nil {
		return nil, invalid("snapshot", "context is required")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	resources := make(map[string]ResourceProfile, len(s.resources))
	for _, name := range s.names {
		resources[name] = cloneResource(s.resources[name])
	}
	return &Plan{names: append([]string(nil), s.names...), resources: resources}, nil
}

// Plan is provider-construction input, not a running resource. Provider
// packages may reveal secrets only while constructing their owned clients.
type Plan struct {
	names     []string
	resources map[string]ResourceProfile
}

func (p *Plan) ResourceNames() []string {
	if p == nil {
		return nil
	}
	return append([]string(nil), p.names...)
}

func (p *Plan) Resource(name string) (ResourceProfile, bool) {
	if p == nil {
		return ResourceProfile{}, false
	}
	resource, ok := p.resources[name]
	return cloneResource(resource), ok
}

func (p *Plan) String() string {
	if p == nil {
		return "RuntimeConfigPlan<nil>"
	}
	return fmt.Sprintf("RuntimeConfigPlan{resources:%d}", len(p.names))
}

func (p *Plan) GoString() string {
	return p.String()
}

type ResourceProfile struct {
	name     string
	provider ProviderKind
	redis    *RedisProfile
}

func (p ResourceProfile) Name() string {
	return p.name
}

func (p ResourceProfile) Provider() ProviderKind {
	return p.provider
}

func (p ResourceProfile) Redis() (RedisProfile, bool) {
	if p.redis == nil {
		return RedisProfile{}, false
	}
	return cloneRedis(*p.redis), true
}

func (p ResourceProfile) String() string {
	return fmt.Sprintf("ResourceProfile{name:%q provider:%q}", p.name, p.provider)
}

func (p ResourceProfile) GoString() string {
	return p.String()
}

type RedisProfile struct {
	mode         RedisMode
	endpoints    []string
	masterName   string
	database     int
	dialTimeout  time.Duration
	readTimeout  time.Duration
	writeTimeout time.Duration
	credentials  RedisCredentialsProfile
	tls          *RedisTLSProfile
}

func (p RedisProfile) Mode() RedisMode {
	return p.mode
}

func (p RedisProfile) Endpoints() []string {
	return append([]string(nil), p.endpoints...)
}

func (p RedisProfile) SentinelMasterName() string {
	return p.masterName
}

func (p RedisProfile) Database() int {
	return p.database
}

func (p RedisProfile) DialTimeout() time.Duration {
	return p.dialTimeout
}

func (p RedisProfile) ReadTimeout() time.Duration {
	return p.readTimeout
}

func (p RedisProfile) WriteTimeout() time.Duration {
	return p.writeTimeout
}

func (p RedisProfile) Credentials() RedisCredentialsProfile {
	return p.credentials
}

func (p RedisProfile) TLS() (RedisTLSProfile, bool) {
	if p.tls == nil {
		return RedisTLSProfile{}, false
	}
	return *p.tls, true
}

func (p RedisProfile) String() string {
	return fmt.Sprintf("RedisProfile{mode:%q endpoints:%d database:%d tls:%t credentials:%q}", p.mode, len(p.endpoints), p.database, p.tls != nil, p.credentials.kind)
}

func (p RedisProfile) GoString() string {
	return p.String()
}

type RedisCredentialsProfile struct {
	kind     RedisCredentialKind
	username ResolvedSecret
	password ResolvedSecret
}

func (p RedisCredentialsProfile) Kind() RedisCredentialKind {
	return p.kind
}

func (p RedisCredentialsProfile) Username() (ResolvedSecret, bool) {
	return p.username, p.username.source != ""
}

func (p RedisCredentialsProfile) Password() (ResolvedSecret, bool) {
	return p.password, p.password.source != ""
}

func (p RedisCredentialsProfile) String() string {
	return fmt.Sprintf("RedisCredentials{kind:%q username:%t password:%t}", p.kind, p.username.source != "", p.password.source != "")
}

func (p RedisCredentialsProfile) GoString() string {
	return p.String()
}

type RedisTLSProfile struct {
	minVersion        uint16
	serverName        string
	ca                ResolvedSecret
	clientCertificate ResolvedSecret
	clientKey         ResolvedSecret
}

func (p RedisTLSProfile) MinVersion() uint16 {
	return p.minVersion
}

func (p RedisTLSProfile) ServerName() string {
	return p.serverName
}

func (p RedisTLSProfile) CA() (ResolvedSecret, bool) {
	return p.ca, p.ca.source != ""
}

func (p RedisTLSProfile) ClientCertificate() (ResolvedSecret, bool) {
	return p.clientCertificate, p.clientCertificate.source != ""
}

func (p RedisTLSProfile) ClientKey() (ResolvedSecret, bool) {
	return p.clientKey, p.clientKey.source != ""
}

func (p RedisTLSProfile) String() string {
	version := "unknown"
	if p.minVersion == tls.VersionTLS12 {
		version = "1.2"
	} else if p.minVersion == tls.VersionTLS13 {
		version = "1.3"
	}
	return fmt.Sprintf("RedisTLS{minVersion:%q serverName:%q customCA:%t clientCertificate:%t}", version, p.serverName, p.ca.source != "", p.clientCertificate.source != "")
}

func (p RedisTLSProfile) GoString() string {
	return p.String()
}

func cloneResource(resource ResourceProfile) ResourceProfile {
	result := resource
	if resource.redis != nil {
		redis := cloneRedis(*resource.redis)
		result.redis = &redis
	}
	return result
}

func cloneRedis(redis RedisProfile) RedisProfile {
	result := redis
	result.endpoints = append([]string(nil), redis.endpoints...)
	if redis.tls != nil {
		tlsProfile := *redis.tls
		result.tls = &tlsProfile
	}
	return result
}
