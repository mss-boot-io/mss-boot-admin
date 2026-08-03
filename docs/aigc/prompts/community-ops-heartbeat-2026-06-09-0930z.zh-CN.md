# 社区运营心跳巡检记录

## 背景

- 巡检时间：2026-06-09 09:30 UTC
- 自动化：`mss-boot-community-ops-triage`
- 范围：`mss-boot`、`mss-boot-admin`、`mss-boot-admin-antd`、`mss-boot-docs`
- 原则：GitHub-first；外部非 GitHub 平台不在 action-time 维护者确认前公开发布或回复。

## GitHub 巡检结论

- PR：四个核心仓库均无 open PR。
- Issues：
  - `mss-boot`：无 open issue。
  - `mss-boot-admin`：仍保留 RFC/讨论队列，主要是 SQL migration/rollback、通知 provider、数据统计、手机号登录；没有新的外部回复需要互动。
  - `mss-boot-admin-antd`：`#101` 是前端 Umi/Vite/router 安全迁移 RFC。
  - `mss-boot-docs`：`#32` 是 docs dumi/Umi 安全迁移 RFC。
- Discussions：
  - 没有新的外部社区回复。
  - `mss-boot-admin#380` 仍是外部反馈回流的主入口。
  - `mss-boot-admin#99` 已补充虚拟模型降级说明，当前无需重复互动。
- Workflows：
  - `mss-boot`、`mss-boot-admin` 主分支 push workflows 绿。
  - `mss-boot-docs` 最新记忆提交的 CI / CodeQL / OpenSSF Scorecard / deploy docs 全绿。
  - `mss-boot-admin-antd` 主分支 push workflows 绿；失败主要来自 Dependabot Updates dynamic runs。

## 已执行治理动作

- 在 `mss-boot-admin-antd` 创建 `needs-rfc` 标签。
- 在 `mss-boot-docs` 创建 `needs-rfc` 标签。
- 更新 `mss-boot-admin-antd#101`：
  - 移除 `needs-triage`。
  - 添加 `needs-rfc`、`help wanted`、`queue/discussion`。
- 更新 `mss-boot-docs#32`：
  - 移除 `needs-triage`。
  - 添加 `needs-rfc`、`help wanted`、`queue/discussion`。

## Dependabot 状态

- `mss-boot`：open Dependabot alerts 为 0。
- `mss-boot-admin`：open Dependabot alerts 为 0。
- `mss-boot-admin-antd`：仍有 npm transitive alerts，集中在 legacy Umi/Vite/router/webpack/yeoman 链路；不要逐个盲目 override，继续通过 `#101` 做迁移 RFC。
- `mss-boot-docs`：仍有 dumi/Umi/Vite/react-router 链路 alerts，继续通过 `#32` 做迁移 RFC。

## 外部发布草稿

### 中文短帖草稿

```text
这轮继续整理 mss-boot-io 的开源治理：四个核心仓库的 PR 已清空，主分支 CI/CodeQL/Scorecard 保持绿色；剩余前端和文档站安全告警没有盲目硬升，而是拆到 Umi/Vite/router 与 dumi/Umi 迁移 RFC。

如果你熟悉 Go 后台、React 管理端、GitHub Actions、依赖安全迁移或开源文档治理，欢迎参与 review：
https://github.com/mss-boot-io

目前最需要反馈的是 quickstart 摩擦、RBAC/Casbin 说明、迁移/回滚设计、前端发布边界和 docs FAQ。
```

### English Short Draft

```text
Quick mss-boot-io community ops update:

The four core repos currently have no open PRs, and main-branch CI / CodeQL / OpenSSF Scorecard checks are green. The remaining frontend/docs security alerts are being tracked as migration RFCs instead of risky one-off overrides.

If you have experience with Go admin backends, React admin consoles, GitHub Actions, dependency security migrations, RBAC/Casbin docs, or release governance, review help is welcome:
https://github.com/mss-boot-io

The most useful feedback right now: quickstart friction, migration/rollback design, frontend release boundaries, and docs gaps for first-time contributors.
```

## 下一步建议

1. 维护者确认后，再公开发布已准备好的掘金、CSDN、知乎草稿。
2. 英文短帖可用于 X / LinkedIn / Dev.to digest，但不要重复发 Reddit；上一轮 Reddit 因账号年龄规则被 AutoModerator 移除。
3. 把 `admin-antd#101` 和 `docs#32` 拆成小的 good-first docs 子任务，例如“列出迁移影响面”“收集 smoke commands”“整理 rollback checklist”。
4. 下一轮心跳优先检查是否有外部回复、Dependabot alert 数是否下降、以及迁移 RFC 是否收到社区反馈。

