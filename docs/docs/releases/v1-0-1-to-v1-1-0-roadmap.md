---
title: v1.1.0 开发优先路线
order: 6
description: 从 v1.0.0 直接推进到 v1.1.0 功能冻结，再集中执行完整验证和发布门禁
keywords: [v1.1.0 roadmap development first feature freeze validation release]
---

> D3 进度（2026-08-11）：`d90b4c7` + `c830b5f` 已完成 Runtime v2 资源图，
> `c57ffc8` 阻止 provider 对象或文本从公共 error tree 泄漏；`86c0e8a` 进一步加入
> additive named Redis Resource，并以全部 22 个顶级测试、完全锚定的单包
> `mss test evidence`、20 次 uncached race run 固化 one-client、isolated Scope、caller
> deadline 与 exactly-once close。该证据只有 standalone miniredis、stalled socket 和
> Sentinel/cluster/TLS construction matrix，不是完整 Provider conformance；因此
> `platform.storage-runtime-v2` 仍为 Planned。Generator/Blueprint 轨在 `5a60ad6` 完成了
> Supplier backend checkpoint：显式三方言 forward DDL、DTO/service/API/OpenAPI/export、
> typed post-commit events 与 fail-closed authorizer 已生成并通过开发证据，但机器计划诚实保持
> `complete=false`，26 项 policy/menu/frontend/docs/E2E 投影继续延后。`1faa9ef` 又新增公共
> `runtime/challenge` 与内部 opaque same-slot fixed-script bridge，并验证 rate replay 与所有语法
> 有效 Verify 路径的固定 I/O；legacy D0 surface 保持源码兼容。`3e9ca94` 已将 named Redis
> `main` / Scope `challenge.email` 接入 Admin：Start/Ready 后才发布，Config 唯一持有并有界关闭，
> optional invalid/outage 固定 503 且不回退 legacy global Redis，required failure 阻断 setup；
> FakeCaptcha 使用 Begin→send→Commit/Abort，登录/注册/重置消费 VerifyOutcome。配置仍是启动快照，
> 变更需要重启。下一步继续 D4 Supplier policy/menu 与其余 D3/D5 功能；
> 真实 Provider 和 100 次 leak 门禁仍只在选定 feature-freeze SHA 后执行。详见
> [D3 Resource Lifecycle](/releases/v1-1-0-d3-resource-lifecycle) 与
> [D3 Named Redis Resource](/releases/v1-1-0-d3-named-redis-resource)、
> [D3 Challenge Runtime](/releases/v1-1-0-d3-challenge-runtime)、
> [D3 Supplier Backend](/releases/v1-1-0-d3-supplier-backend) 内部 checkpoint。

# v1.1.0 开发优先路线

