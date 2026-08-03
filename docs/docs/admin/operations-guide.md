---
title: 运营能力说明
order: 19
nav:
  order: 1
  title: admin
description: mss-boot-admin 系统配置、通知公告、定时任务、监控统计、日志管理、告警通知等运营能力说明
keywords: [admin operations config notice task monitor statistics log alert]
---

## 概述

本文档描述 `mss-boot-admin` 的运营能力架构，覆盖：

- 系统配置管理
- 通知公告系统
- 定时任务与作业调度
- 系统监控信息
- 统计查询能力
- 日志管理
- 告警通知

这些能力使 `mss-boot-admin` 不仅是权限管理平台，更是具备日常运营维护能力的后台系统。

## 1. 系统配置管理

### 数据模型

**SystemConfig** (`models/system_config.go`)

```
SystemConfig
├── Name        → 配置文件名称
├── Extension   → 文件类型枚举 (YAML/JSON/TOML/INI)
├── Content     → 配置内容 (文本)
├── Remarks     → 配置说明
├── Status      → 启用状态
```

**存储位置**: `mss_boot_system_configs` 表

### API 入口

| 路径 | 方法 | 功能 |
|------|------|------|
| `/admin/api/system-configs` | GET | 配置列表查询 |
| `/admin/api/system-configs` | POST | 创建新配置 |
| `/admin/api/system-configs/:id` | GET | 获取单个配置 |
| `/admin/api/system-configs/:id` | PUT | 更新配置内容 |
| `/admin/api/system-configs/:id` | DELETE | 删除配置 |

### 使用场景

- 存储系统级 YAML/JSON 配置文件
- 支持多种配置格式，适配不同子系统需求
- 配置内容可在线编辑，无需重启服务
- 与 mss-boot 核心配置系统集成

### 运行机制

配置存储为文本内容，系统通过 `source.Scheme` 解析不同格式，集成到 mss-boot 配置体系中。

## 2. 通知公告系统

### 数据模型

**Notice** (`models/notice.go`)

```
Notice
├── Type        → 通知类型枚举
│   ├── notification → 系统公告
│   ├── message      → 个人消息
│   ├── event        → 事件通知
│   ├── mail         → 邮件通知
├── UserID      → 接收用户
├── Title       → 通知标题
├── Description → 通知内容
├── ReadAt      → 阅读时间 (未读为空)
├── CreatedAt
├── Status
```

**存储位置**: `mss_boot_notices` 表

### API 入口

| 路径 | 方法 | 功能 |
|------|------|------|
| `/admin/api/notice` | GET | 通知列表 (当前用户) |
| `/admin/api/notice/unread` | GET | 未读通知列表 |
| `/admin/api/notice/read/:id` | POST | 标记为已读 |
| `/admin/api/notice` | POST | 创建通知 (管理员) |
| `/admin/api/notice/:id` | DELETE | 删除通知 |

### 实时通知 (WebSocket)

系统支持通过 WebSocket 实时推送通知：

**WebSocket 连接**

| 路径 | 说明 |
|------|------|
| `/admin/api/ws/connect` | WebSocket 连接入口 (需认证) |
| `/admin/api/ws/online` | 在线用户统计 |

**消息格式**

```json
{
  "event": "notify",
  "code": 200,
  "data": {
    "id": "xxx",
    "type": "notification",
    "title": "系统通知标题",
    "description": "通知内容",
    "createdAt": "2024-01-01T00:00:00Z"
  },
  "timestamp": 1704067200
}
```

**事件类型**

| 事件 | 说明 |
|------|------|
| `ping/pong` | 心跳保活 |
| `notify` | 通知推送 |
| `kick` | 强制下线 |
| `join/quit` | 连接管理 |

### 使用场景

- 系统公告发布与推送
- 个人消息通知 (支持实时推送)
- 运营事件提醒
- 集成邮件发送能力

### 运行机制

