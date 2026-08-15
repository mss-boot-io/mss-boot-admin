---
title: Ant Design 6 独立应用评估与实施方案
order: 15
nav:
  order: 1
  title: admin
description: web/antd-v6 的版本基线、架构边界、功能迁移、独立发布、验证与回滚方案
keywords: [admin ant-design v6 react umi migration release]
---

# Ant Design 6 独立应用评估与实施方案

## 文档状态

- 决策状态：已接受，按独立应用方案实施。
- 目标目录：`web/antd-v6`。
- 旧应用：`web/antd` 保持独立构建、发布、部署和回滚，不在本项目中原位升级或删除。
- 当前阶段：P0/P1、P2 已完成；P3 的身份启动链、授权菜单求交、权限新鲜度、分层主题闭环，以及安全账户中心、个人资料、PAT、OAuth 连接、语言切换已实现。服务端授权 revision 实时推送和其余业务等价仍在进行。生成器和浏览器验收尚未完成，因此不是生产发布候选。
- 机器契约：`.mss/features/admin-antd-v6-application.yaml`。
- 架构决策：`docs/adr/2026-08-15-independent-ant-design-v6-application.md`。

## 结论

采用“最新正式 Ant Design Pro 工程实践作为骨架、实施日兼容的最新稳定核心包、唯一经批准的 ProComponents v3 beta 例外”的方案。迁移保留业务和安全契约，不复制旧前端的源码结构、视觉缺陷或已淘汰实现。

V6 必须是单独应用，而不是 V5 的子路径构建：它有自己的 manifest、lockfile、Node/pnpm 版本、源码、构建产物、镜像、tag、CI、发布环境和回滚历史。后端在双应用并行期保持兼容。

## 实施日版本基线

以下版本在 2026-08-15 重新读取 npm dist-tags，并通过本仓库冻结安装和生产构建验证：

| 层 | 精确版本 | 决策 |
| --- | --- | --- |
| Ant Design Pro 上游骨架 | v6.0.2 / `2b453c67b535b76f5f95d6542397a4b987b61de2` | 只作为可追踪工程来源，不原样复制 lockfile |
| React / React DOM | 19.2.8 | 最新稳定，保持单一业务运行时 |
| Ant Design | 6.6.0 | 用户确认采用，精确锁定 |
| Ant Design Icons | 6.3.2 | 与 antd 6 同步升级 |
| ProComponents | 3.1.14-6 | v3 仍为 beta；唯一预发行例外，禁止 `^` 漂移 |
| Umi Max | 4.7.5 | 最新正式版；必须持续检查其过渡依赖图 |
| React Query | 5.101.4 | 服务端状态唯一缓存所有者 |
| antd-style / Tailwind | 4.1.0 / 4.3.3 | 分别负责 token 感知复杂样式和布局工具类 |
| TypeScript / Biome / Vitest | 7.0.2 / 2.5.8 / 4.1.10 | 最新稳定工程工具 |
| Node / pnpm | 24.14.0 / 10.34.5 | pnpm 11 在当前 Node 24 + WSL 验证中出现 SQLite store 死锁，暂用最新 pnpm 10 补丁 |

所有直接依赖使用精确版本并提交 `pnpm-lock.yaml`。升级按 React/antd、Umi/build、ProComponents 三组分别评审，不能一次性执行无边界的 `latest all`。

## 必须接受的风险和处理

### ProComponents v3 仍是 beta

稳定的 ProComponents 2.x 不支持 antd 6；完整使用 ProLayout、ProTable 和 ProForm 必须采用 v3 beta。当前方案精确锁定一个 beta，升级前要求 CRUD 回归、类型检查、视觉基线、bundle 和 E2E 全部通过。若未来风险不可接受，回退方案是只使用 antd 6 原生 Layout/Table/Form，自行维护 Pro 能力，而不是混装 v2/v3。

### Umi 依赖图处于过渡期

