# LiteLLM 计费运营模块设计（立项）

- Status: Accepted / Implementation（2026-09-10 扩展运营闭环）
- Date: 2026-09-08
- Baseline: `web/antd-v6/v1.3.7`（commit `77b53d41`，前端 1.3.7 发布线）
- Owners: 文祥
- Target release: 下游运营项目自用，不参与上游 mss-boot-admin 发布线
- 实现分支：`litellm-ops/design`（下游运营项目 worktree）

## 1. 背景与问题

cliproxy 网关（LiteLLM 1.100.0 + CLIProxyAPI）已完成公网邀请制上线：用户预算、Key 限速、按官方单价计量计费、SpendLogs 账目均已验收（见 `cliproxy/LAUNCH.md`）。

2026-09-08 运营能力评估确认的缺口：

1. 充值只能由管理员手工调 API/改库，无单据、无审计、无幂等。
2. 支付与自动充值未接入，用户无法自助充值。
3. 无结算/对账产物（结算单、导出、口径说明）。
4. 无监控告警（异常靠肉眼发现，当日已发生一次）。
5. 备份只在本机，无异机灾备；单物理节点。

本模块覆盖 1–3（管理端计费运营）。4–5 属基础设施轨道，另行立项，不在本设计范围。

## 2. 目标与非目标

目标：

- G1：在 mss-boot-admin 内统一查看 LiteLLM 用户/Key/额度（只读同步视图）。
- G2：充值管道：用户页直充或销售订单加额 → 执行 → LiteLLM 生效 → 回读校验 → 审计流水；全程使用整数微美元、幂等记录和绝对目标恢复。Key 消费清零属于管理指令，周期重置尚未开放。
- G3：按用户/时间/模型查询 SpendLogs 账目。结算单、导出和按配置单价重算属于后续里程碑，当前版本不提供对应接口。
- G4：支付接入预留：充值执行器接口化，后续接支付网关不改主流程。
- G5：在运营端提供经过权限控制且可恢复的 LiteLLM 用户、Key 常用管理动作，并提供模型可用性、组织/团队的只读视图；模型、组织和团队写操作以及敏感配置仍走 GitOps 或 LiteLLM 原生管理端。
- G6：建立闲鱼商品映射、订单录入、充值执行、对账与退款复核闭环；首期可手工运营，自动入单仅接受闲鱼官方开放平台或明确授权的连接器。

非目标：

- 不自建用户源。LiteLLM 是用户与 Key 的唯一权威，本模块只读同步、回写只走 Admin API。
- 不修改 LiteLLM 原始账目（SpendLogs 只读）。
- 不做 C 端自助中心。用户自助继续使用 LiteLLM UI；支付自助充值属后续里程碑。
- 不覆盖监控告警、灾备、高可用（见第 1 节分工）。
- 不抓取“我的闲鱼”浏览器 Cookie，不调用未公开私有接口，不以页面自动化充当生产订单源。闲鱼网页当前无法查看“我卖出的”完整订单，浏览器登录态也不是服务端凭据。

## 3. 现状事实（设计依据，2026-09-08 核实）

LiteLLM 侧：

- 版本固定 1.100.0，2 副本，Helm revision 10；管理 API 使用 master key（K8s Secret `litellm-masterkey`）。
- 集群内地址 `http://litellm.litellm.svc.cluster.local:4000`；数据库 `litellm-timescaledb.litellm.svc.cluster.local:5432`（TimescaleDB，库 `litellm`）。
- 运行时保护模块（`cliproxy_key_limits` / `cliproxy_stream_usage` / `cliproxy_user_limits`）锁定 1.100.0 源码与 schema；`proxy_admin` 对预算、周期字段全放行，普通用户被挡。
- `max_internal_user_budget: 5` 仅为新建用户默认值填充，**不是** `/user/update` 的上限（源码核实：`internal_user_endpoints.py:199` 等仅做缺省填充）。
- Key 预算独立封顶：默认 $5；当前 `upperbound_key_generate_params.max_budget` 为 $1000（2026-09-10 复核部署配置）。给用户加额后若 Key 预算更小，会在 Key 侧先封顶，充值必须联动检查；该上限变更时须同步审查执行器常量与测试。
- 授权与用户额度变更经各实例本地缓存，传播窗口约 60 秒；充值回读校验必须容忍该延迟。
- UI 每次登录自动签发 24h / $1 / 无别名会话 Key，会污染用户 Key 列表，快照需可过滤。
- 计费口径边界：gpt-5.6-luna 官方 >272K 才加价，LiteLLM 只有 200K 档位键，200K–272K 区间按高费率计；缓存写 1.25x 未配置；流式计量修复前的历史账目有估算偏差且未回填。

