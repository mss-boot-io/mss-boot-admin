---
title: Kafka Mark 与 D1 lifecycle 内部 checkpoint
order: 8
description: 累积进 v1.1.0 的 Kafka offset 标记顺序、严格配置、资源所有权与有界关闭边界
keywords: [v1.1.0 checkpoint kafka sarama offset mark lifecycle owner workqueue safety]
---

# Kafka Mark 与 D1 lifecycle 内部 checkpoint

适用范围是 `v1.1.0` development train 的 `D0-safety` 与
`D1-provider-owner`；本页记录内部实现边界，不授权 tag、Release 或 Provider
晋级。项目不再计划发布 `v1.0.1`，历史文件名和测试名只保留追踪连续性。

D0 关闭了 handler 失败却提前标记 offset 的已知数据丢失路径。D1 随后关闭
registration/configuration 的进程退出、producer 泄漏、consumer error 无 owner
观察以及 detached start/unbounded shutdown 路径。旧 `AdapterQueue` 保留源代码
兼容；新的应用组合根必须使用 additive `ManagedAdapterQueue`。Kafka 在机器目录中
仍为 `legacy`，ADR 证据状态仍是 **Blocked**。

机器验收目标位于
`.mss/features/foundation-v1-0-1-storage-safety.yaml`，长期 WorkQueue 和
Provider 边界位于 `docs/adr/2026-08-09-storage-runtime-v2.md`。

## D0：只在成功后执行本地 mark

单个 consumer-group claim 内的处理顺序是：

```text
receive message
  -> JSON decode
  -> build framework Message with session context
  -> run synchronous handler
  -> recheck session cancellation
  -> Sarama MarkMessage
```

- JSON 解码失败立即返回带原因的错误，不调用 handler，不标记当前或排队中的后续消息。
- handler 返回错误时立即停止当前 claim；失败消息和后续消息不标记，先前成功消息保留自己的 mark。
- handler 收到 consumer-group session context；session 在开始前取消时不启动 handler，等待开放
  message channel 时也可由取消唤醒。
- handler 成功后若 session 已取消，实现保守地不标记该消息，保留后续 session 重投的可能。
- consumer loop 在父 context 取消或 consumer group 关闭时退出，不持续调用 `Consume` 空转。
- 调试日志只记录 topic、partition 和 offset，不记录原始 payload。

## D1：由应用拥有完整 managed lifecycle

`ManagedAdapterQueue` 在兼容 `AdapterQueue` 的基础上增加：

```go
RegisterContext(context.Context, ...Option) error
Start(context.Context) error
Errors() <-chan error
Close(context.Context) error
```

权威调用路径具有以下边界：

- `Queue.InitContext` 要求 caller-owned context，严格校验 broker `host:port`、Provider、
  Kafka version、timeout、TLS、SASL/MSK 组合，并把校验或 client factory 错误返回给 owner。
  TLS 默认校验证书；MSK token provider 继承启动 context。旧 `Queue.Init` 仅是有界、记录错误、
  不终止进程的兼容 wrapper。
- Kafka adapter 复制并固定一份 Sarama 配置，只构造并拥有一个同步 producer；并发 `Append`
  复用它，缺少 stream、nil message 或 per-call Kafka config 会显式失败。
- 每个已接受的唯一 `{topic, group}` 注册拥有一个 consumer-group client。重复注册、`Start`
  之后注册、无效 handler/options 或 factory failure 都返回错误，不替换已存在 consumer。
- Consumer factory 在 owner 跟踪的 registration worker 中、且不持有 adapter mutex 运行。调用方取消会及时
  返回；迟到 client 必须关闭。接受注册与完成结果在同一临界区线性化，避免“调用返回取消但 consumer 已泄漏”。
  旧 `Register` 兼容入口也有固定上限，不能无限等待 broker dial。
- 当前实现只支持 Sarama auto-commit；禁用 auto-commit 会在构造或注册时失败。这个拒绝不等于
  已经实现 manual-commit 协议。
- `Start` 是阻塞式生命周期：它在 caller context 下启动所有 consume 与 `Errors()` observer，
  把 consumer-group error 转入可观察错误流；consume loop failure 会取消 peer work，关闭后不空转。
- `Close` 先拒绝新注册和新 append，取消运行，关闭每个 consumer group，等待 consumer、observer
  与在途 append，再只关闭一次 producer。它遵守 caller deadline；首次超时后可重试等待同一关闭过程。
