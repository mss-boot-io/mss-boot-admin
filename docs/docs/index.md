---
title: mss-boot Admin 采用状态
hero:
  title: mss-boot Admin
  description: v1.3.2 当前稳定；v1.3.5/v1.3.6 永久停止；v1.3.7 是尚未发布的候选
  actions:
    - text: 查看采用状态
      link: /getting-started
    - text: 查看 v1.3.5 部分发布
      link: /releases/v1-3-5
    - text: 查看 v1.3.6 部分发布
      link: /releases/v1-3-6
    - text: 查看 v1.3.7 候选
      link: /releases/v1-3-7
features:
  - title: Current stable
    emoji: ✅
    description: v1.3.2 仍是当前稳定基线；稳定命令与资产只从该版本记录进入。
  - title: Immutable partial
    emoji: 🧊
    description: v1.3.5 与 v1.3.6 各自公开了部分组件或 Root 身份，但都有缺失面，不能补发或拼装。
  - title: Selected candidate
    emoji: 🔒
    description: v1.3.7 已选但尚未发布；完整公开对账前不展示安装、创建、开发或升级命令。
---

# 当前采用边界

:::warning
v1.3.5 已停止为不可变部分发布：Framework、Admin、Admin Web 与 Root Tag 已公开；Root
Release 及工具、官方 npmjs 包、Docs 和后端镜像未发布。已公开身份保持不可变，缺失制品
不得补附，也不能用本地包、源码或其他 registry 拼成完整发行。

v1.3.6 从 `b1fe47a3a83209574e09d53526b122dd2cbc5277` 公开了 Framework、Admin、
Admin Web 与 Root Release/工具，但 Root image 和官方 npm 发布失败，Docs 未创建。它也
是不可续的不可变部分发布，不能用 token、rerun 或 v1.3.7 源码补齐。

v1.3.7 已选为 release candidate，但尚未发布。当前稳定版本仍是 v1.3.2；本页只描述候选
和 source-only 合同，公共制品完成对账前 v1.3.7 不可采用。
:::

当前稳定版本仍是 **v1.3.2**。需要可执行的稳定安装资料时，请从
[v1.3.2 稳定记录](/releases/archive/v1-3-2)进入；不要把本页的部分列车组件事实解释为
安装、创建或升级指引。

## 按任务阅读

| 目标 | 文档 |
| --- | --- |
| 判断当前能否采用 | [采用状态](/getting-started) |
| 查看包与组件边界 | [包发布状态](/getting-started/packages) |
| 查看 Root 工具缺口 | [工具发布状态](/getting-started/tooling) |
| 了解 mss-shop 前置条件 | [mss-shop 范本采用状态](/getting-started/mss-shop) |
| 审计 v1.3.5 事实 | [v1.3.5 不可变部分发布记录](/releases/v1-3-5) |
| 审计 v1.3.6 事实 | [v1.3.6 不可变部分发布记录](/releases/v1-3-6) |
| 阅读当前候选 | [v1.3.7 发布候选说明](/releases/v1-3-7) |
| 查找当前稳定资料 | [v1.3.2 稳定记录](/releases/archive/v1-3-2) |
| 阅读其他历史版本 | [历史归档](/releases/archive) |

配置、开发和 Agent 页面描述产品与仓库合同；在部分发布状态下，它们不能替代
缺失的 Root、npmjs 与 Docs 发布证据。
