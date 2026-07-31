# mss-boot-admin-antd 去多租户改造影响评估与任务计划

> 评估时点：2026-02-28（基于仓库当前可见代码）

## 目标与边界

- 目标：在前端仓库中完成“单租户化”所需改造，移除多租户管理入口与显式租户语义。
- 边界：仅覆盖 `mss-boot-admin-antd`；后端接口与权限策略需并行评审。
- 原则：可回滚、可验证、分批提交。

## 范围外事项（避免误解）

- 不包含后端数据库迁移与权限模型重构。
- 不包含网关、DNS、部署编排与线上切流策略。
- 不包含新增产品功能，仅做既有多租户能力收敛。

## 影响面分层

### A. 高影响（建议优先改）

1. 路由层

   - 文件：`config/routes.ts`
   - 内容：删除 `/tenant`、`/tenant/:id` 路由。

2. 页面层

   - 文件：`src/pages/Tenant/index.tsx`
   - 内容：删除租户管理页面或改为占位跳转页（推荐直接删除并清理引用）。

3. 服务层

   - 文件：`src/services/admin/tenant.ts`
   - 内容：下线租户相关 API client。

4. 菜单/文案层
   - 文件：`src/locales/zh-CN/menu.ts`、`src/locales/en-US/menu.ts`、`src/locales/zh-CN/pages.ts`（及同类语言包）
   - 内容：删除租户菜单与页面文案键。

### B. 中影响（建议同步）

1. 类型层

   - 文件：`src/services/admin/typings.d.ts`
   - 内容：
     - 删除 `Tenant` 类型与 `getTenants*`、`postTenants*`、`putTenants*`、`deleteTenants*` 参数类型；
     - 评估保留 `tenantID` 字段的必要性（若后端单租户后不再返回，建议清理）。

2. 模型配置层
   - 文件：`src/pages/Model/index.tsx`
   - 内容：评估 `multiTenant` 开关是否仍应展示；单租户目标下建议移除该字段展示。

### C. 低影响（清理项）

- 相关 imports、导出聚合（如 `src/services/admin/index.ts` 中 `tenant` 导出）。
- 可能存在的测试快照、mock 数据、文档描述。

## 分阶段实施建议

### 阶段 1：入口下线（低风险）

- 删除租户路由与菜单文案键。
- 确认用户无法从 UI 进入租户页面。

### 阶段 2：页面与服务清理（中风险）

- 删除 `src/pages/Tenant/index.tsx` 与 `src/services/admin/tenant.ts`。
- 清理 `src/services/admin/index.ts` 中聚合导出。

### 阶段 3：类型与字段收敛（中风险）

- 更新 `typings.d.ts`（优先通过 OpenAPI 重新生成，避免手改漂移）。
- 评估并移除 `Model` 页 `multiTenant` 字段。

### 阶段 4：回归与发布（中风险）

- 执行 `pnpm lint`、`pnpm test`（若项目启用）、`pnpm build`。
- 与后端联调登录、菜单、模型管理等关键路径。

## 风险与缓解

1. 后端仍返回租户字段

   - 风险：前端类型与页面不一致。
   - 缓解：先做兼容删除（页面隐藏），再按后端版本切换做类型硬删除。

2. 菜单权限残留

   - 风险：菜单树出现空节点或 403 跳转异常。
   - 缓解：发布前跑一轮角色菜单回归（超级管理员 + 普通管理员）。

3. 自动生成代码回填

   - 风险：OpenAPI 重新生成后租户类型被带回。
   - 缓解：在生成配置中排除租户接口，或同步后端下线 Swagger 项。

4. 分支间结论偏差
   - 风险：不同分支代码状态不一致，导致评估和实际改造差异。
   - 缓解：实施前固定基线 commit，并在 PR 描述中注明评估基线。

## 验收标准（建议）

- 代码层：`tenant.ts`、`Tenant` 页面、`/tenant` 路由与对应文案全部移除。
- 功能层：登录、菜单渲染、用户/角色/模型页面可正常访问。
- 类型层：`pnpm build` 无 `tenant` 相关类型报错。
- 联调层：与后端单租户分支联调通过。

## 建议 PR 切分

1. PR-1：路由 + i18n 菜单下线。
2. PR-2：页面 + 服务删除。
3. PR-3：typings 与 model 页字段收敛。
4. PR-4：联调修复与文档更新。

## 可复现验证步骤（建议）

1. 变更前后分别执行：`grep -RIn "tenant\|multiTenant\|tenantID" src config` 对比触点。
2. 每个 PR 合并前执行：`pnpm run lint` 与 `pnpm run build`。
3. 若存在测试：执行 `pnpm run test`。
4. 记录回归结果：菜单可见性、路由访问、关键页面 CRUD。
