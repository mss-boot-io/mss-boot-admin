---
title: 源码能力与发布边界
order: 2
description: v1.3.7 稳定版源码能力、v1.3.5/v1.3.6 永久停止记录与采用边界
---

# v1.3.7 稳定版源码能力与发布边界

:::warning
发布状态：**v1.3.7 是当前稳定版**；v1.3.5 与 v1.3.6 已永久停止并保持不可变部分发布。
安装、创建或升级从[快速开始](/getting-started)进入。Docs 网站可通过 `docs/v*` 异步候补，
其状态不影响组件、稳定别名或采用。
:::

机器可执行事实以 [`.mss/project.yaml`](https://github.com/mss-boot-io/mss-boot-admin/blob/main/.mss/project.yaml)
和 [`.mss/capabilities.yaml`](https://github.com/mss-boot-io/mss-boot-admin/blob/main/.mss/capabilities.yaml)
为准。v1.3.7 的公共制品已经完成对账；后续源码能力仍不能冒充新的公共版本。

## merged-main 源码能力

| 范围 | 源码合同 |
| --- | --- |
| 后端 | 会话认证、Casbin 授权、配置提供方、迁移、缓存、任务、通知、对象存储、健康检查 |
| Admin Web | React 19、Ant Design 6、Umi Max、React Query、登录壳、路由、页面状态、主题和 locale |
| 业务扩展 | 编译期 Go 模块、确定性菜单与路由、迁移就绪、生成合同 |
| Agent 工作流 | 规格、生成、诊断、开发、验证、评估和三方升级实现 |
| 发行治理 | 组件资格、受保护发布、制品来源与外部使用方验证 |

这些能力供 Foundation 贡献者在源码工作区验证。一般源码 `doctor`、`verify` 和生成
合同可以保留，但不能被描述成部分列车已交付的完整下游工具。

## v1.3.5 与 v1.3.6 公共结果

- v1.3.5 公开了 Framework、Admin、Admin Web Release/Packages/前端镜像与 Root Tag，
  但 Root Release/工具、官方 npmjs、Docs、后端镜像和完整 Thin Host 使用方资格未完成；
- v1.3.6 从提交 `b1fe47a3a83209574e09d53526b122dd2cbc5277` 公开了 Framework、Admin、
  Admin Web 与 Root Release/工具，但 Root image workflow 和官方 npm workflow 失败，
  Docs 未创建。

两条列车都已永久停止，不能用源码、相邻版本、本地替换或其他 registry 补齐，也不能
继续、补发或与 v1.3.7 修复混用。v1.3.7 已完成 Root 工具、Go/npm 包、镜像和外部使用方
公共对账，才成为本页所述的当前可执行采用路径。

## 明确边界

- Thin Host 不复制 Admin、Framework 或完整前端源码；
- 不提供浏览器运行时建模、虚拟 CRUD 或远程代码加载；
- 不自动把 UI 权限当作后端授权；
- 不要求生产凭据完成源码验证；
- 不在工具设计中加入遥测和采用者登记；
- `mss-shop` 必须以已完成公共对账的 v1.3.7 为基础，并独立证明其单租户范本边界。

后续版本只有在 Root 工具、Go/npm 包、镜像与外部使用方证据全部公开对账后，才能把
源码能力转成新的可执行采用路径；Docs 网站继续在独立链路对账，不加入该采用门禁。
