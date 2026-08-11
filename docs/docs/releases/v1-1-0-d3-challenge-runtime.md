---
title: D3 Challenge Runtime 与 Admin 组合 checkpoint
order: 16
description: v1.1.0 D3 公共 Challenge API、Admin 生命周期组合、固定 503 与精确开发证据
keywords: [v1.1.0 D3 challenge runtime admin composition redis fail closed]
---

# D3 Challenge Runtime 与 Admin 组合 checkpoint

本文记录提交 `1faa9ef178c9aa2b6392f93160154b128d8e822b` 新增的 Framework Challenge
边界，以及 `3e9ca94e0747ae977886aea27e2be149989dd35f` 完成的 Admin composition。
它们是未打 tag 的 `v1.1.0` 开发 checkpoint，不是 feature-freeze SHA、
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
它没有启动真实 multi-node Redis Cluster，因而不证明真实 `CROSSSLOT`、failover、`NOSCRIPT`
或连接恢复。Admin 组合由下面绑定 `3e9ca94` 的独立证据记录，不能反向冒充 Framework provider
conformance。

## Admin composition：`3e9ca94`

- 启动配置构造 named Redis resource `main`，再派生 Scope `challenge.email`。候选 graph 完成
  `Start` 与 `Ready` 后才发布到 center，随后才继续业务 route 与 HTTP listener 组装。
- Config 是 runtime owner 的唯一持有者。setup rollback 与正常 shutdown 都先撤下 capability，
  再以有界 context 关闭 graph；Challenge consumer 不接触 provider `Close`。
- optional 配置无效或 Redis unavailable 时，应用可以继续启动，但 challenge capability 保持 unavailable；
  FakeCaptcha、登录、注册和密码重置固定返回 503，不回退到 legacy global Redis。required 配置失败则
  阻断 setup。
- FakeCaptcha 使用 `BeginIssue` 取得 reservation，SMTP 投递成功后 `Commit`，投递失败或取消时
  `Abort`。登录、注册、密码重置直接消费 `VerifyOutcome`，不再调用 D0 Admin bridge。
- v1.0 导出的 legacy Go surface 继续源码兼容，但不参与 Admin fallback。Runtime 配置仍是 immutable
  startup snapshot；修改 resource、Scope、SecretRef 或 Challenge 选项后需要重启，不承诺热重载。

以下四条命令只选择 `3e9ca94` 新增或改动 fixture 的顶级测试。每条命令都完全锚定测试名，使用
`--count 1`、race detector 与 `GOWORK=off`；本次运行的 13 个 required test 均为 uncached、
run/pass=1、skip=0：

```shell
go run ./cmd/mss test evidence --directory admin --package ./config \
  --run '^(TestChallengeRuntimeStartsAndReadiesBeforePublicationThenCloses|TestOptionalChallengeRuntimeInvalidOrUnavailableDegradesWithoutLegacyFallback)$' \
  --count 1 --race --go-work off \
  --require TestChallengeRuntimeStartsAndReadiesBeforePublicationThenCloses \
  --require TestOptionalChallengeRuntimeInvalidOrUnavailableDegradesWithoutLegacyFallback

go run ./cmd/mss test evidence --directory admin --package ./center \
  --run '^(TestDefaultCenterSettersAndGetters|TestGlobalCenterAccessorsUseCurrentDefault|TestRuntimeChallengeAccessIsSafeDuringConcurrentPublication)$' \
  --count 1 --race --go-work off \
  --require TestDefaultCenterSettersAndGetters \
  --require TestGlobalCenterAccessorsUseCurrentDefault \
  --require TestRuntimeChallengeAccessIsSafeDuringConcurrentPublication

go run ./cmd/mss test evidence --directory admin --package ./apis \
  --run '^(TestAppConfigProfileProjectsFreshEmailChallengeReadinessAfterCachedProfile|TestAppConfigProfileChallengeReadinessUsesBoundedFailClosedCheck|TestAppConfigProfileChallengeReadinessRequiresCompleteSMTPConfig|TestEmailChallengePurposeIsolation|TestEmailChallengeCanonicalEmailBinding|TestEmailChallengeProviderOutageReturnsServiceUnavailable|TestEmailChallengeSendFailureRotation)$' \
  --count 1 --race --go-work off \
  --require TestAppConfigProfileProjectsFreshEmailChallengeReadinessAfterCachedProfile \
  --require TestAppConfigProfileChallengeReadinessUsesBoundedFailClosedCheck \
  --require TestAppConfigProfileChallengeReadinessRequiresCompleteSMTPConfig \
  --require TestEmailChallengePurposeIsolation \
  --require TestEmailChallengeCanonicalEmailBinding \
  --require TestEmailChallengeProviderOutageReturnsServiceUnavailable \
  --require TestEmailChallengeSendFailureRotation

go run ./cmd/mss test evidence --directory admin --package ./middleware \
  --run '^TestEmailChallengeLoginProviderOutageReturnsServiceUnavailable$' \
  --count 1 --race --go-work off \
  --require TestEmailChallengeLoginProviderOutageReturnsServiceUnavailable
```

`TestEmailChallengeCanonicalEmailBinding` 走成功投递与 Commit；
`TestEmailChallengeSendFailureRotation` 走投递失败与 Abort；
`TestEmailChallengePurposeIsolation` 记录登录、注册、重置三个 VerifyOutcome consumer；config 与 outage
测试固定 publication/close、optional 503/no-fallback 和 required block。这里只记录本次 focused evidence，
不复用历史测试扩大结论。内置浏览器 reload、冻结 SHA listener/lifecycle 矩阵和真实 Cluster/failover
仍由后续门禁单独记录。

## 兼容、恢复与下一步

本 checkpoint 没有数据库迁移。公共包为 additive；历史 D0 Go surface 保持源码兼容并继续
fail closed。Admin 增加严格的 startup runtime/Challenge 配置，运行中不重建 graph；修改后必须重启。
若后续需要撤回，应在未发布的 forward commit 中停止注入新 Challenge，保留兼容 surface，不恢复
raw client/global fallback，也不移动已发布 tag。

Admin composition 已完成。下一条可执行工作是继续 v1.1.0 剩余 D3/D5 功能；真实 Cluster/failover、
冻结 SHA lifecycle 与内置浏览器页面复核留在相应门禁单独记录。是否影响 capability 晋级，仍由
冻结前审阅决定。
