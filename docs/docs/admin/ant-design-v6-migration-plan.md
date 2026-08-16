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
- 当前阶段：P0/P1、P2 已完成；P3 的身份启动链、授权菜单求交、权限新鲜度、服务端授权 revision 实时推送、分层主题闭环，以及安全账户中心、个人资料、PAT、近期本人认证、个人改密、OAuth 连接与安全解绑、语言切换已实现；P4 已完成工作台监控、在线会话、任务、个人通知、审计/登录/运行时日志和系统配置；P5 已完成用户、角色、菜单、部门、岗位、语言、Option 与五组应用设置管理；P6 的双目标生成器和 Supplier golden 已完成。真实 Go 后端上的 Chromium 桌面/移动、中文/英文、匿名与 Finance 最小权限矩阵，以及独立 Nginx 容器交付 smoke 已通过。主邮箱变更与 TaskRun 原始输出仍保持冻结，正式发布资格仍以全部必需门禁为准。
- 机器契约：`.mss/features/admin-antd-v6-application.yaml`。
- 架构决策：`docs/adr/2026-08-15-independent-ant-design-v6-application.md`、`docs/adr/2026-08-16-account-reauthentication-and-credential-self-service.md`。

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
| 工作台和监控摘要 | 已用 React Query + 原生语义 SVG 重写 | 权威历史、503 Retry-After、403 停止轮询、last-good/stale、移动端状态 |
| 用户、角色、菜单 | 已用受限 ProTable/ProForm v3 与类型化 authority 模块重写 | root mutation guard、服务端标识、revision 冲突、直接 API 负例 |
| 部门、岗位 | 已用共用组织目录 primitives 重写 | 循环拒绝、引用完整性、响应式树/列表、空态与冲突 |
| 任务 | 已将受控任务 CRUD/运行操作与未开放的 TaskRun 原始输出分离 | provider allowlist、状态刷新、资源上限、权限动作一致 |
| 通知 | 已按当前身份个人收件箱语义重写列表与已读变更 | 服务端 owner、未读数失效、跨身份负例 |
| 语言 | 已用原生 Table/Form、Listy 和受限运行时资源适配重写 | BCP 47、服务端 ID、定义上限、revision 冲突、细粒度权限、仅 zh-CN/en-US 运行时覆盖 |
| Option | 已用原生 Table/Form、Listy 和强 revision 契约重写 | 服务端 ID、完整快照、内置保护、usage 删除约束、细粒度权限 |
| AppConfig / SystemConfig | 已分别用分层主题编辑器和按需加载的有界配置编辑器重写 | secret 不回显、root/配置权限、内置保护、缓存 revision 一致 |
| 在线会话 | 已用 root-only Table + React Query 重写 | 严格响应契约、100 行上限、撤销审计与正反例 |
| 日志、审计、登录日志 | 已按独立摘要和有界运行时文件读取重写 | 敏感字段脱敏、目录/文件/读取上限、分页筛选与受权导出 |
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

本地人工验收使用显式的 `v6-local` 配置层，避免把兼容期默认关闭的浏览器会话误当成运行故障，也避免为 V6 修改 V5 的默认启动语义：

```bash
docker run -d --name mss-boot-admin-local-redis --restart unless-stopped \
  -p 127.0.0.1:6379:6379 redis:7.4-alpine \
  redis-server --requirepass 123456

cd admin
STAGE=v6-local MSS_RUNTIME_REDIS_PASSWORD=123456 go run . server

cd ../web/antd-v6
corepack pnpm@10.34.5 start:dev
```

