---
title: 集成与扩展护栏
order: 23
nav:
  order: 1
  title: admin
description: mss-boot-admin 扩展能力边界、接入规范与治理要求
keywords: [admin extension guardrails i18n storage websocket api]
---

## 概述

本文档定义 `mss-boot-admin` 扩展能力的护栏边界，确保新能力接入时：

- 不绕开治理与运营主线
- 遵守统一的扩展规范
- 便于被评审、验证和文档化

## 覆盖范围

P4 覆盖四类扩展能力：

| 能力 | 说明 | 状态 |
|------|------|------|
| 国际化 (i18n) | 多语言资源管理 | ✅ 已实现 |
| 对象存储/上传 | 文件上传与存储 | ⚠️ Legacy / Blocked |
| WebSocket 事件 | 实时通信与推送 | ✅ 已实现 |
| API-first 扩展 | 显式 Go 模型与标准控制器 | ✅ 已实现 |

---

## 1. 国际化 (i18n)

### 1.1 当前实现

**后端：**
- 模型：`models/language.go` - Language 模型，支持多语言配置
- API：`apis/language.go` - CRUD + `/language/profile` 接口
- 存储：JSON 格式存储翻译数据，Redis 缓存支持

**前端：**
- 框架：Umi.js 内置 i18n 插件
- 静态资源：`src/locales/zh-CN.ts`、`src/locales/en-US.ts`
- 动态加载：从后端 API 获取自定义翻译
- 切换组件：`SelectLang` 语言选择器

**实现文件：**
```
mss-boot-admin/
├── admin/models/language.go
├── admin/apis/language.go
├── admin/service/language.go
└── web/antd-v6/
    ├── src/locales/zh-CN.ts
    ├── src/locales/en-US.ts
    ├── config/config.ts
    └── src/app.tsx
```

### 1.2 扩展边界

| 边界 | 规则 |
|------|------|
| 语言代码 | 必须遵循 ISO 639-1 标准（如 zh-CN, en-US） |
| 静态翻译 | 核心 UI 文本放在前端 `locales/` 目录 |
| 动态翻译 | 可管理内容通过后端 Language API |
| 缓存策略 | 使用 Redis 缓存，变更时自动失效 |

### 1.3 治理集成

| 要求 | 当前状态 | 建议 |
|------|----------|------|
| 权限控制 | ✅ 有菜单权限 | 可增加语言级别细粒度权限 |
| 租户隔离 | ❌ 已移除 | 单租户架构，无需隔离 |
| 审计日志 | ❌ 未实现 | 建议增加语言变更审计 |
| Accept-Language | ❌ 未处理 | 建议中间件处理请求头 |

### 1.4 接入规范

**添加新语言：**

1. 前端添加语言文件：
```typescript
// src/locales/ja-JP.ts
export default {
  'menu.home': 'ホーム',
  'menu.dashboard': 'ダッシュボード',
  // ...
};
```

2. 配置 Umi i18n：
```typescript
// config/config.ts
locale: {
  default: 'zh-CN',
  baseNavigator: true,
  locales: [
    { name: '简体中文', value: 'zh-CN' },
    { name: 'English', value: 'en-US' },
    { name: '日本語', value: 'ja-JP' },  // 新增
  ],
},
```

3. 后端创建语言记录（可选）：
```bash
POST /admin/api/languages
{
  "name": "日本語",
  "status": "enabled",
  "defines": [...]
}
```

### 1.5 当前限制

1. **无细粒度权限**：无法按语言或翻译组分配权限
2. **无 Accept-Language 处理**：不支持自动内容协商
3. **缓存边缘情况**：分布式部署可能存在缓存一致性问题
4. **缺少语言代码校验**：未强制 ISO 639-1 格式

---

## 2. 对象存储/上传

### 2.1 当前证据边界

