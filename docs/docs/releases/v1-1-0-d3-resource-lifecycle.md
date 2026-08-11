---
title: D3 Resource Lifecycle 内部 checkpoint
order: 13
description: v1.1.0 D3 资源图状态机、所有权、精确开发证据与冻结前剩余门禁
keywords: [v1.1.0 D3 runtime resource lifecycle readiness shutdown race]
---

# D3 Resource Lifecycle 内部 checkpoint

本文记录累计进 `v1.1.0` 的 `D3-backend-runtime` 资源生命周期开发检查点。
实现提交为 `d90b4c7`，确定性 close-generation evidence 修复为 `c830b5f`，
provider error-tree 脱敏修复为 `c57ffc8`；它们是未打 tag 的开发 checkpoint，
不是 feature-freeze SHA、Framework Release 或发布授权。`platform.storage-runtime-v2` 仍为 Planned。

## 已落地的边界

新的 `mss-boot/runtime/resource` 是领域无关、可加性的资源所有权边界：

- `Build` 复制资源声明，验证规范名称、唯一名称、缺失依赖、重复依赖、循环、
  required readiness 和同一指针被重复拥有，并生成确定性拓扑顺序。它不调用资源、
  不打开连接，也不启动异步工作。
- `Start` 只允许一次，按拓扑顺序获取资源；required 资源必须在任何 dependent
  启动前通过 `Ready`。即使 provider 的 `Start` 只完成部分获取后失败，该资源也会
  进入反向回滚；原始错误与 cleanup 错误共同返回。
- 可选 `Run` 由图创建和取消工作 context。任一 worker 失败或意外正常退出时取消
  peers，并等待参与的 `Run` 调用返回。
- 独立调用 `Graph.Health` 与 `Graph.Ready` 只在成功启动且尚未关闭时执行；这不
  替代 `Start` 内部的 required `Ready` gate。检查与 `Close` 的竞态由图状态机协调。
- `Close` 是不可逆状态转换：拒绝新生命周期工作，取消正在进行的 Start/Run，
  等待检查结束，再按反向拓扑释放已获取 handle。并发调用共享同一个 close
  generation；超时或资源失败后可以重试，已经成功释放的 handle 不会重复关闭。
- 可打印错误只包含经过验证的资源名与固定 operation，不包含 provider 错误文本；
  递归 `Unwrap` 与 `errors.As` 也不能取回 provider error 对象。程序仍可读取固定
  lifecycle metadata，并通过 `errors.Is` 使用自己已持有的 classifier。

该包不创建 Redis、对象存储或其他真实 provider，也尚未接入 Admin composition root。

## 开发 checkpoint evidence

机器合同要求 `runtime/resource` 文件中全部 11 个顶级测试精确命中。以下命令使用
单包 evidence runner、完整锚定的 `--run`、每项 `--require`、`GOWORK=off`、race
detector 和 20 次非缓存执行；任一测试缺失、skip、失败或零命中都会使 evidence 失败：

```shell
go run ./cmd/mss test evidence --directory mss-boot --package ./runtime/resource \
  --run '^(TestGraphStartsTopologicallyAndClosesInReverse|TestGraphPreflightRejectsInvalidMissingAndCyclicDefinitionsWithoutSideEffects|TestGraphStartupFailureRollsBackInReverseAndJoinsErrors|TestGraphCancellationAndCloseDeadlineReleaseOwnership|TestGraphConcurrentRepeatedCloseIsIdempotentAndRejectsStart|TestGraphConcurrentCloseSharesFailedGenerationBeforeRetry|TestGraphReadinessFailureRollsBackBeforeDependentStart|TestGraphHealthAndReadyJoinRedactedDiagnostics|TestGraphRunCancelsPeersAndJoinsSanitizedDiagnostics|TestGraphCloseCancelsRunBeforeReverseResourceClose|TestGraphProviderErrorsPreserveExpiredCallerDeadline)$' \
  --count 20 --race --go-work off \
  --require TestGraphStartsTopologicallyAndClosesInReverse \
  --require TestGraphPreflightRejectsInvalidMissingAndCyclicDefinitionsWithoutSideEffects \
  --require TestGraphStartupFailureRollsBackInReverseAndJoinsErrors \
  --require TestGraphCancellationAndCloseDeadlineReleaseOwnership \
  --require TestGraphConcurrentRepeatedCloseIsIdempotentAndRejectsStart \
  --require TestGraphConcurrentCloseSharesFailedGenerationBeforeRetry \
  --require TestGraphReadinessFailureRollsBackBeforeDependentStart \
  --require TestGraphHealthAndReadyJoinRedactedDiagnostics \
  --require TestGraphRunCancelsPeersAndJoinsSanitizedDiagnostics \
  --require TestGraphCloseCancelsRunBeforeReverseResourceClose \
  --require TestGraphProviderErrorsPreserveExpiredCallerDeadline
```

