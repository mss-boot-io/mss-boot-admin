# 依赖安全与 PR 队列治理轮次记忆

- 日期：2026-06-09
- 范围：mss-boot、mss-boot-admin、mss-boot-admin-antd、mss-boot-docs
- 目标：用 GitHub-first 的方式处理安全依赖、PR 队列、CI 红灯和开源社区可审查性，同时避免前端 Cloudflare 高频发布。

## 已完成

- `mss-boot-admin`：
  - Dependabot PR `#387`（`go-git` 5.17.1 -> 5.19.1）已复核、approve、squash merge。
  - Dependabot PR `#386`（`go-billy`）因 `#387` 已覆盖而用英文维护说明关闭。
  - main 分支 CI、CodeQL、govulncheck、Swagger、OpenSSF Scorecard、Dependency Graph 均已确认通过。
- `mss-boot-admin-antd`：
  - Dependabot PR `#99`（`uuid` 9 -> 14）已复核、approve、squash merge。
  - PR `#98` 的 Node 22 / pnpm 9 package metadata 调整已合并进更完整的 `#100`，并以英文维护说明关闭 `#98`。
  - 新建 PR `#100`：`chore: converge frontend security dependencies`，处理 `axios`、`js-cookie`、`qs` 等传递依赖收敛，并更新 Node 22 / pnpm 9 基线。
  - `#100` 已通过 GitHub checks：CI build、CodeQL、Copilot Setup Steps、Docs Drift、PR Guard。
  - `#100` 未触发 Cloudflare 发布；Cloudflare workflow 保持 `workflow_dispatch`。
- `mss-boot-docs`：
  - 已补充 Security FAQ、登录排障文档与相关入口。
  - Issue `#5`、`#6` 已用英文说明关闭。
  - main 分支 CI、CodeQL、Scorecard、docs deploy 均通过。

## 当前仍需人工审核的 PR

- `mss-boot #382`：release workflow idempotent，checks 绿，等待 code owner review。
- `mss-boot #383`：阻止新增 golangci-lint debt，checks 绿，等待 code owner review。
- `mss-boot-admin-antd #97`：release workflow idempotent，checks 绿但可能需要与 main 同步后复核，等待 code owner review。
- `mss-boot-admin-antd #100`：前端安全依赖收敛，checks 绿，等待 code owner review。

## 暂缓与后续

- `mss-boot-admin-antd` 的 Vite / React Router 安全告警不能简单 override：
  - Vite 由 Umi bundler 链路固定在 4.x，Dependabot 要求更高 major。
  - React Router 同时受 DVA 老链路和 Umi 4 链路影响。
  - 后续应作为 Umi/DVA/Alita router + bundler 专项迁移处理，单独 RFC、单独验证。
- `mss-boot-docs` 仍有 Vite/dumi/React Router 等前端文档站依赖告警，优先级低于 admin-antd，但后续应安排一次 dumi 兼容治理。
- `mss-boot #352` 仍保留：`#383` 只阻止新增 lint debt，并未完成既有 lint backlog 清零。

## 验证口径

- 后端依赖 PR：以 GitHub CI、govulncheck、CodeQL、Scorecard、Dependency Graph 为准。
- 前端依赖 PR：本地先跑 `pnpm@9.15.9 install --frozen-lockfile`、`lint:js`、`tsc`、Jest、`build:local`、`build:beta`，再交给 GitHub CI/CodeQL/PR Guard。
- 前端 Cloudflare beta/prod 发布必须等本地和 GitHub 验证均完成，并由维护者明确触发。

## 社区沟通原则

- 对外部贡献者先回复、再处理，保持英文、中立、感谢、可复核。
- 对被 supersede 的 PR 说明“工作被更完整 PR 承接”，避免让贡献者感到被否定。
- 对 legacy/degraded 功能（如虚拟模型）保持低投入，必要时说明其降级背景。
