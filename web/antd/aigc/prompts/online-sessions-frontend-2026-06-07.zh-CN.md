# 在线会话与强制下线 · 前端 UI（Online Sessions & Force Logout）

> 范围：`mss-boot-admin-antd` Phase 1 前端 UI（对接后端 PR `mss-boot-io/mss-boot-admin#376`，issue `mss-boot-io/mss-boot-admin#373`）。
>
> 适用代码版本：本文件随 `feat/online-sessions-frontend` 分支提交（2026-06-07）。
>
> 关联文档：后端设计 `aigc/prompts/online-sessions-design-2026-06-07.zh-CN.md`

## 1. 背景与动机

后端已落地"在线会话与强制下线"能力（PR #1），暴露 5 个 REST 端点 + 在 JWT claims 增加 `sid`。本期前端只做两件事：

1. 管理员一个完整的会话管理页：`/security/online-sessions`
2. 顶部 "退出登录" 同期改造为先调用 `POST /online-sessions/logout`，让服务端 sid 同步失效 + 写审计

明确**不在本期范围**：

- 账号中心/账号设置不动（后端未提供"我的会话列表"接口）
- `/users` 列表不增加"踢出该用户全部会话"快捷入口
- WebSocket 主动通知"已被踢"（前端轮询 401 跳登录即可）
- PAT 与在线会话视图合并

## 2. 决策汇总（已与后端对齐）

| 议题 | 决策 |
|------|------|
| 菜单位置 | 新增一级菜单"安全"（`menu.security`），下挂"在线会话"子菜单 |
| 详情承载 | Drawer（GET `/online-sessions/:id`） |
| 列表交互 | ProTable + 顶部 search form（status / userID / username / ip） |
| 单条踢下线 | Popconfirm 二次确认 |
| 批量踢下线 | 顶部工具栏按钮 + Modal 二次确认（输入 userID） |
| 强制下线原因 | 前端不收集，后端固定写 `force-by-session` / `force-by-user` |
| Logout 改造 | 同期改造顶部"退出登录"为先调用 `POST /online-sessions/logout`，失败静默 |
| API service / 类型 | 等后端 swagger 产出后由项目原有 `npm run openapi` 覆盖；本期先手写一份与生成器风格一致的占位 |
| User 列表联动 | 本期不动 |
| 账号中心联动 | 本期不动 |

## 3. 架构总览

```
登录态 ──► localStorage(token) + JWT(sid)

进入 /security/online-sessions ──► ProTable.request
                                  └─► GET /admin/api/online-sessions?status=active...
                                  └─► PageOK 响应：{ data: UserSession[], total, current, pageSize }

行点击用户名 / 操作"详情" ──► SessionDetailDrawer
                            └─► GET /admin/api/online-sessions/:id → Descriptions 渲染

行操作"强制下线" ──► Popconfirm.onConfirm
                  └─► DELETE /admin/api/online-sessions/:id
                  └─► actionRef.reload()

行操作或工具栏"踢出该用户全部会话" ──► RevokeUserModal
                                      └─► DELETE /admin/api/online-sessions/user/:userID
                                      └─► message.success(通用文案) + reload()

顶部 logout ──► try { POST /admin/api/online-sessions/logout } catch {}
              └─► localStorage 清理 + history.replace('/user/login')
```

## 4. 信息架构

### 4.1 路由（`config/routes.ts`，置于 `/log` 之前）

```ts
{
  path: '/security',
  name: 'menu.security',
  icon: 'safety',
  routes: [
    { path: '/security', redirect: '/security/online-sessions' },
    {
      path: '/security/online-sessions',
      name: 'menu.security.online-sessions',
      icon: 'desktop',
      component: './OnlineSession',
    },
  ],
},
```

### 4.2 目录结构（`src/pages/OnlineSession/`）

```
src/pages/OnlineSession/
├── index.tsx                        ProTable 主页（含筛选、列、操作）
├── utils.ts                         getSessionStatus / 状态枚举映射
└── components/
    ├── SessionDetailDrawer.tsx      详情抽屉（按 id 拉取详情）
    └── RevokeUserModal.tsx          "踢该用户全部会话"二次确认 Modal
```

文案统一进 `src/locales/{zh-CN,en-US}/{menu,pages}.ts`，本目录不做本地 i18n。

### 4.3 i18n key

```
menu.security
menu.security.online-sessions

pages.onlineSession.title
pages.onlineSession.status.{active,revoked,expired}
pages.onlineSession.columns.{username,userID,ip,userAgent,loginAt,lastSeenAt,expiredAt,status,option}
pages.onlineSession.action.{detail,revoke,revokeUser,revokeUserToolbar}
pages.onlineSession.confirm.revoke
pages.onlineSession.confirm.revokeUser.{title,userIDLabel,userIDRequired}
pages.onlineSession.result.revoke.success
pages.onlineSession.result.revokeUser.success
pages.onlineSession.drawer.title
pages.onlineSession.drawer.field.{revokedBy,revokedAt,revokeReason}
```