这项证据只证明 hermetic resource 上的图状态机与 owned-handle 语义。它不证明：

- 真实 goroutine 或 file descriptor 的泄漏上界；
- Redis、RustFS/S3-compatible 或其他 provider 的 Health/Ready 行为；
- Admin 在 HTTP/gRPC listener 接受请求前完成 required readiness；
- provider outage、真实网络取消、部署矩阵或长期 soak。

`c57ffc8` 没有增加第 12 个顶级测试；它把 provider-error 对象和 canary 的递归
error-tree 断言加入现有 `TestGraphHealthAndReadyJoinRedactedDiagnostics`，因此上面的
11 项 exact evidence 同时覆盖该修复。

## 冻结前仍未通过的 required gates

feature freeze 仍保留并必须从最终完整 SHA 执行：

1. `lifecycle-conformance-gate` 的 100 次 race 启停、真实 goroutine/FD 泄漏、
   cancellation、deadline、rollback 与恢复检查；
2. `admin-integration-gate` 的 required-resources-before-listeners、bounded shutdown、
   provider fail-closed 和其他 Admin composition 断言；
3. named Redis standalone/Sentinel/cluster/TLS 的真实 fixture、health、namespace、
   one-client/one-close 与 outage conformance；当前 additive named-resource checkpoint
   只完成构造、生命周期、standalone miniredis 与 stalled-socket 开发证据；
4. exact feature-freeze SHA 上的 Framework 全量 `GOWORK=off`、external consumer、
   provider report、`verify --all` 与 `eval run --all`。

当前 20 次 hermetic evidence 不得复用为这些冻结证据，也不提升任何 Redis、Queue、
Lock 或 ObjectStore provider 的成熟度。

## 兼容、安全与恢复

该 checkpoint 新增独立包，不删除 v1.0 的公共 Go API，也不改变数据库 schema 或
Admin 配置。旧的 process-global storage client 仍是 Legacy inventory，不能被包装成
新图的隐式 fallback 或第二 owner。

`c57ffc8` 后公共 error tree 不再暴露可递归 unwrap/As 的 provider 对象或文本；调用方
仍应只记录固定 lifecycle metadata 和受控 classifier，不应另行旁路记录 provider secret。
启动失败时保留原始错误并执行反向 cleanup；shutdown 超时时保持 graph closing，修复
未完成资源后用新 deadline 重试 `Close`，不要重新 `Start`、替换 client 或跳过已声明
依赖。已经发布的 tag 不移动、不重写，只做前向修复。

后续 `86c0e8a` 已从严格 Runtime v2 配置构造 additive named Redis resource；其窄边界与
22 项 exact evidence 见
[D3 Named Redis Resource 内部 checkpoint](/releases/v1-1-0-d3-named-redis-resource)。
下一步是接入 Admin composition root，证明 required readiness 早于 listener 且 shutdown
只有一个反向 owner，再补 server-owned same-slot 原子 capability 并桥接 ChallengeStore。
Generator/Blueprint 轨同时继续 supplier backend。