Umi Max 4.7.5 的工具链仍解析出嵌套 antd 4.24.16 和 React 18.3.1。当前生产 bundle 检查已证明它们没有进入业务运行时，但不能仅凭顶层 `package.json` 判断安全。每次更新继续执行依赖树、去重、React 单例、运行时 marker、bundle budget 和生产审计。

### Cookie 会话不能只改前端

V5 使用浏览器 bearer token；该模式不应复制到 V6。V6 的目标是同源 HttpOnly Secure SameSite cookie、变更请求 CSRF、防重放 WebSocket ticket，并保证 V5 bearer 流程在并行期可用。后端兼容支持、正反安全测试和发布顺序完成前，V6 登录不得进入生产。

## 架构职责

| 职责 | 唯一所有者 | 禁止做法 |
| --- | --- | --- |
| HTTP 传输、鉴权、统一错误、文件与 204/201 | typed transport/OpenAPI adapter | 页面直接拼接重复的请求协议 |
| 服务端资源缓存、失效和乐观更新 | React Query 5 | 同一资源同时放在 model、组件 state 和 query cache |
| 当前身份和启动关键客户端状态 | Umi initialState | 长期缓存业务列表或详情 |
| 品牌、主题、密度、组件变量 | antd 6 ConfigProvider token/CSS variables | 复制旧 Less 变量或覆盖内部 `.ant-*` 层级 |
| 页面布局与间距 | Tailwind 4 | 与 CSS Modules/antd-style 重复声明同一规则 |
| 局部静态样式 | CSS Modules | 依赖 CSS-in-JS hash 类名 |
| token 感知复杂样式 | antd-style 4 | 用它替代所有简单布局样式 |
| 页面权限 | 后端 RBAC 为最终权威，前端 access 改善体验 | 只隐藏按钮而不保护 API |
| 动态菜单 | 编译期 route registry 与后端授权菜单求交集 | 从数据库 component 字符串任意动态 import |

页面必须考虑 loading、确认空态、可重试错误、403、404、并发冲突、桌面/移动、中文/英文、键盘焦点和控制台零弃用警告。桌面与移动默认共享一个响应式业务组件。

## 功能迁移矩阵

“保留”表示保持业务/API/权限语义；“重写”表示不搬运旧实现。

| 能力 | 目标处理 | 关键验收 |
| --- | --- | --- |
| 登录、退出、刷新、OAuth | 安全会话适配后重写 | cookie/CSRF、OAuth token 不进浏览器存储、过期和退出闭环 |
| 当前用户、账户中心、账户设置、PAT | 垂直 identity/account 模块 | self 权限、PAT 只显示一次、旋转后旧值失效 |
| 工作台和监控摘要 | React Query + 按需图表重写 | 部分失败可诊断、移动端可用、轮询可取消 |
| 用户、角色、菜单 | ProTable/ProForm v3 重写 | root 与委派权限、ETag/If-Match 冲突、直接 API 负例 |
| 部门、岗位 | 共用组织目录 primitives | 树/列表响应式、引用完整性、空态与冲突 |
| 任务 | CRUD 与运行记录分离 | 状态刷新、失败信息、权限动作一致 |
| 通知 | 列表与已读变更重写 | 未读数失效、WebSocket 只触发 query invalidation |
| 语言 | 运行时资源适配 | zh-CN/en-US key 同步、缓存与刷新契约 |
| Option | schema 表单重写 | 类型、默认值、删除/更新语义一致 |
| AppConfig / SystemConfig | 类型化配置编辑器 | secret 不回显、root/配置权限、缓存 revision 一致 |
| 日志、审计、登录日志、在线会话 | 只读/撤销切片 | 敏感字段脱敏、分页筛选、撤销正反例 |
| 分层主题 | v6 token adapter | code < application < user、ETag 冲突、首屏与跨标签同步 |
| Supplier | 首个 v6 generator golden | route/locale/client/page/E2E 可重复生成且两次运行无漂移 |
| PWA、动态模型、浏览器代码生成、Pro demo、AI/analytics | 不迁移 | 构建不得生成 service worker 或演示外部依赖 |

