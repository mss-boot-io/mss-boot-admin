---
title: 主题设置优先级、继承与重置
order: 14
nav:
  order: 1
  title: admin
description: 说明应用主题与个人主题的目标优先级、作用域、更新语义、权限、兼容方案和验收标准
keywords: [admin theme settings precedence inheritance application user]
---

## 文档状态

- 评估基线：`main@be22416`，2026-08-07
- 当前状态：P1/P2 核心和 P3 revision/同步实现已进入当前分支；外部 MySQL/PostgreSQL 与完整浏览器 E2E 尚未验收，能力仍为 `planned`
- 机器契约：`.mss/features/admin-theme-settings-precedence.yaml`
- 架构决策：`docs/adr/2026-08-07-layered-theme-settings-precedence.md`

:::warning
本页同时描述已确定的目标契约和当前分支的实现边界。代码已经覆盖 revision、条件写、版本化缓存、跨标签同步、脏表单冲突和 24 小时身份绑定快照；但“已有实现”不等于“已通过发布门禁”，不能提前把该能力标记为完整。
:::

## 当前实现完整度评估

基线问题已经不再代表当前分支。当前代码已形成可评审的分层主题候选实现，但发布证据还未闭环，应按下面的边界管理：

| 维度 | 当前分支实现 | 尚缺证据 | 结论 |
| --- | --- | --- | --- |
| 作用域与优先级 | 同一组件通过显式 scope 使用 app/user 适配器；逐字段按代码、应用、个人解析，`false` 有效 | 完整生产浏览器矩阵 | P1/P2 核心已实现 |
| 继承与重置 | 稀疏 patch、`null` 单项继承、DELETE 整组继承和规范化返回已接入 | 三数据库 reset→set 升级矩阵 | 实现完成，发布证据待补 |
| 权限与身份 | 应用读写 RBAC、个人 authenticated-self、登录/登出身份清理已接入 | 真实角色和双用户 E2E | 安全实现已落地，门禁待验收 |
| 并发与 revision | 复合主键 `ConfigRevision`、强 ETag、`If-Match`、412 canonical 冲突响应已接入 | MySQL/PostgreSQL 并发与升级验证 | P3 后端已实现，外部矩阵待补 |
| 公共缓存 | profile 采用数据库 revision 版本键和 TTL；Redis 故障不改变已提交 HTTP 结果 | 生产 Redis 故障演练 | 一致性模型已实现，运维证据待补 |
| 多标签与首屏 | BroadcastChannel/storage、乱序拒绝、聚焦复核、脏表单冲突、24 小时快照已接入 | 多页 E2E 和首屏 trace | P3 前端已实现，体验证据待补 |
| 审计 | 主题变更记录 scope、字段、outcome、revision，且不复制主题值 | 完整成功/失败审计矩阵 | 结构已实现，专项验收待补 |
| 多端质量 | 已有桌面/移动、明暗主题的人工检查基础 | prod 构建、键盘焦点、自动对比度和完整 E2E | 仍是发布门禁 |

产品结论：当前分支已经从“分散入口”进入“候选能力”阶段。发布门槛仍不是“表单可以保存”，而是外部数据库、真实并发、双用户、多标签、生产首屏和可访问性证据全部闭环，因此能力目录继续保持 `planned`。

## 一条核心规则

每个主题字段都独立按以下顺序取值：

```text
最终主题 = 不可变代码默认值
         + 稀疏应用覆盖
         + 稀疏个人覆盖
```

这里的“+”是字段覆盖，不是整体替换。个人只覆盖主色时，导航主题仍继续继承应用值；应用没有配置的字段继续继承代码默认值。

| 优先级 | 层级 | 所有者 | 用途 |
| --- | --- | --- | --- |
| 3（最高） | 个人覆盖 | 当前登录用户 | 只影响本人 |
| 2 | 应用覆盖 | 有应用配置权限的管理员 | 影响未设置个人覆盖的用户 |
| 1（最低） | 代码默认 | 前端发布物 | 保证每个字段始终有合法值 |

解析依据是“字段是否存在且合法”，不是 JavaScript 真值。`false` 是有效覆盖值，不等于“未配置”。

## 支持的字段

