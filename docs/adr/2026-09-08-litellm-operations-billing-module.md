# LiteLLM 计费运营模块设计（立项）

- Status: Proposed（待评审）
- Date: 2026-09-08
- Baseline: `web/antd-v6/v1.3.7`（commit `77b53d41`，前端 1.3.7 发布线）
- Owners: 文祥
- Target release: 下游运营项目自用，不参与上游 mss-boot-admin 发布线
- 工作区：`/root/workspace/mss-boot-admin-litellm-ops`（git worktree，分支 `litellm-ops/design`）

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
- G2：充值管道：手工充值单（加额 / 消费清零 / 周期重置）→ 可选审批 → 执行 → LiteLLM 生效 → 回读校验 → 审计流水；全程幂等可重放。
- G3：账单与结算：按用户/时间/模型查账；按周期生成结算单并导出；金额口径按 `values.yaml` 配置单价重算校验。
- G4：支付接入预留：充值执行器接口化，后续接支付网关不改主流程。

非目标：

- 不自建用户源。LiteLLM 是用户与 Key 的唯一权威，本模块只读同步、回写只走 Admin API。
- 不修改 LiteLLM 原始账目（SpendLogs 只读）。
- 不做 C 端自助中心。用户自助继续使用 LiteLLM UI；支付自助充值属后续里程碑。
- 不覆盖监控告警、灾备、高可用（见第 1 节分工）。

## 3. 现状事实（设计依据，2026-09-08 核实）

LiteLLM 侧：

- 版本固定 1.100.0，2 副本，Helm revision 10；管理 API 使用 master key（K8s Secret `litellm-masterkey`）。
- 集群内地址 `http://litellm.litellm.svc.cluster.local:4000`；数据库 `litellm-timescaledb.litellm.svc.cluster.local:5432`（TimescaleDB，库 `litellm`）。
- 运行时保护模块（`cliproxy_key_limits` / `cliproxy_stream_usage` / `cliproxy_user_limits`）锁定 1.100.0 源码与 schema；`proxy_admin` 对预算、周期字段全放行，普通用户被挡。
- `max_internal_user_budget: 5` 仅为新建用户默认值填充，**不是** `/user/update` 的上限（源码核实：`internal_user_endpoints.py:199` 等仅做缺省填充）。
- Key 预算独立封顶：默认 $5、`upperbound` $10。给用户加额后若 Key 预算更小，会在 Key 侧先封顶，充值必须联动检查。
- 授权与用户额度变更经各实例本地缓存，传播窗口约 60 秒；充值回读校验必须容忍该延迟。
- UI 每次登录自动签发 24h / $1 / 无别名会话 Key，会污染用户 Key 列表，快照需可过滤。
- 计费口径边界：gpt-5.6-luna 官方 >272K 才加价，LiteLLM 只有 200K 档位键，200K–272K 区间按高费率计；缓存写 1.25x 未配置；流式计量修复前的历史账目有估算偏差且未回填。

mss-boot-admin 侧：

- 基线 `web/antd-v6/v1.3.7`（前端独立发布单元）；后端 `admin/` Go 应用；垂直业务模块位于 `admin/modules/`（生成器产物 + `custom.go` 扩展点，如 `supplier` 模块）。
- 鉴权：JWT + casbin RBAC；有操作日志/审计基建。
- 仓库治理：发布走 PR-to-main；本下游项目暂不推送 origin，推送策略由 Owner 另行决定。

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

- 同步：定时 5 分钟 + 页面手动刷新；只读增量同步用户与 Key 摘要。
- 用户视图：email、角色、授权模型、max_budget、budget_duration、budget_reset_at、spend。
- Key 视图：别名、哈希前缀（**不存完整 Key**）、预算、TPM/RPM/并发、过期时间、spend、归属用户。
- 会话 Key 过滤：无别名 + $1 + 24h 有效期的 UI 会话 Key 默认折叠，可展开。

### 5.2 充值管道

充值单类型：

| 类型 | 语义 | LiteLLM 动作 |
| --- | --- | --- |
| `add_credit` | 加额 | `/user/update` 提高 `max_budget` |
| `reset_spend` | 消费清零 | `/user/update` 置 `spend=0` |
| `reset_window` | 周期重置 | `/user/update` 改 `budget_reset_at` 触发新周期 |

流程：创建（校验目标用户存在、参数合法）→ 审批（可选，配置开关）→ 执行（调 LiteLLM API，记录响应摘要）→ 回读校验（`/user/info` 复核目标字段，容忍 ~60s 缓存延迟，有限重试）→ 完成 / 失败。

- 幂等：幂等键 = 目标用户 + 类型 + 金额 + 业务日期（或显式 client_token）；执行前查重；执行失败可安全重试，不产生双份。
- Key 联动：`add_credit` 执行前检测该用户 Key 预算封顶，列出会卡顶的 Key，操作人确认后同步调 `/key/update`（upperbound $10 以内）。
- 审计：操作人、审批人、时间、变更前后值、LiteLLM 响应摘要，追加不可改。

### 5.3 账单

- 直读 SpendLogs：按用户、时间范围、模型、成功/失败过滤，分页与总计。
- 校验：按 `values.yaml` 配置单价对每条成功记录重算，偏差 > 1e-9 标注；口径边界（见第 3 节）在页面常驻说明。

### 5.4 结算

