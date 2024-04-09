package queue

import "github.com/mss-boot-io/mss-boot-admin/storage"

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2024/4/9 11:28:26
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2024/4/9 11:28:26
 */

type Options struct {
	name      string
	channel   string
	f         storage.ConsumerFunc
	message   storage.Messager
	partition int
}

func DefaultOptions() *Options {
	return &Options{
		partition: -1,
	}
}

type Option func(*Options)

func WithName(name string) Option {
	return func(o *Options) {
		o.name = name
	}
}

func WithChannel(channel string) Option {
	return func(o *Options) {
		o.channel = channel
	}
}

func WithConsumerFunc(f storage.ConsumerFunc) Option {
	return func(o *Options) {
		o.f = f
	}
}

func WithMessage(message storage.Messager) Option {
	return func(o *Options) {
		o.message = message
	}
}

func WithPartition(partition int) Option {
	return func(o *Options) {
		o.partition = partition
	}
}
