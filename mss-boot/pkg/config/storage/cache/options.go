package cache

import "time"

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2024/5/10 23:30:42
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2024/5/10 23:30:42
 */

type Option func(*Options)

type Options struct {
	QueryCacheDuration time.Duration
	QueryCacheKeys     []string
	QueryCachePrefix   string
}

func (o *Options) HasKey(key string) bool {
	if o.QueryCacheKeys == nil {
		return false
	}
	if o.QueryCacheKeys != nil && len(o.QueryCacheKeys) == 0 {
		return true
	}
	var exist bool
	for i := range o.QueryCacheKeys {
		if o.QueryCacheKeys[i] == key {
			exist = true
			break
		}
	}
	return exist
}

func DefaultOptions() Options {
	return Options{
		QueryCacheDuration: time.Hour,
		QueryCacheKeys:     []string{},
		QueryCachePrefix:   "gorm.cache:",
	}
}

// WithQueryCacheDuration 设置缓存时间
func WithQueryCacheDuration(d time.Duration) Option {
	return func(o *Options) {
		o.QueryCacheDuration = d
	}
}

// WithQueryCacheKeys 设置缓存key
func WithQueryCacheKeys(keys ...string) Option {
	return func(o *Options) {
		var all bool
		for i := range keys {
			if keys[i] == "*" {
				all = true
				break
			}
		}
		if all {
			keys = []string{}
			return
		}
		o.QueryCacheKeys = keys
	}
}

// WithQueryCachePrefix 设置缓存前缀
func WithQueryCachePrefix(prefix string) Option {
	return func(o *Options) {
		o.QueryCachePrefix = prefix
	}
}
