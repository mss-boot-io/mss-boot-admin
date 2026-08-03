---
title: 通用模块
nav:
  order: 2
  title: "coding"
description: 服务开发文档
keywords: [coding]
---
## 首次贡献

- [首次贡献指南](/coding/first-contribution) - 30 分钟内完成一个小而有价值的首个 PR

## migrate
> migrate模块用于数据库迁移，目前支持mysql和postgres等数据库

## config
> config模块用于配置数据的读取
- [x]  支持`yaml`和`json`格式的配置文件
- [x]  支持配置文件的热更新(`file`源自带，其他源需要实现接口)
- [x]  支持多种provider，包括`file`、`embed`、`gorm`、`mongodb`、`s3协议`等
- [x]  完全开放的接口，可以自定义provider
- [x]  对custom config优化，集成简单，使用方便
### grpc服务配置
> 通过config模块，可以很方便的配置grpc服务

```go
package config

// GRPC grpc服务公共配置(选用)
type GRPC struct {
  Addr     string `yaml:"addr" json:"addr"`         // default:  :9090 grpc服务监听地址
  CertFile string `yaml:"certFile" json:"certFile"` // default: "" 证书文件
  KeyFile  string `yaml:"keyFile" json:"keyFile"`   // default: "" 私钥文件
  Timeout  int    `yaml:"timeout" json:"timeout"`   // default: 10 服务超时时间
}
```

### http服务配置
> 通过config模块，可以很方便的配置http服务
- [x] 支持健康检查接口
- [x] 支持就绪检查接口
- [x] 支持pprof接口
- [x] 支持metrics接口
- [x] 支持自定义路由

```go
package config

// Listen tcp listener config
type Listen struct {
	Name     string `yaml:"name" json:"name"`         // default: http http服务名称
	Addr     string `yaml:"addr" json:"addr"`         // default: :8080 http服务监听地址
	CertFile string `yaml:"certFile" json:"certFile"` // default: "" 证书文件
	KeyFile  string `yaml:"keyFile" json:"keyFile"`   // default: "" 私钥文件
	Timeout  int    `yaml:"timeout" json:"timeout"`   // default: 10s
	Metrics  bool   `yaml:"metrics" json:"metrics"`   // default: false 是否开启metrics
	Healthz  bool   `yaml:"healthz" json:"healthz"`   // default: false 是否开启健康检查
	Readyz   bool   `yaml:"readyz" json:"readyz"`     // default: false 是否开启就绪检查
	Pprof    bool   `yaml:"pprof" json:"pprof"`       // default: false 是否开启pprof
}
```

### logger配置
> 通过config模块，可以很方便的配置全局logger, 使用slog包，故go1.21以上版本支持, 覆盖了http、grpc、gorm的日志

```go
package config

import (
	"log/slog"
)

// Logger logger配置
type Logger struct {
	Path      string     `yaml:"path" json:"path"`            // default: "" 日志文件路径, stdout为file时有效
	Level     slog.Level `yaml:"level" json:"level"`          // default: info 日志级别
	Stdout    string     `yaml:"stdout" json:"stdout"`        // default: stdout 日志输出方式, stdout/file
	AddSource bool       `yaml:"addSource" json:"addSource"`  // default: false 是否添加日志输出位置
	Cap       uint       `yaml:"cap" json:"cap"`              // default: 1000000 日志文件最大容量, stdout为file时有效
	Json      bool       `yaml:"json" json:"json"`            // default: false 是否以json格式输出
}
```

### storage配置
> 通过config模块，可以很方便的配置对象存储provider，目前借助s3协议
> 
支持如下对象存储提供商:
- aws s3
- 阿里云oss
- 天翼云oos
- 七牛云kodo
- 腾讯云cos
- 华为云obs
- 百度云bos
- 谷歌云gcs
- 金山云ks3
- minio
- 
```go
package config

// ProviderType storage provider type
type ProviderType string

const (
  // S3 aws s3
  S3 ProviderType = "s3"
  // OSS aliyun oss
  OSS ProviderType = "oss"
  // OOS ctyun oos
  OOS ProviderType = "oos"
  // KODO qiniu kodo
  KODO ProviderType = "kodo"
  // COS tencent cos
  COS ProviderType = "cos"
  // OBS huawei obs
  OBS ProviderType = "obs"
  // BOS baidu bos
  BOS ProviderType = "bos"
  // GCS google gcs
  GCS ProviderType = "gcs"
  // KS3 kingsoft ks3
  KS3 ProviderType = "ks3"
  // MINIO minio storage
  MINIO ProviderType = "minio"
)

// Storage storage
type Storage struct {
	Type            ProviderType `yaml:"type"`            // default: s3 provider类型
	SigningMethod   string       `yaml:"signingMethod"`   // default: "" 签名方法
	Region          string       `yaml:"region"`          // default: "" 区域
	Bucket          string       `yaml:"bucket"`          // default: "" 存储桶名称
	Endpoint        string       `yaml:"endpoint"`        // default: "" 存储桶域名
	AccessKeyID     string       `yaml:"accessKeyID"`     // default: "" access key id
	SecretAccessKey string       `yaml:"secretAccessKey"` // default: "" secret access key
}
```
