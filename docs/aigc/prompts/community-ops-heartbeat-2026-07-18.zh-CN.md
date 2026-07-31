# mss-boot-io 社区运营巡检记忆 - 2026-07-18

## 背景

- 自动化任务：`mss-boot-community-ops-triage`
- 记录时点：2026-07-18 09:30:14 Asia/Shanghai
- UTC 基线：2026-07-18T01:30:14.934Z
- 数据范围：只看 `mss-boot-io` 四个核心开源仓库：`mss-boot`、`mss-boot-admin`、`mss-boot-admin-antd`、`mss-boot-docs`
- 数据基线：以本轮巡检时 GitHub open PR、open issue、main workflow、Dependabot alerts、issue/review comments、Discussions 查询结果为准
- 约束：外部非 GitHub 平台只准备草稿，不在没有维护者当次确认的情况下发布；持久记忆必须写入 `mss-boot-docs/aigc`

## 巡检结论

- GitHub Discussions 无新社区回复；最近的社区 Discussions 仍是 2026-06-14 的 reviewer call 和双语社区更新。
- 2026-07-12 后新增 W28 Weekly Digest issue：`mss-boot-io/mss-boot#398`、`mss-boot-io/mss-boot-admin#414`、`mss-boot-io/mss-boot-admin-antd#123`、`mss-boot-io/mss-boot-docs#58`，均为 GitHub Actions 自动生成，无人工回复需要处理。
- 新增 Dependabot PR：
  - `mss-boot-io/mss-boot-admin#415`：`actions/setup-go` `6 -> 7`
  - `mss-boot-io/mss-boot-admin-antd#124`：`actions/setup-node` `6 -> 7`
  - `mss-boot-io/mss-boot-docs#59`：`actions/setup-node` `6 -> 7`
- `mss-boot-io/mss-boot-admin#415` 与 `mss-boot-io/mss-boot-admin#412` 同样被 root `govulncheck` 的 `GO-2026-5932` / `golang.org/x/crypto/openpgp` 阻塞，不是 `setup-go` 升级本身导致。
- `mss-boot-admin` main 的 scheduled `govulncheck` 仍为红灯；这已经是仓库级 release-health blocker。
- 本轮未在非 GitHub 外部社区发布或回复内容。

## 本轮已完成

- 复核四个核心仓库 open PR、open issue、main workflow、open Dependabot alerts、issue/review comments 和 Discussions。
- 确认无新的人工社区回复需要互动。
- 在 `mss-boot-io/mss-boot-admin#413` 补充 follow-up 评论：`mss-boot-io/mss-boot-admin#415` 也被同一 openpgp/govulncheck 问题阻塞，且 main scheduled `govulncheck` 也为红灯。
- 修正 `mss-boot-io/mss-boot-admin#413` 标签：移除误导性的 `area/frontend`，保留 backend/devops/security/release-blocker 方向。
- 记录 Copilot 对 `mss-boot-io/mss-boot-docs#57` 的 review 建议：后续 heartbeat 记忆必须包含精确记录时点、时区和数据基线。

## PR 与安全队列

### mss-boot

- `mss-boot-io/mss-boot#397`：`golang.org/x/image` `0.38.0 -> 0.41.0`
  - 对应 open Dependabot alert：`GHSA-q675-qj96-32m9`，severity `high`
  - PR checks 全绿，仍处于 `REVIEW_REQUIRED`
  - 建议优先 review 并合并，合并后确认 Dependabot alert fixed
- `mss-boot-io/mss-boot#391`、`mss-boot-io/mss-boot#390`、`mss-boot-io/mss-boot#389`、`mss-boot-io/mss-boot#388`、`mss-boot-io/mss-boot#387` 仍为旧 Dependabot PR，状态落后，建议等 `mss-boot-io/mss-boot#397` 合并后再分组 rebase/验证。

### mss-boot-admin

- `mss-boot-io/mss-boot-admin#415`：`actions/setup-go` `6 -> 7`
  - build、CodeQL、Copilot setup、PR Guard、Docs Drift 均通过
  - `govulncheck` 失败，失败 run：`https://github.com/mss-boot-io/mss-boot-admin/actions/runs/29477161242`
  - root cause：`GO-2026-5932`，trace 仍为 `pkg/gist.go` -> `github.init` -> `openpgp` init paths