`application-v6-local.yml` 仅允许 HTTP localhost 和非 Secure 开发 Cookie；它不能作为生产配置。生产仍必须显式提供 HTTPS origin、强 auth key、共享 Redis、Secure Cookie 及独立 OAuth 应用凭据。

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
- `/app-config` 还以同一响应式 Tabs 容器补齐基础、安全、存储准入、邮件和主题五组设置；V6/V5 OAuth 凭据明确分区，密钥输入采用只写轮换语义，空值不会覆盖现有密钥，Logo 保存后即时刷新启动配置与布局。
- 主题保存和重置立即更新唯一 Query cache、Umi 布局和 antd 6 ConfigProvider；跨标签事件有 schema、TTL、去重、单调 revision 与个人随机会话绑定，同 revision 分歧只触发权威重读。
- 首屏快照只包含规范化七字段、revision 和过期时间，最长 24 小时；应用层可匿名使用，个人层必须同时匹配随机会话和已验证用户主体，Web Locks 不可用时拒绝持久化而不是冒险覆盖新修订。
- 密码登录和 OAuth 在取得规范化 current user 后轮换主题会话；退出先清除本标签个人派生状态，服务端退出完成后再通知其他标签重启，避免 cookie 撤销竞态。
- 账户中心与响应式设置页已复用同一个规范化 current-user query；个人资料提交只含后端明确允许的字段，用户名和邮箱只读，服务端以精确 allowlist 拒绝身份字段、未知字段和错误类型，并可正确持久化显式空值。
- PAT 列表只解析元数据；创建与旋转返回的原始 secret 不进入 React Query mutation cache、持久存储或 URL，只存在于不可跳过的一次性内存弹窗中。撤销、旋转和创建均继续由后端 self 权限约束，PAT 本身不能调用这些交互端点。
- OAuth 登录与连接入口只在 public application profile 明确启用 provider 时显示；authorize/callback 使用服务端单次 state 和 cookie 会话，回调同时校验业务成功码、provider 和 intent，provider token 不进入浏览器响应。解绑必须先取得绑定到当前服务端会话的近期本人认证，并由数据库事务保证至少保留本地密码或另一个 OAuth 登录方式。
- 个人改密不进入 React Query mutation cache；服务端只保存加盐后的单向 scrypt 校验值，无法读取或解密原密码。当前密码或同账号既有 OAuth 身份可建立五分钟近期认证，密码失败有持久化限流；改密与所有浏览器会话、PAT 撤销在同一事务提交，发起浏览器也会退出。
- Umi 官方 `SelectLang` 已进入登录页和全局布局，中文/英文 catalog 由测试保证 key 完全一致。
- 权限新鲜度桥在显式变更事件、跨标签 BroadcastChannel、403、网络恢复，以及节流后的 focus/visibility 上权威重读 current user 与编译期菜单求交；权限或可执行菜单签名变化时先取消并移除业务 Query 和 mutation cache，只保留身份、菜单、public profile 与主题启动 Query。身份/菜单不能确认时立即切换到可重试的 fail-closed 页面；异常主体切换会清空个人主题和所有缓存。
- 后端只在已提交的全局授权 revision 成功重载进当前 Casbin 后，以非阻塞、best-effort WebSocket 事件广播十进制 revision；V6 经单次 ticket 建连，处理应用层 heartbeat、踢出与有抖动的上限退避重连，并在同页/跨标签对 revision 去重后从受保护 HTTP 重读身份和编译期菜单。事件不携带策略或用户权限，WebSocket 拥塞也不影响权限事务。
- Max 的过渡 Moment 消费通过官方 `moment2dayjs` 适配统一到 Day.js；release bundle 门禁会拒绝 Moment 进入生产图。当前授权 revision 实时推送切片后的生产构建 entry 为 4.16 KiB、总 JS 为 734.67 KiB、最大异步分包为 195.57 KiB，均通过预算门禁。

本阶段没有按“表面等价”复制三个旧契约缺陷，处理结果如下：

1. 邮箱是登录与恢复身份，在专用双重验证变更流程完成前保持只读。
2. 旧的已登录密码重置不要求当前密码或近期再认证；V6 已改为服务端 session-bound step-up、失败限流、密码策略、不可逆校验值轮换及成功后全会话/PAT 原子撤销。
3. 旧 OAuth 解绑可直接删除最后一种可用登录方式；V6 已改为同一近期认证门禁和行锁事务，无法删除最后一种已验证登录方式，并保留受同样约束的 V5 兼容入口。

主邮箱身份变更仍等待独立的双重验证产品契约；当前个人设置会展示邮箱和手机号状态，但不会提供不安全的直接替换入口。真实浏览器现已覆盖匿名直达、root 会话、Finance 非 root 菜单求交、无创建按钮、直接写 API 403、root-only API 403 和直接路由 403。focus/visibility、网络恢复与 403 权威重读继续作为 WebSocket 不可用时的安全兜底。

完成门：匿名/root/普通用户/撤权/未知菜单/直接路由和直接 API 矩阵通过。

### P4：只读和运维能力

- 迁移工作台、日志、监控、在线会话、通知和运行记录。
- 建立 Query key、轮询、失效、取消和边界状态规范。

已完成的垂直切片：

