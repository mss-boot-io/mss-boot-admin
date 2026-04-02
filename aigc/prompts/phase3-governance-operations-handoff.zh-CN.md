# 三期治理与运营能力补强交接文档

> 适用范围：基于当前仓库可见代码与本轮实现结果（截至 2026-04-02）。
>
> 目标背景：完成三期路线图中 P1 治理基线补强和 P2 运营中枢补强的核心功能实现与验证。

## 1. 已完成结论汇总

### 1.1 CustomDept 数据权限自动填充

**实现结论：**

- 岗位管理支持 `dataScope = "customDept"` 时，`deptIDS` 字段由后端自动填充
- 用户创建/编辑岗位时无需手动选择部门，系统根据用户的部门自动计算
- 自动填充逻辑：获取用户所属部门 + 递归获取所有子部门

**实现位置：**

| 项目 | 文件 | 变更内容 |
|------|------|----------|
| `mss-boot-admin` | `apis/post.go` | 添加 `WithBeforeCreate`/`WithBeforeUpdate` hooks |
| `mss-boot-admin` | `models/post.go` | `DeptIDSArr` 字段已存在 |
| `mss-boot-admin-antd` | `src/pages/Post/index.tsx` | 移除 `deptIDS` 手动选择器 |

**验证结果：**

```json
// 创建岗位 dataScope = "customDept"
{
  "name": "测试岗位customDept",
  "dataScope": "customDept",
  "deptIDS": ["e19bdf394f6d410ca703dfb6f4f3a751"]  // 自动填充
}

// 创建岗位 dataScope = "all"
{
  "name": "测试岗位all",
  "dataScope": "all",
  "deptIDS": null  // 不填充
}
```

### 1.2 WebSocket 实时通知

**实现结论：**

- WebSocket 连接入口：`/admin/api/ws/connect`（需 JWT 认证）
- 在线用户统计：`/admin/api/ws/online`
- 复用主 HTTP 服务端口 8080，无需单独端口
- 支持 `ping/pong` 心跳、`notify` 通知推送、`kick` 强制下线

**实现位置：**

| 项目 | 文件 | 说明 |
|------|------|------|
| `mss-boot-admin` | `apis/ws.go` | WebSocket 连接处理 |
| `mss-boot-admin` | `router/router.go` | 路由注册 |

### 1.3 登录与操作审计日志

**实现结论：**

- 登录日志查询：`/admin/api/audit-logs/login`
- 操作日志查询：`/admin/api/audit-logs/operation`
- 审计日志 CRUD：`/admin/api/audit-logs/*`

**实现位置：**

| 项目 | 文件 | 说明 |
|------|------|------|
| `mss-boot-admin` | `apis/audit_log.go` | 审计日志 API |
| `mss-boot-admin` | `models/audit_log.go` | 审计日志模型 |

### 1.4 扩展监控维度

**实现结论：**

- 监控接口：`/admin/api/monitor`
- 采集维度：CPU、内存、磁盘、网络、运行时、系统信息
- 使用 `gopsutil` 库实时采集

**实现位置：**

| 项目 | 文件 | 说明 |
|------|------|------|
| `mss-boot-admin` | `apis/monitor.go` | 监控 API |
| `mss-boot-admin` | `models/monitor.go` | 监控数据结构 |

### 1.5 Playwright E2E 测试

**测试覆盖：**

| 测试文件 | 测试项 | 状态 |
|----------|--------|------|
| `e2e/login.spec.ts` | API 登录、UI 表单登录、错误凭证 | ✅ 3/3 通过 |
| `e2e/monitor.spec.ts` | 系统监控、网络统计、运行时统计 | ✅ 3/3 通过 |
| `e2e/websocket.spec.ts` | 在线状态查询 | ✅ 1/1 通过 |
| `e2e/post-customdept.spec.ts` | 岗位创建（all/customDept） | ✅ 2/2 通过 |

**总计：9/9 测试通过**

## 2. 当前影响范围

### 2.1 后端变更文件

```
mss-boot-admin/
├── apis/
│   ├── post.go          # 添加 beforeCreate/beforeUpdate hooks
│   ├── ws.go            # WebSocket API
│   ├── audit_log.go     # 审计日志 API
│   └── monitor.go       # 监控 API
├── models/
│   ├── post.go          # DeptIDSArr 字段
│   ├── audit_log.go     # 审计日志模型
│   └── monitor.go       # 监控模型
└── router/
    └── router.go        # 路由注册
```

### 2.2 前端变更文件

