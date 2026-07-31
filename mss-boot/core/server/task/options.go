package task

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2023/2/21 16:23:53
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2023/2/21 16:23:53
 */

import (
	"github.com/robfig/cron/v3"
)

// Option params set
type Option func(*options)

type schedule struct {
	spec    string
	job     cron.Job
	entryID cron.EntryID
}

type options struct {
	task    *cron.Cron
	storage Storage
}

// WithSchedule set schedule
func WithSchedule(key string, spec string, job cron.Job) Option {
	return func(o *options) {
		_ = o.storage.Set(key, 0, spec, job)
	}
}

// WithStorage set storage
func WithStorage(s Storage) Option {
	return func(o *options) {
		o.storage = s
	}

}

func setDefaultOption() options {
	return options{
		task: cron.New(cron.WithSeconds(), cron.WithChain()),
		storage: &defaultStorage{
			schedules: make(map[string]*schedule),
		},
	}
}
