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

本文面向后端 `mss-boot-admin` 与唯一前端镜像 `mss-boot-admin-antd-v6` 的单租户部署场景，覆盖：

- 本地容器化验证
- 基于 MySQL 的生产部署基线
- Nginx 反向代理
- 日志与任务调度目录约定；上传仅覆盖本地非生产验证

## 部署前检查

- 已准备 Docker 与 Docker Compose 环境
- 已确认后端使用的数据库（本地可用 SQLite，生产建议 MySQL）
- 已确认对外端口：后端 `8080`，前端 `8001`
- 已确认生产环境不暴露上传入口；Local/S3-compatible 当前仍为 Legacy / Blocked
- 若启用 WebSocket 集群或缓存，已准备 Redis

下列命令固定使用已发布并完成公开对账的当前稳定版 `v1.3.2`。生产部署还应记录并
固定验证过的镜像 digest，绝不能依赖 `latest`；未来候选的资格验证仍必须使用同一
冻结提交在受控流程中构建的候选制品。

## 推荐目录约定

```text
/opt/mss-boot-admin/
├── config/          # 配置文件
├── logs/            # 后端运行日志
├── public/          # 仅本地开发/评估上传，不属于生产基线
├── data/            # SQLite 或备份文件
└── compose/         # compose 编排文件
```

## 一、本地容器化验证

### 1. 启动 MySQL

```bash
export MSS_LOCAL_MYSQL_PASSWORD="$(openssl rand -hex 24)"

docker run -d \
  --name mss-mysql \
  --restart unless-stopped \
  -e MYSQL_ROOT_PASSWORD="${MSS_LOCAL_MYSQL_PASSWORD}" \
  -e MYSQL_DATABASE=mss_boot_admin \
  -p 3306:3306 \
  mysql:8
```

### 2. 执行迁移

```bash
docker run --rm \
  --network host \
  -e STAGE=local \
  -e DB_DRIVER=mysql \
  -e DB_DSN="root:${MSS_LOCAL_MYSQL_PASSWORD}@tcp(127.0.0.1:3306)/mss_boot_admin?charset=utf8mb4&parseTime=True&loc=Local" \
  ghcr.io/mss-boot-io/mss-boot-admin:v1.3.2 \
  migrate
```

### 3. 同步 API 注册表

```bash
docker run --rm \
  --network host \
  -e STAGE=local \
  -e DB_DRIVER=mysql \
  -e DB_DSN="root:${MSS_LOCAL_MYSQL_PASSWORD}@tcp(127.0.0.1:3306)/mss_boot_admin?charset=utf8mb4&parseTime=True&loc=Local" \
  ghcr.io/mss-boot-io/mss-boot-admin:v1.3.2 \
  server -a
```

该一次性命令与迁移、后端服务使用相同镜像、阶段和 DSN，完成后退出。它负责同步
菜单“绑定 API”所需的 API 注册数据；路由变化后应在启动新版本前重新执行。

### 4. 启动后端

```bash
docker run -d \
  --name mss-boot-admin \
  --restart unless-stopped \
  --network host \
  -e STAGE=local \
  -e DB_DRIVER=mysql \
  -e DB_DSN="root:${MSS_LOCAL_MYSQL_PASSWORD}@tcp(127.0.0.1:3306)/mss_boot_admin?charset=utf8mb4&parseTime=True&loc=Local" \
  -v $(pwd)/logs:/app/logs \
  -v $(pwd)/public:/app/public \
  ghcr.io/mss-boot-io/mss-boot-admin:v1.3.2 \
  server
```

`public` 挂载只用于本地上传边界验证，不能作为生产持久化或 Delivery 方案。

### 5. 启动前端

```bash
docker run -d \
  --name mss-boot-admin-antd-v6 \
  --restart unless-stopped \
  -p 8001:80 \
  ghcr.io/mss-boot-io/mss-boot-admin-antd-v6:v1.3.2
```

## 二、生产部署基线

### 推荐配置

生产环境建议：

- 数据库：MySQL
- 缓存/集群：Redis
- 上传：应用不会自动关闭；ingress 显式阻断两个上传路径，并且不授予通用 `storage:upload` 权限作为纵深防御。头像入口没有独立 Casbin permission，Local/S3-compatible 均为 Legacy / Blocked
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
    proxy_pass http://127.0.0.1:8001;
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
curl -I http://127.0.0.1:8001/healthz
```

### 核心检查项

- [ ] 后端 `8080` 可访问
- [ ] 前端 `8001` 可访问且 `/healthz` 返回 `mss-boot-admin-antd-v6`
- [ ] `/admin/api/storage/upload` 与 `/admin/api/user/avatar` 未向生产流量开放
- [ ] Nginx 未把 `/public/` 当作生产对象 Delivery
- [ ] WebSocket 握手正常
- [ ] `logs/` 目录持续写入
- [ ] 定时任务 `checked_at` 正常更新
- [ ] Redis 可用时 WebSocket 集群模式已启用

### 发布后巡检

- [ ] 登录与退出正常
- [ ] 监控页面数据可见
- [ ] 日志页面有数据
- [ ] 告警通知渠道可联通
- [ ] 上传与头像写入在 provider gate 完成前保持关闭

## 五、常见问题

### 1. 为什么不能把 `/public/` 代理当作生产上传方案

`D0-safety` 内部检查点只证明 Upload admission 与 Local write boundary：
`storage:maxSize` 以 bytes 为单位，默认 10 MiB（`10485760`），硬上限
100 MiB（`104857600`）；`storage:allowedTypes` 使用 MIME types /
wildcards；Local 使用 opaque UUID key、受限根、create-only 写入与 partial
cleanup。

`prod` 模式不会注册 `application.staticPath`，返回的 `/public/...` 或 S3
endpoint 拼接 URL 也不是已鉴权 Delivery。`D1-provider-owner` 必须先完成 provider
fail-closed、immutable profile 与 single owner；S3 conditional create-only
及 Local/S3-compatible 共用 conformance suite 留在 `D4-authorization-object`。

### 2. 用户任务调度未生效

优先检查：

- `task.enable` 是否为 `true`
- `task.spec` 是否为 6 段 cron 表达式
- 数据库中的任务 `status` 是否为 `enabled`

`task.enable` 不控制内置系统作业。即使它为 `false`，监控采样和会话清理也应
随服务运行；这两项应通过监控响应时间戳和服务日志排查，而不是查询 TaskRun。

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