1. 管理员创建通知后，系统根据 `UserID` 投递给目标用户
2. 用户通过 `/unread` 接口获取未读消息列表
3. 点击通知后调用 `/read` 标记为已读
4. 支持按类型筛选和时间排序

## 3. 定时任务与作业调度

### 数据模型

**Task** (`models/task.go`)

```
Task
├── Provider    → 执行提供者枚举
│   ├── default → 内置 cron 调度
│   ├── k8s     → Kubernetes CronJob
│   ├── func    → 注册函数调用
├── Spec        → 调度表达式 (cron 格式)
├── Command     → 执行命令 (HTTP/脚本/gRPC)
├── Args        → 命令参数
├── Image       → 镜像地址 (K8s 模式)
├── Cluster     → 集群名称 (K8s 模式)
├── Name, Remarks
├── Status      → 启用状态
└── TaskRuns    → 执行历史记录
```

**TaskRun** (执行记录)

```
TaskRun
├── TaskID      → 关联任务
├── Status      → 执行状态 (success/failed)
├── Message     → 执行结果/错误信息
├── StartTime
├── EndTime
```

**存储位置**: `mss_boot_tasks` + `mss_boot_task_runs` 表

### API 入口

| 路径 | 方法 | 功能 |
|------|------|------|
| `/admin/api/task` | GET | 任务列表 |
| `/admin/api/task` | POST | 创建任务 |
| `/admin/api/task/:id` | GET/PUT/DELETE | 任务 CRUD |
| `/admin/api/task/:operate/:id` | POST | 任务操作 (start/stop/run) |
| `/admin/api/task/func-list` | GET | 可用函数列表 |

### 执行提供者

#### Default Provider (内置 cron)

- 使用标准 cron 库实现调度
- 支持 HTTP、gRPC、Shell 脚本执行
- 任务在当前进程内运行

#### K8s Provider (Kubernetes CronJob)

- 自动创建/更新/删除 K8s CronJob 资源
- 支持镜像配置和集群选择
- 适合大规模分布式任务

#### Func Provider (注册函数)

- 通过 `TaskFuncMap` 注册自定义函数
- 任务执行时直接调用 Go 函数
- 适合轻量级内部逻辑

### 使用场景

- 定时数据清理
- 周期性报表生成
- 定时消息推送
- K8s 环境下的容器化任务

### 配置参数

```yaml
# config/task.go
Task:
  Enable: true        # 全局任务开关
  DefaultSpec: "0 * * * *"  # 默认调度表达式
```

### 运行机制

1. 系统启动时加载所有启用的 Task
2. 根据 Provider 类型注册调度器
3. 到达触发时间时执行任务并记录 TaskRun
4. `/operate` 接口支持手动启停和立即执行

## 4. 系统监控

### 监控指标

通过 `gopsutil` 库和 Go 运行时实时采集系统指标：

| 指标类别 | 采集项 |
|----------|--------|
| CPU | 逻辑核心数、物理核心数、使用率、型号信息 |
| 内存 | 总量、已用、可用、空闲、使用率 |
| 磁盘 | 总容量、已用空间、使用率 |
| 网络 | 发送/接收字节数、包数、错误数、丢包数、连接状态统计 |
| 运行时 | 协程数、堆内存、栈内存、GC 统计 |
| 系统信息 | Go 版本、启动时间、运行时长 |

### API 入口

| 路径 | 方法 | 功能 |
|------|------|------|
| `/admin/api/monitor` | GET | 系统监控信息 |

### 返回示例

```json
{
  "cpuPhysicalCore": 4,
  "cpuLogicalCore": 8,
  "cpuInfo": [...],
  "memoryTotal": 16384,
  "memoryUsage": 5120,
  "memoryUsagePercent": 31.25,
  "diskTotal": 500,
  "diskUsage": 150,
  "diskUsagePercent": 30,
  "network": {
    "bytesSent": 1234567,
    "bytesRecv": 7654321,
    "connectionCount": {
      "established": 45,
      "listen": 10,
      "timeWait": 5,
      "closeWait": 2,
      "total": 62
    }
  },
  "runtime": {
    "goroutines": 128,
    "heapAlloc": 52428800,
    "heapSys": 67108864,
    "numGC": 15
  },
  "goVersion": "go1.22.0",
  "startTime": 1704067200,
  "uptime": 86400
}
```

