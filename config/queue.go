package config

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2024/3/1 10:19:16
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2024/3/1 10:19:16
 */

import (
	"crypto/tls"
	"log"
	"time"

	"github.com/IBM/sarama"

	"github.com/mss-boot-io/mss-boot-admin/center"

	"github.com/mss-boot-io/redisqueue/v2"
	"github.com/redis/go-redis/v9"

	"github.com/mss-boot-io/mss-boot-admin/storage/queue"
)

type Queue struct {
	Redis  *QueueRedis
	Memory *QueueMemory
	NSQ    *QueueNSQ `json:"nsq" yaml:"nsq"`
	Kafka  *Kafka    `json:"kafka" yaml:"kafka"`
}

type QueueRedis struct {
	RedisConnectOptions `y`
	Producer            *redisqueue.ProducerOptions
	Consumer            *redisqueue.ConsumerOptions
}

type QueueMemory struct {
	PoolSize uint
}

type QueueNSQ struct {
	NSQOptions `yaml:",inline" json:",inline"`
}

type Kafka struct {
	KafkaParams `yaml:",inline" json:",inline"`
	SASL        *SASL `yaml:"sasl" json:"sasl"`
}

type KafkaParams struct {
	Brokers   []string      `yaml:"brokers" json:"brokers"`
	CertFile  string        `yaml:"certFile" json:"certFile"`
	KeyFile   string        `yaml:"keyFile" json:"keyFile"`
	Timeout   time.Duration `yaml:"timeout" json:"timeout"` // default: 30
	KeepAlive time.Duration `yaml:"keepAlive" json:"keepAlive"`
	Version   string        `yaml:"version" json:"version"`
}

func (k *Kafka) getConfig() *sarama.Config {
	c := sarama.NewConfig()
	if k.Timeout == 0 {
		c.Net.DialTimeout = 10 * time.Second
	}
	if k.KeepAlive != 0 {
		c.Net.KeepAlive = k.KeepAlive
	}
	c.Net.TLS.Enable = true
	if k.KeyFile == "" && k.CertFile == "" {
		c.Net.TLS.Config = &tls.Config{
			InsecureSkipVerify: true,
			ClientAuth:         0,
		}
	}
	if k.SASL != nil {
		c.Net.SASL.Enable = k.SASL.Enable
		c.Net.SASL.User = k.SASL.User
		c.Net.SASL.Password = k.SASL.Password
		c.Net.SASL.Mechanism = k.SASL.Mechanism
	}
	c.Version = sarama.V1_0_0_0
	if k.Version != "" {
		v, err := sarama.ParseKafkaVersion(k.Version)
		if err == nil {
			c.Version = v
		}
	}
	c.Producer.Return.Successes = true
	return c
}

type SASL struct {
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
	//SCRAMClientGeneratorFunc func() SCRAMClient
	// TokenProvider is a user-defined callback for generating
	// access tokens for SASL/OAUTHBEARER auth. See the
	// AccessTokenProvider interface docs for proper implementation
	// guidelines.
	//TokenProvider AccessTokenProvider

	GSSAPI sarama.GSSAPIConfig `yaml:"gssapi" json:"gssapi"`
}

// Empty 空设置
func (e *Queue) Empty() bool {
	return e.Memory == nil && e.Redis == nil && e.NSQ == nil
}

// Init 启用顺序 Redis > NSQ > Memory
func (e *Queue) Init() {
	if e.Redis != nil {
		e.Redis.Consumer.ReclaimInterval = e.Redis.Consumer.ReclaimInterval * time.Second
		e.Redis.Consumer.BlockingTimeout = e.Redis.Consumer.BlockingTimeout * time.Second
		e.Redis.Consumer.VisibilityTimeout = e.Redis.Consumer.VisibilityTimeout * time.Second
		client := GetRedisClient()
		if client == nil {
			options, err := e.Redis.RedisConnectOptions.GetRedisOptions()
			if err != nil {
				log.Fatalf("queue redis init error: %s", err.Error())
			}
			client = redis.NewClient(options)
			_redis = client
		}
		e.Redis.Producer.RedisClient = client
		e.Redis.Consumer.RedisClient = client
		q, err := queue.NewRedis(e.Redis.Producer, e.Redis.Consumer)
		if err != nil {
			log.Fatalf("queue redis init error: %s", err.Error())
		}
		center.SetQueue(q)
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

		center.SetQueue(q)
		return
	}
	if e.Kafka != nil {
		q, err := queue.NewKafka(e.Kafka.Brokers, e.Kafka.getConfig(), &queue.MessageHandler{})
		if err != nil {
			log.Fatalf("queue kafka init error: %s", err.Error())
		}
		center.SetQueue(q)
		return
	}
	if e.Memory != nil {
		center.SetQueue(queue.NewMemory(e.Memory.PoolSize))
		return
	}
}
