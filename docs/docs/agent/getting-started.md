---
title: Agent 开发合同状态
order: 2
description: Foundation 源码与 v1.3.6 候选 Thin Host 的检查、规划和交付顺序
---

# Agent 开发合同状态

:::warning
发布状态：v1.3.2 仍是当前稳定版；v1.3.5 已永久停止并保持不可变部分发布；v1.3.6 已选
为 release candidate，但尚未发布。本页保留 source-only 和候选 Agent 工作流合同；
公共制品对账前，v1.3.6 不可采用，也不是 Thin Host 操作指南。
:::

## 1. 建立可验证上下文

Agent 先读取根和目标目录最近的 `AGENTS.md`，再读取 `.mss/project.yaml`、
`.mss/capabilities.yaml` 与 `.mss/commands.yaml`，并记录工作树状态。Foundation
源码使用仓库声明的命令；v1.3.6 候选 Thin Host 仅在发布后使用其 Release 工具。两种上下文不能混用。

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
源码 `verify` 合同仍有效，但必须明确是贡献者工作区验证，不是 v1.3.5 采用证据。

## 5. 可审查交付

报告改动目标、文件所有权、实际命令与结果、跳过项、迁移、安全和兼容性影响。所有发行
变更先通过 PR 合入 main；源码检查成功不能授权公共 Tag、包、镜像或 Docs。

未来完整版本完成公共工具、Go/npm、镜像、Docs 与外部使用方对账后，才能把同一工作流
作为 Thin Host 采用者路径。
