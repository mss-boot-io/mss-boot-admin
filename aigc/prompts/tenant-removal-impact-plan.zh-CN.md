# 去多租户改造评估与任务拆解（评审版）

> 评估范围：`mss-boot-admin` + `mss-boot` 当前可见代码（截至 2026-02-28）。
>
> 前提假设：项目不要求向后兼容既有租户数据，目标是“显著简化架构”。

## 1. 规模评估（便于快速判断成本）

- `mss-boot-admin` 受影响文件：**47**（去重后）
- `mss-boot` 受影响文件：**9**
- 合计：**56 个文件级触点**

说明：该数量是“涉及租户关键词/接口/字段”的扫描结果，实际修改文件数通常会低于触点数，但仍属于**中到大型重构**。

## 2. 改造面分组

### A. 核心架构层（高优先级）

1. 租户中心抽象与注入
   - `center/type.go`, `center/default.go`
   - `cmd/server/server.go`, `cmd/migrate/migrate.go`
2. 租户实体与路由
   - `models/tenant.go`, `models/tenant_migrate.go`, `apis/tenant.go`, `dto/tenant.go`
3. 鉴权中的 tenant 校验
   - `middleware/auth.go`

### B. 数据模型层（高优先级）

1. `ModelGormTenant` 基类
   - `models/type.go`
2. 继承该基类的业务模型（user/role/menu/task/language/...）
   - `models/*.go`（约 15+ 文件）
3. 索引与迁移
   - `cmd/migrate/migration/system/*`（`tenant_id` 相关索引/种子）

### C. 业务流程层（中高优先级）

1. 控制器作用域接入
   - `apis/*` 中 `controller.WithScope(center.Default.Scope)`
2. 服务查询显式 `tenant_id` 条件
   - `service/user_config.go` 等
3. 存储路径 tenant 目录
   - `service/storage.go`

### D. 缓存层（中高优先级）

1. 业务 key 的 tenant 前缀
   - `apis/language.go`, `service/app_config.go`
2. 失效逻辑
   - 同上（删除/重建缓存时 tenant 维度）

### E. 框架通用层（mss-boot，必要同步）

1. 安全接口 tenant 方法
   - `pkg/security/security.go` (`GetTenantID`)
2. 通用工具 tenant 判断
   - `pkg/utils.go` (`SupportMultiTenant`)
3. 虚拟模型 tenant 字段与 scope
   - `virtual/model/model.go`, `virtual/action/*.go`

### F. 文档与生成规则（低风险但必须收口）

- `.github/copilot-instructions.md`
- `.github/prompts/*.md`
- `aigc/prompts/*`（多租户相关说明）

## 3. 风险评估

## 高风险

1. **鉴权与数据权限耦合**：tenant 校验移除后，需要确认不会误放大数据访问范围。
2. **迁移脚本与索引变更**：`tenant_id` 相关索引删除会影响已有 SQL 逻辑。
3. **虚拟模型行为变化**：`MultiTenant` 字段移除会影响动态模型 CRUD 语义。

## 中风险

1. 缓存 key 统一后，旧 key 失效策略与启动预热行为。
2. 上传路径从 `tenant/user/file` 改为 `user/file` 后的访问路径变化。

## 低风险

1. 文档、提示词、注解规范的同步更新。

## 4. 任务安排（可执行清单）

## Phase 0：冻结需求边界（0.5 天）

- 决定是否完全删除 `tenant` API 与数据库表。
- 决定是否保留 `MultiTenant` 字段（建议删除）。

交付物：改造边界说明（1 页）。

## Phase 1：核心链路去租户（2~3 天）

- 移除 `center.TenantImp/TenantMigrator` 依赖链。
- 下线 `apis/tenant.go` 路由与 `models/tenant*.go` 运行时逻辑。
- 移除 `middleware/auth.go` 中 tenant 校验。

交付物：服务可启动、鉴权可通过、基础路由可用。

## Phase 2：模型与 CRUD 作用域简化（2~3 天）

- `ModelGormTenant` 合并/替换为基础模型。
- 批量修改业务模型移除 `tenant_id` 字段与钩子。
- 去除 `WithScope(center.Default.Scope)` 注入及相关实现。

交付物：核心 CRUD 通过，编译通过。

## Phase 3：缓存/存储/虚拟模型收口（1.5~2 天）

- 缓存 key 去 tenant 前缀并统一失效逻辑。
- 上传路径去 tenant 目录。
- 虚拟模型移除 `MultiTenant/TenantScope/TenantIDFunc`。

交付物：配置、语言、上传、虚拟模型链路通过。

## Phase 4：迁移与文档清理（1~1.5 天）

- 清理迁移脚本中的 `tenant_id` 索引与种子租户逻辑。
- 更新 copilot 指令、提示词、README 的多租户描述。

交付物：迁移可执行，文档一致。

## 5. 总工期预估

- 预估：**7 ~ 10 个工作日**（单人）
- 若并行（1 人核心代码 + 1 人迁移/文档），可压缩至 **4 ~ 6 个工作日**。

## 6. 建议的 PR 切分（便于 review）

1. PR-1：中心与鉴权去租户（核心骨架）
2. PR-2：模型与控制器作用域改造
3. PR-3：缓存、存储、虚拟模型改造
4. PR-4：迁移与文档收尾

这种拆分有助于每个 PR 变更面可审阅、可回滚、可验证。
