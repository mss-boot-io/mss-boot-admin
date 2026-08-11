---
title: D3 Challenge Runtime 内部 checkpoint
order: 16
description: v1.1.0 D3 公共 Challenge API、opaque Redis bridge、重放与等 I/O 开发证据及待组合边界
keywords: [v1.1.0 D3 challenge runtime redis bridge replay anti-enumeration]
---

# D3 Challenge Runtime 内部 checkpoint

本文只记录提交 `1faa9ef178c9aa2b6392f93160154b128d8e822b` 新增的 Framework
Challenge 边界。它是未打 tag 的 `v1.1.0` 开发 checkpoint，不是 feature-freeze SHA、
Provider conformance 或发布授权，也不会自动把任何 Challenge 或 Storage Runtime capability
提升为 Stable。

## 已落地的窄边界

- 新的公共 `mss-boot/runtime/challenge` 只接收一个已命名的
  `*redisresource.Scope`。`NewRedis` 是无 I/O 的纯构造器；Challenge 不导出 raw Redis
  client、物理 key、hash tag、任意脚本入口或 `Close`，共享资源生命周期仍由唯一 owner 管理。
- `BeginIssue` 返回 `Reserved`、`Pending`、`Cooldown` 或 `Quota` 固定状态；只有
  `Reserved` 携带 opaque reservation。投递成功显式 `Commit`，投递失败显式 `Abort`；
  `Verify` 只返回 `Verified` 或统一的 `Rejected`，并保持一次性消费、尝试次数上限、
  purpose 隔离、pending 回收、旧 active verifier 保留和 pepper 轮换语义。
- `mss-boot/runtime/internal/redisbridge` 是 Framework 内部的 sealed adapter：服务端派生
  opaque same-slot group/key，只允许仓库内固定 Challenge scripts，并在 structured lease
  内执行。跨 group、非法 script、detached work 和 provider error 都在固定边界内拒绝或脱敏。
- caller/global 限流脚本先识别已写入的 operation ID，再判断当前 cardinality，因此 provider
  已提交但响应丢失后的同 operation replay 在 limit 边界仍保持幂等；单边残缺状态固定拒绝。
- 对所有语法有效的 Verify 请求，missing、损坏、错误或正确路径都执行一次 fixed read 和一次
  fixed completion script；missing/损坏路径还执行 dummy HMAC，避免从 Redis round-trip 数量区分
  subject 是否存在。请求、选项、pepper、reservation、provider error 与其编码形式均使用固定脱敏输出。
- `pkg/config/storage/cache` 中的 D0 exported 类型、构造器与方法签名继续保留并标记
  Deprecated，现有 Admin 因而保持源码兼容。该 bridge 不会成为新 API 的 fallback；本次仅同步
  rate replay 修复、固定脱敏格式和 typed-nil 拒绝。

## Exact development evidence

以下五条命令只选择提交 `1faa9ef` 新增的顶级测试。每条都由单包 evidence runner 解析
`go test -json`，使用完全锚定的 `--run`、`--count 1`、race detector 与 `GOWORK=off`，
并逐项 `--require`；本次运行全部为 uncached、run/pass=1、skip=0：

```shell
go run ./cmd/mss test evidence --directory mss-boot --package ./runtime/challenge \
  --run '^Test.*$' --count 1 --race --go-work off \
  --require TestIssueCommitVerifyAndReservationRedaction \
  --require TestPublicFormattingAndErrorChainsRedactAllMaterial \
  --require TestPendingAbortStaleRecoveryCooldownAndQuota \
  --require TestPendingAbortAndReclaimPreserveActiveVerifier \
  --require TestSameSubjectPurposesAreIsolated \
  --require TestCallerAndGlobalLimitsAreAtomicAcrossSubjects \
  --require TestRateScriptReplayAtLimitIsIdempotent \
  --require TestVerifyExactlyOnceAndBoundedAttempts \
  --require TestParallelRateLimitAndCodeExpiry \
  --require TestPepperRotationAndAntiEnumeration \
  --require TestValidVerifyPathsUseEqualFixedScriptCount \
  --require TestConstructorIsPureClusterPortableAndOwnsNoClose \
  --require TestConstructorCopiesSecrets \
  --require TestOutageContextRandomAndConfigurationFailClosed

go run ./cmd/mss test evidence --directory mss-boot --package ./runtime/internal/redisbridge \
  --run '^Test.*$' --count 1 --race --go-work off \
  --require TestOpaqueAtomicGroupUsesOneServerDerivedSlot \
  --require TestCrossGroupAndInvalidScriptRejectBeforeDriver

go run ./cmd/mss test evidence --directory mss-boot --package ./runtime/redisresource \
  --run '^TestRedisBridge.*$' --count 1 --race --go-work off \
  --require TestRedisBridgeRejectsCrossGroupBeforeProviderIO \
  --require TestRedisBridgeLeaseCancelsAndDrainsDetachedCommand \
  --require TestRedisBridgeContextAndProviderErrorsAreRedacted

go run ./cmd/mss test evidence --directory mss-boot --package ./pkg/config/storage/cache \
  --run '^TestLegacyRateScriptReplayAtLimitAndPartialState$' \
  --count 1 --race --go-work off \
  --require TestLegacyRateScriptReplayAtLimitAndPartialState

go run ./cmd/mss test evidence --directory mss-boot --package ./pkg/config/storage/cache \
  --run '^TestLegacyChallenge.*$' --count 1 --race --go-work off \
  --require TestLegacyChallengeFormattingRedactsCompatibilitySecrets \
  --require TestLegacyChallengeConstructorRejectsTypedNilClients
```

这组证据只证明新增 Framework package、内部 bridge、Redis Scope 适配和 legacy 兼容修复。
它没有执行 Admin HTTP/SMTP composition，也没有启动真实 multi-node Redis Cluster，因而不证明
listener 前 readiness、反向 close owner、真实 `CROSSSLOT`、failover、`NOSCRIPT` 或连接恢复。

## 兼容、恢复与下一步

本 checkpoint 没有数据库迁移或 Admin 配置变更。公共包为 additive；历史 D0 Go surface 保持
源码兼容并继续 fail closed。若后续组合需要撤回，应在未发布的 forward commit 中停止注入新
Challenge，保留现有 bridge，不恢复 raw client/global fallback，也不移动已发布 tag。

下一条可执行工作是把 named Redis Resource 与 `runtime/challenge` 接入 Admin composition root，
证明 required readiness 早于 route/listener、唯一 owner 负责 reverse close，并让邮件投递使用
`BeginIssue -> Commit/Abort`、登录/注册/重置使用统一 `Verify`。真实 Cluster/failover 证据留在
feature-freeze 候选上单独记录；是否影响 capability 晋级，由冻结前审阅决定。