| 路径 | 当前成熟度 | 已证明与未证明 |
|------|------------|----------------|
| Local | Legacy / Blocked | D0 已证明 admission、opaque key、`os.Root` confinement、`O_EXCL` no-clobber 与 partial cleanup；D1 已证明 strict profile、single owner、同一 pinned `os.Root` 的写入/StaticFS、零 fallback 与 dev-only exact static delivery；生产 Delivery、metadata/authorization 与 common conformance 未证明 |
| S3-compatible | Legacy / Blocked | D1 已证明 immutable profile、完整 SecretRef credential mode、single client owner 与零 fallback；Admin 在 `Put` 前返回 503，create-only/checksum/Delivery/RustFS conformance 仍未实现 |

严格配置只接受 Local 或 S3 分支。S3-compatible 产品只能通过显式 endpoint/path-style
进入同一分支，这不等于 OSS、COS、OBS、MinIO、RustFS、GCS、KODO 或 BOS 已有生产支持矩阵。

**实现文件：**
```
mss-boot-admin/
└── admin/
    ├── apis/storage.go               # 上传 API 与固定 503
    ├── cmd/server/object_storage.go  # 启动安装与 dev Local Delivery
    ├── config/object_storage.go      # 单一应用 owner
    └── service/storage.go            # admission 与存储逻辑

mss-boot/pkg/config/
└── storage.go           # 存储配置
```

上传 admission 的非密钥 AppConfig：

| key | 合同 |
| --- | --- |
| `storage:maxSize` | bytes；默认 10 MiB（`10485760`）；硬上限 100 MiB（`104857600`） |
| `storage:allowedTypes` | 逗号分隔 MIME types / `type/*` wildcards，例如 `image/png,image/*`；不是扩展名列表 |

这是 Storage AppConfig 的完整 allowlist。provider、endpoint、region、bucket、TLS、
credential source 和凭据材料既不会投影，也不能经此 API 写入；旧 key 的写入请求
整批返回稳定 422。Provider 与 SecretRef 只允许来自进程启动时的不可变 profile，
本检查点已完成其 fail-closed 解析和单一生命周期 owner 接线。

### 2.2 扩展边界

| 边界 | 规则 |
|------|------|
| 物理 key | 服务端生成 `uploads/<opaque-uuid>`；用户 ID 与原始文件名不得进入 key，原始文件名仅作响应元数据 |
| 写入边界 | multipart 前限制 body，流式 max-plus-one；Local 在受限根中 create-only 写入并清理 partial |
| 配置来源 | AppConfig 只读取 `maxSize` / `allowedTypes`；Provider / SecretRef 只来自一次性启动 profile；未知/非法 profile 拒绝安装对象资源、应用继续运行且上传固定 503，运行时切换不受支持 |
| 认证要求 | 必须通过当前有效身份认证；通用上传还需 `storage:upload` 权限 |
| URL / Delivery | Local 只有在 dev `staticPath` 精确映射配置 root 时才返回实际可读 URL；生产 Local 不安装；S3 不拼接 URL，并在 `Put` 前返回 503 |

严格启动 profile 与 single owner 只关闭 D1 的对象子切片。真实 S3 Delivery、
RustFS fixture 和 Local/S3-compatible 共用 conformance suite 仍留在
`D4-authorization-object`；Kafka lifecycle 仍是 D1 的未完成部分。在这些门禁关闭前，
provider 能力仍是 `Legacy / Blocked`。
精确失败语义与测试命令见
[D1 Object Provider/Owner 内部 checkpoint](/releases/v1-1-0-d1-object-provider-owner)。

### 2.3 治理集成

