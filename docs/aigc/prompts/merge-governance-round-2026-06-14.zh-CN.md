# mss-boot-io PR 合并治理记忆 - 2026-06-14

## 背景

用户要求由 leader 判断四个核心开源仓库的 PR 是否可以合并，并在符合技术方向、CI、社区沟通标准后实施 squash merge。

本轮只处理 `mss-boot-io` 组织的四个核心仓库：

- `mss-boot`
- `mss-boot-admin`
- `mss-boot-admin-antd`
- `mss-boot-docs`

## Leader 判断

- 社区功能 PR 可以合并的前提：符合产品方向、变更范围可控、CI 通过、review 线程已处理、必要说明已补齐。
- Dependabot 依赖 PR 可以合并的前提：依赖升级方向合理、冲突由官方工具链收敛、本地测试和 GitHub checks 均通过。
- 前端 `mss-boot-admin-antd` 合并 main 不会自动触发 Cloudflare alpha/beta/prod 部署；相关部署 workflow 为手动触发。
- 未触碰 kubeconfig、集群凭据或外部部署密钥。

## 已合并 PR

### mss-boot-admin

- #396 `fix(online-session): auto-migrate user sessions table`
  - 合并提交：`30d900a2a718e83a7ee4ebb0f215d115f57c6b92`
  - 本地验证：`go test ./cmd/migrate/migration/system`
  - 额外检查：迁移文件 13 位前缀无重复；过期 Copilot review threads 已 resolved。
- #391 `build(deps): bump github.com/grafana/pyroscope-go from 1.2.7 to 1.3.1`
  - 合并提交：`b121e531ceadb93d6348a77bdea00e78d78681d9`
- #392 `build(deps): bump github.com/larksuite/oapi-sdk-go/v3 from 3.4.26 to 3.9.5`
  - 合并提交：`72239752fc4408d59bd4a3d89d90c0cbd1e5dc13`
  - 冲突处理：保留 #391 的 `github.com/grafana/pyroscope-go v1.3.1`，同时保留 #392 的 `github.com/larksuite/oapi-sdk-go/v3 v3.9.5`。
  - 本地验证：`go mod tidy`、`go test ./...`
  - 社区沟通：补充评论修正了一条因 shell 反引号导致的 review 文案异常。
- #393 `build(deps): bump github.com/aws/aws-sdk-go-v2/config from 1.32.23 to 1.32.25`
  - 合并提交：`765fd5e66b4a5d77309e38fbbdb038d872812e62`
- #394 `build(deps): bump github.com/gin-contrib/cors from 1.7.6 to 1.7.7`
  - 合并提交：`8a6c7c1079cc1150a81aa375ff1e9dc524b870ba`
- #395 `build(deps): bump github.com/aws/aws-sdk-go-v2 from 1.41.12 to 1.42.0`
  - 合并提交：`aeb598d85e14d84add313a1da8157e29c40e6a62`
  - 冲突处理：保留 #395 的 `github.com/aws/aws-sdk-go-v2 v1.42.0`，同时保留 #393 的 `github.com/aws/aws-sdk-go-v2/config v1.32.25`。
  - 本地验证：`go mod tidy`、`go test ./...`

### mss-boot-admin-antd

- #114 `fix(online-session): drawer loading, i18n key, button text`
  - 合并提交：`becf14b79e4220a11ed30bc083f8a7de8e7d6f36`
  - 处理：确认作者补齐 Tests/Docs/Security/Release 说明；resolved 过期 review threads；CI 全绿后 squash merge。

### mss-boot-docs

- #37 `docs(aigc): record PR review governance round`
  - 合并提交：`1539043ff4c8fe4862d7c53e0b29b431247196d2`
- #35 `chore: pin docs toolchain security deps`
  - 合并提交：`65d4058af0b95e4d9afadaf30e655f52e9d71719`
- #36 `chore(deps-dev): bump prettier`
  - 合并提交：`a6ac97221a3c656385f320b732fbeafe092464bf`
  - 冲突处理：Dependabot 自动刷新后已包含最新 main；本地 `pnpm build` 通过，GitHub checks 全绿后 squash merge。

### mss-boot

- 本轮检查时无 open PR。

## 验证结果

- 四个核心仓库 open PR 均已清空。
- `mss-boot-admin-antd` main push checks 全绿。
- `mss-boot-docs` #36 main push CI、CodeQL、OpenSSF Scorecard、docs prod deploy 全绿。
- `mss-boot` 最新 main workflows 保持全绿。
- `mss-boot-admin` PR checks 均在 merge 前全绿；最终 #395 main push 的 `govulncheck`、`CodeQL`、`Swagger`、OpenSSF Scorecard、GitHub Actions Mirror 均已成功。

## 后续安全收敛

- `mss-boot-docs` 的 Dependabot Updates 动态任务显示 `esbuild` 最低安全版本为 `0.28.1`，原 override `0.25.11` 已无法满足安全更新要求。本轮后续验证将 override 提升到 `0.28.1` 后，`pnpm install --lockfile-only --ignore-scripts` 与 `pnpm build` 均通过。
- `mss-boot-admin-antd` 仍有较早的 Dependabot Updates 动态任务针对 esbuild 的失败记录。
- 这些 dynamic Dependabot Updates 不是刚才合并 PR 的 required check，但会影响开源仓库健康观感，后续应单独治理。

## 后续建议

- 下一轮优先处理 `mss-boot-admin-antd` 的 esbuild Dependabot Updates 失败，确认是 lockfile/overrides 配置问题还是 Dependabot grouping 问题。
- 对 Go 依赖 PR 继续保持“逐个合并、每合一个重新同步剩余 PR、等待 CI 全绿”的策略，避免多个依赖 PR 之间的 `go.mod`/`go.sum` 冲突进入主干。
- 对社区 PR 继续坚持先回复、再验证、再 approve/merge 的节奏。
