---
title: Agent 原生开发基础设施
order: 1
nav:
  title: Agent 开发
  order: 2
description: 使用 Codex、Claude Code、other coding agents、Cursor 等编码 Agent 开发管理系统的统一基础设施
keywords: [agent codex claude other coding agents cursor mss management system]
---

# Agent 原生管理系统开发基础设施

`mss-boot-admin` 不再只是一套可运行的 Go + React 管理后台参考实现。它同时提供编码 Agent 开发管理系统所需的上下文、规格、生成、环境、验证、升级和评测基础设施。

完成形态遵循一条固定链路：

```text
自然语言需求
  ↓
FeatureSpec：目标、非目标、角色、约束、风险、验收和回滚
  ↓
AdminModule：字段、权限、菜单、页面和测试契约
  ↓
确定性生成器：后端、迁移、权限、前端、文档和测试骨架
  ↓
Agent：实现非模板化业务规则
  ↓
mss verify：根据 Git Diff 选择并执行充分验证
  ↓
Agent Eval：验证基础设施本身仍然可被 Agent 正确使用
```

## 核心原则

### 一个事实源

以下内容共同组成事实源，而不是为每个 Agent 维护一份不同的项目说明：

- `AGENTS.md`：人类与 Agent 共读的仓库边界；
- `.mss/project.yaml`：技术栈和目录；
- `.mss/capabilities.yaml`：已有能力及生命周期；
- `.mss/commands.yaml`：规范命令；
- `.agents/skills/`：可复用工作流；
- `mss` CLI：确定性实现；
- MCP：将同一套实现暴露给支持 MCP 的 Agent。

`CLAUDE.md`、other coding agents instructions、Cursor rules 和 Codex 配置只是薄适配层，不复制架构真相。

### 自然语言不直接生成最终代码

自然语言用于表达意图；结构化规格负责消除歧义；生成器处理机械代码；Agent 处理业务推理；测试和契约决定是否完成。

### 写操作默认需要显式授权

- `mss module generate` 默认 dry-run；
- `mss new app` 默认 dry-run；
- `mss upgrade plan` 只读；
- `mss upgrade apply` 需要 `--yes`；
- MCP 写工具默认进入 `writes` 审批模式。

### 完成由证据定义

Agent 不能仅以“代码已写完”声明完成。它必须给出：

- 实际执行命令；
- 失败与跳过项；
- `verify` 或 Eval 报告；
- 数据库、权限、兼容性和回滚影响；
- 尚未解决的风险。

## 能力地图

| 能力 | 命令或路径 | 当前状态 |
| --- | --- | --- |
| 项目上下文 | `mss context`、`.mss/` | Beta |
| 环境诊断 | `mss doctor` | Beta |
| 无交互初始化 | `mss setup` | Beta |
| 开发进程管理 | `mss dev` | Beta |
| Feature/Acceptance 契约 | `.mss/features/`、`mss spec validate` | Beta |
| 管理模块契约 | `.mss/modules/`、`mss spec validate` | Beta |
| 确定性模块生成 | `mss module generate` | Beta |
| Agent Skills | `.agents/skills/`、`mss skills` | Beta |
| 项目 MCP | `cmd/mss-mcp`、`.codex/config.toml` | Beta |
| 变更感知验证 | `mss verify` | Beta |
| Agent Evals | `mss eval` | Beta |
| 新应用 Blueprint | `mss new app` | Beta |
| 三方 Foundation 升级 | `mss upgrade` | Beta |
| 运行时动态模型 | 历史实现 | Legacy |

## 快速入口

```shell
# 查看项目事实
go run ./cmd/mss context --format json

# 检查当前环境
go run ./cmd/mss doctor --format json

# 安装本地依赖和安全开发状态
go run ./cmd/mss setup

# 启动后端和前端
go run ./cmd/mss dev --detach

# 查看状态和日志
go run ./cmd/mss dev status
go run ./cmd/mss dev logs backend

# 验证当前变更
go run ./cmd/mss verify --changed

# 验证基础设施本身
go run ./cmd/mss eval run --all
```

## 推荐阅读顺序

1. [开箱即用](/agent/getting-started)
2. [Feature 与模块规格](/agent/specifications)
3. [Skills 与 MCP](/agent/skills-and-mcp)
4. [应用 Blueprint 与升级](/agent/blueprints-and-upgrades)
5. [验证与 Agent Evals](/agent/verification-and-evals)
6. [架构与演进边界](/agent/architecture)
