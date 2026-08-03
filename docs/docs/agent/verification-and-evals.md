---
title: 验证与 Agent Evals
order: 6
nav:
  title: Agent 开发
  order: 2
description: 用确定性验证和 Agent 场景评测定义完成，而不是依赖模型自我判断
keywords: [verify eval agent test evidence ci]
---

# 验证与 Agent Evals

项目区分两类检查：

```text
确定性验证：代码和配置是否正确
Agent Eval：基础设施是否仍然容易被 Agent 正确使用
```

Agent Eval 不能替代编译、单测、迁移和安全检查；它验证的是 Agent-facing contract。

## Change-aware verifier

### 计划模式

```shell
go run ./cmd/mss verify --changed --plan --format json
```

输出包括：

- Git Diff；
- 受影响领域；
- 选择的检查；
- 每项命令、工作目录和原因；
- 是否会执行；
- 报告路径。

### 执行变更检查

```shell
go run ./cmd/mss verify --changed
```

### 模块检查

```shell
go run ./cmd/mss verify --module supplier
```

### 全量检查

```shell
go run ./cmd/mss verify --all
```

## 检查层级

| 层级 | 内容 |
| --- | --- |
| L0 | 格式、生成物漂移、YAML/JSON 语法 |
| L1 | Go lint、TypeScript、ESLint、静态契约 |
| L2 | 单元测试 |
| L3 | API、Feature、Module、MCP 契约测试 |
| L4 | 数据库、Redis、服务集成测试 |
| L5 | Playwright E2E 和权限矩阵 |
| L6 | CodeQL、govulncheck、依赖和 Secret 扫描 |
| L7 | 文档、SDK、路由、权限和生成物一致性 |

`--changed` 必须选择“最小但充分”的检查，而不是为了速度跳过高风险领域。

## 验证报告

默认输出：

```text
.mss/reports/verify.json
.mss/reports/verify.md
```

报告应包含：

- 模式和 Diff；
- 计划检查；
- 实际状态；
- exit code；
- duration；
- stdout/stderr 摘要；
- 跳过原因；
- 总体成功状态。

本地报告默认被 Git 忽略；PR 交接应摘录关键证据，CI 可上传 Artifact。

## Agent Evals

目录：

```text
.mss/evals/catalog.yaml
```

命令：

```shell
go run ./cmd/mss eval list
go run ./cmd/mss eval run --all
go run ./cmd/mss eval report
```

报告：

```text
.mss/reports/evals/latest.json
.mss/reports/evals/latest.md
```

## 当前 Eval 场景

### Project onboarding

验证一个没有历史会话的 Agent 能发现：

- Project contract；
- capability；
- command；
- Skills；
- 必需工具链。

### Supplier module contract

验证：

- AdminModule 语义；
- 字段和权限；
- 确定性生成范围；
- dry-run；
- 输出路径安全。

### Supplier feature contract

验证：

- Feature 目标与非目标；
- Actor、Module、Requirement 引用；
- Security constraint；
- 每个 must Requirement 的 Acceptance；
- Risk、rollout、rollback。

### Downstream application Blueprint

验证：

- Git-tracked 文件选择；
- 文件数量；
- identity rewrite；
- Foundation commit；
- dry-run；
- conflict-free plan。

### MCP project tools

验证 MCP 工具：

- 名称稳定；
- 顺序确定；
- Input Schema 完整；
- context/spec/generation/validation/Blueprint/upgrade 工具可发现。

### Full verification plan

验证全量计划仍覆盖后端、框架、前端、文档和 Agent 工具。

## 新增 Eval 的条件

以下变更默认需要 Eval：

- `AGENTS.md` 或 `.mss/` 事实源变更；
- 新增或修改 Skill；
- 新增 MCP 工具；
- 模块 Schema 或生成器变化；
- Blueprint 或升级算法变化；
- Setup/Dev/Verify 命令行为变化；
- 适配新的 Agent 工具。

## Eval 设计原则

- 场景对应真实用户任务；
- 结果可重复；
- 不依赖生产凭据；
- 默认不调用收费模型；
- 失败信息可定位；
- 不能仅检查字符串存在；
- 必须覆盖安全和越权边界；
- 与确定性单测配合。

后续可以增加真实模型 Eval，统计：

```text
任务完成率
构建通过率
测试通过率
越权缺陷率
无关变更率
人工修复次数
Token 消耗
完成时间
```

但这些指标不能降低确定性验证门槛。