## 分阶段实施

### P0：契约和兼容性试验

- 固定上游 commit、版本矩阵、Node/pnpm 和 lockfile。
- 将两个前端写入 Project、doctor、dev、verify、commands 和 release policy。
- 验证 Umi 过渡依赖、React 单例、antd 运行时和 bundle。
- 产出 FeatureSpec、ADR 和本实施文档。

完成门：冻结安装、依赖契约、生产构建和规格校验通过。

### P1：独立工程和发布骨架

- 创建 React 19 + antd 6.6.0 应用壳、主题、locale、access、route registry、QueryClient、统一请求和页面状态 primitives。
- 创建独立 CI、`web/antd-v6/v{version}` 发布工作流、`mss-boot-admin-antd-v6` 镜像、构建身份、checksum、SBOM/provenance 和 Nginx smoke。
- 明确未完成 Feature 在 release qualification 中 fail-closed。

完成门：lint、TypeScript、Vitest、bundle、审计和发布治理测试通过；仍不可生产发布。

### P2：后端兼容会话安全

- 增加 opt-in cookie 登录和刷新，不删除 bearer header 支持。
- 对 cookie 认证的变更请求执行 CSRF；header bearer 客户端保持兼容。
- 增加短时、单次 WebSocket ticket 和 Origin 校验，移除 V6 query token。
- 覆盖登录、刷新、退出、过期、重放、跨站、OAuth 和 V5 兼容负例。

实现状态：代码与聚焦安全测试已完成。新能力默认关闭，只有同时启用
`auth.sessionEnabled` 和 `auth.browserSession.enabled` 才可使用；生产配置还会强制
`auth.browserSession.secure: true`。V6 使用独立的 `/user/session/*` 登录、刷新和
OAuth callback，不读取响应中的 JWT；V5 的 `/user/login`、`/user/refresh-token`
和 bearer header 契约保持不变。REST 已不再接受 query token；旧
`/ws/connect?token=...` 仅由专用兼容中间件读取，并受独立开关控制。

完成门：后端安全矩阵通过，先部署兼容后端，再启用 V6 登录。

推荐上线顺序：

1. 先发布默认关闭 browser session 的兼容后端，保持 V5 WebSocket 兼容开关开启。
2. 为目标环境配置唯一的 HTTPS 应用/CORS origin、`X-CSRF-Token` header、共享 Redis、生产 auth key、Secure cookie，以及不覆盖 V5 的 V6 专用 OAuth 应用、密钥和精确 callback URI。
3. 开启 server session 与 browser session，先执行 cookie、CSRF、OAuth、ticket、退出和过期的环境 smoke。
4. 独立发布 V6 镜像并小流量启用；V5 的镜像、域名、tag 和回滚路径不变。
5. 仅在 V5 完成退役后关闭 `legacyWebSocketQueryTokenEnabled`；后端兼容端点的删除另立变更。

### P3：身份、菜单、主题和账户

- 完成 current user、RBAC access、授权菜单求交、权限刷新、账户、PAT、OAuth、语言和主题适配。
- 不允许未知菜单路径加载组件；权限变化后清理受影响 query。

当前已完成的启动底座：