- Casbin watcher 使用 `RegisterContext` 返回错误。Admin Config 是唯一 queue owner，只在数据库和
  watcher 绑定成功后安装候选，并把 managed queue 注册为 server `Runnable`；关闭顺序是 queue、
  object owner、database handle。Runnable 同时消费 `Errors()`，runtime error 会取消 managed
  `Start` 并传给 manager；即使 `Start` completion 与 buffered error 同时 ready，也会先合并已缓冲
  诊断而不随机丢失。旧 `Register`、`Run`、`Shutdown` 仍可编译，但只是不会 Exit/Fatal
  的兼容桥，不是新代码的所有权入口。
- Reload 退休旧 queue 时先从 active/center 发布面摘除它。完成但带终止诊断的旧 owner 不阻止新快照
  提交；未在 deadline 内完成的 owner 保留为仅供后续 Close 重试的 retiring resource，同时新候选回滚、
  旧数据库保留，任何调用面都不会继续发布已关闭或正在关闭的 queue。

## `MarkMessage` 不等于 broker commit

Sarama `MarkMessage` 只在当前 session 的 offset manager 中记录
`message.Offset + 1`。真正写入 broker 通常依赖 auto-commit 周期或 session
结束处理；它不是每条消息的同步 broker acknowledgement。

因此当前顺序只是 **at-least-once 的必要安全条件**，不是 exactly-once：

- handler 完成副作用后、offset 被 broker 接受前进程崩溃，消息和副作用都可能重复；
- handler 返回 nil 后若取消先于本地 mark，消息仍可在后续 session 重投；
- session context 检查和 `MarkMessage` 不是原子操作；
- 业务 handler 必须定义自己的幂等键或事务边界，不能把 offset mark 当成业务提交协议。

## Checkpoint 验收

D0 mark-order 测试：

```shell
go run ./cmd/mss test evidence \
  --directory mss-boot --package ./pkg/config/storage/queue \
  --run '^TestKafka(DoesNotMarkOnDecodeFailure|DoesNotMarkOnHandlerFailure|MarksOnceAfterHandlerSuccess|CancellationLeavesUnfinishedUnmarked|RunStopsAfterCancellation|RunStopsAfterConsumerClose)$' \
  --count 20 --race --go-work off \
  --require TestKafkaDoesNotMarkOnDecodeFailure \
  --require TestKafkaDoesNotMarkOnHandlerFailure \
  --require TestKafkaMarksOnceAfterHandlerSuccess \
  --require TestKafkaCancellationLeavesUnfinishedUnmarked \
  --require TestKafkaRunStopsAfterCancellation \
  --require TestKafkaRunStopsAfterConsumerClose
```

D1 Kafka owner、registration、error observation 与 close 测试：

```shell
go run ./cmd/mss test evidence \
  --directory mss-boot --package ./pkg/config/storage/queue \
  --run '^(TestKafkaRunReaderDeprecatedSourceCompatibility|TestKafkaConstructionOwnsOneStrictProducer|TestKafkaConstructionReturnsValidationAndProducerErrors|TestKafkaRegisterReturnsValidationAndConstructionErrors|TestKafkaLegacyRegisterReportsError|TestKafkaLegacyRegisterIsBoundedAndClosesLateGroup|TestKafkaUsesOneProducerForConcurrentAppend|TestKafkaAppendRejectsInvalidOptions|TestKafkaDuplicateRegistrationDoesNotConstructSecondGroup|TestKafkaBlockingFactoryReservesOnceAndCloseHonorsDeadline|TestKafkaCanceledRegistrationDoesNotCancelPeer|TestKafkaAcceptedRegistrationWinsLaterCancellation|TestKafkaRegistrationRejectedAfterStart|TestKafkaConsumerErrorIsObservedWithoutSpin|TestKafkaCloseTimeoutCanBeRetriedAndDrainsOperations|TestKafkaCloseAggregatesAllConsumerAndProducerDiagnostics|TestKafkaCompletedCloseDiagnosticsWinCanceledRetry|TestSampleWatcherManagedRegistrationUsesCallerContext|TestSampleWatcherReturnsManagedRegistrationError|TestSampleWatcherRejectsCanceledRegistrationContext|TestSampleWatcherManagedLegacyRegistrationRequiresContext|TestSampleWatcherCasbinCallbackUpdateDoesNotRegisterDuplicateConsumer)$' \
  --count 1 --race --go-work off \
  --require TestKafkaRunReaderDeprecatedSourceCompatibility \
  --require TestKafkaConstructionOwnsOneStrictProducer \
  --require TestKafkaConstructionReturnsValidationAndProducerErrors \
  --require TestKafkaRegisterReturnsValidationAndConstructionErrors \
  --require TestKafkaLegacyRegisterReportsError \
  --require TestKafkaLegacyRegisterIsBoundedAndClosesLateGroup \
  --require TestKafkaUsesOneProducerForConcurrentAppend \
  --require TestKafkaAppendRejectsInvalidOptions \
  --require TestKafkaDuplicateRegistrationDoesNotConstructSecondGroup \
  --require TestKafkaBlockingFactoryReservesOnceAndCloseHonorsDeadline \
  --require TestKafkaCanceledRegistrationDoesNotCancelPeer \
  --require TestKafkaAcceptedRegistrationWinsLaterCancellation \
  --require TestKafkaRegistrationRejectedAfterStart \
  --require TestKafkaConsumerErrorIsObservedWithoutSpin \
  --require TestKafkaCloseTimeoutCanBeRetriedAndDrainsOperations \
  --require TestKafkaCloseAggregatesAllConsumerAndProducerDiagnostics \
  --require TestKafkaCompletedCloseDiagnosticsWinCanceledRetry \
  --require TestSampleWatcherManagedRegistrationUsesCallerContext \
  --require TestSampleWatcherReturnsManagedRegistrationError \
  --require TestSampleWatcherRejectsCanceledRegistrationContext \
  --require TestSampleWatcherManagedLegacyRegistrationRequiresContext \
  --require TestSampleWatcherCasbinCallbackUpdateDoesNotRegisterDuplicateConsumer
```

