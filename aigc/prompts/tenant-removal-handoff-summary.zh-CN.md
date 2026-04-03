# 去多租户改造交接文档（下次继续用）

> 适用范围：基于当前仓库可见代码与本轮评估结果（截至 2026-02-28）。
>
> 目标背景：SaaS 需求显著下降，项目准备从多租户架构收敛为简化单租户方案，不要求兼容历史租户使用场景。

## 1. 已完成结论汇总

## 1.1 架构判断

- 当前多租户实现是“框架通用能力 + admin 落地实现”组合。
- 若追求简化，去多租户可行，但属于中大型重构，需分批执行。

## 1.2 缓存维度补充结论

- 业务缓存 key 已使用 tenant 前缀隔离（如 language/app-configs）。
- 底层 storage cache 抽象并不内建 tenant namespace，隔离依赖业务约定。
- 删除多租户后需同步重构缓存 key 与失效逻辑，避免脏缓存与路径残留。

## 1.3 是否保留多租户

- 在“无需兼容 + SaaS 需求低”的前提下，可走彻底简化路线。
- 推荐实施方式：分阶段落地，但最终目标可为物理删除租户链路。

## 2. 影响范围（量化）

- `mss-boot-admin` 触点文件：47
- `mss-boot` 触点文件：9
- 合计：56（关键词扫描去重结果）

主要涉及模块：

1. 中心抽象与注入：`center/*`、`cmd/server/*`、`cmd/migrate/*`
2. 租户实体与 API：`models/tenant*`、`apis/tenant.go`、`dto/tenant.go`
3. 鉴权与数据权限：`middleware/auth.go`、`apis/*` 中 `WithScope(center.Default.Scope)`
4. 模型基类：`ModelGormTenant` 及其继承模型
5. 缓存与存储：`service/app_config.go`、`apis/language.go`、`service/storage.go`
6. 虚拟模型：`mss-boot/virtual/*` 与 `apis/virtual.go`
7. 文档/提示词：`.github/*`、`aigc/prompts/*`

## 3. 风险分级

## 高风险

- 鉴权与数据范围联动变化导致权限放大。
- 迁移脚本中 `tenant_id` 索引与初始化数据调整不完整，影响启动与迁移。
- 虚拟模型 `MultiTenant` 行为变化导致动态模型接口语义改变。

## 中风险

- 缓存 key 改造后历史 key 残留与失效时机问题。
- 上传路径去 tenant 目录后的访问 URL/资源定位变化。

## 低风险

- 文档、提示词、注解规范同步。

## 4. 已规划任务（可直接执行）

## PR-1：核心骨架去租户

- 调整 `center` 接口与默认实现，移除 `TenantImp/TenantMigrator` 主依赖。
- `cmd/server`、`cmd/migrate` 去掉租户注入与初始化逻辑。
- `middleware/auth` 移除 tenant 校验分支。

## PR-2：模型与控制器去租户

- `ModelGormTenant` 收敛（替换为基础模型或删除）。
- 业务模型移除 `tenant_id/TenantID` 依赖。
- `apis` 中 `WithScope(center.Default.Scope)` 批量下线或替换。

## PR-3：缓存/存储/虚拟模型收口

- 去租户缓存前缀与失效逻辑重构。
- 上传路径去 tenant 目录。
- `virtual/model` 与 `virtual/action` 去 `MultiTenant/TenantScope/TenantIDFunc`。

## PR-4：迁移与文档收尾

- 清理迁移脚本中的 `tenant_id` 索引、租户表/种子逻辑。
- 更新 README、copilot 指令、提示词模板。

## 5. 下次开工建议顺序

1. 先开 PR-1（编译骨架先通）
2. 再做 PR-2（最大改动面）
3. 然后 PR-3（运行时一致性）
4. 最后 PR-4（迁移和文档）

## 6. 下次启动时的检查清单

开始前：

- 确认“是否保留 tenant 表但停用”还是“物理删除 tenant 表”（建议直接物理删除）。
- 确认是否同步调整 `mss-boot` 中 `security.Verifier` 的 `GetTenantID()` 接口。

执行后（每个 PR 都做）：

- `go test ./...`（至少跑变更相关目录）
- 启动路径验证：`go run main.go server`
- 迁移验证：`go run main.go migrate`

## 7. 相关参考文档

- `aigc/prompts/tenant-removal-impact-plan.zh-CN.md`
- `aigc/prompts/multi-tenant-cache-evaluation.zh-CN.md`
- `aigc/prompts/action-architecture-review.zh-CN.md`
