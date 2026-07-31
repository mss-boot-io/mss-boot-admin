# mss-boot-io 社区运营巡检记忆 - 2026-07-19

## 背景

- 自动化任务：`mss-boot-community-ops-triage`
- 记录时点：2026-07-19 09:35:11 Asia/Shanghai
- UTC 基线：2026-07-19T01:31:23.916Z
- 数据范围：只看 `mss-boot-io` 四个核心开源仓库：`mss-boot`、`mss-boot-admin`、`mss-boot-admin-antd`、`mss-boot-docs`
- 数据基线：以本轮巡检时 GitHub open PR、open issue、main workflow、Dependabot alerts、issue/review comments、Discussions 查询结果为准
- 约束：外部非 GitHub 平台只准备草稿，不在没有维护者当次确认的情况下发布；持久记忆必须写入 `mss-boot-docs/aigc`

## 巡检结论

- GitHub Discussions 无新社区回复；最近的社区 Discussions 仍是 2026-06-14 的 reviewer call 和双语社区更新。
- 2026-07-18T00:00:00Z 后没有新的外部人工 issue comment 或 PR review comment。
- GitHub AI 反馈：Copilot 在 `mss-boot-io/mss-boot-docs#60` 指出历史记忆文件里的短仓库引用会在跨平台转发时产生歧义；本轮已将 2026-07-18 记忆统一为完整 `mss-boot-io/repo#num` 风格。
- 新增 Dependabot PR `mss-boot-io/mss-boot#399`：`actions/setup-go` `6 -> 7`，只改 GitHub Actions workflow，checks 全绿，当前 `REVIEW_REQUIRED`。
- `mss-boot-io/mss-boot-admin#413` 仍是当前主要 release-health blocker 之一：main scheduled `govulncheck` 继续失败，并阻塞 `mss-boot-io/mss-boot-admin#412`、`mss-boot-io/mss-boot-admin#415` 等依赖治理 PR。
- 本轮未在非 GitHub 外部社区发布或回复内容。

## 主分支流水线

- `mss-boot`：最近 main 相关 run 全绿，包括 2026-07-19 Dependabot dynamic workflow run。
- `mss-boot-admin`：最近普通动态 run 绿；2026-07-12 scheduled `govulncheck` 失败，根因仍是 `GO-2026-5932` / `golang.org/x/crypto/openpgp`。
- `mss-boot-admin-antd`：最近 main 相关 run 全绿。
- `mss-boot-docs`：2026-07-18 记忆 PR 合并后的 CI、CodeQL、OpenSSF Scorecard、`(prod) Deploy docs` 全绿。

## PR 与安全队列

### mss-boot

- `mss-boot-io/mss-boot#399`：`actions/setup-go` `6 -> 7`
  - 只改 `.github/workflows/ci.yml` 与 `.github/workflows/copilot-setup-steps.yml`
  - Analyze、CodeQL、copilot-setup-steps、docs-drift、govulncheck、guard、lint、test 全绿
  - 当前仅因 `REVIEW_REQUIRED` 阻塞，可进入 review/merge 队列
- `mss-boot-io/mss-boot#397`：`golang.org/x/image` `0.38.0 -> 0.41.0`
  - 对应 open Dependabot alert：`GHSA-q675-qj96-32m9`，severity `high`
  - PR checks 全绿，仍处于 `REVIEW_REQUIRED`
  - 建议优先 review 并合并，合并后确认 Dependabot alert fixed
- `mss-boot-io/mss-boot#391`、`mss-boot-io/mss-boot#390`、`mss-boot-io/mss-boot#389`、`mss-boot-io/mss-boot#388`、`mss-boot-io/mss-boot#387` 仍为旧 Dependabot PR，状态落后，建议等 `mss-boot-io/mss-boot#397` 合并后再分组 rebase/验证。

### mss-boot-admin

- `mss-boot-io/mss-boot-admin#415`：`actions/setup-go` `6 -> 7`
  - build、CodeQL、Copilot setup、PR Guard、Docs Drift 均通过
  - 被 root `govulncheck` 失败阻塞，不是 workflow 升级本身导致
- `mss-boot-io/mss-boot-admin#412`：`github.com/hashicorp/consul/api` `1.34.3 -> 1.34.4`
  - 仍被同一 root `govulncheck` 问题阻塞
- `mss-boot-io/mss-boot-admin#413` 是当前主要跟踪 issue：优先判断 `pkg/gist.go` / GitHub helper 是否仍应留在 root module；如果只是维护工具，建议移除、替换或隔离，恢复应用 CI 的安全信号。
- 当前 open Dependabot alerts 为 0。

### mss-boot-admin-antd

