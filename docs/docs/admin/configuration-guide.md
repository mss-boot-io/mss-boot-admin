---
title: Admin 配置指南
order: 11
nav:
  order: 1
  title: admin
description: mss-boot-admin 配置加载、环境模板、浏览器会话、生产部署与验证合同
keywords: [admin configuration environment session pat production]
---

# Admin 配置指南

本指南描述当前 `admin/config.Config` 与 `mss-boot/pkg/config` 实际编译的配置合同。
配置结构以 Go 类型为准，仓库中的 `admin/config/application.yml` 是参考 Admin 的开发基线，
`templates/application/config/README.md` 则要求 Thin Host 把环境配置和生产秘密保留在源码之外。

## 配置加载合同

### 基础文件与 Stage 覆盖

默认本地配置源按下面的顺序加载：

1. `config/application.yml`；
2. 如果存在，再加载 `config/application-<STAGE>.yml` 并覆盖已提供的字段。

`STAGE` 未设置时默认为 `local`，因此参考 Admin 会继续加载
`config/application-local.yml`。配置路径相对进程工作目录；从源码运行时应先进入
`admin/`：

```shell
cd admin
STAGE=local go run . server
```

Stage 文件是可选覆盖，不是第二份完整配置。生产部署通常保留一份受控的
`config/application-prod.yml`，并以 `STAGE=prod` 启动。修改配置后应重启并重新验证依赖；
不要把文件监听等同于数据库、缓存、队列等运行资源都会安全热重建。

### 环境变量不是全局覆盖层

系统没有“环境变量自动覆盖同名 YAML 字段”的通用优先级。只有两类环境读取：

- 配置文件显式使用 `{{ .Env.NAME }}` 模板；
- 启动器为特定引导用途直接读取的变量。

当前常用的直接读取项如下：

| 变量 | 实际用途 |
| --- | --- |
| `STAGE`（兼容小写 `stage`） | 选择 `application-<STAGE>.yml`；默认 `local` |
| `CONFIG_PROVIDER` | 选择配置源；`server` 在 `local`/`dev` Stage 强制使用本地文件源 |
| `DB_DRIVER`、`DB_DSN` | 仅引导 `CONFIG_PROVIDER=gorm` 的配置数据库，或覆盖对应 CLI 引导参数；不会在本地文件源下自动覆盖 `database.driver` / `database.source` |

`AUTH_KEY`、`APP_MODE`、`REDIS_ADDR` 不是通用直读接口。仅把这些名字写进进程环境，
不会修改 `auth.key`、`application.mode` 或 `cache.redis.addr`。

### 正确的环境模板

需要从部署环境注入的字段必须在 YAML 中显式引用：

```yaml
database:
  driver: mysql
  source: '{{ .Env.MSS_ADMIN_DATABASE_DSN }}'

auth:
  key: '{{ .Env.MSS_ADMIN_AUTH_SIGNING_KEY }}'

cache:
  redis:
    addr: '{{ .Env.MSS_ADMIN_CACHE_REDIS_ENDPOINT }}'
    password: '{{ .Env.MSS_ADMIN_CACHE_REDIS_PASSWORD }}'
```

模板在 YAML 解码前执行。缺失变量会得到空字符串，不会自动使用示例密码，也不会读取
`.env` 文件。Shell、systemd、容器编排或秘密管理平台必须先把值放入进程环境。建议为模板
变量使用应用命名空间前缀；这些名字之所以生效，是因为 YAML 明确引用了它们，而不是因为
框架内置了这些环境变量。

不要使用 `${NAME}`：当前配置加载器不会展开这种写法。

## 本地开发与 API 注册表

参考 Admin 的基础配置使用 SQLite，`application-local.yml` 把浏览器 Origin 对齐到前端
开发端口。数据库迁移完成后，首次启动或路由发生变化时必须同步一次 API 注册表：

```shell
cd admin

# 与常驻服务使用完全相同的 Stage、配置源和数据库
STAGE=local go run . server -a

# 同步成功退出后再启动常驻服务
STAGE=local go run . server
```

`server -a` 会初始化配置和数据库、挂载实际路由、写入 API 注册表，然后退出。只执行迁移
不会生成这份注册表；遗漏此步骤会使“权限管理 → 菜单管理 → 绑定 API”没有候选项。

