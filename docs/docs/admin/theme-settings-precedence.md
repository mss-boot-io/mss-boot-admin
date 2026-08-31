---
title: v1.3.7 候选主题设置优先级、继承与重置
order: 14
nav:
  order: 1
  title: admin
description: Ant Design V6 应用主题与个人主题的唯一运行时契约
keywords: [admin ant-design-v6 theme settings precedence inheritance]
---

:::warning
发布状态：v1.3.2 仍是当前稳定版；v1.3.5 与 v1.3.6 已永久停止并保持不可变部分发布；v1.3.7 已选
为 release candidate，但尚未稳定且不可采用。官方 npmjs 包和 Root 工具可能处于不同公开
阶段，必须以远端发布台账为准；完整 Distribution stable promotion 和最终 current-stable policy
对账完成前，本页不是 Admin Web 安装、创建或升级指引。Docs 网站可通过 `docs/v*` 异步候补，
其状态不影响这一采用门禁。
:::

## 产品合同

- Admin Web 候选身份：`@mss-boot-io/admin-web@1.3.7`，公开阶段以远端 npm 与 Release 台账为准；
- Admin 后端候选身份：`github.com/mss-boot-io/mss-boot-admin/admin@v1.3.7`，公开阶段以远端 Go 与 Release 台账为准；
- 历史状态：v1.3.5 与 v1.3.6 的已公开组件保持不可变，但不能补全或混用；
- 源码状态：主题继承、重置、并发前置条件和授权已在 Foundation 中实现；
- 采用状态：只有未来完成全部公共对账的协调版本才能把这套源码合同作为 Thin Host 产品面；
- 设计历史保留在[默认 V6 切换 ADR](https://github.com/mss-boot-io/mss-boot-admin/blob/main/docs/adr/2026-08-17-ant-design-v6-default-cutover.md)，使用方无需 Foundation 源码。

本页只描述 V6 规范契约。后端不再返回旧主题投影，不接受缺少修订号的写入，
前端也不会从 V5 的本地存储键、媒体类型或无版本响应中恢复状态。

## 核心规则

每个字段独立解析，低层值不会因为高层仅覆盖其他字段而失效：

```text
最终主题 = 不可变代码默认值
         + 稀疏应用覆盖
         + 稀疏当前用户覆盖
```

优先级为“用户 > 应用 > 代码”。判断依据是字段是否存在且有效，不使用 JavaScript
truthy 语义；因此 `false` 是有效覆盖。省略字段表示不修改，`null` 表示删除当前作用域
的覆盖并恢复继承，空字符串始终是非法输入。

## 字段契约

| 字段 | 类型 | 代码默认值 | 有效值 |
| --- | --- | --- | --- |
| `navTheme` | enum | `light` | `light`、`realDark` |
| `layout` | enum | `mix` | `side`、`top`、`mix` |
| `contentWidth` | enum | `Fluid` | `Fluid`、`Fixed` |
| `fixedHeader` | boolean | `false` | `true`、`false` |
| `fixSiderbar` | boolean | `true` | `true`、`false` |
| `colorWeak` | boolean | `false` | `true`、`false` |
| `colorPrimary` | color | `#1677ff` | 规范化六位十六进制颜色 |

`fixSiderbar` 是既有持久化字段名，本次不做破坏性数据库重命名。`pwa`、
`splitMenus`、Service Worker 生命周期和任意未知字段不属于运行时主题，V6 API 会拒绝写入。

## 资源和 HTTP 契约

主题继续复用应用配置和个人配置资源，不建立第二套持久化服务：

| 作用域 | 读取 | 稀疏更新 | 整组重置 |
| --- | --- | --- | --- |
| 应用 | `GET /admin/api/app-configs/theme` | `PUT /admin/api/app-configs/theme` | `DELETE /admin/api/app-configs/theme` |
| 当前用户 | `GET /admin/api/user-configs/theme` | `PUT /admin/api/user-configs/theme` | `DELETE /admin/api/user-configs/theme` |

规范媒体类型为：

```text
application/vnd.mss.theme.v1+json
```

资源顶层只包含七个稀疏字段和 `_meta`：

```json
{
  "navTheme": "realDark",
  "fixedHeader": false,
  "_meta": {
    "v": 1,
    "scope": "application",
    "revision": "7"
  }
}
```

每次 GET、PUT 和 DELETE 都返回与资源一致的强 ETag：

```text
"theme-{scope}-{revision}"
```

PUT 和 DELETE 必须携带上次读取到的 `If-Match`。缺少前置条件返回 `428`，弱 ETag、
错误作用域或非规范格式返回 `400`，修订号过期返回 `412` 并附带当前规范资源。
HTTP 层与服务层使用同一强制约束，不存在可传空修订号的内部兼容入口。

所有权威主题和公开 profile 响应都使用 `Cache-Control: no-store`。缺少 `_meta`、版本错误
或修订号非法的响应是明确的契约错误，V6 客户端不会把它降级为无版本资源。

## 权限边界

- 应用主题读取需要 `config:read`，更新和重置需要 `config:write`。
- 个人主题只允许已认证当前用户；owner 从服务端验证身份派生，不读取客户端 user ID。
- 前端隐藏按钮和本地 403 仅改善体验，后端仍是最终授权点。
- `/admin/api/app-configs/profile` 只返回明确允许的应用启动字段，不包含个人覆盖、私密配置或 `pwa`。
- 主题审计只记录操作者、作用域、字段名、结果和修订号，不复制主题值或凭据。

## 事务、修订和缓存

一个请求中的设置与删除在同一数据库事务完成。任何字段非法时先整体拒绝；事务失败时
不产生部分覆盖、成功审计或伪造的新修订号。

应用主题和每个用户主题分别使用 `mss_boot_config_revisions` 的单调修订号。应用主题提交
还会在同一事务推进公开 profile 修订号，避免缓存先于数据库事实源成为“新状态”。

Redis 只保存带版本、带 TTL 的可丢弃投影：

- 缓存键由数据库修订号派生；
- 命中后仍用权威修订号确认快照未变化；
- 读、写或清理 Redis 失败不能把已提交数据库事务改写成 HTTP 失败；
- V6 不再读取或清理历史无版本 AppConfig 缓存命名空间。

## V6 前端实现

应用设置与个人设置复用一个 `ThemeSettingsEditor`，但必须显式传入
`application` 或 `user` 作用域。编辑器展示“当前有效值”和“来源”，保存时只提交用户
真正设置或恢复继承的字段，不把继承后的完整主题复制到高优先级作用域。

运行时在以下事件后统一重新解析主题：

- 匿名启动和登录页加载应用层；
- 登录成功后加载已验证用户的个人层；
- 保存、重置、退出登录和切换账户；
- 标签页重新可见、获得焦点或收到跨标签同步事件。

浏览器持久化和 BroadcastChannel 统一使用 `mss.antd-v6.theme.*` 命名空间。
个人快照绑定随机认证会话与服务端验证的用户身份，最多保留 24 小时；乱序或过期事件被拒绝。
编辑器存在未保存修改时，新权威资源只触发冲突提示，不静默覆盖或自动重试草稿。

## 数据升级

升级保留七个支持字段的有效应用和用户覆盖，也保留不属于当前主题契约的未知业务行，
但这些未知行不会进入 V6 运行时。历史字符串布尔值和有效颜色会按规范解析；非法值忽略并
回退到更低层。

V6 切换迁移会删除精确的退役应用 `theme/pwa` 行，并清理退役浏览器 OAuth 配置键；
大小写、重音或其他不确定别名不做猜测性删除。迁移是前向、幂等的，不能删除未知配置或
用户数据。数据库升级与 V6 后端、V6 静态资源作为一个版本对发布。

## 不支持的降级方式

- 不返回无 `_meta` 的主题 JSON；
- 不根据 `Accept` 切换旧投影；
- 不允许缺少 `If-Match` 的更新或重置；
- 不公开或恢复 `pwa`；
- 不读取 V5 localStorage、BroadcastChannel 或旧 AppConfig Redis 键；
- 不以部署旧前端作为回滚方案。

需要回滚时，只能回滚到上一组已经验证且相互匹配的 V6 前端、后端和数据库兼容版本。
任何回滚都不能恢复已退役的浏览器认证或主题协议。

## 发布门禁

提 PR 前集中执行以下验证：

1. SQLite、MySQL、PostgreSQL 的新库迁移、旧库升级、重复迁移和 reset→set。
2. 应用/个人 GET、PUT、DELETE 的 428、400、412、成功修订和事务回滚矩阵。
3. 匿名、登录、退出、换号、多标签、脏草稿冲突和 24 小时快照边界。
4. 中文/英文、桌面/移动、明暗主题、键盘焦点、色弱模式和品牌色保存。
5. V6 生产构建、浏览器控制台零弃用警告和后端路由表无退役入口。

完整发布资格以实际测试报告和已合入 `main` 的提交为准，不能用开发期人工检查代替。
