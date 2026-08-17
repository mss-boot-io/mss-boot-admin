---
title: 开箱即用
order: 2
nav:
  title: Agent 开发
  order: 2
description: 让编码 Agent 在没有历史会话的情况下进入、启动和验证项目
keywords: [agent onboarding setup doctor dev verify]
---

# 开箱即用

目标不是让 Agent “大致知道怎么改”，而是让一个没有历史会话的新 Agent 能够独立完成：

```text
发现仓库 → 理解边界 → 检查环境 → 安装依赖 → 启动服务 → 修改代码 → 选择验证 → 输出证据
```

## 第一次进入仓库

任何 Agent 都应先执行：

```shell
go run ./cmd/mss context --format json
go run ./cmd/mss doctor --format json
go run ./cmd/mss skills list --format json
```

然后阅读：

```text
AGENTS.md
.mss/project.yaml
.mss/capabilities.yaml
.mss/commands.yaml
距离目标文件最近的 AGENTS.md
```

不要先遍历全部历史 Prompt，也不要根据旧目录名和个人工作站绝对路径猜测启动方式。

## 环境要求

规范版本以 `.mss/project.yaml` 和 CI 为准。当前基线包括：

- Go 1.26；
- Node.js 24；
- pnpm 10；
- Git；
- 可选 Docker/Podman；
- 本地开发默认 SQLite；
- Redis 在需要缓存、会话或集群 WebSocket 验证时启用。

`doctor` 将检查：

- 工具版本；
- 仓库根目录；
- `go.work` 和模块；
- 端口占用；
- 前端锁文件；
- 可选服务；
- 是否存在危险或缺失的本地配置。

JSON 输出适合 Agent 解析，文本输出适合人工检查。

## 初始化

```shell
go run ./cmd/mss setup
```

初始化必须满足：

- 无交互；
- 可以重复执行；
- 不需要生产凭据；
- 不覆盖用户已有配置；
- 失败时返回明确步骤和非零退出码；
- 只在仓库允许的本地目录写状态。

初始化后建议重新运行：

```shell
go run ./cmd/mss doctor --format json
```

## 启动和管理开发服务

```shell
# 前台启动
go run ./cmd/mss dev

# 后台启动
go run ./cmd/mss dev --detach

# 查看所有服务状态和 HTTP readiness
go run ./cmd/mss dev status --format json

# 读取日志
go run ./cmd/mss dev logs backend
go run ./cmd/mss dev logs frontend --follow

# 按反向依赖顺序停止进程树
go run ./cmd/mss dev stop
```

无参数启动只包含后端与唯一前端 `web/antd-v6`（端口 `8001`）。

服务定义位于 `.mss/dev.yaml`，包括：

- 工作目录；
- argv 数组，而不是拼接 Shell 字符串；
- 环境变量；
- 依赖关系；
- HTTP 健康检查；
- 启动和停止超时。

PID 状态与日志存储在 `.mss/run/` 和 `.mss/logs/`，均被 Git 忽略。

## 修改前先选工作流

```shell
go run ./cmd/mss skills list
```

典型映射：

| 请求 | Skill |
| --- | --- |
| 新建业务模块 | `mss-add-module` |
| 增加字段 | `mss-add-field` |
| 增加权限 | `mss-add-permission` |
| 新建完整系统 | `mss-new-application` |
| 调试全栈问题 | `mss-debug-fullstack` |
| 基础设施升级 | `mss-upgrade-foundation` |
| PR 评审 | `mss-review-change` |
| 发布 | `mss-release` |

Skill 负责步骤编排，真正写入和验证仍由 `mss` CLI、生成器和测试完成。

## 修改后的最小验证

```shell
go run ./cmd/mss verify --changed
```

它会根据 Git Diff 判断是否需要执行：

- Go 格式化与测试；
- 框架测试；
- 前端 lint、类型、Jest 和构建；
- 文档构建；
- 规格校验；
- 生成物漂移检查；
- 安全和依赖检查；
- E2E 或迁移相关检查。

需要审查计划而不执行时：

```shell
go run ./cmd/mss verify --changed --plan --format json
```

发布或重大变更前：

```shell
go run ./cmd/mss verify --all
go run ./cmd/mss eval run --all
```

## Agent 的完成报告

最终交接至少包含：

```text
变更目标
实际修改路径
数据库/权限/API/前端影响
执行的验证命令
通过、失败、跳过项
生成和 Eval 报告位置
兼容性与回滚方式
未决风险
```

不允许仅写“已完成”“测试应该能过”或省略失败信息。
