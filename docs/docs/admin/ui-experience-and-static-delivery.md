---
title: Admin UI 体验与静态交付基线
order: 31
nav:
  order: 1
  title: admin
description: Admin 前端的产品体验、可访问性、响应式与生产静态资源交付约束
keywords: [admin ui ux accessibility responsive performance nginx delivery]
---

## 适用范围

本文适用于 PR [#465](https://github.com/mss-boot-io/mss-boot-admin/pull/465) 及其合并后的 `main`，
实现基线为 `46856c9`。机器可执行需求以
`.mss/features/admin-ui-experience-quality.yaml` 为准；本文说明产品决策、发布方式和后续治理边界。

本轮只调整 `web/antd/` 和前端交付流水线，不修改后端 API、数据库结构或授权语义。
后端仍是权限判断的唯一权威来源，前端菜单和按钮过滤只用于改善体验。

## 产品体验基线

### 真实状态优先

- 首次请求未完成时显示加载状态，不能提前显示“暂无数据”。
- 首次请求失败时显示明确错误和重试入口，不能伪装成空结果。
- 后台刷新失败时保留最近一次成功数据，避免界面无故回退为空。
- 可见成功提示必须发生在对应 API 成功之后。

当前实现覆盖用户依赖、通知中心以及用户、通知和任务的移动列表。用户列表的角色、岗位和部门数据
仅用于补充显示名称：列表页会按当前身份已有权限选择性加载，缺少权限或单项失败时回退显示原始 ID；
新增和编辑表单仍要求完整依赖并提供重试。

### 桌面与移动端行为一致

- 应用设置在窄屏使用顶部可横向滚动的标签页，避免固定左侧栏挤压表单。
- 移动用户列表保留重置密码入口，移动任务列表保留 Start/Stop 操作。
- Task Start/Stop 使用同步防重锁；请求期间同一动作不能重复提交，其他任务操作暂时禁用。
- 通知中心是只读收件箱，不展示没有后端工作流支撑的新增、编辑或假删除入口。

### 导航和工作台

- `/workplace` 是唯一工作台地址，`/welcome` 和 `/analysis` 只做兼容重定向。
- 顶部搜索只检索后端已授权菜单树中的可导航节点，并按身份和权限刷新版本隔离缓存。
- 工作台监控图使用轻量 SVG，不再加载完整 `@ant-design/charts` 运行时。
- CPU 和内存趋势按真实时间戳定位，采样缺口不会被绘制成等间隔数据。

### 可访问性

- 页面允许浏览器缩放，不限制 `user-scalable` 或最大缩放比例。
- 通知触发器、帮助入口和安全操作使用原生按钮或链接语义。
- 通知弹层打开后焦点进入弹层，Tab 与 Shift+Tab 在可交互元素间循环，Escape 和“查看更多”关闭后
  将焦点还给通知按钮。
- 监控图提供本地化的可访问名称；首屏加载器支持 `aria-live` 和 `prefers-reduced-motion`。

## 生产静态交付

### 构建预算

生产构建命令：

```shell
cd web/antd
corepack pnpm@9.15.9 build:prod
```

`web/antd/scripts/check-bundle-budget.mjs` 使用与 Nginx 一致的 gzip level 6 统计 JavaScript：

| 资源 | 默认上限 |
| --- | ---: |
| 入口脚本 | 575 KiB gzip |
| 任一异步脚本 | 250 KiB gzip |

预算环境变量必须是有限的正数，非法值会使构建失败。前端 CI 和 release 工作流都在构建后执行预算检查，
避免只在本地提示而不阻断发布。

### Nginx 缓存与压缩

`web/antd/nginx.conf` 和 `web/antd/Dockerfile` 定义以下交付语义：

- JavaScript、CSS、SVG 和 JSON 支持 gzip；带标准 `Via` 代理头时仍启用压缩。
- 带内容哈希的静态资源使用一年 `immutable` 缓存。
- `index.html` 与 service worker 使用 `no-store`，保证新版本入口及时生效。
- 缺失的哈希资源返回带 `Cache-Control: no-store` 的 404，避免发布切换期长期缓存错误响应。
- 非文件 SPA 路径回退到 `index.html`。

交付 smoke 命令会构建临时镜像并验证上述响应头和状态码，完成后清理容器与镜像：

```shell
cd web/antd
corepack pnpm@9.15.9 delivery:smoke
```

## 验证证据

PR #465 的合并前验证基线：

- `go run ./cmd/mss verify --changed --format json`
- `corepack pnpm@9.15.9 tsc`
- `corepack pnpm@9.15.9 lint:js`
- `corepack pnpm@9.15.9 test -- --runInBand`：59 个测试套件、297 个测试
- `corepack pnpm@9.15.9 build:prod`：入口 548.68 KiB gzip，最大异步块 202.19 KiB gzip
- `corepack pnpm@9.15.9 delivery:smoke`
- Codex 内置浏览器：桌面与 375px、light 与 realDark，覆盖工作台、应用设置、个人设置、用户、通知、
  任务、键盘焦点、头像和横向溢出

Jest 必须串行执行；多个进程同时生成 `.umi-test` 会互相删除临时产物并产生假失败。

## 发布、安全与回滚

- 发布不需要数据库迁移，先完成生产构建和 delivery smoke，再发布新的前端镜像。
- 静态资源采用内容哈希，可以与旧资源并存；HTML 不长期缓存，发布后会逐步指向新资源。
- 本轮不放宽任何 API 权限，授权菜单搜索不会生成或推测新权限。
- 回滚时直接恢复上一前端镜像，不需要数据回滚；如果 CDN 存在额外规则，需要同时确认 HTML 和 404
  未被覆盖为长期缓存。

## 已知后续项

以下项目不阻断本轮发布，下一阶段按顺序处理：

1. 为 Account Base 和 Security 的 fallback 请求补齐错误与重试状态。
2. 为剩余动态 `Result` 面板统一 `role="alert"` 或合适的 live-region。
3. 对超长移动通知标题增加省略和可展开详情测试。
4. 将 Docker 基础镜像从可变 `nginx` 标签固定到经过验证的版本或摘要。
5. 更新 Browserslist 数据并记录包体基线变化。

完成定义：每项都有中英文文案、focused test、`pnpm tsc`、`pnpm lint:js`、全量 Jest，以及 375px
内置浏览器验收；涉及交付配置时还必须通过 `build:prod` 和 `delivery:smoke`。

