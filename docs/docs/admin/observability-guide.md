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

为 `mss-boot-admin` 提供统一的问题排查与运行观察路径。

## 观测面划分

| 维度 | 关注点 |
|------|--------|
| 系统 | CPU、内存、磁盘、网络 |
| 接口 | 响应错误、慢请求、认证失败 |
| 任务 | 调度是否执行、执行结果、清理任务状态 |
| WebSocket | 在线连接数、集群广播、心跳保活 |
| 运行时 | Goroutine、GC、Heap、pprof |

## 已有能力

- 监控接口：`/admin/api/monitor`
- Welcome 监控卡片与趋势图
- 文件日志输出：`logger.path`, `logger.stdout=file`
- 运行时信息：Goroutines、Heap、GC 次数
- WebSocket 在线状态接口
- Redis 存在时自动启用 WebSocket 集群广播

## 问题排查顺序

### 1. 接口问题

- 先看状态码
- 再看后端日志
- 再看监控页资源占用

### 2. 任务问题

- 检查 `task.enable`
- 检查 cron 表达式是否为 6 段
- 检查任务 `checked_at` 是否变化

### 3. WebSocket 问题

- 检查连接是否建立
- 检查在线人数是否变化
- 检查 Redis 是否可用

## 推荐保留的日志关注点

- 登录失败
- 审计操作写入失败
- 告警发送失败
- Redis Pub/Sub 反序列化失败
- 定时任务执行失败

## pprof 与运行时建议

当出现持续性高 CPU / 高内存时：

1. 先看 Welcome 监控页
2. 再看 `/pprof` 导出信息
3. 再看任务执行和 WebSocket 在线连接数

## 容量评估建议

- 如果 WebSocket 在线连接持续上升，优先评估 Redis 与反向代理配置
- 如果日志量持续增长，优先检查日志清理任务和 retention 设置
- 如果任务数量增长，优先检查任务并发和执行时间

## 推荐阅读

- [四期路线图](/admin/phase-4-roadmap)
- [发布验证清单](/admin/release-verification-checklist)