生产环境同样要在迁移之后、常驻服务启动之前执行一次 `server -a`。它必须使用与常驻服务
相同的二进制版本、`STAGE`、配置源、工作目录和数据库；否则写入的是另一套数据库或另一组
路由。

## 数据库

### SQLite 开发配置

```yaml
database:
  driver: sqlite
  source: mss-boot-admin-local.db
  config:
    disableForeignKeyConstraintWhenMigrating: true
```

### MySQL / PostgreSQL 生产配置

把 DSN 放入部署秘密，而不是提交到仓库：

```yaml
database:
  driver: mysql
  source: '{{ .Env.MSS_ADMIN_DATABASE_DSN }}'
  maxOpenConns: 100
  maxIdleConns: 20
  connMaxIdleTime: 300
  connMaxLifeTime: 3600
  config:
    prepareStmt: true
    disableForeignKeyConstraintWhenMigrating: false
```

PostgreSQL 只需把 `driver` 改为 `postgres`，并注入对应格式的 DSN。连接池四个字段位于
`database` 下；`connMaxIdleTime` 和 `connMaxLifeTime` 是秒数整数。`database.config`
只承载受支持的 GORM 开关，不要把连接池字段或 `timeout`、`name` 放进去。

## 认证：浏览器会话与 API PAT

```yaml
auth:
  realm: mss-boot-admin
  key: '{{ .Env.MSS_ADMIN_AUTH_SIGNING_KEY }}'
  identityKey: mss-boot-admin-identity
  timeout: 12h
  maxRefresh: 2160h
  browserSession:
    secure: true
    sameSite: lax
    webSocketTicketTTL: 30s
```

`auth.key` 是服务端认证签名材料，不是前端可以读取或保存的“API JWT 密钥”。交互式 V6
浏览器登录把会话凭据只放在 host-only、HttpOnly 的 `mss_admin_session` Cookie 中；登录和
刷新响应不返回 Admin JWT。可读的 `mss_csrf` Cookie 只用于为不安全方法生成
`X-CSRF-Token`，不能代替会话凭据。

非浏览器自动化使用用户自行创建、只展示一次的 PAT：

```shell
curl https://admin.example.com/admin/api/user/userInfo \
  -H 'Authorization: Bearer <personal-access-token>'
```

PAT 仍受关联用户当前状态、角色和后端 RBAC 约束，不能作为浏览器登录的回退。轮换
`auth.key` 会使现有浏览器会话和 PAT 签名失效，应按受控发布处理。完整合同见
[V6 browser sessions, PAT, and OAuth2](/admin/token-oauth2-guide)。

生产模式在启动时强制检查：

- `auth.key` 不能是开发默认值，去除首尾空白后至少 32 字节，并应来自高熵秘密；
- `auth.browserSession.secure` 必须为 `true`；
- `application.origin` 与每个 CORS Origin 必须是精确的 HTTPS Origin，不能使用通配符；
- `sameSite` 只能为 `lax` 或 `strict`；
- `webSocketTicketTTL` 必须在 5 秒到 2 分钟之间；
- 跨 Origin 浏览器请求的 `cors.allowHeaders` 必须包含 `X-CSRF-Token`。

GitHub/Lark OAuth 的 BrowserSession Client 配置属于系统 AppConfig，并通过权限受控的
系统配置界面管理，不是 `security:` YAML 顶层块。OAuth 密钥写入后不回显，字段与流程以
[认证指南](/admin/token-oauth2-guide)为准。

## CORS、代理与 TLS

生产浏览器配置示例：

```yaml
application:
  mode: prod
  origin: https://admin.example.com
  # 为空表示不信任转发客户端 IP；使用反向代理时只列出自己控制的精确 IP/CIDR。
  trustedProxies:
    - 10.0.0.10/32

cors:
  allowOrigins:
    - https://admin.example.com
  allowMethods: [GET, POST, PUT, PATCH, DELETE, OPTIONS]
  allowHeaders: [Authorization, Content-Type, If-Match, X-CSRF-Token]
  exposeHeaders: [ETag]
  maxAge: 12h
```

