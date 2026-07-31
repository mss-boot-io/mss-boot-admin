# mss-boot-admin 依赖 PR 治理记忆 - 2026-06-30

## 背景

用户要求继续推进 mss-boot-io 四个核心开源仓库的社区治理和开源完善度，重点处理未合并 PR、流水线健康度和可由 GitHub 自动化承接的事项。

本轮聚焦 `mss-boot-admin` 的 Go module Dependabot PR，采用逐个合并、每次合并后等待 main 全绿、再处理下一个 PR 的节奏。

## 处理原则

- 对 Dependabot PR 先评估 diff、技术方向和检查结果，再 approve。
- 多个 Go 依赖 PR 涉及同一 `go.mod` / `go.sum` 时，优先合并覆盖面更完整的 PR，避免重复冲突。
- 每合并一个 PR 后，必须等待 main 分支 CI、CodeQL、govulncheck、Swagger、OpenSSF Scorecard、GitHub Actions Mirror、Dependency Graph 等关键 workflow 完成。
- 对外部机器人 PR 的回复使用英文、明确、可复核，保持开源社区语境。

## 已完成

### mss-boot-admin #402

- PR：`build(deps): bump k8s.io/apimachinery from 0.36.1 to 0.36.2`
- 实际处理：确认该 PR 同时收敛 `k8s.io/api`、`k8s.io/apimachinery`、`k8s.io/client-go` 到 `0.36.2`，因此优先合并。
- 合并方式：approve 后 squash merge。
- 合并提交：`d841b8e91530bb3e5ea175a41f44f9817f3093b5`
- 结果：main 分支 CI、CodeQL、govulncheck、Swagger、OpenSSF Scorecard、GitHub Actions Mirror、Dependency Graph 和相关 Dependabot Updates 均通过。
- 后续影响：`#399`、`#400` 被该 PR 覆盖后不再保留在 open PR 队列。

### mss-boot-admin #401

- PR：`build(deps): bump golang.org/x/net from 0.55.0 to 0.56.0`
- diff：除直接依赖 `golang.org/x/net` 外，同步收敛 `golang.org/x/crypto`、`x/mod`、`x/sync`、`x/sys`、`x/term`、`x/text`、`x/tools`。
- 合并方式：确认 PR checks 全绿后 approve + squash merge。
- 合并提交：`091666bf7a1c76f72c3a33b006e70f84765f9f00`
- 结果：main 分支 CI、CodeQL、govulncheck、Swagger、OpenSSF Scorecard、GitHub Actions Mirror、Dependency Graph 均通过。

### mss-boot-admin #398

- PR：`build(deps): bump github.com/aws/aws-sdk-go-v2/service/s3 from 1.103.2 to 1.104.1`
- diff：仅升级 `github.com/aws/aws-sdk-go-v2/service/s3`，并同步更新内部模块 `service/internal/checksum`、`service/internal/s3shared`。
- 处理细节：`#401` 合并后该 PR 变为 behind，等待 Dependabot 自动刷新完成并重新确认 PR checks 全绿后再 approve。
- 合并方式：approve 后 squash merge。
- 合并提交：`ef33e6cddf4bab3f6450e19e015aacd00ae8539f`
- 结果：main 分支 CI、CodeQL、govulncheck、Swagger、OpenSSF Scorecard、GitHub Actions Mirror、Dependency Graph 均通过。

## 当前状态

- `mss-boot-admin` 当前 open PR 为空。
- 本轮三个合并后的 main 分支关键 workflow 均已确认 success。
- `govulncheck` workflow 存在非阻断性 annotation：
  - GitHub Actions cache 服务偶发 restore/save 失败，但 job 结论为 success。
  - `actions/setup-go@v5.0.0` 仍提示 Node.js 20 target 被强制运行在 Node.js 24。
  - 同时配置 `go-version` 和 `go-version-file`，实际只使用 `go-version`。
- 落盘本记忆时发现 `mss-boot-docs` 在本地 pnpm 11 下会忽略 `package.json#pnpm.overrides`，导致 frozen install 与 lockfile overrides 不匹配。
- 已在 docs 仓库补充 `pnpm-workspace.yaml`，将 overrides 和 pnpm 11 的 `allowBuilds` 写入 workspace 配置，同时保留 `package.json#pnpm.overrides` 兼容当前 GitHub workflow 使用的 pnpm 9。
- `pnpm-workspace.yaml` 合入 main 后，docs prod deploy 首次失败：`cloudflare/wrangler-action@v4` 推断使用 pnpm 安装 Wrangler，并执行 `pnpm add wrangler@4`，被 workspace root check 拦截。
- 修复方式：在 docs deploy workflow 的 `wrangler-action@v4` 中显式设置 `packageManager: npm`，避免部署阶段依赖 pnpm workspace root 安装语义。

## 后续建议

- 后续单独治理 workflow 噪音：
  - 升级或替换仍声明 Node.js 20 runtime 的 action。
  - 清理 `setup-go` 的重复版本配置，只保留一个来源。
  - 观察 cache 服务错误是否为 GitHub 侧偶发问题；若持续出现，再调整 cache key 或降级对缓存成功的依赖。
- docs 工具链后续可逐步统一到 pnpm 11：
  - 先确认 GitHub Actions、Cloudflare Pages / Wrangler、Copilot Setup Steps 全部兼容 pnpm 11。
  - 再移除 `package.json#pnpm.overrides` 的过渡兼容，避免 overrides 双写。
- 继续使用 GitHub-first 工作流：
  - Dependabot 自动建 PR。
  - PR Guard / Docs Drift / CodeQL / govulncheck 先行筛选。
  - Agent 只对方向正确、diff 可控、checks 全绿的 PR approve + squash merge。
- 下一轮可继续扫描 `mss-boot`、`mss-boot-admin-antd`、`mss-boot-docs` 的 PR、issue 和 workflow，优先处理影响开源体验的红灯与安全告警。