## 5. 数据流与状态

### 5.1 ProTable.request → List

```
ProTable.request({ current, pageSize, ...form }) →
  getOnlineSessions({
    current, pageSize,
    status: form.status ?? 'active',
    userID, username, ip,
  })
  → { data, total, success: true }
```

后端默认 `status=active`、按 `last_seen_at DESC` 排序，前端不需要传 sort。

### 5.2 行操作 → 强制下线

```
onRevoke(row) → Popconfirm → deleteOnlineSession({ id: row.id })
             → message.success → actionRef.current?.reload()
```

### 5.3 工具栏 / 行操作 → 踢某用户全部

```
工具栏按钮 → RevokeUserModal（输入 userID）
行操作链接 → RevokeUserModal（userID 预填，输入框禁用）
→ deleteOnlineSessionUser({ userID })
→ message.success(`已踢出该用户的全部会话`) → reload()

注：后端 DELETE 走 HTTP 204 + body（规范上 204 不应有 body，客户端解析不可靠），前端 UI 不依赖响应中的 `affected` 字段。
```

### 5.4 详情 Drawer

```
点击行 username 或 "详情" → setDetailId(row.id) → 打开 Drawer
Drawer 内 useRequest(() => getOnlineSession({ id: detailId }), { refreshDeps: [detailId] })
```

### 5.5 Logout 改造（`src/components/RightContent/AvatarDropdown.tsx`）

```ts
const loginOut = async () => {
  try {
    await postOnlineSessionLogout();  // 让后端 sid 失效 + 写审计
  } catch {
    // ignore：后端不可用时也要让用户能登出
  }
  // 既有的 localStorage 清理 + history.replace('/user/login') 不变
};
```

要求：

- `postOnlineSessionLogout` 失败时静默吞掉，不弹 message.error
- 401 全局拦截器的自动跳登录不受影响（在 `requestErrorConfig.ts`）

## 6. 列与渲染细则（`src/pages/OnlineSession/index.tsx`）

| 列 | 字段 | 渲染 |
|----|------|------|
| 用户名 | `username` | `<a onClick={openDetail(row.id)}>` |
| 用户 ID | `userID` | `Typography.Text` + copyable + ellipsis |
| IP | `ip` | 文本 |
| User-Agent | `userAgent` | `Typography.Text` ellipsis + Tooltip 完整 UA |
| 登录时间 | `loginAt` | `valueType: 'dateTime'` |
| 最后活跃 | `lastSeenAt` | `dayjs.fromNow` + Tooltip 绝对时间 |
| 过期时间 | `expiredAt` | `valueType: 'dateTime'` |
| 状态 | 计算 | `<Tag color>`，green / red / default |
| 操作 | — | 详情 / 强制下线（仅 active）/ 踢该用户全部（仅 active，预填 userID） |

`getSessionStatus(row, now)`（`utils.ts`）：

```ts
if (row.revoked) return 'revoked';
if (row.expiredAt && dayjs(row.expiredAt).isBefore(now)) return 'expired';
return 'active';
```

## 7. 顶部 search form

ProTable 内建 search：

- `status` Select：`all | active | revoked | expired`，`initialValue: 'active'`；选 `all` 时显式向后端发 `status=all`（后端 list switch 不命中任何 case → 不加 status 过滤）。不能用 undefined/空字符串表示全集——后端会把空 status 默认为 `active`。
- `userID` Input
- `username` Input（后端 LIKE）
- `ip` Input

## 8. 工具栏

- 刷新（ProTable 自带）
- "踢出指定用户全部会话" 按钮 → RevokeUserModal（无预填 userID）

## 9. API service 与类型

### 9.1 与后端的实际响应对齐

| 方法 | HTTP | 实际响应（后端确认） |
|------|------|---------------------|
| `getOnlineSessions` | GET `/admin/api/online-sessions` | `{ data: UserSession[], total, current, pageSize }`（`response.PageOK`） |
| `getOnlineSession` | GET `/admin/api/online-sessions/:id` | 裸 `UserSession` JSON（`api.OK`） |
| `deleteOnlineSession` | DELETE `/admin/api/online-sessions/:id` | `{ id, userID, revokedAt }` |
| `deleteOnlineSessionUser` | DELETE `/admin/api/online-sessions/user/:userID` | `{ affected, userID }`（HTTP 204；规范上 204 不应有 body，前端不依赖响应字段，使用通用 success 文案） |
| `postOnlineSessionLogout` | POST `/admin/api/online-sessions/logout` | `{ ok: true }`（POST 走 201） |

### 9.2 TS 类型（`src/services/admin/typings.d.ts`）

