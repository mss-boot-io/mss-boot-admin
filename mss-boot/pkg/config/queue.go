package config

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2024/3/1 10:19:16
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2024/3/1 10:19:16
 */

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/IBM/sarama"
	"github.com/aws/aws-msk-iam-sasl-signer-go/signer"
	"github.com/redis/go-redis/v9"

	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/config/storage"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/config/storage/queue"
	"github.com/mss-boot-io/redisqueue/v2"
)

type Queue struct {
	Redis  *QueueRedis  `json:"redis" yaml:"redis"`
	Memory *QueueMemory `json:"memory" yaml:"memory"`
	NSQ    *QueueNSQ    `json:"nsq" yaml:"nsq"`
	Kafka  *Kafka       `json:"kafka" yaml:"kafka"`
}

type QueueRedis struct {
	storage.RedisConnectOptions
	Producer *redisqueue.ProducerOptions
	Consumer *redisqueue.ConsumerOptions
}

type QueueMemory struct {
	PoolSize uint `yaml:"poolSize" json:"poolSize"`
}

type QueueNSQ struct {
	storage.NSQOptions `yaml:",inline" json:",inline"`
}

type Kafka struct {
	KafkaParams `yaml:",inline" json:",inline"`
	SASL        *SASL `yaml:"sasl" json:"sasl"`
}

type KafkaParams struct {
	Brokers   []string      `yaml:"brokers" json:"brokers"`
	CaFile    string        `yaml:"caFile" json:"caFile"`
	CertFile  string        `yaml:"certFile" json:"certFile"`
	KeyFile   string        `yaml:"keyFile" json:"keyFile"`
	Timeout   time.Duration `yaml:"timeout" json:"timeout"` // default: 30
	KeepAlive time.Duration `yaml:"keepAlive" json:"keepAlive"`
	Version   string        `yaml:"version" json:"version"`
	Provider  string        `yaml:"provider" json:"provider"`
}

const defaultKafkaDialTimeout = 10 * time.Second
const legacyQueueInitTimeout = 30 * time.Second

// buildConfig validates the complete startup profile and binds caller-owned
// context to providers that may perform credential resolution later.
func (k *Kafka) buildConfig(ctx context.Context) (*sarama.Config, error) {
	if k == nil {
		return nil, errors.New("Kafka configuration is required")
	}
	if ctx == nil {
		return nil, errors.New("Kafka startup context is required")
	}
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("Kafka startup context: %w", err)
	}
	if err := validateKafkaBrokerAddresses(k.Brokers); err != nil {
		return nil, err
	}
	return k.buildSaramaConfig(ctx)
}

func (k *Kafka) buildSaramaConfig(ctx context.Context) (*sarama.Config, error) {
	if k == nil {
		return nil, errors.New("Kafka configuration is required")
	}
	if ctx == nil {
		return nil, errors.New("Kafka startup context is required")
	}
	provider := strings.ToLower(strings.TrimSpace(k.Provider))
	if provider != "" && provider != "kafka" && provider != "msk" {
		return nil, fmt.Errorf("unsupported Kafka provider %q", k.Provider)
	}
	if k.Timeout < 0 {
		return nil, errors.New("Kafka timeout must not be negative")
	}

	c := sarama.NewConfig()
	if k.Timeout == 0 {
		c.Net.DialTimeout = defaultKafkaDialTimeout
	} else {
		c.Net.DialTimeout = k.Timeout
	}
	c.Net.KeepAlive = k.KeepAlive
	c.Net.TLS.Enable = true
	c.Net.TLS.Config = &tls.Config{MinVersion: tls.VersionTLS12}
	c.Producer.Return.Errors = true
	c.Producer.Return.Successes = true
	c.Consumer.Return.Errors = true

	if strings.TrimSpace(k.Version) != "" {
		version, err := sarama.ParseKafkaVersion(strings.TrimSpace(k.Version))
		if err != nil {
			return nil, fmt.Errorf("parse Kafka version %q: %w", k.Version, err)
		}
		c.Version = version
	}

	if provider == "msk" {
		if err := k.configureMSK(ctx, c); err != nil {
			return nil, err
		}
	} else {
		if err := k.configureTLS(c); err != nil {
			return nil, err
		}
		if err := k.configureSASL(c); err != nil {
			return nil, err
		}
	}
	if err := c.Validate(); err != nil {
		return nil, fmt.Errorf("validate Kafka configuration: %w", err)
	}
	return c, nil
}