mss-boot-admin 侧：

- 基线 `web/antd-v6/v1.3.7`（前端独立发布单元）；后端 `admin/` Go 应用；垂直业务模块位于 `admin/modules/`（生成器产物 + `custom.go` 扩展点，如 `supplier` 模块）。
- 鉴权：JWT + casbin RBAC；有操作日志/审计基建。
- 仓库治理：所有变更先通过 PR 合并到 `main`，仅允许从与 `origin/main` 完全一致且干净的提交构建和发布。

## 4. 总体架构

形态：`admin/modules/litellmops` 垂直业务模块 + `web/antd-v6` 新增「LiteLLM 运营」菜单组。

```
管理员浏览器
   │ HTTPS（admin 前端，JWT + casbin）
   ▼
mss-boot-admin（admin 应用，litellmops 模块）
   │ 写：LiteLLM Admin REST API（master key，K8s Secret 注入 env）
   ├──────────────► LiteLLM proxy（litellm:4000）── 用户/Key/预算读写
   │ 读（报表）：只读 DB 账号
   └──────────────► TimescaleDB（litellm-timescaledb:5432）── SpendLogs 等只读
```

集成面决策：

- **写路径只走 LiteLLM Admin REST API**（`/user/update`、`/key/update`、`/user/info`、`/key/list` 等，以 1.100.0 实际端点为准）。不直接改库，保证缓存失效与内部一致性。
- **报表读路径直读 TimescaleDB**（专用只读账号）：账目/用量查询不走代理 API，避免报表压力影响线上推理，且金额精确。
- **实时状态读走 Admin API**（用户详情、Key 列表），快照与实读结合。

部署：新 namespace `litellm-ops`。master key 的 Secret 由运维手工从 `litellm` ns 复制登记（K8s 不允许跨 ns 引用 Secret），复制与轮换流程写入运维手册。NetworkPolicy 仅放行模块到 `litellm:4000` 与 `litellm-timescaledb:5432`。

## 5. 功能设计

### 5.1 用户与额度（只读同步视图）

- 同步：页面手动刷新，以及用户/Key 写入成功后的尽力刷新；当前没有后台定时同步器。
- 用户视图：email、角色、授权模型、max_budget、budget_duration、budget_reset_at、spend。
- Key 视图：别名、哈希前缀（**不存完整 Key**）、预算、TPM/RPM/并发、过期时间、spend、归属用户。
- 会话 Key 过滤：无别名 + $1 + 24h 有效期的 UI 会话 Key 默认折叠，可展开。

### 5.2 充值管道

当前只实现 `add_credit`：提高用户 `max_budget`。入口包括用户页直充和已批准的销售订单；消费清零仅作为 Key 管理动作提供，`reset_window` 尚未开放。

流程：先创建耐久记录并校验目标用户与参数 → 执行（调用 LiteLLM API，记录脱敏摘要）→ 回读校验（`/user/info` 复核目标字段，容忍缓存传播）→ 完成 / 待对账 / 失败。销售订单另有付款验证、用户匹配和批准阶段。

- 幂等：用户页直充必须由调用方为同一业务事件提供稳定且唯一的显式幂等键；销售订单按内部订单 ID 确定性生成充值幂等键。同一键和相同 payload 重放返回原记录，同一键但金额或 `raise_keys` 不同必须以冲突拒绝。
- Key 联动：`add_credit` 执行前检测该用户 Key 预算封顶，列出会卡顶的 Key，操作人确认后同步调 `/key/update`（不得超过当前部署的 upperbound，现值 $1000）。
- 审计：记录操作人、时间、变更前后值和脱敏结果摘要；销售订单另记录批准人。

### 5.3 账单

- 直读 SpendLogs：按用户、时间范围、模型、成功/失败过滤，分页与总计。
- 当前页面展示 LiteLLM 返回的账目事实并说明已知计费口径边界；按 `values.yaml` 逐条重算和偏差标注尚未实现。

### 5.4 结算（后续里程碑）

