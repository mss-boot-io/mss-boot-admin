---
title: 容器化与生产部署
order: 12
nav:
  order: 1
  title: admin
description: mss-boot-admin 的容器化、反向代理与生产部署指南
keywords: [admin docker deploy nginx production]
---

## 适用范围

本文面向 `mss-boot-admin` 与 `mss-boot-admin-antd` 的单租户部署场景，覆盖：

- 本地容器化验证
- 基于 MySQL 的生产部署基线
- Nginx 反向代理
- 日志、上传与任务调度目录约定

## 部署前检查

- 已准备 Docker 与 Docker Compose 环境
- 已确认后端使用的数据库（本地可用 SQLite，生产建议 MySQL）
- 已确认对外端口：后端 `8080`，前端 `8000`
- 已确认静态文件与上传目录需要持久化
- 若启用 WebSocket 集群或缓存，已准备 Redis

## 推荐目录约定

```text
/opt/mss-boot-admin/
├── config/          # 配置文件
├── logs/            # 后端运行日志
├── public/          # 本地上传文件
├── data/            # SQLite 或备份文件
└── compose/         # compose 编排文件
```

## 一、本地容器化验证

### 1. 启动 MySQL

```bash
docker run -d \
  --name mss-mysql \
  --restart unless-stopped \
  -e MYSQL_ROOT_PASSWORD=123456 \
  -e MYSQL_DATABASE=mss_boot_admin \
  -p 3306:3306 \
  mysql:8
```

### 2. 执行迁移

```bash
docker run --rm \
  --network host \
  -e DB_DRIVER=mysql \
  -e DB_DSN='root:123456@tcp(127.0.0.1:3306)/mss_boot_admin?charset=utf8mb4&parseTime=True&loc=Local' \
  ghcr.io/mss-boot-io/mss-boot-admin:latest \
  migrate
```

### 3. 启动后端

```bash
docker run -d \
  --name mss-boot-admin \
  --restart unless-stopped \
  --network host \
  -e DB_DRIVER=mysql \
  -e DB_DSN='root:123456@tcp(127.0.0.1:3306)/mss_boot_admin?charset=utf8mb4&parseTime=True&loc=Local' \
  -v $(pwd)/logs:/app/logs \
  -v $(pwd)/public:/app/public \
  ghcr.io/mss-boot-io/mss-boot-admin:latest \
  server
```

### 4. 启动前端

```bash
docker run -d \
  --name mss-boot-admin-antd \
  --restart unless-stopped \
  -p 8000:80 \
  ghcr.io/mss-boot-io/mss-boot-admin-antd:latest
```

## 二、生产部署基线

### 推荐配置

生产环境建议：

- 数据库：MySQL
- 缓存/集群：Redis
- 上传：本地持久化目录或 S3
- 反向代理：Nginx
- 日志：文件输出 + 定期清理

### 环境变量建议

```bash
export DB_DRIVER=mysql
export DB_DSN='user:password@tcp(mysql:3306)/mss_boot_admin?charset=utf8mb4&parseTime=True&loc=Local'
export CONFIG_PROVIDER=local
export REDIS_PASSWORD='change-me'
```

### 关键配置建议

```yaml
server:
  addr: 0.0.0.0:8080

logger:
  path: logs
  stdout: file
  level: info

task:
  enable: true
  spec: "0 */1 * * * *"

cache:
  redis:
    addr: 'redis:6379'
    password: '{{ .Env.REDIS_PASSWORD }}'
```

## 三、Nginx 反向代理示例

```nginx
server {
  listen 80;
  server_name admin.example.com;

  location / {
    proxy_pass http://127.0.0.1:8000;
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
  }

  location /admin/api/ {
    proxy_pass http://127.0.0.1:8080;
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
  }

  location /public/ {
    proxy_pass http://127.0.0.1:8080;
  }

  location /admin/api/ws/connect {
    proxy_pass http://127.0.0.1:8080;
    proxy_http_version 1.1;
    proxy_set_header Upgrade $http_upgrade;
    proxy_set_header Connection "upgrade";
    proxy_set_header Host $host;
  }
}
```

## 四、运维检查清单

### 启动后验证

```bash
curl -I http://127.0.0.1:8080/healthz
curl -I http://127.0.0.1:8000
```

### 核心检查项

- [ ] 后端 `8080` 可访问
- [ ] 前端 `8000` 可访问
- [ ] `/public/*` 上传访问正常
- [ ] WebSocket 握手正常
- [ ] `logs/` 目录持续写入
- [ ] 定时任务 `checked_at` 正常更新
- [ ] Redis 可用时 WebSocket 集群模式已启用

### 发布后巡检

- [ ] 登录与退出正常
- [ ] 监控页面数据可见
- [ ] 日志页面有数据
- [ ] 告警通知渠道可联通
- [ ] 文件上传与头像访问正常

## 五、常见问题

### 1. 上传后图片无法访问

优先检查：

- `application.yml` 中 `application.staticPath` 是否配置 `/public: public`
- 前端代理是否转发 `/public/`
- Nginx 是否代理 `/public/`

### 2. 任务调度未生效

优先检查：

- `task.enable` 是否为 `true`
- `task.spec` 是否为 6 段 cron 表达式
- 数据库中的任务 `status` 是否为 `enabled`

### 3. WebSocket 集群未启用

优先检查：

- Redis 是否可连接
- `cache.redis` 是否正确配置
- 服务启动日志中是否出现 `WebSocket cluster mode enabled`

## 推荐阅读

- [快速开始](/admin/quickly)
- [本地联调](/admin/local-debug)
- [集成测试指南](/admin/integration-test-guide)
- [四期路线图](/admin/phase-4-roadmap)
- [五期路线图](/admin/phase-5-roadmap)
