# Community Governance Sweep

日期：2026-06-06

## 范围

本次使用 GitHub 当前数据对 `mss-boot-io` 的 open PR 与 open issue 做了一轮
开源社区治理巡检。重点覆盖核心仓库：

- `.github`
- `mss-boot`
- `mss-boot-admin`
- `mss-boot-admin-antd`
- `mss-boot-docs`
- `mss-boot-monorepo`

后续扩展扫描覆盖了非核心但公开可见的仓库：

- `openpanel`
- `meminfra`
- `use-cmd-action`
- `mss-boot-frontend`

## PR 治理结果

- `mss-boot`、`mss-boot-admin`、`mss-boot-admin-antd`、`mss-boot-docs`
  巡检时没有遗留 open PR。
- 合并 `.github` PR `#5`：补齐组织级 Copilot 指令、Dependabot、CodeQL、
  OpenSSF Scorecard 和 GitHub AI handoff 记忆。
- 合并 `.github` Dependabot PR `#6`：将 `actions/upload-artifact` 从 `v4`
  升级到 `v7`。
- `.github` 最新 main 检查已通过：GitHub Actions Mirror、CodeQL、
  OpenSSF Scorecard。
- 合并 `meminfra` Dependabot PR `#2`、`#3`、`#4`、`#5`：分别升级
  `actions/setup-go`、`actions/upload-artifact`、`actions/checkout`、
  `github/codeql-action`。PR 检查与合并后的 main `CI`、`CodeQL` 均已通过。
- `openpanel` Dependabot PR `#1` 到 `#10` 暂不合并。它们共同失败在
  `Spell Check with Typos`，日志显示为仓库既有拼写基线问题，而不是单个依赖
  PR 的变更导致。已新增 `blocked` 标签，并在每个 PR 留言指向 `openpanel#11`。
- 关闭 `use-cmd-action` 的历史 Dependabot PR `#9`、`#15`、`#22`、`#59`、
  `#81`、`#83`、`#87`、`#89`、`#90`。这些 PR 创建于 2022-2023 年，当前
  已 `DIRTY`，且默认分支依赖状态已超过这些旧目标版本，按 superseded/stale
  处理。
- 关闭 `mss-boot-frontend` PR `#15`。这是 2022 年仅修改 package metadata
  的遗留 PR，无检查、无后续上下文，按 stale/unverified 处理；如仍需要应基于
  当前分支重新创建。

## Issue 治理结果

- `mss-boot`：
  - `#352` 保持打开，因为 workflow 里仍有 golangci-lint
    `continue-on-error: true`；已移除 `needs-triage`，补充当前状态评论，并
    标记为 `help wanted`。
  - `#348`、`#349`、`#350` 保持为高质量 `good first issue`。
- `mss-boot-admin`：
  - `#139` 由报告者确认已解决，已按 completed 关闭，并保留端口/租户域名
    不一致的排查说明。
  - `#126`、`#89` 补充维护者 triage 评论，明确功能范围、PR 拆分和验证方向。
  - `#105`、`#75` 保留打开，移除 `needs-triage`，保留 `needs-info`，要求基于
    当前版本补充可复现信息或拆分问题。
  - `#111`、`#87`、`#73`、`#55`、`#53` 移除 `needs-triage`，归入功能 backlog。
  - `#344`、`#345`、`#346` 保持为高质量 `good first issue`。
- `mss-boot-admin-antd`：
  - `#71`、`#72`、`#73` 保持为高质量 `good first issue`。
- `mss-boot-docs`：
  - `#5`、`#6` 保持为高质量 `good first issue`。
- `mss-boot-monorepo`：
  - `#30` 是 2022 年旧 issue，仅有截图且缺少当前版本复现信息；已按
    `not planned` 关闭，并说明如当前 main 仍可复现应重新打开或新建 issue。
- `openpanel`：
  - 新建 `#11`：`ci: restore PR gate baseline before merging dependency updates`。
    该 issue 记录 Typos PR gate 的仓库级基线问题，建议升级
    `.github/workflows/tyops-check.yml`、固定 `crate-ci/typos` 稳定版本、修复或
    精准 allowlist 既有拼写项，并补齐前端/Go 依赖 PR 的真实验证门禁。

## 治理原则

- 不激进关闭外部用户 issue；除报告者确认已解决或明显过期不可行动外，优先
  使用 `needs-info`、公开评论和标签分流。
- 外部贡献者或用户反馈优先使用中文语境回复；Dependabot 与组织级 PR 使用
  简洁英文/技术语境。
- `good first issue` 应保留清晰的背景、范围、非目标、验证命令和验收标准。
- 功能 backlog 不应长期停留在 `needs-triage`，已能归类的 issue 应移除
  `needs-triage` 并保留 `type/feature`、`area/*`、`help wanted` 等可行动标签。
- 组织级自动化优先使用 GitHub 原生能力；涉及外部账号、付费设置、Cloudflare
  发布和安全策略的决策仍由 maintainer 最终确认。

## 当前公开状态

- 核心仓库当前没有遗留 open PR。
- 组织级 open PR 主要剩余 `openpanel` 的 blocked Dependabot PR；这些 PR 有
  公开阻塞说明，等待 `openpanel#11` 修复后再按风险分批评估。
- 当前 open issue 主要由两类组成：
  - 可贡献的 `good first issue`；
  - 已分流的功能 backlog 或等待复现信息的 bug。
