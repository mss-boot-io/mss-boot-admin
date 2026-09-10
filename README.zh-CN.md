# mss-boot 完整 Admin 发行版

[English](./README.md)

mss-boot 是面向 Agent 的管理系统基础设施。**v1.3.7 已是当前稳定且可采用的完整
Admin Distribution。** 所有组件都从已合并到 `main` 的精确提交
`77b53d41092741eac62fa6418c0bdbf87413c7cd` 完成资格验证和发布；生成的 Thin Host
锁定这一协调版本，不拼接不同组件版本。

公开 Docs 网站独立发布，可以晚于组件列车候补。`docs/v*` Tag 只标识网站部署；Docs
缺失、失败或暂时滞后都不会阻断 Framework、Admin、前端、工具、镜像、npm 或稳定别名
发布。仓库内的人类文档和 Agent 指令仍与源码契约一起版本化。

## 稳定发行身份

| 发布面 | v1.3.7 身份 |
| --- | --- |
| Root 工具、后端镜像与 GitHub Release | `v1.3.7` |
| 可复用 Go Framework | `mss-boot/v1.3.7` |
| 可导入 Admin Go Module | `admin/v1.3.7` |
| 完整 Admin Web | `web/antd-v6/v1.3.7` / `@mss-boot-io/admin-web@1.3.7` |
| 文档网站 | 从版本化 `docs/v*` 网站 Tag 独立部署 |

v1.3.5 和 v1.3.6 仍是不可变的部分发布列车。它们已有的 Tag、Release 和制品只是审计
证据，不能替代稳定列车。不得把停止列车组件与源码检出、本地 `replace` 或不存在的包
身份混用。

## 安装 v1.3.7 工具

版本化安装器会校验 Release 摘要并同时安装 `mss` 与 `mss-mcp`；它不会使用 `sudo`，
也不会修改 Shell Profile。

Linux 或 macOS：

```shell
curl -fL -o install-mss.sh \
  https://github.com/mss-boot-io/mss-boot-admin/releases/download/v1.3.7/install-mss.sh
bash ./install-mss.sh --version v1.3.7
export PATH="$HOME/.local/bin:$PATH"
mss --version
mss-mcp --version
```

Windows PowerShell：

```powershell
Invoke-WebRequest `
  https://github.com/mss-boot-io/mss-boot-admin/releases/download/v1.3.7/install-mss.ps1 `
  -OutFile install-mss.ps1
& .\install-mss.ps1 -Version v1.3.7
$env:Path = "$env:LOCALAPPDATA\Programs\mss\bin;$env:Path"
mss --version
mss-mcp --version
```

## 使用稳定组件

Go 与 npm 依赖必须精确锁定协调版本。Admin Module 没有提交本地 `replace`，官方 npmjs
包无需 Registry Token 即可安装。请在**已有外部 consumer module 的根目录**（即包含
`go.mod` 的目录）运行 Go 命令；空目录公共解析请使用[外部消费者步骤](./docs/docs/getting-started/packages.md)。
npm 命令必须另在**已有 frontend package 的根目录**（即包含 `package.json` 的目录，
Thin Host 中通常为 `web/`）运行。

Go Module 根目录：

```shell
go get github.com/mss-boot-io/mss-boot-admin/mss-boot@v1.3.7
go get github.com/mss-boot-io/mss-boot-admin/admin@v1.3.7
```

Frontend package 根目录：

```shell
corepack pnpm@10.34.5 add --save-exact @mss-boot-io/admin-web@1.3.7
```

npm `latest` 当前解析到 `1.3.7`，但 Thin Host 仍固定精确版本。官方 npm 发布使用
GitHub Actions Trusted Publishing，发布契约不保存 npm Token。

## 创建并启动 Thin Host

请从空的目标父目录调用公开工具，不要依赖 Foundation 源码检出：

```shell
mss new app orders-admin --module github.com/acme/orders-admin --repository acme/orders-admin --destination ./orders-admin --write --git-init

cd orders-admin
mss doctor --strict
mss setup
mss dev --detach
```

首次交互式 setup 会隐藏输入初始管理员密码。非交互自动化只能从密钥存储向单次
`mss setup` 进程注入 `MSS_ADMIN_INITIAL_PASSWORD`；不得把它放入参数、报告、生成文件、
依赖安装子进程或长期服务环境。初始化完成后，打开 `http://127.0.0.1:8001`，使用用户
`admin` 和刚设置的密码登录；项目没有默认密码。

## 升级 Thin Host

先备份代码、配置和数据库，安装目标 v1.3.7 工具，并在规划前核对两个二进制版本。
受支持的三方升级必须保留生成的 `.mss/blueprint-manifest.json`：

```shell
mss --version
mss-mcp --version
mss upgrade admin v1.3.7 --format json
mss upgrade admin v1.3.7 --apply --yes --format json
mss doctor --strict
mss verify --all
mss upgrade admin v1.3.7 --format json
```

第一次和最后一次命令都是只读计划；最终计划不得包含 create、update、delete 或 conflict。
升级引擎只修改 Blueprint 管理文件，并保留业务自有文件和未知文件。手工拼装或缺少
manifest 的仓库必须在新目录生成干净的 v1.3.7 基线，再迁入业务规格和自有文件，不能
伪造 manifest。

## 架构边界

生成应用是 **Thin Host**：它导入完整 Admin Go Module 与 Admin Web 包，只持有组合胶水
和业务模块，不复制 Foundation 核心源码。后端业务模块在编译期注册，前端业务路由扩展
已发布应用壳，后端授权始终是最终权威。

运行时动态模型、虚拟 CRUD 和浏览器代码生成保持移除。结构化 Feature 与 AdminModule
规格驱动开发期确定性生成，并形成可审查的迁移、权限、菜单、前端、测试与 Agent Eval
契约。

## 人类文档与 Agent 指令

仓库明确区分解释性文档与 Agent 可执行权威：

| 受众 | 入口 |
| --- | --- |
| 采用者、运维和贡献者 | README 与 `docs/docs/**` |
| 架构维护者 | `docs/adr/**` |
| Foundation AI Agent | 最近的 `AGENTS.md` -> `.mss/**` -> 对应 `.agents/skills/**` |
| 生成 Thin Host AI Agent | 生成仓库自己的 `AGENTS.md`、`.mss/**` 与本地 Skills |

公开的 [Agent 协作说明](./docs/docs/agent/index.md)面向人类解释这套模型，不是 Agent
可执行指令源。采用者请从[快速开始](./docs/docs/getting-started/index.md)、
[包边界](./docs/docs/getting-started/packages.md)、
[工具契约](./docs/docs/getting-started/tooling.md)与
[v1.3.7 发布记录](./docs/docs/releases/v1-3-7.md)开始。

Foundation 贡献者应按 [`CONTRIBUTING.md`](./CONTRIBUTING.md) 与最近的
`AGENTS.md` 使用源码检出命令和验证流程。

## 许可证与安全

项目使用 [MIT License](./LICENSE)。安全问题请按
[`SECURITY.md`](./SECURITY.md) 的私密流程报告，不要提交公开 Issue。