- `mss-boot-io/mss-boot-admin-antd#124`：`actions/setup-node` `6 -> 7`
  - PR checks 全绿，仍处于 `REVIEW_REQUIRED`
  - 这是 workflow dependency update；考虑到前端 Cloudflare 发布次数和公开展示节奏，不建议在 heartbeat 中直接合并，应由维护者按前端发布节奏确认。
- open Dependabot alerts 仍有 4 个：
  - `path-to-regexp` high，可升级路径存在：`3.3.0`
  - `node-fetch` high，可升级路径存在：`2.6.7`
  - `mockjs` high，无 fixed version
  - `elliptic` low，无 fixed version
- 这些仍属于旧前端栈迁移/RFC 范围，不建议拆成无效碎片补丁。

### mss-boot-docs

- `mss-boot-io/mss-boot-docs#59`：`actions/setup-node` `6 -> 7`
  - PR checks 全绿，但当前 branch behind，需要 rebase/update 后再 review
- `mss-boot-io/mss-boot-docs#55`：npm minor/patch dev dependency group
  - PR checks 全绿，但当前 branch behind，需要 rebase/update 后再 review
- `mss-boot-io/mss-boot-docs#45`：社区作者 `12ain` 的工具链迁移影响清单仍为 `CHANGES_REQUESTED`，本轮无新回复。
- open Dependabot alert 仅剩 `elliptic` low，无 fixed version。

## 可转化任务

1. 文档规范：AIGC 记忆、外部社区草稿和巡检摘要统一使用 `mss-boot-io/repo#num` 格式引用仓库任务，避免跨平台转发时歧义。
2. 后端治理：先解决 `mss-boot-io/mss-boot-admin#413`，再恢复 `mss-boot-io/mss-boot-admin#412`、`mss-boot-io/mss-boot-admin#415` 以及后续 Dependabot PR 的正常 review/merge 节奏。
3. 安全治理：优先 review `mss-boot-io/mss-boot#397`，关闭明确 high security alert；随后处理 `mss-boot-io/mss-boot#399` 这类低风险 workflow 更新。
4. 前端治理：`mss-boot-io/mss-boot-admin-antd` 的旧依赖告警应进入旧前端栈迁移/RFC，不建议用孤立补丁消耗 Cloudflare 发布和验证节奏。

## 建议下一步

1. Review 并合并 `mss-boot-io/mss-boot#397`，修复明确 high security alert。
2. Review 并合并 `mss-boot-io/mss-boot#399`，保持 GitHub Actions runtime 更新。
3. 先处理 `mss-boot-io/mss-boot-admin#413`，解除 root `govulncheck` 红灯；这是当前最影响项目外观和依赖治理效率的问题。
4. 更新 `mss-boot-io/mss-boot-docs#59`、`mss-boot-io/mss-boot-docs#55` 的 base 后进入 review 队列。
5. `mss-boot-io/mss-boot-admin-antd#124` checks 绿，但应继续遵守前端发布节奏，避免无外部验证需求时合并 Cloudflare 相关 workflow 变更。

## 外部社区草稿

### 中文短帖

mss-boot 本轮继续做 GitHub-first 开源治理巡检：社区 Discussions 暂无新回复，GitHub AI review 已帮助发现并修复了文档中仓库引用简写带来的跨平台歧义问题。

当前最需要 review 的两个点是：`mss-boot-io/mss-boot#397` 修复 `golang.org/x/image` high security alert，且 checks 全绿；`mss-boot-io/mss-boot#399` 升级 `actions/setup-go` 到 7，属于低风险 workflow 更新。`mss-boot-io/mss-boot-admin#413` 仍在跟踪 `govulncheck` / `openpgp` 阻塞，它影响 `mss-boot-io/mss-boot-admin#412`、`mss-boot-io/mss-boot-admin#415` 和 main 定时安全检查。

欢迎熟悉 Go 安全依赖、govulncheck、GitHub Actions 和前端旧栈迁移的朋友参与 review。

GitHub: https://github.com/mss-boot-io

### English Short Post

mss-boot continued its GitHub-first open-source maintenance sweep. No new community replies are waiting in Discussions, and GitHub AI review helped catch ambiguous short repository references in the maintenance memory, which are now normalized for cross-platform sharing.

The most useful review items right now are `mss-boot-io/mss-boot#397`, which fixes a high `golang.org/x/image` security alert with green checks, and `mss-boot-io/mss-boot#399`, a low-risk `actions/setup-go` 7 workflow update. `mss-boot-io/mss-boot-admin#413` continues to track a repository-level `govulncheck` blocker around `golang.org/x/crypto/openpgp`, affecting `mss-boot-io/mss-boot-admin#412`, `mss-boot-io/mss-boot-admin#415`, and the scheduled main security scan.

Reviews around Go security dependencies, govulncheck, GitHub Actions, and legacy frontend migration are welcome.

GitHub: https://github.com/mss-boot-io
