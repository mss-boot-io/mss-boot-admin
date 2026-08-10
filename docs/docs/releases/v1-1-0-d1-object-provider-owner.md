---
title: D1 Object Provider/Owner 内部 checkpoint
order: 10
description: 累积进 v1.1.0 的严格对象存储启动 profile、单一 owner 与 fail-closed 上传边界
keywords: [v1.1.0 checkpoint object storage provider owner SecretRef lifecycle]
---

# D1 Object Provider/Owner 内部 checkpoint

本文记录 `D1-provider-owner` 的对象存储子切片。它是未打 tag 的开发 checkpoint，
将随 `v1.1.0` 一起交付；它不授权发布 `v1.0.x`，也不表示整个 D1 已完成。
Kafka registration/configuration、producer ownership、error observation 与 bounded close
仍是进入 D2 前的下一项工作。

Local 与 S3-compatible 在机器目录中仍为 `legacy`，ADR 证据状态仍为 **Blocked**。
本切片只关闭严格配置、隐式 fallback、重复 client 与不明确 close owner 路径。

## 已落地的边界

对象存储 Provider 只允许启动配置中的一个分支：

```yaml
application:
  mode: dev
  staticPath:
    /objects: /absolute/path/to/objects
storage:
  local:
    root: /absolute/path/to/objects
```

Local 的 `storage.local.root` 与 `application.staticPath` 文件系统值都必须是清理后完全相同的绝对路径；
相对路径不会被启动逻辑隐式解析，也不会安装 Local。

或：

```yaml
storage:
  s3:
    endpoint: http://127.0.0.1:9000
    region: local
    bucket: uploads
    usePathStyle: true
    tls:
      allowInsecureHTTP: true
    credentials:
      static:
        accessKeyRef: env://MSS_STORAGE_ACCESS_KEY
        secretKeyRef: env://MSS_STORAGE_SECRET_KEY
```

- Local/S3 与 S3 credential mode 都是 exact-one discriminator；未知、缺失、矛盾或
  部分配置在 client 构造和对象写入前失败。
- `Normalize` 一次性解析 immutable profile，不修改调用者配置；D1 的 `SecretRef`
  只支持 `env://NAME`。静态 access key/secret key 必须成对，default chain 必须显式选择，
  TLS 与 credential mode 是两个独立维度。
- 同一个 profile 最多构造一个 `StorageHandle`。Local 与 S3 操作都通过 lease；关闭后
  拒绝新 lease，等待在途操作，HTTP transport 只关闭一次；超时后的 close 可以重试。
- Admin 的 composition root 先安装唯一 handle 与 pinned filesystem，再注册 Application delivery，
  并在应用退出时关闭同一 owner。并发 close 保持幂等，drain callback 重入 lease 时不会与
  Config mutex 死锁。旧的 ghost `Config.Storage.Init()` 与每次上传新建 S3 client 的路径已删除。
- dev Local owner 在启动时只打开一次 `os.Root`；对象写入与 pinned `StaticFS` 使用同一个
  directory handle，退出时由同一 owner 关闭，避免路径在运行中被替换后写入和读取分叉。
- S3 配置源 bootstrap 使用自己的 profile、handle、调用者 context 与 close owner，
  不与 Admin application ObjectStore 共享隐藏 client。只有 stage overlay 对象不存在是可选状态；
  transport/read 失败或 malformed overlay 会让 bootstrap fail closed。HTTP endpoint 只可在环境中
  显式设置 `s3_tls_allow_insecure_http=true`；省略该 opt-in 或填写非布尔值都不能放行 HTTP。

## AppConfig 与失败语义

Storage AppConfig 的完整 allowlist 只有：

| Key | 含义 |
| --- | --- |
| `storage:maxSize` | 上传对象 byte 上限 |
| `storage:allowedTypes` | MIME type / `type/*` allowlist |

Provider、endpoint、region、bucket、TLS、credential mode 与 credential material 不再投影到
AppConfig，也不能通过设置页或 API 写入。历史行保持 inert，不做数据库删除；提交任何已移除 key
会在 mutation 前整批返回稳定 422，secret read/write 权限不会恢复这条旧配置面。

对象存储是可选应用资源，但上传始终 fail closed：

| 状态 | 行为 |
| --- | --- |
| 未配置、SecretRef 无法解析、profile 非法或 client 不可用 | 不安装资源；应用继续启动；两条上传路由固定返回 `STORAGE_UNAVAILABLE` 503；零写入 |
| Local + `prod`，或开发模式 staticPath 未精确映射配置 root | 不安装 Local；固定 503；不返回伪公共 URL |
| Local + `dev` + staticPath 精确映射同一绝对 root | 允许 create-only 写入；返回 URL 必须能由该实际静态路由读取 |
| S3 profile/handle 构造成功 | D1 只持有 client；Admin 在 `Put` 前返回固定 503 |

因此，“profile 非法”表示对象存储能力拒绝安装，而不是让可选 Provider 用 `Fatal` 终止整个
Admin 进程。日志只输出固定、脱敏的诊断，不包含 SecretRef material、endpoint 值或底层凭据错误。

