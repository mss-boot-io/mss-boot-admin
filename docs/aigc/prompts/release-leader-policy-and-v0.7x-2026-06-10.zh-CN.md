# Release Leader Policy And v0.7.x Status

时间：2026-06-10 19:25 CST

## Leader 发版规则

- Leader 可以自行判断并发布小版本/补丁版本；如果验证、流水线、release notes 和跨仓依赖关系满足门禁，不需要额外等待用户确认。
- 大版本、破坏性变更、架构升级、数据库不可逆迁移、公共 API 大幅变更、生产环境切换等必须等待用户审核。
- 发布顺序必须先发 `mss-boot`。
- 如果要发布 `mss-boot-admin`，必须先把 `mss-boot-admin` 里依赖的 `mss-boot` 版本同步到目标 `mss-boot` release，并通过后端本地测试、GitHub CI、govulncheck、CodeQL、Swagger/文档相关门禁。
- 如果后端 release 包要内置前端产物，则需要先发布 `mss-boot-admin-antd`，因为 `mss-boot-admin` release workflow 会下载 `mss-boot-admin-antd` 最新 GitHub Release 的 `dist-local.tar.gz`。
- 前端 Cloudflare 发布仍保持低频原则：只有本地验证、GitHub Actions、真实 beta/prod 冒烟都满足后，才发布到 Cloudflare。

## 本次 v0.7.x 状态

- 已发布 `mss-boot v0.7.4`：
  - tag：`v0.7.4`
  - commit：`e62d7da9678588f3113a964b4e9c9221bf4581ea`
  - Release、Release Draft、Copilot Setup Steps 均通过。
- 已发布 `mss-boot-admin-antd v0.7.1`：
  - tag：`v0.7.1`
  - commit：`282fddd9da26e8d97585e864e48c8c138ad0ac69`
  - Release、Release Draft、Copilot Setup Steps 均通过。
  - GitHub Release 已产出 `dist.tar.gz`、`dist-local.tar.gz`，并推送 GHCR 镜像。
- 本次不继续发布 `mss-boot-admin`：
  - 用户明确说明这次先不用管后端依赖同步。
  - 后续发布 `mss-boot-admin` 前，必须先同步其 `mss-boot` 依赖到已发布的目标版本。

## 前端安全告警后续

- `mss-boot-admin-antd v0.7.1` 发布后，GitHub Dependabot dynamic runs 在 main 上继续暴露旧前端依赖链告警：`@babel/runtime`、`braces`、`ws`、`node-fetch`、`send`、`path-to-regexp`、`esbuild`、`immer`。
- 本地实验分支 `codex/fix-admin-antd-legacy-alerts-20260610` 已验证一组额外 `pnpm.overrides` 可以清掉这些 dynamic 告警目标：
  - `@babel/runtime`
  - `braces`
  - `esbuild`
  - `immer`
  - `node-fetch`
  - `path-to-regexp`
  - `send`
  - `ws`
- 本地验证通过：
  - `pnpm install`
  - `pnpm audit`：上述 8 个目标包命中数为 0；剩余告警集中在旧 mock/legacy toolchain，部分无官方 patched version，适合进入迁移议题而不是继续堆 patch。
  - `pnpm tsc`
  - `pnpm lint:js`
  - `pnpm test -- --runInBand src/util/requestError.test.ts`
  - `pnpm build:local`
  - `pnpm build:alpha`
- 该实验本次不继续发版。后续如果要推进，应先开 PR，浏览器复核、GitHub Actions 通过后再判断是否发布 `mss-boot-admin-antd v0.7.2`。
