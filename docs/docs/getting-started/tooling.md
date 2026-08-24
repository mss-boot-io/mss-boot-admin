---
title: v1.3.3 工具说明
order: 3
description: mss 与 mss-mcp 的安装资产、校验、职责和安全边界
keywords: [v1.3.3 mss mss-mcp install checksum tools]
---

# v1.3.3 工具说明

v1.3.3 对外只发布两个工具：

| 工具 | 职责 |
| --- | --- |
| `mss` | 创建、诊断、安装依赖、开发编排、规格、生成、验证与升级 |
| `mss-mcp` | 将同一组确定性只读/默认 dry-run 能力提供给 MCP 客户端 |

覆盖 Linux、macOS、Windows 的 amd64 与 arm64。Release 资产命名为
`mss-tools-v1.3.3-{linux|darwin}-{amd64|arm64}.tar.gz` 或 Windows zip。

## 安装接口

Shell：

需要 Bash 3.2 或更高版本；不要用 POSIX `sh` 解释该安装器。

```sh
curl -fsSLO https://github.com/mss-boot-io/mss-boot-admin/releases/download/v1.3.3/install-mss.sh
bash ./install-mss.sh --version v1.3.3 --install-dir "$HOME/.local/bin"
```

PowerShell：

```powershell
Invoke-WebRequest https://github.com/mss-boot-io/mss-boot-admin/releases/download/v1.3.3/install-mss.ps1 -OutFile install-mss.ps1
& .\install-mss.ps1 -Version v1.3.3 -InstallDir "$HOME\.local\bin"
```

默认版本同样固定为 v1.3.3，默认目录位于当前用户目录。显式参数更适合可复现脚本。

## 完整性与来源

安装器在替换现有文件前：

1. 识别受支持的系统和架构；
2. 下载一个精确版本工具包；
3. 读取 `SHA256SUMS.tools-v1.3.3`；
4. 校验归档摘要；
5. 安装 `mss` 与 `mss-mcp`；
6. 保留用户对 PATH 的控制。

两个二进制都应通过 `--version` 报告 v1.3.3 与 Release 源提交。摘要、版本或提交身份
不一致时应停止，而不是继续安装。

## 常用命令

```sh
mss context --format json
mss doctor --strict
mss setup
mss dev --detach
mss verify --changed
mss upgrade admin v1.3.3
mss skills list
```

应用生成、模块生成和 Distribution 升级默认先给出计划；生成使用 `--write`，升级使用
`--apply --yes`。`mss setup` 与 `mss dev` 是明确的执行型命令，会安装依赖、迁移本地
数据库或管理开发进程。
全新项目第一次 `mss setup` 的初始管理员密码环境变量和隐藏输入方式见
[快速开始](/getting-started)；工具不会接受或记录命令行密码参数。

## MCP 边界

`mss-mcp` 不绕过 `mss` 的路径限制、dry-run、参数校验或敏感信息脱敏。MCP 客户端
应以 Thin Host 根目录作为工作根，并只授予任务实际需要的能力。安装和使用过程不发送
遥测，也不登记采用者。

`mss-mcp` 是 stdio 服务器：客户端启动后，它会持续等待 JSON-RPC 请求，直到客户端
关闭 stdin 或终止进程。直接在终端运行后“没有返回提示”不是卡死；终端只适合检查
`mss-mcp --version` 和 `mss-mcp --help`。通用客户端配置形态如下：

```json
{
  "mcpServers": {
    "mss": {
      "command": "mss-mcp",
      "args": ["--root", "/absolute/path/to/thin-host"]
    }
  }
}
```

重新加载客户端后，用 MCP 协议的 `tools/list` 确认工具已出现。空目录只允许使用
`mss_plan_application` 生成新应用的只读计划；项目上下文、规格、模块、验证与升级工具
都要求 `--root` 指向包含有效 `.mss/project.yaml` 的 Thin Host，并在缺失时失败，而不是
猜测 Foundation 源码位置。

发布流程内部使用的辅助程序不是公共工具，不应出现在采用者文档、PATH 或 Release
资产清单中。
