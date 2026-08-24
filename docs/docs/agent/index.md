---
title: Agent 开发
order: 1
nav:
  title: Agent 开发
  order: 4
description: 使用已安装 mss 与仓库合同规划、生成和验证管理系统变更
---

# Agent 开发

Agent 与人使用同一组机器合同和命令。先按[快速开始](/getting-started)创建 Thin Host，
再在目标仓库中执行工作；不要依赖 Foundation 源码或隐藏对话上下文。

## 合同层次

| 位置 | 作用 |
| --- | --- |
| `AGENTS.md` | 人可读边界、所有权和验证要求 |
| `.mss/project.yaml` | 工具链、布局、组件和版本 |
| `.mss/capabilities.yaml` | 已有能力与成熟度 |
| `.mss/commands.yaml` | 可执行命令目录 |
| `.mss/features/` | 跨模块需求、约束、验收和风险 |
| `.mss/modules/` | 垂直业务模块规格 |
| `.mss/lock.yaml` | Distribution、Blueprint 和升级记录 |

优先级发生冲突时，以编译代码和迁移、机器合同、测试证据、当前架构文档的顺序判断。

## 标准闭环

```sh
mss context --format json
mss doctor --strict
mss spec validate .mss/features/<feature>.yaml
mss module generate .mss/modules/<module>.yaml
mss module generate .mss/modules/<module>.yaml --write
mss verify --changed
```

先读取和规划，再写入；所有生成默认 dry-run。写入后重新生成并确认第二次无差异。

## 阅读入口

- [仓库内 Agent 起步](/agent/getting-started)
- [规格](/agent/specifications)
- [Blueprint 与升级](/agent/blueprints-and-upgrades)
- [Skills 与 MCP](/agent/skills-and-mcp)
- [验证与评测](/agent/verification-and-evals)
- [Agent 架构摘要](/agent/architecture)

涉及迁移、权限、工作流或新模块时，使用对应 Skill，且后端授权和升级兼容性不能由
前端或生成器替代。
