---
title: 源码能力与发布边界
order: 2
description: v1.3.7 候选源码能力、v1.3.5/v1.3.6 永久停止记录与未开放采用路径
---

# v1.3.7 候选源码能力与发布边界

:::warning
发布状态：v1.3.2 仍是当前稳定版；v1.3.5 与 v1.3.6 已永久停止并保持不可变部分发布；
v1.3.7 已选为 release candidate，但尚未发布。本页只区分 Foundation 源码实现、历史
公开组件和候选合同；公共制品对账前，v1.3.7 不可采用，也不开放安装、创建或升级命令。
:::

机器可执行事实以 [`.mss/project.yaml`](https://github.com/mss-boot-io/mss-boot-admin/blob/main/.mss/project.yaml)
和 [`.mss/capabilities.yaml`](https://github.com/mss-boot-io/mss-boot-admin/blob/main/.mss/capabilities.yaml)
为准。源码能力不等于公共制品已经完成对账。

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

- Framework 与 Admin Go Module 已公开；
- Admin Web 的 GitHub Release、GitHub Packages 与前端镜像已公开；
- Root Tag 已公开并保持不可变；
- Root Release 与工具、官方 npmjs、Docs、后端镜像和完整 Thin Host 使用方资格未完成。

因此不能用源码、相邻版本、本地替换或其他 registry 补齐部分发布。

v1.3.6 也从提交 `b1fe47a3a83209574e09d53526b122dd2cbc5277`
公开了 Framework、Admin、Admin Web 与 Root Release/工具；其 Root image workflow 和
官方 npm workflow 失败，Docs 未创建。它同样不能继续、补发或与 v1.3.7 修复混用。

## 明确边界

- Thin Host 不复制 Admin、Framework 或完整前端源码；
- 不提供浏览器运行时建模、虚拟 CRUD 或远程代码加载；
- 不自动把 UI 权限当作后端授权；
- 不要求生产凭据完成源码验证；
- 不在工具设计中加入遥测和采用者登记；
- `mss-shop` 必须等待 v1.3.7 完成公共对账，且保持单租户边界。

未来版本只有在 Root 工具、Go/npm 包、镜像、Docs 与外部使用方证据全部公开对账后，
才能把源码能力转成可执行采用路径。