通常由反向代理终止 TLS，并把同一站点的 `/admin/api` 转发到后端。如果由 Admin 监听器
直接终止 TLS，证书字段属于 `server`，不存在顶层 `ssl.enabled`：

```yaml
server:
  addr: 0.0.0.0:8443
  certFile: /run/secrets/admin-tls.crt
  keyFile: /run/secrets/admin-tls.key
```

## 缓存、队列与锁

Redis 配置必须显式写在对应资源下；没有 `REDIS_ADDR` 自动覆盖：

```yaml
cache:
  queryCache: false
  queryCacheDuration: 1h
  queryCacheKeys: []
  redis:
    addr: '{{ .Env.MSS_ADMIN_CACHE_REDIS_ENDPOINT }}'
    password: '{{ .Env.MSS_ADMIN_CACHE_REDIS_PASSWORD }}'
    db: 0
    poolSize: 20
    minIdleConns: 5
    dialTimeout: 5s
    readTimeout: 3s
    writeTimeout: 3s
```

Redis 是可选派生缓存，不是配置事实源。通用查询缓存默认应保持关闭；只有在明确数据敏感度、
失效机制和故障降级后，才为 `queryCacheKeys` 配置具体表名。详见
[配置缓存一致性](/admin/config-cache-consistency)。

当前 `queue` 支持 `redis`、`nsq`、`kafka` 和 `memory`，兼容选择顺序为
Redis → NSQ → Kafka → Memory。不要同时配置多种后再依赖隐式优先级。最小本地配置为：

```yaml
queue:
  memory:
    poolSize: 10
```

NSQ 的地址字段是 `addresses` 与 `lookupdAddr`，Kafka 的 Broker 字段是 `brokers`。
当前配置结构没有 `queue.pulsar`。分布式锁使用独立的 `locker.redis` 配置；不要假设
`cache.redis` 的字段会自动复制到其他资源。

## 通知与任务

```yaml
notification:
  email:
    enabled: true
    host: smtp.example.com
    port: 587
    username: noreply@example.com
    password: '{{ .Env.MSS_ADMIN_SMTP_PASSWORD }}'
    from: noreply@example.com
  dingtalk:
    enabled: false
    webhook: ''
    secret: ''
  wechat:
    enabled: false
    webhook: ''

task:
  enable: true
  spec: '0 */1 * * * *'
```

`notification.email` 只有 `enabled`、`host`、`port`、`username`、`password`、`from`
字段；当前结构没有 `useTLS` / `useSSL`。`task` 只有 `enable` 和 Cron `spec`，没有
Kubernetes provider、namespace 或 image 字段。容器镜像应在 Compose/Kubernetes 部署清单
中固定到与 Admin Distribution 一致的 `v1.3.1`，生产再记录验证过的 digest，不要使用
`latest`。

## 日志与可观测性

```yaml
server:
  metrics: true
  healthz: true
  readyz: true
  pprof: false

monitor:
  sampleInterval: 5s
  sampleTimeout: 3s
  historySize: 120

logger:
  stdout: default
  level: info
  json: true
  addSource: false

pyroscope:
  enabled: false
  applicationName: mss-boot-admin
  serverAddress: http://pyroscope:4040
```

`logger.stdout: file` 时还可以设置 `logger.path` 和按容量控制的 `logger.cap`。当前 Logger
没有 `maxSize`、`maxBackups`、`maxAge`、`compress`，Loki writer 也未启用；日志轮转应由
部署平台处理。Pyroscope 支持 `uploadRate`、认证字段、Tags 与 ProfileTypes，不存在
`sampleRate` 配置。

启用对应监听项后可检查：

```shell
curl --fail http://127.0.0.1:8080/healthz
curl --fail http://127.0.0.1:8080/readyz
curl --fail http://127.0.0.1:8080/metrics
```

生产环境不要直接暴露 PProf、metrics 或内部健康端点；通过受控网络或监控采集访问。

## 对象存储边界

启动配置 `storage` 是严格二选一结构：只能配置 `local` 或 `s3`。Local root 必须是绝对路径；
S3-compatible 使用 `s3` 分支并显式设置 endpoint、region、bucket、路径风格、TLS 和凭据来源。
静态凭据使用 `env://NAME` SecretRef，不把值写入 YAML：

