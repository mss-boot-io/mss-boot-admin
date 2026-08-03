# mss-boot-io 社区运营与工具链安全巡检记忆 - 2026-06-27

## 背景

- 自动化任务：`mss-boot-community-ops-triage`
- 时间：2026-06-27
- 范围：只看 `mss-boot-io` 4 个核心开源仓库：`mss-boot`、`mss-boot-admin`、`mss-boot-admin-antd`、`mss-boot-docs`
- 约束：外部非 GitHub 平台只准备草稿，不在没有维护者当次确认的情况下发布；持久记忆必须写入 `mss-boot-docs/aigc`

## GitHub 巡检结论

- 6 月 21 日后没有新的人工 issue/PR 回复需要社区互动，只有 W25 Weekly Digest issue 更新。
- `mss-boot-docs#45` 仍为 `CHANGES_REQUESTED`，作者 `12ain` 暂无新回复。当前要求是把文档从 `Final` 调整为快照/基线表述，并补充 `hono`、`piscina` 新工具链告警，以及补齐 PR Guard 需要的 Tests/Docs/Security/Release impact。
- 4 个仓库仍有 Dependabot PR 处于 open，主要是 `actions/checkout` 和 Go module patch/minor 更新，当前大多是 review required 或 branch protection blocked，需要后续按依赖升级流程逐个合并。
- 主线定时任务总体正常；近期失败主要来自 Dependabot dynamic update，已有后续 PR/修复分支覆盖一部分。

## 本轮已落地修复

### mss-boot-docs

分支：`codex/fix-toolchain-hono-piscina-20260621`

本地修改：

- `package.json` 增加/收敛 pnpm overrides：
  - `@babel/core` -> `7.29.7`
  - `hono` -> `4.12.25`
  - `js-yaml` -> `4.2.0`
  - `piscina` -> `4.9.3`
- `pnpm-lock.yaml` 重新解析到安全版本。

验证：

- `CI=true corepack pnpm@9.15.9 install --no-frozen-lockfile`
- `CI=true corepack pnpm@9.15.9 install --frozen-lockfile --offline`
- `CI=true corepack pnpm@9.15.9 build`
- `corepack pnpm@9.15.9 audit --audit-level=moderate`
- `git diff --check`

结果：

- docs build 通过。
- audit 从 `hono`/`piscina`/`js-yaml` 等可修复项收敛到 only-low，剩余为 `elliptic` low，无官方 patched version。

### mss-boot-admin-antd

分支：`codex/fix-toolchain-hono-piscina-20260621`

本地修改：

- `package.json` 增加/收敛 pnpm overrides：
  - `@babel/core` -> `7.29.7`
  - `form-data` -> `4.0.6`
  - `hono` -> `4.12.25`
  - `js-yaml` -> `4.2.0`
  - `piscina` -> `4.9.3`
- `pnpm-lock.yaml` 重新解析到安全版本。

验证：

- `CI=true corepack pnpm@9.15.9 install --no-frozen-lockfile`
- `CI=true corepack pnpm@9.15.9 install --frozen-lockfile --offline`
- `corepack pnpm@9.15.9 lint:js`
- `corepack pnpm@9.15.9 tsc`
- `corepack pnpm@9.15.9 build:local`
- `corepack pnpm@9.15.9 audit --audit-level=moderate`
- `git diff --check`

结果：

- lint、tsc、local build 全部通过。
- audit 从 25 个 moderate+ 告警降到 20 个，已移除本轮可低风险修复的 `hono`、`piscina`、`form-data`、`@babel/core`、`js-yaml` 相关项。
- 剩余告警主要来自旧前端栈和运行时包：`immer`、`mockjs`、`node-fetch`、`path-to-regexp`、`braces`、`micromatch`、`ws`、`@babel/runtime`、`ajv` 等。不要在同一个小 PR 中强行大范围 override，后续应通过 `mss-boot-admin-antd#101` 的前端工具链迁移 RFC 拆分处理。

## GitHub 合并结果

### 已合并 PR

