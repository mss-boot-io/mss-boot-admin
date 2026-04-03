# 多租户设计补充评估：Storage Cache 维度

> 适用范围：本文基于当前仓库可见代码与文档（截至 2026-02-28）进行评估，重点关注多租户与缓存的交互设计。

## 1. 现状归纳（缓存与多租户如何协同）

### 1.1 业务层缓存 key 已做租户前缀隔离

在 admin 项目中，业务缓存 key 多采用 `{tenantID}:...` 形式：

- AppConfig：`{tenantID}:app-configs`、`{tenantID}:app-configs:{group}`
- Language：`{tenantID}:language`、`{tenantID}:language:{name}`

这种方式能在 key 级别实现基础租户隔离，避免不同租户数据直接冲突。

### 1.2 框架层 cache 抽象本身不感知租户

`mss-boot` 的 `storage.AdapterCache` 与 `gorm cache` 插件关注通用查询缓存与 tag 清理，不内建 tenant namespace 概念；租户隔离主要依赖调用方传入的 tag/key 规范。

### 1.3 多租户作用域与缓存命中路径并行存在

- DB 查询路径：通过 `Scope` 注入 `tenant_id` 条件。
- 业务缓存路径：通过显式 key 前缀命中缓存。

两条路径是“约定协同”，而不是“统一租户上下文驱动”。

## 2. 优点

1. **实现成本低**：无需改动底层 cache 接口，即可在业务层完成隔离。
2. **兼容性好**：对现有 Redis 适配器与 Action 流程侵入小。
3. **可读性尚可**：key 含 tenantID，排障时容易初步判断所属租户。

## 3. 风险与不足

1. **隔离依赖人工约定**：若某处忘记拼 tenantID，可能出现跨租户污染。
2. **失效策略不统一**：业务 key 与 gorm query cache（tag/key）体系分离，容易出现“库已更新、缓存未同步失效”。
3. **存在实现细节风险**：`AppConfig.CreateOrUpdate` 清理缓存时使用 `center.GetTenant().GetID()`，未从当前请求上下文解析 tenant，可能导致清理目标不准确。
4. **租户识别链路稳定性影响缓存正确性**：当前租户解析依赖 `Referer`，一旦识别异常，缓存读写都可能落到错误租户命名空间。
5. **缺少缓存租户维度治理能力**：缺少统一指标（命中率/污染率/按租户 key 数量）与审计工具。

## 4. 改进建议（按优先级）

### P0（优先执行）

1. **统一租户上下文取值**：缓存读写/失效全部从 `ctx` 解析租户，不直接用全局对象字段。
2. **封装租户缓存 key 工具**：集中生成 key，例如 `TenantKey(ctx, "app-configs", group)`，避免手工拼接。
3. **修正关键失效点**：重点检查 AppConfig、Language、SystemConfig 等热点路径的删除与重建逻辑。

### P1

1. **引入 tenant-aware cache helper**：在 `center` 层提供统一方法（Get/Set/Del/SAdd/HSet with tenant）。
2. **打通 DB 更新与缓存失效**：将 `AfterCreate/AfterUpdate/AfterDelete` 与 cache invalidation 显式绑定，减少遗漏。
3. **补充回归用例**：至少覆盖“同 key 不同 tenant 不串数据”“更新后旧缓存失效”两类场景。

### P2

1. **增加观测指标**：按租户统计缓存命中率、key 数量、失效次数。
2. **评估引入版本戳策略**：key 增加 tenant-config version，降低批量删 key 成本与一致性风险。

## 5. 是否有保留必要

结论：**有保留必要，建议保留并增强。**

理由：

- 当前实现已形成可工作的租户隔离路径（key 前缀 + 作用域过滤）。
- 问题主要集中在“治理与一致性”，而非架构方向错误。
- 相比重构缓存体系，先做 tenant-aware 封装与失效治理的投入产出更高。

## 6. 建议的短期落地清单（2~4 周）

1. 新增 `tenant cache key helper` 并替换 AppConfig/Language 现有拼接点。
2. 修复 AppConfig 缓存清理租户来源逻辑，改为从 `ctx` 获取。
3. 增加 3 个集成测试：
   - 多租户同名配置不串读；
   - 更新后旧缓存不可见；
   - 租户识别失败时不写缓存。
4. 输出一页运维排障说明：如何按 tenantID 排查缓存。