D1 startup configuration 与兼容面测试：

```shell
go run ./cmd/mss test evidence \
  --directory mss-boot --package ./pkg/config \
  --run '^(TestKafkaBuildConfigValidatesStartupProfile|TestKafkaBuildConfigRejectsInvalidProfiles|TestKafkaBuildConfigSupportsStrictSASLProfiles|TestKafkaBuildConfigBindsMSKToCallerContext|TestKafkaTLSConfigVerifiesCertificates|TestQueueEmptyIncludesKafka|TestQueueInitContextInstallsMemoryAndPropagatesInstallerError|TestQueueInitContextRejectsInvalidOwnership|TestQueueLegacyInitDoesNotTerminateOnKafkaConfigurationError|TestQueueLegacyInitRequiresOwnerContextForMSK)$' \
  --count 1 --race --go-work off \
  --require TestKafkaBuildConfigValidatesStartupProfile \
  --require TestKafkaBuildConfigRejectsInvalidProfiles \
  --require TestKafkaBuildConfigSupportsStrictSASLProfiles \
  --require TestKafkaBuildConfigBindsMSKToCallerContext \
  --require TestKafkaTLSConfigVerifiesCertificates \
  --require TestQueueEmptyIncludesKafka \
  --require TestQueueInitContextInstallsMemoryAndPropagatesInstallerError \
  --require TestQueueInitContextRejectsInvalidOwnership \
  --require TestQueueLegacyInitDoesNotTerminateOnKafkaConfigurationError \
  --require TestQueueLegacyInitRequiresOwnerContextForMSK

go run ./cmd/mss test evidence \
  --directory mss-boot --package ./pkg/config/storage \
  --run '^(TestManagedAdapterQueueRemainsAnAdapterQueue|TestAdapterErrorClassificationsPreserveCause)$' \
  --count 1 --race --go-work off \
  --require TestManagedAdapterQueueRemainsAnAdapterQueue \
  --require TestAdapterErrorClassificationsPreserveCause
```

Admin owner、reload、Runnable、Errors 转发/缓冲 drain 和 legacy bridge 测试：

