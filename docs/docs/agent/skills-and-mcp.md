---
title: Skills 与 MCP
order: 5
description: 面向人类的 v1.3.7 Foundation 维护 Skill、Thin Host 分发 Skill 与 MCP 安全合同
---

# Skills 与 MCP

本页向人类说明 AI Agent 的本地工作流边界。真正的执行权威是目标仓库内最近的
`AGENTS.md`、`.mss/` 和适用 `.agents/skills/`，网页不是 Agent 的授权来源。v1.3.7
是当前协调稳定版；Docs 网站可以独立异步候补，其状态不阻断 `mss`、`mss-mcp` 或其他
组件采用。

Skill 描述一类可复用工作流；确定性 CLI、生成器和验证器承载实现。Skill 不应复制业务
逻辑，也不能扩大人类给定的修改范围。

## Foundation 维护者 Skills

Foundation 根 `.agents/skills/` 包含维护当前源仓库的工作流：

- `mss-project-onboarding`：建立仓库、机器合同与架构上下文；
- `mss-new-application`：从正式 Blueprint 创建新的独立管理系统；
- `mss-add-module`、`mss-add-field`、`mss-add-permission`、`mss-add-workflow`：按支持边界扩展能力；
- `mss-debug-fullstack`、`mss-review-change`：复现故障和独立审查；
- `mss-upgrade-foundation`：协调 Distribution/Blueprint 升级；
- `mss-release`：候选、正式发布与公共制品对账；
- `mss-update-release-docs`：源码文档和独立生产 Docs 状态对账。

这些 Skills 可以调用 Foundation 仓库在 `.mss/commands.yaml` 中声明的源码验证入口和
发布验证器。发布、Docs 网站发布与 Foundation 自身变更都是维护者能力，不会因为源仓库
中存在相应文件就自动授予下游 Agent。

## v1.3.7 MCP 合同

正式 `mss-mcp` 是 **stdio** 长驻服务器，并与 `mss` 报告同一个 v1.3.7、源提交与构建
时间。客户端通过 MCP `tools/list` 核对工具清单；空目录最多允许
`mss_plan_application` 返回新应用只读计划，其余规格、生成、验证和升级能力要求工作根
存在有效 `.mss/project.yaml`。

人类应把目标 Thin Host 的绝对路径作为显式 `--root`，不要让客户端猜测 Foundation
源码位置。标准客户端结构和排错步骤见[工具与 MCP](/getting-started/tooling)。协议写入
继续遵守 dry-run、显式确认、路径限制、未知文件拒绝、参数校验与敏感信息脱敏；Agent
不会通过 MCP 获得额外文件、网络、Secret 或发布权限。

## 安全与可重复性

- 不从生产系统获取创建、生成或验证所需数据；
- 不把 token、prompt、响应正文或 secret 写入报告；
- 工具不发送遥测、不登记采用者；
- 生成先审变更列表，升级先审三方计划；
- 对 MCP 返回的“完成”继续用仓库状态、测试、生成漂移和真实运行时合同验证；
- Foundation 发布和 Docs 网站发布始终走各自受审流程，不能由普通 Thin Host Agent 触发。

## Thin Host 分发 Skills

v1.3.7 生成应用模板只分发以下七个本地 Skills：

- `mss-thin-host`：项目所有权和日常生成、验证边界；
- `mss-add-module`：当前生成器支持的基础 CRUD 模块；
- `mss-add-field`：当前支持字段类型的前向演进；
- `mss-add-permission`：粗粒度后端 RBAC、API、菜单与动作权限；
- `mss-debug-fullstack`：可复现的全栈诊断；
- `mss-review-change`：安全、迁移、生成漂移与兼容性评审；
- `mss-upgrade-foundation`：协调 Admin Distribution 三方升级。

工作流生成、关系字段生成、行级权限生成、Foundation 发布、Docs 发布和新应用创建不在
这套 Thin Host 分发集合中。需要超出集合的能力时，由人类先补充规格和权限边界，再选择
Foundation 贡献流程或手写设计；下游 Agent 不得自行借用维护者 Skill。
