---
title: mss-boot Complete Admin Distribution
hero:
  title: mss-boot Admin
  description: 完整、可组合、可升级的 Agent 原生管理系统基础设施。使用一套 Go Admin、一个 React 19 + Ant Design 6 前端和 Thin Host 构建真实业务系统。
  actions:
    - text: 了解 Admin
      link: /admin
    - text: v1.3.2 发布合同
      link: /releases/v1-3-2
    - text: GitHub
      link: https://github.com/mss-boot-io/mss-boot-admin
features:
  - title: Complete Admin Distribution
    emoji: 🧭
    description: Framework、可导入 Admin Go Module、完整 Admin Web npm 包、CLI、机器契约和发布证据使用一个协调版本。
  - title: Thin Host
    emoji: 🪶
    description: 下游仓库只保存组合胶水与业务代码，不复制 Foundation 核心源码；前后端仍编译成一个逻辑应用。
  - title: Agent-native
    emoji: 🤖
    description: AGENTS.md、.mss 规格、mss CLI、Skills、确定性生成与变更感知验证让人和编码 Agent 使用同一事实源。
  - title: 权限与迁移闭环
    emoji: 🛡️
    description: 后端 RBAC 强制执行，业务模块同时生成迁移、菜单、API、权限、前端、测试和文档投影。
  - title: 单一前端 Runtime
    emoji: ⚛️
    description: React 19、Ant Design 6、Umi、Session、主题、国际化与业务页面进入同一路由树和同一个 dist。
  - title: 可验证发布
    emoji: ✅
    description: PR 到 main、精确提交冻结、外部消费者、浏览器、不可变标签、制品校验和发布后对账共同定义完成。
---

## 当前版本状态

| 项目 | 状态 |
| --- | --- |
| 当前稳定版 | `v1.2.3` |
| 唯一活动目标 | `v1.3.2` 稳定版候选，尚未公开发布 |
| 不完整稳定列车 | Framework `v1.3.0` 已公开；Admin 发布失败，其余稳定组件未发布 |
| 已完成预览 | `v1.3.0-rc.6` 完整列车；RC1–RC6 保持不可变 |
| 前端主线 | `web/antd-v6`；Ant Design 5 已退役 |
| 下游推荐形态 | `management-system` Thin Host |

`v1.3.2` 只有在准备变更通过 PR 合并、从新的精确 `main` 提交完成资格审查、按顺序公开四个
组件并完成发布后对账后，才会替代 `v1.2.3`。详见
[发布与升级](/releases)和 [v1.3.2 合同](/releases/v1-3-2)。

## 选择你的路径

### 使用完整 Admin

- [Admin 产品概览](/admin)
- [当前功能总览](/admin/current-capabilities)
- [本地启动](/admin/quickly)
- [生产与安全基线](/admin/security-baseline)

### 创建业务系统

- [完整 Admin Distribution 与 Thin Host](/architecture/complete-admin-distribution-and-thin-business-host)
- [Agent 开发入口](/agent)
- [Blueprint 与升级](/agent/blueprints-and-upgrades)
- [Supplier 黄金样例](/modules/supplier)

### 安装、升级与恢复

- [v1.3.2 安装、升级、兼容与回滚](/releases/v1-3-2)
- [Docker 部署](/admin/docker)
- [登录排障](/admin/login-troubleshooting)
- [API 与权限治理](/admin/governance-guide)

## 仓库组成

| 路径 | 作用 |
| --- | --- |
| `mss-boot/` | 领域中立的可复用 Go Framework |
| `admin/` | 可部署且可导入的完整 Admin 应用 |
| `web/antd-v6/` | 唯一正式前端与 `@mss-boot-io/admin-web` 来源 |
| `cmd/mss/`、`internal/mss/` | Agent CLI、生成、验证、评测和升级实现 |
| `.mss/` | 项目、能力、模块、Blueprint 和发布机器契约 |
| `docs/` | 本站源码与可独立发布的 Docs 组件 |

## 反馈与安全

一般问题请提交到
[`mss-boot-admin` Issues](https://github.com/mss-boot-io/mss-boot-admin/issues)。疑似漏洞不要在
公开 Issue 中披露，请先阅读 [Security Policy FAQ](/devops/security-policy-faq)。
