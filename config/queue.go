package config

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2024/3/1 10:19:16
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2024/3/1 10:19:16
 */

import (
	"log"
	"time"

	"github.com/segmentio/kafka-go"

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
	RedisConnectOptions
	Producer *redisqueue.ProducerOptions
	Consumer *redisqueue.ConsumerOptions
}

type QueueMemory struct {
	PoolSize uint
}

type QueueNSQ struct {
	NSQOptions
}

type Kafka struct {
	Brokers      []string            `yaml:"brokers" json:"brokers"`
	Partition    int                 `yaml:"partition" json:"partition"`
	ReaderConfig *kafka.ReaderConfig `yaml:"readerConfig" json:"readerConfig"`
	WriterConfig *kafka.Writer       `yaml:"writerConfig" json:"writerConfig"`
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
		q, err := queue.NewNSQ(e.NSQ.Addresses, cfg)
		if err != nil {
			log.Fatalf("queue nsq init error: %s", err.Error())
		}
		center.SetQueue(q)
		return
	}
	if e.Kafka != nil {

	}
	center.SetQueue(queue.NewMemory(e.Memory.PoolSize))
}
