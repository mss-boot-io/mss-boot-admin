---
title: v1.1.0 精确 Readiness Run 证明 checkpoint
order: 17
description: 将 Framework、Frontend、root 与容器发布绑定到一个明确选择的 release-readiness run ID 及其严格证明制品
keywords: [v1.1.0 release readiness attestation workflow run id publication authority]
---

# v1.1.0 精确 Readiness Run 证明 checkpoint

提交 `0b21bc18ffb51f78571c718448058861a22720bf` 将公开发布的前置条件从“同一 SHA
存在任意一个成功的 `release-readiness` run”收紧为“发布流程验证一个明确选择的 run ID，
并下载该 run 自己的严格证明制品”。这是未打 tag 的 v1.1.0 开发 checkpoint；它只说明仓库内的
绑定机制已经落地，不是 feature-freeze 证据，也不授权发布。

当前 `.mss/release-policy.yaml` 已在阶段执行器与受保护写入路径落地后切换为
`publicationWorkflowsReady: true`。GitHub 已配置带 required reviewer 的 `release` environment，
并用独立 ruleset 控制版本 tag 创建、禁止更新/删除/非快进。策略现在允许运行
`feature-freeze`、`pre-framework` 与 `pre-root`；真正发布仍必须由同一完整 SHA 的指定 authority run
授权。真实 feature-freeze Actions run 与仓库变量选择尚未执行，不能由策略开关本身代替。

## 严格证明合同

`release-readiness.yml` 只在完整 job 成功后上传
`release-readiness-attestation-<workflowRunId>`。由
`.mss/reports/release-readiness-metadata.json` 上传的 `release-readiness-metadata.json`
只允许以下八个字段：

| 字段 | 精确约束 |
| --- | --- |
| `schema` | 固定为 `mss.io/release-readiness-attestation/v1` |
| `targetVersion` | 必须等于当前策略的 `nextPublicVersion`，本列车为 `v1.1.0` |
| `commit` | 必须是被验证的 40 位小写完整 SHA |
| `phase` | 必须等于调用方要求的 `checkpoint`、`feature-freeze`、`pre-framework` 或 `pre-root` |
| `policySha256` | 必须等于当前 `.mss/release-policy.yaml` 的 SHA-256 |
| `workflowRunId` | 必须等于明确选择的正整数 run ID |
| `workflowRunUrl` | 必须是该 run ID 的无 query、无 fragment 标准 URL |
| `publicationAuthority` | 必须是布尔值；publish intent 只接受已启用策略下的 `pre-framework` 或 `pre-root` `true` |

加载器拒绝重复字段、未知字段、非标准 JSON 常量、错误类型和字段变体。策略文件改变后，旧制品的
`policySha256` 不再匹配，必须从目标提交重新运行 qualification，不能继续借用旧证据。

## 发布流程如何选择 run

`tools/release/verify_readiness_run.sh` 先读取指定 run ID 的 GitHub 元数据，再下载名字中包含该
run ID 的证明制品。它要求 run 同时满足：

- 仓库身份、完整 `head_sha` 和标准 run URL 与本次发布一致；
- 状态为 `completed/success`，事件为 `workflow_dispatch`，workflow 路径精确为
  `.github/workflows/release-readiness.yml`；
- 制品名为 `release-readiness-attestation-<run-id>`，并且根目录包含
  `release-readiness-metadata.json`；
- 八字段证明与当前版本、SHA、阶段、策略哈希、run ID、run URL 全部相等；发布意图还要求
  `publicationAuthority=true`。

因此，相同 SHA 上另一个绿色 run、不同阶段的 run、旧策略下的 artifact，或者手工替换的 JSON
都不能代替被选择的 authority run。验证发生在发布写操作之前，失败时 workflow 直接停止。

各发布入口使用的阶段如下：

| 入口 | 必须选择的证明阶段 | run ID 来源 |
| --- | --- | --- |
| `framework-release.yml` | `pre-framework` | 仓库变量 `RELEASE_READINESS_RUN_ID` |
| `frontend-release.yml` | `pre-framework` | 仓库变量 `RELEASE_READINESS_RUN_ID` |
| `release.yml` | `pre-root` | 手工输入 `readiness_run_id`，为空时回退到同名仓库变量 |
| `container.yml` 的版本发布 | `pre-root` | 手工输入 `readiness_run_id`，为空时回退到同名仓库变量 |

一个变量不会让两个阶段混成同一份证据。发布 Framework 前，它指向已批准的 `pre-framework`
run；Framework 发布并完成外部 `GOWORK=off` 解析后，必须重新运行 `pre-root` readiness，再把变量
更新为新的 run ID，才可以进入 root tag、Release 与版本镜像写入。手工 dispatch 可以显式传入
`readiness_run_id`，但仍不能绕过阶段、SHA、策略哈希和 authority 校验。

## 预发布执行顺序

1. 完成 v1.1.0 功能开发和公开合同冻结；在同一提交中配置并评审完整阶段执行器、受保护写入路径、
   release environment 与 tag ruleset，然后才把 `publicationWorkflowsReady` 改为 `true`。
2. 对冻结 SHA 手工运行 `feature-freeze` readiness，使用 `publicationAuthority=false`，保存精确
   run artifact 和完整 qualification 报告。
3. 证据工单批准后，对同一 SHA 运行 `pre-framework` readiness，使用
   `publicationAuthority=true`；把该 run ID 选入仓库变量，然后发布 `mss-boot/v1.1.0`。
4. 从外部 `GOWORK=off` 工作区解析已发布 Framework，更新 Admin 依赖和证据后，对同一 SHA 运行
   `pre-root` readiness，使用 `publicationAuthority=true`；把变量切换到新的 run ID，再发布 root
   `v1.1.0` 与相同版本镜像。
5. 完成安装、运行、恢复、文档、changelog、capability 和发布状态的 post-publication reconciliation。

任何实现或公开合同变更都会产生新的候选 SHA；对应的旧 run 与 artifact 不再满足精确绑定。发布前
可以放弃候选并选择新提交，无需移动 tag。若某个组件已经公开，禁止改写或复用 tag，只能保留证据
工单并用后续同步版本 forward-repair。

## 本 checkpoint 的 focused validation

本页只记录提交 `0b21bc1` 新增的 attestation 与选择器行为，不复用其他历史门禁扩大结论。对应命令为：

```shell
python3 -m unittest discover -s tools/release -p 'test_*.py'
python3 -c 'import yaml; [yaml.safe_load(open(path, encoding="utf-8")) for path in [".mss/commands.yaml", ".mss/features/foundation-v1-1-0-release.yaml"]]'
go test ./internal/mss/spec ./internal/mss/feature
go run ./cmd/mss feature plan .mss/features/foundation-v1-1-0-release.yaml --format json
go run ./cmd/mss context --format json
corepack pnpm@9.15.9 --dir docs build
```

本次补录实际运行了 release 目录全部 21 个测试；上述两个 YAML 合同解析、spec/feature Go 测试、
Feature plan、context 加载和文档构建也都成功。未运行真实 GitHub Actions。

该本地检查不能替代真实 GitHub Actions run，也不能证明尚未配置的 repository environment、ruleset
或变量状态。下一项可执行发布工作仍然是先完成 v1.1.0 剩余功能；进入 feature freeze 前，由维护者
决定并配置上述仓库外保护措施，再评审开启 `publicationWorkflowsReady`。