```yaml
storage:
  s3:
    endpoint: https://s3.example.com
    region: region-1
    bucket: admin-objects
    usePathStyle: true
    credentials:
      static:
        accessKeyRef: env://MSS_ADMIN_S3_ACCESS_KEY
        secretKeyRef: env://MSS_ADMIN_S3_SECRET_KEY
```

`storage:maxSize` 与 `storage:allowedTypes` 是数据库 AppConfig 中唯一的上传 admission 项，
不是上面启动 profile 的子字段。当前 Local/S3-compatible 上传仍是 Legacy / Blocked；生产
入口必须按[生产部署标准化](/admin/production-standardization)阻断上传路由，不能仅凭客户端
已构造、目录已挂载或健康检查通过就开放对象交付。

## 前端配置

V6 前端运行时把 API 基址固定为同源 `/admin/api`，请求携带 HttpOnly Cookie，并为不安全
方法复制签名 CSRF Cookie。生产构建不会读取 `API_BASE_URL`；应由反向代理提供同源路由。

开发模式下 `defineBusinessAdmin` 自动代理 `/admin/`，目标默认为
`http://127.0.0.1:8080`。需要切换本地后端时使用其真实开发变量：

```shell
cd web/antd-v6
MSS_ADMIN_API_TARGET=http://127.0.0.1:18080 corepack pnpm@10.34.5 run dev
```

不要在 `.env.production` 中设置虚构的 `API_BASE_URL`，也不要在浏览器存储会话 Token 或
PAT。

## 生产 Stage 示例

下面是一份 `config/application-prod.yml` 覆盖示例。所有秘密都由模板引用，示例本身不含
固定密码：

```yaml
server:
  addr: 0.0.0.0:8080
  metrics: true
  healthz: true
  readyz: true
  pprof: false

application:
  mode: prod
  origin: https://admin.example.com
  trustedProxies:
    - 10.0.0.10/32

cors:
  allowOrigins:
    - https://admin.example.com
  allowMethods: [GET, POST, PUT, PATCH, DELETE, OPTIONS]
  allowHeaders: [Authorization, Content-Type, If-Match, X-CSRF-Token]
  exposeHeaders: [ETag]
  maxAge: 12h

logger:
  stdout: default
  level: info
  json: true
  addSource: false

database:
  driver: mysql
  source: '{{ .Env.MSS_ADMIN_DATABASE_DSN }}'
  maxOpenConns: 100
  maxIdleConns: 20
  connMaxIdleTime: 300
  connMaxLifeTime: 3600
  config:
    prepareStmt: true

auth:
  realm: mss-boot-admin
  key: '{{ .Env.MSS_ADMIN_AUTH_SIGNING_KEY }}'
  identityKey: mss-boot-admin-identity
  timeout: 12h
  maxRefresh: 2160h
  browserSession:
    secure: true
    sameSite: lax
    webSocketTicketTTL: 30s

cache:
  queryCache: false
  queryCacheDuration: 1h
  queryCacheKeys: []
  redis:
    addr: '{{ .Env.MSS_ADMIN_CACHE_REDIS_ENDPOINT }}'
    password: '{{ .Env.MSS_ADMIN_CACHE_REDIS_PASSWORD }}'
    db: 0
    poolSize: 20

queue:
  memory:
    poolSize: 10

notification:
  email:
    enabled: false
    host: ''
    port: 587
    username: ''
    password: ''
    from: ''
  dingtalk:
    enabled: false
    webhook: ''
    secret: ''
  wechat:
    enabled: false
    webhook: ''

task:
  enable: true
  spec: '0 */1 * * * *'
```

### systemd EnvironmentFile

systemd 的 `EnvironmentFile=` 不执行 shell，因此不能把随机生成命令写在等号右侧。先在受控
终端生成随机材料并存入秘密管理系统，再由部署平台把最终字面值写入只允许服务用户读取的
文件。下面故意保留空值，使未注入秘密的部署失败而不是带示例密码启动：