- `mss-boot-docs#49`：`chore(deps): harden docs toolchain overrides`
  - 合并提交：`fd7a176ad7e7801816417af9320ce913290864ec`
  - PR CI、CodeQL、PR Guard、docs drift 通过；main 上 CI、CodeQL、OpenSSF Scorecard、docs deploy 通过。
  - 已在 `mss-boot-docs#32` 补充维护说明：`https://github.com/mss-boot-io/mss-boot-docs/issues/32#issuecomment-4814600482`
- `mss-boot-admin-antd#119`：`chore(deps): harden frontend toolchain overrides`
  - 合并提交：`0394b49b5c6a086754b640ff2dc826f7c61bb4e4`
  - PR CI、CodeQL、PR Guard、docs drift 通过；main 上 CI、CodeQL、OpenSSF Scorecard、GitHub Actions Mirror 通过。
- `mss-boot-admin-antd#120`：`chore(deps): resolve frontend security overrides`
  - 合并提交：`cf5a8a405c6ff888438ac169328395bffe159b54`
  - 本轮新增安全覆盖：`@babel/runtime`、`braces`、`micromatch`、`send`、`ws`、`ajv`、`cross-spawn`、`immer`
  - 移除未直接使用的 `mockjs` dev 依赖；剩余 `mockjs` 仅来自 Umi OpenAPI 工具链。
  - 本地验证通过：冻结/离线安装、`lint:js`、`tsc`、`test -- --runInBand`、`build:local`、`git diff --check`。
  - PR CI、CodeQL、PR Guard、docs drift 通过；main 上 CI、CodeQL、OpenSSF Scorecard、GitHub Actions Mirror 通过。
  - 已在 `mss-boot-admin-antd#101` 补充维护说明：`https://github.com/mss-boot-io/mss-boot-admin-antd/issues/101#issuecomment-4814654584`

### admin-antd 安全状态

- `#119` 后 audit 从 25 个 moderate+ 收敛到 20 个。
- `#120` 后本地 `pnpm audit --audit-level=moderate` 剩余 7 个漏洞：3 low、4 high。
- 剩余 high 项不应继续用小范围 override 快速合并：
  - `node-fetch` 来自旧 `braft-editor` / `draft-js` / `isomorphic-fetch` 链路，需要富文本编辑器替换或兼容验证。
  - `path-to-regexp` 来自 Umi / Pro Layout 路由链路，需要页面路由和 beta smoke 验证。
  - `mockjs` 来自 Umi OpenAPI 工具链，且 upstream 无 patched version，需要工具链替换策略。
- `elliptic` 等 low/no-patched-version 项继续作为供应链卫生债跟踪，不做不可验证的临时 patch。

## 后续建议

- 下一轮 admin-antd 安全治理应从 RFC 进入实现：
  - 富文本编辑器链路替换或隔离，目标是移除 `braft-editor` / `draft-js` 带来的 `node-fetch` 旧链路。
  - Umi / Pro Layout / routing 链路升级，目标是移除 `path-to-regexp` 旧版本链路。
  - OpenAPI mock 工具链替换，目标是移除无补丁的 `mockjs`。
- 上述三项都需要 beta smoke plan 和 rollback plan，不能走无运行时验证的直接 override。
- docs 的 `elliptic` low 因无 patched version，建议继续保留在安全 FAQ 和 RFC 中跟踪，避免制造不可验证的临时 patch。

## 外部社区草稿

### 中文短帖

mss-boot 本周继续推进开源治理和供应链安全：docs 工具链安全告警已收敛到 only-low，admin-antd 连续合并两轮低风险依赖修复，覆盖 hono、piscina、form-data、@babel/core、js-yaml、@babel/runtime、braces、ws、ajv、cross-spawn、immer 等可验证项。剩余旧前端栈问题会按 RFC 拆分迁移，欢迎对 Umi/Pro Layout/富文本编辑器/OpenAPI mock 迁移有经验的同学参与 review。

### English Short Post

mss-boot continued its open-source governance and supply-chain hardening work this week. The docs toolchain is now reduced to low-only audit findings, and admin-antd merged two focused dependency hardening rounds covering hono, piscina, form-data, @babel/core, js-yaml, @babel/runtime, braces, ws, ajv, cross-spawn, and immer. The remaining legacy frontend stack risks will move through RFC-driven migration work. Reviews around Umi/Pro Layout, rich-text editor, and OpenAPI mock migration are welcome.
