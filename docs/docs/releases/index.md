---
title: 发布
order: 1
nav:
  title: 发布
  order: 6
description: v1.3.7 当前稳定、v1.3.5/v1.3.6 永久停止与 Docs 独立发布
---

# 发布

当前稳定版本是 **v1.3.7**。v1.3.5 与 v1.3.6 已永久停止，只保留不可变部分发布记录。
v1.3.7 的组件、npm、镜像、Root Release、稳定别名和 current-stable 策略已经对账完成。
Docs 网站可通过 `docs/v*` 独立异步候补，其状态不阻断组件发布、稳定推进或采用。

- [v1.3.7 当前稳定版说明](/releases/v1-3-7)
- [v1.3.7 快速开始](/getting-started)
- [v1.3.6 不可变部分发布记录](/releases/v1-3-6)
- [v1.3.5 不可变部分发布记录](/releases/v1-3-5)
- [v1.3.7 工具说明](/getting-started/tooling)
- [v1.3.4 组件部分发布记录](/releases/archive/v1-3-4)
- [历史版本归档](/releases/archive)

发布状态必须同时由精确 merged-main 提交、受保护工作流、标签、Release、Go 代理、
npmjs、镜像摘要和空目录使用方验证证明。PR 成功、标签存在或某个包可下载都不能单独
代表完整发行完成。

v1.3.5 已发布 Framework、Admin 与 Admin Web，并由受保护 promotion 创建了公开 Root
Tag；随后标签消息校验错误使 promotion 失败，Root Release 与工具、Docs、官方 npmjs
包和后端镜像均未发布。自然触发的 Root candidate 与 container 运行在公开前取消。
v1.3.5 不会成为完整可安装版本，所有已公开身份保持不可变。

完整修复已由 v1.3.7 稳定版承接，并从精确 merged-main 提交完成完整资格、发布和最终对账。
v1.3.6 从
`b1fe47a3a83209574e09d53526b122dd2cbc5277` 公开了 Framework、Admin、Admin Web 与
Root，但 Root image 和 npm 失败、Docs 未创建，因此同样冻结为不可续的部分列车。

历史页只提供其自身版本的不可变证据；它们不参与当前 v1.3.7 安装或升级决策。
