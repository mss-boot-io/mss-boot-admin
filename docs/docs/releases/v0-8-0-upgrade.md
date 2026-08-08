---
title: v0.8.0 升级指南
order: 3
nav:
  title: 发布
  order: 3
description: 从 v0.7.x 升级到计划中的 v0.8.0 的预检、迁移、部署和验证步骤
keywords: [v0.8.0 upgrade migration backup preflight]
---

# 从 v0.7.x 升级到 v0.8.0

本文适用于现有 v0.7.x Admin 数据库和下游源码。v0.8.0 是包含 Go 模块路径、认证、授权、配置与运行时能力变化的迁移版本，不支持“替换二进制后直接启动”的无预检升级。

:::warning
当前版本仍是 preview。必须先完成 [发布合同](/releases/v0-8-0) 的门禁并确认正式 tag 存在，再在生产环境执行本文。
:::

## 环境基线

| 依赖 | v0.8.0 合同 |
| --- | --- |
| Go | `1.26.5`；发布校验使用 `GOTOOLCHAIN=local` |
| Node.js | `>=22 <25` |
| pnpm | `9.15.9` |
| 本地数据库 | SQLite |
| 发布数据库矩阵 | MySQL 8.4、PostgreSQL 17、SQLite |
| Redis | 7.x（发布 CI 以 `redis:7-alpine` 验证）；本地单实例可选，多副本 OAuth state、权限通知和生产缓存场景需共享 Redis |

不要在生产升级时自动执行 `go mod tidy` 或非冻结的 `pnpm install`；使用已评审 tag 与锁定依赖生成的制品。

## 1. 备份与恢复演练

升级前必须保存同一时间点的一致快照：

- 完整数据库（包含 migration/version 表、Casbin 规则、菜单/API 元数据、配置、用户、角色、PAT 和 OAuth 绑定）；
- 全部 Admin 配置文件、环境变量清单和 secret 引用，不把 secret 明文写入工单；
- 本地或对象存储中的业务文件及其元数据；
- 当前后端、前端和容器的精确版本、镜像 digest 与启动参数；
- 反向代理、CORS、OAuth provider callback 和 Redis 配置。

在隔离环境完成一次“恢复备份 → 启动 v0.7 → 验证登录与关键读取”的演练。只有备份文件存在但没有恢复证据，不满足门禁。

## 2. 数据预检

在停止旧 writer 之前先生成只读报告；发现歧义时先处理数据并重新预检。

### 角色与权限

- 必须能唯一识别历史 root/default `admin` 角色；root+default 角色被多个用户共享、存在多个非 root 默认角色或默认角色已禁用时，最小权限迁移会停止；
- 盘点所有依赖隐式默认角色权限的公开注册与首次 OAuth 登录；v0.8 只允许一个已启用、非 root 的默认角色；
- 盘点直接调用上传、系统配置、应用配置凭据和角色授权 API 的非 root 自动化；这些能力需要新增的显式权限或改由 root 运维流程执行；
- 导出自定义菜单和 Casbin 策略，特别是 `/develop` 下的非内置子项，以便核对退役工具迁移只重挂无关菜单而不误删。

### PAT 与自动化

- 列出所有 PAT owner、用途、到期时间和调用方，不导出原始 bearer；
- 为依赖旧 token 结构的自动化准备重新签发和双端切换窗口；
- 确认调用方能够在创建/轮换响应中一次性接收新 token，列表接口不会再返回可恢复原值。

### OAuth 与本地密码

- 检查同一 provider + opaque identity 是否存在多个活动 owner。重复绑定会阻止唯一索引迁移，不能自动选一个用户；
- 盘点所有曾经绑定、解绑或软删除过 OAuth 的账户，并准备密码重置/管理员恢复路径；这些账户会按 fail-closed 语义关闭旧本地密码；
- 确认多副本环境使用共享 state store，callback Origin、redirect URI 与 `cors.allowOrigins` 精确匹配；
- 在 provider 侧 rotate/revoke 历史内置 GitHub OAuth 凭据。数据库迁移只能清理匹配记录，不能让 Git 历史中的值失效。

### 配置与运行时工具

- 识别 `AppConfig` 中四个受保护的 provider credential 字段及其 owner，确认新的 secret-read/secret-write 权限；
- 备份主题相关 application/user config 行和未知扩展行；revision 迁移不会替换原表；
- 导出仍依赖动态模型、字段、虚拟 CRUD 或模板 API 的调用方清单。v0.8 不提供兼容路由；
- 记录历史动态模型元数据和生成业务表的保留/导出责任人。自动迁移不会删除这些数据表。

## 3. 下游源码与模块路径

从 v0.7 的 standalone framework 路径迁移：

```text
github.com/mss-boot-io/mss-boot
    -> github.com/mss-boot-io/mss-boot-admin/mss-boot
```

同步更新 `go.mod` 的 `require`/`replace` 与所有 Go import。不要把本仓库 `go.work` 的本地替换复制到下游生产模块。

发布负责人必须先发布 `mss-boot/v0.8.0`，再在仓库外以 `GOWORK=off` 解析该模块。下游随后执行：

```shell
GOWORK=off go get github.com/mss-boot-io/mss-boot-admin/mss-boot@v0.8.0
GOWORK=off go mod tidy -diff
GOWORK=off go test ./...
```

如果外部解析仍落到旧模块、伪版本或本地 replace，停止升级。

## 4. 停止旧 writer

1. 宣布维护窗口并停止定时写入、后台 worker 和外部自动化；
2. 从负载均衡移除全部 v0.7 Admin 实例；
3. 等待在途写请求完成，记录最后一个成功事务和备份时间；
4. 保持数据库与 Redis 可用，但不要同时运行 v0.7 writer 和依赖 revision 的 v0.8 writer；
5. 再取得最终一致数据库快照。

