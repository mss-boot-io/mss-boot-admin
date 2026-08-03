# 项目全量配置教程

本教程涵盖 mss-boot-admin 的所有配置项，帮助您从开发到生产的完整配置。

## 📋 目录

- [配置概览](#配置概览)
- [环境变量](#环境变量)
- [数据库配置](#数据库配置)
- [认证配置](#认证配置)
- [缓存配置](#缓存配置)
- [队列配置](#队列配置)
- [通知配置](#通知配置)
- [任务配置](#任务配置)
- [存储配置](#存储配置)
- [监控配置](#监控配置)
- [日志配置](#日志配置)
- [安全配置](#安全配置)
- [前端配置](#前端配置)
- [生产环境配置](#生产环境配置)

---

## 配置概览

### 配置优先级

```
环境变量 > 配置文件 > 默认值
```

### 配置文件位置

```
mss-boot-admin/
├── config/
│   ├── application.yml    # 主配置文件
│   ├── application.go     # 配置结构定义
│   ├── auth.go            # 认证配置
│   └── notification.go    # 通知配置
```

### 配置文件示例

```yaml
server:
  addr: 0.0.0.0:8080
  
application:
  mode: dev
  
database:
  driver: sqlite
  source: 'mss-boot-admin.db'
  
auth:
  key: 'mss-boot-admin-secret'
  timeout: '12h'
```

---

## 环境变量

### 核心环境变量

```bash
# 数据库连接（必需）
export DB_DSN="user:password@tcp(host:3306)/dbname?charset=utf8mb4&parseTime=True&loc=Local"

# Redis 连接（可选）
export REDIS_ADDR="127.0.0.1:6379"
export REDIS_PASSWORD="your-password"

# 认证密钥（生产环境必需）
export AUTH_KEY="your-production-secret-key"

# 应用模式
export APP_MODE="prod"  # dev, test, prod
```

### 使用方式

```bash
# 方式1: 命令行
DB_DSN="..." go run . server

# 方式2: .env 文件
cat > .env << EOF
DB_DSN="user:password@tcp(host:3306)/dbname"
REDIS_ADDR="127.0.0.1:6379"
AUTH_KEY="your-secret-key"
EOF

source .env
go run . server
```

---

## 数据库配置

### SQLite（开发环境）

```yaml
database:
  driver: sqlite
  source: 'mss-boot-admin.db'
  config:
    disableForeignKeyConstraintWhenMigrating: true
```

### MySQL（生产环境）

```yaml
database:
  driver: mysql
  source: '${DB_DSN}'  # 从环境变量读取
  name: mss_boot_admin
  config:
    maxOpenConns: 100
    maxIdleConns: 10
    connMaxLifetime: 1h
    disableForeignKeyConstraintWhenMigrating: false
```

**环境变量**：
```bash
export DB_DSN="mss_admin:StrongPassword123@tcp(192.168.1.100:3306)/mss_boot_admin?charset=utf8mb4&parseTime=True&loc=Local"
```

**创建数据库**：
```sql
CREATE DATABASE mss_boot_admin 
  CHARACTER SET utf8mb4 
  COLLATE utf8mb4_unicode_ci;

CREATE USER 'mss_admin'@'%' IDENTIFIED BY 'StrongPassword123';
GRANT ALL PRIVILEGES ON mss_boot_admin.* TO 'mss_admin'@'%';
FLUSH PRIVILEGES;
```

### PostgreSQL

```yaml
database:
  driver: postgres
  source: 'host=127.0.0.1 user=postgres password=password dbname=mss_boot_admin port=5432 sslmode=disable'
```

### 连接池优化

```yaml
database:
  config:
    maxOpenConns: 100      # 最大打开连接数
    maxIdleConns: 10       # 最大空闲连接数
    connMaxLifetime: 1h    # 连接最大生命周期
    connMaxIdleTime: 10m   # 空闲连接最大生命周期
```

**性能建议**：
- 生产环境：`maxOpenConns = CPU核心数 * 2 + 有效磁盘数`
- 开发环境：`maxOpenConns = 10`

---

## 认证配置

### JWT 配置

```yaml
auth:
  realm: 'mss-boot-admin zone'           # 认证域
  key: '${AUTH_KEY}'                      # JWT 密钥（生产环境必须修改）
  timeout: '12h'                          # Token 有效期
  maxRefresh: '2160h'                     # 最大刷新时间（90天）
  identityKey: 'mss-boot-admin-identity-key'  # 身份标识键
```

**生成密钥**：
```bash
# 方式1: openssl
openssl rand -base64 32

# 方式2: go
go run -e 'package main\nimport("crypto/rand";"encoding/base64";"fmt")\nfunc main(){b:=make([]byte,32);rand.Read(b);fmt.Println(base64.StdEncoding.EncodeToString(b))}'

# 生产环境设置
export AUTH_KEY="$(openssl rand -base64 32)"
```

### OAuth2 配置

#### GitHub OAuth

```yaml
# 在系统配置中设置（前端访问 /app-config）
security:
  githubEnabled: true
  githubClientId: "your-github-client-id"
  githubClientSecret: "your-github-client-secret"
  githubRedirectURL: "https://your-domain.com/user/github/callback"
  githubScope: "user:email,repo"
```

**创建 GitHub OAuth App**：
1. 访问 https://github.com/settings/developers
2. 点击 "New OAuth App"
3. 填写信息：
   - Application name: `mss-boot-admin`
   - Homepage URL: `https://your-domain.com`
   - Authorization callback URL: `https://your-domain.com/user/github/callback`
4. 获取 Client ID 和 Client Secret

#### 飞书 OAuth

```yaml
security:
  larkEnabled: true
  larkAppId: "cli_xxxxxxxxxx"
  larkAppSecret: "your-lark-app-secret"
  larkRedirectURI: "https://your-domain.com/user/lark/callback"
```

---

## 缓存配置

### 内存缓存（开发环境）

```yaml
cache:
  queryCache: true
  queryCacheDuration: 1h
  queryCacheKeys:
    - '*'
```

### Redis 缓存（生产环境）

```yaml
cache:
  queryCache: true
  queryCacheDuration: 1h
  redis:
    addr: '${REDIS_ADDR}'
    password: '${REDIS_PASSWORD}'
    db: 0
    poolSize: 10
    minIdleConns: 5
    maxRetries: 3
    dialTimeout: 5s
    readTimeout: 3s
    writeTimeout: 3s
```

**环境变量**：
```bash
export REDIS_ADDR="192.168.1.100:6379"
export REDIS_PASSWORD="StrongRedisPassword"
```

**Redis 集群**：
```yaml
cache:
  redis:
    type: cluster
    addrs:
      - "redis-0:6379"
      - "redis-1:6379"
      - "redis-2:6379"
    password: '${REDIS_PASSWORD}'
```

---

## 队列配置

### 内存队列（开发环境）

```yaml
queue:
  memory:
    poolSize: 10
```

### NSQ 队列

```yaml
queue:
  nsq:
    addr: "127.0.0.1:4150"
    lookupAddr: "127.0.0.1:4161"
```

### Pulsar 队列

```yaml
queue:
  pulsar:
    url: "pulsar://localhost:6650"
```

---

## 通知配置

### 邮件通知

```yaml
notification:
  email:
    enabled: true
    host: "smtp.gmail.com"
    port: 587
    username: "your-email@gmail.com"
    password: '${EMAIL_PASSWORD}'
    from: "noreply@your-domain.com"
    useTLS: true
```

**Gmail 配置**：
1. 启用两步验证
2. 生成应用专用密码
3. 使用应用密码作为 `password`

**企业邮箱**：
```yaml
notification:
  email:
    host: "smtp.exmail.qq.com"  # 腾讯企业邮箱
    port: 465
    username: "noreply@company.com"
    password: '${EMAIL_PASSWORD}'
    from: "noreply@company.com"
    useSSL: true
```

### 钉钉通知

```yaml
notification:
  dingtalk:
    enabled: true
    webhook: "https://oapi.dingtalk.com/robot/send?access_token=xxx"
    secret: "your-dingtalk-secret"
```

**创建钉钉机器人**：
1. 打开钉钉群设置
2. 智能群助手 → 添加机器人
3. 选择"自定义"机器人
4. 安全设置选择"加签"
5. 复制 Webhook 和密钥

### 企业微信通知

```yaml
notification:
  wechat:
    enabled: true
    webhook: "https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=xxx"
```

---

## 任务配置

### 内置任务调度

```yaml
task:
  enable: true
  spec: "0 */1 * * * *"  # 每分钟执行
```

### Kubernetes Job 模式

```yaml
task:
  provider: k8s
  config:
    namespace: "default"
    image: "mss-boot-admin:latest"
```

---

## 存储配置

### 本地存储

```yaml
application:
  staticPath:
    /public: public  # 静态文件映射
```

**目录结构**：
```
mss-boot-admin/
└── public/
    └── {userID}/
        └── avatar.png
```

### 文件上传限制

在系统配置中设置（前端访问 `/app-config?key=storage`）：

```yaml
storage:
  maxSize: 10          # 最大文件大小（MB）
  allowedTypes: ".jpg,.png,.pdf,.doc,.docx"  # 允许的文件类型
```

### 对象存储（OSS/COS/MinIO）- Phase 6 计划

```yaml
# 未来支持
storage:
  driver: oss  # oss, cos, minio
  oss:
    endpoint: "oss-cn-hangzhou.aliyuncs.com"
    accessKeyId: '${OSS_ACCESS_KEY}'
    accessKeySecret: '${OSS_ACCESS_SECRET}'
    bucket: "mss-boot-admin"
```

---

## 监控配置

### PProf 性能分析

```yaml
server:
  pprof: true  # 启用 pprof
```

**访问地址**：
```
http://localhost:8080/debug/pprof/
```

### Pyroscope 持续 Profiling

```yaml
pyroscope:
  enabled: true
  applicationName: mss-boot-admin
  serverAddress: "http://pyroscope:4040"
  sampleRate: 100
```

### 健康检查

```yaml
server:
  healthz: true  # 健康检查端点
  readyz: true    # 就绪检查端点
```

**访问地址**：
```
http://localhost:8080/healthz
http://localhost:8080/readyz
```

### 监控指标

```yaml
server:
  metrics: true  # 启用 Prometheus 指标
```

**访问地址**：
```
http://localhost:8080/metrics
```

---

## 日志配置

### 基本配置

```yaml
logger:
  path: logs          # 日志目录
  stdout: file         # 输出方式: stdout, file, both
  level: info          # 日志级别: debug, info, warn, error
  json: false          # JSON 格式（生产环境推荐 true）
  addSource: true      # 添加源码位置
  maxSize: 100         # 单文件最大大小（MB）
  maxBackups: 10       # 保留文件数
  maxAge: 30           # 保留天数
  compress: true       # 压缩旧日志
```

### 生产环境日志

```yaml
logger:
  path: /var/log/mss-boot-admin
  stdout: file
  level: info
  json: true
  addSource: false
  maxSize: 100
  maxBackups: 30
  maxAge: 90
  compress: true
```

### Loki 日志收集

```yaml
logger:
  loki:
    url: "http://loki:3100"
    interval: "5s"
    labels:
      app: mss-boot-admin
      env: production
```

---

## 安全配置

### TLS/SSL

```yaml
ssl:
  enabled: true
  cert: "/path/to/cert.pem"
  key: "/path/to/key.pem"
```

### Casbin 权限模型

```yaml
database:
  casbinModel: |
    [request_definition]
    r = sub, tp, obj, act

    [policy_definition]
    p = sub, tp, obj, act

    [policy_effect]
    e = some(where (p.eft == allow))

    [matchers]
    m = r.sub == p.sub && r.tp == p.tp && keyMatch(r.obj, p.obj) && regexMatch(r.act, p.act)
```

### 安全基线

在系统配置中设置：

```yaml
security:
  # 密码策略
  passwordMinLength: 8
  passwordRequireUppercase: true
  passwordRequireLowercase: true
  passwordRequireNumber: true
  passwordRequireSpecial: false
  
  # 登录策略
  maxLoginAttempts: 5
  lockoutDuration: 30m
  
  # Session 策略
  sessionTimeout: 12h
  maxConcurrentSessions: 3
```

---

## 前端配置

### 环境配置

```bash
# .env.development
API_BASE_URL=http://localhost:8080/admin/api

# .env.production
API_BASE_URL=https://api.your-domain.com/admin/api
```

### 构建配置

```javascript
// .umirc.ts
export default {
  define: {
    'process.env.API_BASE_URL': process.env.API_BASE_URL,
  },
  proxy: {
    '/admin/api': {
      target: 'http://localhost:8080',
      changeOrigin: true,
    },
  },
};
```

### 国际化配置

前端支持的语言由后端 API `/admin/api/languages` 动态加载。

---

## 生产环境配置

### 完整配置示例

```yaml
# config/application.yml
server:
  addr: 0.0.0.0:8080
  metrics: true
  healthz: true
  readyz: true
  pprof: false  # 生产环境关闭

application:
  mode: prod
  origin: https://your-domain.com
  staticPath:
    /public: /var/www/mss-boot-admin/public

logger:
  path: /var/log/mss-boot-admin
  stdout: file
  level: info
  json: true
  addSource: false
  maxSize: 100
  maxBackups: 30
  maxAge: 90
  compress: true

database:
  driver: mysql
  source: '${DB_DSN}'
  name: mss_boot_admin
  config:
    maxOpenConns: 100
    maxIdleConns: 10
    connMaxLifetime: 1h
  timeout: 10s
  casbinModel: |
    [request_definition]
    r = sub, tp, obj, act
    [policy_definition]
    p = sub, tp, obj, act
    [policy_effect]
    e = some(where (p.eft == allow))
    [matchers]
    m = r.sub == p.sub && r.tp == p.tp && keyMatch(r.obj, p.obj) && regexMatch(r.act, p.act)

auth:
  realm: 'mss-boot-admin'
  key: '${AUTH_KEY}'
  timeout: '12h'
  maxRefresh: '2160h'
  identityKey: 'mss-boot-admin-identity'

pyroscope:
  enabled: true
  applicationName: mss-boot-admin
  serverAddress: 'http://pyroscope:4040'

cache:
  queryCache: true
  queryCacheDuration: 1h
  queryCacheKeys:
    - '*'
  redis:
    addr: '${REDIS_ADDR}'
    password: '${REDIS_PASSWORD}'
    db: 0
    poolSize: 20

queue:
  memory:
    poolSize: 10

notification:
  email:
    enabled: true
    host: "smtp.example.com"
    port: 587
    username: "noreply@example.com"
    password: '${EMAIL_PASSWORD}'
    from: "noreply@example.com"
  dingtalk:
    enabled: true
    webhook: '${DINGTALK_WEBHOOK}'
    secret: '${DINGTALK_SECRET}'
  wechat:
    enabled: false
    webhook: ""

task:
  enable: true
  spec: "0 */1 * * * *"
```

### 环境变量文件

```bash
# /etc/mss-boot-admin/.env
DB_DSN="mss_admin:StrongPassword123@tcp(db.internal:3306)/mss_boot_admin?charset=utf8mb4&parseTime=True&loc=Local"
REDIS_ADDR="redis.internal:6379"
REDIS_PASSWORD="StrongRedisPassword"
AUTH_KEY="$(openssl rand -base64 32)"
EMAIL_PASSWORD="your-email-password"
DINGTALK_WEBHOOK="https://oapi.dingtalk.com/robot/send?access_token=xxx"
DINGTALK_SECRET="your-dingtalk-secret"
```

### Systemd 服务

```ini
# /etc/systemd/system/mss-boot-admin.service
[Unit]
Description=mss-boot-admin
After=network.target mysql.service redis.service

[Service]
Type=simple
User=mss
Group=mss
WorkingDirectory=/opt/mss-boot-admin
EnvironmentFile=/etc/mss-boot-admin/.env
ExecStart=/opt/mss-boot-admin/mss-boot-admin server
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
```

```bash
# 启动服务
sudo systemctl daemon-reload
sudo systemctl enable mss-boot-admin
sudo systemctl start mss-boot-admin
sudo systemctl status mss-boot-admin
```

---

## 配置验证

### 检查配置

```bash
# 验证配置文件
go run . server -c config/application.yml --validate

# 查看运行状态
curl http://localhost:8080/healthz
curl http://localhost:8080/readyz
```

### 常见问题

#### 1. 数据库连接失败

```bash
# 检查数据库连接
mysql -h host -u user -p

# 检查环境变量
echo $DB_DSN
```

#### 2. Redis 连接失败

```bash
# 检查 Redis
redis-cli -h host -p 6379 -a password ping

# 检查环境变量
echo $REDIS_ADDR
```

#### 3. JWT 密钥错误

```bash
# 重新生成密钥
export AUTH_KEY="$(openssl rand -base64 32)"
```

---

## 下一步

- [部署指南](/admin/docker)
- [安全基线](/admin/security-baseline)
- [监控告警](/admin/observability-guide)
- [发布前检查清单](/admin/pre-release-checklist)