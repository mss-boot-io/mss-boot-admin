---
title: Kafka Mark-after-success 内部 checkpoint
order: 8
description: 累积进 v1.1.0 的 Kafka 本地 offset 标记顺序、取消语义与开发检查边界
keywords: [v1.1.0 checkpoint kafka sarama offset mark workqueue safety]
---

# Kafka Mark-after-success 内部 checkpoint

本文记录 `D0-safety` 的第二个内部实现检查点。项目不再计划发布 `v1.0.1`；本变更将随
`v1.1.0` 一起交付，也不代表 Kafka
WorkQueue 已达到 Experimental 或生产可用。机器目录继续将该 adapter 标为 `legacy`，ADR
中的证据状态继续是 **Blocked**。本检查点关闭的是已确认的数据丢失路径：v1.0.0 会在 JSON
解码和业务 handler 运行之前调用 Sarama `MarkMessage`，失败消息可能因此被后续提交并跳过。

机器验收目标位于
`.mss/features/foundation-v1-0-1-storage-safety.yaml`，长期目标边界位于
`docs/adr/2026-08-09-storage-runtime-v2.md`。

## 已落地的边界

单个 consumer-group claim 内的处理顺序现在是：

```text
receive message
  -> JSON decode
  -> build framework Message with session context
  -> run synchronous handler
  -> recheck session cancellation
  -> Sarama MarkMessage
```

- JSON 解码失败立即返回带原因的错误，不调用 handler，不标记当前或排队中的后续消息。
- handler 返回错误时立即停止当前 claim；失败消息和排队中的后续消息不标记，之前已经成功完成的
  消息可以保留自己的标记。
- handler 收到 consumer-group session context，可以在取消或 rebalance 时协作停止在途工作并
  返回错误。
- session 在取消息前已经取消时，不启动 handler；等待开放 message channel 时可以由取消唤醒。
- handler 成功后若 session 已取消，本实现保守地不标记该消息，保留后续 session 重投的可能。
- consumer loop 在父 context 取消或 consumer group 已关闭时退出，不再持续调用 `Consume` 形成
  空转或日志风暴。
- 调试日志只记录 topic、partition 和 offset，不记录原始 payload。

## `MarkMessage` 不等于 broker commit

Sarama `MarkMessage` 在当前 session 的 offset manager 中记录 `message.Offset + 1`。真正写入
broker 取决于 consumer-group session 和 offset 配置，通常由 auto-commit 周期或 session
结束处理；它不是每条消息的同步 broker acknowledgement。当前 adapter 也没有定义
`Consumer.Offsets.AutoCommit.Enable=false` 时的显式 commit 策略，因此 manual-commit 配置不在
已支持范围内。

这个顺序建立的是 **at-least-once 的必要安全条件**，不是 exactly-once：

- handler 完成副作用后、offset 被 broker 接受前进程崩溃，消息可能重投，副作用可能重复；
- handler 返回 nil 后若取消先于本地 mark，消息仍可被后续 session 重投；
- session context 检查与 `MarkMessage` 不是原子操作，取消竞态不能转换成 exactly-once 保证；
- 业务 handler 必须自己设计幂等键或事务边界，不能把 offset mark 当成业务提交协议。

## 验收

本检查点使用 hermetic fake session、claim 和 consumer group，验证本地调用顺序、传给
`MarkMessage` 的原始 message、多消息失败边界以及取消/关闭退出：

```shell
cd mss-boot
GOWORK=off go test -json -race ./pkg/config/storage/queue \
  -run '^TestKafka(DoesNotMarkOnDecodeFailure|DoesNotMarkOnHandlerFailure|MarksOnceAfterHandlerSuccess|CancellationLeavesUnfinishedUnmarked|RunStopsAfterCancellation|RunStopsAfterConsumerClose)$' \
  -count=20
```

六个顶层测试分别证明：decode failure 零 handler/零 mark、handler failure 只保留先前成功
消息的 mark、成功 handler 返回后每条消息恰好一次 `MarkMessage`、四种取消边界不标记未完成
消息、父 context 取消不空转，以及关闭 consumer group 后不空转。fake 同时记录
`MarkMessage` 与 `MarkOffset`，防止实现绕过期望的调用路径；成功回调内部会断言当前消息尚未
标记。

这些是 unit evidence，不是 Kafka broker integration。当前 `mss spec validate` 只校验合同
结构；功能冻结后的集中验证还必须解析 `go test -json`，确认每个精确测试在每轮
都产生 `Action=pass`，并拒绝零命中、skip 或缓存结果。

## 仍然阻断 v1.1.0 功能冻结的最低生命周期门禁

- `Register` 和 Kafka 配置构造仍有 `os.Exit`/`Fatal` 路径，未把失败交还给 application owner。
- `Shutdown` 尚未形成统一、幂等的 consumer/producer 所有权与 deadline close；producer client
  仍可能泄漏。
- consumer-group `Errors()` 尚没有受 owner 管理的观测循环，provider failure 无法形成完整的
  readiness/diagnostic 证据。

这些是 FeatureSpec 中 `own-provider-lifecycle` 的 required gate。本地 Mark 顺序检查点没有满足
它们；在 `TestV101ChangedProvidersDoNotExitOrDetach` 和对应 checkpoint evidence 通过前，
不能进入 `FF-v1.1.0`。测试名保留历史编号，但不再表示待发布 patch。

## 仍然阻断 Provider 晋级的完整 conformance

- manual-commit 配置没有显式策略。
- 持续 broker error 尚无 bounded backoff，poison message 尚无 retry budget 或 DLQ。
- 真实 Kafka 的 auto-commit、session release、rebalance、crash redelivery、outage、重复与
  idempotency 行为没有 non-skipped conformance 证据。
- 本 adapter 仍把通用 WorkQueue 和具体 Kafka provider 生命周期混在旧配置包中；Runtime v2
  会重新定义 owned resource 边界。

上述任一项不能由框架版本号自动视为通过。完成专门的 Kafka lifecycle 与真实 broker suite
后，才可以重新评估 Experimental；`v1.1.0` 不承诺把 Kafka 晋级为 Stable。

## 升级、恢复与回滚

本切片没有数据库迁移、配置格式变更或 consumer-group offset 重置。部署前保存当前 group、
topic、partition 和已提交 offset；发现问题时先停止消费者，再保留 offset 与不含 payload 的
诊断日志。优先使用幂等 forward fix，禁止通过恢复“handler 前 Mark”回滚，因为那会重新引入
失败消息被跳过的数据丢失路径。

如果必须暂时停用 Kafka，停止该 consumer group 并让未提交消息保留在 broker；不要未经恢复
计划手工推进或重置 offset。Upload admission 与对象 Provider/Owner 检查点已经落地，证据分别见
[Upload admission 内部 checkpoint](/releases/v1-0-1-upload-admission-safety) 和
[D1 Object Provider/Owner 内部 checkpoint](/releases/v1-1-0-d1-object-provider-owner)。
下一条工作是完成同一 D1 波次中仍未关闭的 Kafka registration/configuration、producer ownership、
error observation 与 cancellable、idempotent bounded close。
