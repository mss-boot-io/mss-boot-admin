# mss-boot-admin-antd 去多租户改造交接摘要

> 交接时点：2026-02-28（基于仓库当前可见代码）

## 一页结论

- 前端多租户能力当前主要集中在“租户管理入口与租户语义展示”，尚未形成统一的“租户上下文驱动请求”前端架构。
- 去多租户改造可执行，且可按“入口下线 → 页面服务删除 → 类型收敛”分阶段完成。
- 关键依赖是后端接口与 OpenAPI 同步下线；否则前端类型会反复回填。

## 适用范围与假设

- 仅适用于 `mss-boot-admin-antd` 当前代码基线。
- 假设后端会提供相应的单租户接口/Swagger 同步。
- 若后端仍长期保留租户字段，前端建议采用兼容收敛而非一次性硬删除。

## 已识别关键触点（供实施团队核对）

- 路由：`config/routes.ts`（`/tenant`、`/tenant/:id`）。
- 页面：`src/pages/Tenant/index.tsx`。
- API：`src/services/admin/tenant.ts`。
- 服务聚合：`src/services/admin/index.ts`（`tenant` 导出）。
- 模型页字段：`src/pages/Model/index.tsx`（`multiTenant`）。
- 文案：`src/locales/zh-CN/menu.ts`、`src/locales/en-US/menu.ts`、`src/locales/zh-CN/pages.ts`（及同类语言包）。
- 类型：`src/services/admin/typings.d.ts`（`Tenant` 与 `tenantID` 相关字段/参数）。

## 推荐实施顺序

1. 下线路由与菜单文案，阻断新入口。
2. 删除租户页面与 API 客户端，清理导出引用。
3. 更新 typings 与 `multiTenant` 展示逻辑。
4. 完成 lint/build/关键页面回归并联调后端。

## 回归检查清单

- 无 `tenant` 页面访问入口。
- 菜单渲染无空节点、无异常跳转。
- 用户、角色、模型、配置页面可正常使用。
- 构建与类型检查通过。

## 可复核操作（建议）

- 关键字扫描：`grep -RIn "tenant\|multiTenant\|tenantID" src config`。
- 工程检查：`pnpm run lint`、`pnpm run build`（若启用测试，再执行 `pnpm run test`）。
- 功能回归：登录后验证菜单渲染、关键页面路由与基础操作。

## 协作要求（开源）

- 变更说明保持中立、可验证，并注明“适用范围与假设”。
- 不写入敏感信息（密钥、凭据、私有地址、个人数据）。
- 每个 PR 提供复现步骤与验证结果，便于外部贡献者复核。
