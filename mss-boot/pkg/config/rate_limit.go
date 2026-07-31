package config

import (
	"context"

	"golang.org/x/time/rate"
)

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2023/11/4 10:41:05
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2023/11/4 10:41:05
 */

type Limiter interface {
	Allow() bool
	Wait(context.Context) error
}

type LimiterName string

const (
	SingleLimiter LimiterName = "single"
)

type RateLimit struct {
	// 限流器类型
	Name LimiterName `json:"name" yaml:"name"`
	// 限流器配置
	Rate float64 `json:"rate" yaml:"rate"`
	// 限流器最大存储令牌数
	Bursts int `json:"bursts" yaml:"bursts"`
}

func (e *RateLimit) String() string {
	return string(e.Name)
}

func (e *RateLimit) Init() Limiter {
	switch e.Name {
	case SingleLimiter:
		return rate.NewLimiter(rate.Limit(e.Rate), e.Bursts)
	default:
		return nil
	}
}
