---
title: 发布与升级
order: 1
nav:
  title: 发布
  order: 3
description: mss-boot-admin 版本状态、升级、兼容性与回滚合同
keywords: [release upgrade rollback compatibility mss-boot-admin]
---

> 最新 D5 开发 checkpoint 已覆盖三条并行路径：[`2771588` Supplier 完整生成投影](/releases/v1-1-0-d5-supplier-generated-module)、
> [Blueprint 0.1→0.2 外部升级演练](/releases/v1-1-0-d5-blueprint-upgrade-rehearsal)，以及
> [`668dfe3` Provider evidence validator](/releases/v1-1-0-d5-provider-evidence-validator)。Provider CLI
> 只严格校验输入报告，不启动任何真实 provider，也不生成或推断 provider 成熟度。ObjectStore/RustFS
> 继续是 v1.1.0 之后的可选成熟度工作；Runtime v2 保持 Planned。以上开发证据都必须在选定冻结 SHA
> 后按发布阶段集中重跑或生成，不能直接授权 tag/Release。

# 发布与升级

这里保存长期有效的版本合同。Git tag、GitHub Release、嵌套 Go 模块的外部解析结果和对应提交上的验证报告共同构成发布证据；分支名、`Unreleased`、`planned`、`preview` 或本地 `go.work` 替换都不代表稳定版本。

## v1.0.0 stable

当前状态：**已于 2026-08-09 发布**。

