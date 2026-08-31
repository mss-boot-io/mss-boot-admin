---
title: v1.0.0 兼容性矩阵
order: 4
nav:
  title: 发布
  order: 3
description: 已发布 v1.0.0 的 Go 模块、数据库、API、配置、前后端和能力状态兼容边界
keywords: [v1.0.0 compatibility api module database frontend]
---

# v1.0.0 兼容性矩阵

> 历史版本：本页仅保留不可变发布、升级与恢复证据，不用于新项目。当前稳定版仍为 v1.3.2；v1.3.5 与 v1.3.6 已永久停止且只保留不可变的部分发布证据；v1.3.7 是未稳定、不可采用的候选，各发布面可能处于不同公开阶段。完整 Distribution stable promotion 和最终 current-stable policy 对账前不得安装、创建或升级；Docs 网站可通过 `docs/v*` 异步候补，其状态不影响这一采用门禁；当前状态以远端发布台账和 [v1.3.7 候选采用页](/getting-started)为准。

本页定义从 v0.7.x 到合并仓库首个稳定 1.0（v1.0.0）的支持边界。`compatible` 表示在列出的前提下受支持，不表示可以跳过迁移；`preview` 表示实现已存在但尚未通过全部稳定门禁。未发布 v0.8.0 候选版的制品或验证结果不构成 v1.0.0 兼容性证据。

## 组件与工具链

| 表面 | v0.7.x | v1.0.0 合同 | 结论 |
| --- | --- | --- | --- |
| Root module | Admin 应用位于 `github.com/mss-boot-io/mss-boot-admin` | 根模块承载 Agent/foundation 工具 | breaking |
| Admin module | 无独立 module | `github.com/mss-boot-io/mss-boot-admin/admin` | breaking |
| Framework module | `github.com/mss-boot-io/mss-boot` | `github.com/mss-boot-io/mss-boot-admin/mss-boot` | breaking |
| Framework tag | standalone 版本线 | `mss-boot/v1.0.0` | 必须先发布并外部解析 |
| Root tag | `v0.7.x` | `v1.0.0` | framework 验证后发布 |
| Frontend tag | predecessor repository/version history | `web/antd/v1.0.0` | 可选独立发布，不得冒充根版本 |
| Go | 1.26.0 baseline | 1.26.5，`GOTOOLCHAIN=local` 验证 | toolchain update |
| Node.js | 历史环境不统一 | `>=22 <25` | build contract |
| pnpm | 历史环境不统一 | `9.15.9` | frozen build contract |

`go.work` 仅服务合并仓库开发。下游和发布验证必须用 `GOWORK=off`，否则本地 replace 会掩盖未发布的 framework tag。

## 数据库

| 数据库 | 本地/CI 角色 | v1.0.0 稳定门禁 |
| --- | --- | --- |
| SQLite | 默认本地开发与快速 migration fixture | fresh、v0.7 upgrade、repeat、failure rollback |
| MySQL 8.4 | 生产兼容矩阵 | fresh、upgrade、PAT/OAuth、权限、ConfigRevision、幂等 |
| PostgreSQL 17 | 生产兼容矩阵 | fresh、upgrade、PAT/OAuth、权限、ConfigRevision、幂等 |

数据库迁移是必须步骤。主题 revision 只增加 `mss_boot_config_revisions`，保留既有 app/user config 行；运行时工具清理保留历史元数据和生成业务表。PAT 明文清理、OAuth 本地密码关闭和 provider credential sanitation 不是自动可逆变更。

## API 与客户端

| 能力 | v0.7 客户端行为 | v1.0.0 合同 | 兼容策略 |
| --- | --- | --- | --- |
| PAT 创建 | 历史 GET generator | `POST /admin/api/user-auth-tokens` | GET 返回 405；调用方必须迁移 |
| PAT 列表 | 可能依赖原 token | 只返回安全元数据/指纹 | 原值只显示一次；重新签发 |
| JWT refresh | 历史 GET 或 POST 混用 | `POST /admin/api/user/refresh-token` | state-changing GET 不支持 |
| OAuth authorize | 客户端参与较多 | POST intent，服务端生成 URL/state | 新前端与新后端配套 |
| OAuth callback | GET query callback | POST JSON body `code/state` | GET 返回 405 |
| OAuth binding | 浏览器 token 入口 | 旧 `/user/binding` 返回 405 | 使用服务端 state/binding intent |
| Role authorization read | 菜单路径快照 | 完整 paths + decimal revision + strong ETag | 新客户端读取后写入 |
| Role authorization write | 无条件覆盖 | bundled client 发送 `If-Match` | 旧客户端暂有告警窗口；stale 返回 412 |
| Theme read/write | 非版本化旧格式 | vendor media type、revision、ETag、412 | 旧投影暂保留；稳定性仍 preview |
| Storage upload | 任意已登录用户 | 非 root/PAT 要求 `storage:upload` | 不自动继承旧角色权限 |
| SystemConfig | 可能由普通管理角色访问 | root-only | breaking authorization tightening |
| AppConfig secrets | 随普通读写权限 | 独立 secret-read/secret-write | 未授权响应省略敏感键 |
| Runtime developer tools | model/template/virtual CRUD APIs | 路由移除 | 无运行时兼容层 |

