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
	"log"
	"strings"
	"time"

	"github.com/IBM/sarama"
	"github.com/aws/aws-msk-iam-sasl-signer-go/signer"
	"github.com/redis/go-redis/v9"

	"github.com/mss-boot-io/mss-boot/pkg/config/storage"
	"github.com/mss-boot-io/mss-boot/pkg/config/storage/queue"
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

func (k *Kafka) getConfig() *sarama.Config {
	c := sarama.NewConfig()
	switch strings.ToLower(k.Provider) {
	case "msk":
		c.Net.SASL.Enable = true
		c.Net.SASL.Mechanism = sarama.SASLTypeOAuth
		c.Net.SASL.TokenProvider = &MSKAccessTokenProvider{
			Region: k.SASL.Region,
			Ctx:    context.Background(),
		}

		tlsConfig := tls.Config{}
		c.Net.TLS.Enable = true
		c.Net.TLS.Config = &tlsConfig
	default:

		if k.Timeout == 0 {
			c.Net.DialTimeout = 10 * time.Second
		}
		if k.KeepAlive != 0 {
			c.Net.KeepAlive = k.KeepAlive
		}
		c.Net.TLS.Enable = true
		tlsConfig := &tls.Config{
			InsecureSkipVerify: true,
			ClientAuth:         tls.NoClientCert,
		}

		if k.KeyFile != "" || k.CertFile != "" || k.CaFile != "" {
			caCertPool := x509.NewCertPool()
			if !caCertPool.AppendCertsFromPEM([]byte(k.CaFile)) {
				log.Fatalf("queue kafka failed to append CA certificate")
			}
			clientCert, err := tls.X509KeyPair([]byte(k.CertFile), []byte(k.KeyFile))
			if err != nil {
				log.Fatalf("queue kafka load cert error: %s", err.Error())
			}
			tlsConfig.RootCAs = caCertPool
			tlsConfig.Certificates = []tls.Certificate{clientCert}
		}
		c.Net.TLS.Config = tlsConfig

		if k.SASL != nil {
			c.Net.SASL.Enable = k.SASL.Enable
			c.Net.SASL.User = k.SASL.User
			c.Net.SASL.Password = k.SASL.Password
			c.Net.SASL.Mechanism = k.SASL.Mechanism
			switch c.Net.SASL.Mechanism {
			case sarama.SASLTypeSCRAMSHA256:
				c.Net.SASL.SCRAMClientGeneratorFunc = queue.SCRAMClientGeneratorFuncSHA256
				c.Net.SASL.Mechanism = sarama.SASLTypeSCRAMSHA256
			case sarama.SASLTypeSCRAMSHA512:
				c.Net.SASL.SCRAMClientGeneratorFunc = queue.SCRAMClientGeneratorFuncSHA512
				c.Net.SASL.Mechanism = sarama.SASLTypeSCRAMSHA512
			}
		}
		// c.Version = sarama.V1_0_0_0
		if k.Version != "" {
			v, err := sarama.ParseKafkaVersion(k.Version)
			if err == nil {
				c.Version = v
			}
		}
	}
	c.Producer.Return.Successes = true
	return c
}

type MSKAccessTokenProvider struct {
	Region      string
	Ctx         context.Context
	accessToken *sarama.AccessToken
	expired     time.Time
}

func (m *MSKAccessTokenProvider) Token() (*sarama.AccessToken, error) {
	if m.accessToken != nil && time.Now().Before(m.expired) {
		return m.accessToken, nil
	}
	if m.Ctx == nil {
		m.Ctx = context.Background()
	}
	token, expirationTimeMs, err := signer.GenerateAuthToken(m.Ctx, m.Region)
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

// Empty 空设置
func (e *Queue) Empty() bool {
	return e.Memory == nil && e.Redis == nil && e.NSQ == nil
}

// Init 启用顺序 Redis > NSQ > Memory
func (e *Queue) Init(set func(storage.AdapterQueue)) {
	if e.Redis != nil {
		e.Redis.Consumer.ReclaimInterval = e.Redis.Consumer.ReclaimInterval * time.Second
		e.Redis.Consumer.BlockingTimeout = e.Redis.Consumer.BlockingTimeout * time.Second
		e.Redis.Consumer.VisibilityTimeout = e.Redis.Consumer.VisibilityTimeout * time.Second
		client := storage.GetRedisClient()
		if client == nil {
			options, err := e.Redis.GetRedisOptions()
			if err != nil {
				log.Fatalf("queue redis init error: %s", err.Error())
			}
			client = redis.NewUniversalClient(options)
			storage.SetRedisClient(client)
		}
		e.Redis.Producer.RedisClient = client
		e.Redis.Consumer.RedisClient = client
		q, err := queue.NewRedis(e.Redis.Producer, e.Redis.Consumer)
		if err != nil {
			log.Fatalf("queue redis init error: %s", err.Error())
		}
		if set != nil {
			set(q)
		}
		return
	}
	if e.NSQ != nil {
		cfg, err := e.NSQ.GetNSQOptions()
		if err != nil {
			log.Fatalf("queue nsq init error: %s", err.Error())
		}
		q, err := queue.NewNSQ(cfg, e.NSQ.LookupdAddr, e.NSQ.AdminAddr, e.NSQ.Addresses...)
		if err != nil {
			log.Fatalf("queue nsq init error: %s", err.Error())
		}
		if set != nil {
			set(q)
		}
		return
	}
	if e.Kafka != nil {
		q, err := queue.NewKafka(e.Kafka.Brokers, e.Kafka.getConfig(), &queue.MessageHandler{}, e.Kafka.Provider)
		if err != nil {
			log.Fatalf("queue kafka init error: %s", err.Error())
		}
		if set != nil {
			set(q)
		}
		return
	}
	if e.Memory != nil {
		if set != nil {
			set(queue.NewMemory(e.Memory.PoolSize))
		}
		return
	}
}
