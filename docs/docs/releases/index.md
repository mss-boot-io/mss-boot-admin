---
title: 发布与升级
order: 1
nav:
  title: 发布
  order: 3
description: mss-boot Admin Distribution 的版本状态、安装、升级、兼容性、不可变制品与回滚合同
keywords: [release upgrade rollback compatibility mss-boot-admin thin host]
---

# 发布与升级

:::info
**发布状态**

<code>v1.3.2</code> 已从精确 merged-main 提交
<code>635fbb03a82976941e527d8ac1000fec0624abac</code> 完成协调发布和公开制品对账，
现为当前稳定版。<code>v1.2.3</code> 保留为上一稳定版与协调回滚基线。
:::

## 当前版本

| 版本 | 状态 | 用途 |
| --- | --- | --- |
| [v1.3.2](/releases/v1-3-2) | 当前稳定版 / 已完整发布 | 最终 Framework 校验和绑定、Complete Admin Distribution、Thin Host、正式安装与升级合同 |
| [v1.3.1](/releases/v1-3-1) | component-partial / 不复用 | Framework 已发布；Admin Tag 存在但校验和资格验证失败；其余组件未发布 |
| [v1.3.0](/releases/v1-3-0) | component-partial / 不复用 | Framework 已发布，Admin 发布失败，其余稳定组件未发布 |
| [v1.2.3](/releases/v1-2-3) | 上一稳定版 / 已发布 | v1.3.2 的协调回滚基线与历史证据 |
| v1.3.0-rc.6 | 完整预览 / 已发布 | 从一个精确 merged-main 提交完成四组件列车，作为 stable 资格证据 |

<code>v1.3.2</code> 的协调版本同时覆盖：

| 组件 | 目标身份 |
| --- | --- |
| Root | <code>v1.3.2</code> |
| Framework | <code>mss-boot/v1.3.2</code> |
| Admin Go Module | <code>admin/v1.3.2</code> |
| Admin Web | <code>web/antd-v6/v1.3.2</code> 与 <code>@mss-boot-io/admin-web@1.3.2</code> |
| Docs | 独立的 <code>docs/v1.3.2</code> |

Root、Framework、Admin、Admin Web 和 Docs 均发布自同一个已合并 <code>main</code> 的精确提交
<code>635fbb03a82976941e527d8ac1000fec0624abac</code>。Docs 仍保留独立标签和发布工作流。

## v1.3.2 当前稳定版

RC6 已证明可导入 Admin、完整 Admin Web、外部 Thin Host、单 Runtime、生成与升级、浏览器
行为和协调发布路径。v1.3.2 在此基础上修复了 v1.3.1 的错误校验和：发布前从最终
Framework 跟踪文件生成 replace-free 本地 Go Proxy，由 Go 计算规范 Module/GoMod Sum，
并与 Admin 元数据逐项比较。全部组件、双架构镜像、GitHub Release、官方 npm 包、Docs 与
外部消费者随后从同一提交完成验证和公开对账。

本次稳定版对账包括：

1. checkpoint、feature-freeze、pre-framework 与独立 pre-root 门禁全部通过；
2. 三数据库、API 注册表、Codex 内置浏览器和外部 Thin Host 证据绑定同一提交；
3. 按 Framework → Admin → Admin Web → Root → Docs → 官方 npm 顺序公开；
4. 公开 Go Module、npm integrity/provenance、镜像 digest/架构、Release 资产与
   <code>latest</code> 均完成只读对账；
5. 机器契约将 <code>v1.3.2</code> 与精确发布提交记录为 current stable。

完整操作见 [v1.3.2 发布、安装、升级与回滚合同](/releases/v1-3-2)，证据索引见
[GitHub issue #519](https://github.com/mss-boot-io/mss-boot-admin/issues/519)。

## v1.3.0 预览列车审计

以下记录保持不可变。失败的 RC 只能通过更高版本前向修复，不能移动标签或覆盖制品。

| 列车 | 不可变结果 |
| --- | --- |
| RC1 | Framework、Admin、前端镜像已发布；多架构校验缺陷阻止前端包和后续 Release |
| RC2 | 修复镜像校验；npm 预发布缺少显式 dist-tag，前端包和后续 Release 未发布 |
| RC3 | Admin Web 包已发布；GitHub Packages dist-tag 查询方式错误，前端与 Root Release 未完成 |
| RC4 | Framework、Admin、前端已发布；保留 Thin Host 暴露 Supplier 迁移顺序冲突，Root Release 未完成 |
| RC5 | Framework、Admin、前端包/镜像和 Root 镜像已发布；旧的“大型复制仓库”评测阻止 Root Release |
| RC6 | Framework、Admin、完整 Admin Web 包/镜像、Root 镜像和 Root GitHub Release 全部从同一提交发布 |

历史预览引用说明：

- [v1.3.0-rc.5 引用与 RC4 前向修复](/operations/admin-distribution-v1/3/0-rc/5)
- [v1.3.0-rc.4 引用与 RC1–RC4 审计](/operations/admin-distribution-v1/3/0-rc/4)

## v1.2.x 历史

| 版本 | 状态 | 记录 |
| --- | --- | --- |
| v1.2.3 | 已完整发布、上一稳定版 | [发布合同](/releases/v1-2-3) |
| v1.2.2 | component-partial | [根镜像发布未完成](/releases/v1-2-2) |
| v1.2.1 | component-partial | [根便携资产发布未完成](/releases/v1-2-1) |
| v1.2.0 | component-partial | [Framework 已发布，其他组件不完整](/releases/v1-2-0) |

已有标签、Release、包、镜像和证据均保持不可变；不会为了补齐旧列车而移动或复用引用。

## 更早的稳定与开发记录

- [v1.0.0 发布合同](/releases/v1-0-0)
- [v1.0.0 升级](/releases/v1-0-0-upgrade)
- [v1.0.0 兼容性](/releases/v1-0-0-compatibility)
- [v1.0.0 回滚](/releases/v1-0-0-rollback)
- [v1.1.0 开发优先路线与 checkpoint 索引](/releases/v1-0-1-to-v1-1-0-roadmap)
- [v1.1.0 exact readiness attestation](/releases/v1-1-0-exact-readiness-attestation)

这些页面是历史审计材料，不再授权当前版本发布。

## 永久发布规则

1. 所有发布变更先通过 Pull Request 合并到 <code>main</code>。
2. 发布候选冻结自一个干净、精确、包含于最新 <code>origin/main</code> 的提交。
3. 发布前证据可以支持评审，但不能从 PR head、topic branch、detached commit 或本地替换发布。
4. 冻结后任何修复都要新建 PR、选择新的 <code>main</code> 提交并重跑受影响门禁。
5. 已公开标签和制品不可移动、覆盖、删除或解释成另一版本。
6. 首次写入前失败可停止；部分组件已经公开后，记录 component-partial 并用更高版本前向修复。
7. 回滚使用上一组协调制品和匹配数据库备份，不临时编写 down migration。
