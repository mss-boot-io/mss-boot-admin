---
title: FAQ
order: 3
description: v1.3.5 安装、创建、开发、验证和升级的常见问题
---

# FAQ

## 为什么不再先克隆 Foundation？

v1.3.5 的 `mss` 内置与自身版本、提交和时间戳绑定的 Blueprint。采用者只需要
Release 工具与公开 Go/npm 包；克隆流程只属于 Foundation 贡献者。

## 安装后找不到 mss

安装器不会修改 profile。当前终端可执行：

```sh
export PATH="$HOME/.local/bin:$PATH"
mss --version
```

PowerShell：

```powershell
$env:Path = "$HOME\.local\bin;$env:Path"
mss --version
```

需要持久化时由用户按自己的 shell 管理 PATH。

## 可以只下载一个二进制吗？

工具包固定包含 `mss`、`mss-mcp`、`BUILD-INFO` 与 `LICENSE`，并由一个摘要清单
校验。不要从未知来源拆分或改名下载。

## 为什么 mss new app 没写文件？

写入默认关闭。确认计划后加 `--write`；需要初始化 Git 时再加 `--git-init`。
目标目录存在未知文件时会拒绝覆盖。

## mss new app 需要访问哪些公共服务？

创建时会匿名读取 `registry.npmjs.org` 上精确版本的 Admin Web 元数据，把公开的
SHA-512 完整性值写入冻结锁；`mss setup` 随后按 Go 与 npm 的标准公共代理下载依赖。
如果公司代理阻断这些地址，命令会明确失败且不写入半成品。不要把 npm token、私有
镜像地址或临时本地包写进生成仓库来绕过发布合同。

## doctor 为什么在项目刚生成后失败？

`mss doctor --strict` 会检查 Go、Node、Corepack/pnpm、冻结锁、Go sums、Blueprint
快照与项目合同。按输出修复真实缺项，然后重新运行；不要关闭 strict 掩盖问题。

## setup 为什么要求初始管理员密码？

全新 Thin Host 的第一次 `mss setup` 会在本地 SQLite 中执行迁移并创建初始管理员。
交互终端使用内置隐藏提示；非交互自动化只在 setup 进程中从密钥存储注入一次性
`MSS_ADMIN_INITIAL_PASSWORD`。密码必须为 8-128 个字符并同时包含字母和数字，且不会
进入参数、报告或生成文件。迁移记录存在后，重复 `setup` 不再要求这个值。不要使用
命令行密码参数。

初始用户名固定为 `admin`。打开 `http://127.0.0.1:8001`，使用该用户名和首次 setup
时提供的密码登录；系统没有默认密码。

## setup 会连接生产系统吗？

不会。它按 Thin Host 合同安装冻结依赖并迁移本地 SQLite；初始管理员密码是本地启动
密钥，不是生产凭据。Redis 或其他外部提供方只有在配置明确启用后才属于运行时依赖。

## 前端包装好后为何仍不能构建？

确认 Node 24、Corepack pnpm 10.34.5、`@mss-boot-io/admin-web@1.3.5` 和冻结
`pnpm-lock.yaml` 一致。运行 `mss doctor --strict` 后再执行 `mss verify --all`。

## 如何升级？

先备份业务仓库和数据库，安装目标版本工具，并确认仓库仍有生成时的三方合并基线：

```sh
mss --version
mss-mcp --version
mss upgrade status --format json
mss upgrade admin v1.3.5
mss upgrade admin v1.3.5 --apply --yes
mss doctor --strict
mss verify --all
mss upgrade admin v1.3.5
```

第一次 upgrade 只读。只有看过无冲突计划和备份策略后才执行 apply；最后一次计划必须
为空。匹配的公开发行版不需要额外源码目录。

`.mss/blueprint-manifest.json` 缺失时不能直接升级。对手工拼装或基线丢失的仓库，使用
目标版本 `mss new app` 在新目录生成干净 Thin Host，再按业务所有权迁入规格和业务文件，
然后运行完整验证；不要手写或复制别人的 manifest。

## 能否混用不同补丁版本？

不能。Admin、Framework、Admin Web、工具、Blueprint 和锁记录构成一个协调发行版。
混用版本会失去资格验证证据。

## 何时直接修改生成文件？

不要直接修改生成区。能由规格表达的变化先修改 `.mss/` 规格，再生成并运行漂移检查。
业务所有文件不在生成管理范围内，可以正常维护。

## 从哪里确认版本是否已经公开？

查看 [v1.3.5 发布合同](/releases/v1-3-5)以及
[GitHub Release](https://github.com/mss-boot-io/mss-boot-admin/releases/tag/v1.3.5)。
本地分支或 PR 成功不代表公共包已经可用。
