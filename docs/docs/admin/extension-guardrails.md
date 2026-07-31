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
| 对象存储/上传 | 文件上传与存储 | ✅ 已实现 |
| WebSocket 事件 | 实时通信与推送 | ✅ 已实现 |
| API-first 扩展 | 动态模型与控制器 | ✅ 已实现 |

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
├── models/language.go
├── apis/language.go
└── service/language.go

mss-boot-admin-antd/
├── src/locales/
│   ├── zh-CN.ts
│   ├── en-US.ts
│   └── zh-CN/menu.ts
├── config/config.ts  # i18n 配置
└── src/app.tsx       # 动态加载语言
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

### 2.1 当前实现

**支持的后端：**
| 类型 | 说明 |
|------|------|
| Local | 本地文件系统 (`public/{userID}/`) |
| S3 | AWS S3 |
| OSS | 阿里云对象存储 |
| COS | 腾讯云对象存储 |
| OBS | 华为云对象存储 |
| MinIO | 私有化部署 S3 兼容存储 |
| GCS | Google Cloud Storage |
| KODO | 七牛云存储 |
| BOS | 百度对象存储 |

**实现文件：**
```
mss-boot-admin/
├── apis/storage.go      # 上传 API
└── service/storage.go   # 存储逻辑

mss-boot/pkg/config/
└── storage.go           # 存储配置
```

**配置方式：**
```yaml
# 通过 app_config 表配置
storage:
  type: s3              # local 或 s3
  endpoint: https://...
  s3Region: us-east-1
  s3Bucket: my-bucket
  s3AccessKeyID: xxx
  s3SecretAccessKey: xxx
```

### 2.2 扩展边界

| 边界 | 规则 |
|------|------|
| 存储路径 | 必须使用 `{userID}/` 前缀隔离 |
| 访问方式 | 通过 `/storage/upload` API，禁止直接文件系统访问 |
| 配置来源 | 从 `app_config` 表读取，支持运行时切换 |
| 认证要求 | 必须通过 JWT 认证 |

### 2.3 治理集成

| 要求 | 当前状态 | 建议 |
|------|----------|------|
| JWT 认证 | ✅ 已实现 | - |
| 用户隔离 | ✅ 按用户ID存储 | - |
| 细粒度权限 | ❌ 未实现 | 建议增加 Casbin 存储权限 |
| 租户隔离 | ❌ 已移除 | 单租户架构 |
| 审计日志 | ❌ 未实现 | 建议记录上传/删除操作 |
| 文件校验 | ❌ 未实现 | **必须**增加类型/大小限制 |

### 2.4 接入规范

**上传文件：**
```bash
POST /admin/api/storage/upload
Authorization: Bearer <token>
Content-Type: multipart/form-data

file: <binary>
```

**响应：**
```json
{
  "url": "https://endpoint/userID/filename"
}
```

**安全要求（建议实现）：**

```go
// 文件类型白名单
var allowedTypes = []string{
    "image/jpeg", "image/png", "image/gif",
    "application/pdf",
    "text/plain",
}

// 文件大小限制
const maxSize = 10 * 1024 * 1024 // 10MB
```

### 2.5 当前限制

1. **无文件校验**：缺少类型/大小限制，存在安全风险
2. **无细粒度权限**：所有认证用户均可上传
3. **单租户架构**：当前为单租户模式
4. **无审计日志**：无法追溯文件操作
5. **无大文件支持**：缺少分片上传能力

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
| 认证 | 连接前必须通过 JWT 认证 |

### 3.3 治理集成

| 要求 | 当前状态 | 建议 |
|------|----------|------|
| JWT 认证 | ✅ 已实现 | - |
| 用户路由 | ✅ 按用户ID推送 | - |
| 在线统计 | ✅ 已实现 | - |
| 事件审计 | ❌ 未实现 | 建议记录关键事件 |
| 权限控制 | ❌ 未实现 | 可增加事件级别权限 |

### 3.4 接入规范

**连接 WebSocket：**
```javascript
const ws = new WebSocket('ws://localhost:8080/admin/api/ws/connect');

// 认证（通过 URL 参数或首次消息）
ws.onopen = () => {
  ws.send(JSON.stringify({
    event: 'connected',
    data: { token: 'your-jwt-token' }
  }));
};

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
| Virtual API | 动态模型生成 API |
| Hook 机制 | Before/After 扩展点 |
| 自动路由 | 通过 `AppendController` 注册 |

**实现文件：**
```
mss-boot/
├── pkg/response/controller/
│   ├── simple.go        # Simple 控制器
│   └── controller.go    # 扩展点
├── virtual/
│   ├── model/           # 虚拟模型
│   └── action/          # 虚拟动作

mss-boot-admin/
├── apis/virtual.go      # 虚拟 API 实现
└── models/model.go      # 模型定义
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

**创建虚拟模型 API：**

1. 通过数据库创建 Model 记录
2. 定义 Field 字段配置
3. 系统自动生成 `/api/{key}` 路由

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
2. **运行时模型更新**：虚拟模型修改需重启服务
3. **Hook 机制有限**：缺少更细粒度的扩展点
4. **权限粒度**：认证控制较粗，缺少字段级别权限

---

## 5. 扩展能力总览

### 5.1 能力矩阵

| 能力 | 实现完整度 | 治理集成度 | 扩展灵活性 | 安全性 |
|------|-----------|-----------|-----------|--------|
| 国际化 | 高 | 中 | 中 | 中 |
| 对象存储 | 中 | 低 | 高 | 低 |
| WebSocket | 高 | 中 | 低 | 中 |
| API-first | 高 | 高 | 高 | 高 |

### 5.2 治理要求对照

| 要求 | i18n | Storage | WebSocket | API-first |
|------|------|---------|-----------|-----------|
| JWT 认证 | ✅ | ✅ | ✅ | ✅ |
| Casbin 权限 | ✅ | ❌ | ❌ | ✅ |
| 租户隔离 | ❌ | ❌ | ❌ | ❌ |
| 审计日志 | ❌ | ❌ | ❌ | ⚠️ |
| 数据权限 | N/A | ❌ | N/A | ✅ |

### 5.3 改进优先级

**高优先级（安全相关）：**
1. 存储文件类型/大小校验
2. 存储权限控制

**中优先级（治理完善）：**
1. 存储审计日志
2. WebSocket 事件审计
3. 国际化审计日志
4. WebSocket 集群支持

**低优先级（体验优化）：**
1. 存储大文件分片上传
2. WebSocket 消息持久化
3. 国际化 Accept-Language 处理
4. API Swagger 自动生成

---

## 6. 新能力接入检查清单

当需要新增扩展能力时，必须完成以下检查：

### 6.1 治理集成

- [ ] 是否需要 JWT 认证？
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