- `/workplace` 不再展示工程占位信息，而是提供当前身份、经过权限检查的已迁移快捷入口和服务监控。
- `/monitor` 使用独立 Query key 和严格响应解析，最多接受 120 个按时间排序、去重且范围有效的服务端历史点；浏览器不推算或补造样本。
- 正常刷新跟随服务端 `sampleIntervalMs`（最低 5 秒、最高 60 秒）；503 遵守有界 `Retry-After`，瞬时错误指数退避，401/403 停止自动轮询。后台标签不轮询，窗口重新聚焦后由 React Query 校验。
- 首次加载、空历史、503 预热、403、普通错误、刷新失败保留 last-good 以及后端 `stale` 均有独立中英文状态；手动重试始终可用。
- CPU、内存和磁盘摘要使用 antd 6 公开组件 API；CPU/内存趋势使用 Design Token 驱动的可访问原生 SVG，不增加图表运行时依赖，也不依赖 `.ant-*` 内部结构。
- 单元/组件测试覆盖传输路径、契约拒绝、采样排序和边界、轮询策略、403/503、last-good/stale 与空历史。
- `/security/online-sessions` 同时在编译期路由、菜单求交和后端 API 失败关闭为 root-only；未确认身份和普通用户不能渲染页面，直接 API 仍由后端最终授权。
- 列表和详情在传输边界严格校验标识、日期、撤销原因和分页结构；前后端统一只允许 20/50/100 行，服务端拒绝更大的页面。30 秒前台刷新在 401/403 后停止，瞬时刷新失败保留 last-good 列表。
- 单会话和按用户批量下线只能从权威列表行发起，继续使用后端目标保护与安全审计；旧界面允许手填任意 user ID 的工具栏没有复制。筛选、详情、空态、403、初次错误、刷新警告和破坏性确认均有中英文状态。
- 此页面只需要受控筛选、分页和行操作，因此采用 antd 6 原生 `Table`。资格构建证明引入 `ProTable` 会把总 JS 推高到 952.10 KiB；改用原生组件后 entry 为 4.16 KiB、总 JS 为 848.54 KiB、最大异步分包为 197.45 KiB，同时保留所需交互。
- `/task` 只向拥有读取权限的用户返回脱敏摘要；详情、启停、立即执行、删除和函数目录均要求 root。任务只允许受控进程函数、已注册 Go 函数、HTTP 和 Kubernetes CronJob provider，不迁移任意脚本执行；删除只允许已停用任务，Kubernetes 启停直接映射 `spec.suspend`。
- `/notice` 是按当前身份隔离的个人收件箱，记录 ID、owner 和已读状态由服务端拥有；批量已读只能作用于当前用户可见记录。列表、读取和空态使用独立 Query key，不把通知内容写入启动状态。
- `/log` 将审计摘要、任务运行摘要和运行时文件日志分开。浏览器不接收原始请求/响应体；运行时读取和导出受 root 与独立权限双重约束，并执行目录、符号链接、文件大小、读取字节数和脱敏上限。
- `/system-config` 全路径 root-only。列表只返回摘要，内容按需读取并禁止缓存；写入只接受有界 JSON/YAML，内置配置的关键字段和删除受保护。
- 四个运维页面复用一个懒加载入口。将它们机械拆成四个入口会使 Utoo 生成的总 gzip JS 从 936.47 KiB 增至 1018.24 KiB，因此资格实现保留共享入口，并以编译期 route、页面权限守卫和后端授权维持隔离。

随着权限、组织和完整运维切片进入同一独立应用，总懒加载 JS 语料库预算从脚手架阶段的 900 KiB 校准为 960 KiB；entry 32 KiB 与最大异步分包 240 KiB 的用户路径门限保持不变。资格基线为 entry 4.16 KiB、总 JS 936.67 KiB、最大异步分包 199.60 KiB。预算调整记录在 ADR 和机器契约中，不能仅以环境变量放宽。

工作台监控、在线会话和四个运维页面均进入桌面/移动、中文/英文、权限负例、资源上限与零弃用 warning 验收；在线会话和系统配置的 root-only 直接路由/API 负例也纳入同一权限套件。

旧契约问题没有按表面行为复制，处理结果如下：

1. 通知页明确收敛为按已验证身份隔离的个人收件箱；管理员广播另立产品能力，不借用请求中的 `userID` 扩大范围。
2. 文件日志改为 allowlist 目录内的有界扫描、普通文件检查、最大文件/读取字节数和统一 redaction；审计与登录日志只返回脱敏摘要。
3. TaskRun 原始输出仍未开放。当前页面只迁移受控任务定义和允许的运行操作；在输出大小、脱敏、保留期和独立权限契约完成前，不以日志详情近似该能力。