| 字段 | 代码默认 | 合法值 | 说明 |
| --- | --- | --- | --- |
| `navTheme` | `realDark` | `light` / `realDark` | 导航及整体明暗模式 |
| `colorPrimary` | `#1890ff` | 六位十六进制颜色 | 统一规范化后保存 |
| `layout` | `mix` | `side` / `top` / `mix` | 主布局 |
| `contentWidth` | `Fluid` | `Fluid` / `Fixed` | 内容宽度 |
| `fixedHeader` | `false` | boolean | 固定页头 |
| `fixSiderbar` | `true` | boolean | 保留现有兼容拼写 |
| `colorWeak` | `false` | boolean | 色弱辅助模式 |

`pwa` 属于构建和 Service Worker 生命周期，不属于个人或应用运行时主题。新版主题编辑器和规范资源均不能写入它。仅在滚动兼容期内，未协商规范媒体类型的旧版应用请求暂时保留原有 `pwa` 读写行为；个人作用域始终拒绝 `pwa`。`splitMenus` 当前未启用，也不进入本次主题契约。

## 同一组件、两个作用域

应用设置和个人设置继续复用同一个主题编辑器，但必须显式传入作用域：

| 页面 | scope | 读取 | 保存与逐项重置 | 全部重置 |
| --- | --- | --- | --- | --- |
| 超级权限 → 应用设置 → 主题设置 | `application` | `GET /app-configs/theme` | `PUT /app-configs/theme` | `DELETE /app-configs/theme` |
| 个人设置 → 主题设置 | `user` | `GET /user-configs/theme` | `PUT /user-configs/theme` | `DELETE /user-configs/theme` |

以上路径均相对于 `/admin/api`。

组件可以复用表单 schema、字段、预览和响应式布局，但不能复用错误的数据源。应用 scope 只允许调用 app-config；user scope 只允许调用 user-config。

表单需要同时展示：

- 当前生效值；
- 值的来源：个人、应用或代码；
- 当前 scope 是否存在显式覆盖；
- “恢复继承”与“重置当前 scope”操作；
- loading、error、permission-denied、dirty、saving、success 状态。

显示继承值不等于保存继承值。如果用户没有主动覆盖某字段，保存其他字段时不能把该继承值写入个人配置。

## 更新语义

### 设置或修改覆盖

PUT 是稀疏 patch。下面的应用请求只修改两个字段：

```json
{
  "data": {
    "navTheme": "light",
    "fixedHeader": false
  }
}
```

`false` 会明确保存为 false。未出现在 `data` 中的字段保持不变。

### 版本化资源与条件写入

新版客户端在 GET、PUT 和 DELETE 请求中发送：

```http
Accept: application/vnd.mss.theme.v1+json
```

该 `Accept` 值属于 CORS 安全请求头，旧后端会忽略它，因此不会为“新前端 + 旧后端”的跨域滚动部署增加新的预检要求。`q=0` 表示明确拒绝该规范表示，不会进入规范分支；协商成功及规范 `412` 响应都会返回相同的 vendor `Content-Type`。规范返回保持七个覆盖值扁平，并携带 `_meta`：

```http
Content-Type: application/vnd.mss.theme.v1+json
ETag: "theme-application-12"
Cache-Control: no-store
Vary: Accept
```

```json
{
  "navTheme": "realDark",
  "fixedHeader": false,
  "_meta": {
    "v": 1,
    "scope": "application",
    "revision": "12"
  }
}
```

revision 使用十进制字符串，避免浏览器数字精度问题。新客户端保存和重置时发送读取到的强 ETag：

```http
If-Match: "theme-application-12"
```

- revision 过期：返回 HTTP 412、当前 ETag 和 `data.current` 中的规范资源，数据库不写入；
- ETag 为弱标签、通配符、多值、格式错误或 scope 不匹配：拒绝请求；
- 缺少 `If-Match`：滚动升级兼容期内暂时接受旧客户端，后续版本再收紧；
- 成功：返回服务端事务内重新读取的规范资源，客户端不得自行推测新状态。

未发送上述规范媒体类型时，后端在兼容期内返回不带 `_meta` 的旧格式。应用旧格式暂时保留 `pwa`，防止已经部署的旧前端把既有 Service Worker 选择静默覆盖；个人旧格式不接受 `pwa`。公共 `/app-configs/profile` 也暂时保留这个公开的应用兼容字段，但新版七字段解析器会忽略它。旧格式投影与缺失 `If-Match` 的宽限必须在旧前端静态资源排空后，通过独立评审版本一起退场。

### 恢复单项继承

`null` 删除当前 scope 的该字段覆盖：