| 要求 | 当前状态 | 建议 |
|------|----------|------|
| 当前身份认证 | ✅ 已实现 | 浏览器使用 HttpOnly 会话；API 自动化使用文档化的 PAT/Bearer |
| 对象所有权/用户隔离 | ❌ 未实现 | opaque key 不是授权；在 Delivery/metadata 边界补 owner 与反向授权测试 |
| 上传入口权限 | ⚠️ 部分实现 | 通用上传使用 `storage:upload`；头像为已认证本人接口，但不等于对象读取授权 |
| 租户隔离 | ❌ 已移除 | 单租户架构 |
| 审计日志 | ❌ 未形成对象审计合同 | 后续记录操作者、opaque ref 与结果，不记录 multipart 内容 |
| 文件校验 | ⚠️ `D0-safety` 内部检查点 | MIME/wildcard allowlist；默认 10 MiB、硬上限 100 MiB，单位均为 bytes |

### 2.4 接入规范

**上传文件：**
```bash
POST /admin/api/storage/upload
Authorization: Bearer <具有 storage:upload 权限的 PAT>
Content-Type: multipart/form-data

file: <binary>
```

**响应：**
```json
{
  "url": "/public/uploads/<opaque-uuid>",
  "filename": "<original-name>",
  "size": 1234,
  "mimeType": "image/png"
}
```

该响应仅说明显式 dev Local 路径的返回形状；只有启动配置把同一绝对 root 映射为
`/public` 时才会返回并实际提供该 URL。生产模式返回 503，URL 也不能替代鉴权
Delivery。原始 `filename` 不是存储 key。

### 2.5 当前限制

1. **Local/S3-compatible 仍为 Legacy / Blocked**：不得作为生产可用 provider 宣传
2. **D1 对象子切片已收敛**：strict profile、single owner、AppConfig 移除与 fail-closed 503 已完成；Kafka lifecycle 仍未完成
3. **Delivery 与对象所有权未实现**：`prod` 模式不安装 Local，opaque key 本身也不是授权
4. **S3 I/O 尚未开放**：Put、conditional create-only 与 RustFS 共用 provider conformance 留在 `D4-authorization-object`
5. **无配额、速率限制和大文件协议**：当前无分片上传，且配置硬上限为 100 MiB

---

## 3. WebSocket 事件能力

### 3.1 当前实现

**支持的事件类型：**
| 事件 | 说明 |
|------|------|
| `ping`/`pong` | 心跳保活 |
| `notify` | 通知推送 |
| `kick` | 强制下线 |
| `join`/`quit` | 连接管理 |
| `connected` | 连接成功 |

**实现文件：**
```
mss-boot-admin/
├── apis/ws.go                    # API 路由
└── center/websocket/
    ├── manager.go                # Hub 管理
    ├── client.go                 # 客户端连接
    └── handler.go                # 事件处理
```

**API 端点：**
| 路径 | 说明 |
|------|------|
| `/admin/api/ws/connect` | WebSocket 连接（需认证） |
| `/admin/api/ws/online` | 在线用户统计 |

### 3.2 扩展边界

| 边界 | 规则 |
|------|------|
| 事件类型 | 使用预定义 `EventType`，新事件需评审 |
| 消息大小 | 最大 512KB (`maxMessageSize`) |
| 发送缓冲 | 100 条消息 (`sendBufferSize`) |
| 心跳超时 | 5 分钟无心跳断开连接 |
| 认证 | 已认证浏览器会话先通过 `POST /admin/api/ws/tickets` 领取短期一次性 ticket；握手只经 `Sec-WebSocket-Protocol` 传递 |
| Origin | ticket 绑定可信 Origin 和当前服务端会话；URL 参数、Cookie JWT 与首条消息 Token 均不接受 |

### 3.3 治理集成

| 要求 | 当前状态 | 建议 |
|------|----------|------|
| 会话与一次性 ticket | ✅ 已实现 | ticket 单次消费，浏览器脚本不读取 Admin JWT |
| 用户路由 | ✅ 按用户ID推送 | - |
| 在线统计 | ✅ 已实现 | - |
| 事件审计 | ❌ 未实现 | 建议记录关键事件 |
| 权限控制 | ❌ 未实现 | 可增加事件级别权限 |

### 3.4 接入规范

**连接 WebSocket：**

