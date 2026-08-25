---
title: mss-boot Admin v1.3.3
hero:
  title: mss-boot Admin
  description: 通过已发布工具和包创建、开发、验证与升级可维护的 Thin Host
  actions:
    - text: 5 分钟快速开始
      link: /getting-started
    - text: 查看 v1.3.3
      link: /releases/v1-3-3
features:
  - title: Package first
    emoji: 📦
    description: 精确固定 Admin Go Module 与 Admin Web npm 包，不复制 Foundation 核心源码。
  - title: Agent native
    emoji: 🧭
    description: mss 提供可检查的生成、诊断、开发、验证和三方升级入口。
  - title: Thin Host
    emoji: 🧩
    description: 下游只拥有组合胶水和业务模块，安全与运行时核心由统一发行版维护。
---

# 从 v1.3.3 开始

当前文档只维护一条采用者路径：

1. 从 `v1.3.3` Release 安装 `mss` 与 `mss-mcp`；
2. 在空目录运行 `mss new app`；
3. 运行 `mss doctor --strict`、`mss setup` 与 `mss dev`；
4. 用 `mss verify --changed` 验证变更；
5. 用 `mss upgrade admin v1.3.3` 查看匹配发行版的升级计划。

[进入快速开始](/getting-started)

## 按任务阅读

| 目标 | 文档 |
| --- | --- |
| 创建第一个应用 | [快速开始](/getting-started) |
| 理解 Go/npm 依赖 | [包与导入边界](/getting-started/packages) |
| 安装和验证工具 | [工具说明](/getting-started/tooling) |
| 参考真实业务范本 | [mss-shop](/getting-started/mss-shop) |
| 配置和运行 Admin | [Admin 指南](/admin) |
| 编写规格与生成模块 | [Agent 开发](/agent) |
| 了解版本边界 | [v1.3.3 发布合同](/releases/v1-3-3) |

旧版本发布材料只作为不可变证据保留在[历史归档](/releases/archive)，不会参与当前入门。