- current user 在传输边界按后端真实契约规范化：权限是精确布尔 map，root 只来自 `role.root`，畸形响应不会降级成匿名身份。
- 401 才表示会话缺失；身份或策略的 5xx 与授权菜单失败会显示可重试的 fail-closed 状态，不再错误跳转登录或伪装为空菜单。
- 后端 `/welcome` 能力显式映射到编译期 `/workplace` 页面；数据库 `component`、外链和未知路径不能成为可执行路由，未知目录只能保留为无链接容器。
- 关闭 Max 自动创建的私有 React Query client，由应用提供唯一 QueryClient，使启动查询、页面 hook、退出清理和后续权限失效共享一套缓存。
- application public profile 与 current-user canonical theme 解析为七字段稀疏层，按 code < application < user 求值，再通过 antd 6 CSS Variables、dark algorithm 和语义 token 应用；读取失败只降级到有效低层并留下 scope 状态。
- canonical theme transport 已支持 vendor media type、强 ETag、`If-Match`、412 authoritative resource 和无静默重试。
- `/app-config` 与 `/account/settings` 复用同一个显式 scope 主题编辑器；应用读取、应用控制和个人 self-service 分开判定，逐字段继承、整层重置、来源、草稿、降级与 412 决策均有中英文状态。
- 主题保存和重置立即更新唯一 Query cache、Umi 布局和 antd 6 ConfigProvider；跨标签事件有 schema、TTL、去重、单调 revision 与个人随机会话绑定，同 revision 分歧只触发权威重读。
- 首屏快照只包含规范化七字段、revision 和过期时间，最长 24 小时；应用层可匿名使用，个人层必须同时匹配随机会话和已验证用户主体，Web Locks 不可用时拒绝持久化而不是冒险覆盖新修订。
- 密码登录和 OAuth 在取得规范化 current user 后轮换主题会话；退出先清除本标签个人派生状态，服务端退出完成后再通知其他标签重启，避免 cookie 撤销竞态。
- 账户中心与响应式设置页已复用同一个规范化 current-user query；个人资料提交只含后端明确允许的字段，用户名和邮箱只读，服务端以精确 allowlist 拒绝身份字段、未知字段和错误类型，并可正确持久化显式空值。
- PAT 列表只解析元数据；创建与旋转返回的原始 secret 不进入 React Query mutation cache、持久存储或 URL，只存在于不可跳过的一次性内存弹窗中。撤销、旋转和创建均继续由后端 self 权限约束，PAT 本身不能调用这些交互端点。
- OAuth 登录与连接入口只在 public application profile 明确启用 provider 时显示；authorize/callback 使用服务端单次 state 和 cookie 会话，回调同时校验业务成功码、provider 和 intent，provider token 不进入浏览器响应。旧的直接解绑入口暂不迁移。
- Umi 官方 `SelectLang` 已进入登录页和全局布局，中文/英文 catalog 由测试保证 key 完全一致。
- 权限新鲜度桥在显式变更事件、跨标签 BroadcastChannel、403、网络恢复，以及节流后的 focus/visibility 上权威重读 current user 与编译期菜单求交；权限或可执行菜单签名变化时先取消并移除业务 Query 和 mutation cache，只保留身份、菜单、public profile 与主题启动 Query。身份/菜单不能确认时立即切换到可重试的 fail-closed 页面；异常主体切换会清空个人主题和所有缓存。
- Max 的过渡 Moment 消费通过官方 `moment2dayjs` 适配统一到 Day.js；release bundle 门禁会拒绝 Moment 进入生产图。当前权限新鲜度切片后的生产构建 entry 为 4.16 KiB、总 JS 为 733.34 KiB、最大异步分包为 195.57 KiB，均通过预算门禁。

本阶段主动冻结三个旧契约缺陷，未按“表面等价”复制：

1. 邮箱是登录与恢复身份，在专用双重验证变更流程完成前保持只读。
2. 旧的已登录密码重置不要求当前密码或近期再认证；V6 不展示该操作，待增加 step-up/re-auth、限流及成功后全会话/PAT 撤销契约。
3. 旧 OAuth 解绑可直接删除最后一种可用登录方式；V6 只允许安全连接，待后端事务性保证至少保留一种已验证登录方式并要求近期再认证后再开放解绑。

仍待本阶段完成：把后端全局授权 revision 接入安全 WebSocket 作为无焦点场景的即时刷新提示、上述密码/邮箱/解绑安全契约的产品确认与后端实现，以及真实浏览器权限矩阵。focus/visibility 与 403 权威重读保留为 WebSocket 不可用时的安全兜底。