这是合并仓库后的首个稳定 1.0 版本边界。根 `v1.0.0` 与先行发布的
`mss-boot/v1.0.0` 均指向
`ee800262c035c5f4242aca1841d077554481d2c4`。公开 Release 直接证明发布事实；验收工单
[#471](https://github.com/mss-boot-io/mss-boot-admin/issues/471) 记录精确提交、workflow、制品与发布后证据。独立
`web/antd/v1.0.0` tag 尚未发布。

- [发布合同](/releases/v1-0-0)
- [从 v0.7.x 升级](/releases/v1-0-0-upgrade)
- [兼容性矩阵](/releases/v1-0-0-compatibility)
- [回滚与恢复](/releases/v1-0-0-rollback)

本次稳定发布遵循了以下顺序，后续版本继续复用：

1. 在已评审的同一发布提交上完成全部验证；
2. 发布 `mss-boot/v1.0.0`；
3. 在仓库外、关闭 workspace 替换后验证嵌套模块可解析；
4. 再发布根标签 `v1.0.0`；
5. 对已发布制品执行禁用缓存的安装与运行冒烟。

首次 publication 前失败时保持 preview。Framework 已公开后若外部解析或 pre-root gate 失败，
该组件记录为 `component-partial / evidence-incomplete`，根标签不发布，并从下一更高同步补丁
forward-repair；根版本公开后的 reconciliation 失败则记录为
`published / evidence-incomplete`。两种情况都不得移动或删除标签，并停止后续发布直到 evidence
issue 完成终态记录或链接已验证的替代列车。

## 下一公开版本：v1.1.0

当前采用“开发优先、冻结后集中验证”策略：原 `v1.0.1-v1.0.3` 和 alpha 切片只作为
内部开发波次，不创建 tag、GitHub Release 或版本化包。Challenge、Kafka Mark/lifecycle、
Upload admission 与 D1 object provider/owner 已形成安全 checkpoint，将继续作为永久
回归哨兵。`D1-provider-owner` 已整体完成；Kafka 保持 `AdapterQueue` 兼容面并通过新增
`ManagedAdapterQueue` 由 Admin 统一拥有、作为 `Runnable` 运行和有界关闭，但仍保持
Legacy/Blocked。对象 Provider 采用维护者明确的 v1.1.0 窄范围：不启动 RustFS，不要求全面
Local/S3-compatible、authorization 或 Delivery 矩阵；若对象代码有改动，只做受影响编译、既有
focused owner/config 测试与一次基础 smoke。这是 scope 决策，不是新增测试通过声明。Local/S3
继续 Legacy/Blocked，production Local 与 S3 `Put`/未完成 Delivery 入口继续 unavailable；缺少可选
对象证据不阻断 Foundation 发布，也不会触发晋级。`D2-contract-substrate` 正在推进：canonical email 的迁移、模型、Admin 写边界和
server schema-readiness 已形成开发 checkpoint；冻结 SHA 上仍须重跑 readiness 正负套件以及
MySQL/PostgreSQL 双 DSN zero-skip evidence，因此 capability 保持 Planned。`151a91c` 还完成了
downstream snapshot identity consumer checkpoint：CLI/MCP/doctor 共用严格 SnapshotStatus，区分
精确 source sentinel、有效 generated pair 与 malformed/orphaned state，并阻止 nested Admin module
冒充 root module。该 checkpoint 只有 fully anchored 本地测试和 workflow 静态合同；真实 GitHub Actions
尚未运行。冻结 SHA 仍须真实执行 `foundation-compatibility.yml`，证明四身份/digests、Blueprint
0.1→0.2 定制保留和第二次空升级；pre-root 仍须 release-built external artifact。

`D3-backend-runtime` 已形成三个 Framework checkpoint：资源图在 `d90b4c7` + `c830b5f`
建立确定性生命周期，`c57ffc8` 阻止 provider 对象从公共 error tree 泄漏；`86c0e8a`
新增 additive named Redis Resource。22 项完全锚定的单包 race×20 evidence 证明构造、
Scope 隔离、caller deadline、lease drain 与 one-close，并使用 standalone miniredis 和
stalled socket；Sentinel/cluster/TLS 仍只有 construction matrix，不能据此晋级 capability。

`1faa9ef` 随后新增公共 `runtime/challenge` 与 internal opaque same-slot bridge。Challenge
不拥有 raw client 或 `Close`，rate replay 在 limit 边界保持幂等，所有语法有效 Verify 路径固定
执行一次 read 与一次 completion script；D0 exported surface 保持源码兼容并 Deprecated。
本次新增的 22 个顶级测试通过五条 fully anchored、uncached count1/race/GOWORK=off evidence。
Admin 尚未注入公共 API，也未运行真实 Cluster/failover，因此 capability 不自动晋级。

同一波次的 Generator/Blueprint 轴在 `5a60ad6` 形成 Supplier backend checkpoint，并在 `d92458c`
增加 `20260811120000` authorization migration：source spec 现在同时投影显式 entity migration、
model/DTO/service/API/OpenAPI/export、typed events、default-role policy、MENU/COMPONENT/API metadata、
role/global revision、AdminAuthorizer 与 exact allow/deny tests；没有 injected authorizer 时仍拒绝挂载路由。
SQLite 以及临时 MySQL 8.4/PostgreSQL 17 的 entity migration 开发运行已通过，冻结 SHA 仍须重跑。
typed client、UI/browser E2E、模块文档与 upgrade rehearsal 尚未生成，因此 full-stack capability 保持
Planned，generation plan 保持 `complete=false`，当前为 backend.3 的 19 managed / 19 deferred 真值。

只有 Generator/Blueprint 与 Storage Runtime 的 v1.1.0 选定范围完成、选定一个功能冻结 SHA
后，才手工启动 `release-readiness`，集中执行三数据库、browser、required Provider、upgrade、
external consumer、recovery、`verify --all` 与 `eval --all`。全部通过后才进入 Framework →
外部解析 → root → post-publication reconciliation。可选 ObjectStore/RustFS 矩阵不在该 required
集合中；缺失时保持 Provider 原成熟度即可。

`.mss/release-policy.yaml` 已在受保护 `release` environment、不可变 tag ruleset 与范围化阶段执行器
落地后切换为 `publicationWorkflowsReady: true`。该开关只表示发布设施可用，不授予 tag、Release 或 package
发布权；仍需先在完整冻结 SHA 上完成内置浏览器与范围化 feature-freeze evidence，再由独立的
`pre-framework` 和 `pre-root` authority run 批准实际发布。

- [v1.1.0 开发优先完整路线](/releases/v1-0-1-to-v1-1-0-roadmap)
- [Challenge 内部安全 checkpoint](/releases/v1-0-1-challenge-safety)
- [Kafka Mark-after-success 内部 checkpoint](/releases/v1-0-1-kafka-ack-safety)
- [Upload admission 内部 checkpoint](/releases/v1-0-1-upload-admission-safety)
- [D1 Object Provider/Owner 内部 checkpoint](/releases/v1-1-0-d1-object-provider-owner)
- [D2 Canonical Email Identity 内部 checkpoint](/releases/v1-1-0-d2-canonical-email-identity)
- [D2 Downstream Snapshot Identity 内部 checkpoint](/releases/v1-1-0-d2-snapshot-identity)
- [D3 Resource Lifecycle 内部 checkpoint](/releases/v1-1-0-d3-resource-lifecycle)
- [D3 Named Redis Resource 内部 checkpoint](/releases/v1-1-0-d3-named-redis-resource)
- [D3 Challenge Runtime 内部 checkpoint](/releases/v1-1-0-d3-challenge-runtime)
- [D3 Supplier Backend 与 Authorization 内部 checkpoint](/releases/v1-1-0-d3-supplier-backend)
- [D5 Scoped Runtime Cache 内部 checkpoint](/releases/v1-1-0-d5-scoped-runtime-cache)
- [D5 Revision EventBus 与 Admin reconciliation 内部 checkpoint](/releases/v1-1-0-d5-revision-eventbus)
- [D5 Blueprint 0.1→0.2 外部升级演练](/releases/v1-1-0-d5-blueprint-upgrade-rehearsal)
- [D5 Supplier 完整生成模块内部 checkpoint](/releases/v1-1-0-d5-supplier-generated-module)
- [D5 Provider evidence validator 内部 checkpoint](/releases/v1-1-0-d5-provider-evidence-validator)
- [v1.1.0 exact readiness attestation 合同](/releases/v1-1-0-exact-readiness-attestation)
