---
title: Agent 协作起步
order: 2
description: 人类维护者如何在 v1.3.7 Foundation 与 Thin Host 中为 AI Agent 选择正确权威入口
---

# Agent 协作起步

本页是**给人类看的协作说明**，帮助维护者把任务交给 AI Agent；它不是可执行 Agent
指令，也不替代仓库内合同。v1.3.7 是当前协调稳定版，v1.3.5 与 v1.3.6 已永久停止。
Docs 网站可通过独立 `docs/v*` 标签异步候补，其部署状态不阻断组件发布或 Thin Host
采用。

## 先选择上下文

| 要完成的工作 | Agent 的权威入口 | 人类应检查什么 |
| --- | --- | --- |
| 修改 mss-boot-admin Foundation | 根与目标目录最近的 `AGENTS.md`，再读 `.mss/` 和适用的 `.agents/skills/` | 工作树、PR 边界、模块所有权、受影响验证 |
| 修改 v1.3.7 生成的 Thin Host | 生成仓库自己的 `AGENTS.md`、`.mss/` 和本地 `.agents/skills/` | 是否固定 v1.3.7、业务文件是否保持自有 |
| 创建新的 Thin Host | 人类先按[快速开始](/getting-started)安装正式 v1.3.7 工具 | 是否从 Root Release 二进制和空目录开始 |
| 了解公开能力 | 本站产品与架构页面 | 不把网页当成 Agent 的写入授权 |

两种 Agent 上下文不能混用。Foundation checkout 中的源码命令用于贡献者验证，不是
正式工具或公共包证据；Thin Host 也不得借用 Foundation 源码、本地 `replace` 或维护者
专用 Skills 绕开冻结依赖。

## 1. 建立可验证上下文

让 Agent 先报告：

- 精确仓库、分支、提交和工作树状态；
- 最近的 `AGENTS.md` 以及 `.mss/project.yaml`、`.mss/capabilities.yaml`、
  `.mss/commands.yaml`；
- 目标文件属于 Blueprint 管理、生成管理、业务所有还是未知；
- 当前工具版本和准备执行的最小验证。

人类维护者据此判断 Agent 是否在正确仓库、正确版本和授权范围内工作。不要把旧 prompt、
聊天记录或 Docs 页面当成高于编译代码与机器合同的新需求。

## 2. 先查能力，再写规格

先检查已有 capability、Feature 与 AdminModule。中大型变化更新 Feature；垂直 CRUD
更新 AdminModule；关系、工作流、行级权限等复杂行为先形成显式设计。规格验证通过后再
实现，避免创建平行框架或把业务能力放入通用 `mss-boot/`。

## 3. 计划并生成

确定性生成先输出只读计划，检查目标路径、所有权、冲突和未知文件，再显式写入。写入后
执行生成漂移检查；生成区通过规格和模板更新，不手改生成片段。Thin Host 的业务代码、
业务页面和未知文件在升级时必须保留。

## 4. 按风险验证

让 Agent 先跑最小相关检查，再按影响扩大：迁移覆盖空库和升级路径，权限覆盖正反例，
前端覆盖 loading、empty、error、denied 与 locale，高风险交互增加 Codex 内置浏览器
验收。Foundation 中使用 `.mss/commands.yaml` 声明的仓库验证入口；生成 Thin Host 使用
正式工具的 `mss verify --all`。报告必须区分已运行、失败、跳过及具体原因。

## 5. 可审查交付

一个可审查交付应说明目标、重要文件、所有权、实际命令与结果、迁移、安全、兼容性、
跳过项和下一步。Foundation 发行变更全部通过 PR 合入 `main`；Tag、包、镜像与 Release
只由对应发布流程创建。Docs 网站发布属于独立后续工作，不能倒过来改变组件发布结论。

## 6. 人类最终复核

在接受 Agent 结果前，至少确认：

1. Diff 与原始目标一致，没有越权修改锁、Secret、发布状态或业务外文件；
2. 机器合同、生成结果、测试和人类说明互相一致；
3. 变更保留后端授权、迁移和恢复边界；
4. 声称通过的检查有精确命令、提交和可复现结果；
5. 若涉及 v1.3.7 采用，所有依赖来自正式公共身份，而不是本地替代。

具体升级所有权见[Blueprint 与升级](/agent/blueprints-and-upgrades)，CLI/MCP 与 Skill
分层见[Skills 与 MCP](/agent/skills-and-mcp)。
