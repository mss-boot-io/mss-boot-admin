---
title: 当前能力与边界
order: 2
description: v1.3.4 Complete Admin Distribution 的实现能力与明确非目标
---

# v1.3.4 当前能力与边界

机器可执行事实以 [`.mss/project.yaml`](https://github.com/mss-boot-io/mss-boot-admin/blob/main/.mss/project.yaml)
和 [`.mss/capabilities.yaml`](https://github.com/mss-boot-io/mss-boot-admin/blob/main/.mss/capabilities.yaml)
为准；本页提供使用者视角的摘要。

## 已提供

| 领域 | 能力 |
| --- | --- |
| 身份与授权 | 登录、浏览器会话、OAuth 绑定、个人凭据自助、Casbin 后端授权、短期 WebSocket ticket |
| Admin 运行时 | 配置、数据库迁移、健康与就绪、日志、缓存、任务、通知和可选资源 |
| 完整前端 | React 19、Ant Design 6、Umi Max、React Query、响应式页面、zh-CN/en-US |
| 业务扩展 | 编译期 Go 模块、确定性菜单与路由、迁移就绪、生成合同 |
| Agent 工作流 | 规格、生成、doctor、setup、dev、verify、eval 和三方升级 |
| 发行 | 精确版本 Go/npm 包、跨平台工具、容器、Docs 与来源对账 |

## 明确边界

- Thin Host 不复制 Admin、Framework 或完整前端源码；
- 不提供浏览器运行时建模、虚拟 CRUD 或远程代码加载；
- 不自动把 UI 权限当作后端授权；
- 不要求生产凭据完成创建、设置或验证；
- 不在工具安装或运行中加入遥测和采用者登记；
- `mss-shop` 的初始业务范本是显式单租户，不暗示多租户隔离。

## 状态判定

本地可执行：

```sh
mss doctor --strict
mss context --format json
mss verify --changed
```

进程健康、页面能打开或工作流单个 Job 成功都不是完整业务证据。变更应同时覆盖实际
API、后端权限、迁移、前端状态和必要浏览器流程。
