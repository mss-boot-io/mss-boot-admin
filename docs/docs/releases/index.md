---
title: 发布
order: 1
nav:
  title: 发布
  order: 6
description: v1.3.5 候选发布合同与只读历史归档
---

# 发布

当前文档面向 **v1.3.5 Complete Admin Distribution 候选列车**：

- [v1.3.5 发布、安装、升级与回滚](/releases/v1-3-5)
- [v1.3.5 快速开始](/getting-started)
- [公开工具资产](/getting-started/tooling)
- [v1.3.4 组件部分发布记录](/releases/archive/v1-3-4)
- [历史版本归档](/releases/archive)

发布状态必须同时由精确 merged-main 提交、受保护工作流、标签、Release、Go 代理、
npmjs、镜像摘要和空目录使用方验证证明。PR 成功、标签存在或某个包可下载都不能单独
代表完整发行完成。

v1.3.4 只完成 Framework、Admin 与 Admin Web 组件发布；Root、Docs 和 npmjs 未发布。
这些公开身份保持不可变，完整修复列车使用 v1.3.5。v1.3.5 只有在全部公开身份和
空目录使用方验证完成对账后才成为可安装版本；源码页和候选 Tag 都不能提前证明完成。

历史页不参与当前安装或升级决策；公共历史 ref 永久不可变。