完成门：桌面/移动、zh-CN/en-US、错误降级和资源上限 E2E 通过。

### P5：核心 CRUD

- 按用户与组织、角色与菜单、任务、语言与 Option、AppConfig/SystemConfig 分批迁移。
- 每个切片同时完成 API 类型、权限、冲突、页面状态、测试和文档，不先复制全部页面再统一补质量。

已完成用户、角色、菜单、部门、岗位、语言与 Option 管理垂直切片。

语言管理：

- 后端公开接口只返回启用语言和必要投影，并对语言总数、单语言定义条数/体积、公开聚合响应（1 MiB）以及管理分页设置硬上限；管理列表默认不携带大体积 `defines`，详情按需加载。
- 创建/更新只绑定显式 DTO，记录 ID、时间戳、租户字段和空白定义 ID 均由服务端控制；旧 `BeforeCreate` 遮蔽嵌入模型钩子造成的空 ID/状态由前向兼容迁移修复。
- 语言名按 BCP 47 解析并规范化，不再使用只接受 `xx-XX` 的狭窄正则；重复 definition ID 或 `group + key` 会失败关闭，避免运行时覆盖顺序产生歧义。
- 更新携带精确 `expectedUpdatedAt`；冲突返回稳定错误码，编辑器保留当前草稿并让用户显式选择是否载入最新服务端版本。
- 新的 read/create/update/delete 菜单与 API 节点可独立分配；迁移只从迁移前已启用的精确父节点继承对应权限，读取不会隐式获得写入。
- 列表使用 antd 6 原生 `Table`，编辑使用 `Form.List`；详情在定义较多时使用 antd 6.6 `Listy` 虚拟化。所有页面共享响应式实现并覆盖 loading、empty、error、403、404 和 conflict。
- 动态 profile 是启动增强而非启动依赖：2.5 秒内失败或超时不会阻断应用。它只能覆盖仓库随包发布且 catalog key 对齐的 `zh-CN`、`en-US`；新增其他 BCP 47 数据不会静默变成不完整的 UI 语言，扩展运行时 locale 需单独产品决策与完整 catalog。
- 语言切片最初的 release build 为 891.99 KiB，总预算只余 8.01 KiB。随后将 Umi 默认 Chrome 80 全量 polyfill 收敛为契约化的 Chromium/Edge 120+、Firefox 121+、Safari/WebKit 17.4+ 现代浏览器基线，并将业务图标改为包公开子路径导入；release gate 现在同时拒绝 `core-js` 和图标 barrel 回归。
- 优化后的语言切片 entry 为 4.16 KiB、总 JS 为 847.32 KiB、最大异步分包为 199.59 KiB，在不放宽 900/250 KiB 门禁的前提下恢复 52.68 KiB 余量。

Option 管理：

- 移除会绕过历史快照、缓存失效、内置资源保护和并发控制的通用 CRUD 注册，改为显式、allowlist、受限的管理 API；V5 在并行期缺少 revision 的写入暂时兼容并返回迁移警告，V6 的更新和删除始终发送同一资源的强 `If-Match`。
- 服务端拥有记录 ID、version、built-in 标记和新增 item ID；更新携带未知 item ID 时按新增项处理，不能夺取其他字典项身份。每次更新和删除在同一事务内保存完整 prior-resource snapshot，CAS 失败返回最新权威资源，浏览器保留草稿并要求用户显式重载。
- 字典、单项、嵌套 extra、排序、管理分页和完整列表响应都有硬上限；读取缓存或数据库中的畸形数据时失败关闭。Redis key 对 tenant/category/name 做命名空间和编码，cache miss 固定读 writer，提交后的缓存失效是有界 best-effort，不会诱发客户端重试已成功事务。
- 内置字典允许调整显示信息与字典项，但 category、name、status 和删除保持受保护；自定义字典只有在没有启用的 `OptionUsage` 时可删。`icon`、`color` 与 opaque `extra` 在 V6 仅作为非执行文本显示，未开放动态组件或 HTML 注入路径。
- read/create/update/delete 菜单和 API 权限独立；列表使用 summary 投影，详情按需加载。V6 使用原生 `Table`、`Form.List` 和 antd 6.6 `Listy`，共享桌面/移动响应式页面并覆盖 loading、empty、error、403、404、refresh failure 和 412 conflict。
- Option 切片后的 release build entry 为 4.16 KiB、总 JS 为 864.91 KiB、最大异步分包为 199.59 KiB，仍通过 900/250 KiB 门禁，余量为 35.09 KiB。新增业务仍须复用现有运行时并逐切片测量，不能把路由分包当成忽略总体积增长的理由。

