---
title: 部署指南
order: 13
nav:
  title: 指南
  order: 1
keywords: [deployment, docker, production, ant-design-v6]
---

# 当前交付边界

Admin 前后端分别发布：

| 组件 | 默认制品 | 身份 |
| --- | --- | --- |
| Go 后端 | 根版本二进制与 OCI 镜像 | `mss-boot-admin` |
| 默认前端 | 独立静态包与 OCI 镜像 | `mss-boot-admin-antd-v6` |

V6 始终位于 `web/antd-v6`，使用 `web/antd-v6/v{version}` 标签独立发布。
历史标签和制品保持不可变，但不再作为活动构建、部署或回滚输入。

生产部署必须使用已经合入 `main`、通过发布门禁且带完整 digest 的不可变镜像。
不要使用 `latest`、分支名镜像、PR head、个人构建或重新构建的历史源码。

# 本地同源交付验证

后端先在宿主机 `8080` 端口启动。默认 `local` 配置会启用 V6 的 HttpOnly
浏览器会话、CSRF 与 WebSocket ticket，并信任 `http://localhost:8001`。

```shell
# 仓库根目录
make web-build
docker compose -f compose/admin/docker-compose.yml up --detach --build
curl --fail http://localhost:8001/healthz
```

Compose 中的 Nginx 将 `/admin/` HTTP 和 WebSocket 请求转发到宿主机后端，浏览器
始终使用 `http://localhost:8001` 同源访问。停止本地前端：

```shell
docker compose -f compose/admin/docker-compose.yml down
```

也可通过 `MSS_FRONTEND_V6_IMAGE=<repository>@sha256:<digest>` 验证一个已资格化的
V6 镜像。该本地配置不注入生产凭据，也不替代生产 ingress、TLS 或 Secret 管理。

# 生产部署要求

## 前端和 API 路由

生产入口必须满足以下条件：

- 浏览器通过一个 HTTPS origin 访问 V6；
- `/admin/` API 与 WebSocket upgrade 路由到同一版本兼容的 Go 后端；
- `/healthz` 能区分并返回 `mss-boot-admin-antd-v6`；
- HTML 与 `release.json` 不缓存，带内容 hash 的静态资源可 immutable 缓存；
- 旧 chunk 缺失返回 404，Service Worker 路径保持禁用；
- 发布记录保存 V6 镜像 digest、前端 `release.json`、后端 commit 和部署时间。

## 后端安全配置

V6 生产浏览器会话必须显式配置，基础配置不会替部署者猜测：

- `application.mode: prod`；
- 唯一、精确的 HTTPS `application.origin` 和 CORS origin；
- V6 浏览器会话和服务端会话校验始终启用，不存在兼容模式开关；
- `auth.browserSession.secure: true`，SameSite 只能为 `lax` 或 `strict`；
- 强随机 `auth.key` 和独立 `auth.identityKey`，通过 Secret 管理注入；
- 所有副本共享的 Redis 会话资源；
- 跨 origin 时允许 `X-CSRF-Token`，否则启动校验失败；
- BrowserSession OAuth 应用、密钥和精确 `/user/oauth/callback/:provider` URI；
- 只信任实际受控的反向代理地址，不接受任意 forwarded headers。

生产校验会对 HTTP origin、非 Secure Cookie、默认/弱密钥、缺少会话依赖和不完整
CSRF 配置失败关闭。不要通过关闭校验来解决部署错误。

数据库 DSN、认证密钥、OAuth secret、对象存储凭据和第三方 token 不得写入 Compose、
Kubernetes ConfigMap、命令行历史或仓库文件；使用部署平台的 SecretRef/Secret 机制。

# 发布与提升顺序

1. 通过 PR 将 V6-only 后端、前端和前向配置清理迁移合入 `main`。
2. 从同一 merged-main commit 生成后端制品以及 V6 静态包、checksum、SBOM、provenance 和镜像 digest。
3. 在生产等价环境验证迁移、登录、刷新、退出、OAuth、CSRF、WebSocket、权限、直达路由与 Nginx 缓存。
4. 只提升精确的后端与 V6 digest，并记录版本、配置、迁移和负责人。
5. 覆盖至少一次正常会话到期前续期，再完成发布资格确认。

根版本发布包可以附带同一 commit 构建的 V6 `dist`，但这不替代 V6 独立标签和镜像
发布，也不能据此推断某个 `web/antd-v6/v{version}` Release 已存在。

# 回滚

回滚时同时重部署上一个已资格化的 V6 前端与后端镜像 digest。禁止从当前
分支重建历史源码、移动标签、force push、复用镜像名或反向执行已经上线的
前向数据库迁移。V5 源码、制品和浏览器协议不作为回滚机制恢复。

任何需要代码修复的切流问题必须经新 PR 合入 `main`，重新生成制品并重启受影响的
观察门禁。

# Kubernetes 与其他平台

基础仓库不假设特定生产平台。Kubernetes、Nomad 或其他托管环境都必须
实现相同合同：不可变 digest、同源路由、TLS/Secret、readiness、逐步提升、可观测信号
和可审计回滚。建议至少配置：

- 后端 `/healthz` 与 `/readyz`；
- V6 `/healthz` 且校验 application identity；
- Pod/实例滚动更新与可用性预算；
- 登录/刷新失败、401/403、CSRF、OAuth callback、WebSocket 重连、路由错误和前端异常告警；
- 数据库备份与恢复演练；
- 镜像 digest、配置版本、迁移版本和回滚决定的发布记录。

# API 认证边界

浏览器只使用 V6 HttpOnly 会话、CSRF、BrowserSession OAuth 和一次性 WebSocket
ticket。通用 `Authorization: Bearer` 与 PAT 继续服务已记录的非浏览器自动化，
不用于恢复 token 返回式浏览器登录或 URL WebSocket token。

详细门禁与回滚原则见 [Ant Design 6 迁移计划](/admin/ant-design-v6-migration-plan)。
