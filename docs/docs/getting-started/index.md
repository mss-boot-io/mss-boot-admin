---
title: v1.3.3 快速开始
order: 1
nav:
  title: 快速开始
  order: 1
description: 不克隆 Foundation 源码，使用 v1.3.3 工具和公开包创建可开发、可验证、可升级的 Thin Host
keywords: [v1.3.3 package first mss thin host quick start]
---

# v1.3.3 快速开始

这是一条唯一受支持的新项目入门路径。它从公开 Release 安装工具，在空目录生成
Thin Host，并通过公开 Go/npm 包组合完整 Admin。

## 1. 准备环境

| 依赖 | 版本 |
| --- | --- |
| Go | 1.26.6 |
| Node.js | 24.x |
| pnpm | 10.34.5（通过 Corepack） |
| Bash | 3.2+（仅 Linux/macOS 安装器） |
| Git | 支持初始化和提交的当前稳定版 |

本地默认可使用 SQLite；Redis、MySQL、PostgreSQL 等外部资源仅在业务明确启用时需要。
创建、安装和验证过程不需要生产凭据。

## 2. 安装 mss 工具

Linux 或 macOS：

```sh
curl -fsSLO https://github.com/mss-boot-io/mss-boot-admin/releases/download/v1.3.3/install-mss.sh
bash ./install-mss.sh --version v1.3.3 --install-dir "$HOME/.local/bin"
export PATH="$HOME/.local/bin:$PATH"
mss --version
mss-mcp --version
```

Windows PowerShell：

```powershell
Invoke-WebRequest https://github.com/mss-boot-io/mss-boot-admin/releases/download/v1.3.3/install-mss.ps1 -OutFile install-mss.ps1
& .\install-mss.ps1 -Version v1.3.3 -InstallDir "$HOME\.local\bin"
$env:Path = "$HOME\.local\bin;$env:Path"
mss --version
mss-mcp --version
```

脚本会选择当前操作系统和架构的工具包，并在替换二进制前校验
`SHA256SUMS.tools-v1.3.3`。它不会请求 `sudo`，也不会修改 profile。正式支持的公开
工具只有 `mss` 与 `mss-mcp`。

## 3. 从空目录创建应用

```sh
mss new app orders-admin --module github.com/acme/orders-admin --destination ./orders-admin --write --git-init
cd orders-admin
```

`mss new app` 默认先产生只读计划；`--write` 才写文件，`--git-init` 在成功后
初始化 Git。生成结果精确固定：

- `github.com/mss-boot-io/mss-boot-admin/admin@v1.3.3`；
- `github.com/mss-boot-io/mss-boot-admin/mss-boot@v1.3.3`；
- `@mss-boot-io/admin-web@1.3.3`；
- 与工具二进制同源的 Blueprint 身份和快照。

## 4. 检查、安装并启动

```sh
mss doctor --strict
mss setup
mss dev --detach
mss dev status
```

`doctor --strict` 会把缺失工具、版本不匹配、锁文件漂移和不完整 Thin Host 作为失败；
`setup` 使用生成仓库的布局与冻结依赖，并幂等初始化本地 SQLite。首次执行需要通过
内置隐藏提示提供 8-128 个字符、同时包含字母和数字的初始管理员密码。非交互自动化
只在 setup 进程中从密钥存储注入 `MSS_ADMIN_INITIAL_PASSWORD`；setup 会从依赖安装
子进程中显式移除它，只在迁移命令中临时注入，且不写入参数、报告或生成文件。首次迁移
成功后，重复执行不再需要它。
首次冷启动还会下载完整 Admin 后端与前端依赖，网络和本机缓存为空时可能持续数分钟；
命令暂时安静不等于失败，请通过同一终端输出或 `mss dev logs` 判断进度，不要因短暂无输出
中断安装。依赖进入本机缓存后，后续 setup、verify 和启动通常会明显更快。
`dev` 统一管理后端和前端进程。

### 首次登录

打开 `http://127.0.0.1:8001`，用户名为 `admin`，密码是刚才通过隐藏提示或一次性
环境变量提供的值。系统没有默认密码；该环境变量不会重置已经初始化的账号。

查看日志或停止：

```sh
mss dev logs backend
mss dev logs admin-web --follow
mss dev stop
```

## 5. 验证第一条变更

```sh
mss verify --changed
```

提交前可运行完整检查：

```sh
mss verify --all
```

验证结果会写入 `.mss/reports/`，可供人和 Agent 检查；不要把“进程存在”当作业务成功
证据。

## 6. 升级 Admin Distribution

升级前先备份业务仓库和数据库，并安装目标版本的工具。两个二进制都必须报告目标版本：

```sh
mss --version
mss-mcp --version
mss upgrade status --format json
```

三方升级只支持由 Blueprint 生成、且保留 `.mss/blueprint-manifest.json` 的 Thin Host。
手工拼装或丢失 manifest 的仓库不能直接套用升级；先用 v1.3.3 在新目录生成基线，再按
业务所有权迁入规格和业务文件并重新验证，不要伪造 manifest。

匹配 v1.3.3 工具的只读计划不需要 Foundation 源码：

```sh
mss upgrade admin v1.3.3
```

确认计划无冲突后再应用：

```sh
mss upgrade admin v1.3.3 --apply --yes
mss doctor --strict
mss verify --all
mss upgrade admin v1.3.3
```

升级只管理 Blueprint 声明的 Thin Host 文件，保留业务所有和未知文件，并在所有写入成功
后最后更新快照。最后一次计划必须为空；若仍有变化或冲突，升级没有完成。

## 下一步

- [理解 Go 与 npm 包边界](/getting-started/packages)
- [了解工具资产与完整性校验](/getting-started/tooling)
- [参考 mss-shop 的单租户业务组织](/getting-started/mss-shop)
- [配置与运行 Admin](/admin)

如果 Release 资产或公共包尚未可用，请查看
[v1.3.3 发布合同](/releases/v1-3-3)；不要回退到源码克隆路径伪装成使用方验证。
