# PR 合并完成轮次记忆

## 背景

- 时间：2026-06-09
- 范围：`mss-boot`、`mss-boot-admin`、`mss-boot-admin-antd`、`mss-boot-docs`
- 目标：评估所有未合并 PR 的正确性和技术方向一致性；对 `codex/**` 且检查通过的 PR 直接合并；外部 PR 需要评估正确性、项目方向和技术路线后再合并。

## 评估结论

- 本轮扫描时，四个核心仓库没有外部作者的 open PR。
- 所有 open PR 均来自 `lwnmengjing` 账号下的 `codex/**` 分支，属于当前 AI 协作治理轮次产生或维护的 PR。
- 技术方向一致：
  - 后端：补测试、补文档、增强 CI/lint 防线，不引入新产品行为风险。
  - 前端：补环境矩阵、收敛依赖安全问题，不触发 Cloudflare 发布。
  - 文档：收敛文档站依赖安全问题，并保留 dumi/Umi 迁移为 RFC 轨道。
  - CI：发布 workflow 幂等、PR Guard/docs-drift/CodeQL/govulncheck/Scorecard 继续作为低成本质量护栏。

## 已 squash merge 的 PR

### mss-boot

- `#382 ci: make release workflow idempotent`
- `#383 ci: block new golangci-lint issues`
- `#384 docs: document test workflow and action glossary`
- `#385 test: cover config env fallback`

### mss-boot-admin

- `#388 docs: document admin test prerequisites and rbac glossary`
- `#389 test: cover language fallback middleware`

`#389` 在 `#388` 合并后出现 AIGC 索引冲突，已在分支上合入 `origin/main`，保留两边记忆入口后重新验证并合并。

### mss-boot-admin-antd

- `#97 ci: make release workflow idempotent`
- `#100 chore: converge frontend security dependencies`
- `#102 docs: add frontend environment matrix`

未触发 Cloudflare 发布，符合“前端低频、公开展示前必须充分本地验证和冒烟测试”的发布约束。

### mss-boot-docs

- `#31 chore: converge docs security dependencies`

## 验证结果

- 四个核心仓库 open PR 已清空。
- `mss-boot` main push workflows：CI、lint、govulncheck、CodeQL、Scorecard、mirror 均通过。
- `mss-boot-admin` main push workflows：CI、Swagger、govulncheck、CodeQL、Scorecard、mirror 均通过。
- `mss-boot-admin-antd` main push workflows：CI、CodeQL、Scorecard、Copilot setup、mirror 均通过。
- `mss-boot-docs` main push workflows：CI、CodeQL、Scorecard、docs deploy 均通过。

## Dependabot dynamic 失败说明

- `mss-boot-admin-antd` 中部分 Dependabot dynamic runs 失败，但日志显示包括 `qs` 在内的任务属于安全告警已不再需要更新或上游无法生成新的安全更新 PR，不是合并 PR 的 push CI 失败。
- `mss-boot-docs` 的 `vite` Dependabot dynamic run 失败原因是 `latest-resolvable-version=4.5.2`，而 `lowest-non-vulnerable-version=6.4.2`，属于 dumi/Umi toolchain 迁移问题，已由 `mss-boot-docs#32` 继续跟踪。
- 这类 dynamic failure 应作为安全迁移/RFC 信号看待，不应误判为本轮 PR 合并破坏主分支。

## 后续边界

- 若后续出现外部作者 PR，必须先评估正确性、测试覆盖、技术方向一致性、发布边界和社区沟通语境，再决定 approve/merge。
- 依赖安全类 PR 仍需区分“可安全收敛的 override/patch/minor”与“必须框架迁移的主链路问题”。
- 前端 Cloudflare 仍保持手动发布，不因依赖/文档 PR 合并而自动发布。
