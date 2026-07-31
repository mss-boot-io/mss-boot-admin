---
title: 四期路线图
order: 25
nav:
  order: 1
  title: admin
description: mss-boot-admin 四期演进规划与优先级排序
keywords: [admin roadmap phase4 planning enhancement]
---

## 概述

三期路线图已全部完成，产品主线（治理 + 运营 + AI 注解协同）已清晰。四期重点在于：

1. **运营能力深化**：补齐与竞品差距，提升运维效率
2. **安全能力增强**：补齐存储安全、审计追溯能力
3. **扩展能力完善**：WebSocket 集群、API 文档自动化
4. **国际化完善**：Accept-Language 处理、语言代码校验

## 四期优先级排序

| 优先级 | 能力线 | 说明 | 状态 |
|--------|--------|------|------|
| P1 | 运行时日志查看 | 运维排障基础能力 | ✅ 已完成 |
| P2 | 监控数据可视化 | 用户体验提升 | ✅ 已完成 |
| P3 | 存储安全增强 | 文件校验、权限控制 | ✅ 已完成 |
| P4 | 告警规则配置 | 主动发现问题 | ✅ 已完成 |
| P5 | WebSocket 集群支持 | 分布式部署 | ✅ 已完成 |
| P6 | API 文档自动化 | Swagger 完善 | ✅ 已完成 |
| P7 | 国际化完善 | Accept-Language、语言代码校验 | ✅ 已完成 |

---

## P1. 运行时日志查看 ✅

### 背景

当前系统缺少运行时日志查看能力，运维排障需要登录服务器查看日志文件。

### 目标

提供 Web 界面查看运行时日志，支持过滤、搜索、导出。

### 功能范围

| 功能 | 说明 | 状态 |
|------|------|------|
| 登录日志 | 记录用户登录时间、IP、状态 | ✅ 已完成 |
| 审计日志 | 记录用户操作行为 | ✅ 已完成 |
| 运行时日志 | 读取本地日志文件展示 | ✅ 已完成 |
| 日志清理 | 定时清理过期日志 | ✅ 已完成 |

### 实现说明

已实现：
- 登录日志记录：`middleware/auth.go` 在登录成功/失败时记录
- 审计日志记录：`middleware/audit.go` 记录 POST/PUT/DELETE 操作
- 运行时日志配置：`config/application.yml` 设置 `logger.stdout: file` 和 `path: logs`
- 日志清理任务：`service/log_cleaner.go` 注册为 `log_cleaner` 任务函数

### 验收标准

- [x] 登录日志正确记录
- [x] 审计日志正确记录
- [x] 运行时日志写入文件
- [x] 日志清理任务可配置

---

## P2. 监控数据可视化 ✅

### 背景

当前监控数据仅 API 返回 JSON，无前端图表展示。

### 目标

提供可视化监控仪表盘，实时展示系统状态。

### 功能范围

| 功能 | 说明 | 状态 |
|------|------|------|
| CPU 使用率趋势 | 使用率历史曲线 | ✅ 已完成 |
| 内存使用率趋势 | 使用量历史曲线 | ✅ 已完成 |
| 磁盘使用展示 | 各分区使用率 | ✅ 已完成 |
| 运行时信息 | Goroutine、GC、堆内存 | ✅ 已完成 |
| 实时刷新 | 可配置刷新间隔 | ✅ 已完成 |

### 实现说明

已实现：
- 后端监控接口：`apis/monitor.go` 返回 CPU/内存/磁盘/网络/运行时信息
- 前端监控图表：`pages/Welcome.tsx` 使用 Ant Design Charts 展示趋势图
- 数据精度：CPU/内存/磁盘使用率保留2位小数
- 单位转换：磁盘容量显示为 GB

### 验收标准

- [x] CPU 使用率趋势图
- [x] 内存使用趋势图
- [x] 磁盘使用图表
- [x] 自动刷新间隔可配置

---

## P3. 存储安全增强

### 背景

当前存储能力缺少安全校验，存在恶意文件上传风险。

### 目标

增加文件类型、大小校验，细粒度权限控制。

### 功能范围

| 功能 | 说明 |
|------|------|
| 文件类型白名单 | 只允许指定类型上传 |
| 文件大小限制 | 单文件最大限制 |
| MIME 类型校验 | 检测真实文件类型 |
| 存储权限控制 | Casbin 权限集成 |
| 上传审计日志 | 记录上传操作 |

### 实现方案

**后端配置：**
```yaml
storage:
  maxSize: 10485760  # 10MB
  allowedTypes:
    - image/jpeg
    - image/png
    - image/gif
    - application/pdf
    - text/plain
  checkMime: true
```

**代码实现：**
```go
// service/storage.go
func (s *Storage) validateFile(file *multipart.FileHeader) error {
    // 大小校验
    if file.Size > maxSize {
        return errors.New("file too large")
    }
    // 类型校验
    if !isAllowedType(file.Header.Get("Content-Type")) {
        return errors.New("file type not allowed")
    }
    // MIME 校验
    if checkMime {
        actualType := detectMIME(file)
        if !isAllowedType(actualType) {
            return errors.New("MIME type mismatch")
        }
    }
    return nil
}
```

### 验收标准

