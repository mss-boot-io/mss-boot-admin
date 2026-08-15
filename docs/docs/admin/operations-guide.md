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
├── Ext         → 文件类型枚举 (JSON/YAML/YML)
├── Content     → 配置内容 (文本)
├── Remark      → 配置说明
└── BuiltIn     → 内置配置保护标记
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
- 支持 JSON 与 YAML 配置格式
- 配置内容可在线编辑，无需重启服务
- 与 mss-boot 核心配置系统集成

### 运行机制

配置存储为不透明文本内容，系统通过 `source.Scheme` 解析格式并集成到 mss-boot 配置体系中。
该内容可能包含凭据，因此所有读取和变更在后端再次强制 root 身份，不以菜单隐藏代替授权；列表只返回
名称、格式、备注和内置标记，正文仅在详情请求中按需返回，相关响应使用 `Cache-Control: no-store`。

名称和 ID 由服务端约束，名称唯一，正文最多 256 KiB，并在保存前按 JSON 或 YAML 语法校验。内置配置
不可删除，更新时也不能修改名称和格式。`web/antd-v6` 在关闭详情或编辑器后立即清除正文查询缓存。

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
├── Read        → 是否已读
├── CreatedAt
├── Status
```

**存储位置**: `mss_boot_notices` 表

### API 入口

| 路径 | 方法 | 功能 |
|------|------|------|
| `/admin/api/notices` | GET | 通知列表（强制当前用户范围） |
| `/admin/api/notice/unread` | GET | 未读通知列表 |
| `/admin/api/notice/read/:id` | GET | 获取当前用户的通知详情 |
| `/admin/api/notice/read/:id` | PUT | 按 ID、类型或 `all` 标记为已读 |
| `/admin/api/notices` | POST | 创建当前用户通知（仍需 API 权限） |
| `/admin/api/notices/:id` | PUT/DELETE | 更新或删除当前用户通知 |

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

1. 通知创建、查询、更新和删除都从已验证会话派生所有者；请求中的 `UserID` 不能扩大范围
2. 用户通过 `/unread` 接口获取未读消息列表
3. 点击通知后调用 `/read` 标记为已读
4. 支持按类型筛选和时间排序

新建时 ID、所有者和已读状态由服务端生成或重置，更新时这些字段保持原值。列表页最多返回 100 条，
未读快捷入口最多返回 100 条，跨用户读取统一不可见。

## 3. 定时任务与作业调度

调度器明确区分两条通道：

| 通道 | 生命周期与配置 | 存储与管理 |
|------|----------------|------------|
| 用户任务 | 由 `task.enable` 控制，可通过 Task API 管理 | 使用 `mss_boot_tasks` 和 TaskRun |
| 内置系统作业 | 始终随服务启动，由 task server 统一调度和关闭 | 进程内不可变注册，不写 Task/TaskRun，用户 CRUD 不可修改 |

### 用户任务数据模型

**Task** (`models/task.go`)

```
Task
├── Provider    → 执行提供者枚举
│   ├── default → 内置 cron 调度
│   ├── k8s     → Kubernetes CronJob
│   ├── func    → 注册函数调用
├── Spec        → Provider 对应的 cron 表达式
├── Protocol    → HTTP/HTTPS（default 模式）
├── Endpoint    → HTTP 目标主机与路径（default 模式）
├── Method/Body/Metadata → HTTP 或注册函数参数
├── Command     → 容器命令（K8s 模式）
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

**存储位置**: `mss_boot_tasks` + `mss_boot_task_runs` 表。这里只存用户管理的任务，
不包含内置系统作业。

### 用户任务 API 入口

| 路径 | 方法 | 功能 |
|------|------|------|
| `/admin/api/tasks` | GET | 任务列表 |
| `/admin/api/tasks` | POST | 创建任务 |
| `/admin/api/tasks/:id` | GET/PUT/DELETE | 任务 CRUD |
| `/admin/api/tasks/:id/actions/:operate` | POST | 任务操作（start/stop） |
| `/admin/api/task/func-list` | GET | 可用函数列表 |