```ts
namespace API {
  type UserSession = {
    id?: string;
    userID?: string;
    username?: string;
    roleID?: string;
    loginAt?: string;
    lastSeenAt?: string;
    expiredAt?: string;
    ip?: string;
    userAgent?: string;
    revoked?: boolean;
    revokedAt?: string;
    revokedBy?: string;
    revokeReason?: 'logout' | 'force-by-session' | 'force-by-user';
    createdAt?: string;
    updatedAt?: string;
  };

  type getOnlineSessionsParams = {
    status?: 'active' | 'revoked' | 'expired';
    userID?: string;
    username?: string;
    ip?: string;
    current?: number;
    pageSize?: number;
  };

  type getOnlineSessionParams        = { id: string };
  type deleteOnlineSessionParams     = { id: string };
  type deleteOnlineSessionUserParams = { userID: string };

  type OnlineSessionRevokeResult     = { id?: string; userID?: string; revokedAt?: string };
  type OnlineSessionRevokeUserResult = { affected?: number; userID?: string };
}
```

注：`revokeReason` 的枚举值与后端 `models.SessionRevokeReason` 严格一致（仅 3 个）。

### 9.3 service 文件（`src/services/admin/onlineSession.ts`）

按项目现有 `services/admin/*.ts` 的 openapi-generated 风格手写，等后端 swagger 拉到本地后跑 `npm run openapi` 覆盖即可。

## 10. 权限

- 路径 `/admin/api/online-sessions*` 由后端 Casbin 控制
- 前端菜单与"踢下线"按钮显隐由 `useAccess()` 判定，沿用现有 `src/access.ts` 模式
- 现状 `access.ts` 仅返回 `currentUser.permissions`，本期不新增 access key

## 11. 边界与异常

| 场景 | 行为 |
|------|------|
| 管理员强制下线自己当前会话 | 下次请求 401 → `requestErrorConfig.ts` 自动跳登录，无需特殊处理 |
| `revokedBy` 为空 | 渲染 "—" |
| 长 UA 撑爆单元格 | `Typography.Text ellipsis` + Tooltip 完整内容 |
| Logout 接口失败 | 静默，继续本地清理 |
| List 返回 0 | ProTable 自带 empty 态 |
| 全局错误拦截 | 依赖 `requestErrorConfig.ts`，本页不重复 try/catch |

## 12. 质量门控与验证

项目内 pages 单测覆盖很少，本期采用：

1. `npm run tsc` 通过（`tsc --noEmit`）
2. `npm run lint:js` 通过（eslint）
3. `prettier --check` 对本次涉及文件通过（避免触碰 baseline 文件）
4. `npm start` 编译通过（Webpack 报 Compiled）
5. 手工 UI 验证清单（用 admin 账号登录后端环境）：
   1. 默认进入 status=active 的列表能加载
   2. 切换 status 到 revoked / expired，列表数据正确切换
   3. userID / username / ip 各筛选一次
   4. 单条强制下线 → success message + 列表 reload
   5. 工具栏"踢用户全部" → 通用成功提示
   6. 行操作"踢用户全部" → 弹窗内 userID 已预填且禁用编辑
   7. 详情 Drawer 完整字段可见，UA 完整渲染
   8. 顶部"退出登录" → 后端 `mss_boot_audit_logs` 有 `action=logout` 记录
   9. 中英文切换：无 `Missing message:` 警告

PR 描述附 4 张截图：列表 / 筛选 / Popconfirm / Drawer。

## 13. 兼容性与回滚

- 后端 `SessionEnabled=false` 时旧 JWT 仍工作，本页可空载（列表为空），不破坏现有访问
- Logout 改造在后端不可用时静默退化，不会卡住用户
- 回滚：删除 `/security/online-sessions` 路由 + revert Logout 改造（保留 service 文件无害）
- 与 PAT 完全独立（PAT 仍在 `src/pages/Account/Settings/components/AccessToken.tsx` 不动）

## 14. 文件清单（实施落地）

新增：

```
src/services/admin/onlineSession.ts
src/pages/OnlineSession/index.tsx
src/pages/OnlineSession/utils.ts
src/pages/OnlineSession/components/SessionDetailDrawer.tsx
src/pages/OnlineSession/components/RevokeUserModal.tsx
```

修改：

```
config/routes.ts                                    新增 /security 一级菜单
src/services/admin/typings.d.ts                     追加 UserSession / 参数 / 结果类型
src/locales/zh-CN/menu.ts                           menu.security{,.online-sessions}
src/locales/en-US/menu.ts
src/locales/zh-CN/pages.ts                          pages.onlineSession.*
src/locales/en-US/pages.ts
src/components/RightContent/AvatarDropdown.tsx     loginOut 增加 postOnlineSessionLogout()
```