完成门：匿名/root/普通用户/撤权/未知菜单/直接路由和直接 API 矩阵通过。

### P4：只读和运维能力

- 迁移工作台、日志、监控、在线会话、通知和运行记录。
- 建立 Query key、轮询、失效、取消和边界状态规范。

完成门：桌面/移动、zh-CN/en-US、错误降级和资源上限 E2E 通过。

### P5：核心 CRUD

- 按用户与组织、角色与菜单、任务、语言与 Option、AppConfig/SystemConfig 分批迁移。
- 每个切片同时完成 API 类型、权限、冲突、页面状态、测试和文档，不先复制全部页面再统一补质量。

完成门：关键 CRUD、批量操作、上传/下载、ETag 冲突和权限负例通过。

### P6：生成器与 Supplier golden

- 为模块规范增加显式 `frontendTarget`，V5 默认行为保持稳定。
- V6 profile 只写 `web/antd-v6`，支持 dry-run、path confinement、stable ordering、obsolete cleanup 和 idempotency。
- Supplier 覆盖 route、locale、typed client、页面和 Playwright。

完成门：生成两次零差异、drift check、生产构建和 Supplier 浏览器验收通过。

### P7：发布资格和切流

- 在同一 merged-main SHA 上运行依赖、Go、两个前端、文档、权限、浏览器、容器和恢复门禁。
- 生成独立 V6 tag、dist、镜像 digest 和 release；不修改 V5 artifact。
- 先小范围部署 V6，观察错误、性能、401/403、WebSocket 和权限一致性，再扩大流量。

完成门：所有 Feature acceptance 有 exact-SHA 证据，资格清单移除 V6 exclusion 后才允许正式发布。

## 独立发布拓扑

| 项目 | V5 | V6 |
| --- | --- | --- |
| 目录 | `web/antd` | `web/antd-v6` |
| tag | `web/antd/v{version}` | `web/antd-v6/v{version}` |
| 镜像 | `mss-boot-admin-antd` | `mss-boot-admin-antd-v6` |
| CI | Frontend CI | Frontend v6 CI |
| Release workflow | `frontend-release.yml` | `frontend-v6-release.yml` |
| 构建身份 | V5 专属 | `FRONTEND-V6-BUILD-INFO` / `release.json` |
| 回滚 | 重部署上一 V5 digest | 重部署上一 V6 digest |

V6 发布只生成不可变静态包和 OCI 镜像，不自动替换 V5 部署。实际环境的部署/提升动作使用 V6 专属环境变量和 secret；未确定托管平台前不把供应商专属部署逻辑写进基础仓库。

## 验证与发布门禁

开发检查从最小到最大执行：

```shell
go run ./cmd/mss spec validate .mss/features/admin-antd-v6-application.yaml --format json
make web-v6-install web-v6-lint web-v6-test web-v6-build
go run ./cmd/mss verify --changed
```

发布前额外要求：

- `pnpm dedupe --check`、生产依赖 high audit、完整依赖 critical audit；
- 生产 bundle 无 React 18/antd 4 运行时，且满足 entry/chunk/total gzip budget；
- Cookie、CSRF、OAuth、WebSocket ticket、权限正反例；
- Chromium/Firefox/WebKit 桌面与移动、中文与英文、键盘/焦点、零弃用 warning；
- Nginx 缓存、SPA refresh、404、service worker 禁用和旧 chunk 失败恢复；
- merged-main、clean worktree、精确 tag、artifact checksum、image digest、SBOM 和 provenance。

## 回滚原则

V6 回滚只重部署上一个已验证的 V6 immutable image digest。不要重建历史版本，不移动 tag，不回滚 V5，也不因为 UI 回滚而逆转已经上线的兼容性后端变更。若发布后必须修复代码，创建后续 PR、合入 `main`、选择新的 commit 并重跑受影响门禁。
