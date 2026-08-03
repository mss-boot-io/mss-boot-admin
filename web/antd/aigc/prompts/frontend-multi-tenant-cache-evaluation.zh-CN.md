# mss-boot-admin-antd 多租户前端评估（含缓存与状态维度）

> 评估时点：2026-02-28（基于仓库当前可见代码）

## 适用范围与假设

- 适用仓库：`mss-boot-admin-antd`（当前可见代码范围）。
- 评估目标：从前端视角梳理多租户能力的实现位置、收益、复杂度和去除成本。
- 假设：后端仍提供现有接口；本评估不覆盖线上运维配置与网关层。

## 证据来源（代码可复核）

- 路由：`config/routes.ts`。
- 页面：`src/pages/Tenant/index.tsx`、`src/pages/Model/index.tsx`。
- 服务：`src/services/admin/tenant.ts`、`src/services/admin/index.ts`。
- 类型：`src/services/admin/typings.d.ts`。
- 文案：`src/locales/zh-CN/menu.ts`、`src/locales/en-US/menu.ts`、`src/locales/zh-CN/pages.ts`。

## 当前实现观察（可复核）

### 1) 路由与页面层

- 存在独立租户管理路由：`/tenant`、`/tenant/:id`（`config/routes.ts`）。
- 存在完整租户页面：列表、详情/编辑、创建、删除、域名编辑（`src/pages/Tenant/index.tsx`）。

### 2) API 客户端层

- 存在租户专用客户端：`src/services/admin/tenant.ts`。
- 覆盖接口：`GET/POST /admin/api/tenants`、`GET/PUT/DELETE /admin/api/tenants/:id`。

### 3) 类型系统与模型配置层

- OpenAPI typings 中存在大量 `tenantID` 字段（如 `AppConfig`、`Department` 等，见 `src/services/admin/typings.d.ts`）。
- 模型管理页面中存在 `multiTenant` 字段（`src/pages/Model/index.tsx`），表明“模型是否多租户”在前端可配置。

### 4) 国际化与菜单语义层

- 菜单与页面文案存在租户相关键值（如 `menu.super-permission.tenant.*`、`pages.tenant.*`、`pages.fields.tenantID`）。
- 中文与英文词条均可见（`src/locales/zh-CN/menu.ts`、`src/locales/en-US/menu.ts`、`src/locales/zh-CN/pages.ts`）。

### 5) 缓存与状态维度（前端）

- 当前代码中未观察到“显式租户切换态 + 本地缓存命名空间”这一前端机制。
- 前端以“租户管理页面 + 租户字段呈现”为主，而非“租户上下文驱动所有请求”。
- 因此，前端缓存复杂度主要体现在：
  - 字段/列表中是否显示 `tenantID`；
  - 若后续新增客户端缓存（SWR/React Query/本地缓存），需要避免跨租户 key 复用。

## 设计收益与代价（前端视角）

### 已获得收益

- 平台管理员可在 UI 直接管理租户生命周期（增删改查与域名）。
- 多租户字段在类型与页面中可见，便于与后端数据模型对齐。

### 主要代价

- 菜单、路由、页面、服务、i18n、typings 形成多点耦合。
- 文案与类型冗余增加维护面，影响代码生成/升级后的清理成本。
- 若未来要做“单租户化”，前端需做跨层一致裁剪，不能只删页面。

## 是否建议保留多租户（条件化结论）

该结论依赖产品边界：

- 若产品明确继续服务多组织/多域名运营，建议保留前端多租户入口，但可收敛为“平台管理专属能力”，避免扩散到普通页面。
- 若产品转向单租户部署，建议执行前端去多租户改造：删除租户管理入口、清理租户相关类型与文案，并与后端接口裁剪同步发布。

## 可执行改进建议（不改变业务语义）

1. 建立“租户相关清单”基线文件（路由、页面、服务、i18n、typings）并纳入 PR 检查。
2. 将 `multiTenant` 字段改为后端能力探测后显示，减少误配。
3. 约定缓存 key 规范：若出现客户端缓存，统一前缀策略（例如包含 `tenantID` 或部署级固定值）。
4. 将租户文案键统一归组（如 `tenant.*`），降低未来批量迁移成本。

## 验证建议

- 路由验证：确认移除/保留策略后，`/tenant` 路由与菜单权限一致。
- API 验证：租户页面操作与 `tenant.ts` 请求结果一致，错误提示可回归。
- i18n 验证：中英文资源中租户键无死链、无残留引用。
- 类型验证：`typings.d.ts` 更新后，页面编译无 `tenantID` 相关类型错误。

## 可复现检查步骤（示例）

1. 搜索租户触点：`grep -RIn "tenant\|multiTenant\|tenantID" src config`。
2. 运行类型与构建检查：`pnpm run build`。
3. 若启用 lint/test：`pnpm run lint`、`pnpm run test`。
4. 人工回归：登录后检查菜单、`/tenant` 页面可达性、`Model` 页面字段展示。
