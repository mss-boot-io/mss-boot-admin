---
title: Token 与 OAuth2 联调说明
order: 20
nav:
  order: 1
  title: admin
description: mss-boot-admin 个人令牌管理、OAuth2 第三方登录、API 联调与安全审计说明
keywords: [admin token oauth2 pat api security audit]
---

## 概述

本文档描述 `mss-boot-admin` 的认证扩展能力，包括：

- 个人访问令牌 (Personal Access Token, PAT)
- OAuth2 第三方登录集成
- API 联调与调试指南
- 安全审计与日志追踪

这些能力使系统不仅支持标准用户名密码认证，还支持 API 程序化访问和第三方身份集成。

## 1. 个人访问令牌 (PAT)

### 概念

Personal Access Token 是一种用于 API 程序化访问的凭证，类似于 GitHub 的 PAT：

- 用户可自行创建、轮换、撤销令牌
- 令牌用于替代用户名密码调用 API
- 支持细粒度权限控制（可扩展）

### 当前数据模型

```
UserAuthToken
├── ID           → PAT 记录 ID，同时写入已签名的 personAccessToken claim
├── UserID       → 所属用户
├── LegacyToken  → 兼容列；迁移和新写入后始终为空
├── TokenHash    → 带版本前缀的 SHA-256 摘要，不通过 JSON 暴露
├── Fingerprint  → 可展示的短指纹，不用于认证
├── ExpiredAt    → 过期时间
├── Revoked      → 是否已撤销
└── CreatedAt / UpdatedAt / DeletedAt
```

### API 入口

| 路径                                     | 方法 | 功能                                      |
| ---------------------------------------- | ---- | ----------------------------------------- |
| `/admin/api/user-auth-tokens`            | GET  | 当前用户的未撤销令牌列表                  |
| `/admin/api/user-auth-tokens`            | POST | 创建令牌                                  |
| `/admin/api/user-auth-token/:id/revoke`  | PUT  | 按 owner 撤销令牌                         |
| `/admin/api/user-auth-token/:id/refresh` | PUT  | 按 owner 刷新令牌                         |
| `/admin/api/user-auth-token/generate`    | GET  | 历史入口，仅返回 `405 Method Not Allowed` |

JWT 刷新使用 `POST /admin/api/user/refresh-token`。历史 `GET` 入口仅返回
`405 Method Not Allowed`，不会签发或刷新 token。

### 使用方式

**创建令牌**

```bash
POST /admin/api/user-auth-tokens?validityPeriod=24h
Authorization: Bearer <interactive-session-jwt>

Response:
{
  "id": "<token-record-id>",
  "token": "<jwt-text>",
  "fingerprint": "<safe-short-fingerprint>",
  "expiredAt": "<timestamp>"
}
```

创建和轮换响应带 `Cache-Control: no-store`，完整 PAT 只在成功响应中出现一次。
列表接口只返回 ID、指纹、有效期和撤销状态等元数据，不返回 PAT 或摘要。关闭
一次性展示窗口后无法找回原值，只能重新轮换。

**使用令牌调用 API**

```bash
curl -H "Authorization: Bearer <jwt-text>" \
     https://admin.example.com/api/user/info
```

### 安全建议

- PAT 不能创建、列出、刷新或撤销 PAT，也不能执行密码重置、账户恢复标识修改或 OAuth2 绑定/解绑；这些交互式操作统一返回 `403 Forbidden`
- 服务端仅保存不可逆摘要；认证时先验证 JWT 签名，再按已签名的 PAT ID 读取记录，并以常量时间比较完整 bearer 的摘要
- 轮换使用 owner-scoped compare-and-swap；成功返回新值后旧 PAT 立即失效，并发轮换只允许一个成功结果
- 为兼容另行治理的 WebSocket 流程，`query: token` 暂时保留；API 与内置 UI 的访问/恢复日志只记录脱敏副本，所有大小写形式的 `token` query 值固定显示为 `[REDACTED]`，认证和 handler 仍读取原始请求
- 升级前必须备份数据库并排空旧实例；迁移会清空兼容明文列，且回滚不会恢复明文
- 当前 PAT 尚未提供 scopes 与 last-used 追踪，自动化权限仍由关联用户和现有 RBAC 决定
- 定期轮换令牌，并及时撤销不再使用的令牌

## 2. OAuth2 第三方登录

### 支持的提供商

当前已集成：

- GitHub
- Lark (飞书)

### 数据模型

```
OAuth2User (OAuth2 绑定信息)
├── UserID      → 系统用户 ID
├── Provider    → 提供商 (github/lark)
├── IdentityKey → provider + 精确 opaque ID；活动绑定唯一
├── OpenID / UnionID / Sub → 提供商侧身份标识
├── Name / Email / Picture → 同步的公开身份资料
└── CreatedAt / UpdatedAt / DeletedAt
```

