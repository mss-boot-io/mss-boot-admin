# 社区巡检与 PR/Issue 治理记录

- 日期：2026-06-09
- 范围：mss-boot-io 四个核心开源仓库
- 执行模式：leader 协调多 agent，只读巡检后由 leader 统一执行 GitHub 互动、PR 修复、CI 验证与记忆落盘。

## PR 状态

- mss-boot #382：release workflow 幂等化 PR。
  - 修复 Copilot review：为 `gh release` 设置 `GH_REPO=${{ github.repository }}`。
  - review thread 已回复并 resolve。
  - PR checks 全部成功。
  - 当前仍需 code owner review，不绕过开源 review 门禁。
- mss-boot #383：阻断新增 golangci-lint 问题 PR。
  - 关联 #352。
  - 首次移除 `continue-on-error: true` 后暴露全量 lint 仍有 131 个历史问题。
  - PR 已调整为 pull_request 场景使用 `only-new-issues: true` 阻断新增 lint 债，main push 继续保留 baseline advisory。
  - 本地已通过 `git diff --check` 与 `go test ./...`。
  - 当前等待 GitHub PR CI 与 code owner review。
- mss-boot-admin-antd #97：release workflow 幂等化 PR。
  - 修复 Copilot review：移除 `gh release create` 中与 `--notes-file` 互斥的 `--generate-notes`，并同步 AIGC 说明。
  - review threads 已回复并 resolve。
  - PR checks 全部成功。
  - 当前仍需 code owner review，不绕过开源 review 门禁。
- mss-boot-admin-antd #98：package metadata / Node 22 基线 PR。
  - 新增 `packageManager: pnpm@9.15.9` 与 Node/pnpm engine。
  - 修复 CI 暴露的问题：`pnpm/action-setup@v6` 不能同时声明 workflow `version` 与 package `packageManager`，因此 CI、Cloudflare、Release、Copilot setup 均删除显式 `version: 9`。
  - PR Guard、CI、CodeQL、Copilot setup、Docs Drift 均通过。
  - 当前仍需 code owner review。

## Issue / Discussion 互动

- mss-boot-admin Discussion #99：
  - 已把 2026 更新设为 answer。
  - 当前推荐路径：新增数据表优先使用 Go model + migration；virtual model 只作为 legacy/degraded 既有安装路径。
- mss-boot-admin Issue #53：
  - 已回复手机号登录需要先补 RFC。
  - 重点边界：provider contract、短信限流、失败锁定、审计日志、租户级开关、测试覆盖。
  - 已加 `needs-rfc`、`queue/discussion`。
- mss-boot-admin Issue #55：
  - 已回复通用数据统计需要拆分范围后再实现。
  - 重点边界：统计对象、数据来源、展示方式、多租户权限、聚合与空数据测试。
  - 已加 `needs-rfc`、`queue/discussion`。
- mss-boot Issue #381：
  - Weekly Digest 已复核，release 红灯由 #382 跟踪，已关闭为 completed。
- mss-boot-admin-antd Issue #96：
  - Weekly Digest 失败 workflow 数为 0，外部 PR 已处理，已关闭为 completed。

## CI 状态

- mss-boot main：最新 CI、CodeQL、govulncheck、OpenSSF Scorecard、mirror 均 success。
- mss-boot-admin main：最新 CI、CodeQL、govulncheck、Swagger、OpenSSF Scorecard、mirror 均 success。
- mss-boot-admin-antd main：最新 CI、CodeQL、OpenSSF Scorecard、mirror 均 success。
- mss-boot-docs main：最新 CI、CodeQL、OpenSSF Scorecard、docs deploy 均 success。

## 继续推进原则

- 对外 PR 可以在验证合理、方向一致、checks 通过后 approve 并 squash merge。
- 维护者/AI 自己创建的治理 PR 保持 code owner review 语义；除非用户明确要求 admin bypass，否则不绕过 review。
- 历史 issue 若仍有产品价值，先转 RFC/Discussion 队列；如果范围不清，不直接开发大功能。
- 自动周报 issue 用于 triage，不长期堆积；能分流到 PR/RFC 的及时关闭。
