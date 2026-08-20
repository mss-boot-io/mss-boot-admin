---
title: Admin Distribution v1.3.0-rc.5 预览版引用
order: 34
---

# Admin Distribution v1.3.0-rc.5 预览版引用

`v1.3.0-rc.5` 是完整 Admin Distribution 的公开预览版，不替代当前
`v1.2.3` 稳定版。Root、Framework、Admin Go Module 与 Admin Web 必须使用同一精确版本，
并从同一个已合并到 `main` 的提交发布。

| 组件 | 引用 |
| --- | --- |
| Root | `v1.3.0-rc.5` |
| Framework | `mss-boot/v1.3.0-rc.5` |
| Admin Go Module | `admin/v1.3.0-rc.5` |
| Admin Web | GitHub Packages：`@mss-boot-io/admin-web@1.3.0-rc.5` |
| 前端镜像 | `ghcr.io/mss-boot-io/mss-boot-admin-antd-v6:v1.3.0-rc.5` |

## RC4 前向修复

`v1.3.0-rc.4` 的 Framework、Admin 和前端制品已经公开且保持不可变；Root 标签已经创建，
但 Root GitHub Release 未完成。保留的仓库外 Thin Host 验收发现两个问题：

- Core 与 Business 迁移按全局数字 ID 排序，导致 Core 的示例 Supplier 清理迁移在业务授权种子之后执行，
  删除“采购管理 → 供应商管理”菜单及对应策略；
- Root Release 的依赖下载以 workspace 模式改写 `go.work.sum`，随后 Blueprint 评估按安全规则拒绝脏检出目录。

RC5 固定执行 Core 后 Business 两阶段迁移，并包含一次幂等前向修复：只在识别到 RC4 冲突历史时清除
Supplier 授权迁移的旧账本标记，再由业务阶段重建菜单、API 清单、Casbin 策略、修订号和必要角色。
数据表和供应商业务数据不会被删除。发布流程同时改为独立 Admin 模块依赖下载，并在评估前检查 Git 差异。

## 后端引用

Thin Host 的 `go.mod` 必须同时精确固定 Admin 与 Framework：

```go
require (
    github.com/mss-boot-io/mss-boot-admin/admin v1.3.0-rc.5
    github.com/mss-boot-io/mss-boot-admin/mss-boot v1.3.0-rc.5
)
```

仓库外验收必须关闭本地 workspace，并且不得用 `replace` 指向 Foundation：

```shell
GOWORK=off go mod tidy
GOWORK=off go test ./...
GOWORK=off go vet ./...
GOWORK=off go build ./cmd/server
```

## 前端引用

预览包发布到 GitHub Packages，不发布到 npmjs。生成项目的 `web/.npmrc` 只保留令牌占位符：

```ini
@mss-boot-io:registry=https://npm.pkg.github.com
//npm.pkg.github.com/:_authToken=${NODE_AUTH_TOKEN}
```

GitHub Packages 即使读取公开 npm 包也要求认证。令牌只注入当前命令，不能写入 `.npmrc`、lockfile、
日志或报告：

```shell
gh auth refresh -h github.com -s read:packages
cd web
NODE_AUTH_TOKEN="$(gh auth token)" corepack pnpm@10.34.5 install --frozen-lockfile
corepack pnpm@10.34.5 run lint
corepack pnpm@10.34.5 run test
corepack pnpm@10.34.5 run build
```

## 生成和验收 Thin Host

从与 RC5 一致的 Foundation checkout 执行：

```shell
go run ./cmd/mss new app preview-admin \
  --module github.com/example/preview-admin \
  --repository example/preview-admin \
  --destination /absolute/path/preview-admin \
  --write --format json
```

完整引用测试必须覆盖真实 Go Module 解析、GitHub Packages tarball 安装、lint、单测、构建和浏览器行为。
Supplier 浏览器用例不能直接打开 `/suppliers` 代替导航：它必须先确认授权菜单接口返回该路径，再从侧边栏
展开“采购管理”并点击“供应商管理”，随后完成 CRUD、拒绝访问、刷新和控制台健康检查。

若从 RC4 预览数据库升级，先备份数据库，再执行 RC5 `migrate`，最后启动服务并重新登录以刷新当前授权菜单。
回滚只回退应用版本和 Thin Host 锁定版本；保留 Supplier 表和业务数据，通过后续前向迁移修复，不执行破坏性降级。
