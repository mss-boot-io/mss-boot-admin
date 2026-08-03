---
title: 架构与演进边界
order: 7
nav:
  title: Agent 开发
  order: 2
description: Agent 原生管理系统基础设施的分层、事实源、信任边界和长期演进规则
keywords: [agent architecture contracts generator cli mcp blueprint]
---

# 架构与演进边界

## 完成形态

项目的目标不是“在后台里增加一个 AI 聊天框”，也不是“让大模型自由修改所有文件”。完成形态是：

> 一套面向 Codex、Claude Code、other coding agents、Cursor 等编码 Agent 的管理系统工程基础设施，提供稳定运行时、结构化项目事实、确定性生成、可重复开发环境、自动验证、Agent Skills、MCP 工具、应用 Blueprint 和持续升级能力。

## 分层

```text
┌────────────────────────────────────────────┐
│ Codex / Claude Code / other coding agents / Cursor     │
└──────────────────────┬─────────────────────┘
                       │
┌──────────────────────▼─────────────────────┐
│ Thin adapters: AGENTS / CLAUDE / Cursor    │
│ Skills / MCP                              │
└──────────────────────┬─────────────────────┘
                       │
┌──────────────────────▼─────────────────────┐
│ mss CLI                                  │
│ context / doctor / setup / dev           │
│ spec / module / verify / eval            │
│ new app / upgrade                        │
└──────────────────────┬─────────────────────┘
                       │
┌──────────────────────▼─────────────────────┐
│ Deterministic engines                     │
│ project / spec / generator / verifier     │
│ blueprint / upgrade / eval / process mgr  │
└──────────────────────┬─────────────────────┘
                       │
┌──────────────────────▼─────────────────────┐
│ Management-system runtime                 │
│ mss-boot / backend / web/antd / docs      │
└────────────────────────────────────────────┘
```

## 事实源优先级

1. 编译、测试和运行事实；
2. `.mss/` 机器契约；
3. 根和路径附近的 `AGENTS.md`；
4. ADR 和长期文档；
5. Skills；
6. Agent-specific 薄适配文件；
7. 历史 Prompt 和会话记录。

历史 Prompt 不是当前事实源。

## CLI 是唯一确定性实现

以下逻辑只能存在一份：

- 规格语义验证；
- 模块生成；
- 项目上下文；
- 环境诊断；
- 开发服务管理；
- 变更感知验证；
- 应用生成；
- Foundation 升级；
- Eval。

Skills 只编排，MCP 只包装，Agent-specific 配置只导航。

## 目录边界

```text
AGENTS.md
CLAUDE.md
.cursor/
.codex/
.agents/skills/
.mss/
cmd/mss/
cmd/mss-mcp/
internal/mss/
modules/
templates/
mss-boot/
web/antd/
docs/
```

### `.mss/`

稳定机器契约和版本化输入：

```text
project.yaml
capabilities.yaml
commands.yaml
dev.yaml
lock.yaml
schemas/
features/
modules/
evals/
blueprints/
```

运行态内容放入被忽略目录：

```text
.mss/run/
.mss/logs/
.mss/cache/
.mss/reports/
.mss/output/
```

### `internal/mss/`

Agent 基础设施实现。禁止业务模块反向依赖 CLI/MCP 层。

### `modules/`

新业务能力使用垂直切片。历史横向目录保留兼容，但不再作为生成器目标。

### `mss-boot/`

只承载通用 Go runtime，不放供应商、合同等具体业务，也不放 Agent tool-specific 协议适配。

## 信任边界

### 读取

- 默认仅仓库内；
- Blueprint 只读 Git-tracked 文件；
- 规格路径经过相对路径和 symlink 检查；
- MCP 不读取生产 Secret；
- 历史 Prompt 默认不进入上下文。

### 写入

- 生成默认 dry-run；
- 路径限制；
- 冲突时拒绝；
- 原子文件写入；
- 不自动 Git commit；
- destructive operation 明确确认；
- Foundation Manifest 最后提交。

### 执行

- command catalog 使用固定 argv/工作目录；
- `mss dev` 不拼接任意 Shell；
- `verify` 命令来自受控目录；
- MCP 执行工具需要批准；
- 不执行来自 FeatureSpec 的任意命令作为隐式 Hook。

FeatureSpec 中的 command evidence 是验收说明，不是自动信任的远程代码执行入口。

## Legacy 边界

以下能力保留兼容但不是新主线：

- runtime dynamic model；
- virtual CRUD；
- runtime code generation；
- 各导入子目录中的旧 `.github/workflows`；
- 一次性 Prompt 作为流程控制方式。

替代路线：

```text
runtime dynamic generation
→ development-time Feature/AdminModule specs
→ deterministic generated vertical modules
→ compiled and tested code
```

## 兼容性规则

以下变化视为基础设施 API 变更：

- `.mss` apiVersion 或字段；
- CLI command/flag/exit code；
- MCP tool name/schema/result；
- Skill name/trigger/required command；
- generated file layout；
- Blueprint selection/transformation；
- Manifest hash semantics；
- Verify check ID；
- Eval case ID。

发布前必须提供：

- 兼容策略；
- Upgrade Recipe 或 Codemod；
- 回归测试；
- 文档；
- 下游影响说明。

## Stable 完成标准

Agent-native foundation 从 Beta 进入 Stable 至少需要：

1. `mss doctor/setup/dev` 在 Linux、macOS 和 Windows 或明确支持矩阵中验证；
2. Feature 和 AdminModule contracts 版本冻结；
3. 模块生成完整全栈闭环并通过幂等/漂移测试；
4. MCP 工具 Schema 和审批策略稳定；
5. 下游 Blueprint 能生成并独立通过基础验证；
6. 两个 Foundation 版本间三方升级通过 create/update/delete/preserve/conflict 测试；
7. `verify --changed` 风险选择经过真实 PR 校验；
8. Evals 在 CI 持续运行；
9. 安全扫描和 Secret scanning 开启；
10. 文档和示例由新的零上下文 Agent 成功复现。

## 不在第一稳定版内

- 让 Agent 自动合并 PR；
- 在没有审批的情况下修改生产环境；
- 自动执行生产数据库迁移；
- 把所有历史模块一次性重写；
- 绑定唯一模型供应商；
- 使用自然语言替代结构化权限和迁移契约。
