---
title: D2 Downstream Snapshot Identity 内部 checkpoint
order: 12
description: v1.1.0 D2 四身份消费者、严格快照状态、原子记录与冻结前剩余门禁
keywords: [v1.1.0 D2 snapshot identity blueprint upgrade doctor MCP compatibility]
---

# D2 Downstream Snapshot Identity 内部 checkpoint

本文记录累计进 `v1.1.0` 的 `D2-contract-substrate` 下游快照身份检查点。实现提交是
`151a91c`；它是未打 tag 的开发 checkpoint，不是 feature-freeze SHA、GitHub Release 或发布授权。
`foundation.upgrade` 继续保持 Beta，`agent.fullstack-module-generator` 继续保持 Planned。

本 checkpoint 已执行本地精确测试并固定了可重跑的机器合同，但**没有真实运行**
`.github/workflows/foundation-compatibility.yml`。静态 workflow 合同测试只能证明 checked-in YAML
包含指定断言，不能证明 GitHub Actions、artifact、0.1→0.2 升级或第二次空升级已经通过。

## 四个身份与两个记录

四个身份有独立来源，任何一个都不能从另一个版本字段推导：

| 身份 | 权威来源 | SnapshotStatus 中的关键字段 |
| --- | --- | --- |
| Foundation release | committed `.mss/release-policy.yaml` 与 Foundation 完整 Git commit | repository、version、full commit、channel、source |
| Blueprint | `.mss/blueprints/management-system.yaml` 的语义 revision 与规范化内容 digest | name、version、SHA-256 |
| generator | 实际运行的 `mss` binary build info | tool、version、full commit |
| downstream snapshot | new-app/upgrade 的项目名、root module、repository 与生成 baseline | project、module、repository、SHA-256 |

`.mss/lock.yaml` 与 `.mss/blueprint-manifest.json` 是同一个 snapshot 的两份原子记录，不是第五个
版本。manifest 记录 lock SHA-256，两份记录共同约束四身份、owned file baseline 与 record path。
writer 在所有普通文件成功后才提交这对记录；reader 在同一锁协议下读取并交叉验证。

源码仓库当前的 `.mss/lock.yaml` 仍是精确的 legacy `v1alpha1` development sentinel。
其中 `foundationVersion: 0.1.0` 是历史项目生成基线，不是目标公开版本；不得手工把 source sentinel
改写成看似已发布的 `v1.1.0` snapshot。

## 已落地的 consumer 合同

- `blueprint.SnapshotStatus` 保留旧的 flat JSON 字段以兼容已有消费者，同时新增权威的
  `identities` 与 `records`。flat 字段只做投影，不构成第二套来源。
- CLI `mss upgrade status`、MCP `mss_get_blueprint_status` 和 doctor
  `snapshot:foundation` 都使用严格 `ReadSnapshot` 结果；同一 downstream 上三者必须逐字段一致。
- upgrade CLI 与 MCP 只从已安装 snapshot 恢复 downstream root module。
  `.mss/project.yaml` 的 `spec.backend.module` 是嵌套 Admin module，不能冒充 root module；
  `spec.foundationVersion` 也不能冒充 Foundation、Blueprint 或 generator identity。
- `InspectSnapshot` 只有三种合法结论：精确 Foundation source sentinel、完整且互相一致的 generated
  lock/manifest pair，或失败。当前格式的 malformed/orphaned pair 不会回退为 source。
- source→generated 转换与 reader 使用同一 snapshot 锁协议；即使 writer 已提交 lock、尚未提交
  manifest，reader 也会等待原子 pair 完成，而不是瞬间误分类为 source。
- doctor 对 Foundation source 返回非 required 的 info；对 generated downstream 要求严格 snapshot
  并返回 pass；对 malformed/orphaned/current partial state 返回 required failure。
- checked-in compatibility workflow 分别构建带真实 version/full SHA 的 CLI 与 MCP binary，并声明
  CLI/MCP/doctor 状态、四身份及 Blueprint/snapshot/lock digest 的一致性断言。

## 开发 checkpoint evidence

Feature 机器合同中的以下命令都选择一个 repository-local package，使用完全锚定的顶级测试名、
`--require`、`--race`、`--go-work off` 和 20 次非缓存执行。任一 required test 缺失、skip、失败或
未精确命中都会使 evidence 失败。