- [x] 文件大小超限时拒绝上传
- [x] 非白名单类型拒绝上传
- [x] MIME 类型校验生效
- [x] 上传操作记录审计日志

---

## P4. 告警规则配置 ✅

### 背景

当前系统无告警能力，问题依赖人工发现。

### 目标

支持配置告警规则，触发后通过多种渠道通知。

### 功能范围

| 功能 | 说明 | 状态 |
|------|------|------|
| 告警规则配置 | 指标、阈值、触发条件 | ✅ 已完成 |
| 系统内通知 | WebSocket 推送 | ✅ 已完成 |
| 邮件通知 | SMTP 发送 | ✅ 已完成 |
| 钉钉通知 | Webhook 推送 | ✅ 已完成 |
| 企业微信通知 | Webhook 推送 | ✅ 已完成 |
| 告警历史 | 历史告警记录 | ✅ 已完成 |

### 实现说明

已实现：
- 告警规则模型：`models/alert.go` 定义 AlertRule 和 AlertHistory
- 告警检查服务：`service/alert_checker.go` 定时检查规则并触发告警
- 通知渠道实现：`service/notification_channel.go` 支持 Email/DingTalk/WeChat
- 配置项：`config/notification.go` 定义通知渠道配置

### 配置示例

```yaml
notification:
  email:
    enabled: true
    host: "smtp.example.com"
    port: 587
    username: "alert@example.com"
    password: "xxx"
    from: "alert@example.com"
  dingtalk:
    enabled: true
    webhook: "https://oapi.dingtalk.com/robot/send?access_token=xxx"
    secret: "xxx"
  wechat:
    enabled: true
    webhook: "https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=xxx"
```

### 验收标准

- [x] 可配置告警规则
- [x] 可配置通知渠道
- [x] 告警触发后发送通知
- [x] 告警历史可查询

---

## P5. WebSocket 集群支持 ✅

### 背景

当前 WebSocket 为单机模式，不支持分布式部署。

### 目标

支持多实例部署，会话共享。

### 实现说明

已实现（`center/websocket/manager.go`）：
- Redis Pub/Sub 集成：自动检测 Redis 并启用集群模式
- 广播消息同步：通过 `websocket:cluster:broadcast` 频道
- 用户消息同步：通过 `websocket:cluster:usercast` 频道
- 连接管理：支持用户多连接、心跳检测、过期清理

### 配置要求

```yaml
cache:
  redis:
    addr: 'localhost:6379'
    password: ''
    db: 0
```

### 验收标准

- [x] 多实例部署后消息正常推送
- [x] 用户连接任一实例都能收到消息
- [x] 自动检测 Redis 启用集群模式

---

## P6. API 文档自动化 ✅

### 背景

当前 Swagger 注解不完整，API 文档不全面。

### 目标

完善 Swagger 注解，自动生成完整 API 文档。

### 实现说明

已完成：
- 主要 API 文件均有 Swagger 注解
- 启动时自动生成 Swagger 文档
- 访问 `/swagger/index.html` 查看文档

### 验收标准

- [x] 主要 API 有 Swagger 注解
- [x] Swagger UI 可访问
- [x] 响应格式有示例

---

## P7. 国际化完善 ✅

### 背景

当前国际化缺少 Accept-Language 处理和语言代码校验。

### 实现说明

已完成（`middleware/language.go`）：
- Accept-Language 请求头解析
- Content-Language 响应头设置
- URL 参数 `?lang=xx` 支持
- 语言代码格式校验（ISO 639-1）
- 支持语言：zh-CN, en-US

### 语言代码校验

`models/language.go` 中的 `validateName()` 方法强制校验语言代码格式：
```go
matched, _ := regexp.MatchString(`^[a-z]{2,3}(-[A-Z]{2,4})?$`, l.Name)
```

### 验收标准

- [x] 请求自动匹配 Accept-Language
- [x] 语言代码格式强制校验

---

## 四期里程碑

| 里程碑 | 内容 | 状态 |
|--------|------|------|
| M1 | 运行时日志查看 + 监控可视化 | ✅ 已完成 |
| M2 | 存储安全 + 告警配置 + 通知渠道 | ✅ 已完成 |
| M3 | WebSocket 集群 + API 文档 | ✅ 已完成 |
| M4 | 国际化完善 + 收尾 | ✅ 已完成 |

## 四期完成总结

四期路线图已全部完成，主要成果：

**运维能力增强：**
- ✅ 运行时日志查看（登录日志、审计日志、运行时日志）
- ✅ 监控数据可视化（CPU/内存趋势图、磁盘展示）
- ✅ 日志清理任务（定时清理过期日志）

**安全能力增强：**
- ✅ 存储安全增强（文件类型/大小校验、MIME检测）
- ✅ 告警规则配置（CPU/内存/磁盘监控）
- ✅ 多渠道通知（Email/DingTalk/WeChat）

**架构能力增强：**
- ✅ WebSocket 集群支持（Redis Pub/Sub）
- ✅ API 文档自动化（Swagger 完善）
- ✅ 国际化完善（Accept-Language 处理）

---

## 推荐阅读

- [三期路线图](/admin/phase-3-roadmap)
- [运营能力说明](/admin/operations-guide)
- [运营能力规划](/admin/operations-planning)
- [集成与扩展护栏](/admin/extension-guardrails)