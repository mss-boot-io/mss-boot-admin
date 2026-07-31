---
title: 介绍
order: 10
nav:
  order: 0
  title: admin
description: 介绍mss-boot-admin
keywords: [admin mss-boot-admin]
---

## 简介

> `mss-boot-admin` 是基于 `Gin` + `React` + `Ant Design v5` + `Umi v4` + `mss-boot` 的前后端分离后台管理平台。当前产品主线聚焦于权限治理、组织管理、系统配置、通知任务、国际化、监控统计，以及 AI 注解协同驱动的研发流程。

## 当前产品主线

- 治理核心：用户、角色、部门、岗位、菜单、API 权限
- 运营能力：系统配置、通知公告、任务、监控、统计
- 平台能力：国际化、对象存储、事件能力、API-first 扩展
- 研发协同：以 AI 注解作为设计、实现、文档与交接的统一上下文载体

:::info
当前仓库中仍可看到动态模型与代码生成相关实现，但它们不再是后续阶段的主要产品投入方向。
:::

## 🎬 体验环境

[体验地址](https://admin-beta.mss-boot-io.top)

> 账号：`admin` 密码：`123456`
>
> 或者直接点击github图标通过`github`登录

## ✨ 特性

- 支持国际化
- 支持移动端 H5 响应式适配
- 标准 Restful API 开发规范
- 基于 Casbin 的 RBAC 权限管理
- 基于 Gorm 的数据库存储
- 基于 Gin 的中间件开发
- 基于 Gin 的 Swagger 文档生成
- 支持 oauth2.0 第三方登录
- 支持 swagger 文档生成
- 支持多种配置源(本地文件、embed、对象存储 s3 等、gorm 支持的数据库、mongodb)
- 支持数据库迁移
- 支持通知、任务、监控、统计等运营能力
- 支持 AI 注解协同的工程化演进方向

## 📦 内置功能

- 用户管理: 用户是系统操作者，该功能主要完成系统用户配置。
- 部门管理: 管理组织树结构，支撑数据归属与权限边界。
- 岗位管理: 管理岗位信息，辅助组织与权限配置。
- 角色管理: 角色菜单权限分配、设置角色按机构进行数据范围权限划分。
- 菜单管理: 配置系统菜单，操作权限，按钮权限标识等。
- API 管理: 维护系统接口注册信息，辅助权限与接口治理。
- 选项管理: 动态配置枚举。
- 系统配置: 管理各种环境的配置。
- 通知公告: 用户通知消息。
- 任务管理: 管理定时任务，包括执行日志。
- 国际化管理: 管理国际化资源。
- 账号与令牌管理: 支持 OAuth2 绑定、个人令牌等账号安全能力。
- 监控与统计: 支持基础监控信息与统计查询接口。

## 📌 当前阶段说明

- 动态模型与代码生成相关能力当前仍存在于仓库实现中，但不再作为主要产品卖点继续强化。
- 当前阶段的重点是统一产品方向，并围绕治理、运营与 AI 注解协同完善平台能力。
- **架构决策**: 多租户功能已移除，当前为单租户架构，适合单一组织或团队的内部管理系统。

## 🧭 推荐阅读

- [快速开始](/admin/quickly)
- [产品方向调整](/admin/product-direction)
- [当前功能总览](/admin/current-capabilities)
- [权限与组织治理说明](/admin/governance-guide)
- [运营能力说明](/admin/operations-guide)
- [Token 与 OAuth2 联调说明](/admin/token-oauth2-guide)
- [AI 注解协同规范](/admin/ai-annotation-spec)
- [AI 注解产物模板](/admin/ai-annotation-templates)
- [四期路线图](/admin/phase-4-roadmap)
- [五期路线图](/admin/phase-5-roadmap)
- [移动端 H5 适配说明](/admin/mobile-h5-adaptation)
- [HotGo 对比分析](/admin/hotgo-comparison)
- [产品打磨与工程治理方案](/admin/product-polish-governance-plan)
- [首轮整改清单](/admin/product-polish-remediation-round-1)
- [生产部署标准化](/admin/production-standardization)
- [发布验证清单](/admin/release-verification-checklist)
- [性能与可观测性指南](/admin/observability-guide)
- [安全基线指南](/admin/security-baseline)
- [登录排障](/admin/login-troubleshooting)
- [集成测试指南](/admin/integration-test-guide)
- [本地联调](/admin/local-debug)
- [国际化排障](/admin/i18n-troubleshooting)
- [配置操作](/admin/tutorials)
- [Docker 部署](/admin/docker)

## 贡献者

<span style="margin: 0 5px;" ><a href="https://github.com/lwnmengjing" ><img src="https://images.weserv.nl/?url=avatars.githubusercontent.com/u/12806223?s=64&v=4&w=60&fit=cover&mask=circle&maxage=7d" /></a></span> <span style="margin: 0 5px;" ><a href="https://github.com/wangde7" ><img src="https://images.weserv.nl/?url=avatars.githubusercontent.com/u/56955959?s=64&v=4&w=60&fit=cover&mask=circle&maxage=7d" /></a></span>

## 建议反馈

本文档为静态站点，项目托管在`github` [mss-boot-docs](https://github.com/mss-boot-io/mss-boot-docs) 基于 [dumi](https://d.umijs.org/) 开发，
使用`github actions`将编译后的静态文件发布到`cloudflare workers`。

如果您有任何建议或者意见，欢迎提`issue`或者`pr`。

## 📨 互动

<table>
   <tr>
    <td><img src="https://mss-boot-io.github.io/.github/images/wechat.jpg" width="180px"></td>
    <td><img src="https://mss-boot-io.github.io/.github/images/wechat-mp.jpg" width="180px"></td>
    <td><img src="https://mss-boot-io.github.io/.github/images/qq-group.jpg" width="200px"></td>
    <td><a href="https://space.bilibili.com/597294782/channel/seriesdetail?sid=3881026&ctype=0">mss-boot-io</a></td>
  </tr>
  <tr>
    <td>微信</td>
    <td>公众号🔥🔥🔥</td>
    <td><a target="_blank" href="https://shang.qq.com/wpa/qunwpa?idkey=0f2bf59f5f2edec6a4550c364242c0641f870aa328e468c4ee4b7dbfb392627b"><img border="0" src="https://pub.idqqimg.com/wpa/images/group.png" alt="mss-boot技术交流群" title="mss-boot技术交流群"></a></td>
    <td>哔哩哔哩🔥🔥🔥</td>
  </tr>
</table>