```json
{
  "data": {
    "navTheme": null
  }
}
```

- 个人 scope 删除后，字段立即继承应用有效值；
- 应用 scope 删除后，字段立即继承代码默认值。

空字符串不是删除语义。空字符串、非法颜色、未知字段和不支持的枚举会使整个请求失败，且数据库不发生部分更新。

### 重置全部

- `DELETE /app-configs/theme`：清空应用主题覆盖，应用有效主题回到代码默认；
- `DELETE /user-configs/theme`：清空当前用户主题覆盖，个人有效主题回到应用有效主题。

重置必须二次确认，并在完成后返回规范空资源及新 revision，再显示每个字段新的来源。

## 权限和审计

| 操作 | 后端要求 |
| --- | --- |
| 读取应用覆盖 | `config:read` |
| 保存或重置应用覆盖 | `config:write` |
| 读取、保存或重置个人覆盖 | 已认证且只能操作当前用户 |
| 公共启动读取应用主题 | `/app-configs/profile` 主题白名单 |

个人接口从服务端已验证身份推导 user ID，不接受客户端选择其他用户。隐藏按钮不能替代后端鉴权。

应用和个人保存、逐项重置、全部重置都留下结构化审计证据，包括操作者、scope、有序变更字段、结果和可用时的规范 revision。主题审计元数据不记录字段值，并替代原始主题请求体；通用敏感字段继续脱敏。审计跳过规则按路径段匹配，不能误跳过 `/user-configs/*`。

## 原子性和缓存

一次 PUT 中的校验、设置和删除构成一个逻辑操作：

1. 先验证全部字段；
2. 再在一个数据库事务中执行有序 set/unset；
3. 失败时全部不变；
4. 成功时返回规范化后的稀疏 scope 和单调递增 revision；
5. 应用 scope 在同一事务推进主题 revision 与 public-profile revision；
6. 当前页按返回结果立即重算，不要求刷新。

revision 存放在新增的 `mss_boot_config_revisions` 表，复合主键为 `(scope, owner_id, resource)`。应用资源的 `owner_id` 为空，个人主题使用服务端已认证 user ID。revision 不放进七个主题值行，也不依赖 Redis。

公共 profile 缓存使用数据库 public-profile revision 组成缓存键，缓存 envelope 同时校验 profile revision 与 theme revision，当前 TTL 为 15 分钟。旧 revision 的缓存永远不会被新读请求接受。Redis 读取、写入或清理失败只记录告警；如果数据库事务已经提交，API 仍返回成功，不能把已提交结果伪装成失败。

同源标签页通过 schema-versioned `BroadcastChannel` 同步，并以 `storage` 事件作为回退。事件有时效、去重、scope 和 revision；旧 revision 被忽略，相同 revision 却不同内容时重新读取服务端。页面重新可见时进行节流的权威复核。

编辑器存在未保存改动时，新规范资源不会静默覆盖草稿，也不会自动重试；界面进入冲突状态，用户可加载最新版本，或保留已触碰字段并在新 revision 上明确复核后重试。

浏览器可以缓存不敏感的最后主题快照来减少暖启动闪烁，但缓存不是第四层。快照只包含七个主题值和资源元数据，最长 24 小时；个人快照和个人同步事件绑定随机认证会话标识，而不是客户端 user ID。登录、登出和账号切换时清理或重新绑定个人快照，并最终以服务端 revision 为准。

快照写入按实际 storage key 使用 Web Locks，将读取、比较和写入放在同一把跨标签独占锁内。浏览器不支持 Web Locks 或锁申请失败时，客户端会跳过持久化，避免旧标签覆盖新 revision；权威主题仍正常生效，只是不再享受暖启动快照优化。

## 登录、登出与错误回退

- 未登录与登录页：代码默认 + 公共应用主题；
- 登录成功：加载当前用户稀疏覆盖并立即重算；
- 登出：先清个人层及其缓存，再回落至应用主题；
- A 用户切换到 B 用户：不能显示或保存 A 的个人覆盖；
- 应用读取失败：临时使用代码默认，并显示降级提示；
- 个人读取失败：临时使用应用 + 代码，不把回退值写成个人覆盖；
- 保存失败：保留原有已提交主题，回滚临时预览并支持重试。

## 兼容与升级

当前实现继续使用现有 `mss_boot_app_configs` 和 `mss_boot_user_configs`，只新增元数据表 `mss_boot_config_revisions`。新增迁移是前向、附加、幂等的，不替换或改写原配置表：

