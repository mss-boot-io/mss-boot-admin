---
title: 集成测试指南
order: 21
nav:
  order: 1
  title: admin
description: mss-boot-admin 前后端联调、E2E 测试与回归验证指南
keywords: [admin integration test e2e playwright]
---

## 概述

本文档描述 `mss-boot-admin` 的集成测试方法，包括：

- 前后端联调流程
- Playwright E2E 测试
- API 验证方式
- 回归检查清单

## 前置条件

### 开发环境

- Go 1.26+
- Node.js 18+ 和 pnpm
- SQLite（用于本地数据库）

### 服务端口

| 服务 | 端口 | 说明 |
|------|------|------|
| 后端 API | 8080 | `go run . server` |
| 前端 Dev | 8000 | `pnpm dev` |

## 快速开始

### 1. 启动后端

```bash
cd mss-boot-admin
go run . migrate  # 首次运行迁移
go run . server   # 启动服务
```

验证：

```bash
curl http://localhost:8080/healthz
# 预期: {"status":"ok"}
```

### 2. 启动前端

```bash
cd mss-boot-admin-antd
pnpm install
pnpm dev
```

验证：

```bash
curl http://localhost:8000
# 预期: HTML 响应
```

### 3. 运行 E2E 测试

```bash
cd mss-boot-admin-antd
npx playwright test --reporter=list
```

## Playwright E2E 测试

### 测试文件结构

```
mss-boot-admin-antd/e2e/
├── login.spec.ts           # 登录测试
├── monitor.spec.ts         # 监控测试
├── websocket.spec.ts       # WebSocket 测试
└── post-customdept.spec.ts # 岗位数据权限测试
```

### 测试覆盖

| 测试文件 | 测试项 | 说明 |
|----------|--------|------|
| `login.spec.ts` | API 登录 | 通过 API 登录获取 Token |
| `login.spec.ts` | UI 表单登录 | 输入用户名密码点击登录 |
| `login.spec.ts` | 错误凭证验证 | 错误密码返回 401 |
| `monitor.spec.ts` | 系统监控 | 返回 CPU/内存/磁盘信息 |
| `monitor.spec.ts` | 网络统计 | 返回网络连接状态 |
| `monitor.spec.ts` | 运行时统计 | 返回 Go 运行时信息 |
| `websocket.spec.ts` | 在线状态 | 返回 WebSocket 在线用户数 |
| `post-customdept.spec.ts` | 创建岗位 (all) | dataScope=all 时 deptIDS 为空 |
| `post-customdept.spec.ts` | 创建岗位 (customDept) | dataScope=customDept 时 deptIDS 自动填充 |

### 运行测试

```bash
# 运行所有测试
npx playwright test

# 运行特定测试文件
npx playwright test e2e/login.spec.ts

# 带界面运行（调试用）
npx playwright test --ui

# 生成报告
npx playwright show-report
```

## API 验证

### 登录获取 Token

```bash
curl -X POST http://localhost:8080/admin/api/user/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"123456"}'

# 响应
{
  "code": 200,
  "token": "eyJhbGciOiJIUzI1NiIs...",
  "expire": "2026-04-03T07:36:38+08:00"
}
```

### 岗位管理

**创建岗位（customDept 自动填充 deptIDS）**

```bash
curl -X POST http://localhost:8080/admin/api/posts \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{"name":"测试岗位","dataScope":"customDept","status":"enabled"}'

# 响应（deptIDS 自动填充）
{
  "id": "xxx",
  "name": "测试岗位",
  "dataScope": "customDept",
  "deptIDS": ["e19bdf394f6d410ca703dfb6f4f3a751"]
}
```

### 监控接口

```bash
curl http://localhost:8080/admin/api/monitor \
  -H "Authorization: Bearer <token>"

# 响应包含
{
  "cpuPhysicalCore": 4,
  "cpuLogicalCore": 8,
  "memoryTotal": 16384,
  "memoryUsage": 5120,
  "diskTotal": 500,
  "diskUsage": 150,
  "network": {...},
  "runtime": {...}
}
```

### WebSocket 接口

```bash
# 在线用户数
curl http://localhost:8080/admin/api/ws/online \
  -H "Authorization: Bearer <token>"

# 响应
{
  "online": 1
}
```

### 审计日志

```bash
# 登录日志
curl http://localhost:8080/admin/api/audit-logs/login \
  -H "Authorization: Bearer <token>"

# 操作日志
curl http://localhost:8080/admin/api/audit-logs/operation \
  -H "Authorization: Bearer <token>"
```

## 回归检查清单

### 登录流程

- [ ] 输入正确账号密码，登录成功
- [ ] 登录成功后跳转到首页（不停留在登录页）
- [ ] 输入错误密码，显示错误提示
- [ ] Token 存储到 localStorage

### 岗位管理

- [ ] 岗位列表正常显示
- [ ] 创建岗位成功，dataScope 可选择
- [ ] `dataScope = "customDept"` 时，deptIDS 自动填充（无需手动选择）
- [ ] `dataScope = "all"` 时，deptIDS 为空
- [ ] 编辑岗位正常保存