用户、角色、菜单、部门和岗位切片已在 root-only mutation guard、服务端标识解析、部门循环拒绝、引用完整性和 role authorization revision 契约下完成。旧 `User` 的 `AfterCreate`/`AfterDelete` 统计写入也已移到事务提交后，并被定义为不能反转已提交业务结果的可选遥测。菜单树会在服务端保留完整 `menu.*` 标识并在 V6 展示层兼容清理历史重复前缀，中英文名称与图标均来自显式 catalog/allowlist。密码与 OAuth 身份变更已具备近期认证和跨资源撤销契约；仍冻结的是主邮箱变更和尚无安全输出契约的 TaskRun 详情。

完成门：关键 CRUD、批量操作、上传/下载、ETag 冲突和权限负例通过。

### P6：生成器与 Supplier golden

- 为模块规范增加显式 `frontendTargets` 声明，并通过 `--frontend-target` 单次选择一个投影；未声明的新旧规格继续默认 V5，兼容行为保持稳定。
- V6 profile 只写 `web/antd-v6`，支持 dry-run、path confinement、stable ordering、obsolete cleanup 和 idempotency。
- Supplier 的同一受版本控制规格同时声明 V5/V6，V6 覆盖编译期 route registry、双语 catalog、运行时契约校验、typed transport、React Query、响应式页面和 HttpOnly/CSRF Playwright 流程。
- 首版 V6 profile 的资格边界是带时间戳和 uuid/string ID 的完整 CRUD+export，以及非 nullable 的 string/text/uuid/enum/bool 字段。必填 create 字段必须具有可见编辑控件；启用 E2E 时还必须提供可重复清理和更新验证的唯一、可搜索、列表/表单可见文本字段。数值、文件、关系、create-only immutable 编辑、batch、import、workflow 等尚未实现的语义在规格校验阶段 fail-closed，不能用普通输入框近似。
- 生成目录由生成器而非 Biome formatter/organize-imports 拥有；Biome lint、严格 TypeScript、Vitest、双目标 drift 和生产构建仍覆盖这些产物，任意手工格式化导致的漂移都会失败。
- 生成的筛选/编辑表单使用独立 DOM 命名空间，有限枚举关闭虚拟化以保留真实 option 语义，操作按钮具有稳定的本地化可访问名称；E2E 唯一字段在长度和正则契约内生成 worker 隔离值且不受软删除唯一索引污染。Supplier 已在 Playwright 自启真实 Go 后端和隔离 SQLite 的模式下，串行通过 Chromium 桌面和移动完整 CRUD、详情、导出及删除流程；串行执行用于规避 SQLite 单写者语义干扰跨项目资格结果。
- 生产路由在每个 Supplier 请求范围内获取当前数据库租约，认证、授权与生成 Application 共享同一 request-pinned handle；配置热更新会等待旧请求退出，再关闭旧连接池，新请求自动使用替换后的数据库。该生命周期属于服务端组合层，不手改生成代码，并有跨数据库替换回归测试。
- Supplier 切片进入时的 release build 基线为 entry 4.16 KiB、总 JS 882.96 KiB、最大异步分包 199.59 KiB；其后权限、组织和运维切片的当前资格基线与 960/240 KiB 预算记录见 P4 与 ADR。该结果已包含后端菜单图标的显式 allowlist 适配器，后续业务切片仍必须给出依赖复用和预算证据。
- 桌面与移动端的中英文浏览器资格会访问工作台、账户中心、个人设置和 Supplier，验证无横向溢出、语言按钮可访问、动态菜单文案/图标正确，并将所有 console warning/error 作为失败；权限资格以隔离 Finance 身份同时证明后端拒绝与 UI 最小权限。

完成门：生成两次零差异、V5/V6 drift check、生产构建和 Supplier 桌面/移动浏览器验收通过。

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
go run ./cmd/mss module generate .mss/modules/example-supplier.yaml --frontend-target antd-v5 --check
go run ./cmd/mss module generate .mss/modules/example-supplier.yaml --frontend-target antd-v6 --check
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