当前版本没有结算表、结算状态机或 CSV 导出接口。后续实现仍须保持用户 × 周期聚合可复算，并且不修改 LiteLLM 原始数据。

### 5.5 支付接入预留（后续里程碑）

当前版本没有支付网关回调或 `TopupExecutor` 接口。已实现的是销售订单状态机及官方/授权订单连接器入口；未来接入支付时必须复用其签名验证、幂等、批准、充值和对账安全边界，不能创建旁路加额接口。

### 5.6 LiteLLM 常用管理动作

运营端采用固定、手写、版本化的 API 契约，不把 LiteLLM OpenAPI 直接透传给浏览器：

- 用户：创建/邀请、修改额度与模型权限、启用/停用、删除；创建/邀请显式设置 `auto_create_key=false`，避免产生未交付的隐式 Key。
- Key：签发、修改限额、启用/停用、删除、轮换、消费清零；原始 Key 只在创建或轮换成功响应中展示一次，禁止落库和日志。
- 用户和 Key 的写操作统一进入耐久 `management command` 隔离层：先在本模块数据库记录命令、脱敏的类型化预期、写前摘要和尝试记录，再调用 LiteLLM；同一用户与充值共用租约，任何 `executing/result_unverified` 指令都会阻止新的管理写入和充值。确定失败记为 `failed`；成功并验证记为 `completed`；超时、5xx、进程中断或回读不确定则保留为 `result_unverified`，不得直接重发。
- 管理指令恢复默认只做权威回读。预期已出现时收敛为 `resolved_applied`；超过 2 分钟且两次相隔至少 5 秒的观察均稳定等于写前状态时收敛为 `resolved_not_applied`；观察到第三种状态时标记人工复核。Key 签发/轮换等一次性结果以及无法由安全类型化投影完整验证的更新，只能在相同等待和稳定观察条件后由管理员确认“已生效/未生效”并填写原因。人工决议只解除隔离，不重放写入，也不恢复密钥。
- 模型：读取已配置模型与网关健康；当前 LiteLLM 1.100.0 列表响应不能提供可权威枚举的启停状态，因此模型写路由固定返回 `501 operation_disabled`，前端只展示只读/未知状态并链接 LiteLLM 原生后台。
- 组织/团队：仅保留组织、成员与团队的只读视图。创建、修改、删除、成员分配、团队变更以及组织/团队充值均固定返回 `501 operation_disabled`，前端不展示写动作。

模型或组织/团队写能力只有在拥有可持久化的写前状态、幂等命令、权威回读和异常恢复机制后才可开放；新增模型、供应商密钥、路由参数等敏感配置仍走受控 GitOps/LiteLLM 原生管理端。

所有管理动作均以 LiteLLM 1.100.0 实际 schema 为准；客户端只返回稳定的本模块错误码和脱敏摘要，不向前端透传上游响应正文。

### 5.7 闲鱼订单与商品映射

首期运营来源是“我的闲鱼”：商品位于“我发布的”，成交位于“我卖出的”。“我发布的”仅用于确认商品与外部商品 ID，不是成交订单入口；成交事实必须在闲鱼 App 的“我的闲鱼 → 我卖出的”核对。因闲鱼桌面网页将卖出订单限制在 App，本模块采用以下分层：

1. 手工入单（立即可运营）：录入闲鱼订单号、买家提供的 LiteLLM 注册邮箱、实付金额，选择或自动匹配商品映射。
2. 授权连接器导入：受保护的服务端接口接收官方开放平台或经授权 ERP 的标准化订单；同时要求 Admin PAT、独立连接器 token、HMAC-SHA256 请求签名、±5 分钟时间窗、`channel:shop` 来源白名单与 payload hash 防重放。服务身份还必须分别拥有核验与匹配的动作权限，缺失时订单只停留在 `received`。
3. 自动充值：仅对“可信来源 + 已付款 + 唯一启用映射 + 精确用户匹配 + auto_apply 开启”的订单自动批准和执行；执行前还要求服务身份分别拥有批准和执行权限。任一条件不满足即停留在可审计状态，默认失败关闭。

当前部署默认不配置 `LITELLMOPS_CONNECTOR_SHARED_TOKEN` 和 `LITELLMOPS_CONNECTOR_ALLOWED_SOURCES`，因此导入端点返回 `connector_disabled`；即使商品设置 `auto_apply=true`，也不会启用自动入单或自动充值。

