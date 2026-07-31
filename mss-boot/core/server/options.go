package server

/*
 * @Author: lwnmengjing
 * @Date: 2021/6/7 5:54 下午
 * @Last Modified by: lwnmengjing
 * @Last Modified time: 2021/6/7 5:54 下午
 */

import "time"

// Option set Options
type Option func(*Options)

// Options options
type Options struct {
	gracefulShutdownTimeout time.Duration
}

func setDefaultOptions() Options {
	return Options{
		gracefulShutdownTimeout: 5 * time.Second,
	}
}