func validateKafkaBrokerAddresses(brokers []string) error {
	if len(brokers) == 0 {
		return errors.New("at least one Kafka broker is required")
	}
	for index, broker := range brokers {
		broker = strings.TrimSpace(broker)
		if broker == "" {
			return fmt.Errorf("Kafka broker %d is empty", index)
		}
		host, portText, err := net.SplitHostPort(broker)
		if err != nil || strings.TrimSpace(host) == "" {
			return fmt.Errorf("Kafka broker %d must use host:port syntax", index)
		}
		port, err := strconv.Atoi(portText)
		if err != nil || port < 1 || port > 65535 {
			return fmt.Errorf("Kafka broker %d has invalid port %q", index, portText)
		}
	}
	return nil
}

func (k *Kafka) configureTLS(c *sarama.Config) error {
	tlsConfig := c.Net.TLS.Config
	if k.CaFile != "" {
		caCertPool := x509.NewCertPool()
		if !caCertPool.AppendCertsFromPEM([]byte(k.CaFile)) {
			return errors.New("Kafka CA certificate is not valid PEM")
		}
		tlsConfig.RootCAs = caCertPool
	}
	if k.KeyFile == "" && k.CertFile == "" {
		return nil
	}
	if k.KeyFile == "" || k.CertFile == "" {
		return errors.New("Kafka client certificate and key must be configured together")
	}
	clientCert, err := tls.X509KeyPair([]byte(k.CertFile), []byte(k.KeyFile))
	if err != nil {
		return fmt.Errorf("load Kafka client certificate: %w", err)
	}
	tlsConfig.Certificates = []tls.Certificate{clientCert}
	return nil
}

func (k *Kafka) configureMSK(ctx context.Context, c *sarama.Config) error {
	if k.CaFile != "" || k.CertFile != "" || k.KeyFile != "" {
		return errors.New("MSK provider does not accept custom Kafka TLS material")
	}
	if k.SASL == nil || strings.TrimSpace(k.SASL.Region) == "" {
		return errors.New("MSK provider requires a SASL region")
	}
	if k.SASL.User != "" || k.SASL.Password != "" || k.SASL.AuthIdentity != "" ||
		k.SASL.SCRAMAuthzID != "" || k.SASL.Mechanism != "" || k.SASL.Version != 0 ||
		!reflect.DeepEqual(k.SASL.GSSAPI, sarama.GSSAPIConfig{}) {
		return errors.New("MSK provider does not accept static SASL credentials or mechanisms")
	}
	c.Net.SASL.Enable = true
	c.Net.SASL.Mechanism = sarama.SASLTypeOAuth
	c.Net.SASL.TokenProvider = &MSKAccessTokenProvider{
		Region: strings.TrimSpace(k.SASL.Region),
		Ctx:    ctx,
	}
	return nil
}

func (k *Kafka) configureSASL(c *sarama.Config) error {
	if k.SASL == nil {
		return nil
	}
	sasl := k.SASL
	if strings.TrimSpace(sasl.Region) != "" {
		return errors.New("Kafka SASL region is only valid for the MSK provider")
	}
	if !reflect.DeepEqual(sasl.GSSAPI, sarama.GSSAPIConfig{}) {
		return errors.New("Kafka GSSAPI configuration is not supported by this queue profile")
	}
	if !sasl.Enable {
		if sasl.User != "" || sasl.Password != "" || sasl.AuthIdentity != "" ||
			sasl.SCRAMAuthzID != "" || sasl.Mechanism != "" || sasl.Version != 0 || sasl.Handshake {
			return errors.New("Kafka SASL credentials require SASL to be enabled")
		}
		return nil
	}
	if strings.TrimSpace(sasl.User) == "" || sasl.Password == "" {
		return errors.New("Kafka SASL requires both user and password")
	}
	if sasl.Version != sarama.SASLHandshakeV0 && sasl.Version != sarama.SASLHandshakeV1 {
		return fmt.Errorf("unsupported Kafka SASL version %d", sasl.Version)
	}

	mechanism := sasl.Mechanism
	if mechanism == "" {
		mechanism = sarama.SASLTypePlaintext
	}
	switch mechanism {
	case sarama.SASLTypePlaintext:
	case sarama.SASLTypeSCRAMSHA256:
		c.Net.SASL.SCRAMClientGeneratorFunc = queue.SCRAMClientGeneratorFuncSHA256
	case sarama.SASLTypeSCRAMSHA512:
		c.Net.SASL.SCRAMClientGeneratorFunc = queue.SCRAMClientGeneratorFuncSHA512
	default:
		return fmt.Errorf("unsupported Kafka SASL mechanism %q", mechanism)
	}

	c.Net.SASL.Enable = true
	c.Net.SASL.User = strings.TrimSpace(sasl.User)
	c.Net.SASL.Password = sasl.Password
	c.Net.SASL.Mechanism = mechanism
	c.Net.SASL.Version = sasl.Version
	c.Net.SASL.AuthIdentity = sasl.AuthIdentity
	c.Net.SASL.SCRAMAuthzID = sasl.SCRAMAuthzID
	// Sarama defaults to a handshake. Keep that safe default because the
	// historical bool field cannot distinguish omitted from explicit false.
	if sasl.Handshake {
		c.Net.SASL.Handshake = true
	}
	if sasl.Version == sarama.SASLHandshakeV0 {
		c.ApiVersionsRequest = false
	}
	return nil
}

