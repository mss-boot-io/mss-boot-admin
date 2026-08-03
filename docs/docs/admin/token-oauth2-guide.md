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

### 数据模型

```
PersonalAccessToken
├── UserID      → 所属用户
├── Name        → 令牌名称/用途说明
├── Token       → 令牌值 (加密存储)
├── ExpiresAt   → 过期时间
├── LastUsedAt  → 最后使用时间
├── Status      → 启用状态
├── CreatedAt
```

### API 入口

| 路径                                           | 方法   | 功能     |
| ---------------------------------------------- | ------ | -------- |
| `/admin/api/personal-access-token`             | GET    | 令牌列表 |
| `/admin/api/personal-access-token`             | POST   | 创建令牌 |
| `/admin/api/personal-access-token/:id`         | DELETE | 撤销令牌 |
| `/admin/api/personal-access-token/refresh/:id` | POST   | 刷新令牌 |

### 使用方式

**创建令牌**

```bash
POST /admin/api/personal-access-token
{
  "name": "CI Pipeline Token",
  "expiresAt": "2025-12-31T23:59:59Z"
}

Response:
{
  "id": 1,
  "name": "CI Pipeline Token",
  "token": "pat_xxxxxxxxxxxxx",  // 仅创建时返回一次
  "expiresAt": "2025-12-31T23:59:59Z"
}
```

**使用令牌调用 API**

```bash
curl -H "Authorization: Bearer pat_xxxxxxxxxxxxx" \
     https://admin.example.com/api/user/info
```

### 安全建议

- 令牌创建后仅显示一次，需妥善保存
- 定期轮换令牌
- 为不同用途创建独立令牌
- 不再使用的令牌及时撤销

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

| 路由                       | 功能             |
| -------------------------- | ---------------- |
| `/auth/:provider`          | 发起 OAuth2 授权 |
| `/auth/:provider/callback` | OAuth2 回调处理  |

### 扩展新提供商

1. 在 `config/application.yml` 中配置提供商参数：

```yaml
oauth2:
  github:
    client_id: 'your-client-id'
    client_secret: 'your-client-secret'
    redirect_url: 'https://your-domain/auth/github/callback'
  lark:
    client_id: 'your-app-id'
    client_secret: 'your-app-secret'
    redirect_url: 'https://your-domain/auth/lark/callback'
```

2. 实现对应提供商的用户信息获取逻辑

3. 注册路由处理授权与回调

### 安全配置

- 所有 OAuth2 通信必须使用 HTTPS
- 配置 `state` 参数防止 CSRF 攻击
- Token 存储应考虑加密

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
- 需配置 `server.cors` 参数

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

| 能力                | 状态        | 位置                            |
| ------------------- | ----------- | ------------------------------- |
| JWT Token 签发/验证 | ✅ 已实现   | `middleware/auth.go`            |
| PAT 管理            | ✅ 已实现   | `apis/personal_access_token.go` |
| OAuth2 登录         | ✅ 已实现   | `apis/oauth2.go`                |
| 登录日志            | ⚠️ 部分实现 | 需检查审计模块                  |
| 操作日志            | ⚠️ 部分实现 | 需检查审计模块                  |
| 审计日志查询界面    | 📋 待完善   | 后续迭代                        |

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
- PAT 设置合理过期时间
- 敏感操作需要二次验证

### OAuth2 安全

- 使用 HTTPS
- 配置 `state` 参数
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