- `mss-boot-io/mss-boot-admin#412`：`github.com/hashicorp/consul/api` `1.34.3 -> 1.34.4`
  - 仍被同一 root `govulncheck` 问题阻塞
- `mss-boot-io/mss-boot-admin#413` 是当前权威跟踪 issue：优先判断 `pkg/gist.go` / GitHub helper 是否仍应留在 root module；如果只是维护工具，建议移除、替换或隔离，恢复应用 CI 的安全信号。
- 当前 open Dependabot alerts 为 0。

### mss-boot-admin-antd

- `mss-boot-io/mss-boot-admin-antd#124`：`actions/setup-node` `6 -> 7`
  - PR checks 全绿，仍处于 `REVIEW_REQUIRED`
  - 这是 workflow dependency update；考虑到前端发布需要节制，不建议在 heartbeat 中直接合并，应由维护者按前端发布节奏确认。
- open Dependabot alerts 仍有 4 个：
  - `path-to-regexp` high，可升级路径存在：`3.3.0`
  - `node-fetch` high，可升级路径存在：`2.6.7`
  - `mockjs` high，无 fixed version
  - `elliptic` low，无 fixed version
- 这些仍属于旧前端栈迁移/RFC 范围，不建议拆成无效碎片补丁。

### mss-boot-docs

- `mss-boot-io/mss-boot-docs#59`：`actions/setup-node` `6 -> 7`
  - PR checks 全绿，仍处于 `REVIEW_REQUIRED`
- `mss-boot-io/mss-boot-docs#55`：npm minor/patch dev dependency group
  - PR checks 全绿，仍处于 `REVIEW_REQUIRED`
- `mss-boot-io/mss-boot-docs#45`：社区作者 `12ain` 的工具链迁移影响清单仍为 `CHANGES_REQUESTED`，本轮无新回复。
- open Dependabot alert 仅剩 `elliptic` low，无 fixed version。

## 建议下一步

1. 先处理 `mss-boot-io/mss-boot-admin#413`，解除 root `govulncheck` 红灯；这是当前最影响项目外观和依赖治理效率的问题。
2. Review 并合并 `mss-boot-io/mss-boot#397`，修复明确 high security alert。
3. 在 `mss-boot-io/mss-boot-admin#413` 解决后，重新跑 `mss-boot-io/mss-boot-admin#415` 和 `mss-boot-io/mss-boot-admin#412`，再评估是否合并。
4. `mss-boot-io/mss-boot-admin-antd#124`、`mss-boot-io/mss-boot-docs#59`、`mss-boot-io/mss-boot-docs#55` checks 均绿，可进入人工 review 队列；其中 `mss-boot-io/mss-boot-admin-antd#124` 不建议绕过前端发布节奏直接合并。

## 外部社区草稿

### 中文短帖

mss-boot 本轮继续做 GitHub-first 开源治理巡检：社区 Discussions 暂无新回复，四个核心仓库 W28 周报已生成。当前最需要 review 的两个点是：`mss-boot-io/mss-boot#397` 修复 `golang.org/x/image` high security alert，且 checks 全绿；`mss-boot-io/mss-boot-admin#413` 跟踪 `govulncheck` / `openpgp` 阻塞，它已经影响 `mss-boot-io/mss-boot-admin#412`、`mss-boot-io/mss-boot-admin#415` 和 main 定时安全检查。

欢迎熟悉 Go 安全依赖、govulncheck、GitHub Actions 和前端旧栈迁移的朋友参与 review。

GitHub: https://github.com/mss-boot-io

### English Short Post

mss-boot continued its GitHub-first open-source maintenance sweep. No new community replies are waiting in Discussions, and the W28 weekly digest issues are now available across the four core repositories.

The most useful review items right now are `mss-boot-io/mss-boot#397`, which fixes a high `golang.org/x/image` security alert with green checks, and `mss-boot-io/mss-boot-admin#413`, which tracks a repository-level `govulncheck` blocker around `golang.org/x/crypto/openpgp`. That blocker now affects `mss-boot-io/mss-boot-admin#412`, `mss-boot-io/mss-boot-admin#415`, and the scheduled main security scan.

Reviews around Go security dependencies, govulncheck, GitHub Actions, and legacy frontend migration are welcome.

GitHub: https://github.com/mss-boot-io