```
mss-boot-admin-antd/
├── src/
│   ├── pages/Post/index.tsx    # 移除 deptIDS 手动选择器
│   └── app.tsx                 # 修复登录跳转问题
└── e2e/
    ├── login.spec.ts           # 登录测试
    ├── monitor.spec.ts         # 监控测试
    ├── websocket.spec.ts       # WebSocket 测试
    └── post-customdept.spec.ts # 岗位测试
```

### 2.3 文档变更文件

```
mss-boot-docs/docs/admin/
├── governance-guide.md         # 权限与组织治理说明
├── operations-guide.md         # 运营能力说明
└── token-oauth2-guide.md       # Token 与 OAuth2 联调说明
```

## 3. 风险分级

### 高风险

- 无

### 中风险

1. **CustomDept 递归查询性能**
   - 当部门层级较深或部门数量较多时，`getChildDeptIDS` 递归查询可能影响性能
   - 建议：后续可考虑缓存部门树或使用闭包表优化

2. **WebSocket 连接数限制**
   - 当前未实现连接数限制和负载均衡
   - 建议：生产环境需考虑 WebSocket 集群方案

### 低风险

1. **Playwright 测试依赖服务状态**
   - 测试需要后端和前端服务同时运行
   - 建议：CI/CD 环境需要先启动服务再运行测试

## 4. 下一步建议顺序

### P3 AI 注解协同流程落地（继续）

1. **创建运营能力规划文档**
   - 位置：`mss-boot-docs/docs/admin/operations-planning.md`
   - 内容：配置/通知/任务/监控/统计的中长期演进规划

2. **创建前后端联调指南**
   - 位置：`mss-boot-docs/docs/admin/integration-test-guide.md`（已存在，需更新）
   - 内容：联调流程、接口契约、测试数据准备、回归清单

3. **评估 P3 完成度**
   - 对照三期路线图检查 P3 各项是否满足"预期结果"

### P4 集成与扩展护栏（下一阶段）

1. 国际化能力边界定义
2. WebSocket 事件能力扩展规范
3. API-first 扩展方式文档

### P5 历史能力降级（后续）

1. 动态模型定位评估
2. 代码生成能力叙事收口

## 5. 启动前检查项

### 开发环境

- [ ] Go 1.26+ 已安装
- [ ] Node.js 18+ 和 pnpm 已安装
- [ ] SQLite 数据库文件存在（`mss-boot-admin-local.db`）

### 服务启动

```bash
# 启动后端
cd mss-boot-admin && go run . server

# 启动前端
cd mss-boot-admin-antd && pnpm dev
```

### 验证服务

```bash
# 检查后端
curl http://localhost:8080/healthz

# 检查前端
curl http://localhost:8000
```

## 6. 验证/回归清单

### API 验证

```bash
# 登录获取 Token
curl -X POST http://localhost:8080/admin/api/user/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"123456"}'

# 创建 CustomDept 岗位（应自动填充 deptIDS）
curl -X POST http://localhost:8080/admin/api/posts \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{"name":"测试岗位","dataScope":"customDept","status":"enabled"}'

# 查询监控信息
curl http://localhost:8080/admin/api/monitor \
  -H "Authorization: Bearer <token>"

# 查询在线用户
curl http://localhost:8080/admin/api/ws/online \
  -H "Authorization: Bearer <token>"
```

### E2E 测试

```bash
cd mss-boot-admin-antd
npx playwright test --reporter=list
```

### 功能验证

- [ ] 登录成功后跳转到首页（不再停留在登录页）
- [ ] 创建岗位时无需选择 deptIDS，系统自动填充
- [ ] WebSocket 连接成功，在线用户统计正确
- [ ] 监控页面显示 CPU/内存/磁盘/网络/运行时信息

## 7. 相关参考文档

### 规范与模板

- `mss-boot-docs/docs/admin/ai-annotation-spec.md` - AI 注解协同规范
- `mss-boot-docs/docs/admin/ai-annotation-templates.md` - AI 注解产物模板
- `mss-boot-docs/aigc/prompts/roles/role-collaboration-map.zh-CN.md` - 五角色协作总览

### 功能文档

- `mss-boot-docs/docs/admin/governance-guide.md` - 权限与组织治理说明
- `mss-boot-docs/docs/admin/operations-guide.md` - 运营能力说明
- `mss-boot-docs/docs/admin/token-oauth2-guide.md` - Token 与 OAuth2 联调说明
- `mss-boot-docs/docs/admin/phase-3-roadmap.md` - 三期路线图

### 样例文档

- `mss-boot-admin/aigc/prompts/tenant-removal-handoff-summary.zh-CN.md` - 交接文档样例
- `mss-boot-admin/aigc/prompts/action-architecture-review.zh-CN.md` - 评审文档样例