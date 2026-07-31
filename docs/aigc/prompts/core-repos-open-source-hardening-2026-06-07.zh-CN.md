# Core Repositories Open Source Hardening

日期：2026-06-07

## 范围

本轮只覆盖 `mss-boot-io` 的 4 个核心开源仓库：

- `mss-boot`
- `mss-boot-admin`
- `mss-boot-admin-antd`
- `mss-boot-docs`

## 已直接推进

- 组织级 GitHub 安全治理
  - `mss-boot`、`mss-boot-admin`、`mss-boot-admin-antd`、`mss-boot-docs`
    均已开启 private vulnerability reporting，安全研究者可以通过 GitHub
    私密通道提交漏洞报告。
- `mss-boot`
  - 审查并合并 5 个 Dependabot PR：`#374`、`#375`、`#376`、`#377`、`#378`。
  - 合并后 main 的 `ci`、`govulncheck`、`CodeQL Advanced`、`OpenSSF Scorecard`、
    `Dependency Graph`、`GitHub Actions Mirror` 均已通过。
  - 补齐 issue forms：bug、feature、docs request、config contact links。
  - 修复 Mongo delete action 对 `_id` 的处理：删除前将所有外部传入 id 转换为
    `primitive.ObjectID`，非法 id 返回 `422`，避免将原始字符串直接放入 Mongo
    查询。
  - README 增加 CodeQL、Scorecard badge，更新最新版本到 `v0.7.2`，统一 docs
    域名，补充贡献/安全/good first issue 入口。
- `mss-boot-admin`
  - main 已具备 CodeQL、Scorecard、PR Guard、Docs Drift、Issue Triage、
    govulncheck、weekly digest、Security、Code of Conduct 等基线。
  - 删除 audit middleware 里未使用的 response writer 包装，降低反射写回误报和
    不必要的响应拦截面。
- `mss-boot-admin-antd`
  - 补齐 issue forms：frontend bug、feature、docs request、config contact links。
  - 修复 README / README.zh-CN 的 License 链接，使其指向本仓库 LICENSE。
  - 更新 GitHub description 和 topics，补齐 `react`、`ant-design`、
    `typescript`、`rbac`、`cloudflare` 等展示信号。
- `mss-boot-docs`
  - 补齐 docs bug/docs request issue forms 与 contact links。
  - README 增加 CI、CodeQL、OpenSSF Scorecard、License badge 和公开站点入口。
  - 本轮组织级记忆落盘到 `aigc/prompts/core-repos-open-source-hardening-2026-06-07.zh-CN.md`。

## 验证

- `mss-boot`：`go test ./pkg/response/actions/mgm`
- `mss-boot-admin`：`go test ./middleware ./service`
- `mss-boot-admin-antd`：issue template YAML parse
- `mss-boot-docs`：issue template YAML parse，`pnpm build`

## GitHub 扫描结论

- `mss-boot-admin` 与 `mss-boot-admin-antd` 的 GitHub Community Profile 为 `100%`。
- `mss-boot` 与 `mss-boot-docs` 当前主要缺口是 issue templates，本轮已补齐；
  合并后应重新检查 Community Profile。
- `mss-boot`、`mss-boot-admin`、`mss-boot-admin-antd` 当前各有 1 个 Codex PR
  等待 maintainer review，全部 GitHub checks 已通过。
- `mss-boot-docs` 本轮 PR 已合并，当前无 open PR。

## 仍需 Maintainer 决策或 GitHub 设置

- 在 GitHub 仓库设置中确认 Dependabot alerts、secret scanning 状态；private
  vulnerability reporting 已在 4 个核心仓库开启。
- 为 4 个核心仓库确定 CODEOWNERS 人员/团队，或关闭 code owner review。
- 将 `CI`、`CodeQL`、`govulncheck`、`PR Guard`、`Docs Drift` 等设置为 required
  status checks。当前前三个核心仓有 branch protection，但 required checks 仍需
  maintainer 在 GitHub 设置中确认。
- `mss-boot-docs` 是否需要正式 tag/release 策略仍未定；如果持续部署即为发布，
  需要在 README 或治理文档里明确“不发 release”的策略。
- 是否启用 GitHub Discussions 作为社区支持入口仍需决定。

## 后续优先级

1. 合并本轮 4 仓库 PR 后，复扫 Community Profile、open PR、open issue。
2. 等 GitHub 新一轮 CodeQL/Scorecard 结果，确认源码安全修复和依赖合并是否关闭
   相关 alerts。
3. 补一页 docs 治理索引，集中放 4 仓贡献、安全报告、good first issue、发布前
   检查清单和 release 策略。
