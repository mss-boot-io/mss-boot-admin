---
title: Admin 组件状态
order: 1
nav:
  title: Admin
  order: 3
description: v1.3.7 Admin 稳定版、v1.3.5/v1.3.6 永久停止记录与采用路径
---

# Admin v1.3.7 稳定状态

:::warning
发布状态：**v1.3.7 是当前稳定版**；v1.3.5 与 v1.3.6 已永久停止并保持不可变部分发布。
安装、创建或升级从[快速开始](/getting-started)进入。Docs 网站可通过 `docs/v*` 异步候补，
其状态不影响组件、稳定别名或采用。
:::

v1.3.5 是不可变部分发布，只公开了部分 Admin 组件：

- Go Module `github.com/mss-boot-io/mss-boot-admin/admin@v1.3.5`；
- Admin Web GitHub Release、GitHub Packages 与前端镜像身份；
- 编译期后端业务模块与显式前端业务路由；
- 统一认证、授权、配置、迁移、应用壳和页面状态。

但 Root Release 与工具、官方 npmjs 包 `@mss-boot-io/admin-web@1.3.5`、Docs 和后端
镜像未发布，v1.3.5 因而不是可创建或升级的完整 Admin Distribution。公开组件保持
不可变，不能用仓库源码、本地包、其他 registry 或相邻版本补齐。

版本采用判断从[快速开始](/getting-started)进入；本节其余页面记录架构和仓库合同，
不替代公共制品证据。v1.3.7 以 Thin Host 承载业务组合，不要求业务仓库复制核心启动逻辑。

## 参考入口

- [当前能力与边界](/admin/current-capabilities)
- [页面展示配置发布治理](/admin/presentation-configuration)
- [页面展示配置完整设计](/admin/presentation-complete-design)
- [配置指南](/admin/configuration-guide)
- [本地调试](/admin/local-debug)
- [集成验证](/admin/integration-test-guide)
- [容器部署](/admin/docker)
- [安全基线](/admin/security-baseline)
- [操作指南](/admin/operations-guide)
- [权限强化](/admin/permission-hardening)
- [登录故障排查](/admin/login-troubleshooting)

浏览器隐藏按钮不是授权。任何业务扩展都必须由后端执行正向和负向权限测试。
