---
title: D3 Named Redis Resource 内部 checkpoint
order: 14
description: v1.1.0 D3 named Redis 单一所有权、隔离 scope、调用方 deadline、精确开发证据与冻结前门禁
keywords: [v1.1.0 D3 redis named resource scope lifecycle race]
---

# D3 Named Redis Resource 内部 checkpoint

本文记录累计进 `v1.1.0` 的 `D3-backend-runtime` named Redis 开发检查点。
实现提交为 `86c0e8a`，它组合了 `c57ffc8` 修复后的资源 error-tree 脱敏边界。
二者都是未打 tag 的开发 checkpoint，不是 feature-freeze SHA、Framework Release、
Provider conformance 或发布授权。`platform.storage-runtime-v2` 继续保持 Planned。

## 已落地的窄边界

新增的 `mss-boot/runtime/redisresource` 是 additive Framework 包；它消费已经归一化的
`runtime/config.ResourceProfile` 与 `runtime/resource`，不改动或接管 v1.0 的
process-global Redis 兼容路径：

- `Build` 严格只接受 `ProviderRedis`，复制 profile，并保持无网络、无 goroutine、
  无 client construction。`Start` 才为 standalone、Sentinel 或 cluster 构造唯一 client；
  cluster 再次防御性固定 database 0。
- TLS CA、mTLS certificate/key、data-node ACL 和密码只从已解析 profile 构造；公共 API
  不导出 raw secret、raw go-redis client 或 provider `Close`。
- 一个 named `Resource` 只贡献一个 graph `Definition`、一个 health state 和一个 tracked
  close generation。多个 canonical `Scope` 确定性复用各自的窄 capability，但共享同一 client；
  物理 key 同时绑定 resource name 与 scope name，不同 resource 或 scope 不碰撞。
- `Scope.Use` 是 structured callback lease。命令同时受 `Use` context 和 command context
  约束；callback 返回时禁止新命令、取消并 drain 已开始命令，retained 或 detached lease
  work 返回固定 typed rejection。
- `Start`、`Ready`、`Health` 使用 caller-scoped `PING`；Get/Set/Delete/Exists 同样保留
  caller deadline。三种 go-redis topology 都启用 context timeout，stalled socket 证据覆盖
  lifecycle PING 与 Get/Set deadline。
- context-free provider `Close` 只在一个后台 tracked generation 中调用一次。当前 caller
  可以按自己的 deadline 返回，后续或并发 `Close` 加入同一 generation，并共享固定脱敏结果。
- missing key 映射为 provider-neutral `ErrNotFound`。公共错误链只保留固定 package sentinel
  与 caller cancellation/deadline；provider 对象、错误文本和 secret canary 不能通过格式化、
  递归 `Unwrap` 或 `errors.As` 取回。
- cluster 的多 key `Delete`/`Exists` 先验证全部 opaque key，再按单 key fail-fast 执行，
  避免 `CROSSSLOT` 与 scope-wide hash-tag 热点。失败返回已完成的 partial count 和固定错误。

## Exact development evidence

机器合同逐一要求 `runtime/redisresource` 的 22 个真实顶级测试。命令使用单包 runner、
完全锚定的 `--run`、每项 `--require`、`GOWORK=off`、race detector 和 20 次非缓存执行；
缺失、skip、失败、cached-only 或零命中都会失败：

