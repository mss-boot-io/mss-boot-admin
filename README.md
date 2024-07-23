# mss-boot-admin

[![Build Status](https://github.com/mss-boot-io/mss-boot-admin/workflows/CI/badge.svg)](https://github.com/mss-boot-io/mss-boot-admin)
[![Release](https://img.shields.io/github/v/release/mss-boot-io/mss-boot-admin.svg?style=flat-square)](https://github.com/mss-boot-io/mss-boot-admin/releases)
[![License](https://img.shields.io/github/license/mashape/apistatus.svg)](https://github.com/mss-boot-io/mss-boot-admin)

English | [简体中文](./README.zh-CN.md)

## Introduction
> Based on Gin + React + Atn Design v5 + Umi v4 + mss-boot's front and back end separation permission management system, the system initialization only needs an environment variable to start the system,
> The system supports multiple configuration sources, and the migration command can make the initialization database information simpler. The service command can easily start the service.

## Tutorial
[Online documentation](https://docs.mss-boot-io.top)
[Video tutorial](https://space.bilibili.com/597294782/channel/seriesdetail?sid=3881026)

## Project address
[Backend project](https://github.com/mss-boot-io/mss-boot-admin)
[Front-end project](https://github.com/mss-boot-io/mss-boot-admin-antd)

## 🎬 Experience environment
[Experience address](https://admin-beta.mss-boot-io.top)
> Account: admin Password: 123456

## ✨ Features
- Support internationalization
- Standard Restful API development specifications
- RBAC permission management based on Casbin
- Database storage based on Gorm
- Middleware development based on Gin
- Swagger document generation based on Gin
- Support oauth2.0 third-party login
- Support swagger document generation
- Support multiple configuration sources (local files, embed, object storage s3, etc., databases supported by gorm, mongodb)
- Support virtual model (dynamic configuration supports front-end and back-end functions)
- Support database migration
- Support microservice code generation

## 📦 Built-in functions
- User management: Users are system operators, and this function mainly completes the configuration of system users.
- Role management: Role menu permission allocation, set role data range permission division by organization.
- Menu management: Configure system menus, operation permissions, button permission identifiers, etc.
- Option management: dynamically configure enumerations.
- Model management: Manage virtual models.
- System configuration: Manage the configuration of various environments.
- Notice announcement: user notification message.
- Task management: Manage scheduled tasks, including execution logs.
- Internationalization management: Manage internationalization resources.
- Microservice code generation: Generate microservice code based on templates.

## 📦 Preparation
- Install golang1.21+
- Install mysql8.0+
- Install nodejs18.16.0+

## 📦 Quick start
### 1. Download the project
```shell
# Download the backend project
git clone https://github.com/mss-boot-io/mss-boot-admin.git
# Download the front-end project
git clone https://github.com/mss-boot-io/mss-boot-admin-antd.git
```

### 2. Migrate the database
```shell
# Enter the backend project
cd mss-boot-admin
# Configure database connection information (can be modified according to actual situation)
export DB_DSN="root:123456@tcp(127.0.0.1:3306)/mss-boot-admin-local?charset=utf8mb4&parseTime=True&loc=Local"
# Migrate the database
go run main.go migrate
```

### 3. Generate API interface information
```shell
# Generate API interface information
go run main.go server -a
```

### 4. Start the backend service
```shell
# Start the backend service
go run main.go server
```

### 5. Start the front-end service
```shell
# Enter the front-end project
cd mss-boot-admin-antd
# Install dependencies
npm install
# Start the front-end service
npm run start
```

## 📨 Interaction
<table>
   <tr>
    <td><a href="https://t.me/+318z6NULrw81N2E1" target="_blank"><img src="https://th.bing.com/th/id/OIP.lYN2s7Dv1a4pLAVUaXMCVgAAAA?rs=1&pid=ImgDetMain" width="180px"></a></td>
    <td><img src="https://mss-boot-io.github.io/.github/images/wechat.jpg" width="180px"></td>
    <td><img src="https://mss-boot-io.github.io/.github/images/wechat-mp.jpg" width="180px"></td>
    <td><img src="https://mss-boot-io.github.io/.github/images/qq-group.jpg" width="200px"></td>
    <td><a href="https://space.bilibili.com/597294782/channel/seriesdetail?sid=3881026&ctype=0">mss-boot-io</a></td>
  </tr>
  <tr>
    <td>telegram🔥🔥🔥</td>
    <td>WeChat</td>
    <td>WeChat MP🔥🔥🔥</td>
    <td><a target="_blank" href="https://shang.qq.com/wpa/qunwpa?idkey=0f2bf59f5f2edec6a4550c364242c0641f870aa328e468c4ee4b7dbfb392627b"><img border="0" src="https://pub.idqqimg.com/wpa/images/group.png" alt="mss-boot技术交流群" title="mss-boot技术交流群"></a></td>
    <td>bilibili🔥🔥🔥</td>
  </tr>
</table>

## 💎 Contributors

<span style="margin: 0 5px;" ><a href="https://github.com/lwnmengjing" ><img src="https://images.weserv.nl/?url=avatars.githubusercontent.com/u/12806223?s=64&v=4&w=60&fit=cover&mask=circle&maxage=7d" /></a></span>
<span style="margin: 0 5px;" ><a href="https://github.com/wangde7" ><img src="https://images.weserv.nl/?url=avatars.githubusercontent.com/u/56955959?s=64&v=4&w=60&fit=cover&mask=circle&maxage=7d" /></a></span>
<span style="margin: 0 5px;" ><a href="https://github.com/mss-boot" ><img src="https://images.weserv.nl/?url=avatars.githubusercontent.com/u/109259065?s=64&v=4&w=60&fit=cover&mask=circle&maxage=7d" /></a></span>
<span style="margin: 0 5px;" ><a href="https://github.com/wxip" ><img src="https://images.weserv.nl/?url=avatars.githubusercontent.com/u/25923931?s=64&v=4&w=60&fit=cover&mask=circle&maxage=7d" /></a></span>

## JetBrains open source certificate support

The `mss-boot-io` project has always been developed in the GoLand integrated development environment under JetBrains, based on the **free JetBrains Open Source license(s)** genuine free license. I would like to express my gratitude.

<a href="https://www.jetbrains.com/?from=kubeadm-ha" target="_blank"><img src="https://raw.githubusercontent.com/panjf2000/illustrations/master/jetbrains/jetbrains-variant-4.png" width="250" align="middle"/></a>

## 🤝 Special thanks

1. [ant-design](https://github.com/ant-design/ant-design)
2. [ant-design-pro](https://github.com/ant-design/ant-design-pro)
3. [umi](https://umijs.org)
4. [gin](https://github.com/gin-gonic/gin)
5. [casbin](https://github.com/casbin/casbin)
6. [gorm](https://github.com/jinzhu/gorm)
7. [gin-swagger](https://github.com/swaggo/gin-swagger)
8. [jwt-go](https://github.com/dgrijalva/jwt-go)
9. [oauth2](https://pkg.go.dev/golang.org/x/oauth2)

## 🤟 Sponsor Us

If you think this project helped you, you can buy a glass of juice for the author to show encouragement 🍹

<img class="no-margin" src="https://mss-boot-io.github.io/.github/images/sponsor-us.jpg"  height="400px"  alt="Sponsor Us">

## 🔑 License

[MIT](https://github.com/mss-boot-io/mss-boot-admin/blob/main/LICENSE)

Copyright (c) 2024 mss-boot-io