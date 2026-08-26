---
title: Skills 与 MCP
order: 5
description: 仓库 Skill 选择、mss-mcp 使用和安全边界
---

# Skills 与 MCP

Skill 描述一类可复用工作流，`mss` 提供确定性实现，`mss-mcp` 让支持 MCP 的客户端
访问同一合同。Skill 不应复制生成器或验证器逻辑。

## 检查 Skills

```sh
mss skills list --format json
mss skills validate --format json
```

生成的 v1.3.5 Thin Host 精确分发以下 Skill：

- `mss-thin-host`：项目所有权和日常生成/验证边界；
- `mss-add-module`：当前生成器支持的基础 CRUD 模块；
- `mss-add-field`：当前支持字段类型的前向演进；
- `mss-add-permission`：粗粒度后端 RBAC、API、菜单与动作权限；
- `mss-debug-fullstack`：可复现的全栈诊断；
- `mss-review-change`：安全、迁移、生成漂移与兼容性评审；
- `mss-upgrade-foundation`：协调 Admin Distribution 三方升级。

工作流生成、关系字段生成、行级权限生成、Foundation 发布和文档发布不在这一集合中；
不要因为 Foundation 仓库里存在同名内部流程就假设下游具备能力。先读取所选
`SKILL.md` 的完整说明和必要资源。

## 启动 MCP

```sh
mss-mcp --root .
```

这是 stdio 长驻服务器，不是执行后立即退出的命令；它会等待 MCP 客户端请求。通用客户
端配置把 `command` 设为 `mss-mcp`，把 `args` 设为 `--root` 和目标 Thin Host 的绝对
路径。连接后通过 MCP `tools/list` 验证工具清单。

空目录只支持 `mss_plan_application` 的新应用只读计划。其余项目上下文、规格、生成、验证
和升级工具要求工作根存在有效 `.mss/project.yaml`；缺少项目合同时会失败。写操作继续
遵守 dry-run、路径限制、未知文件拒绝、参数校验和敏感信息脱敏；MCP 不获得额外权限。

## 安全与可重复性

- 不从生产系统获取创建或验证所需数据；
- 不把 token、prompt、响应正文或 secret 写入报告；
- 工具不发送遥测、不登记采用者；
- 生成先看变更列表，升级先看三方计划；
- 对 MCP 返回的“完成”继续用仓库状态、测试和真实合同验证。

`mss` 与 `mss-mcp` 必须来自同一个 v1.3.5 工具包并报告相同源提交。
