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

- 用户可自行创建、刷新、撤销令牌
- 令牌用于替代用户名密码调用 API
- 支持细粒度权限控制（可扩展）

### 当前数据模型

```
UserAuthToken
├── ID          → PAT 记录 ID
├── UserID      → 所属用户
├── Token       → 完整 JWT 文本（当前兼容实现）
├── ExpiredAt   → 过期时间
├── Revoked     → 是否已撤销
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
  "expiredAt": "<timestamp>"
}
```

**使用令牌调用 API**

```bash
curl -H "Authorization: Bearer <jwt-text>" \
     https://admin.example.com/api/user/info
```

### 安全建议

- PAT 不能创建、列出、刷新或撤销 PAT，也不能执行密码重置、账户恢复标识修改或 OAuth2 绑定/解绑；这些交互式操作统一返回 `403 Forbidden`
- 当前实现仍持久化并在列表返回完整 JWT 文本，尚未达到“一次展示、不可逆存储、scope 与使用追踪”的生产级生命周期要求
- 在完成令牌不可逆存储和升级迁移前，不要把当前 PAT 用作生产自动化的长期凭证
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
├── ProviderID  → 提供商侧用户 ID
├── AccessToken → OAuth2 Token
├── RefreshToken
├── ExpiresAt
├── CreatedAt
```

### 登录流程

```
用户点击第三方登录图标
    ↓
跳转到 OAuth2 授权页面
    ↓
用户授权后回调到系统
    ↓
系统获取 OAuth2 Token
    ↓
查询是否已绑定系统用户
    ├── 已绑定 → 直接登录，签发 JWT
    └── 未绑定 → 创建新用户并绑定，或关联已有账户
    ↓
跳转到系统首页
```

### API 入口

| 路由                                 | 方法 | 功能                                      |
| ------------------------------------ | ---- | ----------------------------------------- |
| `/admin/api/user/oauth2/authorize`   | POST | 发起登录或绑定授权                      |
| `/admin/api/user/:provider/callback` | GET  | OAuth2 回调处理                           |

### 扩展新提供商

1. 在应用配置的 `security` 分组中配置 provider 的 client ID、client secret、redirect URI 和 scope；secret 仅允许服务端读取，不会进入公开 profile 或浏览器缓存

2. 实现对应提供商的用户信息获取逻辑

3. 注册路由处理授权与回调

### 安全配置

- 所有 OAuth2 通信必须使用 HTTPS
- 前端只提交 provider 和 `login` / `binding` 意图；授权 URL 与高熵 state 由服务端生成
- state 仅保存哈希，默认 5 分钟有效，并绑定 provider、意图、浏览器 nonce；绑定流程还绑定当前用户和交互式会话
- callback 在交换 code 或写数据库前原子消费 state，过期、重放或任一绑定不匹配均失败
- 单进程开发可使用内存 state store；生产多副本必须配置共享 Redis，不能降级为跨副本绕过校验
- 跨 Origin 部署必须让浏览器携带 credentials，并在 `cors.allowOrigins` 中配置精确的 HTTP(S) Origin；禁止 `*`、userinfo、路径、查询和 fragment
- provider access/refresh token 的浏览器持久化和服务端加密仍需单独完成安全升级

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
| PAT 管理            | ⚠️ 兼容实现 | `apis/user_auth_token.go` |
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
- PAT 设置合理过期时间；完成不可逆存储升级前不要作为生产长期凭证
- 敏感操作需要二次验证

### OAuth2 安全

- 使用 HTTPS
- 使用服务端签发且一次性消费的 `state`
- 验证回调 URL
- 及时刷新过期 Token

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
