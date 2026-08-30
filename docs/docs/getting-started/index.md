---
title: v1.3.7 候选采用状态
order: 1
nav:
  title: 采用状态
  order: 1
description: v1.3.2 当前稳定、v1.3.5/v1.3.6 永久停止与 v1.3.7 未稳定候选边界
keywords: [v1.3.7 v1.3.6 v1.3.5 v1.3.2 candidate immutable partial component availability]
---

# v1.3.7 候选采用状态

当前协调稳定发行版仍是 **v1.3.2**。v1.3.5 与 v1.3.6 已永久停止并保持不可变部分发布。v1.3.7
已选为 release candidate，但尚未稳定且不可采用。候选发布面可能处于不同公开阶段，必须以
远端发布台账为准；完整 stable promotion 和最终 policy/Docs 对账完成前，不提供安装、创建、
初始化或升级命令。

:::warning
v1.3.5 的 Framework、Admin、Admin Web 与 Root Tag 已公开，但 Root Release、工具、
Root 镜像、Docs Release 和公开 npmjs 包均未形成完整列车。不得用源码检出、本地替换、
GitHub Packages tarball 或混合版本绕过缺失门禁。

v1.3.6 的 Framework、Admin、Admin Web 与 Root Release/工具也已公开，但 Root image
和官方 npm 发布失败、Docs 未创建。不得 rerun 补发、恢复 npm token 或用 v1.3.7 源码
完成这个已经公开的版本。
:::

## 已公开与缺失矩阵

| 表面 | 状态 | 采用边界 |
| --- | --- | --- |
| v1.3.2 协调发行版 | 当前稳定 | 已有采用者继续以[稳定记录](/releases/archive/v1-3-2)为准 |
| v1.3.7 协调发行版 | 已选候选、未稳定 | 各发布面按阶段公开；等待完整 stable promotion 与最终 policy/Docs 对账 |
| v1.3.6 Framework/Admin/Admin Web/Root | 已公开 | 不可变部分列车；仅作为组件与审计证据 |
| v1.3.6 Root image/npmjs/Docs | 失败或缺失 | 永久不补发，不能组成公开 Thin Host |
| v1.3.5 Framework | Go Module 已公开 | 仅可作为独立组件使用 |
| v1.3.5 Admin | Go Module 已公开 | 仅可作为独立组件使用 |
| v1.3.5 Admin Web | GitHub Release、GitHub Packages 与前端镜像已公开 | npmjs 缺失，不能组成公开 Thin Host |
| v1.3.5 Root | annotated Tag 已公开 | Root Release、工具和 Root 镜像缺失 |
| v1.3.5 Docs | 未发布 | 当前公开 Docs 仍属于 v1.3.2 稳定线 |
| v1.3.5 mss-shop | 未建立可复现基线 | 不能从部分列车生成 |

## 当前采用决策

- 新应用或发行版升级：不要选择 v1.3.5 或 v1.3.6，也不要混用其组件；等待显式选择并完整对账的
  未使用版本。
- 已有 v1.3.2 应用：保持匹配版本的代码、配置、锁、数据库和制品，按
  [v1.3.2 稳定记录](/releases/archive/v1-3-2)维护。
- 只需要 Go 基础设施或 Admin 后端的开发者：先阅读
  [已发布组件与导入边界](/getting-started/packages)，并明确这不是完整发行版证明。
- 发行审计：查看 [v1.3.5 不可变部分发布记录](/releases/v1-3-5) 与
  [v1.3.6 不可变部分发布记录](/releases/v1-3-6)。

## 未来完整发行的必要条件

后续版本只有在 Root 工具、Go Module、公开 npmjs、Root 与前端镜像、Docs 和空目录
使用方验证全部绑定同一 merged-main 提交后，才会重新提供可复制的安装、创建、初始化、
开发和升级流程。单个 Tag、Release 或组件包不能提前开放采用路径。
