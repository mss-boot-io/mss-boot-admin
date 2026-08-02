---
title: Skills 与 MCP
order: 4
nav:
  title: Agent 开发
  order: 2
description: Agent Skills 如何编排工作流，MCP 如何暴露同一套项目工具
keywords: [agent skills mcp codex tools]
---

# Skills 与 MCP

Skills 与 MCP 解决不同问题：

```text
Skill：什么时候做、按什么顺序做、怎样交接
MCP：Agent 如何调用结构化项目工具
CLI：所有确定性行为的唯一实现
```

正确依赖关系：

```text
Codex / Claude Code / 其他 Agent
            │
       Skills / MCP
            │
          mss CLI
            │
规格 / 生成器 / 验证器 / Blueprint / Upgrade
            │
        Repository
```

不允许 Skill、MCP 和 CLI 分别实现三套生成或验证逻辑。

## Repository Skills

仓库级 Skills 位于：

```text
.agents/skills/
```

检查 Skills：

```shell
go run ./cmd/mss skills list --format json
go run ./cmd/mss skills validate --format json
```

验证内容包括：

- Frontmatter 和名称；
- 目录名与 Skill 名一致；
- 描述是否足够用于触发；
- 是否存在绝对工作站路径；
- 是否引用不存在的命令；
- 是否重复实现 CLI 逻辑；
- 是否包含生产凭据或危险示例。

### 当前主要 Skills

| Skill | 作用 |
| --- | --- |
| `mss-project-onboarding` | 无历史上下文进入项目 |
| `mss-new-application` | 从 Blueprint 创建独立系统 |
| `mss-add-module` | 从 AdminModule 增加完整业务模块 |
| `mss-add-field` | 修改模块字段与迁移 |
| `mss-add-permission` | 增加后端强制权限 |
| `mss-add-workflow` | 增加状态和流程规则 |
| `mss-debug-fullstack` | 后端、前端、数据库和网络联调 |
| `mss-review-change` | 基于事实和验证证据评审 |
| `mss-upgrade-foundation` | 三方升级下游系统 |
| `mss-release` | 发布和回滚准备 |

Skill 应引用 `.mss/` 和 CLI，不应复制全部项目知识。

## 项目 MCP Server

入口：

```text
cmd/mss-mcp
internal/mss/mcp
```

Codex 项目配置：

```text
.codex/config.toml
```

手工测试 stdio MCP：

```shell
printf '%s\n' \
  '{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{"_meta":{"io.modelcontextprotocol/protocolVersion":"2026-07-28"}}}' \
  | go run ./cmd/mss-mcp --root .
```

服务同时兼容旧客户端的 `initialize` 生命周期和当前每请求协议元数据。

## 工具边界

### 只读工具

```text
mss_get_project_context
mss_list_capabilities
mss_list_skills
mss_validate_spec
mss_validate_module_spec
mss_get_validation_plan
mss_plan_application
mss_get_blueprint_status
mss_plan_foundation_upgrade
```

### 可写或可执行工具

```text
mss_generate_module
mss_run_validation
mss_apply_foundation_upgrade
```

写工具遵循：

- dry-run 或 plan-only 为默认值；
- 参数使用 JSON Schema 约束；
- 路径必须限制在仓库或明确批准的目标范围；
- 返回结构化内容和文本摘要；
- 幂等；
- 不自动提交 Git；
- 不读取生产凭据；
- 不绕过 CLI 的安全检查；
- Codex 侧使用 `writes` 审批模式。

## Codex

Codex 会读取根 `AGENTS.md` 和路径附近的覆盖规则，并通过 `.codex/config.toml` 启动项目 MCP。

推荐开始：

```text
先调用 mss_get_project_context，检查 capability 和适用 Skill，再规划代码变更。
```

## Claude Code

`CLAUDE.md` 是薄入口，只指向：

- `AGENTS.md`；
- `.mss/`；
- `.agents/skills/`；
- `mss` CLI。

不要在 `CLAUDE.md` 再维护一套架构和命令清单。

## other coding agents 与 Cursor

适配文件：

```text
.github/other coding agents-instructions.md
.cursor/rules/mss-agent-foundation.mdc
```

它们同样只负责引导到统一事实源。

## 安全建议

- MCP stdout 只输出协议帧，日志写 stderr；
- 不把 Prompt、业务正文、Token 或附件内容写入审计日志；
- 外部路径和写操作必须显式确认；
- 对新的 MCP 写工具增加审批、单测和 Eval；
- 工具返回错误时必须设置 `isError`，不能伪装成功；
- 工具 Schema 变更视为 Agent API 兼容性变更。
