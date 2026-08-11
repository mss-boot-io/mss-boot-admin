---
title: D5 Scoped Runtime Cache checkpoint
order: 18
description: v1.1.0 D5 数据库权威 scoped cache、显式 QueryCache、事务绕过与精确开发证据
keywords: [v1.1.0 D5 cache querycache gorm redis transaction singleflight]
---

# D5 Scoped Runtime Cache checkpoint

本文记录提交 `88f40c34175fb3e215e0de228a348f42e950ef6f` 新增的
`mss-boot/runtime/cache`。这是未打 tag 的 `v1.1.0` 开发 checkpoint，不是
feature-freeze SHA、发布授权或 Stable 晋级证据。`platform.storage-runtime-v2` 继续保持
Planned，`platform.scoped-derived-cache` 与 `platform.query-cache` 继续保持 Beta。

## 已落地的窄边界

- `Policy` 显式声明 database authority、Redis Scope 内 namespace、TTL、最大 payload、
  provider failure 的 authority bypass，以及 loader reconstruction。构造器不执行 provider I/O，
  cache 不接触 raw Redis client、SecretRef 或 provider `Close`。
- `Derived.Load` 对同一目标执行本地 singleflight。datasource、table 与 query identity 在进入
  provider key 前均被摘要化；不同 Scope、namespace 与逻辑 datasource 保持隔离，同一逻辑 datasource
  可以由不同进程实例形成一致身份。
- generation invalidation 通过切换 dataset generation 使并发中的旧 loader 写入不可达；旧 entry 由 TTL
  回收。provider 不可用时读取直接调用 database-authoritative loader，写回失败也不覆盖成功的权威结果；
  当前进程保留 pending generation 并在后续读取时尝试修复。
- 最大 payload 之外的结果仍返回给调用方，但不进入共享缓存。cache entry 同时保留 `NotFound` 与
  `RowsAffected`，因此命中路径可以重现空结果和 `gorm.ErrRecordNotFound`，不会把它们压成普通 miss。
- `QueryCache` 是显式 opt-in target API，不安装 GORM callback，也不是透明 GORM plugin。检测到活动
  transaction 时完全绕过共享缓存，并使用原 transaction handle 执行 loader；read-your-writes 结果与
  rollback 内读取都不会写入 Redis。
- `Close` 只取消并等待本 adapter 的 flight、transaction bypass 和 invalidation；共享 Scope 与其唯一
  Redis resource owner 不受影响。

## Caller 责任与恢复边界

调用方负责 payload codec，以及稳定、非敏感的 `QueryIdentity`。该 identity 必须覆盖所有会改变结果的
查询参数、preload graph 与 scan/result shape；不得把 raw SQL、DSN、凭据或敏感业务值当作 identity。
本 checkpoint 不解析 GORM statement，也不自动判断一个表是否允许 stale-tolerant cache。

provider outage 中的当前请求会 bypass 到数据库，发起失效的当前进程也会保留 pending generation；但这不是
跨进程权威 revision。provider 恢复后的多副本一致性仍依赖后续 EventBus 通知和 database-revision
reconciliation。安全状态、OAuth state 与 Challenge 不得复用本 cache-style fallback。

## Exact development evidence

以下命令只选择 `88f40c3` 新增的八个顶级测试。evidence runner 解析 `go test -json`，固定
`--count 1`、race detector 与 `GOWORK=off`，并逐项 `--require`。本次运行的八项测试均为
uncached、run/pass=1、skip=0：

```shell
go run ./cmd/mss test evidence --directory mss-boot --package ./runtime/cache \
  --run '^(TestDatabaseAuthoritativeFallbackAndCommittedWriteOutcome|TestMissSingleflightAcrossConcurrentReaders|TestGenerationInvalidationWinsLoaderRace|TestNamespaceScopeAndCrossInstanceDatasourceIdentity|TestPayloadBoundBypassesSharedCache|TestNotFoundAndRowsAffectedMetadataRoundTrip|TestActiveTransactionBypassesCacheReadYourWritesAndRollback|TestCloseAndContextBoundFlightsWithoutClosingScope)$' \
  --count 1 --race --go-work off \
  --require TestDatabaseAuthoritativeFallbackAndCommittedWriteOutcome \
  --require TestMissSingleflightAcrossConcurrentReaders \
  --require TestGenerationInvalidationWinsLoaderRace \
  --require TestNamespaceScopeAndCrossInstanceDatasourceIdentity \
  --require TestPayloadBoundBypassesSharedCache \
  --require TestNotFoundAndRowsAffectedMetadataRoundTrip \
  --require TestActiveTransactionBypassesCacheReadYourWritesAndRollback \
  --require TestCloseAndContextBoundFlightsWithoutClosingScope
```

这组开发证据覆盖 provider failure 后的数据库回退与已提交写结果、miss singleflight、generation race、
namespace/Scope 与跨实例 datasource identity、payload bound、not-found/RowsAffected round trip、活动事务
bypass/read-your-writes/rollback，以及 context/Close 不关闭共享 Scope。它不证明 EventBus、跨进程 outage
恢复、透明 GORM callback、Admin composition 或 feature-freeze 全量矩阵。

## 兼容、恢复与下一步

本 checkpoint 没有数据库迁移或配置迁移；新 package 和 API 都是 additive，既有 legacy cache package
未被替换或删除。若需要撤回，应在未发布的 forward commit 中停止新 consumer 注入并保留 database loader，
不要恢复 process-global client，也不要把安全状态切回 cache fallback。

下一条可执行工作是完成 EventBus 与 database-revision reconciliation，使失效通知丢失、重复、乱序或
provider outage 后都能从数据库权威 revision 修复。选定 feature-freeze SHA 后，必须从该 SHA 重跑同一条
八测试 evidence command，并在集中门禁中记录结果；本页的 `88f40c3` 开发运行不能复用为发布证据。
