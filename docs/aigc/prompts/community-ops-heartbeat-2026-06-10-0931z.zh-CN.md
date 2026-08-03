# 社区运营巡检记忆 2026-06-10 09:31Z

## 巡检范围

- `mss-boot-io/mss-boot`
- `mss-boot-io/mss-boot-admin`
- `mss-boot-io/mss-boot-admin-antd`
- `mss-boot-io/mss-boot-docs`
- `mss-boot-io/mss-boot-community-bot`

## GitHub 状态

- 五个仓库当前均无 open PR，直到本轮新建 `mss-boot-admin-antd#111`。
- `mss-boot` main 最新 CI、CodeQL、govulncheck、OpenSSF Scorecard 均为 success。
- `mss-boot-admin` main 最新 CI、CodeQL、govulncheck、Swagger、OpenSSF Scorecard 均为 success。
- `mss-boot-admin-antd` main CI 为 success，但 Dependabot Updates 对 `form-data` 与 `min-document` 的历史安全更新任务仍显示失败；默认分支仍有多条 Dependabot vulnerability 提示。
- `mss-boot-docs` main 最新 CI、CodeQL、OpenSSF Scorecard、docs deploy 均为 success。
- `mss-boot-community-bot` main 最新 CI、govulncheck、CodeQL、OpenSSF Scorecard、Dependabot Updates 均为 success。
- 组织 `.github` 仓库的 discussions GraphQL 查询返回空列表，当前未发现需要回复的新 GitHub Discussions。

## 本轮直接处理

- 在 `mss-boot-admin-antd` 新建分支 `codex/fix-admin-antd-router-vite-alerts-20260610`。
- 新建 PR `mss-boot-admin-antd#111`：`chore: pin router and vite security deps`。
- PR 内容：
  - 将 React Router 6.x 传递依赖固定到 `6.30.4`。
  - 通过 `@vitejs/plugin-react@4.7.0` 将 Umi Vite 链路从 `vite@4.5.2` 提升到 `vite@6.4.3`。
  - 保留 legacy `react-router@4.3.1` / DVA 链路不变，避免破坏老功能。
- 在 `mss-boot-admin-antd#101` 追加英文进展说明，说明 `#111` 的边界、验证结果和剩余 audit debt。
- `#111` 本地验证通过：
  - `pnpm install --frozen-lockfile`
  - `pnpm lint:js`
  - `pnpm tsc`
  - `pnpm test -- --runInBand`
  - `pnpm build:local`
  - `pnpm build:alpha`
  - `pnpm why vite react-router react-router-dom @vitejs/plugin-react`
- `#111` GitHub checks 已通过：
  - CI build
  - CodeQL
  - Docs Drift
  - PR Guard

## 当前阻塞

- `mss-boot-admin-antd#111` 仍处于 `REVIEW_REQUIRED`。
- PR 作者为 `lwnmengjing`，当前凭据也是该维护者账号；不应绕过 code owner review，也不能把自审作为有效开源审核。
- 需要维护者在 GitHub 上完成 review 后再 squash merge。

## 后续安全治理建议

- `mss-boot-admin-antd` 仍有旧前端栈 audit debt，建议拆分专项，不要混入 `#111`：
  - `mockjs` 无 patched version，需要评估移除 mock 或替代方案。
  - `braft-editor` / `draft-js` / `node-fetch` 链路较旧，建议评估编辑器替换或隔离。
  - `path-to-regexp` 来自 Ant Design Pro / Pro Layout 链路，直接 override 有兼容风险。
  - `immer`、`braces`、`micromatch`、`ws`、`@babel/*` 等可后续分批验证 overrides。
- 前端 Cloudflare beta/prod 发布仍遵守低频策略：只有本地和 GitHub 验证均完成，并由维护者明确触发时才发布。
