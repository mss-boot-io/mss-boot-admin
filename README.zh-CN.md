# mss-boot 完整 Admin 发行版

[English](./README.md)

mss-boot 是面向 Agent 的管理系统基础设施。统一的 **v1.3.5** 发布候选计划通过
已发布工具和包使用；下游应用不需要克隆或复制本仓库。

## v1.3.5 计划交付内容

| 入口 | 发布身份 | 用途 |
| --- | --- | --- |
| Agent 工具 | 根 `v1.3.5` GitHub Release 中的 `mss`、`mss-mcp` | 创建、检查、开发、验证和升级 Thin Host |
| Framework | `github.com/mss-boot-io/mss-boot-admin/mss-boot@v1.3.5` | 领域无关的 Go 基础设施 |
| Admin | `github.com/mss-boot-io/mss-boot-admin/admin@v1.3.5` | 可导入的完整 Admin 后端 |
| Admin Web | `@mss-boot-io/admin-web@1.3.5` | 完整 React 19 与 Ant Design 6 前端 |

所有组件都从已经合入 `main` 的同一个精确提交完成资格验证。只有公开发布和包对账
全部完成后，该版本才可供下游使用。

以下命令只在 v1.3.5 GitHub Release、Go Module、npm 包、镜像与 Docs 全部对账到
同一提交后成为受支持的采用者路径；在此之前，本源码只描述候选合同。

## 快速开始

公开对账完成后，Linux 或 macOS：

```sh
curl -fsSLO https://github.com/mss-boot-io/mss-boot-admin/releases/download/v1.3.5/install-mss.sh
bash ./install-mss.sh --version v1.3.5 --install-dir "$HOME/.local/bin"
export PATH="$HOME/.local/bin:$PATH"
mss --version
mss-mcp --version
```

Windows PowerShell：

```powershell
Invoke-WebRequest https://github.com/mss-boot-io/mss-boot-admin/releases/download/v1.3.5/install-mss.ps1 -OutFile install-mss.ps1
& .\install-mss.ps1 -Version v1.3.5 -InstallDir "$HOME\.local\bin"
$env:Path = "$HOME\.local\bin;$env:Path"
mss --version
mss-mcp --version
```

在空目录中创建完整 Thin Host：

```sh
mss new app orders-admin --module github.com/acme/orders-admin --destination ./orders-admin --write --git-init
cd orders-admin
mss doctor --strict
mss setup
mss dev --detach
mss dev status
mss verify --changed
```

首次在交互终端执行 `mss setup` 时，工具会用隐藏输入安全询问初始管理员密码；密码
必须为 8-128 个字符，并同时包含字母和数字。非交互自动化只在 setup 进程中从密钥
存储注入一次性 `MSS_ADMIN_INITIAL_PASSWORD`。首次迁移成功后，重复执行不再需要它。

打开 `http://127.0.0.1:8001`，使用初始用户名 `admin` 和本次 setup 提供的密码登录；
系统没有默认密码。

安装器会校验 `SHA256SUMS.tools-v1.3.5`，不需要 `sudo`，也不会修改 shell
profile。环境要求、Windows PATH、升级和故障排查见
[package-first 快速开始](https://docs.mss-boot-io.top/getting-started)。

## 架构边界

生成的应用是 **Thin Host**：精确固定 Admin Go Module 与 Admin Web 包，只持有组合
胶水和业务模块，不复制 Foundation 核心源码。后端业务模块在编译期显式注册，前端
业务路由扩展已发布的应用壳；后端授权始终是最终权威。

先安装目标版本工具、备份应用和数据库，并确认 `.mss/blueprint-manifest.json` 存在，
再用 `mss upgrade admin v1.3.5` 查看升级计划。无冲突后才加 `--apply --yes`，随后运行
`mss doctor --strict`、`mss verify --all`，并确认第二次计划为空。手工拼装或丢失
manifest 的仓库必须把业务所有文件迁入新生成的基线，不能伪造升级状态。全程不需要
Foundation 源码目录。

## 文档

- [快速开始](https://docs.mss-boot-io.top/getting-started)
- [包与导入边界](https://docs.mss-boot-io.top/getting-started/packages)
- [工具说明](https://docs.mss-boot-io.top/getting-started/tooling)
- [mss-shop 范本](https://docs.mss-boot-io.top/getting-started/mss-shop)
- [v1.3.5 发布合同](https://docs.mss-boot-io.top/releases/v1-3-5)

Foundation 贡献者请阅读 [`CONTRIBUTING.md`](./docs/CONTRIBUTING.md)。源码检出命令
与下游入门路径明确隔离。

## 许可证与安全

项目使用 [MIT License](./LICENSE)。安全问题请按
[`SECURITY.md`](./SECURITY.md) 的私密流程报告，不要提交公开 Issue。