基线是 2026-08-09 已发布的根版本 `v1.0.0` 与 Framework
`mss-boot/v1.0.0`，二者均指向提交
`ee800262c035c5f4242aca1841d077554481d2c4`。公开 Release 直接证明发布事实；验收工单
[#471](https://github.com/mss-boot-io/mss-boot-admin/issues/471) 记录精确提交、workflow、
制品、外部解析和发布后证据。

## 策略结论

从现在开始，开发节奏调整为：

1. **持续开发到 v1.1.0 目标完成**：原先称为 `v1.0.1`、`v1.0.2`、`v1.0.3`
   和 `alpha.1-alpha.5` 的内容改为内部开发波次，只用分支、提交和 checkpoint ID 标识；
2. **达到功能冻结后再集中验证**：Generator/Blueprint、Storage Runtime、数据库、浏览器、
   Provider、升级、恢复和外部消费矩阵只在一个冻结候选提交上形成完整发布证据；
3. **验证通过后才进入发布流程**：先做 pre-framework authority，再发布 Framework 并完成
   外部解析，随后做 pre-root authority、根发布和 post-publication reconciliation；
4. **下一个计划公开版本只有 `v1.1.0`**：不发布 `v1.0.x`，也不创建公开
   `v1.1.0-alpha/beta/rc` tag。SemVer 预发布 tag 仍是公开版本，不能把它称为内部 checkpoint。

机器策略位于 `.mss/release-policy.yaml`，聚合发布合同位于
`.mss/features/foundation-v1-1-0-release.yaml`。所有 tag 驱动的发布 workflow 都必须先验证该策略。
当前策略的 `publicationWorkflowsReady` 固定为 `false`：它允许开发期构建候选制品并运行 bootstrap qualification，
但会拒绝 Framework、root、frontend 与镜像的实际发布。该字段不是阶段审批锁；它只表示完整阶段执行器、证据
attestation、受保护写入 job 和 tag ruleset 已经具备强制能力。所有开发波次完成后，先补齐这些基础设施并评审
设为 `true`，然后才选择该提交作为冻结 SHA、启动完整 readiness。后续发布授权仍来自绑定同一 SHA 的阶段证据。
若出现不能等待 v1.1.0 的远程可利用漏洞或数据丢失风险，安全负责人可以发起紧急补丁；但必须先
通过评审修改 release policy 的公开目标，不能用手工参数或 workflow bypass 绕过。

这项决策显式接受一个风险：已在开发分支完成的 Challenge、Kafka Mark/lifecycle 和 Upload 修复不会立刻交付给
`v1.0.0` 用户。项目会持续评估其可利用性；一旦延迟不可接受，就启用上述紧急补丁例外。

## 两条开发主轴

开发仍保持两条并行主轴，但不在每个小切片建立发布出口：

- **A 轴：Generator / Blueprint 旗舰轴**：版本身份、migration engine、FeatureModule contract kind、
  supplier golden backend 与 OpenAPI 已形成 checkpoint；继续权限/menu、typed client、前端、
  三方升级与 generated drift；
- **B 轴：Storage Runtime 风险轴**：Provider fail-closed、资源所有权、named Redis、ChallengeStore、
  Cache、EventBus 与 v1.1.0 选定 Provider evidence；ObjectStore/Delivery 成熟度移到可选后续波次；
- **共同基座**：canonical identity、三数据库 forward migration、严格配置、SecretRef、Feature phase
  和发布策略。

Generator 是 v1.1.0 的产品旗舰。Storage Runtime 是独立风险轴：未完成的 Kafka、NSQ、WorkQueue、
Lock 或 S3-compatible Provider 可以保持 Legacy/Blocked/Experimental，而不是为了版本号被自动晋级；
但已选择进入 v1.1.0 的 required Provider 必须在冻结后通过自己的完整证据。

对象存储采用维护者明确的窄范围决策：现有实现视为可信边界，v1.1.0 不启动 RustFS，也不要求
Local/S3-compatible、application provider authorization 或 Delivery 的全面矩阵。若候选提交改动了对象
存储实现，只做受影响编译、既有 focused owner/config 测试与一次基础 fail-closed 或 dev-Local smoke；
未改动时没有对象存储专属冻结门禁。Local/S3 继续保持 Legacy/Blocked，生产 Local 与未完成的 S3
Delivery 入口继续 unavailable。缺少可选对象 Provider 证据不阻断 Foundation v1.1.0 发布，也不产生晋级。

## 内部开发波次

下表中的 ID 不是版本号，不得创建 Git tag、GitHub Release 或版本化包。

| 波次 | A：Generator / Blueprint | B：Storage Runtime | 退出信号 |
| --- | --- | --- | --- |
| `D0-safety`（已完成） | 建立 v1.1.0 Generator/Blueprint 合同 | Challenge 原子安全、Kafka Mark-after-success、Upload pre-parse admission 与 Local create-only confinement | 对应 focused/race 测试通过；Provider 状态仍诚实保持 |
| `D1-provider-owner`（已完成） | 冻结新增 scaffold 范围，以 `admin/modules/<name>` 作为机器合同、生成器和文档的唯一新增模块目标 | object provider 完成严格 startup profile、AppConfig 移除、单一 owner、dev-only Local Delivery 与 fail-closed 503；Kafka 保留 `AdapterQueue` 兼容面并新增 `ManagedAdapterQueue`，完成 caller-context 配置/注册、单 producer 与唯一 consumer-group owner、`Errors()` 观察、可取消 `Start` 和幂等有界 `Close`；Admin 是唯一 owner 并把它注册为 `Runnable` | object 与 Kafka exact owner/config/Admin 测试非零命中并通过；changed path 无 Exit/Fatal 或 detached long-lived work；Kafka 仍保持 Legacy/Blocked，不把 D1 完成解释为 Provider 晋级 |
| `D2-contract-substrate`（已完成开发 checkpoint） | FeatureModule contract kind；Foundation/Blueprint/generator/downstream snapshot 四身份；lock+manifest 原子双记录；CLI/MCP/doctor 共用严格 SnapshotStatus；typed migration ID 和 duplicate fail-fast | canonical email 存量冲突预检、三库唯一 forward migration 与启动 schema readiness；strict one-of config、SecretRef、doctor preflight | infrastructure Feature 可规划；source/generated/malformed 三态 fail closed；source→generated 竞态受同一锁协议保护；SQLite/model/API/schemahealth/composition checkpoint 非零命中；真实双 DSN 与真实 compatibility workflow 都保留为 feature-freeze required gate |
| `D3-backend-runtime`（进行中） | `5a60ad6` 已完成 supplier spec projection、显式 migration、model/DTO/service/API/operations/index/export/OpenAPI、typed events 与 fail-closed authorizer；计划保持 `complete=false` | 顶层资源图、additive named Redis、公共 Challenge API 与 internal opaque fixed-script bridge 已落地；`3e9ca94` 完成 Admin readiness/publication、Config bounded close 与 Challenge consumer 注入 | backend golden 两次生成零 diff且 exact evidence 非零；Framework Challenge 22 项与 Admin composition 13 项本次顶级测试均 count1/race/GOWORK=off 非零；100 次 race 启停、browser listener 复核与真实 Cluster/failover 留到冻结 SHA；listener 不早于 required resource ready |
| `D4-authorization-object` | permissions/defaultRoles/menu/ownership；成功与拒绝审计；完整正负授权，并复用 D3 已生成的 post-commit typed events | v1.1.0 只保持既有 object owner/config 与 fail-closed 边界；ObjectStore/Delivery、metadata、Local/S3 全面矩阵移到可选 post-v1.1 波次 | Supplier 权限矩阵全绿；对象代码若有改动则完成受影响编译、既有 focused owner/config tests 与一次基础 smoke；不启动 RustFS、不晋级 Local/S3，缺少可选证据不阻断 FF |
| `D5-frontend-events-upgrade` | typed client、list/form/detail/actions/export、双语 locale、完整 UI 状态；Blueprint 0.1→0.2 三方升级 | scoped cache、transaction-bypass QueryCache、Memory/Redis EventBus、same-tx revision/reconcile、provider evidence report | 第二次 upgrade 为空；无 ignored spec field；前端 focused checks 通过；非目标 Queue/Lock 状态锁定 |
| `FF-v1.1.0` | Generator/Blueprint schema、API、模板和 golden 输出冻结 | Runtime 配置、资源接口、Provider 选择与 maturity 候选冻结 | P0/P1 清零；不再接受新功能或公共合同变化；选择一个完整 SHA 进入集中验证 |

D1 的运行与验证边界见
[D1 Object Provider/Owner 内部 checkpoint](/releases/v1-1-0-d1-object-provider-owner) 和
[Kafka Mark/lifecycle 内部 checkpoint](/releases/v1-0-1-kafka-ack-safety)。

D2 的 canonical email 核心已形成开发 checkpoint：存量 active 用户先做不输出身份值的冲突预检，
SQLite/PostgreSQL 使用 active、non-empty partial expression unique index，MySQL 使用等价的 nullable
stored `VARBINARY` generated key；模型、邮件注册、首次 OAuth 建号和 Admin 写错误映射共用一个 typed
contract，且 OAuth 不按 provider email 合并既有账户。精确 SQLite migration/model/API evidence 已非零命中；
`8b361ce` 完成了首轮 schemahealth 正负验证，以及 migrate 后复验和 server 挂载业务路由前 fail-closed
两个 composition checkpoint；`1171df6` 进一步把验证与迁移全链固定到 DBResolver writer，并为 PostgreSQL
强制 deterministic `C` collation、为 MySQL 强制显式 ASCII canonical key，形成当前七项 schemahealth evidence。
这只证明当前开发切片，不是 feature-freeze 证据；全部 readiness tests 和
MySQL/PostgreSQL 真实集成必须在最终冻结 SHA 上重跑，并同时提供两个 allowlisted DSN，由 evidence runner
证明两条 required test 均 run/pass 且 zero-skip。边界和迁移说明见
[D2 Canonical Email Identity 内部 checkpoint](/releases/v1-1-0-d2-canonical-email-identity)。

`151a91c` 完成了 D2 下游快照身份 consumer checkpoint：CLI `upgrade status`、MCP status 与
doctor 的 `snapshot:foundation` 都读取同一个严格 SnapshotStatus，并同时输出 Foundation release、
Blueprint、实际 generator build 和 downstream snapshot 四身份以及 lock/manifest digest。根 module
从已安装 snapshot 恢复，不能误用 `spec.backend.module`；`spec.foundationVersion` 仍只是项目生成基线。
Foundation source 只接受精确的 legacy development sentinel；当前 generated pair 缺失、损坏、孤儿化或
正处于 source→generated 写入时，不会被误判为 source。fully anchored 的单包 `mss test evidence`
命令覆盖了 SnapshotStatus、竞态、CLI/MCP module deferral、doctor 三态和 workflow 静态合同。
这仍只是开发 checkpoint：没有真实运行 GitHub Actions。冻结阶段必须把真实
`.github/workflows/foundation-compatibility.yml` run 绑定到完整 SHA，并补齐真实 Blueprint 0.1→0.2、
业务定制保留、四身份/digest 一致和第二次空升级。边界见
[D2 Downstream Snapshot Identity 内部 checkpoint](/releases/v1-1-0-d2-snapshot-identity)。

`86c0e8a` 完成了 D3 named Redis development checkpoint：一个 resource name 只构造并拥有
一个 client，多个稳定 Scope 自动绑定 resource+scope 物理前缀，structured lease 继承 caller
context 并拒绝 retained/detached work，Close caller 可以超时但 provider Close 只执行一个 tracked
generation。`c57ffc8` 同时收紧底层资源图的 provider error tree。22 项 fully anchored 单包
race×20 evidence 包含 standalone miniredis 与 stalled socket，但 Sentinel/cluster/TLS 只做构造矩阵；
Sentinel control ACL 匿名、cluster multi-key 非原子 partial、Admin composition、真实 Provider、FD/
goroutine 仍未完成。边界见
[D3 Named Redis Resource 内部 checkpoint](/releases/v1-1-0-d3-named-redis-resource)。

`1faa9ef` 完成了 D3 Challenge Framework checkpoint：公共 `runtime/challenge` 只接收 named
Redis Scope，不导出 provider client、物理 key、任意 Eval 或 `Close`；内部 bridge 以服务端派生的
opaque same-slot group/key 运行固定 scripts。rate operation 在 limit 边界可安全 replay，所有语法有效
Verify 路径固定为一次 read、一次 completion 与 comparison work。D0 exported surface 保持源码兼容并
Deprecated。五条 fully anchored 单包命令只要求本次新增的 22 个顶级测试，各以 count1、race、
GOWORK=off uncached 通过。`3e9ca94` 随后把 named Redis `main` 与 Scope `challenge.email` 接入
Admin：graph Start/Ready 完成后才发布，Config 在 rollback 与正常 shutdown 中以有界 context 唯一关闭；
optional invalid/outage 让业务固定返回 503 而不使用 legacy global Redis，required failure 阻断 setup。
FakeCaptcha 采用 BeginIssue→SMTP→Commit/Abort，登录、注册、重置采用 VerifyOutcome；四条 fully
anchored Admin 单包命令选中本次新增或改动的 13 个顶级测试，全部 count1/race/GOWORK=off uncached
通过且无 skip。配置热变更仍需重启；browser reload、冻结 SHA lifecycle 与真实 Cluster/failover
尚未执行，能力未自动晋级。
边界见 [D3 Challenge Runtime 内部 checkpoint](/releases/v1-1-0-d3-challenge-runtime)。

`5a60ad6` 完成了 A 轴 D3 Supplier backend checkpoint。AdminModule 的每个 source field 现在都有
implemented、validation-only 或 deferred output-kind；显式 migration ID `20260810160000` 生成并验证
SQLite/MySQL/PostgreSQL DDL，DTO/service/API/OpenAPI/export 与 typed post-commit events 已落地，
route composition 在没有 injected authorizer 时 fail closed。生成器还会删除带 marker 的旧 auto-mounted
输出，并在任何写入前拒绝 user-owned 冲突。SQLite hermetic suite 与临时 MySQL 8.4/PostgreSQL 17
开发运行均通过；它们不是冻结证据。计划仍是 `phase=backend-checkpoint`、`complete=false`、15 个
managed output unchanged、26 项 deferred；defaultRoles/policy/menu、typed client、UI/E2E、模块文档和
upgrade rehearsal 分别留给 D4/D5。边界见
[D3 Supplier Backend 内部 checkpoint](/releases/v1-1-0-d3-supplier-backend)。

波次可以并行开发，但依赖不能倒置：migration engine 必须先于真实生成迁移和 object metadata；严格
profile 必须先于 owned runtime；named Redis 必须先于 Challenge/Cache/EventBus；完整 golden 输出必须先于
Blueprint 升级 rehearsal。一个提交必须同时包含行为、测试和合同更新，不能依赖后续提交恢复编译或
安全不变量。

## 开发期间保留的最小检查

“冻结后集中验证”不等于开发期间不测试。每个可合并波次只保留防止错误继续扩散的快速检查：

- 受影响包可编译；Framework 使用 `GOWORK=off`；前端变更运行 `tsc` 和 focused Jest；
- 相关 FeatureSpec 执行 `mss spec validate`；当前该命令只证明结构和引用，不代表 acceptance 已执行；
- 精确 focused tests 必须真实出现 `run/pass`；并发、取消和资源所有权代码至少运行一次 `-race`；
- Generator 始终保留 dry-run、路径限制、unsupported-before-write、稳定排序和已存在 golden 的 `--check`；
- Challenge fail-closed/exactly-once、Kafka failure 不 Mark、Upload pre-parse/no-clobber、Provider 零
  fallback、secret redaction、权限负例是永久安全哨兵；
- `gofmt`、`git diff --check`、受影响 generated drift 和 `mss verify --changed --plan`。

普通 PR 不再自动运行 `release-readiness` 或 `eval run --all`。常规 CI 继续承担编译、单元测试、
changed-scope 合同和生成器快速反馈；完整矩阵由人工在功能冻结后启动。

## 功能冻结后的集中验证

冻结候选必须是一个完整 SHA。任何实现、迁移、公共 API/配置/schema、生成模板或 required evidence
变更都会产生新候选，并使受影响的旧证据失效。

当前（2026-08-10）GitHub API 审计显示仓库尚无受保护的 `release` environment，repository ruleset 也为空；
现有 `release-readiness` 因此明确标记为 `bootstrap-incomplete`，不构成 publication authority。进入正式冻结前必须先：

- 把三数据库、Browser E2E、真实 Provider/broker、100 次生命周期、恢复演练和 exact run/pass/no-skip 汇总为阶段执行器；
- 让下游按指定 run ID 验证 version、SHA、phase、policy hash 与完整矩阵 attestation，而非只查询任意绿色 run；
- 创建带独立 reviewer 的受保护 `release` environment，并把所有 write job 与 package push 移入该环境；
- 建立限制 `v*`、`mss-boot/v*`、`web/antd/v*` 创建、更新、删除主体的 enforced ruleset；
- 评审 `publicationWorkflowsReady: true`，再把这个新提交选为冻结候选。

### 阶段 1：Feature freeze qualification

人工运行 `.github/workflows/release-readiness.yml`，显式输入 `v1.1.0`、完整 SHA 并确认 feature freeze。
至少完成：

- `doctor --strict`、`verify --all`、`eval run --all`；
- Framework/Admin/Agent 全量测试、race、vet、frontend lint/tsc/Jest/build、docs build；
- SQLite/MySQL/PostgreSQL fresh/upgrade/repeat/failure migration matrix；
- Redis standalone/Sentinel/cluster/TLS、选定的 required 真实 broker 与故障注入；application ObjectStore/RustFS 明确不在 v1.1.0 required 矩阵；
- Browser E2E、权限正负矩阵、secret/subject/payload redaction canary；
- Generator 两次生成、全字段投影、external new-app、Blueprint 0.1→0.2 升级和二次空升级；
- `foundation-compatibility.yml` 从冻结 SHA 真实运行，CLI/MCP/doctor 四身份与 digests 一致，
  并把 0.1→0.2、定制保留、确定性冲突和第二次空升级写入 evidence artifact；
- 资源 100 次启停、泄漏、readiness、shutdown、恢复 rehearsal；
- 所有 required exact tests 非零命中，且无 skip、cached-only 或 `[no tests to run]`。

### 阶段 2：Pre-framework authority

建立仍保持 open 的 evidence issue，记录版本、完整 SHA、release policy hash、SemVer 决策、完整验证
报告、候选制品 manifest、恢复计划与审批。此阶段通过前不能创建任何公开组件 tag。
仓库必须保持受保护的 `release` environment（独立 reviewer），并用 ruleset 限制 `v*`、`mss-boot/v*`、
`web/antd/v*` 的创建、更新和删除；workflow guard 只能阻止 Release/package 写入，不能撤回一个已经公开的 Git tag。

### 阶段 3：Pre-root authority

从同一 SHA 发布 `mss-boot/v1.1.0`，随后在仓库外临时模块中以 `GOWORK=off` 解析并消费；确认
Admin 精确依赖、checksum 和完整 SHA 后，才批准根 `v1.1.0`。若 Framework 已公开而本阶段失败，
记录 `component-partial / evidence-incomplete`，根版本不发，任何 tag 不移动，并以前向版本修复。

### 阶段 4：Post-publication reconciliation

根 tag/Release 发布后验证 ZIP、checksum、容器 digest、禁缓存安装烟测、运行时、文档、changelog、
Feature/capability 状态和恢复证据。全部一致后才能关闭 issue；失败时版本保持
`published / evidence-incomplete`，停止后续发布并前向修复。

## SemVer 与身份边界

从公开 `v1.0.0` 直接到 `v1.1.0` 符合 SemVer，不需要先公开连续 patch。评估和目标设计不受旧 API、
旧配置或旧 global 约束，但发布时必须诚实分类：若 `mss-boot` 删除或更改公共 Go API 且不能用
非权威 bridge 隔离，Framework 需要 major 版本。当前同步列车要求根版本、`mss-boot/<version>`、
Admin 依赖和提交一致；要发布 root 1.1 + Framework 2.0，必须先建立 component-version mapping，不能
临时绕过 workflow。

`.mss/project.yaml` 的 `foundationVersion` 和源码仓库 `.mss/lock.yaml` 的 `0.1.0/development` 是当前
项目生成基线与精确 source sentinel，不是公开 release target。本路线不手工改号。`151a91c` 已让工具
从 release policy、Blueprint digest、实际 binary build 与 downstream 输入分别建立四身份，原子写入
snapshot 的 lock/manifest，并由 CLI/MCP/doctor 共用严格读取；稳定晋级仍取决于冻结 SHA 上的真实
compatibility workflow 与 pre-root release-built external artifact。

## 社区 issue 的纳入方式

- [#374 SQL migration scripts and rollback](https://github.com/mss-boot-io/mss-boot-admin/issues/374)
  进入 `D2` migration engine 与 `D3` 真实 supplier forward migration；
- [#53 手机号登录](https://github.com/mss-boot-io/mss-boot-admin/issues/53) 必须复用新的
  ChallengeStore purpose、rate limit、anti-enumeration 和 fail-closed 语义，不复制验证码状态机；
- [#111 通知多 Provider](https://github.com/mss-boot-io/mss-boot-admin/issues/111) 只有在 typed
  provider profile 和独立 delivery contract 之后再评估；
- [#471 v1.0.0 exact-main evidence](https://github.com/mss-boot-io/mss-boot-admin/issues/471)
  作为 v1.1.0 evidence issue 的历史模板，但历史结果不能复用为新版本证据。

## 下一条可执行工作

`D1-provider-owner` 已整体完成。对象路径在启动时构造严格 immutable profile，由单一 owner
持有 client；无效或不可用资源固定返回 503 且零 Local fallback；Local 只在 dev 模式与实际
`staticPath` 精确映射时安装；Provider/SecretRef 已移出 AppConfig。Kafka 路径在保留旧
`AdapterQueue` 编译兼容面的同时增加 `ManagedAdapterQueue`，由 caller context 构造/注册，单一
producer 与唯一 topic/group consumer client 均有明确 owner，provider error 可观察，`Start` 可取消，
`Close` 幂等、有界且超时后可重试；Admin Config 是唯一 owner，server lifecycle 把 managed queue
作为 `Runnable` 运行。它仍是 Legacy/Blocked；manual commit、retry/backoff、DLQ、idempotency、
rebalance、outage 与真实 broker conformance 尚未证明。

`D2-contract-substrate` 已完成 canonical FeatureModule 路径、typed/lossless migration ID 与 duplicate
fail-fast、strict runtime config/SecretRef、Foundation/Blueprint/generator/downstream snapshot 四身份与
lock+manifest 原子双记录，以及 canonical email 的模型/迁移/API/schema-readiness 开发 checkpoint。
其中 `151a91c` 已收口四身份在 CLI/MCP/doctor 的 consumer，并为 source/generated/malformed 三态和
source→generated 竞态提供精确测试；真实 GitHub Actions 尚未运行。继续推进后续功能波次；选择
feature-freeze SHA 后，必须从该提交重跑 canonical-email schemahealth/composition
和 MySQL/PostgreSQL 两个真实 DSN 且 zero-skip，不能复用 `eb1277c`、`8b361ce` 或 `1171df6` 的开发运行冒充冻结证据。
同一 SHA 还必须真实运行 `foundation-compatibility.yml`，证明四身份/digest、Blueprint 0.1→0.2 与第二次
空升级；静态 workflow contract test 不能冒充这项证据。
D2/D3 期间不插入 v1.0.x 发布准备、RustFS qualification 或全量
release-readiness。S3 `Put`、Delivery 与完整 Local/S3 conformance 已移到可选 post-v1.1 波次，
不再是 D4 或 feature-freeze 前置。D3 Admin Challenge composition 已在 `3e9ca94` 完成；运行时配置
仍是 immutable startup snapshot，修改资源、scope 或 SecretRef 后必须重启。下一条可执行工作是继续
剩余 D3/D5 Runtime 范围；A 轴随后进入 D4，为已生成的 Supplier permission code 增加 default-role/
policy/menu persistence 和完整正负授权。真实 Sentinel/cluster/TLS 与 leak evidence、Supplier 三方言
重跑都留给冻结 SHA，开发期证据不能复用为发布证据。

本策略调整完成的定义是：release policy 能拒绝 v1.0.x 和公开 prerelease tag；Feature acceptance 能按
checkpoint、feature-freeze、pre-framework、pre-root、post-publication 聚合；普通 PR 不自动运行完整
release readiness；路线、FeatureSpec、ADR、capability 和 checkpoint notes 都不再承诺中间版本发布。
