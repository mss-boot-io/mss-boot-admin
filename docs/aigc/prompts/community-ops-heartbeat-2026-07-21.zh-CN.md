# mss-boot-io 社区运营巡检记忆 - 2026-07-21

## 背景

- 自动化任务：`mss-boot-community-ops-triage`
- 记录时点：2026-07-21 09:33:41 Asia/Shanghai
- UTC 基线：2026-07-21T01:30:21.422Z
- 数据范围：只看 `mss-boot-io` 四个核心开源仓库：`mss-boot`、`mss-boot-admin`、`mss-boot-admin-antd`、`mss-boot-docs`
- 数据基线：以本轮巡检时 GitHub open PR、open issue、main workflow、Dependabot alerts、issue/review comments、Discussions 查询结果为准
- 约束：外部非 GitHub 平台只准备草稿，不在没有维护者当次确认的情况下发布；持久记忆必须写入 `mss-boot-docs/aigc`

## 巡检结论

- 本轮查询暂未观察到 GitHub Discussions 新社区回复；最近的社区 Discussions 仍是 2026-06-14 的 reviewer call 和双语社区更新。
- 2026-07-19T01:31:23Z 后，本轮查询暂未观察到新的外部人工 issue comment 或外部人工 PR review comment。
- W29 Weekly Digest issue 已生成：`mss-boot-io/mss-boot#400`、`mss-boot-io/mss-boot-admin#416`、`mss-boot-io/mss-boot-admin-antd#125`、`mss-boot-io/mss-boot-docs#62`，均为 GitHub Actions 自动生成，本轮暂未观察到需要人工回复的内容。
- GitHub AI 反馈：Copilot 在 `mss-boot-io/mss-boot-docs#61` 指出部分维护状态表述偏绝对化。已在 2026-07-19 记忆中改为更中立的 `主要 release-health blocker 之一` 与 `主要跟踪 issue`。
- `mss-boot-io/mss-boot-admin` 的 2026-07-19 scheduled `govulncheck` 仍失败；这延续了 `mss-boot-io/mss-boot-admin#413` 跟踪的问题，并继续影响相关 Dependabot PR 的合并信号。
- 本轮未在非 GitHub 外部社区发布或回复内容。

## 主分支流水线

- `mss-boot`：2026-07-19 scheduled Weekly Digest、OpenSSF Scorecard、govulncheck、CodeQL Advanced 全绿。
- `mss-boot-admin`：2026-07-19 scheduled Weekly Digest、CodeQL、OpenSSF Scorecard 全绿；scheduled `govulncheck` 失败，仍需处理 `GO-2026-5932` / `golang.org/x/crypto/openpgp`。
- `mss-boot-admin-antd`：2026-07-19 scheduled Weekly Digest、CodeQL、OpenSSF Scorecard 全绿。
- `mss-boot-docs`：2026-07-19 scheduled Weekly Digest、CodeQL、OpenSSF Scorecard 全绿；2026-07-19 记忆 PR 合并后的 CI、CodeQL、OpenSSF Scorecard、`(prod) Deploy docs` 也全绿。

## PR 与安全队列

### mss-boot

- `mss-boot-io/mss-boot#397`：`golang.org/x/image` `0.38.0 -> 0.41.0`
  - 对应 open Dependabot alert：`GHSA-q675-qj96-32m9`，severity `high`
  - 当前仍处于 `REVIEW_REQUIRED`
  - 建议优先 review 并合并，合并后确认 Dependabot alert fixed
- `mss-boot-io/mss-boot#399`：`actions/setup-go` `6 -> 7`
  - workflow-only 更新，当前仍处于 `REVIEW_REQUIRED`
  - 建议在 `mss-boot-io/mss-boot#397` 之后 review/merge，保持 GitHub Actions runtime 更新
- `mss-boot-io/mss-boot#391`、`mss-boot-io/mss-boot#390`、`mss-boot-io/mss-boot#389`、`mss-boot-io/mss-boot#388`、`mss-boot-io/mss-boot#387` 仍为旧 Dependabot PR，状态落后。

### mss-boot-admin

