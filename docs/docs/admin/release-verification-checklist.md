---
title: 发布验证清单
order: 28
nav:
  order: 1
  title: admin
description: mss-boot-admin 发布前后的最小验证清单
keywords: [admin release checklist smoke regression]
---

## 使用方式

本清单适用于每次版本发布、重要配置变更、生产部署后巡检。

## 一、发布前检查

### 构建检查

- [ ] 后端 `go build ./...` 成功
- [ ] 前端 `pnpm -s tsc --noEmit` 成功
- [ ] 文档如有更新，关键链接可访问

### 核心能力检查

- [ ] 登录成功且首次登录状态正常
- [ ] Welcome 页监控卡片与趋势图正常
- [ ] 日志页面能看到登录日志、审计日志、运行时日志
- [ ] 头像上传成功并可访问
- [ ] 告警规则可以保存
- [ ] 已启用任务状态正常
- [ ] 国际化接口可返回有效语言资源

## 二、发布后冒烟检查

- [ ] `/healthz` 正常
- [ ] 前端首页可打开
- [ ] WebSocket 在线状态正常
- [ ] `/public/` 资源访问正常
- [ ] 后端日志无持续性 fatal 错误

## 三、异常归属建议

| 现象 | 优先排查 |
|------|----------|
| 前端空白页 | 前端构建、代理、接口鉴权 |
| 监控图表无数据 | `/admin/api/monitor` 返回结构、登录态 |
| 头像无法显示 | `/public/` 代理、返回 URL、静态目录 |
| 告警不发送 | 通知配置、规则状态、日志输出 |
| 任务未执行 | `task.enable`、cron 表达式、任务状态 |

## 四、回滚前确认

- [ ] 问题是否来自配置而非代码
- [ ] 数据库结构是否发生变化
- [ ] 上传或日志目录是否需要保留
- [ ] 通知渠道凭据是否改动

## 推荐阅读

- [集成测试指南](/admin/integration-test-guide)
- [生产部署标准化](/admin/production-standardization)
