# mss-boot 完整 Admin 发行版

[English](./README.md)

mss-boot 是面向 Agent 的管理系统基础设施。**v1.3.7 已选为完整 package-first 发布候选，
但尚未稳定且不可采用。** 候选 Distribution 发布面可能处于不同公开阶段，必须以远端发布
台账为准；完整 stable promotion 和最终 `currentStableVersion` policy 对账完成前，不得使用
v1.3.7 安装、创建或升级。Docs 是异步、非阻断的网站发布；`docs/v*` 只标识网站部署，不决定
组件、稳定别名、current stable 或采用状态。
v1.3.5 与 v1.3.6 都是不可变部分发布列车；不得删除、移动、重建、复用或补全其中任何身份。

## 当前可用状态

发布策略仍将 **v1.3.2** 定义为当前协调稳定发行版；其不可变发布记录继续作为已有采用者
的稳定边界。

两条停止列车的身份命名空间继续作为明确审计证据：

| 列车 | Framework | Admin | 官方 npm 身份 | Docs 身份 |
| --- | --- | --- | --- | --- |
| v1.3.5 | `github.com/mss-boot-io/mss-boot-admin/mss-boot@v1.3.5` | `github.com/mss-boot-io/mss-boot-admin/admin@v1.3.5` | `@mss-boot-io/admin-web@1.3.5` | `docs/v1.3.5` |
| v1.3.6 | `github.com/mss-boot-io/mss-boot-admin/mss-boot@v1.3.6` | `github.com/mss-boot-io/mss-boot-admin/admin@v1.3.6` | `@mss-boot-io/admin-web@1.3.6` | `docs/v1.3.6` |

表中有些 Go 或 GitHub 身份已存在，而 npm 或 Docs 身份缺失；列出名称不代表它已经发布。

v1.3.6 只从精确提交 `b1fe47a3a83209574e09d53526b122dd2cbc5277` 发布了部分列车：

| 入口 | v1.3.6 结果 | 可用性 |
| --- | --- | --- |
| Framework | `mss-boot/v1.3.6` | 公开 Go 组件与 GitHub Release |
| Admin | `admin/v1.3.6` | 公开 Go 组件与 GitHub Release |
| Admin Web | `web/antd-v6/v1.3.6` | GitHub Release 与 GitHub Packages；官方 npmjs 缺失 |
| Root | `v1.3.6` | Root Release 与工具公开；Root 镜像缺失 |
| Docs | 原计划 `docs/v1.3.6` | 未发布 |

这些组件身份保持不可变，但不能组成完整 Thin Host 发行版。不得把它们与 v1.3.2、源码
检出、本地替换或未发布包混合使用。

v1.3.7 是新的候选。它必须先从修复后的精确 merged-main 提交完成一次非发布 preview，
包含真实 Root OCI artifact，并核对 `npm-release.yml` 与 `npm-auto` 的无令牌 Trusted
Publisher 绑定。候选 Distribution 发布面随后按治理顺序公开，可能处于不同阶段；完整
stable promotion 和最终 current-stable policy 对账完成前，不支持 v1.3.7 下载、安装、创建
或升级流程。Docs 可在此前或此后独立候补，其缺失、失败或站点滞后不影响这一边界。

## 采用者状态

v1.3.5 与 v1.3.6 都没有受支持的安装器、空目录应用创建、本地初始化或发行版升级流程。v1.3.7 现已
选为这些 package-first 接口的候选，但在 stable promotion、外部 Thin Host 验收和最终
current-stable policy 对账完成前，当前入门文档仍不提供可执行命令。公开 Docs 部署单独记录，
不是采用前置条件。

当前稳定边界见 [v1.3.2 稳定记录](./docs/docs/releases/archive/v1-3-2.md)，
不可变审计证据见
[v1.3.5 部分发布记录](./docs/docs/releases/v1-3-5.md) 与
[v1.3.6 部分发布记录](./docs/docs/releases/v1-3-6.md)。
[v1.3.7 候选记录](./docs/docs/releases/v1-3-7.md) 只描述恢复、迁移、安全与回退边界，
不声称已经发布。

## 架构边界

v1.3.7 候选继续把生成应用保持为 **Thin Host**：它精确固定一个协调版本的 Admin Go
Module 与 Admin Web 包，只持有组合胶水和业务模块，不复制 Foundation 核心源码。后端
业务模块在编译期显式注册，前端业务路由扩展已发布应用壳，后端授权始终是最终权威。
这项候选架构合同不代表 v1.3.7 已可采用，也不代表 v1.3.5 或 v1.3.6 已被补全。

## 文档

仓库明确区分人类说明和 Agent 可执行权威：

| 受众 | 入口 |
| --- | --- |
| 采用者、运维和贡献者 | README 与 `docs/docs/**` |
| 架构维护者 | `docs/adr/**` |
| Foundation AI Agent | 最近的 `AGENTS.md` → `.mss/**` → 对应 `.agents/skills/**` |
| 生成 Thin Host AI Agent | 生成仓库自己的 `AGENTS.md`、`.mss/**` 与本地 Skills |

公开的 [Agent 协作说明](./docs/docs/agent/index.md)面向人类解释这套模型；它不是 Agent
可执行指令源，也不会把 Foundation 维护技能混入 Thin Host 能力。

- [采用者与组件状态](./docs/docs/getting-started/index.md)
- [已发布组件与导入边界](./docs/docs/getting-started/packages.md)
- [工具发布状态](./docs/docs/getting-started/tooling.md)
- [mss-shop 范本状态](./docs/docs/getting-started/mss-shop.md)
- [v1.3.7 发布候选记录](./docs/docs/releases/v1-3-7.md)
- [v1.3.6 不可变部分发布记录](./docs/docs/releases/v1-3-6.md)
- [v1.3.5 不可变部分发布记录](./docs/docs/releases/v1-3-5.md)

Foundation 贡献者请阅读 [`CONTRIBUTING.md`](./docs/CONTRIBUTING.md) 与最近的
`AGENTS.md`。源码检出命令
与下游入门路径明确隔离。

## 许可证与安全

项目使用 [MIT License](./LICENSE)。安全问题请按
[`SECURITY.md`](./SECURITY.md) 的私密流程报告，不要提交公开 Issue。