```ini
# /etc/mss-boot-admin/admin.env
# 由秘密管理/部署系统写入等号右侧的最终字面值；不要原样部署空值。
STAGE=prod
MSS_ADMIN_AUTH_SIGNING_KEY=
MSS_ADMIN_DATABASE_DSN=
MSS_ADMIN_CACHE_REDIS_ENDPOINT=
MSS_ADMIN_CACHE_REDIS_PASSWORD=
MSS_ADMIN_SMTP_PASSWORD=
```

认证签名材料应一次生成并持久保存，例如用 `openssl rand -base64 48` 生成后写入秘密管理系统；
不要在每次服务启动时重新生成，否则会使所有既有浏览器会话和 PAT 失效。保护该文件为
`0600`，不要提交到 Git。

### systemd 服务与 API 同步

常驻服务：

```ini
# /etc/systemd/system/mss-boot-admin.service
[Unit]
Description=mss-boot-admin
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=mss
Group=mss
WorkingDirectory=/opt/mss-boot-admin
EnvironmentFile=/etc/mss-boot-admin/admin.env
ExecStart=/opt/mss-boot-admin/mss-boot-admin server
Restart=on-failure
RestartSec=5

[Install]
WantedBy=multi-user.target
```

配套的一次性 API 同步服务复用完全相同的工作目录和 EnvironmentFile：

```ini
# /etc/systemd/system/mss-boot-admin-api-sync.service
[Unit]
Description=Synchronize mss-boot-admin API registry
After=network-online.target
Wants=network-online.target

[Service]
Type=oneshot
User=mss
Group=mss
WorkingDirectory=/opt/mss-boot-admin
EnvironmentFile=/etc/mss-boot-admin/admin.env
ExecStart=/opt/mss-boot-admin/mss-boot-admin server -a
```

每次首次部署或路由变更时，按“迁移 → API 同步 → 常驻服务”的顺序执行：

```shell
sudo systemctl daemon-reload
sudo systemctl start mss-boot-admin-api-sync.service
sudo systemctl restart mss-boot-admin.service
sudo systemctl status --no-pager mss-boot-admin.service
```

## 验证与故障定位

当前 CLI 没有 `server --validate`，`-c`/`--config-provider` 的参数是配置源名称，不是配置
文件路径。不要使用 `server -c config/application.yml --validate`。

部署前在隔离或预生产数据库完成迁移和 `server -a`，再启动常驻服务并检查 readiness。生产
`server -a` 会写 API 注册表，不是只读配置检查。

检查环境是否已注入时只判断存在性，不要把 DSN、认证材料或密码打印到终端和日志：

```shell
test -n "${MSS_ADMIN_AUTH_SIGNING_KEY:-}" && echo 'auth signing key: set' || echo 'auth signing key: missing'
test -n "${MSS_ADMIN_DATABASE_DSN:-}" && echo 'database DSN: set' || echo 'database DSN: missing'
```

常见问题：

- 启动仍是开发模式：检查 `STAGE=prod`、工作目录以及
  `config/application-prod.yml` 是否实际存在；`APP_MODE=prod` 不会覆盖 YAML。
- 数据库连接到了意外实例：检查 `database.source` 的 `{{ .Env.* }}` 名称与服务进程实际
  环境；不要把 `DB_DSN` 的配置源引导语义误当成应用数据库覆盖。
- “绑定 API”为空：使用与常驻服务相同的版本、Stage、配置源和数据库重新执行
  `server -a`，确认命令成功退出。
- 浏览器登录或写请求 403：核对精确 HTTPS Origin、Secure Cookie、反向代理协议以及
  `X-CSRF-Token` CORS header；不要改成浏览器 Bearer Token 绕过。
- Redis 配置未生效：确认 YAML 明确引用了部署变量，并检查字段位于目标资源的
  `redis` 节点；设置 `REDIS_ADDR` 本身不会覆盖配置。
- YAML 中的字段看似生效但运行时无变化：对照 `admin/config.Config` 及其嵌入的 Framework
  配置类型；未知字段可能被 YAML 解码忽略，不要从旧示例推断字段名。

## 下一步

- [本地调试](/admin/local-debug)
- [容器化与生产部署](/admin/docker)
- [安全基线](/admin/security-baseline)
- [监控与可观测性](/admin/observability-guide)
- [发布前检查清单](/admin/pre-release-checklist)
