---
title: Admin 发布与部署验证清单
order: 28
nav:
  order: 1
  title: admin
description: 完整 Thin Host 业务发行前后的最小验证清单
keywords: [admin release checklist smoke regression]
---

## 使用方式

:::warning
v1.3.5 已永久停止为不可变部分发布，缺少 Root 工具、官方 npmjs、Docs 和完整 Thin Host
路径。本清单不能把 v1.3.5 变成可安装或可升级版本，也不能授权补发其缺失制品。当前稳定
资料见 [v1.3.2 稳定记录](/releases/archive/v1-3-2)。
:::

本清单区分未来完整版本或业务 Thin Host 的代码门禁与生产部署巡检。Foundation 发布方先
在唯一 Root preview 中完成测试、审计、浏览器、归档和多架构资格，再依次发布组件 Tag；
一个 Root Tag 独立并行触发 Root Release、后端镜像和官方 npmjs，Docs 从后续文档 Tag
发布。正式 Tag 不重复昂贵验证，也不接受 promotion、readiness run ID 或人工环境审批。

## 一、发布前检查

### 构建检查

- [ ] Thin Host 根目录执行 `mss verify --all` 成功
- [ ] 后端和 Admin Web 依赖都精确为同一个已完成公共对账的协调版本，锁文件无漂移
- [ ] 文档如有更新，关键链接可访问

### 核心能力检查

- [ ] 登录成功且首次登录状态正常
- [ ] Welcome 页监控卡片与趋势图正常
- [ ] 日志页面能看到登录日志、审计日志、运行时日志
- [ ] 仅在显式 Local 开发 Delivery 中执行头像上传 smoke，并确认 `/public/uploads/{uuid}` 内容一致；该结果不作为生产 Provider 证据
- [ ] 当 Local/S3-compatible 仍为 Legacy / Blocked 时，生产 ingress 已阻断两个上传路径，且未授予通用 `storage:upload` 权限作为纵深防御；头像入口没有独立 Casbin permission
- [ ] 告警规则可以保存
- [ ] 已启用任务状态正常
- [ ] 国际化接口可返回有效语言资源

## 二、发布后冒烟检查

- [ ] `/healthz` 正常
- [ ] 前端首页可打开
- [ ] WebSocket 在线状态正常
- [ ] `/public/` 未被当作生产对象 Delivery 代理，两个上传入口仍被部署侧阻断
- [ ] 后端日志无持续性 fatal 错误

## 三、异常归属建议

| 现象 | 优先排查 |
|------|----------|
| 前端空白页 | 前端构建、代理、接口鉴权 |
| 监控图表无数据 | `/admin/api/monitor` 的 `collectedAt`/`stale`/`instanceId`、内置系统作业日志、登录态 |
| 头像入口被禁用 | 当前 Provider / Delivery evidence 状态、ingress 与权限门禁；不要通过临时开放 `/public/` 绕过 |
| 告警不发送 | 通知配置、规则状态、日志输出 |
| 用户任务未执行 | `task.enable`、cron 表达式、任务状态、TaskRun |
| 内置监控/会话清理未执行 | task server 启动日志和系统作业错误；不要以 Task/TaskRun 无记录判定失败 |

## 四、回滚前确认

- [ ] 问题是否来自配置而非代码
- [ ] 数据库结构是否发生变化
- [ ] 若开发环境曾显式启用 Local，已有 opaque-key 对象是否已保留；不要回滚到解析后限流或 user/filename key
- [ ] 日志目录是否需要保留
- [ ] 通知渠道凭据是否改动

## 推荐阅读

- [集成测试指南](/admin/integration-test-guide)
- [v1.3.5 不可变部分发布记录](/releases/v1-3-5)
- [当前采用状态](/getting-started)
