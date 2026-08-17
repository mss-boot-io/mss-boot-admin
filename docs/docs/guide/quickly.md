---
title: 快速开始
order: 11
nav:
  title: 指南
  order: 1
keywords: [quickstart, installation, setup]
---

# 环境准备

## 必要环境

### 1. 安装 Go 1.26.6

访问 [Go 官网](https://go.dev/dl/) 下载并安装。

验证安装：

```bash
go version
# 输出应包含: go1.26.6
```

### 2. 准备数据库（默认 SQLite）

本地开发默认使用 SQLite，无需先安装或启动数据库服务。运行迁移时会按当前
配置初始化本地数据库。

MySQL 8.0+ 和 PostgreSQL 是可选集成目标。切换数据库时应通过本地环境或
部署平台的 Secret 机制注入 DSN；不要把生产用户名、密码或完整 DSN 写入
文档、命令历史或仓库配置。

### 3. 安装 Node.js >= 24 且 < 25（默认 V6 前端）

访问 [Node.js 官网](https://nodejs.org/) 下载并安装。

验证安装：

```bash
node -v
# 输出应为 24.x
```

通过 Corepack 使用仓库固定的 pnpm 10.34.5：

```bash
corepack enable
corepack pnpm@10.34.5 --version
# 输出: 10.34.5
```

以上版本和目录约定以仓库根目录的 `.mss/project.yaml` 为准。

# 获取项目

## 下载代码

```bash
# 下载单一仓库；后端和前端已合并在同一仓库
git clone https://github.com/mss-boot-io/mss-boot-admin.git
cd mss-boot-admin
```

# 启动后端

## 1. 确认数据库配置

进入后端项目目录：

```bash
cd admin
```

默认 SQLite 可直接继续下一步。如需使用 MySQL/PostgreSQL，请先创建目标数据库，
再通过安全的本地环境配置注入 `DB_DSN`；不要在终端输出或提交完整连接串。

## 2. 执行数据库迁移

```bash
# 创建数据库表结构和初始数据
go run . migrate
```

输出示例：

```
2024/xx/xx xx:xx:xx Migration completed successfully
```

## 3. 启动服务

```bash
# 启动后端服务（端口 8080）
go run . server
```

输出示例：

```
2024/xx/xx xx:xx:xx Starting server on :8080
2024/xx/xx xx:xx:xx Server is ready
```

验证服务：

```bash
curl http://localhost:8080/healthz
# 输出: {"status":"ok"}
```

# 启动前端

## 1. 安装依赖

进入前端项目目录：

```bash
cd ../web/antd-v6
```

按仓库 lockfile 安装依赖：

```bash
corepack pnpm@10.34.5 install --frozen-lockfile
```

## 2. 启动开发服务器

```bash
corepack pnpm@10.34.5 start:dev
```

输出示例：

```
√ Compiled successfully!
  App running at:
  - Local:   http://localhost:8001
  - Network: http://192.168.x.x:8001
```

## 3. 访问系统

浏览器访问：http://localhost:8001

使用初始化流程或部署者提供的本地开发凭据登录。生产环境必须使用独立的
Secret 管理，不要在文档、工单或仓库中记录密码。

# 开发模式启动

## 后端热重载

推荐使用 [air](https://github.com/cosmtrek/air) 实现热重载：

```bash
# 安装 air
go install github.com/cosmtrek/air@latest

# 在 admin/ 目录下运行
air
```

配置 `.air.toml`（示例）：

```toml
root = "."
tmp_dir = "tmp"

[build]
cmd = "go build -o ./tmp/main ."
bin = "./tmp/main"
full_bin = "./tmp/main server"
include_ext = ["go", "tpl", "tmpl", "html", "yml", "yaml"]
exclude_dir = ["assets", "tmp", "vendor"]
delay = 1000
```

## 前端热重载

Umi 已内置热重载，修改代码后自动刷新页面。

# 使用模板项目

如需创建新的服务项目，可使用模板：

## HTTP 服务模板

```bash
# 使用 service-http 模板
git clone https://github.com/mss-boot-io/service-http.git my-service
cd my-service

# 修改项目名称和配置
# 运行迁移
go run . migrate

# 启动服务
go run . server
```

## GRPC 服务模板

```bash
# 使用 service-grpc 模板
git clone https://github.com/mss-boot-io/service-grpc.git my-grpc-service
cd my-grpc-service

# 安装 protoc 和 Go plugins（如需生成 pb 文件）
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

# 生成 pb.go 文件
protoc --go_out=. --go-grpc_out=. proto/*.proto

# 运行迁移
go run . migrate

# 启动服务
go run . server
```

# 常见问题

## 端口冲突

后端默认端口 8080，V6 前端默认端口 8001。如遇冲突：

后端修改 `config/application-local.yml`：

```yaml
server:
  addr: ":9080"
```

前端修改 `.env.local`：

```
PORT=9000
```

## 数据库连接失败

检查：

1. 默认 SQLite 文件所在目录是否可写
2. 如果已切换 MySQL/PostgreSQL，对应服务是否启动且目标数据库是否存在
3. `DB_DSN` 是否通过安全环境正确注入（不要输出完整值）
4. 外部数据库的网络和账号权限是否满足迁移要求

## 前端依赖安装失败

尝试：

```bash
# 在 web/antd-v6/ 中清理依赖缓存，再按 lockfile 重试
corepack pnpm@10.34.5 store prune
corepack pnpm@10.34.5 install --frozen-lockfile
```

不要删除 `pnpm-lock.yaml`，也不要改用其他包管理器绕过冻结安装。若仍失败，
检查 Node.js 版本、网络和 lockfile 是否与当前提交一致，并保留完整错误日志。

# 下一步

- [核心功能](/guide/features) - 了解系统内置功能
- [服务配置](/guide/config) - 详细配置说明
- [部署指南](/guide/deployment) - 生产环境部署