- `mss-boot-io/mss-boot-admin#413` 仍是主要跟踪 issue：root `govulncheck` 失败来自 `GO-2026-5932` / `golang.org/x/crypto/openpgp`。
- `mss-boot-io/mss-boot-admin#415` 和 `mss-boot-io/mss-boot-admin#412` 仍受同一 root `govulncheck` 信号影响；不建议把失败归因到对应依赖升级本身。
- 当前 open Dependabot alerts 为 0。

### mss-boot-admin-antd

- `mss-boot-io/mss-boot-admin-antd#124`：`actions/setup-node` `6 -> 7`
  - workflow dependency update，仍处于 `REVIEW_REQUIRED`
  - 考虑前端 Cloudflare 发布次数和公开展示节奏，不建议在 heartbeat 中直接合并
- open Dependabot alerts 仍有 4 个：
  - `path-to-regexp` high，可升级路径存在：`3.3.0`
  - `node-fetch` high，可升级路径存在：`2.6.7`
  - `mockjs` high，无 fixed version
  - `elliptic` low，无 fixed version
- 这些仍属于旧前端栈迁移/RFC 范围，不建议拆成无效碎片补丁。

### mss-boot-docs

- `mss-boot-io/mss-boot-docs#59`：`actions/setup-node` `6 -> 7`，仍处于 `REVIEW_REQUIRED`。
- `mss-boot-io/mss-boot-docs#55`：npm minor/patch dev dependency group，仍处于 `REVIEW_REQUIRED`。
- `mss-boot-io/mss-boot-docs#45`：社区作者 `12ain` 的工具链迁移影响清单仍为 `CHANGES_REQUESTED`，本轮暂未观察到新回复。
- open Dependabot alert 仅剩 `elliptic` low，无 fixed version。

## 可转化任务

1. 文档规范：开源巡检记忆中避免绝对化措辞；用 `主要`、`重点`、`当前观察到` 等可讨论表述。
2. PR 治理：优先 review `mss-boot-io/mss-boot#397` 和 `mss-boot-io/mss-boot#399`，一个关闭明确 high alert，一个保持 GitHub Actions runtime 更新。
3. 后端治理：继续处理 `mss-boot-io/mss-boot-admin#413`，恢复 `mss-boot-io/mss-boot-admin` scheduled `govulncheck` 绿灯。
4. Issue 卫生：W29 Weekly Digest issue 均为自动生成，可后续按周归档或总结，但本轮没有外部社区反馈需要转成新 issue 或 good-first-issue。

## 建议下一步

1. Review 并合并 `mss-boot-io/mss-boot#397`，修复明确 high security alert。
2. Review 并合并 `mss-boot-io/mss-boot#399`，保持 GitHub Actions runtime 更新。
3. 处理 `mss-boot-io/mss-boot-admin#413`，解除 root `govulncheck` 红灯。
4. 更新或复核 `mss-boot-io/mss-boot-docs#59`、`mss-boot-io/mss-boot-docs#55` 后进入 review 队列。

## 外部社区草稿

### 中文短帖

mss-boot 本轮继续做 GitHub-first 开源治理巡检：Discussions 暂未观察到新社区回复，四个核心仓库 W29 周报已生成，主分支定时任务总体健康。当前需要优先 review 的是 `mss-boot-io/mss-boot#397`，它对应 `golang.org/x/image` high security alert；其次是 `mss-boot-io/mss-boot#399`，用于更新 GitHub Actions 的 `setup-go` runtime。

`mss-boot-io/mss-boot-admin#413` 仍在跟踪 `govulncheck` / `openpgp` 问题。欢迎熟悉 Go 安全依赖、govulncheck、GitHub Actions 和前端旧栈迁移的朋友参与 review。

GitHub: https://github.com/mss-boot-io

### English Short Post

mss-boot continued its GitHub-first open-source maintenance sweep. This check did not observe new community replies in Discussions. W29 weekly digest issues are now available across the four core repositories, and the scheduled main-branch workflows are mostly healthy.

A high-priority review item is `mss-boot-io/mss-boot#397`, which addresses a high `golang.org/x/image` security alert. `mss-boot-io/mss-boot#399` is also ready for review as a low-risk GitHub Actions `setup-go` runtime update.

`mss-boot-io/mss-boot-admin#413` continues to track the `govulncheck` / `openpgp` issue. Reviews around Go security dependencies, govulncheck, GitHub Actions, and legacy frontend migration are welcome.

GitHub: https://github.com/mss-boot-io