- 不替换表，不批量删除未知 theme 行；
- 旧的字符串 `"true"` / `"false"` 在读取时规范化为布尔值；
- 合法枚举和颜色继续有效；
- 非法历史值保留但不参与运行时解析，并产生可诊断信息；
- 与 `theme` 或公共保留键在数据库大小写、重音、Unicode 规范化或尾空格排序规则下碰撞的别名会被拒绝或按主键规范化；无关的自定义配置标识符保持原有兼容行为；
- reset 后再次 set 必须处理软删除与唯一索引冲突；
- SQLite、MySQL、PostgreSQL 的新装、升级、重复执行结果一致。

如果后续确实需要迁移，只能增加新的前向、幂等、窄范围迁移，不能编辑已经发布的历史迁移。

### 滚动升级顺序

1. 备份数据库，并只盘点 theme 范围内的非法历史值；
2. 执行附加 revision 迁移，保留原应用/个人配置表和全部未知行；
3. 部署支持 revision 的后端，并在依赖 revision 顺序前排空旧后端写流量——旧实例不会推进 `ConfigRevision`；
4. 部署新版前端。新版前端通过 `Accept: application/vnd.mss.theme.v1+json` 请求规范资源；遇到旧后端空成功体时会再 GET，一旦读取到 versioned 资源才发送 `If-Match`；
5. 监控迁移错误、412、旧媒体类型请求、缺失 `If-Match`、Redis 告警和审计 outcome；旧前端资源排空后，再通过独立兼容版本同时移除应用 `pwa` 旧投影并评审是否强制 `If-Match`；
6. 只有外部 MySQL/PostgreSQL 升级矩阵和完整浏览器 E2E 通过后，才把 capability 从 `planned` 提升。

## 回滚

上线前备份数据库。回滚时先停止主题写入，避免新旧 writer 绕过同一 revision 契约，再一起回滚前后端。仍然存在的旧格式主题行保持兼容，不需要恢复整张配置表；附加 revision 表可以保留不用，待独立清理版本再处理。

`null` 和 DELETE 是用户明确发起的覆盖删除，代码回滚不会自动恢复这些选择。只有在用户或管理员明确要求时，才从上线前备份或脱敏审计记录恢复指定字段。回滚同步机制时，还要清理 revision 版本公共缓存，以及新版浏览器快照、同步事件和认证会话 key。Redis 清理失败不要求回滚数据库，因为缓存受 revision 约束且会过期。

## 分阶段开发计划

### P0：契约与准备（已完成）

- 固化机器规格、ADR、产品说明、字段表、API 语义和验收矩阵；
- 将能力目录标记为 planned，不能提前宣称完整；
- 盘点现有 theme 行、非法值、权限策略和审计跳过规则；
- 明确 `pwa` 不属于运行时主题。

### P1：后端正确性（当前分支已实现）

- typed theme DTO 与规范化；
- 复用现有 GET/PUT，增加 DELETE；
- 事务式 set/unset、逐项/整组重置；
- application RBAC、authenticated-self、正反权限测试；
- 修正个人配置审计误跳过；
- 规范化主题资源、API 兼容适配和基础自动化证据。

### P2：前端作用域与运行时（当前分支已实现）

- 不可变代码默认、纯 resolver、字段来源；
- 共享编辑器的 application/user adapter；
- 稀疏 form state、逐项/全部重置；
- 保存立即应用；
- 登录、登出、账号切换和错误降级；
- 应用写权限只读态和统一主题运行时状态。

### P3：revision、同步与首屏（实现已进入当前分支）

- `ConfigRevision` 复合主键表和幂等附加迁移；
- 扁平 canonical 资源、`_meta`、强 ETag、`If-Match` 和 412 冲突；
- 数据库 revision 版本化 public-profile TTL 缓存及 Redis 降级；
- BroadcastChannel/storage、聚焦复核、脏草稿冲突和身份绑定 24 小时快照；
- 结构化、无主题值的成功/失败审计元数据。

### 发布门禁（尚未完成）

- 外部 MySQL/PostgreSQL fresh、upgrade、并发、reset→set 和迁移幂等矩阵；
- 真实双用户、真实多标签页、登录/登出/切换账号 E2E；
- 暖启动第一可见画面 trace、local/prod 等价性；
- 中文/英文、移动布局、键盘焦点、图表和自定义主色自动对比度；
- 完整 OpenAPI/客户端漂移检查与全仓验证。

