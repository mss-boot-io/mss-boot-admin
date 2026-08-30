---
title: 验证与评测
order: 6
description: change-aware verify、确定性 eval 和证据交付
---

# 验证与评测

## 先看计划

```sh
mss verify --changed --plan --format json
```

计划说明检测到的变更、选择的检查和报告路径，不执行外部命令。

## 执行检查

```sh
mss verify --changed
mss verify --module orders
mss verify --all
```

发布冻结后的权威本地证据使用自绑定模式：

```bash
mss verify --all --release-evidence --expect-commit "$SHA"
```

该模式把完整提交、验证前后的 tracked-clean 状态和结果写入同一份报告；提交变化或任意 tracked 文件漂移都会失败。`.mss/reports` 中正常生成且未跟踪的报告不影响判定，但该目录内若存在 tracked 文件，其变化同样失败。

- `--changed` 是默认日常入口；
- `--module` 聚焦一个垂直模块；
- `--all` 是显式完整审计入口；发布资格只在冻结后的精确 merged-main commit 上通过 `--release-evidence` 执行一次。它包含严格环境与 Skill 合同、Agent 独立测试和构建、Admin 与 Framework 的 race/coverage/vet/模块元数据检查、独立 next-Foundation 生成与升级、CLI/MCP/doctor 身份一致性、确定性冲突、第二次升级零变更、eval、依赖策略、release workflow 合同、前端 delivery 与 Playwright、Thin Host 和文档构建。PR 阶段使用 `--changed`、聚焦检查和受影响浏览器验收；合并后的候选流水线只构建并核验发布制品，不重复这些宽泛测试。

机器报告位于 `.mss/reports/verify.json`，Markdown 摘要用于审查。退出码和单项结果都
应保留。`--release-evidence` 下，真实外部 Thin Host 检查还会在命令输出中记录一个持久的系统临时证据目录，并在其中写入脱敏的 `evidence-manifest.json`；需要详细复核时，应连同主报告一起归档或记录哈希。

## 固定完整验证证据

用于发布资格的完整验证必须在一个精确且 tracked-clean 的提交上执行，并在执行前后
确认完整 SHA 未变化：

```sh
SHA="$(git rev-parse HEAD)"
test -z "$(git status --porcelain=v1 --untracked-files=no)"
mss verify --all --release-evidence --expect-commit "$SHA"
jq -e \
  --arg commit "$SHA" \
  '.success == true and .evidenceMode == true and .commit == $commit and
   .trackedCleanBefore == true and .trackedCleanAfter == true' \
  .mss/reports/verify.json >/dev/null
test -z "$(git status --porcelain=v1 --untracked-files=no)"
test "$(git rev-parse HEAD)" = "$SHA"
sha256sum .mss/reports/verify.json
```

PR、审查记录或仓库外发布台账必须记录完整 commit、`trackedCleanBefore: true`、
`trackedCleanAfter: true`、精确命令、
报告摘要或哈希和退出结果。`.mss/reports/verify.json` 单独存在不构成发布授权；它也不
替代合并到 `main`、精确 SHA、Tag actor、不可变引用、OIDC、provenance 和公共制品对账。
PR 服务器端只保留治理、漏洞、CodeQL、普通 Admin 单测以及 Framework 变更的普通独立单测；PR 本地使用 `verify --changed`、聚焦检查和受影响浏览器验收。完整 Agent、Admin、Framework、前端、真实 Thin Host 与文档矩阵只在冻结后的精确 merged-main commit 上执行一次自绑定 `verify --all`。Agent 和 Frontend 扩展工作流不响应 `push` 或 `pull_request`，仅在需要跨平台或 GitHub 托管环境诊断时明确手动调度；它们不是发布授权，也不会在每次合并后产生重复的不确定性结果。候选 preview 只验证同一 merged-main commit 产生的精确发布字节。

## 手动扩展资格验证

本地证据通过后，如变更涉及 Agent 安装器、跨平台进程管理、前端包交付或浏览器运行环境，可针对需要复核的精确引用手动运行扩展工作流：

```sh
gh workflow run agent-native-ci.yml --ref <branch-or-tag>
gh workflow run frontend-v6-ci.yml --ref <branch-or-tag>
```

手动运行后必须记录该 run 的完整 Head SHA，并确认它与预期提交一致。运行结果用于补充 GitHub 托管环境证据和诊断环境差异。不得用重跑成功掩盖确定性缺陷，也不得以这两个工作流替代 `mss verify --changed`、受影响页面验收或冻结 merged-main 提交上的 release evidence。

## Evals

```sh
mss eval list
mss eval run --all
mss eval report
```

Eval 衡量 Agent 是否遵守规格、边界、生成幂等、迁移、权限和验证合同；它不替代产品
测试或浏览器验收。

## 按影响增加证据

| 变化 | 最低附加证据 |
| --- | --- |
| Go | focused test，再运行受影响模块 |
| Framework | 独立 `GOWORK=off` test/vet |
| 前端 | lint、TypeScript、focused test、相关 build |
| 迁移 | 空库和上一版本升级 |
| 权限 | 后端允许与拒绝 |
| 生成器 | schema、golden、路径限制、两次幂等 |
| UI | Codex 内置浏览器桌面、窄屏、深链、控制台 |

报告未执行检查及具体原因；不要把计划、服务健康或单一页面截图写成完整通过。
