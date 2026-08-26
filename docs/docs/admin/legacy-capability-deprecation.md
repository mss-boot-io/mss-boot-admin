---
title: 运行时开发工具移除与升级说明
order: 24
nav:
  order: 1
  title: admin
description: Admin 运行时动态模型、虚拟 CRUD 与浏览器代码生成的移除范围、数据保留和迁移路径
keywords: [admin removed capability virtual model code generation migration]
---

## 适用范围

本文适用于 v1.3.5 Admin 产品和生产运行时。开发期确定性生成继续由 Release 中安装的
`mss` 工具提供，与已经删除的浏览器运行时生成能力互不相同。

## 当前状态

以下能力已经从 Admin 产品中移除，当前状态为 **已删除**：

| 已移除能力 | 不再提供的入口 |
| --- | --- |
| 动态模型与字段管理 | Go model/field 实现、管理 API、前端页面、菜单和权限 |
| 虚拟 CRUD | 运行时虚拟模型框架、动态路由和前端虚拟模型页面 |
| 浏览器模板/代码生成 | Admin 模板 API、Git 模板处理、生成页面、菜单和权限 |

新安装不会创建这些运行时入口或对应的初始化菜单。任何新模块都不得依赖这些
已移除接口。

## 升级和数据保留

升级迁移会以事务方式清理已移除功能的菜单、API 元数据和对应 Casbin 策略，
包括历史动态菜单的后代节点。迁移是幂等的，不应影响无关菜单或权限。

若下游把无关菜单直接挂在 `/develop` 下，迁移会把该菜单提升到 `/develop` 的父级。
相对路由会改写为原先实际生效的 `/develop/...` 绝对路由，并复制精确匹配的
Casbin 策略；旧策略不会被删除，已存在的目标策略也不会被覆盖或重复创建。
数据库语言包中能由幸存菜单名称可靠识别的 `menu.develop.<child>` 定义，会在目标键
不存在时复制为 `menu.<child>`。旧定义仍会保留。

为避免不可逆的数据丢失，升级过程不会自动删除：

- 历史动态模型和字段元数据表；
- 由历史虚拟模型创建的业务表及其中的数据。
- 无法通过幸存菜单可靠归属的历史语言定义。

这些表和语言定义的保留只是数据保护措施，不表示运行时能力仍然可用。历史语言定义
没有可靠的功能来源标记，因此自动迁移不猜测、不批量删除。确认完成数据导出、业务迁移
和备份之前，不要手工删除它们。

## 推荐迁移路径

1. 盘点历史模型元数据、字段定义、业务表、角色和调用方。
2. 为仍需使用的数据定义显式 Go model、DTO、service、handler 和权限规则。
3. 添加 forward-compatible migration，把历史数据迁入受版本控制的表结构。
4. 补齐 OpenAPI、前端类型、菜单、正反向权限测试和升级路径测试。
5. 在备份和业务验收完成后，由运维人员通过单独、可回滚的变更清理遗留表。

标准管理模块可以用结构化 `AdminModuleSpec` 描述，并在开发工作树中执行：

```shell
mss module generate .mss/modules/<module>.yaml --format json
mss module generate .mss/modules/<module>.yaml --write --format json
mss verify --changed
```

生成结果必须像普通源码一样经过审查、编译、迁移和权限验证后再部署。

## `mss` 为什么继续保留

v1.3.5 候选合同将 `mss` 作为 Root Release 工具发布，公共对账完成后供 Thin Host
开发期使用。它与已移除的 Admin 浏览器生成器有不同的信任边界：

| `mss` 确定性生成器 | 已移除的 Admin 浏览器生成器 |
| --- | --- |
| 读取受版本控制的结构化规格 | 在运行时页面接受模板和仓库参数 |
| 默认 dry-run，限制写入仓库根目录 | 由 Admin API 执行模板/Git 操作 |
| 输出可编译、可测试、可审查的源码 | 生产 Admin 中提供生成入口 |
| 通过幂等、golden 和路径约束测试 | 已从当前产品删除 |

因此，“移除代码生成”专指 Admin 产品中的浏览器/运行时代码生成，不包括
`mss module generate`、应用 Blueprint 或升级工具。

## 回滚边界

应用版本回滚不会自动恢复已经清理的菜单和 Casbin 策略。若必须回滚到旧版本，
应从升级前备份恢复数据库，或按旧版本的受审计迁移重新创建必要元数据；不要仅
依赖保留下来的历史业务表恢复运行时路由。

## 推荐阅读

- [当前功能总览](/admin/current-capabilities)
- [权限与组织治理说明](/admin/governance-guide)
- [Agent 原生基础设施](/architecture/agent-native-foundation)