## 开发 checkpoint 证据

Framework 严格 profile 与 handle 生命周期：

```shell
cd mss-boot
GOWORK=off go test -json -race ./pkg/config \
  -run '^(TestObjectStorageProfileRejectsInvalidConfiguration|TestObjectStorageProfileBuildIsImmutable|TestObjectStorageProfileCanceledBuildCanRetry|TestObjectStorageHandleUseCoversLocalAndS3|TestObjectStorageHandleCloseDrainsAndIsIdempotent|TestObjectStorageHandleCloseTimeoutCanRetry)$' \
  -count=1

GOWORK=off go test -json -race ./pkg/config/source/s3 \
  -run '^(TestSourceClosesBodyOnReadSuccessAndFailure|TestSourceRequiresCallerOwnedContextAndClient|TestSourceWatchReturnsUnsupported|TestSourceContinuesAcrossMissingExtensions)$' \
  -count=1

GOWORK=off go test -json -race ./pkg/config \
  -run '^(TestS3BootstrapOwnedHandleClosesAndMissingOverlayIsOptional|TestS3BootstrapOverlayFailureFailsClosed|TestS3BootstrapGenericNotFoundFailsClosed|TestS3BootstrapMalformedOverlayFailsClosed|TestS3BootstrapRejectsInvalidInsecureHTTPFlag)$' \
  -count=1
```

Admin owner、Application delivery 顺序、fail-closed 与 AppConfig 表面：

```shell
cd admin
GOWORK=off go test -json -race ./config \
  -run '^(TestObjectStorageSingleClientOwnedClose|TestObjectStorageConcurrentCloseIsIdempotent|TestObjectStorageCloseDoesNotDeadlockReentrantLease)$' \
  -count=1

GOWORK=off go test -json -race ./service \
  -run '^(TestStorageValidS3RemainsUnavailableBeforePut|TestStorageLocalProductionRequiresExplicitDelivery|TestStorageLocalSuccessURLIsActuallyServed|TestStoragePolicySnapshotFailureFailsClosed|TestStoragePolicyWithoutSnapshotFailsClosed|TestStorageAppConfigAllowsOnlyUploadAdmissionPolicy)$' \
  -count=1

GOWORK=off go test -json -race ./cmd/server \
  -run '^(TestStorageInstalledBeforeApplicationDelivery|TestStorageAPICheckClosesOwnedResources|TestStoragePinnedRootSurvivesConfiguredPathReplacement|TestStorageSymlinkRootRemainsUninstalled|TestStorageProductionLocalRemainsUninstalled|TestStorageInvalidProfileRemainsUninstalled)$' \
  -count=1

GOWORK=off go test -json -race ./apis \
  -run '^(TestStorageUploadUnboundReturnsFixed503|TestAvatarUploadUnboundReturnsFixed503|TestStorageInvalidPolicyReturnsFixed503|TestStorageAppConfigSurfaceIsAdmissionPolicyOnly)$' \
  -count=1
```

这些是开发 checkpoint 命令。功能冻结后的 required evidence 仍必须解析 `go test -json`，证明
每个精确测试非零命中、通过且无 skip、`[no tests to run]` 或仅缓存结果。

## D4 延后项与 RustFS

D1 没有发出任何 S3 `Put`，也没有启动 RustFS 容器。以下内容统一留在
`D4-authorization-object`：

- private `ObjectStore` 的 Put/Open/Stat/Delete、create-only conflict 与 checksum；
- Admin object metadata migration、tenant/owner/purpose authorization 与 reconciliation；
- 独立 `Delivery` 的 authenticated proxy、signed URL 或显式 public policy；
- Local 与 S3-compatible 共用的 cancel、limit、no-clobber、outage、privacy、lifecycle/leak suite；
- 固定镜像 digest 的 RustFS fixture，以及 required integration 无 skip 的证据。

在这些门禁关闭前，S3 endpoint 拼接、opaque key 或已构造 client 都不是成功 Delivery 的证据。

## 迁移、兼容性与恢复

本切片没有数据库 schema migration。它有意移除旧 Storage AppConfig/provider 配置面与运行时切换，
现有历史行不再生效；下游必须把 Provider 和 SecretRef 移到启动 YAML/环境变量。评估阶段不以旧行为
兼容为设计输入，但发布说明仍必须把这项配置变更标为 breaking contract。

发现问题时先撤销两条上传权限或不安装 Storage profile，再做幂等 forward fix。不得回滚到未知
Provider 静默写 Local、每请求创建 S3 client、拼接伪 URL、恢复浏览器凭据管理或 Provider `Fatal`。
保留不含 secret 的配置字段名、provider 状态、close 结果和对象 key/checksum 诊断。

下一条可执行工作是完成同一 `D1-provider-owner` 波次的 Kafka 生命周期；随后直接进入
`D2-contract-substrate`，不插入发布门禁或 RustFS qualification。
