---
title: Skills 与 MCP 合同状态
order: 5
description: 面向人类的 Foundation 维护 Skill、Thin Host 分发 Skill 与 v1.3.7 候选 MCP 边界
---

# Skills 与 MCP 合同状态

:::warning
发布状态：v1.3.2 仍是当前稳定版；v1.3.5 与 v1.3.6 已永久停止并保持不可变部分发布；v1.3.7 已选
为 release candidate，但尚未稳定且不可采用。Root Release、`mss` 与 `mss-mcp` 可能处于
不同公开阶段，必须以远端发布台账为准；完整 stable promotion 和最终 current-stable policy
对账完成前，本页不是工具安装、应用创建、升级或 MCP 客户端配置指引。Docs 网站可异步候补
且不阻断该采用门禁。
:::

Skill 描述一类可复用工作流。源码实现由确定性 CLI、生成器和验证器承载，v1.3.7 候选
的 MCP 工具复用同一合同；Skill 不应复制实现逻辑。

## Foundation 维护者 Skills

Foundation 根 `.agents/skills/` 包含维护当前仓库的工作流：

- `mss-project-onboarding`：建立仓库与架构上下文；
- `mss-new-application`：从 Blueprint 创建新的独立管理系统；
- `mss-add-module`、`mss-add-field`、`mss-add-permission`、`mss-add-workflow`：按支持边界扩展能力；
- `mss-debug-fullstack`、`mss-review-change`：诊断与独立审查；
- `mss-upgrade-foundation`：协调 Distribution/Blueprint 升级；
- `mss-release`：候选、发布与公共制品对账；
- `mss-update-release-docs`：源码文档与生产 Docs 状态对账。

这些 Skills 调用 `mss` 和仓库验证器，不复制实现。发布、文档发布、新应用创建和
Foundation 工作流设计属于维护者能力，不自动进入下游应用。

## Thin Host 分发 Skills

生成应用模板只分发以下七个本地 Skills：

- `mss-thin-host`：项目所有权和日常生成/验证边界；
- `mss-add-module`：当前生成器支持的基础 CRUD 模块；
- `mss-add-field`：当前支持字段类型的前向演进；
- `mss-add-permission`：粗粒度后端 RBAC、API、菜单与动作权限；
- `mss-debug-fullstack`：可复现的全栈诊断；
- `mss-review-change`：安全、迁移、生成漂移与兼容性评审；
- `mss-upgrade-foundation`：协调 Admin Distribution 三方升级。

mss-add-workflow、关系字段生成、行级权限生成、Foundation 发布、文档发布与新应用
创建不在这套 Thin Host 分发集合中。仓库中存在维护者流程不代表下游已经获得该能力。

## 未来 MCP 合同

未来完整发行的 `mss-mcp` 是 stdio 长驻服务器。客户端通过 MCP `tools/list` 验证工具
清单；空目录最多允许 `mss_plan_application` 返回新应用只读计划，其余项目上下文、
规格、生成、验证和升级能力要求工作根存在有效 `.mss/project.yaml`。

写操作继续遵守 dry-run、路径限制、未知文件拒绝、参数校验和敏感信息脱敏；MCP 不获得
额外权限。客户端必须把目标 Thin Host 绝对路径作为显式工作根，不能猜测 Foundation
源码位置。

v1.3.5 没有可核验的 MCP 二进制，因此本页不展示启动命令或客户端 `command` 配置。
未来工具只有在同一完整 Root Release 中报告相同版本、源提交和 Blueprint 来源后，才能
成为采用者入口。

## 安全与可重复性

- 不从生产系统获取创建或验证所需数据；
- 不把 token、prompt、响应正文或 secret 写入报告；
- 工具不发送遥测、不登记采用者；
- 生成先看变更列表，升级先看三方计划；
- 对 MCP 返回的“完成”继续用仓库状态、测试和真实合同验证。