旧实例不会推进配置或授权 revision。混跑 writer 会绕过 v0.8 的并发合同，因此不属于支持的滚动升级方式。

## 5. 执行迁移

:::info MySQL v0.7 基线说明
`v0.7.0` 的历史迁移 `1746193492486` 使用了 ANSI SQL 的 `"group"` 标识符，Options 迁移还使用了现代 MySQL 不支持的 `ADD COLUMN/CREATE INDEX IF NOT EXISTS`，默认角色查询则使用了 PostgreSQL 不支持的 MySQL 反引号。发布升级演练在一次性 v0.7 基线中启用会话级 `ANSI_QUOTES`，以当前仓库中同版本、同目标结构的跨数据库修正版替换 Options 迁移，并只把默认角色查询改写成等价的 GORM `clause.Eq`；每个替换都有特征校验，其他 v0.7 数据和迁移语义保持不变。随后当前候选仍以默认 SQL mode 完成升级和重复迁移。已经完成 v0.7 迁移的生产数据库不需要为 v0.8 开启该模式，也不要全局修改生产 SQL mode 或迁移版本行。
:::

从正式 tag 的源码执行：

```shell
cd admin
GOWORK=off go run . migrate
```

或者使用对应 root Release 中已校验版本的 Admin 二进制：

```shell
./admin migrate
```

不要手工插入、删除或改写 migration version 行。失败迁移应保留错误报告、修复预检数据后原样重跑。

### v0.8 关键迁移效果

| 迁移领域 | 主要效果 | 可逆性 |
| --- | --- | --- |
| Session 与菜单 | 增加/修复 session、session menu 和相应元数据 | 通常可由旧代码忽略；保留新增行 |
| Admin 路由权限 | 增加明确的组件/API 权限，清理未授权直达边界 | 使用前备份策略；优先 forward-fix |
| PAT 摘要 | 写入 token hash/fingerprint 并清空兼容明文列 | **不可由代码回滚恢复明文**；需重签或整库恢复 |
| OAuth 本地密码 | 对有 OAuth 历史账户设置 `local_password_disabled` | 通过显式密码重置恢复，不回填旧不可信密码 |
| OAuth identity | 生成精确 `identity_key` 并建立活动绑定唯一性 | 重复 owner 时迁移停止；人工裁决后重跑 |
| 运行时工具退役 | 删除内置菜单、API 元数据和策略，重挂无关菜单 | 历史数据表保留；要恢复旧产品需完整备份 |
| 内置 OAuth 清理 | 仅按历史凭据指纹清空匹配配置并收窄 scope | 不得恢复已泄露凭据；provider 侧必须撤销 |
| Monitor/Theme/Upload/Secret 权限 | 增加显式最小权限元数据，不自动给普通角色扩大授权 | 根据评审后的角色策略重新授权 |
| Config revision | 新增 `(scope, owner_id, resource)` revision 资源 | 附加表可由旧代码忽略；不要回退已提交选择 |
| 默认角色 | 移除历史 root 默认角色语义并创建最小权限 `user` 默认角色 | 歧义时停止；不自动改派存量用户 |

## 6. 部署顺序

1. 先部署全部 v0.8 后端，但保持流量关闭；
2. 运行健康、数据库、权限、缓存和 migration version 检查；
3. 部署与 v0.8 API 配套的前端静态制品并清理旧 HTML/CDN 入口缓存；
4. 开放内部管理员流量，验证 root 与一个非 root 角色；
5. 再开放普通用户和自动化流量；
6. 生产环境必须显式启用 `auth.sessionEnabled: true`，否则改密后已签发浏览器 JWT 仍可能有效到过期。不含 `sid` 的旧浏览器 JWT 会要求重新登录，应提前通知用户；
7. 多副本逐个加入负载均衡，并验证权限 revision/通知和 OAuth state 共享。

## 7. 升级后验证

至少验证以下场景并保存脱敏证据：

- root 仍能访问全部业务能力，非 root 未授权 API 返回 403，禁用用户/角色立即失效；
- 动态菜单保持存在，已移除开发工具的菜单、直达页和 API 返回正常 not-found/405 边界；
- 新默认注册用户不是 root，且只获得显式迁移授予的最小能力；
- PAT 创建/轮换只显示一次原值，旧值立即失效，PAT 不能执行交互式安全操作；
- OAuth state 重放失败，callback URL 不保留 code/state，OAuth 历史账户按预期进入密码恢复；
- 重复保存角色或主题时，过期 `If-Match` 返回 412 且数据库不发生部分写入；
- Redis 停机时配置读取回退数据库，已提交写入不被伪装成失败；
- SystemConfig 仅 root 可见，应用 credential 字段按 secret 权限省略或允许盲轮换；
- 等待至少两个后台采样周期后，CPU/内存接口返回按时间递增的多个采样点，浅色/深色图表均可读；
- 审计记录包含 actor/path/outcome/revision，但不包含密码、token、provider secret 或主题值；
- 重跑迁移不产生重复版本、菜单、权限或 revision 行。

源代码安装还应运行：

```shell
go run ./cmd/mss doctor --strict
go run ./cmd/mss verify --all
go run ./cmd/mss eval run --all
```

## 8. 升级判定

只有数据库、后端、前端、自动化和多实例行为全部通过，才结束维护窗口。任一安全迁移失败时不要绕过 version 检查启动新 writer；按照 [回滚与恢复](/releases/v0-8-0-rollback) 选择 forward-fix 或完整恢复。