```shell
go run ./cmd/mss test evidence --directory . --package ./internal/mss/blueprint \
  --run '^(TestReadSnapshotStatusPreservesFlatMetadataAndAddsIdentityRecords|TestInspectSnapshotDistinguishesStrictFoundationSourceSentinel|TestInspectSnapshotRequiresValidGeneratedPairAndNeverFallsBack|TestInspectSnapshotWaitsForSourceToGeneratedTransition|TestFoundationCompatibilityWorkflowPinsIndependentIdentityEvidence)$' \
  --count 20 --race --go-work off \
  --require TestReadSnapshotStatusPreservesFlatMetadataAndAddsIdentityRecords \
  --require TestInspectSnapshotDistinguishesStrictFoundationSourceSentinel \
  --require TestInspectSnapshotRequiresValidGeneratedPairAndNeverFallsBack \
  --require TestInspectSnapshotWaitsForSourceToGeneratedTransition \
  --require TestFoundationCompatibilityWorkflowPinsIndependentIdentityEvidence

go run ./cmd/mss test evidence --directory . --package ./internal/mss/app \
  --run '^(TestUpgradeApplicationDefersRootModuleIdentityToManifest|TestWriteUpgradeStatusPreservesFlatJSONAndReportsFourIdentities)$' \
  --count 20 --race --go-work off \
  --require TestUpgradeApplicationDefersRootModuleIdentityToManifest \
  --require TestWriteUpgradeStatusPreservesFlatJSONAndReportsFourIdentities

go run ./cmd/mss test evidence --directory . --package ./internal/mss/mcp \
  --run '^TestFoundationUpgradeApplicationDefersRootModuleToSnapshot$' \
  --count 20 --race --go-work off \
  --require TestFoundationUpgradeApplicationDefersRootModuleToSnapshot

go run ./cmd/mss test evidence --directory . --package ./internal/mss/doctor \
  --run '^(TestRunAgentScopeDoesNotRequireFrontendOrDocsToolchains|TestRunAgentScopeRequiresValidGeneratedSnapshot|TestRunAgentScopeFailsMalformedCurrentSnapshotWithoutSourceFallback)$' \
  --count 20 --race --go-work off \
  --require TestRunAgentScopeDoesNotRequireFrontendOrDocsToolchains \
  --require TestRunAgentScopeRequiresValidGeneratedSnapshot \
  --require TestRunAgentScopeFailsMalformedCurrentSnapshotWithoutSourceFallback
```

这些命令证明 `151a91c` 的本地开发边界；后续 implementation 变化必须重跑。它们不能替代下面的
冻结或 pre-root 证据。

## 仍未通过的 required gates

### Feature freeze：真实 compatibility workflow

选择完整 feature-freeze SHA 后，必须真实运行该 SHA 的
`.github/workflows/foundation-compatibility.yml`，并保存 run URL 与不可混淆的 artifact manifest。
冻结 gate 至少要求：

1. checkout SHA、Foundation identity commit 和 generator build commit 都是同一个预期完整 SHA；
2. CLI、MCP 与 doctor 输出完全相同的四身份、Blueprint digest、snapshot digest 与 lock digest；
3. 生成一个真实 Blueprint 0.1 downstream，加入 downstream-only 业务定制，再升级到 Blueprint 0.2；
4. 升级保留定制、确定性报告冲突、只在成功后提交身份 pair；
5. 对同一目标执行第二次 upgrade，报告零 create/update/delete/conflict 和零身份漂移。

当前 checked-in workflow 与静态合同测试不是这项 gate 的已通过证据；正式运行前还必须确认 workflow
fixture 确实覆盖 0.1→0.2 和第二次空升级，而不是只做 0.2.x 开发 revision 的一次升级。

### Pre-root：release-built external artifact

Framework 从同一 reviewed commit 发布并完成仓库外解析后，仍须使用 release-built `mss` artifact，
在源码仓库外、`GOWORK=off` 且禁用本地 replacement 的临时目录中生成新应用。报告必须绑定 artifact
checksum、Framework/root 完整 commit，核对 generation output、lock、manifest、doctor、CLI status 与
MCP status 的四身份和 digests。只有源码 checkout 中的 `go run` 结果不能满足 pre-root gate。

## 失败恢复与回滚

- status/doctor 发现 malformed、orphaned、digest mismatch 或身份矛盾时应停止 upgrade，不要删除其中
  一份记录后回退为 source，也不要手工重算 digest 掩盖不一致。
- dry-run 或 conflict plan 不修改 downstream。apply 失败时保留旧的有效 lock/manifest pair 与冲突报告；
  通过前向修复重新计划，不覆盖 user-owned 文件。
- Framework 已发布而 external artifact gate 失败时，记录 `component-partial / evidence-incomplete`，
  不发布 root `v1.1.0`，不移动或删除已公开 tag，并从更高版本前向修复。

下一条可执行工作仍是继续 D3-D5 开发；完成后选择冻结 SHA，先补齐 workflow 的 0.1→0.2 与第二次空
升级 fixture，再运行真实 Actions 并验证 artifact。D2 checkpoint 本身不提前启动全量发布门禁。
