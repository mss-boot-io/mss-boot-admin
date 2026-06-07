# 在线会话与强制下线（Online Sessions & Force Logout）设计

> 范围：`mss-boot-admin` Phase 1 后端（issue #373）。前端 UI 单独 PR。
>
> 适用代码版本：本文件随 `feature/373-online-sessions` 分支提交（2026-06-07）。

## 1. 背景与动机

当前 JWT 在签发后到自然过期前无法在服务端吊销，遇到账号异常时无法立刻断开访问。issue #373 要求：

- 管理员可见到当前所有活跃登录会话。
- 管理员可强制下线指定会话或指定用户的全部会话。
- 普通用户的"登出"也需要让服务端 token 失效，而非仅清理前端 Cookie。
- 强制下线、登出等敏感操作需要进入审计日志。
- 现有 PAT（Personal Access Token，`mss_boot_user_auth_token`）保持原状，不并入"在线会话"列表。

## 2. 决策汇总（已和 issue requester 对齐）

| 议题 | 决策 |
|------|------|
| 上线时旧 JWT 如何处理 | 上线即失效，强制全部用户重新登录 |
| 强制登出粒度 | 默认踢选中 session，并提供"踢该用户全部"按钮 |
| 与 PAT 关系 | 分离——在线会话只含普通登录，PAT 维持现有 `user-auth-tokens` 页 |
| 历史 session 保留 | active 长期保留；revoked/expired 满 30 天后清理 |
| 存储 | Redis + DB 双存，Redis 负责高频 lookup |
| sid 承载位置 | JWT claims 新增 `sid` 字段 |

## 3. 架构总览

登录成功后，服务端创建一条 `UserSession`，把 `sid` 写入 JWT。每次受保护请求由 auth 中间件做 sid 校验：

```
登录 ──► Authenticator 通过验证
       └─► PayloadFunc 调用 service.Session.Create
             ├─► DB: 插入 mss_boot_user_sessions
             └─► Redis: SET mss:session:{sid} + SADD mss:session:user:{uid}
       └─► JWT claims = { verifier, sid }

受保护请求 ──► IdentityHandler
              └─► service.Session.Lookup(sid)
                    ├─► Redis hit → active
                    └─► miss → DB → 回填 Redis
              ├─► active：放行 + 异步 Touch(last_seen_at, 60s 节流)
              └─► revoked/expired/missing：返回 nil，被中间件拒绝

强制下线 ──► UPDATE mss_boot_user_sessions SET revoked=true ...
          └─► DEL mss:session:{sid}  /  按 user 批量删
          └─► audit_log 写一条 type=security 的记录

清理 ──► 每天 03:30 cron 删除 30 天前的 revoked/expired 行
```

PAT 走原 `personAccessToken` 分支，**不**进入 sid 校验路径，行为完全不变。

## 4. 数据模型

### 4.1 `mss_boot_user_sessions`

继承 `ModelGormTenant`，主键沿用 `pkg.SimpleID()`。

| 字段 | 类型 | 含义 |
|------|------|------|
| `user_id` | varchar(64) index | 登录用户ID |
| `username` | varchar(255) | 登录用户名（冗余便于列表展示） |
| `role_id` | varchar(64) | 角色ID |
| `login_at` | datetime index | 登录时间 |
| `last_seen_at` | datetime | 最后活跃时间（节流刷新） |
| `expired_at` | datetime index | 过期时间 |
| `ip` | varchar(50) | 登录 IP |
| `user_agent` | varchar(500) | UA |
| `revoked` | bool index | 是否吊销 |
| `revoked_at` | datetime | 吊销时间 |
| `revoked_by` | varchar(64) | 操作者 user_id |
| `revoke_reason` | varchar(32) | logout / force-by-session / force-by-user |

> 自然过期不写回 `revoked=true`：Lookup 时按 `expired_at < now` 即时判定，避免与"管理员显式吊销"语义混淆，也保持 List 三态过滤简单。已过期的行由 30 天 cleanup cron 硬删。

Migration：`cmd/migrate/migration/system/20260607162057_user_sessions.go`。

### 4.2 Redis Key 约定

| Key | 类型 | 内容 | TTL |
|-----|------|------|-----|
| `mss:session:{sid}` | string | `{userID, roleID, exp}` JSON | session TTL |
| `mss:session:user:{uid}` | set | 该用户所有活跃 sid | session TTL + 1h |
| `mss:session:seen:{sid}` | string | last_seen 节流哨兵 | 60s |

