---
title: 当前功能总览
order: 13
nav:
  order: 1
  title: Admin
description: 基于 v1.3.2 当前稳定版代码和机器契约梳理 mss-boot Admin 的已实现能力与边界
keywords: [admin capabilities features session rbac generator thin host]
---

# 当前功能总览

本文描述 <code>v1.3.2</code> 当前稳定版的实现，不是未来路线图。事实来源包括
<code>admin/</code>、<code>web/antd-v6/</code>、<code>admin/modules/supplier/</code>、
<code>.mss/</code>、迁移、测试和外部消费者门禁。当前稳定提交是
<code>635fbb03a82976941e527d8ac1000fec0624abac</code>。

## 1. 浏览器会话和账号安全

- 用户名/密码与已配置的 OAuth2 登录；
- 服务端 HttpOnly Session Cookie，不把 Admin Bearer Token 暴露给浏览器业务代码；
- 签名 CSRF、受约束的 CORS 和一次性 WebSocket ticket；
- 当前用户资料、头像、密码修改和管理员重置；
- GitHub、Lark 等已配置 Provider 的绑定和解绑；
- Personal Access Token 的创建、轮换、撤销和查询；
- 在线会话列表、按会话或用户撤销、当前会话退出；
- 敏感账号操作的近期身份验证和审计记录。

OAuth、邮件和第三方身份是否可用取决于部署配置。前端控制只改善体验，所有敏感写入仍由
后端身份与权限检查保护。

## 2. RBAC、组织和 API 治理

- 用户、部门、岗位、角色和组织树；
- 菜单目录、页面、组件、按钮与 API 节点；
- Casbin 角色授权、数据范围和后端 API 强制执行；
- 角色菜单授权、菜单 API 绑定及授权修订；
- 真实 Gin 路由到 API 注册表的一次性同步；
- 权限正向、负向和拒绝时无副作用测试；
- 登录日志、操作审计和授权变更传播。

### API 注册表运行合同

首次安装或迁移后必须执行：

~~~bash
cd admin
STAGE=local go run . server -a
~~~

该命令使用完整已挂载路由生成 API 注册表。同步后，“权限管理 → 菜单管理”中的
<code>MENU</code>/<code>COMPONENT</code>“绑定 API”列表必须非空。若为空，应先检查新旧二进制、
Stage 或 DSN 混用，而不是手工补数据。

## 3. 配置、国际化和主题

- 系统、应用和用户级配置；
- Local、文件、数据库和受支持的外部配置源；
- 中文、英文国际化资源与动态语言管理；
- 应用默认、用户偏好和浏览器临时设置的分层主题优先级；
- 默认深色体验、跨标签页同步和首屏主题快照；
- 选项管理及受约束的动态枚举。

配置来源和可选外部服务需要部署者逐项启用；存在代码入口不等于任意环境默认可用。

## 4. 运营与可观测性

- 通知公告、未读/已读处理；
- 用户管理的定时任务和执行日志，由 <code>task.enable</code> 控制；
- 监控采样和会话清理等内置系统作业，与用户 Task 表分离；
- CPU、内存、磁盘、网络、Go Runtime 和实例内短期趋势；
- 统计接口、运行时日志查看与清理；
- 告警规则和已配置渠道通知；
- readiness、health、Prometheus 和 pprof 等基础运维入口。

健康检查只证明相应运行状态，不自动证明数据库迁移、API 注册表、业务权限或外部消息完成。

## 5. Complete Admin Distribution

### 可导入后端

<code>admin/app</code> 提供完整应用构造和执行入口；官方可执行程序与外部 Thin Host 使用同一
生命周期。<code>admin/business</code> 的最小接口允许业务模块显式注册：

- 描述信息和权限/菜单元数据；
- Core 之后执行的 Business 迁移；
- readiness；
- 认证与授权中间件之后的路由；
- 当前生成模块使用的事件能力。

Registry 完成组合后冻结。重复模块、无效迁移或未就绪路由 fail closed。

### 完整 Admin Web

<code>web/antd-v6</code> 同时是参考前端和
<code>@mss-boot-io/admin-web</code> 的源码。公开入口提供 Umi preset、业务路由注册、
Runtime、样式和测试辅助；包内 CLI 提供 <code>dev</code>、<code>lint</code>、
<code>test</code> 和 <code>build</code>。

下游只生成一个开发服务器、一个路由树、一个 React/AntD/Query Runtime 和一个
<code>dist</code>。

## 6. Agent-native 生成与升级

- <code>mss context</code> 和 <code>mss doctor</code> 读取项目事实源；
- Feature/AdminModule 规格在写代码前表达需求；
- Supplier 可以完整生成后端、迁移、权限、菜单、前端、测试、E2E 和文档；
- 生成器支持 dry-run、路径限制、稳定顺序和两次运行幂等；
- <code>mss verify --changed</code> 根据变更范围计算最小充分验证；
- <code>management-system</code> Blueprint 创建 Thin Host；
- <code>mss upgrade admin</code> 协调 Go/npm 版本和受管胶水，保留业务自有文件。

Supplier 是示例和回归契约，不把采购领域能力移动到领域中立的 <code>mss-boot/</code>。

## 7. 存储、队列和其他可选集成

框架与 Admin 包含配置源、缓存、Redis、队列、对象存储、文件上传和事件抽象。不同 Provider
有独立成熟度和部署要求。只有发布资格明确覆盖的 Provider 和行为才获得对应稳定声明；能构造、
能编译或容器健康都不是业务成功证明。

## 8. 已移除和不支持的能力

| 能力 | 状态 | 数据与兼容处理 |
| --- | --- | --- |
| Ant Design 5 | 已退役 | 不再构建、发布或作为回滚面 |
| 运行时动态模型/虚拟 CRUD | 已移除 | 菜单、API 和运行时入口清理；历史数据表可保留 |
| 浏览器模板/代码生成 | 已移除 | 使用受版本控制规格和 <code>cmd/mss</code> 开发期生成 |
| 多租户 | 已移除 | 当前是单租户 Admin |
| 第二套 SPA/远程业务入口 | 不支持 | 业务代码在 Go/Umi 构建期组合 |
| 自动采集采用者信息 | 不支持 | 创建、安装、运行和升级不增加遥测或 call-home |

## 9. 使用边界

- 菜单可见性取决于迁移、API 同步、角色授权和当前 Session；
- 可选集成取决于真实配置和外部服务；
- Admin、Framework、Admin Web 必须使用同一个协调版本；
- npmjs 是 Admin Web 默认公开安装源，不需要 Registry Token；`latest` 已指向 `1.3.2`，发布端使用绑定 `npm-release.yml` 与 `release-v6` 的 Trusted Publishing OIDC；GitHub Packages 仅作为完全相同制品的兼容镜像，选择镜像时 Token 不能写入仓库；
- 生产升级先备份并验证恢复，再迁移、同步 API、启动和切流；
- 回滚恢复上一组协调制品及其数据库备份，不临时 down migration。

下一步阅读：

- [完整 Admin Distribution 与 Thin Host 架构](/architecture/complete-admin-distribution-and-thin-business-host)
- [v1.3.2 安装、升级与回滚](/releases/v1-3-2)
- [权限与组织治理](/admin/governance-guide)
- [运行时开发工具移除说明](/admin/legacy-capability-deprecation)
- [Supplier 生成模块](/modules/supplier)
