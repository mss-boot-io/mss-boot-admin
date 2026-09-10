---
title: 性能与可观测性指南
order: 29
nav:
  order: 1
  title: admin
description: mss-boot-admin 的日志、指标、pprof、任务与 WebSocket 可观测性说明
keywords: [admin observability metrics pprof logs websocket]
---

## 目标

适用基线：当前稳定版 **mss-boot-admin v1.3.7**。

为 `mss-boot-admin` 提供统一的问题排查与运行观察路径。

## 观测面划分

| 维度 | 关注点 |
|------|--------|
| 系统 | CPU、内存、磁盘、网络 |
| 接口 | 响应错误、慢请求、认证失败 |
| 用户任务 | Task CRUD、调度是否执行、TaskRun 结果 |
| 内置系统作业 | 监控采样时间、陈旧状态、会话清理日志 |
| WebSocket | 在线连接数、集群广播、心跳保活 |
| 运行时 | Goroutine、GC、Heap、pprof |

## 已有能力

- 监控接口：`/admin/api/monitor`
- 后端每 5 秒采样、最多保留 120 个实例内历史点；接口读取缓存快照，不在请求内阻塞测量 CPU
- Welcome/Monitor 共用主题感知趋势图，显示服务端时间、实例标识和陈旧状态
- Admin 内置系统作业通道始终调度监控采样与会话清理
- 文件日志输出：`logger.path`, `logger.stdout=file`
- 运行时信息：Goroutines、Heap、GC 次数
- WebSocket 在线状态接口
- Redis 存在时自动启用 WebSocket 集群广播

## 问题排查顺序

### 1. 接口问题

- 先看状态码
- 再看后端日志
- 再看监控页资源占用

### 2. 用户任务问题

- 检查 `task.enable`；该开关只控制持久化用户任务
- 检查 cron 表达式是否为 6 段
- 检查任务 `checked_at` 是否变化
- 检查对应 TaskRun 和执行错误

### 3. 内置系统作业问题

- 不要在 `mss_boot_tasks` 或 TaskRun 中查找 `monitor-sampler`、`session-cleanup`；内置作业不会写入这些表
- 监控采样应在 `task.enable: false` 时继续运行；检查 `/admin/api/monitor` 的 `collectedAt`、`sampleIntervalMs`、`stale` 和 `instanceId`
- 会话清理应在用户任务关闭时继续运行；检查服务日志中的 `session cleanup done/failed`
- 用户任务 API 不能更新、删除或覆盖内置系统作业 key；启动时发生 key 冲突应作为配置错误处理

### 4. WebSocket 问题

- 检查连接是否建立
- 检查在线人数是否变化
- 检查 Redis 是否可用

## 推荐保留的日志关注点

- 登录失败
- 审计操作写入失败
- 告警发送失败
- Redis Pub/Sub 反序列化失败
- 用户任务执行失败
- 内置监控采样失败或会话清理失败

## pprof 与运行时建议

当出现持续性高 CPU / 高内存时：

1. 先看 Welcome 监控页
2. 再看 `/pprof` 导出信息
3. 再区分用户任务与内置系统作业日志，并检查 WebSocket 在线连接数

## 容量评估建议

- 如果 WebSocket 在线连接持续上升，优先评估 Redis 与反向代理配置
- 如果日志量持续增长，优先检查用户配置的日志清理任务和 retention 设置
- 如果任务数量增长，优先检查任务并发和执行时间

## 推荐阅读

- [运营能力说明](/admin/operations-guide)
- [Admin 发布与部署验证清单](/admin/release-verification-checklist)
