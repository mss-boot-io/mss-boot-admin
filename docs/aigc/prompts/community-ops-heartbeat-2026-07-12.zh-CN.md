# mss-boot-io 社区运营巡检记忆 - 2026-07-12

## 背景

- 自动化任务：`mss-boot-community-ops-triage`
- 时间：2026-07-12
- 范围：只看 `mss-boot-io` 四个核心开源仓库：`mss-boot`、`mss-boot-admin`、`mss-boot-admin-antd`、`mss-boot-docs`
- 约束：外部非 GitHub 平台只准备草稿，不在没有维护者当次确认的情况下发布；持久记忆必须写入 `mss-boot-docs/aigc`

## 巡检结论

- GitHub Discussions 无新社区回复；最近仍是 2026-06-14 的社区更新和 reviewer call。
- 四个核心仓库最近更新的 issue 主要是 W27 Weekly Digest，均由 GitHub Actions 创建，无人工回复需要处理。
- 四个核心仓库 main 分支最近 workflow 整体为绿；`mss-boot-admin#412` 的 PR 检查中 `govulncheck` 为红灯。
- 新增可执行跟踪项：`mss-boot-admin#413`，用于处理 `govulncheck` 报出的 `GO-2026-5932` / `golang.org/x/crypto/openpgp` 阻塞。
- 本轮未在非 GitHub 外部社区发布或回复内容。

## PR 与安全队列

### mss-boot

- `#397`：`golang.org/x/image` `0.38.0 -> 0.41.0`
  - 对应 open Dependabot alert：`GHSA-q675-qj96-32m9`，severity `high`
  - PR checks 已全绿，仍处于 `REVIEW_REQUIRED`
  - 建议下一步优先 review 并合并，合并后确认 Dependabot alert fixed
- 旧 Dependabot PR `#391`、`#390`、`#389`、`#388`、`#387` 仍为 `BEHIND`，可在安全 PR 合并后分组 rebase/验证。

### mss-boot-admin

- `#412`：`github.com/hashicorp/consul/api` `1.34.3 -> 1.34.4`
  - build、CodeQL、PR Guard、Docs Drift 均通过
  - `govulncheck` 失败，失败 run：`https://github.com/mss-boot-io/mss-boot-admin/actions/runs/28999033648`
  - 失败原因：`GO-2026-5932`，`golang.org/x/crypto/openpgp` 无 fixed version，trace 经过 `pkg/gist.go` 到 `github.init`
  - 已创建 `mss-boot-admin#413` 跟踪：优先判断 gist/GitHub helper 是否仍需要在 root module 中保留；如果只是维护工具，倾向移除、替换或隔离出应用 CI 的 govulncheck 面。
- `#406`、`#407`、`#408`、`#409`、`#410` 仍为 Dependabot PR，需等 `#413` 决策后再批量恢复红灯队列。
- 当前 open Dependabot alerts 为 0。

### mss-boot-admin-antd

- 当前无 open PR。
- open Dependabot alerts 仍有 4 个：
  - `path-to-regexp` high，可升级路径存在：`3.3.0`
  - `node-fetch` high，可升级路径存在：`2.6.7`
  - `mockjs` high，无 fixed version
  - `elliptic` low，无 fixed version
- 这些仍属于旧前端栈迁移/RFC 范围，不建议做碎片化补丁；继续以安全迁移任务统一推进。

### mss-boot-docs

- `#55`：npm minor/patch dev dependency group，checks 已全绿，仍处于 `REVIEW_REQUIRED`
- `#45`：社区作者 `12ain` 的工具链迁移影响清单仍为 `CHANGES_REQUESTED` 且 `BEHIND`，本轮无新回复。
- open Dependabot alert 仅剩 `elliptic` low，无 fixed version。

## 本轮已完成

- 复核四个核心仓库 open PR、open issue、Discussions、main workflow 和 open Dependabot alerts。
- 确认无新的人工社区回复需要互动。
- 创建 `mss-boot-admin#413`：`security: resolve govulncheck GO-2026-5932 openpgp blocker`。
- 将本轮巡检记忆写入 `mss-boot-docs/aigc/prompts/community-ops-heartbeat-2026-07-12.zh-CN.md`。

## 建议下一步

1. 优先处理并合并 `mss-boot#397`，这是明确 high security alert 且 checks 全绿的 PR。
2. 处理 `mss-boot-admin#413`，解除 root `govulncheck` 红灯后再推进 `mss-boot-admin` 的 Dependabot PR 队列。
3. Review `mss-boot-docs#55`，若无兼容性问题可 squash merge，并确认 docs main CI / deploy。
4. 继续保留 `admin-antd` 的旧前端栈安全迁移为较大任务，不把 `mockjs`、`elliptic` 这种无 fixed version 的 alert 拆成无效小补丁。

## 外部社区草稿

### 中文短帖

mss-boot 本轮继续做开源治理巡检：四个核心仓库 main 分支流水线整体健康，社区 Discussions 暂无新回复。当前最需要 reviewer 的两个点是：`mss-boot#397` 修复 `golang.org/x/image` high security alert，且 PR checks 已全绿；`mss-boot-admin#413` 用于处理新出现的 `govulncheck` / `openpgp` 阻塞，让后续依赖升级恢复干净的 CI 信号。

欢迎对 Go 安全依赖、govulncheck、前端旧栈迁移有经验的朋友参与 review。

GitHub: https://github.com/mss-boot-io

### English Short Post

mss-boot continued its GitHub-first open-source maintenance sweep. The main branches of the four core repositories are generally green, and there are no new community replies waiting for a response.

Two reviewer-friendly items need attention now: `mss-boot#397` addresses a high `golang.org/x/image` security alert and already has green PR checks; `mss-boot-admin#413` tracks a new `govulncheck` blocker around `golang.org/x/crypto/openpgp`, so dependency PRs can return to a clean CI signal.

Reviews around Go security dependencies, govulncheck, and frontend legacy-stack migration are welcome.

GitHub: https://github.com/mss-boot-io
