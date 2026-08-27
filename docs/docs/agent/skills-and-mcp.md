---
title: Skills 与 MCP 合同状态
order: 5
description: Foundation 源码 Skill 清单，以及未来 mss-mcp 的安全边界
---

# Skills 与 MCP 合同状态

:::warning
v1.3.5 是不可变部分发布，Root Release 与 `mss`、`mss-mcp` 工具未发布；当前稳定
版本仍是 v1.3.2。本页记录 Foundation 源码中的 Skill 和未来完整发行合同，不是
v1.3.5 工具安装或 MCP 客户端配置指引。
:::

Skill 描述一类可复用工作流。源码实现由确定性 CLI、生成器和验证器承载，未来完整发行
的 MCP 工具复用同一合同；Skill 不应复制实现逻辑。

## Foundation 源码 Skill 清单

当前仓库源码维护以下下游 Skill 设计：

- `mss-thin-host`：项目所有权和日常生成/验证边界；
- `mss-add-module`：当前生成器支持的基础 CRUD 模块；
- `mss-add-field`：当前支持字段类型的前向演进；
- `mss-add-permission`：粗粒度后端 RBAC、API、菜单与动作权限；
- `mss-debug-fullstack`：可复现的全栈诊断；
- `mss-review-change`：安全、迁移、生成漂移与兼容性评审；
- `mss-upgrade-foundation`：协调 Admin Distribution 三方升级。

工作流生成、关系字段生成、行级权限生成、Foundation 发布和文档发布不在这一公开集合
中。仓库中存在内部流程不代表 v1.3.5 下游工具已经分发这些能力。

## 未来 MCP 合同

未来完整发行的 `mss-mcp` 是 stdio 长驻服务器。客户端通过 MCP `tools/list` 验证工具
清单；空目录最多允许 `mss_plan_application` 返回新应用只读计划，其余项目上下文、
规格、生成、验证和升级能力要求工作根存在有效 `.mss/project.yaml`。

写操作继续遵守 dry-run、路径限制、未知文件拒绝、参数校验和敏感信息脱敏；MCP 不获得
额外权限。客户端必须把目标 Thin Host 绝对路径作为显式工作根，不能猜测 Foundation
源码位置。

v1.3.5 没有可核验的 MCP 二进制，因此本页不展示启动命令或客户端 `command` 配置。
未来工具只有在同一完整 Root Release 中报告相同版本、源提交和 Blueprint 来源后，才能
成为采用者入口。

## 安全与可重复性

- 不从生产系统获取创建或验证所需数据；
- 不把 token、prompt、响应正文或 secret 写入报告；
- 工具不发送遥测、不登记采用者；
- 生成先看变更列表，升级先看三方计划；
- 对 MCP 返回的“完成”继续用仓库状态、测试和真实合同验证。