- 结算单：用户 × 周期聚合（Token 分项、金额、记录数），状态机 `open → confirmed → exported`。
- 导出 CSV（明细 + 汇总），结算单备注自动写入口径边界；不修改 LiteLLM 原始数据。

### 5.5 支付接入预留

- 充值执行器接口：`TopupExecutor`（Manual 本期实现 / Payment 后续接入）。
- 预留支付回调入口与订单状态机设计（签名验证、重复回调幂等），本期不实现，仅冻结接口契约。

## 6. 数据模型（模块自有表，迁移随模块注册）

| 表 | 关键字段 |
| --- | --- |
| `litellmops_user_snapshot` | user_id(PK)、email、role、models(json)、max_budget、budget_duration、budget_reset_at、spend、synced_at |
| `litellmops_key_snapshot` | key_hash_prefix(PK)、alias、user_id、max_budget、tpm/rpm/parallel、expires、spend、synced_at |
| `litellmops_topup_order` | id(PK)、idempotency_key(UK)、user_id、type、amount、params(json)、status(draft/pending/executing/done/failed)、operator、approver、before/after(json)、litellm_response(json)、created_at/updated_at/executed_at |
| `litellmops_settlement` | id(PK)、user_id、period_start/end、usage_summary(json)、amount、status、operator、exported_at |
| 审计 | 复用 admin 操作日志基建；充值/结算动作强制落审计 |

## 7. 接口设计（模块 REST，前缀 `/admin/litellmops`）

- `GET /users`、`GET /users/{id}`（含 Key 列表）、`POST /users/sync`
- `POST /topup-orders`、`GET /topup-orders`、`GET /topup-orders/{id}`、`POST /topup-orders/{id}/approve`、`POST /topup-orders/{id}/execute`
- `GET /bills`（SpendLogs 查询）、`GET /bills/summary`
- `POST /settlements`、`GET /settlements`、`POST /settlements/{id}/confirm`、`GET /settlements/{id}/export`

前端页面（antd-v6，「LiteLLM 运营」菜单）：用户与额度、充值、账单、结算；写操作按钮带二次确认与审计提示。

## 8. 安全设计

- master key 仅存 K8s Secret，注入模块 env；不落日志、快照、接口响应、前端。
- 报表用专用只读 DB 账号（`SELECT` on `litellm` 库相关表），账号创建纳入部署清单。
- 快照只存 Key 哈希前缀，不存完整 Key。
- 模块 RBAC 三角色：`litellmops-admin`（全部）、`litellmops-finance`（充值审批、结算）、`litellmops-readonly`。
- 充值可配置要求审批人 ≠ 操作人（双人规则）。
- NetworkPolicy 最小化出向；模块仅监听集群内。

## 9. 风险与缓解

| 风险 | 缓解 |
| --- | --- |
| LiteLLM 升级破坏 API/schema | 版本锁定 1.100.0；运行时模块同源审查；升级流程与本模块联测纳入运维手册 |
| 充值生效延迟（~60s 缓存） | 回读校验有限重试 + 前端"生效中"状态 |
| 会话 Key 污染视图 | 快照过滤规则 + 可展开 |
| 计费口径近似（200K–272K、缓存写、历史偏差） | 账单重算校验标红；结算单备注口径；对外结算前人工复核标注记录 |
| 单节点/灾备缺失 | 基础设施轨道另行立项，与本模块解耦 |

## 10. 里程碑与验收

| 里程碑 | 内容 | 验收 |
| --- | --- | --- |
| M1 | 设计评审（本文档） | Owner 评审通过 |
| M2 | 模块骨架 + 只读同步 + 用户/额度/账单页面 | 同步视图与 LiteLLM 实读一致；会话 Key 可过滤；无完整 Key 落库 |
| M3 | 手工充值管道 + 审计 | 三类充值单端到端；幂等重放无双份；回读一致；审计字段完整 |
| M4 | 结算单 + 导出 | 与 SpendLogs 总额一致；口径备注；CSV 可复算 |
| M5 | 支付接口预留评审 | Executor 接口契约冻结；回调状态机文档 |
| M6 | 上线 | 部署清单、NetworkPolicy、Secret 登记、回滚方案 |

每个里程碑先在 242 完成验证；验证命令与结果按仓库约定记录（最小验证集先行，再按影响面扩大）。

## 11. 开发约定

- 工作区：`/root/workspace/mss-boot-admin-litellm-ops`（worktree，分支 `litellm-ops/design`，基线 `web/antd-v6/v1.3.7`）。
- 环境与校验：`go run ./cmd/mss context`、`go run ./cmd/mss doctor`；模块代码优先用仓库生成器产出，不手写重复骨架。
- 规格先行：中大型变更先更新设计/规格再实现；完成的设计先提交再扩大测试。
- 发布边界：遵守仓库 PR-to-main 治理；本下游分支暂不推送 origin，推送策略待 Owner 确认。
- 所有产物只放 242 对应目录，不落开发者本地电脑。

## 12. 关联资料

- `cliproxy/HANDOFF.md`、`cliproxy/LAUNCH.md`：网关上线路径、运营策略、验收证据。
- `cliproxy/k8s/litellm/values.yaml`：模型与单价配置（账单重算口径来源）。
- `cliproxy/ops/gpt-5.6-luna-receipt.json`：模型上线回执样例。
- LiteLLM 1.100.0 Admin API（以镜像内源码为准）。
