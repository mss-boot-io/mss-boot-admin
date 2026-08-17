---
title: 框架介绍
order: 10
nav:
  title: 指南
  order: 1
---

## 什么是 mss-boot

[mss-boot](https://github.com/mss-boot-io/mss-boot) 是一个企业级服务开发框架，基于 Gin、GORM、Casbin 等成熟组件构建，专注于提供稳定、可维护的后台管理系统开发基础。

## 核心定位

当前产品方向聚焦于：

- **治理型后台系统** - 权限管理、组织管理、访问控制
- **运营型后台系统** - 通知公告、任务管理、监控统计
- **配置管理** - 系统配置、国际化、多环境支持
- **单租户架构** - 适合单一组织或团队的内部管理系统

> Admin 运行时动态模型、虚拟 CRUD 和浏览器代码生成已经移除。开发期仍可使用仓库级 `cmd/mss` 离线确定性生成器，它生成的是可编译、可测试、可审查的模块代码。

## 生态项目

| 项目 | 说明 |
|------|------|
| [mss-boot](https://github.com/mss-boot-io/mss-boot) | 核心框架，提供 HTTP/GRPC 服务开发基础 |
| [mss-boot-admin](https://github.com/mss-boot-io/mss-boot-admin) | 后端、V6 前端、生成器和交付契约的统一事实源 |
| `mss-boot-admin-antd-v6` | 唯一前端与独立发布单元，基于 React 19 + Ant Design 6 + Umi Max |
| [mss-boot-docs](https://github.com/mss-boot-io/mss-boot-docs) | 文档站点，在线文档与教程 |

## 核心特性

### 服务开发

- HTTP 服务基于 Gin 框架
- GRPC 服务基于 grpc-go
- 支持 Swagger 文档生成
- 标准 RESTful API 规范

### 数据与存储

- 数据库存储基于 GORM（MySQL、PostgreSQL 等）
- 支持多配置源（本地文件、embed、对象存储、数据库）
- 自动数据库迁移

### 权限与安全

- RBAC 权限管理基于 Casbin
- JWT 认证与 OAuth2.0 第三方登录
- 个人访问令牌（Personal Access Token）

### 可观测性

- Prometheus 指标暴露
- 健康检查与就绪检查
- pprof 性能分析
- 日志收集与链路追踪

### 国际化

- 多语言支持
- 国际化资源管理
- 前后端国际化协同

## 适用场景

mss-boot 适合以下场景：

- 企业内部管理系统
- 团队协作平台
- 配置型后台系统
- 需要权限治理的运营平台
- 单一组织的后台服务

## 快速开始

- [快速开始](/guide/quickly) - 环境准备与项目启动
- [核心功能](/guide/features) - 了解内置功能模块
- [服务配置](/guide/config) - 配置文件说明
- [部署指南](/guide/deployment) - 生产环境部署

## 贡献者

<span style="margin: 0 5px;" ><a href="https://github.com/lwnmengjing" ><img src="https://images.weserv.nl/?url=avatars.githubusercontent.com/u/12806223?s=64&v=4&w=60&fit=cover&mask=circle&maxage=7d" /></a></span>
<span style="margin: 0 5px;" ><a href="https://github.com/wangde7" ><img src="https://images.weserv.nl/?url=avatars.githubusercontent.com/u/56955959?s=64&v=4&w=60&fit=cover&mask=circle&maxage=7d" /></a></span>
<span style="margin: 0 5px;" ><a href="https://github.com/wxip" ><img src="https://images.weserv.nl/?url=avatars.githubusercontent.com/u/25923931?s=64&v=4&w=60&fit=cover&mask=circle&maxage=7d" /></a></span>

## 获取帮助

- [在线文档](https://docs.mss-boot-io.top)
- [GitHub Issues](https://github.com/mss-boot-io/mss-boot/issues)
- [视频教程](https://space.bilibili.com/597294782/channel/seriesdetail?sid=3881026)
