package api

/*
 * @Author: lwnmengjing
 * @Date: 2023/1/26 11:24:55
 * @Last Modified by: lwnmengjing
 * @Last Modified time: 2023/1/26 11:24:55
 */

import (
	"fmt"

	"github.com/mss-boot-io/mss-boot/pkg/response"
	"github.com/mss-boot-io/mss-boot/virtual/model"
)

// Option set options
type Option func(*Options)

// Options options
type Options struct {
	actions       []response.Action
	search        response.Searcher
	model         model.ModelImpl
	auth          bool
	modelProvider fmt.Stringer
}

// getAction get action
func (o *Options) getAction(key string) response.Action {
	for i := range o.actions {
		if o.actions[i].String() == key {
			return o.actions[i]
		}
	}
	return nil
}

// DefaultOptions make default options
func DefaultOptions() Options {
	return Options{
		actions: make([]response.Action, 0),
	}
}

// WithAction set action
func WithAction(action response.Action) Option {
	return func(o *Options) {
		if o.actions == nil {
			o.actions = make([]response.Action, 0)
		}
		o.actions = append(o.actions, action)
	}
}

// WithSearch set search
func WithSearch(search response.Searcher) Option {
	return func(o *Options) {
		o.search = search
	}
}

// WithModel set model
func WithModel(m model.ModelImpl) Option {
	return func(o *Options) {
		o.model = m
	}
}

// WithAuth set auth
func WithAuth(auth bool) Option {
	return func(o *Options) {
		o.auth = auth
	}
}

// WithModelProvider set model provider
func WithModelProvider(provider model.ModelProvider) Option {
	return func(o *Options) {
		o.modelProvider = provider
	}
}
