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

### 1. 安装 Go 1.21+

访问 [Go 官网](https://go.dev/dl/) 下载并安装。

验证安装：

```bash
go version
# 输出: go version go1.21.x linux/amd64
```

### 2. 安装 MySQL 8.0+

使用 Docker 快速启动：

```bash
# 启动 MySQL 容器
docker run -d \
  --name mysql \
  -p 3306:3306 \
  -e MYSQL_ROOT_PASSWORD=123456 \
  mysql:8

# 创建数据库
docker exec -it mysql mysql -uroot -p123456 \
  -e "CREATE DATABASE mss_boot_admin DEFAULT CHARSET utf8mb4 COLLATE utf8mb4_unicode_ci;"
```

或使用本地 MySQL：

```sql
CREATE DATABASE mss_boot_admin DEFAULT CHARSET utf8mb4 COLLATE utf8mb4_unicode_ci;
```

### 3. 安装 Node.js 18.16.0+（前端）

访问 [Node.js 官网](https://nodejs.org/) 下载并安装。

验证安装：

```bash
node -v
# 输出: v18.x.x 或更高
npm -v
# 输出: 9.x.x 或更高
```

推荐使用 pnpm：

```bash
npm install -g pnpm
pnpm -v
```

# 获取项目

## 下载代码

```bash
# 下载后端项目
git clone https://github.com/mss-boot-io/mss-boot-admin.git

# 下载前端项目
git clone https://github.com/mss-boot-io/mss-boot-admin-antd.git
```

# 启动后端

## 1. 配置数据库连接

进入后端项目目录：

```bash
cd mss-boot-admin
```

设置数据库连接（环境变量方式）：

```bash
export DB_DSN="root:123456@tcp(127.0.0.1:3306)/mss_boot_admin?charset=utf8mb4&parseTime=True&loc=Local"
```

或修改 `config/application-local.yml`：

```yaml
database:
  dsn: "root:123456@tcp(127.0.0.1:3306)/mss_boot_admin?charset=utf8mb4&parseTime=True&loc=Local"
```

## 2. 执行数据库迁移

```bash
# 创建数据库表结构和初始数据
go run main.go migrate
```

输出示例：

```
2024/xx/xx xx:xx:xx Migration completed successfully
```

## 3. 启动服务

```bash
# 启动后端服务（端口 8080）
go run main.go server
```

输出示例：

```
2024/xx/xx xx:xx:xx Starting server on :8080
2024/xx/xx xx:xx:xx Server is ready
```

验证服务：

```bash
curl http://localhost:8080/api/v1/health
# 输出: {"status":"ok"}
```

# 启动前端

## 1. 安装依赖

进入前端项目目录：

```bash
cd mss-boot-admin-antd
```

使用 pnpm 安装依赖：

```bash
pnpm install
```

## 2. 启动开发服务器

```bash
pnpm dev
```

输出示例：

```
√ Compiled successfully!
  App running at:
  - Local:   http://localhost:8000
  - Network: http://192.168.x.x:8000
```

## 3. 访问系统

浏览器访问：http://localhost:8000

默认登录账号：
- 用户名：`admin`
- 密码：`123456`

# 开发模式启动

## 后端热重载

推荐使用 [air](https://github.com/cosmtrek/air) 实现热重载：

```bash
# 安装 air
go install github.com/cosmtrek/air@latest

# 在 mss-boot-admin 目录下运行
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
go run main.go migrate

# 启动服务
go run main.go server
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
go run main.go migrate

# 启动服务
go run main.go server
```

# 常见问题

## 端口冲突

后端默认端口 8080，前端默认端口 8000。如遇冲突：

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

1. MySQL 服务是否启动：`docker ps | grep mysql`
2. 数据库是否创建：`mysql -uroot -p123456 -e "show databases;"`
3. DSN 配置是否正确
4. 防火墙是否允许 3306 端口

## 前端依赖安装失败

尝试：

```bash
# 清理缓存
pnpm store prune

# 删除 node_modules 和 lockfile
rm -rf node_modules pnpm-lock.yaml

# 重新安装
pnpm install
```

# 下一步

- [核心功能](/guide/features) - 了解系统内置功能
- [服务配置](/guide/config) - 详细配置说明
- [部署指南](/guide/deployment) - 生产环境部署