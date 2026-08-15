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
- 当前阶段：P0/P1、P2 已完成；P3 的身份启动链、授权菜单求交、权限新鲜度、服务端授权 revision 实时推送、分层主题闭环，以及安全账户中心、个人资料、PAT、OAuth 连接、语言切换已实现；P4 已完成工作台监控和在线会话两个垂直切片；P5 已完成语言管理垂直切片。其余业务等价、生成器和浏览器验收尚未完成，因此不是生产发布候选。
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
| 工作台和监控摘要 | 已用 React Query + 原生语义 SVG 重写 | 权威历史、503 Retry-After、403 停止轮询、last-good/stale、移动端状态 |
| 用户、角色、菜单 | ProTable/ProForm v3 重写 | root 与委派权限、ETag/If-Match 冲突、直接 API 负例 |
| 部门、岗位 | 共用组织目录 primitives | 树/列表响应式、引用完整性、空态与冲突 |
| 任务 | CRUD 与运行记录分离 | 状态刷新、失败信息、权限动作一致 |
| 通知 | 列表与已读变更重写 | 未读数失效、WebSocket 只触发 query invalidation |
| 语言 | 已用原生 Table/Form、Listy 和受限运行时资源适配重写 | BCP 47、服务端 ID、定义上限、revision 冲突、细粒度权限、仅 zh-CN/en-US 运行时覆盖 |
| Option | 已用原生 Table/Form、Listy 和强 revision 契约重写 | 服务端 ID、完整快照、内置保护、usage 删除约束、细粒度权限 |
| AppConfig / SystemConfig | 类型化配置编辑器 | secret 不回显、root/配置权限、缓存 revision 一致 |
| 在线会话 | 已用 root-only Table + React Query 重写 | 严格响应契约、100 行上限、撤销审计与正反例 |
| 日志、审计、登录日志 | 待安全查询契约后重写 | 敏感字段脱敏、资源上限、分页筛选与导出 |
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
- 后端只在已提交的全局授权 revision 成功重载进当前 Casbin 后，以非阻塞、best-effort WebSocket 事件广播十进制 revision；V6 经单次 ticket 建连，处理应用层 heartbeat、踢出与有抖动的上限退避重连，并在同页/跨标签对 revision 去重后从受保护 HTTP 重读身份和编译期菜单。事件不携带策略或用户权限，WebSocket 拥塞也不影响权限事务。
- Max 的过渡 Moment 消费通过官方 `moment2dayjs` 适配统一到 Day.js；release bundle 门禁会拒绝 Moment 进入生产图。当前授权 revision 实时推送切片后的生产构建 entry 为 4.16 KiB、总 JS 为 734.67 KiB、最大异步分包为 195.57 KiB，均通过预算门禁。

本阶段主动冻结三个旧契约缺陷，未按“表面等价”复制：

1. 邮箱是登录与恢复身份，在专用双重验证变更流程完成前保持只读。
2. 旧的已登录密码重置不要求当前密码或近期再认证；V6 不展示该操作，待增加 step-up/re-auth、限流及成功后全会话/PAT 撤销契约。
3. 旧 OAuth 解绑可直接删除最后一种可用登录方式；V6 只允许安全连接，待后端事务性保证至少保留一种已验证登录方式并要求近期再认证后再开放解绑。

仍待本阶段完成：上述密码/邮箱/解绑安全契约的产品确认与后端实现，以及真实浏览器权限矩阵。focus/visibility、网络恢复与 403 权威重读继续作为 WebSocket 不可用时的安全兜底。

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
- 此页面只需要受控筛选、分页和行操作，因此采用 antd 6 原生 `Table`。资格构建证明引入 `ProTable` 会把总 JS 推高到 952.10 KiB 并突破 900 KiB 预算；改用原生组件后 entry 为 4.16 KiB、总 JS 为 848.54 KiB、最大异步分包为 197.45 KiB，同时保留所需交互。

仍待本阶段完成：日志、通知和运行记录，以及工作台监控与在线会话的真实浏览器桌面/移动、中文/英文和授权正反例证据。

以下旧契约经评估后冻结，不在 P4 中直接复制，需先完成后端产品与安全契约：

1. 通知通用列表的租户范围不能表达“个人收件箱”，旧的全部已读语义也不完整；目标应拆分个人收件箱与管理员广播。
2. 文件日志读取存在无界目录扫描/文件读取且缺少统一脱敏；审计筛选也有未落实字段，必须先完成分页、上限、保留和 redaction。
3. TaskRun 尚无稳定前端/API，原始任意输出没有大小、脱敏、保留期和独立权限边界；任务执行器的并发错误变量也须先修复。

完成门：桌面/移动、zh-CN/en-US、错误降级和资源上限 E2E 通过。

### P5：核心 CRUD

- 按用户与组织、角色与菜单、任务、语言与 Option、AppConfig/SystemConfig 分批迁移。
- 每个切片同时完成 API 类型、权限、冲突、页面状态、测试和文档，不先复制全部页面再统一补质量。

已完成语言与 Option 管理垂直切片。

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

P5 中尚未实施的关键后端前置问题包括：部门/岗位需要先明确树循环、删除引用完整性与数据范围语义。完成这些契约前不会机械复制旧页面。

完成门：关键 CRUD、批量操作、上传/下载、ETag 冲突和权限负例通过。

### P6：生成器与 Supplier golden

- 为模块规范增加显式 `frontendTargets` 声明，并通过 `--frontend-target` 单次选择一个投影；未声明的新旧规格继续默认 V5，兼容行为保持稳定。
- V6 profile 只写 `web/antd-v6`，支持 dry-run、path confinement、stable ordering、obsolete cleanup 和 idempotency。
- Supplier 的同一受版本控制规格同时声明 V5/V6，V6 覆盖编译期 route registry、双语 catalog、运行时契约校验、typed transport、React Query、响应式页面和 HttpOnly/CSRF Playwright 流程。
- 首版 V6 profile 的资格边界是带时间戳和 uuid/string ID 的完整 CRUD+export，以及非 nullable 的 string/text/uuid/enum/bool 字段。必填 create 字段必须具有可见编辑控件；启用 E2E 时还必须提供可重复清理和更新验证的唯一、可搜索、列表/表单可见文本字段。数值、文件、关系、create-only immutable 编辑、batch、import、workflow 等尚未实现的语义在规格校验阶段 fail-closed，不能用普通输入框近似。
- 生成目录由生成器而非 Biome formatter/organize-imports 拥有；Biome lint、严格 TypeScript、Vitest、双目标 drift 和生产构建仍覆盖这些产物，任意手工格式化导致的漂移都会失败。
- Supplier 加入后的 release build entry 为 4.16 KiB、总 JS 为 881.98 KiB、最大异步分包为 199.59 KiB，继续通过 900/250 KiB 门禁，但总量仅余 18.02 KiB；下一业务切片必须先给出依赖复用和预算证据。

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