封装在 `pkg/sessioncache`，全部经 `Cache.Set/Get/Del/DelByUser/TryTouch` 走，禁止直接拼 key。

## 5. 模块边界

| 包 | 职责 |
|----|------|
| `models.UserSession` | GORM 实体 + 状态枚举 |
| `pkg/sessioncache` | Redis 操作的薄封装，Redis 不可用时静默退化（cli==nil 直接 short-circuit） |
| `service.SessionService` | Create / Lookup / Touch / RevokeBySID / RevokeByUserID / CleanupOlderThan |
| `middleware/auth` | PayloadFunc 写 sid；IdentityHandler 校验 sid；RefreshResponse 检查 sid |
| `apis.OnlineSessionAPI` | REST 端点 |
| `service.Audit.LogSecurity` | 强制下线/自登出的审计落地 |

## 6. API

挂在 `/admin/api/`，统一通过 `response.AuthHandler` 鉴权，按 Casbin 控制。

| Method | Path | 说明 |
|--------|------|------|
| GET | `/online-sessions?status=active\|revoked\|expired&userID=&username=&ip=&current=&pageSize=` | 列表 |
| GET | `/online-sessions/:id` | 详情 |
| DELETE | `/online-sessions/:id` | 强制下线指定会话（写审计） |
| DELETE | `/online-sessions/user/:userID` | 强制下线该用户全部会话（写审计） |
| POST | `/online-sessions/logout` | 当前会话自登出（吊销本次请求的 sid） |

控制器沿用 `controller.NewSimple + GetAction(_) returns nil + Other(...)` 模式（参考 `apis/user_auth_token.go`），并附 Swagger 注解。

## 7. Auth 中间件改造要点

`config.Auth` 新增 `SessionEnabled bool`：

- `false`（默认，向后兼容）→ 老行为完全不变。
- `true` → 启用本特性，旧 JWT（无 sid claim）立即失效。

`gin-jwt/v2.PayloadFunc` 没有 `*gin.Context` 入参，本期采用 **goroutine-local context bag**：`Authenticator` 成功路径 Store `*gin.Context`，`PayloadFunc` Load+Clear。备选方案（改造 `Authenticator` 返回 `{Verifier, GinCtx}`）改动面更大，未采用。

PAT 分支（`personAccessToken != ""`）跳过 sid 校验，行为不变。

## 8. 审计联动

强制下线（by-session / by-user）以及自登出走 `service.Audit.LogSecurity`，落 `mss_boot_audit_logs`，`type=security`，`action ∈ {force_logout, logout}`，`resource = session:{sid}` 或 `user:{uid}`。

## 9. 清理

`cmd/server/server.go` 在 `task.WithSchedule(...)` 列表里追加一项：

```go
task.WithSchedule("session-cleanup", "0 30 3 * * *", taskSessionCleanup{})
```

每天 03:30 调用 `service.Session.CleanupOlderThan(ctx, gormdb.DB, 30*24*time.Hour)`，硬删 30 天前的 revoked/expired 行。

## 10. 兼容性与回滚

- 默认 `SessionEnabled=false`，发布即不影响任何现有行为。
- 灰度打开后旧 JWT 因缺 `sid` 立即失效。如需回滚，关闭 flag 即可恢复旧鉴权路径；DB/Redis 数据保留，不需要 down migration。
- PAT 路径完全独立，不参与本特性。

## 11. 测试覆盖

| 用例 | 位置 |
|------|------|
| Redis Set/Get/Del/DelByUser/TryTouch | `pkg/sessioncache/cache_test.go`（miniredis） |
| Create / Lookup（cache hit / cache miss + DB 回填 / revoked / expired / missing） | `service/session_test.go` |
| RevokeBySID / RevokeByUserID | `service/session_test.go` |
| CleanupOlderThan | `service/session_test.go` |
| LogSecurity | `service/audit_test.go` |
| List / RevokeBySID / RevokeByUserID（HTTP） | `apis/online_session_test.go` |

`go test ./pkg/sessioncache/... ./service/... ./apis/...` 全部 PASS。

## 12. 不在本期范围

- 前端 UI（单独 PR 跟进）
- 设备指纹 / 异常登录检测
- WebSocket 主动通知客户端"已被踢"（前端轮询 401 即可达到目的）
- PAT 视图与在线会话视图合并