```shell
go run ./cmd/mss test evidence \
  --directory admin --package ./config \
  --run '^(TestConfigInitialQueueOutageCommitsWithoutQueue|TestConfigQueueCancellationPropagatesAndRollsBackDatabase|TestConfigQueueConfigurationAndWatcherFailuresRollBackDatabase|TestConfigOwnsAndClosesManagedQueueBeforeDatabase|TestConfigCloseRetainsDatabaseUntilManagedQueueCanStop|TestConfigCloseReleasesOwnerAndDatabaseAfterTerminalQueueDiagnostic|TestConfigReplacementRetiresOldManagedQueueBeforeOldDatabase|TestConfigReplacementCommitsNewOwnerAfterTerminalQueueDiagnostic|TestConfigReplacementTimeoutDegradesQueueAndRetainsOldDatabaseForRetry|TestReloadDatabaseDuplicateConsumerKeepsPreviousHandle|TestBindPolicyWatcherRegistersManagedConsumerExactlyOnce)$' \
  --count 1 --race --go-work off \
  --require TestConfigInitialQueueOutageCommitsWithoutQueue \
  --require TestConfigQueueCancellationPropagatesAndRollsBackDatabase \
  --require TestConfigQueueConfigurationAndWatcherFailuresRollBackDatabase \
  --require TestConfigOwnsAndClosesManagedQueueBeforeDatabase \
  --require TestConfigCloseRetainsDatabaseUntilManagedQueueCanStop \
  --require TestConfigCloseReleasesOwnerAndDatabaseAfterTerminalQueueDiagnostic \
  --require TestConfigReplacementRetiresOldManagedQueueBeforeOldDatabase \
  --require TestConfigReplacementCommitsNewOwnerAfterTerminalQueueDiagnostic \
  --require TestConfigReplacementTimeoutDegradesQueueAndRetainsOldDatabaseForRetry \
  --require TestReloadDatabaseDuplicateConsumerKeepsPreviousHandle \
  --require TestBindPolicyWatcherRegistersManagedConsumerExactlyOnce

go run ./cmd/mss test evidence \
  --directory admin --package ./cmd/server \
  --run '^(TestManagedQueueRunnableParticipatesInManagerErrorPropagation|TestManagedQueueErrorsChannelIsNotLost|TestManagedQueueCompletedStartStillDrainsBufferedError|TestStartLegacyQueueDoesNotDetachManagedQueue|TestStartLegacyQueueRetainsNonManagedCompatibility)$' \
  --count 1 --race --go-work off \
  --require TestManagedQueueRunnableParticipatesInManagerErrorPropagation \
  --require TestManagedQueueErrorsChannelIsNotLost \
  --require TestManagedQueueCompletedStartStillDrainsBufferedError \
  --require TestStartLegacyQueueDoesNotDetachManagedQueue \
  --require TestStartLegacyQueueRetainsNonManagedCompatibility
```

Repository root changed-path forbidden-call 测试：

```shell
go run ./cmd/mss test evidence \
  --directory . --package ./internal/mss/verify \
  --run '^TestV101ChangedProvidersDoNotExitOrDetach$' \
  --count 1 --go-work off \
  --require TestV101ChangedProvidersDoNotExitOrDetach
```

验收解析 `go test -json`：每个列出的顶层测试都必须出现 `Action=run` 和
`Action=pass`。零匹配、`[no tests to run]`、skip 或只命中缓存均失败；成功退出码本身
不构成 no-zero-hit 证据。

这些测试是 hermetic checkpoint evidence，不是 Kafka broker integration。它们足以证明 D1
changed-path owner gate，但不能证明 broker commit 或 Provider 成熟度。

## D1 完成不代表 Kafka 晋级

`D1-provider-owner` 已完成，因此下一开发波次是 `D2-contract-substrate`，不是 v1.0.x
发布准备，也不是提前运行完整 release-readiness。以下项目仍阻断 Kafka 从
Legacy/Blocked 进入 Experimental：

- 明确定义并验证 manual-commit 策略；
- bounded retry/backoff、poison-message budget 与 DLQ；
- duplicate/idempotency 业务合同；
- 真实 Kafka 下的 auto-commit、session release、rebalance、crash redelivery、outage 和恢复；
- 非跳过的 real-broker 生命周期、错误观察、泄漏与配置负例证据。

功能冻结门禁仍会重跑 D0/D1 永久哨兵，并要求所选择真实 broker 的 outage、rebalance、
recovery 和 leak evidence。即使这些证据通过，`v1.1.0` 也不自动承诺 Kafka Stable；任何
Provider 晋级必须单独评审 capability 与证据报告。

## 升级、恢复与回滚

本切片没有数据库迁移、consumer-group offset 重置或公开配置格式迁移。严格校验会拒绝以前
可能被默许的矛盾 TLS/SASL/MSK 配置、无效 broker、manual-commit 和 per-call Kafka config；
部署前应先在 checkpoint 环境执行配置负例与启动验证。

回滚或故障恢复时先停止新 append 和 consumer，保留 group、topic、partition、已提交 offset
以及不含 payload/credential 的诊断。优先使用幂等 forward fix。禁止恢复 handler 前 mark、
process Exit/Fatal、detached consumer 或隐式 client owner。若必须暂时停用 Kafka，停止该
consumer group 并让未提交消息保留在 broker；不要在没有 replay 计划时推进或重置 offset。

Object Provider/Owner 的相邻 D1 边界见
[D1 Object Provider/Owner 内部 checkpoint](/releases/v1-1-0-d1-object-provider-owner)。
