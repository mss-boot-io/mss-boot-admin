package storage

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2024/3/1 10:14:14
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2024/3/1 10:14:14
 */

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/redis/go-redis/v9"
)

var _redis redis.UniversalClient

// GetRedisClient 获取redis客户端
func GetRedisClient() redis.UniversalClient {
	return _redis
}

// SetRedisClient 设置redis客户端
func SetRedisClient(c redis.UniversalClient) {
	if _redis != nil && _redis != c {
		_redis.Shutdown(context.TODO())
	}
	_redis = c
}

type RedisConnectOptions struct {
	// Addr In order to be compatible with the previous configuration
	// Deprecated: Use Addrs instead.
	Addr             string        `yaml:"addr" json:"addr"`
	Addrs            []string      `yaml:"addrs"`
	ClientName       string        `yaml:"clientName"`
	DB               int           `yaml:"db"`
	Username         string        `yaml:"username"`
	Password         string        `yaml:"password"`
	SentinelUsername string        `yaml:"sentinelUsername"`
	SentinelPassword string        `yaml:"sentinelPassword"`
	MasterName       string        `yaml:"masterName"`
	Protocol         int           `yaml:"protocol"`
	MaxRetries       int           `yaml:"maxRetries"`
	MinRetryBackoff  time.Duration `yaml:"minRetryBackoff"`
	MaxRetryBackoff  time.Duration `yaml:"maxRetryBackoff"`
	DialTimeout      time.Duration `yaml:"dialTimeout"`
	ReadTimeout      time.Duration `yaml:"readTimeout"`
	WriteTimeout     time.Duration `yaml:"writeTimeout"`
	ContextTimeout   bool          `yaml:"contextTimeoutEnabled"`
	PoolFIFO         bool          `yaml:"poolFIFO"`
	PoolSize         int           `yaml:"poolSize"`
	PoolTimeout      time.Duration `yaml:"poolTimeout"`
	MinIdleConns     int           `yaml:"minIdleConns"`
	MaxIdleConns     int           `yaml:"maxIdleConns"`
	MaxActiveConns   int           `yaml:"maxActiveConns"`
	ConnMaxIdleTime  time.Duration `yaml:"connMaxIdleTime"`
	ConnMaxLifetime  time.Duration `yaml:"connMaxLifetime"`
	TLS              *TLS          `yaml:"tls" json:"tls"`
	MaxRedirects     int           `yaml:"maxRedirects"`
	ReadOnly         bool          `yaml:"readOnly"`
	RouteByLatency   bool          `yaml:"routeByLatency"`
	RouteRandomly    bool          `yaml:"routeRandomly"`
	DisableIdentity  bool          `yaml:"disableIdentity"`
	IdentitySuffix   string        `yaml:"identitySuffix"`
	UnstableResp3    bool          `yaml:"unstableResp3"`
}

type TLS struct {
	Cert string `yaml:"cert" json:"cert"`
	Key  string `yaml:"key" json:"key"`
	Ca   string `yaml:"ca" json:"ca"`
}

func (e *RedisConnectOptions) GetRedisOptions() (opt *redis.UniversalOptions, err error) {
	opt = &redis.UniversalOptions{
		Addrs:                 e.Addrs,
		ClientName:            e.ClientName,
		DB:                    e.DB,
		Username:              e.Username,
		Password:              e.Password,
		SentinelUsername:      e.SentinelUsername,
		SentinelPassword:      e.SentinelPassword,
		MasterName:            e.MasterName,
		Protocol:              e.Protocol,
		MaxRetries:            e.MaxRetries,
		MinRetryBackoff:       e.MinRetryBackoff,
		MaxRetryBackoff:       e.MaxRetryBackoff,
		DialTimeout:           e.DialTimeout,
		ReadTimeout:           e.ReadTimeout,
		WriteTimeout:          e.WriteTimeout,
		ContextTimeoutEnabled: e.ContextTimeout,
		PoolFIFO:              e.PoolFIFO,
		PoolSize:              e.PoolSize,
		PoolTimeout:           e.PoolTimeout,
		MinIdleConns:          e.MinIdleConns,
		MaxIdleConns:          e.MaxIdleConns,
		MaxActiveConns:        e.MaxActiveConns,
		ConnMaxIdleTime:       e.ConnMaxIdleTime,
		ConnMaxLifetime:       e.ConnMaxLifetime,
		MaxRedirects:          e.MaxRedirects,
		ReadOnly:              e.ReadOnly,
		RouteByLatency:        e.RouteByLatency,
		RouteRandomly:         e.RouteRandomly,
		DisableIdentity:       e.DisableIdentity,
		IdentitySuffix:        e.IdentitySuffix,
		UnstableResp3:         e.UnstableResp3,
	}
	if e.Addr != "" {
		opt.Addrs = []string{e.Addr}
	}
	opt.TLSConfig, err = getTLS(e.TLS)
	return opt, err
}

func getTLS(c *TLS) (*tls.Config, error) {
	if c == nil || (c.Cert == "" && c.Key == "" && c.Ca == "") {
		return nil, nil
	}

	tlsConfig := &tls.Config{}

	// 加载客户端证书（可选，只有开启双向认证才需要）
	if c.Cert != "" && c.Key != "" {
		cert, err := tls.LoadX509KeyPair(c.Cert, c.Key)
		if err != nil {
			slog.Error("tls.LoadX509KeyPair err", "err", err)
			return nil, err
		}
		tlsConfig.Certificates = []tls.Certificate{cert}
	}

	// 加载 CA 证书（用来验证 Redis 服务器端证书）
	if c.Ca != "" {
		caCert, err := os.ReadFile(c.Ca)
		if err != nil {
			slog.Error("os.ReadFile err", "err", err)
			return nil, err
		}

		certPool := x509.NewCertPool()
		if ok := certPool.AppendCertsFromPEM(caCert); !ok {
			slog.Error("failed to append CA cert", "ca", c.Ca)
			return nil, fmt.Errorf("failed to append CA cert: %s", c.Ca)
		}
		tlsConfig.RootCAs = certPool
	}

	return tlsConfig, nil
}
