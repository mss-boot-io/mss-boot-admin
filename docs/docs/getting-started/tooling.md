---
title: v1.3.6 候选工具发布状态
order: 3
description: v1.3.6 未发布 mss/mss-mcp 候选与 v1.3.5 永久停止工具边界
keywords: [v1.3.6 v1.3.5 v1.3.2 candidate mss mss-mcp immutable partial]
---

# v1.3.6 候选工具发布状态

:::warning
发布状态：v1.3.2 仍是当前稳定版；v1.3.5 已永久停止且 Root 工具从未发布；v1.3.6 已选
为 release candidate，但 `mss`、`mss-mcp`、安装器和校验和尚未公开。公共制品对账前，
v1.3.6 不可采用，本页也不提供安装、创建、开发、验证或升级命令。
:::

## 实际结果

原候选合同计划在 Root Release 中提供 Linux、macOS 与 Windows 的 amd64/arm64 工具归档、
安装器、校验和和构建来源证明。这些资产均未发布。源码中的命令入口、兼容性编译结果或
旧版本二进制都不能代替对应版本的 Release 工具包。

因此，以下能力在 v1.3.5 上只能视为未发布的产品合同，不能作为采用者入口：

| 工具 | 未来完整发行中的职责 | v1.3.5 状态 |
| --- | --- | --- |
| `mss` | 创建、诊断、依赖安装、开发编排、生成、验证与三方升级 | Root 资产缺失 |
| `mss-mcp` | 向 MCP 客户端暴露相同的确定性、路径受限和默认 dry-run 能力 | Root 资产缺失 |

## 未来工具的完整性边界

未来完整版本的工具必须来自该版本 Root Release，且归档摘要、版本、源提交和构建证明
完全一致。安装过程需先校验归档再替换目标文件，不依赖管理员权限，不修改用户 shell
配置，也不能把内部发布辅助程序暴露为公共工具。

应用创建与 Distribution 升级还必须绑定同一 Release 中的 Blueprint 来源；单独从源码
编译二进制不能证明这条来源链，必须失败关闭。

## MCP 合同（非 v1.3.5 使用指引）

未来的 `mss-mcp` 是 stdio 服务器，并继承 CLI 的路径限制、敏感信息脱敏和默认 dry-run
规则。客户端通过 MCP 协议的 `tools/list` 发现能力；空目录最多允许
`mss_plan_application` 返回只读计划。需要项目上下文的能力必须以包含有效
`.mss/project.yaml` 的 Thin Host 为根，缺失时直接失败，不能猜测 Foundation 源码位置。

这些描述只定义产品安全边界，不表示 v1.3.5 已有可配置的 MCP 命令。完整发布证据见
[v1.3.5 不可变部分发布记录](/releases/v1-3-5)，当前稳定入口见
[v1.3.2 稳定记录](/releases/archive/v1-3-2)。