兼容说明：任务操作已从有副作用的 `GET /admin/api/task/:operate/:id` 迁移到
`POST /admin/api/tasks/:id/actions/:operate`。旧 GET 仅保留为迁移提示入口，固定返回
`405 Method Not Allowed`，不会启动、停止或执行任务；调用方必须改用新的 POST 路径。

任务列表是去除请求正文、元数据、端点和容器命令的安全摘要；完整详情、函数列表以及所有变更都在
后端强制 root 身份。新任务固定以停用状态创建，Provider 不允许在更新中切换，Kubernetes 任务的
集群和命名空间也不可原地迁移；需要更换执行边界时应停用并新建任务。仅停用任务可删除。

### 执行提供者

#### Default Provider (内置 cron)

- 使用带秒字段的六段 cron 表达式
- 只支持 HTTP/HTTPS 的 GET、POST、PUT、DELETE 请求
- 请求正文最多 64 KiB，元数据必须是最多 64 项的字符串映射
- 任务在当前进程内运行

#### K8s Provider (Kubernetes CronJob)

- 自动创建/更新/删除 K8s CronJob 资源
- 使用不含秒字段的五段 Kubernetes Cron 表达式
- 支持镜像配置和集群选择
- 停用状态对应 CronJob `spec.suspend: true`，启停 API 直接同步该字段，不进入进程内调度器
- 适合大规模分布式任务

#### Func Provider (注册函数)

- 通过 `TaskFuncMap` 注册自定义函数
- 任务执行时直接调用 Go 函数
- 适合轻量级内部逻辑
- 仅允许调用服务端已经注册的函数，浏览器脚本和任意 Python/命令执行入口不再提供
- 需要访问数据库的内置或自定义函数必须从任务上下文调用 `pkg.TaskDatabase(ctx)`，不得直接读取全局
  `gormdb.DB`。配置热加载会把新租约切到新连接，并等待旧句柄上的在途任务结束后再关闭旧连接；任务函数也不得
  在返回后保存该 `*gorm.DB` 指针

### 使用场景

- 定时数据清理
- 周期性报表生成
- 定时消息推送
- K8s 环境下的容器化任务

### 配置参数

```yaml
task:
  enable: true            # 仅控制持久化用户任务
  spec: "0 */1 * * * *"  # 用户任务协调器的六段 cron 表达式
```

`task.enable` 与 `task.spec` 在进程启动时形成调度快照；热加载配置不会重建 cron，修改后必须重启 Admin。

### 内置系统作业

| key | 默认调度 | 作用 | 持久化 |
|-----|----------|------|--------|
| `monitor-sampler` | 每 5 秒 | 采集当前实例 CPU/内存等快照并维护有界历史 | 仅进程内历史，不写 Task/TaskRun |
| `session-cleanup` | 每日 03:30 | 清理超过保留期的会话 | 不创建 Task/TaskRun |

内置作业使用 `mss-boot/core/server/task.WithSystemSchedule` 注册到独立的进程内存储。
task server 始终随服务启动，因此即使 `task.enable: false`，上述作业仍会运行。
其 key 是保留 key：用户任务不能覆盖，Task CRUD 不能更新或删除。
当 `task.enable: true` 时，服务会额外注册内部 `task` reconciliation 作业，用于
同步持久化用户任务；它不是第三个始终运行的维护作业。

内置作业按 Admin 副本运行：每个副本必须采集自己的监控数据；`session-cleanup` 也会在每个副本的
03:30 执行同一幂等清理。大规模多副本部署应评估同时清理带来的数据库压力，并在需要时通过外部
领导者选举或分布式租约统一执行。当前 default/func 用户任务同样没有跨副本单次执行保证；需要
严格单次执行时应使用 Kubernetes Provider 或外部任务协调器。

### 运行机制

