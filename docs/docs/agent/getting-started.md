---
title: Agent 协作起步
order: 2
description: 人类维护者如何为 Foundation Agent 或 Thin Host Agent 选择正确的本地权威入口
---

# Agent 协作起步

:::warning
发布状态：v1.3.2 仍是当前稳定版；v1.3.5 与 v1.3.6 已永久停止并保持不可变部分发布；v1.3.7 已选
为 release candidate，但尚未稳定且不可采用。候选发布面可能处于不同公开阶段，必须以
远端发布台账为准；完整 stable promotion 和最终 policy/Docs 对账完成前，本页不是 Thin Host
安装、创建或升级指南。
:::

## 先选择上下文

| 要完成的工作 | Agent 从哪里开始 | 当前边界 |
| --- | --- | --- |
| 修改 mss-boot-admin Foundation | 根与目标目录最近的 `AGENTS.md` | 使用源码内 `.mss`、Skills 和仓库命令，所有发行变更通过 PR |
| 修改已经生成的 Thin Host | 该生成仓库自己的 `AGENTS.md` | 只使用它固定的 Distribution 与本地业务合同 |
| 创建新的 Thin Host | 当前采用状态页 | v1.3.7 完成 stable promotion 与最终对账前不开放 |

两种上下文不能混用。Foundation 源码验证不是已发布工具证据，未来 Thin Host 也不得借用
Foundation checkout 或本地 `replace` 掩盖公共包问题。

## 1. 建立可验证上下文

Agent 读取本地权威入口与 `.mss/project.yaml`、`.mss/capabilities.yaml`、
`.mss/commands.yaml`，并记录工作树状态。人类维护者检查 Agent 是否使用了正确仓库、
分支、版本和文件所有权。

## 2. 先查能力，再写规格

先检查已有 capability、Feature 与 AdminModule。中大型变化更新 Feature，垂直 CRUD
更新 AdminModule；规格先验证，再开始实现。不要从历史 prompt 或对话推断当前需求。

## 3. 计划并生成

确定性生成先输出只读计划，检查路径、所有权和未知文件，再显式写入。写入后执行漂移
检查；生成区只通过规格和模板变化更新。v1.3.5 没有发布下游生成工具，源码生成结果不能
冒充该版本 Thin Host。

## 4. 按风险验证

先运行最小相关检查，再按影响扩大。迁移覆盖空库和升级路径，权限覆盖正反例，前端覆盖
loading、empty、error、denied 与 locale，高风险交互增加 Codex 内置浏览器验收。一般
源码 `verify` 合同仍有效，但必须明确是贡献者工作区验证，不是 v1.3.5、v1.3.6 或
未稳定 v1.3.7 的采用证据。

## 5. 可审查交付

报告改动目标、文件所有权、实际命令与结果、跳过项、迁移、安全和兼容性影响。所有发行
变更先通过 PR 合入 main；源码检查成功不能授权公共 Tag、包、镜像或 Docs。

未来完整版本完成公共工具、Go/npm、镜像、Docs 与外部使用方对账后，才能把同一工作流
作为 Thin Host 采用者路径。
