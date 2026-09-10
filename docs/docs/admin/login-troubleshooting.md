---
title: v1.3.7 登录与会话排障
order: 14
nav:
  order: 1
  title: admin
description: v1.3.7 稳定版 HttpOnly Cookie 会话、CSRF、刷新、权限和同源代理合同
keywords: [v1.3.7 v1.3.5 v1.3.6 admin login session cookie csrf troubleshooting]
---

# v1.3.7 登录与会话排障

:::warning
发布状态：**v1.3.7 是当前稳定版**；v1.3.5 与 v1.3.6 已永久停止并保持不可变部分发布。
Docs 网站可通过 `docs/v*` 异步候补，不影响组件、稳定别名或采用。
:::

浏览器登录使用服务端会话和 HttpOnly Cookie；浏览器不会接收或保存 Admin JWT，
也不应把个人访问令牌（PAT）当作登录兜底。排查时只记录请求路径、状态码、错误 key
和脱敏时间线，不要公开密码、Cookie、CSRF 值、PAT、数据库连接串或生产地址。

## 先按现象定位

| 现象 | 优先检查 |
| --- | --- |
| 登录返回 401 | 用户是否存在、启用，密码是否正确，数据库和认证配置是否加载 |
| 登录返回 403 | 请求 Origin、CSRF 双提交值、账号或后端权限 |
| 登录后刷新又退出 | 会话 Cookie 的 Domain、Path、SameSite、Secure，代理转发和服务端会话存储 |
| 登录成功但菜单为空 | 用户角色、菜单和 API 权限；后端 RBAC 是最终权威 |
| 仅旧标签页异常 | 退出后重新登录，或用无痕窗口区分旧会话与当前构建 |

## 1. 确认服务与同源代理

Foundation 源码调试先按 `.mss/commands.yaml` 确认后端、前端与日志进程身份，再检查
健康入口和浏览器实际访问的 `/admin/api/**` 是否落到同一个开发后端。该 source-only
流程不能被复制为 v1.3.5 下游命令。前端应通过同源代理访问 API；不要额外配置浏览器
Bearer token、第二套 API host 或退役登录路由。反向代理必须保留 Cookie、Origin 和
CSRF 请求头。

## 2. 检查浏览器会话合同

当前登录入口是 `POST /admin/api/user/session/login`。成功响应建立 HttpOnly 会话
Cookie，并返回到期元数据而不是 Admin token。状态变更请求要求：

- Origin 精确匹配受信任来源；
- `X-CSRF-Token` 与签名的双提交 Cookie 匹配；
- 生产 Cookie 使用 Secure，Domain、Path 和 SameSite 与部署域名一致；
- 多实例部署共享符合配置合同的服务端会话存储。

若登录后立即退出，先在浏览器网络面板检查登录响应是否设置 Cookie、后续请求是否携带
Cookie、刷新接口是否成功。不要尝试把 Cookie 复制到 localStorage。

## 3. 清理单个浏览器的旧会话

先使用产品退出入口；无法退出时调用当前的 Cookie 清理入口后重新登录。无痕窗口正常而
原窗口失败，通常说明该站点仍有过期 Cookie、CSRF Cookie 或非敏感到期提示。只清理当前
站点数据，不要把清空全部浏览器数据作为服务器问题的修复。

## 4. 初始管理员与密码

新数据库没有默认密码。v1.3.7 Thin Host 的第一次交互式初始化通过内置隐藏提示读取
用户提供的强密码；非交互自动化只在初始化进程中从密钥存储注入一次性
`MSS_ADMIN_INITIAL_PASSWORD`。迁移成功后不再需要它。若数据库已经初始化，重新设置
该环境变量不会重置现有管理员密码。此时应走
受审计的自助改密、恢复身份或运维恢复流程。

全新数据库创建的初始用户名是 `admin`，密码是首次 setup 提供的值；系统没有默认密码。
本地 Admin Web 地址为 `http://127.0.0.1:8001`。

## 5. 登录成功但菜单为空

认证成功不代表拥有业务权限。依次检查：

1. 用户状态和角色绑定；
2. 角色的菜单与 API 权限；
3. 菜单是否启用、路由是否存在；
4. 目标 API 的后端授权结果；
5. 当前前端是否为与后端同一发行版本的构建。

必须同时验证允许和拒绝用例；隐藏前端按钮不能替代后端授权。

## 提交问题时

对 Foundation 源码问题，提供精确提交、页面和 API 路径、状态码、错误 key、发生时间、
是否仅影响特定用户或角色、无痕窗口是否可复现；不要把源码提交称为 v1.3.5 公共使用方
版本。响应头只保留无敏感信息的字段，截图前遮盖所有 Cookie、CSRF、PAT、邮箱和业务数据。

更多协议细节见 [浏览器会话、PAT 与 OAuth2](/admin/token-oauth2-guide)，本地进程与
代理见 [本地调试](/admin/local-debug)，权限边界见
[安全基线](/admin/security-baseline)。