### 监控功能

- [ ] CPU 信息正确显示
- [ ] 内存信息正确显示
- [ ] 磁盘信息正确显示
- [ ] CPU/内存趋势图正常刷新
- [ ] 百分比与容量单位保留 2 位小数
- [ ] 网络统计正确显示
- [ ] 运行时信息正确显示

### WebSocket

- [ ] WebSocket 连接建立成功
- [ ] 在线用户统计正确
- [ ] 心跳保活正常

### 审计日志

- [ ] 登录日志正确记录
- [ ] 操作日志正确记录
- [ ] 日志列表查询正常

### 存储安全

- [ ] 超过最大文件大小时上传被拒绝
- [ ] 非白名单 MIME 类型上传被拒绝
- [ ] 合法图片上传成功并可通过 `/public/` 访问

### 告警与通知

- [ ] 告警规则可创建和保存
- [ ] 告警历史可查询
- [ ] WebSocket 告警通知可收到
- [ ] 已配置的邮件 / 钉钉 / 企业微信通知可验证

### 国际化

- [ ] `Accept-Language: zh-CN` 返回中文语言结果
- [ ] `Accept-Language: en-US` 返回英文语言结果
- [ ] 非法语言代码创建时被后端拒绝

### 任务调度

- [ ] `task.enable=true` 时任务调度器启动
- [ ] 日志清理任务 `checked_at` 持续更新
- [ ] `log_cleaner` 任务可执行

## 前后端联调流程

### 1. 接口契约确认

**后端提供：**

- API 路径和方法
- 请求参数结构
- 响应数据结构
- 鉴权要求

**前端确认：**

- 字段命名符合前端约定
- 分页参数格式
- 错误码处理方式

### 2. 接口 Mock（可选）

前端可先使用 Mock 数据开发：

```typescript
// mock/post.ts
export default {
  'POST /admin/api/posts': {
    id: 'mock-id',
    name: 'Mock 岗位',
    dataScope: 'all',
    deptIDS: null,
  },
};
```

### 3. 联调验证

1. 后端先完成接口并提供 Swagger 文档
2. 前端对接真实接口
3. 验证请求/响应格式
4. 验证错误处理
5. 编写 E2E 测试

### 4. 问题归属判断

| 现象 | 可能原因 | 排查方向 |
|------|----------|----------|
| 401 Unauthorized | Token 无效/过期 | 检查 Token 是否正确传递 |
| 403 Forbidden | 权限不足 | 检查 Casbin 策略 |
| 400 Bad Request | 参数格式错误 | 检查请求体格式 |
| 500 Internal Error | 后端异常 | 查看后端日志 |
| CORS 错误 | 跨域配置 | 检查 server.cors 配置 |

## 发布前冒烟检查

建议在每次发布前至少完成以下最小验证：

```bash
# 1. 后端构建
cd mss-boot-admin && go build ./...

# 2. 前端类型检查
cd ../mss-boot-admin-antd && pnpm -s tsc --noEmit

# 3. 后端健康检查
curl -I http://127.0.0.1:8080/healthz

# 4. 前端可达性检查
curl -I http://127.0.0.1:8000
```

发布前最低检查项：

- [ ] 登录成功且首次登录后页面状态正常
- [ ] Welcome 监控卡片与趋势图正常
- [ ] 日志页面三类日志可见
- [ ] 头像上传成功并可访问
- [ ] WebSocket 在线状态正常
- [ ] 关键配置页可保存
- [ ] 至少一条定时任务处于启用状态

## CI/CD 集成

### GitHub Actions 示例

```yaml
name: Test

on: [push, pull_request]

jobs:
  backend-test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v6
      - uses: actions/setup-go@v6
        with:
          go-version: '1.26'
      - name: Run backend tests
        run: |
          cd mss-boot-admin
          go test ./... -v -race -coverprofile=coverage.out
      - name: Upload coverage
        uses: codecov/codecov-action@v3
        with:
          files: ./mss-boot-admin/coverage.out

  e2e-test:
    runs-on: ubuntu-latest
    needs: backend-test
    steps:
      - uses: actions/checkout@v6
      - uses: actions/setup-node@v6
        with:
          node-version: '24'
      - uses: pnpm/action-setup@v6
        with:
          version: 8
      - name: Install dependencies
        run: |
          cd mss-boot-admin-antd
          pnpm install
          npx playwright install --with-deps
      - name: Start backend
        run: |
          cd mss-boot-admin
          go run . server &
          sleep 5
      - name: Start frontend
        run: |
          cd mss-boot-admin-antd
          pnpm dev &
          sleep 10
      - name: Run E2E tests
        run: |
          cd mss-boot-admin-antd
          npx playwright test
```

## 推荐阅读

- [权限与组织治理说明](/admin/governance-guide)
- [运营能力说明](/admin/operations-guide)
- [Token 与 OAuth2 联调说明](/admin/token-oauth2-guide)
- [四期路线图](/admin/phase-4-roadmap)