type MSKAccessTokenProvider struct {
	Region      string
	Ctx         context.Context
	mu          sync.Mutex
	accessToken *sarama.AccessToken
	expired     time.Time
}

func (m *MSKAccessTokenProvider) Token() (*sarama.AccessToken, error) {
	if m == nil {
		return nil, errors.New("MSK token provider is required")
	}
	if m.Ctx == nil {
		return nil, errors.New("MSK token provider context is required")
	}
	if err := m.Ctx.Err(); err != nil {
		return nil, fmt.Errorf("MSK token provider context: %w", err)
	}
	region := strings.TrimSpace(m.Region)
	if region == "" {
		return nil, errors.New("MSK token provider region is required")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.accessToken != nil && time.Now().Before(m.expired) {
		return m.accessToken, nil
	}
	token, expirationTimeMs, err := signer.GenerateAuthToken(m.Ctx, region)
	if err != nil {
		return nil, err
	}
	m.expired = time.UnixMilli(expirationTimeMs).Add(-time.Minute)
	m.accessToken = &sarama.AccessToken{Token: token}
	return m.accessToken, nil
}

type SASL struct {
	Region string `yaml:"region" json:"region"`
	// Whether or not to use SASL authentication when connecting to the broker
	// (defaults to false).
	Enable bool `yaml:"enable" json:"enable"`
	// SASLMechanism is the name of the enabled SASL mechanism.
	// Possible values: OAUTHBEARER, PLAIN (defaults to PLAIN).
	Mechanism sarama.SASLMechanism `yaml:"mechanism" json:"mechanism"`
	// Version is the SASL Protocol Version to use
	// Kafka > 1.x should use V1, except on Azure EventHub which use V0
	Version int16 `yaml:"version" json:"version"`
	// Whether or not to send the Kafka SASL handshake first if enabled
	// (defaults to true). You should only set this to false if you're using
	// a non-Kafka SASL proxy.
	Handshake bool `yaml:"handshake" json:"handshake"`
	// AuthIdentity is an (optional) authorization identity (authzid) to
	// use for SASL/PLAIN authentication (if different from User) when
	// an authenticated user is permitted to act as the presented
	// alternative user. See RFC4616 for details.
	AuthIdentity string `yaml:"authIdentity" json:"authIdentity"`
	// User is the authentication identity (authcid) to present for
	// SASL/PLAIN or SASL/SCRAM authentication
	User string `yaml:"user" json:"user"`
	// Password for SASL/PLAIN authentication
	Password string `yaml:"password" json:"password"`
	// authz id used for SASL/SCRAM authentication
	SCRAMAuthzID string `yaml:"scramAuthzID" json:"scramAuthzID"`
	// SCRAMClientGeneratorFunc is a generator of a user provided implementation of a SCRAM
	// client used to perform the SCRAM exchange with the server.
	// SCRAMClientGeneratorFunc func() SCRAMClient
	// TokenProvider is a user-defined callback for generating
	// access tokens for SASL/OAUTHBEARER auth. See the
	// AccessTokenProvider interface docs for proper implementation
	// guidelines.
	// TokenProvider AccessTokenProvider

	GSSAPI sarama.GSSAPIConfig `yaml:"gssapi" json:"gssapi"`
}

// Empty reports whether no queue adapter is configured.
func (e *Queue) Empty() bool {
	return e == nil || (e.Memory == nil && e.Redis == nil && e.NSQ == nil && e.Kafka == nil)
}

// Init is the legacy compatibility bridge. It logs initialization failures;
// new owners should use InitContext and handle the returned error.
func (e *Queue) Init(set func(storage.AdapterQueue)) {
	if e == nil || e.Empty() {
		return
	}
	if set == nil {
		slog.Error("queue initialization skipped: adapter installer is required")
		return
	}
	if e != nil && e.Redis == nil && e.NSQ == nil && e.Kafka != nil &&
		strings.EqualFold(strings.TrimSpace(e.Kafka.Provider), "msk") {
		slog.Error("queue initialization failed: MSK requires InitContext with an owner context")
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), legacyQueueInitTimeout)
	defer cancel()
	err := e.InitContext(ctx, func(adapter storage.AdapterQueue) error {
		set(adapter)
		return nil
	})
	if err != nil {
		slog.Error("queue initialization failed", "err", err)
	}
}

