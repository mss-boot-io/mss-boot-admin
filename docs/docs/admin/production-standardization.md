---
title: 生产部署标准化
order: 27
nav:
  order: 1
  title: admin
description: mss-boot-admin 的单机、容器、代理与运维基线说明
keywords: [admin production deploy standardization nginx redis mysql]
---

## 目标

把 `mss-boot-admin` 的生产部署从“能启动”提升到“可复用、可巡检、可运维”。

## 推荐部署基线

### 组件建议

| 组件 | 推荐方案 | 说明 |
|------|----------|------|
| 数据库 | MySQL | 本地可用 SQLite，生产建议 MySQL |
| 缓存 | Redis | WebSocket 集群、缓存、部分配置依赖 |
| 反向代理 | Nginx | 统一入口、静态资源与 WS 代理 |
| 日志 | 文件输出 | 配合日志清理任务 |
| 上传 | 部署侧显式阻断 | 应用不会自动关闭上传；ingress 必须拒绝两个上传路径，并且不得授予通用 `storage:upload` 权限作为纵深防御。头像入口没有独立 Casbin permission，不能替代 ingress 门禁；Local/S3-compatible 仍为 Legacy / Blocked |

## 标准启动顺序

1. 启动数据库
2. 启动 Redis
3. 执行数据库迁移
4. 启动后端服务
5. 启动前端服务
6. 配置 Nginx
7. 验证 `/healthz`、前端首页、WebSocket，并确认上传入口未暴露

## 必备配置基线

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
```

## 目录基线

```text
logs/     # 运行日志
config/   # 部署配置
backup/   # 数据库或配置备份
```

## Upload admission 的当前边界

`D0-safety` 内部检查点已经把 `storage:maxSize` 定义为 bytes：默认 10 MiB
（`10485760`），硬上限 100 MiB（`104857600`）。`storage:allowedTypes`
使用逗号分隔的 MIME types / wildcards（例如 `image/png,image/*`），不是文件
扩展名。入口在 multipart 解析前限制 body，Local 使用随机 opaque key、受限根、
create-only 写入与 partial cleanup。

这些是 admission/local-write safety，不是 provider 或 Delivery 的生产晋级。
Local 与 S3-compatible 仍是 `Legacy / Blocked`。`D1-provider-owner` 完成未知
provider fail-closed、immutable profile 和 single owner；S3 conditional
create-only 与共用 provider conformance 留在 `D4-authorization-object`。

当前应用没有“生产模式自动关闭上传”的开关。部署必须在 ingress 阻断
`/admin/api/storage/upload` 与 `/admin/api/user/avatar`，并确保权限策略不授予
通用 `storage:upload` 能力作为纵深防御。头像是 authenticated-self 路由，没有独立 Casbin
permission，因此权限配置不能替代它的 ingress 门禁。

## 必做巡检

- [ ] 后端健康检查正常
- [ ] 前端页面可访问
- [ ] 登录后首页可正常加载
- [ ] `/admin/api/storage/upload` 与 `/admin/api/user/avatar` 在生产入口未开放
- [ ] 未配置或代理 `/public/` 作为生产对象 Delivery
- [ ] 日志目录持续写入
- [ ] 至少一个启用任务的 `checked_at` 正常变化
- [ ] Redis 可用时 WebSocket 集群模式正常启用

## 不建议的做法

- 不建议生产继续使用默认账号密码
- 不建议把通知密钥直接写死在仓库文件中
- 不建议关闭日志输出
- 不得仅凭 `/public/` 代理、持久化目录或 endpoint URL 上线头像/上传功能
- 在 `D1-provider-owner` 与 `D4-object-delivery` 冻结门禁通过前，不得启用 Local 或 S3-compatible 上传

## 推荐阅读

- [容器化与生产部署](/admin/docker)
- [发布验证清单](/admin/release-verification-checklist)
- [安全基线指南](/admin/security-baseline)
