---
title: Upload admission 内部安全 checkpoint
order: 9
description: 累积进 v1.1.0 的解析前限流、Local create-only 写入与开发检查边界
keywords: [v1.1.0 checkpoint upload multipart local storage security]
---

# Upload admission 内部安全 checkpoint

本文记录 `D0-safety` 的第三个内部实现检查点。项目不再计划发布 `v1.0.1`；本变更将随
`v1.1.0` 一起交付，也不表示
Local 或 S3-compatible ObjectStore 已达到 Beta 或生产可用。机器目录继续把两个
Provider 标为 `legacy`，ADR 的证据状态继续是 **Blocked**。

本切片只关闭两条已确认的安全路径：上传请求在 `FormFile` 解析后才限流，以及
Local 存储用用户 ID 和原文件名构造可覆盖、可穿越的目标路径。严格 Provider
选择和单一 S3 client owner 当时留给 `D1-provider-owner`，现已由后续对象 checkpoint
完成；生产 Delivery 仍属于 D4。

## 请求与对象上限

两个认证入口共用同一个 admission 实现：

- `POST /admin/api/storage/upload`
- `POST /admin/api/user/avatar`

每个请求只解析一次不可变策略：

| 配置 | 语义 |
| --- | --- |
| `storage:maxSize` | 对象字节上限；默认 `10485760`（10 MiB），合法范围 `1..104857600` |
| `storage:allowedTypes` | 逗号分隔的 MIME type；支持 `image/*`，不接受 `.jpg` 一类扩展名 |
| 原始请求上限 | `storage:maxSize + 65536`，固定 64 KiB 只用于 multipart envelope |

已知 `Content-Length` 超过原始请求上限时，handler 在读取 body 前返回 413。未知
长度或 chunked 请求先安装 `http.MaxBytesReader`，最多读取上限加一个探测字节。
`FileHeader.Size` 不是权威值；同一个已打开的 multipart 句柄还会被实际扫描到
`maxSize + 1`，再 `Seek(0)` 交给 Provider，避免检查和写入重新打开临时文件。

如果未来 middleware 已经设置 `Request.MultipartForm`，admission 会清理现有 form
并返回 422，而不是在失效的 body limiter 后继续处理。

公开错误合同固定为：

| HTTP | `errorCode` | 语义 |
| --- | --- | --- |
| 413 | `UPLOAD_REQUEST_TOO_LARGE` | 原始请求或实际文件流超限 |
| 422 | `INVALID_UPLOAD` | multipart 无效、缺文件或 MIME policy 拒绝 |

响应和日志不包含内部阈值、文件 body 或临时路径。

## Opaque key 与 Local 写入

新对象 key 使用 `crypto/rand` 驱动的 canonical UUID，并统一放在 `uploads/` 前缀下。
原文件名只作为响应 metadata，不再参与 Local 或 S3 key。Local 写入以当前应用目录
为根打开 Go 1.26 `os.Root`，再执行：

```text
public/uploads/<uuid>
  -> O_WRONLY | O_CREATE | O_EXCL
  -> mode 0600
  -> context-aware max+1 copy
  -> Sync
  -> final context check
  -> Close
```

碰撞返回冲突而不截断已有文件。外部 root symlink、`uploads` 内部 symlink、非法
生成值、伪造的 `FileHeader.Size`、请求取消及返回给调用方的读写/Sync/Close 失败都不能遗留 partial
文件；清理错误会被合并进失败结果。对象已经成功发布后，source 或 multipart temp
cleanup 失败只产生脱敏运维告警，不把成功翻转成 500，从而避免客户端重试生成第二对象。

S3 新写入也使用 opaque key，但本切片没有提供 conditional create。S3 create-only、
typed conflict、checksum、Local 的 crash-atomic temp/publish 和 Local/固定 digest RustFS 共用 conformance
已按维护者范围决策移到可选 post-v1.1 Provider 成熟度波次，不再是 feature-freeze 前置。

## 验收证据

开发检查命令是：

```shell
cd admin
GOWORK=off go test -json -race ./apis ./service ./router \
  -run '^(TestStorageUploadHardLimitBeforeMultipart|TestAvatarUploadHardLimitBeforeMultipart|TestStorageLocalNoClobberAndConfinement|TestStorageUploadPolicyBytesAndMIMEContract|TestCustomRouteContractsCoverOtherRegistrations|TestCustomRouteContractsMatchRuntimeAuthentication)$' \
  -count=20
```

上述命令记录历史开发 checkpoint，不表示本次范围调整重新执行了测试。若 v1.1.0 候选实际改动
对象存储路径，只选择受影响的既有 focused tests，配合受影响编译和一次基础 fail-closed 或
dev-Local smoke；不启动 RustFS，也不要求这组 20 次运行作为发布前置。这些既有测试覆盖：

- known cap+1 零 body read；unknown-length cap+1 恰好读取 cap 和探测字节；
- exact request cap 与 exact object limit 成功；
- `MaxMultipartMemory=1` 的真实 spill 在失败后不可再次打开且临时目录为空；
- 两个生产路径的 handler 以及 `Other()` 注册、认证 mutation 合同；
- 随机 key、顺序/并发 `O_EXCL` 冲突、root/内部 symlink、伪造 size；
- policy 的 bytes/MIME/default/hard-ceiling 契约；admission/MIME 拒绝不查询或调用 Provider；
- 已写入部分字节后的取消会删除目标；冲突只保留既有或胜出的完整对象，不留下新增 partial。

## 后续 D1 状态与功能冻结边界

本检查点可以独立提交，但不会据此晋级对象 Provider。缺少后续全面对象证据本身不阻断
`FF-v1.1.0` 或 Foundation release-readiness：

- 后续 D1 object checkpoint 已经让空、未知、矛盾或不可用 Provider fail closed，删除 ghost/
  per-request client split，并由单一 owner 持有 immutable profile；
- Local 只在 dev 模式与实际 `staticPath` 精确映射时安装，生产 Local 保持 unavailable；
- Provider/SecretRef 已移出 AppConfig；S3 在 D1 只构造和持有 client，上传会在 `Put` 前返回 503；
- S3 Put、Delivery、RustFS conformance 已移到可选 post-v1.1 波次；Kafka changed-path lifecycle 已作为相邻
  D1 checkpoint 关闭，但真实 broker conformance 与冻结后完整 release evidence 仍未通过。

完整语义与精确测试见
[D1 Object Provider/Owner 内部 checkpoint](/releases/v1-1-0-d1-object-provider-owner)。
Kafka 的 managed registration/configuration、producer ownership、error observation 与 bounded close
已完成并记录在 [Kafka Mark/lifecycle 内部 checkpoint](/releases/v1-0-1-kafka-ack-safety)。
下一条可执行切片是 `D2-contract-substrate` 的安全迁移引擎与合同收敛。

## 升级、回滚与兼容影响

本切片没有数据库迁移。新上传 URL/key 从 `{userID}/{filename}` 改为
`uploads/{uuid}`；既有对象不会移动或改名。`storage:maxSize` 的单位继续沿用代码已有的
bytes 语义，但现在有 100 MiB hard ceiling；旧文档中的“数值 10 表示 10 MB”是错误的，
不再保留。`storage:allowedTypes` 必须填写 MIME type。

若上线后发现问题，优先禁用两条上传权限/入口并 forward-fix。不要回滚到解析后限流、
用户/文件名 key 或 `os.Create` 覆盖写入；这些行为会重新引入资源耗尽和数据覆盖路径。