### 使用场景

- 运维仪表盘展示
- 健康检查与告警阈值判断
- 负载评估与容量规划
- 性能调优与问题排查

### 运行机制

每次请求时实时采集，不持久化存储。适合即时监控场景，不适合历史趋势分析。

## 5. 统计查询

### 数据模型

**Statistics** (`models/statistics.go`)

```
Statistics
├── Name        → 统计项名称
├── Type        → 统计类型
├── Value       → 统计值 (*100 精度)
├── Time        → 统计时间
├── Remarks
```

**存储位置**: `mss_boot_statistics` 表

### API 入口

| 路径 | 方法 | 功能 |
|------|------|------|
| `/admin/api/statistics/:name` | GET | 查询指定统计项 |

### 统计接口

**StatisticsObject** (`center/type.go`)

```go
type StatisticsObject interface {
    StatisticsCalibrate()   // 校准值 (设置精确值)
    StatisticsNowIncrease() // 增量
    StatisticsNowReduce()   // 减量
}
```

### 使用场景

- 用户数统计 (`User` 模型已实现)
- 业务指标追踪
- 运营报表数据源

### 运行机制

1. 需要统计的业务模型实现 `StatisticsObject` 接口
2. 通过 `center.StatisticsImp` 调用统计方法
3. 增减时写入 Statistics 表并 *100 提升精度
4. 查询时返回时间序列数据

## 与 HotGo 等项目的对比

| 能力维度 | mss-boot-admin | HotGo |
|----------|----------------|-------|
| 系统配置 | 数据库存储多格式配置 | 配置中心 + 系统参数 |
| 通知系统 | Notice 表 + WebSocket 实时推送 | WebSocket 实时推送 |
| 定时任务 | Default/K8s/Func 三种 Provider | CronJob + 插件任务 |
| 系统监控 | CPU/Memory/Disk 实时采集 + 图表展示 | 服务监控 + 日志系统 |
| 统计查询 | Statistics 表 + 接口实现 | 统计报表 + 可视化 |
| 日志管理 | 登录日志 + 审计日志 + 运行时日志 | 服务日志系统 |
| 告警通知 | 规则配置 + 多渠道推送 | 告警系统 |

## 6. 日志管理

### 登录日志

**数据模型** (`models/login_log.go`)

```
LoginLog
├── ID          → 日志ID
├── UserID      → 用户ID
├── Username    → 用户名
├── IP          → 登录IP
├── UserAgent   → 浏览器信息
├── Status      → 登录状态 (success/failed)
├── Message     → 登录消息
├── LoginAt     → 登录时间
```

**存储位置**: `mss_boot_login_logs` 表

**API 入口**

| 路径 | 方法 | 功能 |
|------|------|------|
| `/admin/api/audit-logs/login` | GET | 登录日志列表 |

**记录时机**

在 `middleware/auth.go` 的 `Authenticator` 函数中：
- 登录成功：记录用户ID、用户名、IP、状态为 success
- 登录失败：记录用户名、IP、状态为 failed、错误消息

### 审计日志

**数据模型** (`models/audit_log.go`)

```
AuditLog
├── ID          → 日志ID
├── UserID      → 操作用户ID
├── Username    → 操作用户名
├── Method      → HTTP方法
├── Path        → 请求路径
├── IP          → 请求IP
├── UserAgent   → 浏览器信息
├── RequestBody → 请求体
├── Status      → 响应状态码
├── CreatedAt   → 操作时间
```

**存储位置**: `mss_boot_audit_logs` 表

