---
title: 服务配置
order: 12
nav:
  title: 指南
  order: 1
keywords: [config]
---
## 文件说明
配置文件分为两种`全局`和`环境`配置，全局配置文件名默认为application.yml/application.yaml。
环境配置文件名默认为application-{profile}.yml/application-{profile}.yaml，其中{profile}为环境名称，如dev、test、prod等。
例如, 本地环境开发配置文件如下:
```shell
### 默认stage为local
├── config
│   ├── application.yml
│   ├── application-local.yml
```
prod环境配置文件如下:
```shell
### 配置环境变量
export stage=prod

├── config
│   ├── application.yml
│   ├── application-prod.yml
```

## 加载顺序
配置文件加载顺序如下:
1. 加载全局配置文件
2. 加载环境配置文件, 存在的配置项会覆盖全局配置

## 配置源
目前支持的配置源
- 本地文件
- 文件系统(embed.FS等)
- gorm
- mongodb
- s3

## 配置项
> 所有的配置项都在`mss-boot`的`pkg/config`包下，使用请自取！
### grpc
> grpc服务配置项
```yaml
addr: ':9090' # grpc服务地址
certFile: '' # 证书文件
keyFile: '' # 私钥文件
timeout: 10s # 超时时间
```
### http
> http服务配置项
```yaml
name: 'service-http' # 服务名称
addr: ':8080' # http服务地址 
certFile: '' # 证书文件
keyFile: '' # 私钥文件
timeout: 10s # 超时时间
metrics: true # 是否开启metrics
healthz: true # 是否开启健康检查
readyz: true # 是否开启就绪检查
pprof: true # 是否开启pprof
```
### logger
> 日志配置项
```yaml
level: 'info' # 日志级别,支持slog的所有等级
stdout: "file" # 日志输出方式,支持file、stdout、stderr
addSource: true # 是否添加日志源
cap: 100 # 日志缓冲区大小, 只有stdout为file时才有效
json: true # 是否开启json格式
```
### storage
> 对象存储配置项 <br/>
> 目前支持`s3`、`oss`、`oos`、`kodo`、`cos`、`obs`、`bos`、`gcs`、`ks3`
```yaml
type: 's3' # 对象存储类型
signingMethod: 'v4' # 签名方法
region: ap-northeast-1 # 区域
bucket: 'bucket' # 桶名称
accessKeyID: '' # 访问密钥ID
secretAccessKey: '' # 访问密钥
```
### oauth2
> oauth2配置项
```yaml
issuer: '' # 签发者
clientID: '' # 客户端ID
clientSecret: '' # 客户端密钥
scopes:       # 作用域
  - groups
  - email
  - openid
redirectURL: '' # 回调地址
```
### tls
> 证书配置项
```yaml
cert: '' # 证书文件
key: '' # 私钥文件
ca: '' # ca证书文件
```

## 配置文件示例
application.yml
```yaml
grpc:
  addr: ':9090'
  timeout: 10s
http:
  name: 'service-http'
  addr: ':8080'
  timeout: 10s
  metrics: true
  healthz: true
  readyz: true
  pprof: true
logger:
  level: 'error'
  json: true
```
application-local.yml
```yaml
grpc:
  addr: ':19090'
http:
  addr: ':18080'
  metrics: false
  healthz: false
  readyz: false
  pprof: false
logger:
  level: info
  stdout: "file"
  addSource: true
  json: false
```