// InitContext enables the first configured queue in compatibility order:
// Redis > NSQ > Kafka > Memory.
func (e *Queue) InitContext(
	ctx context.Context,
	set func(storage.AdapterQueue) error,
) error {
	if e == nil || e.Empty() {
		return nil
	}
	if ctx == nil {
		return errors.New("queue startup context is required")
	}
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("queue startup context: %w", err)
	}
	if set == nil {
		return errors.New("queue adapter installer is required")
	}

	if e.Redis != nil {
		producerOptions := &redisqueue.ProducerOptions{}
		if e.Redis.Producer != nil {
			copy := *e.Redis.Producer
			producerOptions = &copy
		}
		consumerOptions := &redisqueue.ConsumerOptions{}
		if e.Redis.Consumer != nil {
			copy := *e.Redis.Consumer
			consumerOptions = &copy
		}
		consumerOptions.ReclaimInterval *= time.Second
		consumerOptions.BlockingTimeout *= time.Second
		consumerOptions.VisibilityTimeout *= time.Second

		client := storage.GetRedisClient()
		ownsClient := false
		if client == nil {
			options, err := e.Redis.GetRedisOptions()
			if err != nil {
				return fmt.Errorf("build Redis queue options: %w", err)
			}
			client = redis.NewUniversalClient(options)
			ownsClient = true
		}
		producerOptions.RedisClient = client
		consumerOptions.RedisClient = client
		q, err := queue.NewRedis(producerOptions, consumerOptions)
		if err != nil {
			if ownsClient {
				_ = client.Close()
			}
			return fmt.Errorf("create Redis queue: %w", err)
		}
		if err := set(q); err != nil {
			q.Shutdown()
			if ownsClient {
				_ = client.Close()
			}
			return fmt.Errorf("install Redis queue: %w", err)
		}
		if ownsClient {
			storage.SetRedisClient(client)
		}
		return nil
	}
	if e.NSQ != nil {
		cfg, err := e.NSQ.GetNSQOptions()
		if err != nil {
			return fmt.Errorf("build NSQ queue options: %w", err)
		}
		q, err := queue.NewNSQ(cfg, e.NSQ.LookupdAddr, e.NSQ.AdminAddr, e.NSQ.Addresses...)
		if err != nil {
			return fmt.Errorf("create NSQ queue: %w", err)
		}
		if err := set(q); err != nil {
			q.Shutdown()
			return fmt.Errorf("install NSQ queue: %w", err)
		}
		return nil
	}
	if e.Kafka != nil {
		cfg, err := e.Kafka.buildConfig(ctx)
		if err != nil {
			return fmt.Errorf("build Kafka queue configuration: %w", err)
		}
		provider := strings.ToLower(strings.TrimSpace(e.Kafka.Provider))
		q, err := queue.NewKafka(e.Kafka.Brokers, cfg, &queue.MessageHandler{}, provider)
		if err != nil {
			return fmt.Errorf("create Kafka queue: %w", err)
		}
		if err := set(q); err != nil {
			_ = q.Close(ctx)
			return fmt.Errorf("install Kafka queue: %w", err)
		}
		return nil
	}
	if e.Memory != nil {
		if err := set(queue.NewMemory(e.Memory.PoolSize)); err != nil {
			return fmt.Errorf("install memory queue: %w", err)
		}
		return nil
	}
	return nil
}