**API 入口**

| 路径 | 方法 | 功能 |
|------|------|------|
| `/admin/api/audit-logs/operation` | GET | 审计日志列表 |

**记录范围**

通过 `middleware/audit.go` 中间件自动记录：
- POST/PUT/DELETE 请求
- 排除登录/登出/认证相关接口
- 记录请求体、响应状态、操作时间

### 运行时日志

**配置方式** (`config/application.yml`)

```yaml
logger:
  path: logs           # 日志文件目录
  stdout: file         # 输出到文件
  level: info          # 日志级别
  json: false          # 非JSON格式
  addSource: true      # 添加源码位置
```

**日志清理任务**

系统内置 `log_cleaner` 任务函数，支持：
- 清理数据库中的登录日志和审计日志
- 清理本地日志文件

**配置清理任务**

```bash
# 在数据库中创建任务
INSERT INTO mss_boot_tasks (id, name, provider, method, spec, args, status)
VALUES ('log-cleaner-001', '日志清理任务', 'func', 'log_cleaner', '0 0 3 * * *', '["30","7","logs"]', 'enabled');
```

参数说明：
- 第1个参数：数据库日志保留天数（默认30天）
- 第2个参数：本地日志文件保留天数（默认7天）
- 第3个参数：日志目录路径（默认 logs）

## 7. 告警通知

### 告警规则

**数据模型** (`models/alert.go`)

```
AlertRule
├── ID          → 规则ID
├── Name        → 规则名称
├── Metric      → 监控指标 (cpu/memory/disk)
├── Operator    → 比较运算符 (>/</>=/<=)
├── Threshold   → 阈值
├── Duration    → 持续时间(秒)
├── Channels    → 通知渠道(JSON数组)
├── Message     → 告警消息模板
├── Status      → 状态 (enabled/disabled)
```

**存储位置**: `mss_boot_alert_rules` 表

**监控指标**

| 指标 | 说明 |
|------|------|
| `cpu` | CPU使用率 |
| `memory` | 内存使用率 |
| `disk` | 磁盘使用率 |

### 告警历史

**数据模型** (`models/alert.go`)

```
AlertHistory
├── ID          → 记录ID
├── RuleID      → 关联规则ID
├── RuleName    → 规则名称
├── Metric      → 监控指标
├── Value       → 触发值
├── Threshold   → 阈值
├── Status      → 状态 (firing/resolved)
├── TriggeredAt → 触发时间
├── ResolvedAt  → 恢复时间
```

**存储位置**: `mss_boot_alert_histories` 表

### 通知渠道配置

**配置项** (`config/application.yml`)

```yaml
notification:
  email:
    enabled: true
    host: "smtp.example.com"
    port: 587
    username: "alert@example.com"
    password: "your-password"
    from: "alert@example.com"
  dingtalk:
    enabled: true
    webhook: "https://oapi.dingtalk.com/robot/send?access_token=xxx"
    secret: "your-secret"
  wechat:
    enabled: true
    webhook: "https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=xxx"
```

### 告警检查机制

`service/alert_checker.go` 实现：
- 定时检查（可配置检查间隔）
- 读取所有启用的告警规则
- 获取对应指标的当前值
- 评估是否触发告警
- 通过配置的渠道发送通知
- 记录告警历史

### 通知渠道说明

| 渠道 | 配置要求 | 说明 |
|------|----------|------|
| WebSocket | 无需额外配置 | 系统内置，实时推送给在线用户 |
| Email | SMTP服务器配置 | 支持TLS/SSL |
| DingTalk | Webhook + Secret | 支持签名验证 |
| WeChat | Webhook | 企业微信群机器人 |

## 推荐阅读

- [产品方向调整](/admin/product-direction)
- [权限与组织治理说明](/admin/governance-guide)
- [当前功能总览](/admin/current-capabilities)
- [四期路线图](/admin/phase-4-roadmap)