---
title: Admin
order: 1
nav:
  title: Admin
  order: 3
description: v1.3.5 完整 Admin 后端与浏览器应用的使用和扩展入口
---

# Admin v1.3.5

Admin 是一个协调发布的完整产品，不是业务仓库需要复制的模板：

- Go Module `github.com/mss-boot-io/mss-boot-admin/admin@v1.3.5`；
- npm Package `@mss-boot-io/admin-web@1.3.5`；
- 编译期后端业务模块与显式前端业务路由；
- 统一认证、授权、配置、迁移、应用壳和页面状态。

这些 v1.3.5 身份在协调发行完成公共对账后才可供下游解析；候选期只描述组合边界，
不允许用仓库源码或相邻版本代替公共包。

新应用从 [快速开始](/getting-started)生成 Thin Host；不要手工拼装核心启动逻辑。

## 参考入口

- [当前能力与边界](/admin/current-capabilities)
- [配置指南](/admin/configuration-guide)
- [本地调试](/admin/local-debug)
- [集成验证](/admin/integration-test-guide)
- [容器部署](/admin/docker)
- [安全基线](/admin/security-baseline)
- [操作指南](/admin/operations-guide)
- [权限强化](/admin/permission-hardening)
- [登录故障排查](/admin/login-troubleshooting)

浏览器隐藏按钮不是授权。任何业务扩展都必须由后端执行正向和负向权限测试。
