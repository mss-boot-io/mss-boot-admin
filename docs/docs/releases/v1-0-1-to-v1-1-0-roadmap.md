---
title: v1.0.1 至 v1.1.0 迭代路线
order: 6
description: 基于 v1.0.0 真实基线，以安全、版本合同、Storage Runtime 与全栈生成器为主线的小步发布计划
keywords: [v1.0.1 v1.1.0 roadmap storage runtime generator release]
---

# v1.0.1 至 v1.1.0 迭代路线

基线是 2026-08-09 已发布的根版本 `v1.0.0` 与 Framework
`mss-boot/v1.0.0`，二者均指向提交
`ee800262c035c5f4242aca1841d077554481d2c4`。根 Release、Framework
Release 直接证明发布事实；验收工单
[#471](https://github.com/mss-boot-io/mss-boot-admin/issues/471) 记录精确提交、workflow、
制品、外部解析和发布后冒烟证据。

本次交付的是**机器可解析、声明式**规划合同：当前 `mss spec validate` 只校验结构与引用；
`mss feature plan` 仍把无 `specPath` 的 cross-cutting module 错当成 AdminModule，`mss verify`
也不执行 `validation.custom`、acceptance evidence 或发布 phase。它们在对应 planner/runner 落地前
不得被报告为已执行或已通过。`v1.0.1` 的 evidence 切片补 phase-aware release runner，
`v1.0.2` 再补 FeatureModule contract kind 与 infrastructure planning。

本路线采用“小步快跑、证据晋级”：日期是目标窗口，不是跳过门禁的承诺。发布前 must
证据缺失时版本顺延；发布后 required evidence 只在 tag/Release 存在后验收，它不反向成为
首次发布的前置条件，但在通过前版本处于 evidence-incomplete 状态，不能关闭 evidence issue、
宣称证据完整或推进下一个版本。

## 排序结论

后续不是“先做完 Storage、再做 Generator”的串行路线，而是一个安全前置加两条并行轴：

1. **共同前置（v1.0.1）**：OTP、Kafka、上传和 provider fallback 的已知安全/数据完整性
   路径先止血，并建立发布后 truth reconciliation；目标窗口放宽为两周，不用日期压缩证据。
2. **A 轴：Generator / Blueprint 主线**：从 `v1.0.2` 的版本身份、migration engine、
   canonical path 与字段投影开始，每个 prerelease 都交付可验证的 supplier golden slice，
   这是 `v1.1.0` 的产品旗舰。
3. **B 轴：Storage Runtime 风险轴**：从严格配置和资源所有权逐步进入 named Redis、
   ChallengeStore、ObjectStore、Cache 与 EventBus；每个 Provider 独立晋级，未通过的
   Kafka/NSQ/WorkQueue/Lock 保持 legacy/experimental，不拖延 Generator 主线也不伪装 stable。
4. **共同发布轴**：同一版本只有在 A/B 当期 prepublication required gate 与当前发布阶段的
   authority evidence 交集通过后才进入下一阶段；postpublication reconciliation 负责关闭证据，
   非目标可选 Provider 以“不晋级”收口，而不是为了赶版本放宽门禁。

当前只有一份
`example-supplier` 规格，且 operations、export、ownership、events、test flags、索引组、
permissions、defaultRoles、menu 等字段尚未全部成为生成结果；在一条 golden module
完整闭环前，不扩张更多业务能力。

## 版本梯队

| 版本 | 2026 目标窗口 | A：Generator / Blueprint 主线 | B：Storage Runtime 风险轴 | 发布出口 |
| --- | --- | --- | --- | --- |
| `v1.0.1` | 08-10 至 08-21 | 冻结新 scaffold 范围；建立 full-stack Feature 合同、真实 capability 拆分与 canonical path 决策 | Challenge 原子安全和 Admin purpose/anti-enumeration；Kafka ack-after-success；两个上传入口 pre-parse hard limit；object provider fail-closed | `acceptance.phase` 与 phase-aware release evidence runner；精确 required tests 非零命中；Framework 先发且外部可解析；发布后 truth reconciliation |
| `v1.0.2` | 08-24 至 09-04 | FeatureModule contract kind 与 cross-cutting infrastructure plan；Foundation release/Blueprint/generator/downstream snapshot 四种身份，lock+manifest 原子记录同一 snapshot；lock/sync/check；typed migration ID、duplicate fail-fast、error return；字段到 output-kind 矩阵与 supplier golden fixture | strict one-of config、SecretRef、doctor preflight；移除 v1.0.1 changed paths 之外其余 Provider 的 Fatal/Exit/background/ghost clients；NSQ duration；Hermetic Redis/MinIO/broker fixtures | 两轴 focused/race 全绿；infra `feature plan` 成功且 AdminModule 缺 specPath 仍失败；两次生成零漂移；矛盾配置零副作用；migration 不再截断、吞错或退出进程 |
| `v1.0.3` | 09-07 至 09-18 | canonical `admin/modules` 全合同对齐；Blueprint 0.1→0.2 ownership/upgrade fixture；supplier forward migration 骨架 | storage 配置升级、provider failure matrix、外部 `GOWORK=off` resource consumer | SQLite/MySQL/PostgreSQL fresh/upgrade/repeat/failure 与 provider matrix 全绿，无 required skip |
| `v1.1.0-alpha.1` | 09-21 至 10-02 | supplier migration、model/DTO/service/API/operations/index/export/OpenAPI；不支持字段 pre-write 拒绝 | owned resource lifecycle、named Redis、required readiness、逆序 close、ChallengeStore target API | golden backend/check 两次零 diff；100 次 race 启停无泄漏；listener 不早于 required resources ready |
| `v1.1.0-alpha.2` | 10-05 至 10-16 | permissions/defaultRoles/menu、ownership、commit-after-transaction events 与完整正反授权 | ObjectStore/Delivery、Admin metadata migration、owner/tenant permission、Local/MinIO create-only/no-clobber/checksum、S3 bootstrap 独立 | 权限矩阵全绿；Local/MinIO 共用 suite；并发同 ObjectRef 返回 conflict；错误 provider 零 fallback |
| `v1.1.0-alpha.3` | 10-19 至 10-30 | typed client、list/form/detail/actions/export、locale、loading/empty/error/denied/conflict 与 frontend tests | scoped cache、transaction-bypass QueryCache、Memory/Redis EventBus、same-tx revision/reconcile；Lock/WorkQueue 晋级或延后决策 | frontend lint/tsc/Jest/build/E2E；Casbin crash-window 最终收敛；缓存不破坏事务/DB 权威；Queue/Lock 不搭便车 |
| `v1.1.0-alpha.4` | 11-02 至 11-13 | Blueprint 0.1→0.2 三方升级保留业务定制；全字段投影、generated drift、external new-app identity | provider evidence schema/report、standalone/Sentinel/cluster/TLS 与 Local/MinIO conformance、外部 resource consumer | 第二次 upgrade 空；无 ignored spec field；Provider 报告逐项且 required test 零 skip |
| `v1.1.0-alpha.5` | 11-16 至 11-27 | golden module 端到端修复、文档、示例和 Generator/Blueprint API freeze | Admin runtime E2E、故障注入、soak 预检；非目标 Provider 明确保持 legacy/experimental | 两轴所有 P0/P1 清零，schema/API freeze，全量 verify/eval 通过 |
| `v1.1.0-beta.1` | 11-30 至 12-04 | downstream rehearsal 与生成制品冻结 | provider maturity 冻结 | 新功能冻结；全量 DB/browser/provider/external matrix 无 required skip |
| `v1.1.0-rc.1` | 12-07 至 12-11 | 精确提交 golden/upgrade 证据 | 精确提交 provider/恢复证据 | Framework/root 产物、容器、ZIP/checksum、备份恢复与 SemVer 决策齐全，进入至少 7 天 soak |
| `v1.1.0` | 最早 12-18 | 发布已验收 full-stack golden module 与 Blueprint | 仅发布证据达标的 Provider；其余状态不变 | RC 同提交或仅含已复验阻断修复；Framework 先发布并外部解析，再发布根 Release |

## v1.0.1 可执行切片

`v1.0.1` 不做整包重写，按可独立评审和回滚的顺序拆为五个切片：

| 顺序 | 切片 | 代码边界 | 首个失败测试 | 完成信号 |
| --- | --- | --- | --- | --- |
| 1 | Challenge safety | `verify_code.go` 及 email login/register/password reset 调用链；只落 internal/provisional 安全状态机，公开 Storage Runtime v2 API 留到 alpha.1 | 100 并发 Verify 同一正确码成功不超过一次；错码不立即烧毁；发送失败和过期发送结果不改写较新的 challenge；crash/hung sender 的 pending lease 可回收且不延长旧 active；Redis outage fail closed | crypto/rand、purpose/subject HMAC、versioned Begin/Commit/Abort/Verify、pending lease、cooldown/quota/max-attempt 全过 race |
| 2 | Kafka delivery | Kafka consumer 的 decode/handler/commit 顺序 | decode/handler failure 不 Mark；cancel 后不提交在途消息 | ack-after-success 与 cancel 语义通过；provider 仍为 experimental，不在补丁版承诺 retry/DLQ 完整平台 |
| 3 | Upload admission | HTTP upload API 与 `admin/service/storage.go` | limit+1 在 multipart parse 前返回 413，临时目录/对象无残留 | body hard limit、stream max+1、local random key/no-clobber/path/symlink confinement；S3 create-only 留到 alpha.2 共用 Provider suite |
| 4 | Provider fail-closed | AppConfig storage profile 与 `mss-boot/pkg/config/storage.go` | unknown/unreadable/partial credentials 不写 local、不返回 success URL | immutable profile；S3 client 复用；local 显式启用且实际可 delivery |
| 5 | Evidence/release | Feature phase schema、phase-aware runner、`.mss`、docs、workflow、release issue | phase 只阻断当前状态转换，post evidence 不循环阻断首次发布；aggregate stable contract 被拒绝；required skip 阻断；release dispatch 不再默认 `v1.0.0` | 机器目录按 Provider 拆 identity/lifecycle status，ADR 记录详细 evidence maturity；开放并批准 exact-commit issue，发布后补证据再关闭；tag/Release/changelog/docs/capability 自动一致 |

每个切片遵守：测试先描述失败、实现最小安全行为、focused race、affected module、
`mss verify --changed`。前一切片不必等待大分支合并后才开始下一切片，但 release 分支只接收
已独立审查且可复验的提交。

## Storage Runtime v2 的门禁

目标架构见 [Storage Runtime v2 ADR](https://github.com/mss-boot-io/mss-boot-admin/blob/main/docs/adr/2026-08-09-storage-runtime-v2.md)，
机器验收合同是 `.mss/features/storage-runtime-v2.yaml`。

Provider 使用独立证据状态，不因 `v1.1.0` 自动变 stable：

- **Blocked**：已知安全/丢数据路径，默认不可启用；
- **Experimental**：只有构造/校验证据，明确非生产；
- **Beta**：统一 conformance、race、真实依赖、failure injection、lifecycle/leak、配置负例、metrics/docs 均通过；
- **Stable**：再增加受支持部署矩阵、soak/SLO、upgrade/rollback、外部 consumer、零 required skip 和精确发布证据。

`v1.1.0` 的稳定目标只包括证据通过的 named Redis、scoped cache、ChallengeStore、
Memory/Redis EventBus 和 Local/S3-compatible ObjectStore。QueryCache 可以继续 beta；Kafka、
NSQ、Redis WorkQueue 和 Redis Lock 不属于必须晋级项。

## Generator 黄金竖切

机器验收合同是 `.mss/features/foundation-v1-1-0-generator-blueprint.yaml`；它与
Storage Runtime v2 的 FeatureSpec 独立演进，并在每个版本出口汇合 required gate。

`v1.1.0` 的 Generator 目标不是“支持更多 YAML 字段”，而是一个可重复生成、可升级、可验证的
业务模块：

```text
AdminModule spec
  -> forward SQL migration
  -> backend model / dto / service / api
  -> RBAC / defaultRoles / menu
  -> OpenAPI contract and frontend client
  -> list / form / detail / actions / locale
  -> unit / integration / authorization / E2E
  -> docs and generated-drift report
```

数据库迁移设计直接承接社区 RFC
[#374](https://github.com/mss-boot-io/mss-boot-admin/issues/374)。在 forward migration、
升级路径和回滚边界明确前，不能把 AutoMigrate 或只有 fresh schema 的生成结果称为完整模块。
在 Generator alpha 之前，`v1.0.2` 先修复 migration engine：迁移 ID 不再依赖文件名前
13 个字符的截断和静默整数转换，重复 ID 必须 fail-fast，库代码只返回 error 而不执行
`os.Exit`/`Fatal`，并由 `v1.0.3` 的 SQLite/MySQL/PostgreSQL fresh、upgrade、repeat 与
failure matrix 证明。

目标路径先以当前机器合同和实现使用的 `admin/modules/<name>` 为准，修正文档与工具之间的
冲突；是否在 Blueprint 0.2.0 迁移到根 `modules/<name>`，必须另做一次路径 ADR 和确定性
downstream upgrade，不能在生成器里同时支持两个“默认”位置。

## 社区 issue 的纳入方式

截至基线日，仍开放且有明确产品含义的 issue 数量不多，多数处于 `needs-rfc` 或
`queue/discussion`：

- [#53 手机号登录](https://github.com/mss-boot-io/mss-boot-admin/issues/53)：先复用
  v1.0.1 internal challenge state 的 purpose、rate-limit、anti-enumeration 与 fail-closed
  语义，并在 alpha.1 收敛到 ChallengeStore target API；不得在
  `v1.0.1` 的 P0 修复之前再复制一套验证码逻辑。
- [#111 通知多 Provider](https://github.com/mss-boot-io/mss-boot-admin/issues/111)：进入
  `v1.1.0` 后续候选，但必须基于 typed provider profile 和独立 delivery contract；不能借
  Queue 聚合接口扩张。
- [#374 SQL migration scripts and rollback](https://github.com/mss-boot-io/mss-boot-admin/issues/374)：
  从 `v1.0.2` 的 migration engine 合同开始，进入 Generator alpha.1 的真实 forward migration
  与后续 Blueprint 0.1→0.2 upgrade must 范围。
- 已关闭的 [#471 v1.0.0 exact-main evidence](https://github.com/mss-boot-io/mss-boot-admin/issues/471)：
  作为 `v1.0.1+` 每次稳定发布的证据工单模板，而不是一次性历史附件。

周报类 issue 只作为信号汇总，不直接进入版本承诺；新社区请求必须先映射到现有 capability、
安全边界和维护 owner，再决定进入哪个版本。

## 版本与兼容边界

评估和目标设计不以旧 API、旧配置或旧 global 为约束。实施阶段仍必须诚实处理版本号：

- `v1.0.x` 只承载安全修复、严格失败行为、合同和增量基础设施，不故意删除稳定公共 Go API；
- `v1.1.0` 可以新增干净 package 并把旧入口标为 legacy，但如果必须删除/改签名而无法用
  非权威 bridge 隔离，就应把 Framework 发布目标改为 `mss-boot/v2.0.0`；
- 当前 release workflow 强制根版本、`mss-boot/<version>`、`admin/go.mod` 依赖版本与提交
  全部相同，所以 `v1.0.1` 和非破坏性的 `v1.1.0` 仍采用同步版本列车；
- 不允许为了守住路线表而把破坏性 Framework artifact 错标为 minor。若必须采用
  `mss-boot/v2.0.0` 而根版本保持另一条版本线，必须先实现 component-version release
  manifest、workflow 映射和对应 Admin 依赖校验，不能临时绕过现有门禁。

这条约束只保证发布标签真实，不改变 Storage Runtime v2 的理想目标结构。

## Release 门禁与节奏

每个 stable 或 prerelease 都从一个冻结提交产生，`required` 按阶段解释，不能形成“先有
Release 才能授权 Release”的循环：

1. **Pre-framework authority**：相关 FeatureSpec、实现、安全、Admin、Docs、制品构建和
   recovery 门禁在冻结提交上通过；开放的 evidence issue 记录明确版本、完整 SHA、命令并获批准；
2. **Pre-root authority**：先发布 Framework tag/Release，在仓库外空模块完成 `GOWORK=off`
   解析与消费，确认 Admin 依赖、checksum 与完整 SHA 后，仍在开放 issue 中批准根发布；
3. **Post-publication reconciliation**：发布根 tag/Release 后验证 checksum、容器、安装冒烟、
   changelog、docs、Feature/capability 状态与恢复证据，再补最终评论并关闭 issue。

若第二阶段失败，已公开 Framework 标记为 **component-partial / evidence-incomplete**：根版本
不得发布、issue 保持开放、任何标签不得移动或删除；在当前同步版本列车下跳过 root `v1.0.1`，
从下一更高补丁（例如 Framework/root `v1.0.2`）在新冻结提交上 forward-repair 并重新走完整列车，
同时把孤立 Framework 版本到“无对应 root”的映射写入原 evidence issue。若第三阶段失败，
已公开根版本标记为 **published / evidence-incomplete**，同样停止下一版本并执行 forward repair
或 recovery。两类失败都只能在同一 issue 完成终态记录或链接替代列车后关闭，不得伪造完整证据。

`v1.0.1` 起，手工 release dispatch 必须显式输入目标版本，不能继续使用已发布且不可变的
`v1.0.0` 默认值。到 `v1.1.0` 前，把仍以 `v0.7 Upgrade`、`Theme Settings` 等历史里程碑
命名的硬编码 workflow 依赖改成 capability gate 清单，并新增 Storage Runtime、Blueprint 与
Generator 的独立 required checks；保留有价值的回归测试，但不让历史名称定义未来发布范围。

推荐发布频率是补丁每 1–2 周、alpha 每两周、beta/RC 按证据推进。若一个版本的唯一主题
膨胀，应拆下一个版本，不把未验证范围藏在同一标签里。

## 明确不手工改写的状态

`.mss/project.yaml` 的 `foundationVersion` 与 `.mss/lock.yaml` 当前 `0.1.0/development`
记录的是现有 Blueprint/生成基线，不是 GitHub Release 公告。当前实现还混用了 Foundation、
Blueprint 与 generator 版本，因此本路线不把它们直接改成 `1.0.0` 或 `1.0.1`。

`v1.0.2` 必须先建立确定性的 version/lock/sync/check 流程、字段语义和测试，再由工具写入真实
状态。手工把数字改得“看起来一致”会制造新的 downstream upgrade 假证据。

## 下一条可执行工作

创建 `v1.0.1` 的第一个实现 PR：只包含 internal/provisional challenge state 的失败测试与安全修复，
不在补丁版冻结公开 Storage Runtime v2 API。完成条件是
并发 Issue/Verify、cooldown、quota、max attempts、purpose isolation、expiry、Redis outage、
anti-enumeration 和 redaction 全部通过 `-race -count=20`；该 PR 不同时修改 Queue、ObjectStore
或 Generator。另一条独立分支可以并行启动 `v1.0.2` 的 Foundation release、Blueprint、
generator、downstream snapshot 四身份失败测试，以及 lock+manifest 原子双记录在失败时保留旧
snapshot 的测试和 field-to-output 投影测试；两条泳道分别评审、分别合并，不互相隐藏门禁。