闲鱼官方接入不是复用个人账号网页登录态。订单能力需要申请并获批商家端 B 端 AppKey，淘宝开放平台、闲鱼入驻及合同签约主体须一致，并使用签约企业所有的闲鱼账号。处理用户 ID、订单信息的服务须部署在聚石塔，通过 OAuth 2.0 换取 access token 后调用 TOP API；正向订单查询接口为 `alibaba.idle.isv.order.query`。准入、凭据与聚石塔部署未完成前，只允许手工核单。

商品映射以 `channel + shop + external_item_id (+ sku)` 唯一定位，记录商品名称、人民币分、充值微美元、是否启用及是否允许自动执行。金额采用整数（CNY 分、USD micro）存储；展示时再格式化，禁止浮点作为账本金额。

订单状态机：

`received → verified_paid → mapped → approved → executing → applied_unverified → completed`

旁路状态为 `retryable_failed`、`terminal_failed`、`reconcile_required`、`refund_review`、`reversed`。当前版本的退款接口只要求填写原因并将订单送入 `refund_review`，不自动扣减额度，也没有批准、拒绝、关闭或冲正 API；`reversed` 是后续受控冲正能力的预留状态。

唯一约束为 `(channel, shop, external_order_id, adjustment_type)`；同时保存标准化 payload hash。相同订单与 payload 重放返回原结果，不重新加额；相同订单但 payload 不同必须拒绝并告警。

### 5.8 充值一致性与恢复

充值执行必须先在本模块数据库中保留幂等记录，再调用 LiteLLM，禁止“先加额、后记单”。执行器遵循：

同一用户只能有一个未决充值命令。`approved`、`executing`、`applied_unverified`、`retryable_failed`、`reconcile_required` 都会阻止后续直接充值，或阻止其他销售订单在执行阶段创建新的充值记录；运营必须处理原记录，不能用新业务单绕过屏障。

1. 对用户获取带过期时间的串行租约；状态用 compare-and-swap 从 `approved/retryable_failed` 进入 `executing`。
2. 实读用户与 Key 的当前值，计算并持久化**绝对目标值** `target_after`，同时记录 before 快照和执行尝试。
3. 调 `/user/update` 与必要的 `/key/update`；不使用可重复累加的远端语义。
4. 网络超时或未知响应先只读回读 LiteLLM，不立即重写。达到绝对目标即完成；观察到非 before/target 的值则进入 `reconcile_required`。只有未知状态持续超过 2 分钟，且两次相隔至少 5 秒的权威回读都稳定等于原 before，才允许重发**同一个绝对目标值**；不得重新计算增量。
5. 回读容忍 LiteLLM 缓存传播窗口；每个子步骤和响应只保存脱敏摘要。当前没有后台定时对账器，未确认记录由运营等待传播后在原订单或原充值记录上触发恢复操作。

对账不创建新充值。已有绝对目标的未知结果先按上述只读规则收敛；`retryable_failed` 且尚未持久化目标时，才会在证明未写入后安全续跑原预留。

旧的用户页“直接充值”接口必须复用同一充值账本与执行器，只能作为兼容入口，不能绕过预留、状态机、审计或幂等约束。客户端必须为业务事件提供稳定且唯一的显式幂等键；服务端仍以数据库唯一约束为最终防线。若进程在预留后、远端写入前中断，原记录会停在 `approved`，运营只能在该记录上“继续原充值”。

销售订单必须从原订单执行或对账，用户页直充必须从原充值记录恢复；直充创建与对账 HTTP 接口都拒绝销售订单来源和保留幂等命名空间，避免绕过订单 claim/CAS/租约。充值没有人工“标记完成”、强制跳状态或凭工单结论解除屏障的接口；人工决议只适用于第 5.6 节中无法自动验证的管理指令，不适用于充值。

## 6. 数据模型（模块自有表，迁移随模块注册）