## 验收矩阵

| 领域 | 场景 | 期望 | 证据 |
| --- | --- | --- | --- |
| 默认 | app/user 均无覆盖 | 全字段来自 code | resolver unit |
| 应用 | app 只覆盖主色 | 主色来自 application，其余来自 code | resolver unit |
| 个人 | user 只覆盖导航 | 导航来自 user，其余继续继承 | resolver unit |
| false | app=false 覆盖 code=true | false 生效，来源 application | resolver unit |
| false | user=false 覆盖 app=true | false 生效，来源 user | resolver unit |
| 省略 | PUT 不包含某字段 | 该字段数据库不变 | API integration |
| 单项重置 | PUT 字段为 null | 删除当前 scope 覆盖并立即继承 | API + component |
| 全部重置 | DELETE theme group | 只清当前 scope | API + E2E |
| 校验 | 同一请求含一个非法字段 | 全部拒绝、无部分写入 | transaction test |
| application scope | 打开应用主题 | 只调用 app-config API | Jest |
| user scope | 打开桌面/移动个人主题 | 只调用 user-config API | Jest |
| root 个人设置 | root 保存个人主题 | 只改 root 用户行，不改应用行 | integration |
| 应用权限 | 无 config:write 写应用 | 403 且数据库不变 | security negative |
| 用户隔离 | 尝试指定其他 user ID | 拒绝或忽略客户端 ID，只操作自己 | security negative |
| 公共启动 | 未登录读取 profile | 仅返回主题白名单 | API contract |
| 原子性 | 中途注入数据库失败 | 所有字段保持旧值 | Go integration |
| 资源格式 | GET/PUT/DELETE 成功 | 扁平 overrides、`_meta`、强 ETag 一致 | API contract |
| 条件写 | stale `If-Match` | 412 返回 `data.current`，无数据库写入 | concurrency integration |
| 兼容写 | 旧客户端不带 `If-Match` | 兼容期内成功并返回新 revision | rolling-upgrade integration |
| 公共缓存 | 旧 revision 缓存仍存在 | 新请求不接受旧缓存 | cache integration |
| Redis 降级 | 提交后缓存 set/delete 失败 | API 保持成功，数据库结果可读 | fault injection |
| 审计 | app/user 保存和重置 | scope、字段、结果、revision 完整 | audit test |
| 登录 | 登录页 → 登录成功 | 自动加入个人层，无需刷新 | Playwright |
| 登出 | 个人主题激活时退出 | 清个人层并回到应用主题 | Playwright |
| 切换账号 | A → 退出 → B | B 不看到 A 的个人主题 | Playwright |
| 多标签 | A 标签保存，B 标签已打开 | B 收到同 revision 并更新 | multi-page E2E |
| 乱序 | 旧 revision 后到 | 不覆盖新主题 | unit/integration |
| 脏表单 | B 有草稿时 A 保存 | B 保留 touched 字段并显示冲突，不自动重试 | multi-page E2E |
| 快照隔离 | A 快照存在后切换 B | B 不读取 A 快照，过期快照自动删除 | session unit + E2E |
| 错误回退 | app 或 user 读取失败 | 使用低层级值但不写覆盖 | Jest |
| 首屏 | 暗色暖缓存刷新 | 第一个可见画面无浅色闪烁 | browser trace |
| 移动端 | 两 scope 编辑与重置 | 无横向溢出且操作等价 | mobile E2E |
| 无障碍 | light/realDark/自定义主色 | 文本、焦点、图表达到约定对比度 | automated contrast |
| 国际化 | zh-CN/en-US | scope、来源、继承、错误完整 | locale contract |
| 生产构建 | local 与 prod | 相同输入得到相同有效主题 | dual-build browser |
| 数据库 | fresh/upgrade/reset→set | SQLite/MySQL/PostgreSQL 一致 | migration matrix |

## 下一可执行步骤

先冻结当前 P3 wire/schema 契约并完成本地专项回归，再依次运行外部 MySQL、PostgreSQL 的 fresh/upgrade/concurrency/reset→set 矩阵；随后用真实双用户和两个浏览器页验证 412、脏表单、BroadcastChannel/storage 回退、身份隔离及 24 小时快照，最后补 production 首屏 trace、移动端、浅色/深色和自动对比度门禁。全部证据通过前保持 `planned`。