V6 的权威实现位于 `web/antd-v6/src/shared/realtime/socket.ts`。调用方必须先使用
同源 HttpOnly 会话和 CSRF 保护的 POST 请求领取 ticket，再把应用协议与 ticket 协议一起
交给浏览器 WebSocket API；不得把凭证放进 URL 或业务消息：

```javascript
import { connectRealtimeSocket } from '@/shared/realtime/socket';

const ws = await connectRealtimeSocket();

// 接收消息
ws.onmessage = (event) => {
  const msg = JSON.parse(event.data);
  // msg.event: 'ping' | 'notify' | 'kick' | ...
  // msg.data: 具体数据
  // msg.code: 状态码
  // msg.timestamp: 时间戳
};

// 心跳响应
ws.onmessage = (event) => {
  const msg = JSON.parse(event.data);
  if (msg.event === 'ping') {
    ws.send(JSON.stringify({ event: 'pong' }));
  }
};
```

**服务端推送：**
```go
// 推送给指定用户
websocket.GetHub().SendToUser(userID, &websocket.WResponse{
    Event: websocket.EventNotify,
    Code:  200,
    Data: map[string]interface{}{
        "title": "系统通知",
        "content": "您有新的消息",
    },
})

// 广播给所有用户
websocket.GetHub().Broadcast(&websocket.WResponse{
    Event: websocket.EventNotify,
    Code:  200,
    Data:  notification,
})
```

### 3.5 当前限制

1. **无事件扩展机制**：事件类型硬编码，难以动态扩展
2. **无自动重连**：断线后需客户端自行重连
3. **消息可靠性有限**：缓冲区满时丢弃消息
4. **无集群支持**：单机模式，不支持分布式部署
5. **无消息持久化**：离线消息不存储

---

## 4. API-first 扩展方式

### 4.1 当前实现

**扩展机制：**
| 机制 | 说明 |
|------|------|
| Controller.Simple | 标准 CRUD 控制器 |
| Hook 机制 | Before/After 扩展点 |
| 自动路由 | 通过 `AppendController` 注册 |

**实现文件：**
```
mss-boot/
├── pkg/response/controller/
│   ├── simple.go        # Simple 控制器
│   └── controller.go    # 扩展点

mss-boot-admin/
├── apis/                # 显式 API 控制器
└── models/              # 受版本控制的 Go 模型
```

### 4.2 扩展边界

| 边界 | 规则 |
|------|------|
| 控制器注册 | 必须通过 `response.AppendController()` |
| 模型定义 | 必须实现 `schema.Tabler` 接口 |
| 认证控制 | 使用 `controller.WithAuth(true/false)` |
| Hook 注入 | 使用 `controller.WithBefore*/After*` 选项 |

### 4.3 治理集成

| 要求 | 当前状态 | 建议 |
|------|----------|------|
| 认证控制 | ✅ WithAuth 选项 | - |
| 权限控制 | ✅ Casbin 集成 | - |
| 数据权限 | ✅ WithScope 支持 | - |
| Swagger 文档 | ⚠️ 需手动注解 | 建议自动生成 |
| 审计日志 | ✅ 可通过 Hook 实现 | - |

### 4.4 接入规范

**创建标准控制器：**
```go
func init() {
    e := &MyResource{
        Simple: controller.NewSimple(
            controller.WithAuth(true),
            controller.WithModel(&models.MyModel{}),
            controller.WithSearch(&dto.MySearch{}),
            controller.WithModelProvider(actions.ModelProviderGorm),
            // 可选 Hook
            controller.WithBeforeCreate(beforeCreate),
            controller.WithAfterCreate(afterCreate),
        ),
    }
    response.AppendController(e)
}

type MyResource struct {
    *controller.Simple
}

// 可选：自定义路由
func (e *MyResource) Other(r *gin.RouterGroup) {
    r.GET("/my-resource/custom", e.CustomHandler)
}
```

