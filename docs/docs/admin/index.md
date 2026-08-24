---
title: Admin 产品概览
order: 10
nav:
  order: 0
  title: Admin
description: mss-boot Complete Admin Distribution 的产品定位、组成、内置能力与扩展边界
keywords: [admin mss-boot-admin complete distribution thin host react ant design]
---

# Admin 产品概览

<code>mss-boot-admin</code> 是基于 Go、Gin、GORM、Casbin、React 19、Ant Design 6.6 和
Umi Max 4 的完整管理系统基础设施。它既是可直接部署的参考 Admin，也是可以被 Thin Host
通过版本化 Go Module 和单一 npm 包引入的 Complete Admin Distribution。

:::info
**v1.3.2 状态**

<code>v1.3.2</code> 已从精确 merged-main 提交
<code>635fbb03a82976941e527d8ac1000fec0624abac</code> 完整发布并完成公开对账，现为当前稳定版。
<code>v1.2.3</code> 是上一稳定版；不完整的 v1.3.0 列车和完整预览
<code>v1.3.0-rc.6</code> 保持不可变。
:::

## 一套产品，四个协同组件

| 组件 | 作用 |
| --- | --- |
| <code>mss-boot/</code> | 领域中立 Framework，提供 HTTP、配置、运行时资源和基础设施抽象 |
| <code>admin/</code> | 完整 Admin 后端、迁移、权限、菜单、API、可导入应用和业务模块接口 |
| <code>web/antd-v6/</code> | 唯一正式前端，也是 <code>@mss-boot-io/admin-web</code> 的来源 |
| Root Foundation | <code>mss</code> CLI、机器契约、Blueprint、生成、验证、评测、升级与发布治理 |

Root、Framework、Admin 和 Admin Web 在产品层面使用同一个 Distribution 版本。各组件可以
独立发布，但不支持任意混配。

## 内置产品能力

### 身份和权限

- HttpOnly 浏览器 Session、签名 CSRF 和一次性 WebSocket ticket；
- 用户、角色、部门、岗位、菜单、API 注册表与 Casbin RBAC；
- 角色菜单/API 授权、权限正负路径和后端强制执行；
- OAuth2 账号绑定、Personal Access Token、密码与近期身份验证；
- 在线会话查看和撤销、登录日志与审计日志。

### 组织、配置和运营

- 组织树、岗位、选项、国际化资源；
- 系统配置、应用配置、用户配置和分层主题；
- 通知公告、定时任务与执行记录；
- 监控、统计、日志、告警和基础运行状态；
- 文件上传、对象存储与事件等可选集成入口。

### 完整业务模块

受版本控制的 AdminModule 规格可以确定性投影：

- 数据模型、DTO、Service、API 和 OpenAPI；
- 前向迁移、权限、菜单、API 元数据和领域事件；
- React 列表、筛选、表单、详情、路由、国际化和 API client；
- 正负授权测试、生成测试、E2E 和模块文档。

Supplier 是黄金样例。默认 Thin Host 通过“采购管理 → 供应商管理”从侧边栏进入，同一模块
同时证明迁移、菜单、图标、API 绑定、CRUD、权限拒绝和刷新行为。

## Thin Host：推荐的业务项目形态

业务仓库不再复制完整 Foundation。Thin Host 只保存：

- 很薄的 Go 组合入口；
- 自有业务模块和业务前端页面；
- AdminModule/Feature 规格、迁移、测试和文档；
- <code>.mss/lock.yaml</code>、部署与 CI；
- Foundation 管理的少量组合胶水。

后端业务模块在编译期注册到同一个 Admin 应用；前端业务页面在构建期进入同一个 Umi 路由树。
最终仍然只有一个后端二进制、一个前端 <code>dist</code>、一套 Session 和一套权限体系。

## API 注册表是必需的部署步骤

数据库迁移不会代替实际路由同步。首次安装或升级后要使用与常驻服务相同的版本、Stage 和 DSN
执行一次：

~~~bash
cd admin
STAGE=local go run . server -a
~~~

随后在“权限管理 → 菜单管理”中选择 <code>MENU</code> 或 <code>COMPONENT</code>，点击
“绑定 API”，可选项必须非空。只有健康检查或页面能打开，不代表权限注册表已经完成。

详见 [v1.3.2 安装与升级合同](/releases/v1-3-2)。

## 已退役边界

- Ant Design 5 已退役，不是回滚面，也不再接受兼容性修复；
- Admin 运行时动态模型、虚拟 CRUD 和浏览器代码生成已移除；
- 多租户已移除，当前产品是单租户 Admin；
- 不提供 Qiankun、Module Federation、iframe、远程业务代码或第二套 SPA；
- 历史动态模型表可以为保护用户数据而保留，但不代表相关运行时能力仍可用。

仓库级 <code>cmd/mss</code> 是开发期确定性工具，与已经移除的浏览器运行时生成器不是同一
能力。

## 推荐阅读

- [当前功能总览](/admin/current-capabilities)
- [快速开始](/admin/quickly)
- [完整 Admin Distribution 与 Thin Host 架构](/architecture/complete-admin-distribution-and-thin-business-host)
- [Agent 开发入口](/agent)
- [权限与组织治理](/admin/governance-guide)
- [页面展示配置发布治理](/admin/presentation-configuration)
- [生产和安全基线](/admin/security-baseline)
- [Docker 部署](/admin/docker)
- [v1.3.2 发布、升级与回滚](/releases/v1-3-2)

## 反馈

文档和产品源码位于同一个
[`mss-boot-admin` 仓库](https://github.com/mss-boot-io/mss-boot-admin)。一般问题请提交
[GitHub Issue](https://github.com/mss-boot-io/mss-boot-admin/issues)；疑似漏洞请遵循
[Security Policy FAQ](/devops/security-policy-faq)，不要公开披露可被滥用的细节。