| 表 | 关键字段 |
| --- | --- |
| `litellmops_user_snapshot` | user_id(PK)、email、role、models(json)、max_budget、budget_duration、budget_reset_at、spend、synced_at |
| `litellmops_key_snapshot` | key_hash_prefix(PK)、alias、user_id、max_budget、tpm/rpm/parallel、expires、spend、synced_at |
| `litellmops_recharge` | id(PK)、user_id/idempotency_key(UK)、amount_usd_micro、before_budget_usd_micro、target_after_usd_micro、source/source_ref、payload_hash、status、uncertain_since、last_observed值/时间、keys_updated、operator/reason、version、timestamps |
| `litellmops_sales_product` | id(PK)、channel/shop/external_item_id/sku(UK)、title、price_cny_fen、credit_usd_micro、enabled、auto_apply、created_at/updated_at |
| `litellmops_sales_order` | id(PK)、channel/shop/external_order_id/adjustment_type(UK)、payload_hash、product_id、user_id/email、paid_cny_fen、credit_usd_micro、source_trust、status、operator/approver、timestamps |
| `litellmops_operation_attempt` | id(PK)、operation_type/operation_id、target_id、step/attempt(UK)、request_digest、result_code、operator、started_at/finished_at |
| `litellmops_user_lease` | user_id(PK)、holder、lease_until、updated_at（跨进程串行化充值） |
| `litellmops_management_command` | id(PK)、user_id/email、target_type/id、action、payload_hash、expected_state、before/last_observed_digest、status、manual_review、operator/resolution、version、timestamps（用户/Key 写入的耐久隔离与恢复；敏感字段不对外返回） |
| 审计 | 复用 admin 操作日志基建；充值、销售订单和管理动作强制落脱敏审计 |

## 7. 接口设计（模块 REST，前缀 `/admin/api/litellmops`）

- `GET /users`、`GET /users/{id}`（含 Key 列表）、`POST /sync`
- `POST /users/{id}/recharge`、`GET /users/{id}/recharges`、`POST /recharges/{id}/reconcile`（只有安全对账，无手工完成接口）
- `GET /bills`（SpendLogs 查询）；当前没有 summary、settlement 或 export 路由
- `POST /users`、`PATCH/DELETE /users/{id}`、`POST /users/{id}/block|unblock`
- `POST /keys`、`PATCH/DELETE /keys/{id}`、`POST /keys/{id}/block|unblock|rotate|reset-spend`
- `GET /management/commands`、`POST /management/commands/{id}/reconcile|resolve`；`resolve` 仅允许处理无法自动验证且已满足等待、稳定观察、确认和原因要求的管理指令
- `GET /gateway/models`、`GET /gateway/health`；保留的 `POST /gateway/models/{id}/block|unblock` 固定返回 `501 operation_disabled`
- `GET /organizations`、`GET /organizations/{id}`；组织、成员、团队的所有本地写路由均固定返回 `501 operation_disabled`
- `GET/POST/PATCH /sales/products`、`GET/POST /sales/orders`、`GET /sales/orders/{id}`
- `POST /sales/orders/import`（连接器专用认证）、`POST /sales/orders/{id}/verify|match|approve|execute|reconcile|refund-review`

`refund-review` 只进入人工复核并保存原因；当前没有自动扣额、批准、拒绝、关闭或冲正接口。

前端页面（antd-v6，「LiteLLM 运营」菜单）：总览/网关、用户与额度、Key、管理指令恢复、组织/团队、闲鱼订单与商品映射、充值/对账、账单。用户/Key 写操作必须有动作级权限、二次确认与审计提示；模型和组织/团队页只读并提供 LiteLLM 原生后台入口。所有页面覆盖加载、空态、错误、403 和移动端布局。

## 8. 安全设计

- master key 仅存 K8s Secret，注入模块 env；不落日志、快照、接口响应、前端。
- 报表用专用只读 DB 账号（`SELECT` on `litellm` 库相关表），账号创建纳入部署清单。
- 快照只存 Key 哈希前缀，不存完整 Key。
- 模块 RBAC 三角色：`litellmops-admin`（全部）、`litellmops-finance`（账单、商品/订单、充值与对账）、`litellmops-readonly`；管理指令人工决议仅管理员可用。
- 销售订单保存验证人、批准人和执行人；当前没有强制双人审批配置，生产流程要求运营按岗位分离执行。
- 连接器凭据与 LiteLLM master key 分离；闲鱼订单导入端点不得使用普通浏览器会话认证。Admin PAT 不能单独获得连接器信任，还必须通过独立 token、HMAC、±5 分钟时间窗与 `channel:shop` 白名单校验。
- 连接器 Secret 或来源白名单未配置时端点以 `connector_disabled` 失败关闭；签名失败、时间窗超限或来源非可信时拒绝导入和自动执行。
- NetworkPolicy 最小化出向；模块仅监听集群内。

