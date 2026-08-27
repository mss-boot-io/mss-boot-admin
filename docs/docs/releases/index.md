---
title: 发布
order: 1
nav:
  title: 发布
  order: 6
description: 当前稳定 v1.3.2、v1.3.5 不可变部分发布与只读历史归档
---

# 发布

当前稳定版本是 **v1.3.2**。v1.3.5 只保留不可变部分发布记录，不能作为安装、创建或
升级基线。

- [v1.3.2 当前稳定记录](/releases/archive/v1-3-2)
- [v1.3.5 不可变部分发布记录](/releases/v1-3-5)
- [v1.3.5 采用状态](/getting-started)
- [v1.3.5 Root 工具未发布状态](/getting-started/tooling)
- [v1.3.4 组件部分发布记录](/releases/archive/v1-3-4)
- [历史版本归档](/releases/archive)

发布状态必须同时由精确 merged-main 提交、受保护工作流、标签、Release、Go 代理、
npmjs、镜像摘要和空目录使用方验证证明。PR 成功、标签存在或某个包可下载都不能单独
代表完整发行完成。

v1.3.5 已发布 Framework、Admin 与 Admin Web，并由受保护 promotion 创建了公开 Root
Tag；随后标签消息校验错误使 promotion 失败，Root Release 与工具、Docs、官方 npmjs
包和后端镜像均未发布。自然触发的 Root candidate 与 container 运行在公开前取消。
v1.3.5 不会成为完整可安装版本，所有已公开身份保持不可变；完整修复需要尚未选择的
未使用版本和新的 merged-main 资格证据。

历史页只提供其自身版本的不可变证据；除当前稳定 v1.3.2 外，不参与当前安装或升级决策。
