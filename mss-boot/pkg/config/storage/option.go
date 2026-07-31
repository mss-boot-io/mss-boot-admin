package storage

import (
	"time"

	"github.com/IBM/sarama"
)

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2024/4/9 11:28:26
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2024/4/9 11:28:26
 */

type Options struct {
	Topic                       string
	GroupID                     string
	F                           ConsumerFunc
	Message                     Messager
	Partition                   int
	PartitionAssignmentStrategy sarama.BalanceStrategy
	KafkaConfig                 *sarama.Config
	Delay                       time.Duration
}

func DefaultOptions() *Options {
	return &Options{
		Partition: -1,
	}
}

func SetOptions(opts ...Option) *Options {
	o := DefaultOptions()
	for _, opt := range opts {
		opt(o)
	}
	return o
}

type Option func(*Options)

func WithDelay(delay time.Duration) Option {
	return func(o *Options) {
		o.Delay = delay
	}
}

func WithStrategy(f sarama.BalanceStrategy) Option {
	return func(o *Options) {
		o.PartitionAssignmentStrategy = f
	}
}

func WithConsumerFunc(f ConsumerFunc) Option {
	return func(o *Options) {
		o.F = f
	}
}

func WithMessage(message Messager) Option {
	return func(o *Options) {
		o.Message = message
	}
}

func WithPartition(partition int) Option {
	return func(o *Options) {
		o.Partition = partition
	}
}

func WithGroupID(groupID string) Option {
	return func(o *Options) {
		o.GroupID = groupID
	}
}

func WithTopic(topic string) Option {
	return func(o *Options) {
		o.Topic = topic
	}
}

func WithKafkaConfig(c *sarama.Config) Option {
	return func(o *Options) {
		o.KafkaConfig = c
	}
}