Admin 运行时虚拟模型和动态 API 已移除。标准化模块可以先通过
`mss module generate` 在开发期从受版本控制的规格离线生成，再将生成结果作为
普通源码审查、测试和部署；该命令不会在生产运行时动态挂载路由。

**Hook 扩展点：**
```go
// Before 钩子 - 可修改数据或验证
func beforeCreate(c *gin.Context, db *gorm.DB, m schema.Tabler) error {
    // 业务逻辑
    return nil
}

// After 钩子 - 可触发副作用
func afterCreate(c *gin.Context, db *gorm.DB, m schema.Tabler) error {
    // 如发送通知、更新统计等
    return nil
}
```

### 4.5 当前限制

1. **Swagger 注解缺失**：需手动补充 API 文档
2. **Hook 机制有限**：缺少更细粒度的扩展点
3. **权限粒度**：认证控制较粗，缺少字段级别权限

---

## 5. 扩展能力总览

### 5.1 能力矩阵

| 能力 | 实现完整度 | 治理集成度 | 扩展灵活性 | 安全性 |
|------|-----------|-----------|-----------|--------|
| 国际化 | 高 | 中 | 中 | 中 |
| 对象存储 | Legacy / Blocked | 低 | 低 | 低 |
| WebSocket | 高 | 中 | 低 | 中 |
| API-first | 高 | 高 | 高 | 高 |

### 5.2 治理要求对照

| 要求 | i18n | Storage | WebSocket | API-first |
|------|------|---------|-----------|-----------|
| 当前身份认证 | ✅ | ✅ | ✅（会话 + ticket） | ✅ |
| Casbin 权限 | ✅ | ⚠️ 仅通用上传入口 | ❌ | ✅ |
| 租户隔离 | ❌ | ❌ | ❌ | ❌ |
| 审计日志 | ❌ | ❌ | ❌ | ⚠️ |
| 数据权限 | N/A | ❌ | N/A | ✅ |

### 5.3 改进优先级

**高优先级（安全相关）：**
1. 保持已完成的 D1 object fail-closed、immutable profile 与 single owner 哨兵
2. 完成 D1 Kafka lifecycle；在 D4 Provider/Delivery 门禁通过前保持生产上传关闭

**中优先级（治理完善）：**
1. `D4-authorization-object` 完成 S3 conditional create-only、共用 conformance 与 Delivery 授权
2. 存储审计日志
3. WebSocket 事件审计
4. 国际化审计日志
5. WebSocket 集群支持

**低优先级（体验优化）：**
1. 存储大文件分片上传
2. WebSocket 消息持久化
3. 国际化 Accept-Language 处理
4. API Swagger 自动生成

---

## 6. 新能力接入检查清单

当需要新增扩展能力时，必须完成以下检查：

### 6.1 治理集成

- [ ] 浏览器是否使用 HttpOnly 会话、API 自动化是否使用文档化 PAT，并禁止把凭证放进 URL？
- [ ] 是否需要 Casbin 权限控制？
- [ ] 是否需要数据权限隔离？
- [x] ~~是否需要租户隔离？~~ 已移除多租户功能

### 6.2 安全检查

- [ ] 输入验证是否完整？
- [ ] 权限边界是否清晰？
- [ ] 敏感数据是否脱敏？
- [ ] 操作是否可审计？

### 6.3 文档要求

- [ ] API 文档是否完整？
- [ ] 接入指南是否清晰？
- [ ] 限制说明是否明确？
- [ ] 示例代码是否提供？

### 6.4 测试覆盖

- [ ] 单元测试是否覆盖？
- [ ] E2E 测试是否覆盖？
- [ ] 边界条件是否测试？
- [ ] 权限控制是否验证？

---

## 7. 推荐阅读

- [权限与组织治理说明](/admin/governance-guide)
- [运营能力说明](/admin/operations-guide)
- [集成测试指南](/admin/integration-test-guide)
- [Token 与 OAuth2 联调说明](/admin/token-oauth2-guide)
- [三期路线图](/admin/phase-3-roadmap)