## 9. 风险与缓解

| 风险 | 缓解 |
| --- | --- |
| LiteLLM 升级破坏 API/schema | 版本锁定 1.100.0；运行时模块同源审查；升级流程与本模块联测纳入运维手册 |
| 充值生效延迟（~60s 缓存） | 回读校验有限重试 + 前端"生效中"状态 |
| 会话 Key 污染视图 | 快照过滤规则 + 可展开 |
| 计费口径近似（200K–272K、缓存写、历史偏差） | 页面说明已知口径；当前没有自动重算或结算产物，对外结算前必须人工复核并标注记录 |
| 单节点/灾备缺失 | 基础设施轨道另行立项，与本模块解耦 |
| 订单重复回调或充值请求超时 | 数据库唯一约束 + 用户租约 + CAS + 绝对目标值 + 先回读后重试；未知结果进入人工对账 |
| 用户/Key 管理写入超时或 5xx 后结果未知 | 写前创建耐久管理指令 + 与充值共用用户租约 + 类型化权威回读；未决时隔离后续管理与充值，只有受限人工决议可解除不可验证命令 |
| 闲鱼网页无卖出订单接口 | 首期手工录单；只接官方/授权连接器，不依赖 Cookie 或私有接口 |
| 退款时额度已消费 | 一律进入人工退款复核，不自动造成负余额或篡改 SpendLogs |

## 10. 里程碑与验收

| 里程碑 | 内容 | 验收 |
| --- | --- | --- |
| M1 | 设计评审（本文档） | Owner 评审通过 |
| M2 | 模块骨架 + 只读同步 + 用户/额度/账单页面 | 同步视图与 LiteLLM 实读一致；会话 Key 可过滤；无完整 Key 落库 |
| M3 | 手工充值管道 + 审计 | `add_credit` 端到端；幂等重放无双份；回读一致；审计字段完整 |
| M4（后续） | 结算单 + 导出 | 尚未实现；完成定义为与 SpendLogs 总额一致、口径有备注且 CSV 可复算 |
| M5 | 闲鱼运营闭环 + 连接器接口 | 商品映射、手工录单、审批/执行/对账/退款复核；重复订单不重复充值；未配置官方/授权来源时自动入单失败关闭 |
| M5.1 | LiteLLM 常用管理动作 | 用户/Key 类型化写操作通过 RBAC、耐久隔离、恢复、审计与脱敏验收；模型和组织/团队只读，本地写路由统一失败关闭 |
| M6 | 上线 | 部署清单、NetworkPolicy、Secret 登记、回滚方案 |

每个里程碑先在目标部署环境完成验证；验证命令与结果按仓库约定记录（最小验证集先行，再按影响面扩大）。

## 11. 开发约定

- 工作区：下游运营项目 worktree（分支 `litellm-ops/design`，基线 `web/antd-v6/v1.3.7`）。
- 环境与校验：`go run ./cmd/mss context`、`go run ./cmd/mss doctor`；模块代码优先用仓库生成器产出，不手写重复骨架。
- 规格先行：中大型变更先更新设计/规格再实现；完成的设计先提交再扩大测试。
- 发布边界：所有变更先通过 PR 合并到 `main`，并从与 `origin/main` 完全一致且干净的提交构建和发布；不得从 topic branch 发布。
- 所有实现与验收产物保留在受控的下游项目及部署环境，不复制生产凭据或私有数据到开发者本地电脑。

## 12. 关联资料

- `cliproxy/HANDOFF.md`、`cliproxy/LAUNCH.md`：网关上线路径、运营策略、验收证据。
- `cliproxy/k8s/litellm/values.yaml`：模型与单价配置（账单重算口径来源）。
- `cliproxy/ops/gpt-5.6-luna-receipt.json`：模型上线回执样例。
- LiteLLM 1.100.0 Admin API（以镜像内源码为准）。
- [闲鱼开放平台：小程序快速接入](https://open.goofish.com/doc/quick-start.html)：企业主体、商家端 B 端 AppKey 与订单能力审批。
- [闲鱼开放平台：服务端接入](https://open.goofish.com/doc/development/dev/server.html)：聚石塔、OAuth/TOP 与订单 API。