```shell
go run ./cmd/mss test evidence --directory mss-boot --package ./runtime/redisresource \
  --run '^(TestBuildIsPureAndStartConstructsOneOwnedClient|TestBuildTopologyCredentialAndTLSMatrix|TestDefaultFactoryConstructsTopologySpecificGoRedisClients|TestDefaultFactoryEnablesContextTimeoutsForEveryTopology|TestBuildRejectsNonRedisAndScopeRejectsUnsafeNames|TestErrorsAndFormattingRedactProfileAndProviderData|TestRealStalledPipeHonorsCallerDeadlines|TestOneNamedResourceSharesOneClientAcrossIsolatedScopes|TestDifferentNamedResourcesCannotSharePhysicalScopePrefix|TestClusterPortableMultiKeyCommandsArePartitionedBySlot|TestScopeUseContextBoundsBackgroundCommand|TestScopeUseCancelsAndRejectsDetachedCommandAtCallbackReturn|TestLeaseCommandsUseCallerContextAndRedactProviderErrors|TestStandaloneDefaultClientAgainstMiniredis|TestGetMissingReturnsProviderNeutralNotFound|TestStartFailureCleansUpExactlyOnce|TestReadyHealthAndCallerCancellation|TestCloseDeadlineDrainsLeaseAndPermanentlyRejectsUse|TestCloseDeadlineBoundsProviderCloseAndRetryJoinsGeneration|TestResourceRejectsScopeCreationAfterCloseBegins|TestConcurrentCloseAndUseNeverCommandsAfterClientClose|TestResourceGraphIntegration)$' \
  --count 20 --race --go-work off \
  --require TestBuildIsPureAndStartConstructsOneOwnedClient \
  --require TestBuildTopologyCredentialAndTLSMatrix \
  --require TestDefaultFactoryConstructsTopologySpecificGoRedisClients \
  --require TestDefaultFactoryEnablesContextTimeoutsForEveryTopology \
  --require TestBuildRejectsNonRedisAndScopeRejectsUnsafeNames \
  --require TestErrorsAndFormattingRedactProfileAndProviderData \
  --require TestRealStalledPipeHonorsCallerDeadlines \
  --require TestOneNamedResourceSharesOneClientAcrossIsolatedScopes \
  --require TestDifferentNamedResourcesCannotSharePhysicalScopePrefix \
  --require TestClusterPortableMultiKeyCommandsArePartitionedBySlot \
  --require TestScopeUseContextBoundsBackgroundCommand \
  --require TestScopeUseCancelsAndRejectsDetachedCommandAtCallbackReturn \
  --require TestLeaseCommandsUseCallerContextAndRedactProviderErrors \
  --require TestStandaloneDefaultClientAgainstMiniredis \
  --require TestGetMissingReturnsProviderNeutralNotFound \
  --require TestStartFailureCleansUpExactlyOnce \
  --require TestReadyHealthAndCallerCancellation \
  --require TestCloseDeadlineDrainsLeaseAndPermanentlyRejectsUse \
  --require TestCloseDeadlineBoundsProviderCloseAndRetryJoinsGeneration \
  --require TestResourceRejectsScopeCreationAfterCloseBegins \
  --require TestConcurrentCloseAndUseNeverCommandsAfterClientClose \
  --require TestResourceGraphIntegration
```

这项证据包含 factory-injected standalone/Sentinel/cluster/TLS 构造矩阵、hermetic 并发与
failure injection、真实 stalled `net.Pipe` deadline，以及 standalone miniredis。它没有把
构造矩阵冒充真实 Provider conformance。

## 保留到冻结或后续波次的 P2

- Runtime v2 profile 尚无独立 Sentinel control-plane credential refs；Sentinel discovery
  ACL 因而仍为匿名，只映射 Redis data-node ACL。
- cluster 多 key `Delete`/`Exists` 是非原子 fail-fast 操作，可能返回 partial count；它不适合
  Challenge 或其他跨 key 原子状态机。
- 真实 Sentinel、multi-node cluster、TLS/mTLS、failover/outage、Admin readiness-before-listen
  composition，以及 100 次真实 goroutine/file-descriptor 上界仍是 feature-freeze required gates。
- ChallengeStore 需要 server-owned opaque same-slot atomic capability；当前 Lease 明确不提供
  跨 key 原子语义，也不允许 consumer 自带原始 `{...}` hash tag。

## 兼容、安全与恢复

该 checkpoint 只新增公共包与 API，不删除 v1.0 exported Go surface，不改数据库 schema，
也不迁移 Admin consumer。legacy Redis global 仍是 Legacy inventory，不能和 named Resource
共享 client、重复 Close 或作为失败 fallback。

错误分类保留固定 package sentinel 与 caller cancellation/deadline，但不公开 provider object
或 secret 文本。关闭 caller 超时只表示等待超时，不表示 provider `Close` 被再次调用；恢复时
应使用新 deadline 重试同一 `Resource.Close`，加入已存在的 generation，不替换 client 或重启资源。
由于 Admin 尚未接线，本 checkpoint 没有数据迁移或运行时回滚步骤；若需撤回，只能在未发布的
后续提交中停止组合该 additive 包，不能移动已发布 tag 或恢复隐式 global fallback。

下一条可执行工作是把该 named Resource 作为唯一 Definition 接入 Admin composition root，
证明 required readiness 早于 listener 且 shutdown 只有一个反向 owner；随后增加 server-owned
same-slot atomic capability并桥接 ChallengeStore。完成这些开发项后，才在选定的 feature-freeze SHA
上运行真实 Sentinel/cluster/TLS、failure、leak、external consumer、`verify --all` 与 `eval run --all`。
