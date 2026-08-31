---
title: v1.3.7 工具与 MCP
order: 3
description: v1.3.7 正式 mss、mss-mcp 安装来源、版本验证和客户端配置
keywords: [v1.3.7 stable mss mss-mcp MCP stdio]
---

# v1.3.7 工具与 MCP

v1.3.7 Root Release 同时发布 `mss` 与 `mss-mcp`。两个二进制内置同源
`management-system` Blueprint，并在 `--version` 中报告 v1.3.7、完整提交和构建时间。
安装器、归档与校验和必须来自同一个 Root Release；Docs 网站可独立候补，不参与工具
完整性判断。

v1.3.5 与 v1.3.6 已永久停止为不可变部分发布；它们的工具或缺失发布面不能与 v1.3.7
混用，也不能用源码编译结果补齐。

## 安装与验证

按照[快速开始](/getting-started)下载 `install-mss.sh` 或 `install-mss.ps1`。安装器先
验证 `SHA256SUMS.tools-v1.3.7`，再同时替换两个命令。完成后运行：

```shell
mss --version
mss-mcp --version
```

版本、源提交或构建时间不一致时停止使用；不要从 Foundation checkout 临时编译一个
二进制与正式另一个二进制混用。

## `mss` 的人类入口

`mss` 将创建、诊断、依赖安装、开发编排、规格生成、验证和三方升级收敛为确定性命令。
常用路径是：

```shell
mss new app example-admin --module github.com/example/example-admin --destination ./example-admin
mss new app example-admin --module github.com/example/example-admin --destination ./example-admin --write --git-init
cd example-admin
mss doctor --strict
mss setup
mss dev --detach
mss verify --all
```

计划默认只读；会写文件的操作需要显式标志。目标路径受限，未知文件和冲突应失败关闭。

## MCP 协议合同

`mss-mcp` 是使用标准输入和标准输出通信的 **stdio** 长驻服务器。MCP 客户端先调用
`tools/list` 核对能力清单。空目录最多允许 `mss_plan_application` 返回新应用的只读
计划；规格、生成、验证与升级工具需要工作根中存在有效 `.mss/project.yaml`。

下面是客户端配置结构示例。把路径换成目标 Thin Host 的绝对路径；创建新应用时可改为
一个专用空目录。不要把 Foundation 源码位置或密钥放入配置。

```json
{
  "mcpServers": {
    "mss": {
      "command": "mss-mcp",
      "args": ["--root", "/absolute/path/to/example-admin"]
    }
  }
}
```

启动后用客户端的 MCP inspector 或协议日志确认 initialize 和 `tools/list` 成功。协议
消息只走 stdout；诊断写 stderr。MCP 写工具继续遵守 dry-run、显式确认、路径限制、未知
文件拒绝、参数校验和敏感信息脱敏，不因 Agent 调用而获得额外权限。

## 权威与排错

在 Thin Host 内，Agent 先读该仓库自己的 `AGENTS.md`、`.mss/` 和本地 Skills，再选择
工具；公共网页只是给人类解释这些合同。若客户端能启动但项目工具不可用，依次检查：

1. `mss-mcp --version` 是否精确为 v1.3.7；
2. `--root` 是否指向预期绝对目录；
3. 目录是否包含有效 `.mss/project.yaml`；
4. `tools/list` 返回的名称与当前工具一致；
5. stderr 是否给出路径、规格或冲突错误。

不要用长期 token、生产配置或本地 `replace` 让检查通过。Foundation 维护工作和 Thin Host
业务工作具有不同的 Skills 集合，详见[Skills 与 MCP](/agent/skills-and-mcp)。
