package storage

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2024/3/1 10:17:19
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2024/3/1 10:17:19
 */

import (
	"time"

	"github.com/nsqio/go-nsq"
)

type NSQOptions struct {
	DialTimeout time.Duration `opt:"dial_timeout" default:"1s" yaml:"dialTimeout" json:"dialTimeout"`

	LookupdAddr string `opt:"-" json:"lookupdAddr" yaml:"lookupdAddr"`

	AdminAddr string `opt:"-" json:"adminAddr" yaml:"adminAddr"`

	// Deadlines for network reads and writes
	ReadTimeout  time.Duration `opt:"read_timeout" min:"100ms" max:"5m" default:"60s" yaml:"readTimeout" json:"readTimeout"`
	WriteTimeout time.Duration `opt:"write_timeout" min:"100ms" max:"5m" default:"1s" yaml:"writeTimeout" json:"writeTimeout"`

	// Addresses is the local address to use when dialing an nsqd.
	Addresses []string `opt:"addresses" yaml:"addresses" json:"addresses"`

	// Duration between polling lookupd for new producers, and fractional jitter to add to
	// the lookupd pool loop. this helps evenly distribute requests even if multiple consumers
	// restart at the same time
	//
	// NOTE: when not using nsqlookupd, LookupdPollInterval represents the duration of time between
	// reconnection attempts
	LookupdPollInterval time.Duration `opt:"lookupd_poll_interval" min:"10ms" max:"5m" default:"60s" yaml:"lookupdPollInterval" json:"lookupdPollInterval"`
	LookupdPollJitter   float64       `opt:"lookupd_poll_jitter" min:"0" max:"1" default:"0.3" yaml:"lookupdPollJitter" json:"lookupdPollJitter"`

	// Maximum duration when REQueueing (for doubling of deferred requeue)
	MaxRequeueDelay     time.Duration `opt:"max_requeue_delay" min:"0" max:"60m" default:"15m" yaml:"maxRequeueDelay" json:"maxRequeueDelay"`
	DefaultRequeueDelay time.Duration `opt:"default_requeue_delay" min:"0" max:"60m" default:"90s" yaml:"defaultRequeueDelay" json:"defaultRequeueDelay"`

	// Maximum amount of time to backoff when processing fails 0 == no backoff
	MaxBackoffDuration time.Duration `opt:"max_backoff_duration" min:"0" max:"60m" default:"2m" yaml:"maxBackoffDuration" json:"maxBackoffDuration"`
	// Unit of time for calculating consumer backoff
	BackoffMultiplier time.Duration `opt:"backoff_multiplier" min:"0" max:"60m" default:"1s" yaml:"backoffMultiplier" json:"backoffMultiplier"`

	// Maximum number of times this consumer will attempt to process a message before giving up
	MaxAttempts uint16 `opt:"max_attempts" min:"0" max:"65535" default:"5" yaml:"maxAttempts" json:"maxAttempts"`

	// Duration to wait for a message from an nsqd when in a state where RDY
	// counts are re-distributed (e.g. max_in_flight < num_producers)
	LowRdyIdleTimeout time.Duration `opt:"low_rdy_idle_timeout" min:"1s" max:"5m" default:"10s" yaml:"lowRdyIdleTimeout" json:"lowRdyIdleTimeout"`
	// Duration to wait until redistributing RDY for an nsqd regardless of LowRdyIdleTimeout
	LowRdyTimeout time.Duration `opt:"low_rdy_timeout" min:"1s" max:"5m" default:"30s" yaml:"lowRdyTimeout" json:"lowRdyTimeout"`
	// Duration between redistributing max-in-flight to connections
	RDYRedistributeInterval time.Duration `opt:"rdy_redistribute_interval" min:"1ms" max:"5s" default:"5s" yaml:"rdyRedistributeInterval" json:"rdyRedistributeInterval"`

	// Identifiers sent to nsqd representing this client
	// UserAgent is in the spirit of HTTP (default: "<client_library_name>/<version>")
	ClientID  string `opt:"client_id" yaml:"clientID" json:"clientID"` // (defaults: short hostname)
	Hostname  string `opt:"hostname" yaml:"hostname" json:"hostname"`
	UserAgent string `opt:"user_agent" yaml:"userAgent" json:"userAgent"`

	// Duration of time between heartbeats. This must be less than ReadTimeout
	HeartbeatInterval time.Duration `opt:"heartbeat_interval" default:"30s" yaml:"heartbeatInterval" json:"heartbeatInterval"`
	// Integer percentage to sample the channel (requires nsqd 0.2.25+)
	SampleRate int32 `opt:"sample_rate" min:"0" max:"99" yaml:"sampleRate" json:"sampleRate"`

	Tls *TLS `yaml:"tls" json:"tls"`

	// Compression Settings
	Deflate      bool `opt:"deflate" yaml:"deflate" json:"deflate"`
	DeflateLevel int  `opt:"deflate_level" min:"1" max:"9" default:"6" yaml:"deflateLevel" json:"deflateLevel"`
	Snappy       bool `opt:"snappy" yaml:"snappy" json:"snappy"`

	// Size of the buffer (in bytes) used by nsqd for buffering writes to this connection
	OutputBufferSize int64 `opt:"output_buffer_size" default:"16384" yaml:"outputBufferSize" json:"outputBufferSize"`
	// Timeout used by nsqd before flushing buffered writes (set to 0 to disable).
	//
	// WARNING: configuring clients with an extremely low
	// (< 25ms) output_buffer_timeout has a significant effect
	// on nsqd CPU usage (particularly with > 50 clients connected).
	OutputBufferTimeout time.Duration `opt:"output_buffer_timeout" default:"250ms" yaml:"outputBufferTimeout" json:"outputBufferTimeout"`

	// Maximum number of messages to allow in flight (concurrency knob)
	MaxInFlight int `opt:"max_in_flight" min:"0" default:"1" yaml:"maxInFlight" json:"maxInFlight"`

	// The server-side message timeout for messages delivered to this client
	MsgTimeout time.Duration `opt:"msg_timeout" min:"0" yaml:"msgTimeout" json:"msgTimeout"`

	// secret for nsqd authentication (requires nsqd 0.2.29+)
	AuthSecret string `opt:"auth_secret" yaml:"authSecret" json:"authSecret"`
}