`UserOAuth2` 不保存 provider access token 或 refresh token。短时集成凭据进入加密的
服务端 credential store，不进入用户绑定模型。

### 登录流程

```
用户点击第三方登录图标
    ↓
跳转到 OAuth2 授权页面
    ↓
用户授权后回调到系统
    ↓
服务端交换并短时持有 OAuth2 Token
    ↓
查询是否已绑定系统用户
    ├── 已绑定 → 直接登录，签发 JWT
    └── 未绑定 → 创建新用户并绑定，或关联已有账户
    ↓
服务端签发 Admin JWT；浏览器从未收到 provider token
```

### API 入口

| 路由                                 | 方法 | 功能                                      |
| ------------------------------------ | ---- | ----------------------------------------- |
| `/admin/api/user/oauth2/authorize`   | POST | 发起登录或绑定授权                        |
| `/admin/api/user/:provider/callback` | POST | 在 JSON body 中提交 code/state 并完成回调 |
| `/admin/api/user/:provider/callback` | GET  | 历史入口，仅返回 `405 Method Not Allowed` |
| `/admin/api/user/binding`            | POST | 历史浏览器 token 入口，仅返回 `405`       |
| `/admin/api/user/auth-cookie/clear`  | POST | 登录前清除可能残留的 HttpOnly 认证 Cookie |

### 扩展新提供商

1. 在应用配置的 `security` 分组中配置 provider 的 client ID、client secret、redirect URI 和 scope；secret 仅允许服务端读取，不会进入公开 profile 或浏览器缓存

2. 实现对应提供商的用户信息获取逻辑

3. 注册路由处理授权与回调

### 安全配置

- 所有 OAuth2 通信必须使用 HTTPS
- 前端只提交 provider 和 `login` / `binding` 意图；授权 URL 与高熵 state 由服务端生成
- state 仅保存哈希，默认 5 分钟有效，并绑定 provider、意图、浏览器 nonce；绑定流程还绑定当前用户和交互式会话
- callback 在交换 code 或写数据库前原子消费 state，过期、重放或任一绑定不匹配均失败
- callback 页面立即从地址栏清除 code/state，再以 POST body 提交；服务端响应只包含 Admin 会话或绑定完成状态
- provider access/refresh token 不会序列化到浏览器、`localStorage`、URL、Admin JWT、本地密码、provider 错误日志或审计请求 JSON
- OAuth 登录签发的 Admin JWT 只保存在当前页面内存，不进入 `localStorage` 或 `sessionStorage`；刷新或关闭页面后需要重新登录。该临时会话不会接入现有的 query-token WebSocket，通知轮询仍通过 `Authorization` 请求头工作；实时 WebSocket 后续应改为一次性 ticket 或连接后认证
- 活动 GitHub/Lark 绑定通过数据库唯一的 `identity_key` 保证只有一个 Admin owner；MySQL 使用二进制排序规则，与 PostgreSQL/SQLite 一样精确区分 opaque ID 大小写；历史重复绑定会阻止迁移，必须先人工确认并处理
- 单进程开发可使用内存 state store；生产多副本必须配置共享 Redis，不能降级为跨副本绕过校验
- 跨 Origin 部署必须让浏览器携带 credentials，并在 `cors.allowOrigins` 中配置精确的 HTTP(S) Origin；禁止 `*`、userinfo、路径、查询和 fragment
- 开始新的 OAuth 登录前，前端清除旧的非持久会话 bearer，并尽力调用 `/user/auth-cookie/clear` 过期 HttpOnly Cookie，避免旧会话成为回调 principal
- 生产环境必须注入唯一随机、至少 32 字节且不等于公开开发默认值的 `auth.key`；配置不合规时进程拒绝启动，轮换该密钥会使现有 Admin JWT 失效

### 已移除的运行时代码生成器

Admin 中面向浏览器的模板 Generator、相关 `/admin/api/template/*` 路由、OAuth `integration`
意图和短时 credential handle 已全部移除，不再属于受支持的运行时能力，也不得通过恢复旧路由或
provider token 流转重新引入。需要生成模块时，使用开发期的确定性命令
`go run ./cmd/mss module generate modules/<name>/module.yaml`，并把生成结果纳入代码评审。

### 本地密码升级与验证

- OAuth 新建账户默认设置 `local_password_disabled=true`，随机生成的内部密码不能作为本地登录凭据使用
- 历史升级迁移对所有“曾经绑定过 OAuth”的账户采用 fail-closed 策略，包括先本地注册后绑定、后来解绑以及已软删除的历史账户；旧数据无法可靠判断密码是否曾来自 provider token
- 从未有 OAuth 历史的本地账户保持不变；迁移后 OAuth 登录仍可使用
- 用户完成密码重置后会写入新的 hash/salt，并清除 `local_password_disabled`，从而显式恢复本地密码登录
- 上线前必须盘点受影响账户并验证密码重置或管理员辅助恢复路径；这是一项有意的兼容性收紧，不能通过恢复旧 hash 或 provider token 绕过
- `.github/workflows/pat-migration-integration.yml` 在 MySQL 8.4 和 PostgreSQL 17 上运行凭据迁移集成契约，覆盖 PAT、OAuth 绑定/解绑历史、软删除、重复执行幂等、身份唯一性、迁移版本唯一性和密码重置恢复语义