HTTP 401 表示身份无效或失效，403 表示当前身份缺权限，405 表示历史 state-changing GET/旧 callback 已退役，412 表示版本化资源冲突，422 表示整资源校验失败。客户端不得把这些状态统一伪装为成功或空数据。

## 身份、角色与 root

- root 的核心语义不变：当前数据库中已启用的 root 身份绕过普通 Casbin 策略；
- JWT/PAT 中的 root 或完整角色快照不可信，后端每次从权威存储解析当前 principal；
- 通用 CRUD 不能删除或降级 root/default 角色和 root 用户，这是系统不变量保护，不是把 root 纳入普通策略约束；
- 公共注册和首次 OAuth 账户创建必须显式开启，并绑定唯一、启用、非 root 的默认角色；
- 最小权限默认角色迁移不会自动改派存量用户，歧义数据会使迁移失败；
- 旧 PAT 缺少最小 signed identity 时可能失效；PAT 沿用 owner 当前 RBAC，v1.0.0 仍没有独立 scopes/last-used 合同；
- 生产部署必须显式启用 `auth.sessionEnabled: true`，才能保证改密或会话撤销后旧浏览器 JWT 立即失效；保持关闭时，已签发 JWT 可能继续有效到过期。首次启用会要求没有 `sid` 的旧浏览器 JWT 重新登录。

## 配置、缓存与多实例

| 项目 | 支持边界 |
| --- | --- |
| 代码默认值 | 最低优先级、不可变 fallback |
| 应用主题 | 稀疏覆盖代码默认；需要 `config:read/config:write` |
| 个人主题 | 稀疏覆盖应用设置；authenticated-self |
| ConfigRevision | 数据库权威，复合键 `(scope, owner_id, resource)` |
| Redis 7.x | 发布 CI 以 `redis:7-alpine` 验证；派生缓存/通知失败回退数据库，不回滚已提交事务 |
| 浏览器 snapshot | 最多 24 小时、绑定随机认证会话，不是第四层配置 |
| SystemConfig | opaque/root-only，不进入通用 query cache |
| AppConfig credential | 不进入普通缓存；按 secret capability 省略或写入 |

旧后端 writer 不会推进 revision，因此版本化写场景不支持 v0.7/v1.0.0 后端长期混跑。先排空旧 writer，再部署新后端，最后部署新前端。

## 能力状态

| 能力 | v1.0.0 发布后状态 | 说明 |
| --- | --- | --- |
| Authentication / PAT / RBAC | stable | 已通过 v1.0.0 安全迁移和正反授权门禁 |
| Configuration cache consistency | stable | 数据库 revision 权威，Redis 可降级 |
| Layered theme settings | planned / preview | 外部 DB 并发/升级与完整浏览器矩阵未全部封板 |
| Monitoring history | beta | 单实例、有界、进程内历史，不替代集群可观测性 |
| User-managed tasks | beta | 与 always-on system jobs 分离 |
| Deterministic Agent CLI/generator/upgrade | planned | 开发期能力，不等于运行时开发工具 |
| Runtime dynamic model / virtual CRUD / browser generator | removed | 不提供兼容路由；历史数据保留 |

发布 v1.0.0 不会自动把 `planned` 或 `preview` 能力提升为 stable。状态只能在对应验收证据通过并经独立评审后修改。

## 已知限制

- 历史业务控制器的数据范围尚未全部接入统一 fail-closed resolver，不能仅凭菜单授权推断行级隔离已完成；
- PAT 尚无独立 scopes 与 last-used 追踪；
- monitor history 为当前实例内存数据，重启丢失且不聚合多个副本；
- OAuth 内存 state store 只适合单进程开发，多副本必须使用共享后端；
- 旧主题投影和缺失 `If-Match` 的兼容窗口仍存在，必须在后续明确版本中一起退场；
- 通用静态发行包必须证明 API endpoint 可部署；`build:local` 只适用于访问者浏览器与后端同机的本地场景。
