# mss-boot 完整 Admin 发行版

[English](./README.md)

mss-boot 是面向 Agent 的管理系统基础设施。**v1.3.6 已选为完整 package-first 发布候选，
但尚未公开，也不是稳定版。** v1.3.5 仍是不可变的部分发布列车：Framework、Admin、
Admin Web 与 Root Tag 已公开，但 Root Release、工具、Docs 和公开 npmjs 包未发布。
不得删除、移动、重建、复用或补全任何 v1.3.5 身份。

## 当前可用状态

发布策略仍将 **v1.3.2** 定义为当前协调稳定发行版；其不可变发布记录继续作为已有采用者
的稳定边界。

v1.3.6 已预留为下一个候选。其带摘要工具、Framework、Admin、Admin Web、npmjs 包、
镜像和 Root Release 必须先从精确 merged-main 提交完成一次非公开 preview，再从同一提交
发布并对账。ledger 完成之前，不支持 v1.3.6 下载、安装、创建或升级流程。

v1.3.5 只从提交 `396f60615cdfa589353b16ef9d3531e249e65432` 发布了部分列车：

| 入口 | v1.3.5 公开身份 | 可用性 |
| --- | --- | --- |
| Framework | `github.com/mss-boot-io/mss-boot-admin/mss-boot@v1.3.5` | 公开 Go 组件 |
| Admin | `github.com/mss-boot-io/mss-boot-admin/admin@v1.3.5` | 公开 Go 组件 |
| Admin Web | `web/antd-v6/v1.3.5` 与 `@mss-boot-io/admin-web@1.3.5` Release 资产 | 仅 GitHub Release 与 GitHub Packages；npmjs 缺失 |
| Root | annotated `v1.3.5` Tag | 只有 Tag；没有 Root Release、工具或 Root 镜像 |
| Docs | 原计划 `docs/v1.3.5` | 未发布 |

这些组件身份保持不可变，但不能组成完整 Thin Host 发行版。不得把它们与 v1.3.2、源码
检出、本地替换或未发布包混合使用。

## 采用者状态

v1.3.5 没有受支持的安装器、空目录应用创建、本地初始化或发行版升级流程。v1.3.6 现已
选为这些 package-first 接口的候选，但在公共制品与外部 Thin Host 验收完成对账前，当前
入门文档仍不提供可执行命令。

当前稳定边界见 [v1.3.2 稳定记录](./docs/docs/releases/archive/v1-3-2.md)，
不可变审计证据见
[v1.3.5 部分发布记录](./docs/docs/releases/v1-3-5.md)。
[v1.3.6 候选记录](./docs/docs/releases/v1-3-6.md) 只描述包、迁移、安全与回退边界，
不声称已经发布。

## 架构边界

v1.3.6 候选继续把生成应用保持为 **Thin Host**：它精确固定一个协调版本的 Admin Go
Module 与 Admin Web 包，只持有组合胶水和业务模块，不复制 Foundation 核心源码。后端
业务模块在编译期显式注册，前端业务路由扩展已发布应用壳，后端授权始终是最终权威。
这项候选架构合同不代表 v1.3.6 已可采用，也不代表不完整的 v1.3.5 已被补全。

## 文档

- [采用者与组件状态](./docs/docs/getting-started/index.md)
- [已发布组件与导入边界](./docs/docs/getting-started/packages.md)
- [工具发布状态](./docs/docs/getting-started/tooling.md)
- [mss-shop 范本状态](./docs/docs/getting-started/mss-shop.md)
- [v1.3.6 发布候选记录](./docs/docs/releases/v1-3-6.md)
- [v1.3.5 不可变部分发布记录](./docs/docs/releases/v1-3-5.md)

Foundation 贡献者请阅读 [`CONTRIBUTING.md`](./docs/CONTRIBUTING.md)。源码检出命令
与下游入门路径明确隔离。

## 许可证与安全

项目使用 [MIT License](./LICENSE)。安全问题请按
[`SECURITY.md`](./SECURITY.md) 的私密流程报告，不要提交公开 Issue。