1. 服务启动时先注册不可变内置系统作业，再按 `task.enable` 决定是否加载用户 Task 存储。
2. task server 合并两个通道并拒绝重复或冲突 key。
3. 用户 Task 根据 Provider 执行并记录 TaskRun；内置系统作业只记录业务日志或内存状态。
4. 用户任务操作 API 支持手动启停；启用 default/func 任务后会执行一次并进入调度，Kubernetes
   任务则解除 CronJob 暂停。任何操作都不能作用于内置系统作业。

## 4. 系统监控

### 监控指标

通过 `gopsutil` 库和 Go 运行时在后台系统作业中周期采集指标：

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
  "cpuUsage": 12.34,
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
  "uptime": 86400,
  "collectedAt": 1786000000000,
  "sampleIntervalMs": 5000,
  "stale": false,
  "instanceId": "admin-instance-id",
  "history": [
    {
      "timestamp": 1786000000000,
      "cpuUsage": 12.34,
      "memoryUsagePercent": 31.25
    }
  ]
}
```

### 使用场景

- 运维仪表盘展示
- 健康检查与告警阈值判断
- 负载评估与容量规划
- 性能调优与问题排查

### 运行机制

`monitor-sampler` 作为内置系统作业默认每 5 秒采样，最多保留 120 个按时间排序的
实例内点（约 10 分钟）。API 只复制最近一次成功快照和请求的历史窗口，不在请求
内执行一秒 CPU 测量。瞬时采集失败会保留 last-good 数据并设置 `stale: true`。

历史不写数据库，进程重启后重置，多副本之间也不聚合；长期趋势和集群视图仍应
由外部可观测性系统承担。`task.enable` 只影响用户 Task，不能关闭监控采样。

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
| 定时任务 | 用户 Task 三种 Provider + 独立内置系统作业通道 | CronJob + 插件任务 |
| 系统监控 | 5 秒后台采样、约 10 分钟实例内历史 + 主题感知图表 | 服务监控 + 日志系统 |
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
├── Status      → 登录结果（enabled/disabled）
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
- 登录成功：记录用户ID、用户名、IP、状态为 enabled
- 登录失败：记录用户名、IP、状态为 disabled、错误消息

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
├── Status      → 操作结果状态
├── CreatedAt   → 操作时间
```

**存储位置**: `mss_boot_audit_logs` 表

**API 入口**

| 路径 | 方法 | 功能 |
|------|------|------|
| `/admin/api/audit-logs/operation` | GET | 审计日志列表 |

浏览器 API 只返回有界安全投影，原始 Request/Response 正文不会通过通用 CRUD 或上述列表暴露；
路径、消息和 User-Agent 中常见的 token、密码、Cookie、Authorization 等值会在序列化前脱敏。
审计证据由服务写入，浏览器侧不能创建、覆盖或删除。

**记录范围**

通过 `middleware/audit.go` 中间件自动记录：
- POST/PUT/DELETE 请求
- 排除登录/登出/认证相关接口
- 记录请求体、响应状态、操作时间

### 运行时日志

| 路径 | 方法 | 功能与权限 |
|------|------|------------|
| `/admin/api/logs` | GET | 分页读取脱敏日志，需要 `/log/runtime` |
| `/admin/api/logs/files` | GET | 只返回顶层 `.log` 文件名，需要 `/log/runtime` |
| `/admin/api/logs/export` | GET | 导出当前筛选，需要独立 `/log/export` |

运行时读取只接受字面量关键词、固定日志级别和最长 31 天时间范围。单页最多 100 条，单次最多扫描
32 个普通文件、每文件尾部 16 MiB、合计 64 MiB、100,000 行和 10,000 个匹配；符号链接、子目录和
文件系统路径不会返回浏览器。结果被截断时会明确标记，并拒绝导出；完整导出仍限制为 5 MiB。

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
- 按相同保留期先清理 TaskRunLog，再清理 TaskRun
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
