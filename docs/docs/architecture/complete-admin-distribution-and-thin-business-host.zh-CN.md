---
title: Complete Admin Distribution 与 Thin Host
order: 2
description: v1.3.7 稳定 Admin 发行架构与 v1.3.5/v1.3.6 永久停止事实
---

# Complete Admin Distribution 与 Thin Host

## 当前版本状态

:::warning
发布策略将 **v1.3.7** 定义为当前稳定版本；v1.3.5 与 v1.3.6 已永久停止并保持不可变部分发布。
安装、创建或升级从[快速开始](/getting-started)进入。Docs 网站可异步候补，不阻断组件、
稳定别名或采用。
:::

| 组件 | v1.3.5 身份 | 实际结果 |
| --- | --- | --- |
| Root | `v1.3.5` | 只有不可变 Tag；Release、工具与后端镜像未发布 |
| Framework | `mss-boot/v1.3.5` | GitHub Release 与 Go Module 已发布 |
| Admin | `admin/v1.3.5` | GitHub Release 与 Go Module 已发布 |
| Admin Web | `web/antd-v6/v1.3.5` | GitHub Release、GitHub Packages 与前端镜像已发布 |
| npmjs | `@mss-boot-io/admin-web@1.3.5` | 未发布 |
| Blueprint | 原计划嵌入 v1.3.5 Root 工具 | 没有可消费的 Release 工具 |
| Docs | `docs/v1.3.5` | 未创建、未部署 |

所有已公开身份保持不可变，但不能与 v1.3.7、分支头、Foundation 源码、本地替换或其他
registry 拼成完整发行。

v1.3.6 从 `b1fe47a3a83209574e09d53526b122dd2cbc5277` 公开了 Framework、Admin、
Admin Web 与 Root Release/工具，但 Root image 与官方 npm 发布失败，Docs 未创建。
这些身份也保持不可变，不能补发、混用或由 v1.3.7 修复源码完成。

## 架构决策

v1.3.7 Admin Distribution 由 Root 工具、Framework、Admin、Admin Web 与 Blueprint
使用一个协调版本，同时保留独立发布身份。Docs 源码说明同一稳定基线，但网站发布是
独立异步链路，不属于 Distribution 的完整性或采用门禁。下游应用是 Thin Host，不是
Foundation 源码副本。

```text
cmd/server/main.go                 Admin 组合入口
internal/modules/<business>/       业务后端所有
web/src/business/                  业务前端所有
.mss/                              项目、模块、命令和锁合同
受管配置/构建胶水                  Blueprint 所有
```

Thin Host 不包含 Admin 核心路由、中间件、迁移、Framework、完整前端页面、Foundation
文档或发布工作流副本。

## 后端组合

`admin/app` 构造完整应用，`admin/business` 接收显式有序模块。模块注册在事务式
registry 中完成；迁移冲突、路由冲突、重入或部分注册失败必须 fail closed。所有业务
API 位于核心安全中间件后的受保护组中。

后端模块可以拥有前向迁移、模型、DTO、服务、受保护 API、权限、菜单投影与就绪检查；
它不能替换认证、会话、CSRF、核心路由或迁移 registry。正向和负向授权测试必须同时
证明允许与拒绝路径。

## 前端组合

完整发行只提供一个 Admin Web SPA。Thin Host 使用公开 preset、runtime 和 business
导出注册业务路由与菜单；核心路由先注册，业务扩展随后注册，403/404 回退最后注册。

业务页面覆盖 loading、empty、retryable error、denied、responsive 和 locale。浏览器
route guard 只改善体验，不能替代后端授权。v1.3.5 的概念性包身份不代表官方 npmjs
安装路径存在。

## Blueprint 与三方升级

v1.3.7 Root Release 已把单一 Blueprint 源嵌入工具，并绑定版本、源提交、构建时间和仓库。
创建流程先产生只读计划，再检查路径与未知文件、原子写入受管文件、固定同一完整版本的
公共依赖，并写入 manifest、lock 与同源快照；第二次生成必须无差异。

三方升级比较：

```text
旧 Blueprint 基线 + 当前 Thin Host + 新完整版本基线
                         │
                         ▼
                 只读计划 / 冲突列表
                         │ 人工确认
                         ▼
                       应用
```

仓库必须保留 `.mss/blueprint-manifest.json`。手工拼装或基线丢失的仓库迁入由目标完整
版本生成的新 Thin Host，不得伪造摘要。应用只更新受管文件，保留业务和未知文件，最后
更新快照；再次计划必须为空。v1.3.5 缺少 Root 工具与完整包图，不能执行这条升级路径。

## 完整发布与公共验证

后续未使用版本先从同一个精确 merged-main 提交完成候选预演，再依次创建 Framework、
Admin、Admin Web 与 Root 正式 Tag。各 Tag 只触发自己的发布面；Root Tag 发布 Root
Release、工具与 Root 镜像，不发布 npm。Root 对账后，单独的已评审策略 PR 一次性授权
从精确 Root Tag 手工调度 `npm-release.yml`，验证 npm provenance 后再推进 npm `latest`
与 GitHub Latest。Docs 不参与上述顺序或门禁；网站内容确认后由可替换的
`docs/vX.Y.Z` Tag 异步发布。更新时先删同名 Docs Release 和 Tag，再从受审 merged-main
后代同名重建，禁止直接 force-update。公共对账仍需匿名解析 Go/npm 依赖，
并覆盖生成、升级、测试、构建、运行、权限与浏览器关键流程。

任一 Distribution 门禁失败，后续 Distribution 阶段保持关闭。需要修复时通过后续 PR
合入 main，选择新的 merged-main 提交并重新资格；Docs 网站失败只阻断自身部署，不回退
已通过的组件证据，也不得移动或补附任何核心已公开身份。

## mss-shop 证明

[mss-shop](/getting-started/mss-shop) 必须等待维护者显式选择的未使用完整版本，再提交
未修改生成基线并加入通用单租户商城模块。它验证包导入、业务所有权、升级、自动测试和
Codex 内置浏览器流程，而不是复制 R1Shop 或 Foundation。
