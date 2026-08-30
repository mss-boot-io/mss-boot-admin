---
title: FAQ
order: 3
description: v1.3.7 未稳定候选、v1.3.5/v1.3.6 永久停止与 v1.3.2 稳定边界 FAQ
---

# FAQ

:::warning
发布状态：v1.3.2 仍是当前稳定版；v1.3.5 与 v1.3.6 已永久停止并保持不可变部分发布；v1.3.7 已选
为 release candidate，但尚未稳定且不可采用。候选发布面可能处于不同公开阶段，必须以
远端发布台账为准；完整 stable promotion 和最终 policy/Docs 对账完成前，不开放安装、创建
或升级命令。
:::

## v1.3.7 现在可以安装或升级吗？

不可以。v1.3.7 只是未稳定候选，必须先完成唯一非发布 preview、正式组件与 Root 候选发布、
候选 Docs、单独受审的 stable promotion，以及最终 policy/Docs 对账。任何源码构建、本地包
或单个候选制品都不能代替这条证据链。

## v1.3.6 现在可以安装或升级吗？

不可以。v1.3.6 已从
`b1fe47a3a83209574e09d53526b122dd2cbc5277` 公开 Framework、Admin、Admin Web 与
Root Release/工具，但 Root image 与官方 npm 发布失败，Docs 未创建。公开身份不可变，
不能 rerun 补发、恢复 npm token、混入本地包或用 v1.3.7 修复源码完成。

## v1.3.5 现在可以安装或升级吗？

不可以。v1.3.5 已停止为不可变部分发布：Framework、Admin、Admin Web 的部分身份与
Root Tag 已公开，但 Root Release 与工具、官方 npmjs 包、Docs 和后端镜像未发布。
当前稳定版本仍是 **v1.3.2**。

已公开 v1.3.5 身份不能删除、移动、重建或补附缺失制品，也不能与 v1.3.2、源码、本地包
或其他 registry 混合成完整发行。

## 为什么文档不再展示 v1.3.5 安装和升级命令？

这些命令依赖不存在的 Root 工具、官方 npmjs 包或后端镜像。即使标注“不可执行”，代码
块仍容易被复制为快速开始，因此活跃文档只保留实际发布状态和未来合同。资产名称或包名
可以作为审计身份出现，但不代表存在可执行下载或安装路径。

## 已公开的 Admin Go Module 可以单独使用吗？

`github.com/mss-boot-io/mss-boot-admin/admin@v1.3.5` 是真实、不可变的公开组件；它只
证明 Admin Go Module 发布，不证明完整 Admin Distribution。缺少官方 npmjs 包、Root
工具与完整外部使用方资格时，不应把它单独组装成 Thin Host。

## Admin Web 为什么显示已发布却不能从 npmjs 安装？

`web/antd-v6/v1.3.5` 的 GitHub Release、GitHub Packages 和前端镜像已公开，而官方
npmjs 身份 `@mss-boot-io/admin-web@1.3.5` 未发布。不同发布面不能互相替代；GitHub
Packages、Release tarball 或本地包不能冒充官方 npmjs 对账。

## 为什么不能直接使用 v1.3.5 容器？

后端 Root 镜像 `ghcr.io/mss-boot-io/mss-boot-admin:v1.3.5` 未发布。前端参考镜像虽已
公开，也不包含业务模块、组合入口或业务路由，不能单独构成 Thin Host。生产应用还必须
构建并固定自身业务镜像的不可变 digest。

## 未来为什么不再要求先克隆 Foundation？

未来完整 Root Release 的工具会内置与版本、源提交和构建身份绑定的 Blueprint。采用者
从公开工具和包生成 Thin Host；Foundation checkout 只服务贡献者。v1.3.5 没有发布这套
Root 工具，因此这项设计不能被解释为当前创建入口。

## 未来创建流程怎样保护现有文件？

它应默认先返回只读计划，验证目标目录、未知文件、公共包身份与冻结锁，再在显式确认后
原子写入。生成结果记录 Blueprint manifest 与同源快照，两次生成必须稳定；未知文件和
业务所有文件不得被静默覆盖。

## 初始管理员凭据的未来合同是什么？

完整 Thin Host 的首次初始化使用隐藏输入；非交互自动化只向初始化进程注入一次性
`MSS_ADMIN_INITIAL_PASSWORD`。密码不进入参数、报告、生成文件或长期服务环境，只存
一向验证值。生成应用的初始用户名为 `admin`，本地地址为
`http://127.0.0.1:8001`，系统没有默认密码。这是安全合同，不表示 v1.3.5 初始化工具
已经发布。

## 未来三方升级需要哪些前置条件？

目标完整发行的 Root 工具、Blueprint、Admin、Framework、Admin Web、官方 npmjs 与锁
必须同源；仓库还要保留 `.mss/blueprint-manifest.json`。升级前备份代码、配置、数据库
和业务数据，先评审只读计划，冲突清零后才应用，最后再次计划必须为空。

手工拼装或丢失 manifest 的仓库不能伪造摘要。它应迁入由目标完整版本生成的新 Thin
Host，再按所有权迁入规格和业务文件。v1.3.5 不具备这些前置条件，因此没有受支持的
升级命令。

## 能否混用不同补丁版本？

不能。Admin、Framework、Admin Web、工具、Blueprint 和锁记录构成一个协调发行版。
混用版本会失去公共资格验证证据。

## 何时可以直接修改生成文件？

不要直接修改生成区。能由规格表达的变化先修改 `.mss/` 规格，再生成并运行漂移检查。
业务所有文件不在生成管理范围内，可以正常维护。

## 从哪里确认版本状态？

当前采用判断见[采用状态](/getting-started)，部分列车的不可变事实见
[v1.3.5 部分发布记录](/releases/v1-3-5) 与
[v1.3.6 部分发布记录](/releases/v1-3-6)，当前稳定历史见
[v1.3.2 稳定记录](/releases/archive/v1-3-2)。本地分支、PR 成功、一个标签或一个组件
可下载都不能单独证明完整发行。