### 历史内置凭据升级

旧版本曾把一组 GitHub OAuth 凭据写入内置 application YAML。当前前向迁移仅通过该历史 client ID 与 secret
组合的 SHA-256 指纹识别旧内置记录：匹配时清空凭据，并把 scope 收窄为 `read:user`、`user:email`；已经轮换、
自定义或非内置的配置不会被覆盖，迁移代码和日志也不会再次包含明文。

该迁移只能清理数据库，不能让 Git 历史中的值失效。升级到生产前必须在 GitHub 侧 rotate/revoke，并同时用
仓库 secret scanner 与 provider 审计确认旧值已不可用；在完成这一步之前不得把版本标记为生产就绪。

## 3. API 联调指南

### 认证方式

| 方式    | Header 格式                         | 适用场景             |
| ------- | ----------------------------------- | -------------------- |
| JWT     | `Authorization: Bearer <jwt_token>` | 用户登录后的前端请求 |
| PAT     | `Authorization: Bearer <pat_token>` | 程序化 API 调用      |
| API Key | `X-API-Key: <key>`                  | 服务间调用（可扩展） |

### Swagger 文档

系统集成了 Swagger 文档：

- 开发环境：`http://localhost:8080/swagger/index.html`
- 可直接在 Swagger UI 中测试 API

### 常见联调问题

#### 401 Unauthorized

- Token 过期或无效
- Header 格式错误
- 用户被禁用

#### 403 Forbidden

- 用户无该 API 的权限
- 角色权限未正确配置
- Casbin 策略未生效

#### CORS 错误

- 前端域名未在 CORS 白名单
- 需配置精确的 `cors.allowOrigins`，并确认响应包含匹配请求 Origin 的 `Access-Control-Allow-Origin` 与 `Access-Control-Allow-Credentials: true`

### 联调检查清单

- [ ] 确认服务已启动且端口可达
- [ ] 确认 Token 有效且未过期
- [ ] 确认用户有对应 API 权限
- [ ] 确认请求路径和方法正确
- [ ] 确认请求体格式符合预期

## 4. 安全审计与日志

### 审计能力维度

| 维度           | 说明                              |
| -------------- | --------------------------------- |
| 登录日志       | 记录登录时间、IP、设备、成功/失败 |
| 操作日志       | 记录关键操作的执行者、时间、参数  |
| API 调用日志   | 记录 API 调用统计（可扩展）       |
| Token 使用日志 | PAT 的使用记录（可扩展）          |

### 当前实现状态

| 能力                | 状态        | 位置                      |
| ------------------- | ----------- | ------------------------- |
| JWT Token 签发/验证 | ✅ 已实现   | `middleware/auth.go`      |
| PAT 管理            | ✅ 不可逆存储与一次展示 | `apis/user_auth_token.go` |
| OAuth2 登录         | ✅ 已实现   | `apis/oauth.go`           |
| 登录日志            | ⚠️ 部分实现 | 需检查审计模块            |
| 操作日志            | ⚠️ 部分实现 | 需检查审计模块            |
| 审计日志查询界面    | 📋 待完善   | 后续迭代                  |

### 建议补强方向

1. **完善登录审计**
   - 记录登录失败原因
   - 异常登录告警（异地、频繁失败）
   - 登录设备管理

2. **增加操作审计**
   - 关键配置变更记录
   - 权限变更审计
   - 敏感数据访问日志

3. **Token 审计**
   - PAT 使用追踪
   - OAuth2 Token 刷新记录
   - Token 泄露检测

## 5. 安全最佳实践

### Token 安全

- JWT 签名密钥定期轮换
- PAT 设置合理过期时间，创建后立即保存，并使用轮换而不是尝试找回旧值
- 敏感操作需要二次验证

### OAuth2 安全

- 使用 HTTPS
- 使用服务端签发且一次性消费的 `state`
- 验证回调 URL
- 不向浏览器或长期任务保存 provider refresh token；需要后台授权时设计独立的服务端凭据生命周期

### API 安全

- 所有管理 API 需要 JWT/PAT 认证
- 敏感 API 考虑限流
- 避免在 URL 中传递敏感参数
- 返回数据脱敏处理

## 推荐阅读

- [登录排障](/admin/login-troubleshooting)
- [权限与组织治理说明](/admin/governance-guide)
- [运营能力说明](/admin/operations-guide)
- [产品方向调整](/admin/product-direction)
- [当前功能总览](/admin/current-capabilities)
- [三期路线图](/admin/phase-3-roadmap)