func (e NSQOptions) GetNSQOptions() (*nsq.Config, error) {
	cfg := nsq.NewConfig()
	var err error
	cfg.TlsConfig, err = getTLS(e.Tls)
	if err != nil {
		return nil, err
	}
	if e.DialTimeout > 0 {
		cfg.DialTimeout = e.DialTimeout * time.Second
	}
	if e.ReadTimeout > 0 {
		cfg.ReadTimeout = e.ReadTimeout * time.Second
	}
	if e.WriteTimeout > 0 {
		cfg.WriteTimeout = e.WriteTimeout * time.Second
	}
	if e.LookupdPollInterval > 0 {
		cfg.LookupdPollInterval = e.LookupdPollInterval * time.Second
	}
	if e.MaxRequeueDelay > 0 {
		cfg.MaxRequeueDelay = e.MaxRequeueDelay * time.Second
	}
	if e.DefaultRequeueDelay > 0 {
		cfg.DefaultRequeueDelay = e.DefaultRequeueDelay * time.Second
	}
	if e.MaxBackoffDuration > 0 {
		cfg.MaxBackoffDuration = e.MaxBackoffDuration * time.Millisecond
	}
	if e.BackoffMultiplier > 0 {
		cfg.BackoffMultiplier = e.BackoffMultiplier * time.Second
	}
	if e.LowRdyIdleTimeout > 0 {
		cfg.LowRdyIdleTimeout = e.LowRdyIdleTimeout * time.Second
	}
	if e.LowRdyTimeout > 0 {
		cfg.LowRdyTimeout = e.LowRdyTimeout * time.Second
	}
	if e.RDYRedistributeInterval > 0 {
		cfg.RDYRedistributeInterval = e.RDYRedistributeInterval * time.Second
	}
	if e.HeartbeatInterval > 0 {
		cfg.HeartbeatInterval = e.HeartbeatInterval * time.Second
	}
	if e.OutputBufferTimeout > 0 {
		cfg.OutputBufferTimeout = e.OutputBufferTimeout * time.Second
	}
	if e.MsgTimeout > 0 {
		cfg.MsgTimeout = e.MsgTimeout * time.Second
	}
	if e.LookupdPollJitter > 0 {
		cfg.LookupdPollJitter = e.LookupdPollJitter
	}
	cfg.MaxAttempts = e.MaxAttempts
	if e.ClientID != "" {
		cfg.ClientID = e.ClientID
	}
	if e.Hostname != "" {
		cfg.Hostname = e.Hostname
	}
	if e.UserAgent != "" {
		cfg.UserAgent = e.UserAgent
	}
	if e.SampleRate > 0 {
		cfg.SampleRate = e.SampleRate
	}
	cfg.Deflate = e.Deflate
	if e.DeflateLevel >= 6 && e.DeflateLevel <= 9 {
		cfg.DeflateLevel = e.DeflateLevel
	}
	cfg.Snappy = e.Snappy
	if e.OutputBufferSize > 0 {
		cfg.OutputBufferSize = e.OutputBufferSize
	}
	if e.MaxInFlight > 0 {
		cfg.MaxInFlight = e.MaxInFlight
	}
	if e.AuthSecret != "" {
		cfg.AuthSecret = e.AuthSecret
	}
	return cfg, nil
}
