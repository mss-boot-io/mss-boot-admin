---
title: 发布
order: 1
nav:
  title: 发布
  order: 6
description: v1.3.3 当前发布合同与只读历史归档
---

# 发布

当前文档面向 **v1.3.3 Complete Admin Distribution**：

- [v1.3.3 发布、安装、升级与回滚](/releases/v1-3-3)
- [v1.3.3 快速开始](/getting-started)
- [公开工具资产](/getting-started/tooling)
- [历史版本归档](/releases/archive)

发布状态必须同时由精确 merged-main 提交、受保护工作流、标签、Release、Go 代理、
npmjs、镜像摘要和空目录使用方验证证明。PR 成功、标签存在或某个包可下载都不能单独
代表完整发行完成。

历史页不参与当前安装或升级决策；公共历史 ref 永久不可变。
