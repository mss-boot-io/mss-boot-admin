# mss-boot-admin-antd

[![CI](https://github.com/mss-boot-io/mss-boot-admin-antd/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/mss-boot-io/mss-boot-admin-antd/actions/workflows/ci.yml) [![CodeQL](https://github.com/mss-boot-io/mss-boot-admin-antd/actions/workflows/codeql.yml/badge.svg?branch=main)](https://github.com/mss-boot-io/mss-boot-admin-antd/actions/workflows/codeql.yml) [![OpenSSF Scorecard](https://github.com/mss-boot-io/mss-boot-admin-antd/actions/workflows/scorecard.yml/badge.svg?branch=main)](https://github.com/mss-boot-io/mss-boot-admin-antd/actions/workflows/scorecard.yml) [![Release](https://img.shields.io/github/v/release/mss-boot-io/mss-boot-admin-antd.svg?style=flat-square)](https://github.com/mss-boot-io/mss-boot-admin-antd/releases) [![License](https://img.shields.io/github/license/mss-boot-io/mss-boot-admin-antd.svg?style=flat-square)](https://github.com/mss-boot-io/mss-boot-admin-antd/blob/main/LICENSE)

[English](./README.md) | 简体中文

## 简介

> `mss-boot-admin-antd` 是 mss-boot admin 平台的前端部分。当前产品主线正在收敛为治理、运营、访问控制、配置能力，以及 AI 注解协同驱动的研发流程，而不再以动态模型或代码生成为核心方向。

## 教程

[在线文档](https://docs.mss-boot-io.top) [视频教程](https://space.bilibili.com/597294782/channel/seriesdetail?sid=3881026)

## 社区

[贡献指南](./CONTRIBUTING.md) [安全策略](./SECURITY.md) [支持渠道](./SUPPORT.md) [适合首次贡献的任务](https://github.com/mss-boot-io/mss-boot-admin-antd/issues?q=is%3Aissue%20is%3Aopen%20label%3A%22good%20first%20issue%22) [AI 记忆](./aigc/prompts/readme.zh-CN.md)

## 项目地址

[后端项目](https://github.com/mss-boot-io/mss-boot-admin) [前端项目](https://github.com/mss-boot-io/mss-boot-admin-antd)

## 🎬 体验环境

[体验地址](https://admin-beta.mss-boot-io.top)

> 账号：admin 密码：123456

## ✨ 特性

- 支持国际化
- 标准 Restful API 开发规范
- 基于 Casbin 的 RBAC 权限管理
- 基于 Gorm 的数据库存储
- 基于 Gin 的中间件开发
- 基于 Gin 的 Swagger 文档生成
- 支持 oauth2.0 第三方登录
- 支持 swagger 文档生成
- 支持多种配置源(本地文件、embed、对象存储 s3 等、gorm 支持的数据库、mongodb)
- 支持数据库迁移
- 支持用户、角色、部门、岗位、菜单、API、配置等治理型后台能力
- 支持通知、任务、监控、统计等运营型能力

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

## 📦 准备工作

- 后端安装 Go 1.26+
- 后端集成依赖可选安装 MySQL 8.0+、Redis 7+
- 前端安装 Node.js 22+、pnpm 9.x

## 📦 快速开始

### 1. 下载项目

```shell
# 下载后端项目
git clone https://github.com/mss-boot-io/mss-boot-admin.git
# 下载前端项目
git clone https://github.com/mss-boot-io/mss-boot-admin-antd.git
```

### 2. 迁移数据库

```shell
# 进入后端项目
cd mss-boot-admin
# 默认本地后端配置使用 SQLite: mss-boot-admin-local.db
go run . migrate
```

如需本地使用 MySQL，请先在后端仓库启动 `compose/mysql/docker-compose.yml`，并修改 `config/application.yml` 后再执行迁移。

### 3. 生成 API 接口信息

```shell
# 生成api接口信息
go run . server -a
```

### 4. 启动后端服务

```shell
# 启动后端服务
go run . server
```

### 5. 启动前端服务

```shell
# 进入前端项目
cd mss-boot-admin-antd
# 安装依赖
corepack enable
pnpm install
# 启动前端服务
pnpm dev
```

## 前端环境矩阵

前端通过 `UMI_ENV` 与 `REACT_APP_ENV` 选择环境配置。`API_URL` 由对应的 `config/config.prod.*.ts` 文件定义，本地开发也会使用 dev proxy。

| 场景 | 命令 | API 目标 | 用途 |
| --- | --- | --- | --- |
| 本地开发 | `pnpm dev` 或 `pnpm start:no-mock` | `/admin/`、`/public/` 通过 dev proxy 转发到 `http://localhost:8080` | 对接本地 `mss-boot-admin` 后端。 |
| 本地构建 | `pnpm build:local` | `http://localhost:8080` | 以生产构建方式验证本地后端。 |
| Alpha | `pnpm start:alpha` / `pnpm build:alpha` | `https://admin-api-alpha.mss-boot-io.top` | 开发后端环境，用于联调验证。 |
| Beta | `pnpm start:beta` / `pnpm build:beta` | `https://admin-api-beta.mss-boot-io.top` | 对外 beta 目标，需先完成本地和 CI 验证。 |
| Production | `pnpm start:prod` / `pnpm build:prod` | `https://admin-api.mss-boot-io.top` | 生产发布构建目标。 |

CI 与 Cloudflare workflow 使用 Node.js 22 和 pnpm 9。Cloudflare alpha、beta、prod 发布均保持手动 `workflow_dispatch`；PR、Dependabot 分支和普通 `codex/**` 审查分支不应自动发布前端。

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

## 💎 贡献者

<span style="margin: 0 5px;" ><a href="https://github.com/lwnmengjing" ><img src="https://images.weserv.nl/?url=avatars.githubusercontent.com/u/12806223?s=64&v=4&w=60&fit=cover&mask=circle&maxage=7d" /></a></span> <span style="margin: 0 5px;" ><a href="https://github.com/wangde7" ><img src="https://images.weserv.nl/?url=avatars.githubusercontent.com/u/56955959?s=64&v=4&w=60&fit=cover&mask=circle&maxage=7d" /></a></span>

## JetBrains 开源证书支持

`mss-boot-io` 项目一直以来都是在 JetBrains 公司旗下的 GoLand 集成开发环境中进行开发，基于 **free JetBrains Open Source license(s)** 正版免费授权，在此表达我的谢意。

<a href="https://www.jetbrains.com/?from=kubeadm-ha" target="_blank"><img src="https://raw.githubusercontent.com/panjf2000/illustrations/master/jetbrains/jetbrains-variant-4.png" width="250" align="middle"/></a>

## 🤝 特别感谢

1. [ant-design](https://github.com/ant-design/ant-design)
2. [ant-design-pro](https://github.com/ant-design/ant-design-pro)
3. [gin](https://github.com/gin-gonic/gin)
4. [casbin](https://github.com/casbin/casbin)
5. [gorm](https://github.com/jinzhu/gorm)
6. [gin-swagger](https://github.com/swaggo/gin-swagger)
7. [jwt-go](https://github.com/dgrijalva/jwt-go)
8. [oauth2](https://pkg.go.dev/golang.org/x/oauth2)

## 🔑 License

[MIT](https://github.com/mss-boot-io/mss-boot-admin-antd/blob/main/LICENSE)

Copyright (c) 2024 mss-boot-io
