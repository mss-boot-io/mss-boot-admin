---
title: Admin Distribution v1.3.0-rc.4 预览版引用
order: 35
---

# Admin Distribution v1.3.0-rc.4 预览版引用

`v1.3.0-rc.4` 是完整 Admin Distribution 的公开预览版，不替代当前
`v1.2.3` 稳定版。Root、Framework、Admin Go Module 与 Admin Web 使用同一个
精确版本，并从同一个已合并到 `main` 的提交发布。

> RC4 现为不可变历史预览记录。Framework、Admin 与前端制品已公开，Root 标签已创建但
> Root GitHub Release 未完成；当前前向修复目标为 [v1.3.0-rc.5](./admin-distribution-v1.3.0-rc.5.zh-CN.md)。

对应制品如下：

| 组件 | 引用 |
| --- | --- |
| Root | `v1.3.0-rc.4` |
| Framework | `mss-boot/v1.3.0-rc.4` |
| Admin Go Module | `admin/v1.3.0-rc.4` |
| Admin Web | GitHub Packages：`@mss-boot-io/admin-web@1.3.0-rc.4` |
| 前端镜像 | `ghcr.io/mss-boot-io/mss-boot-admin-antd-v6:v1.3.0-rc.4` |

## rc.1 状态

`v1.3.0-rc.1` 是不可变的部分发布记录：Framework、Admin 和多架构前端镜像已经发布；
前端包、前端 GitHub Release 与 Root Release 未发布。失败原因是发布工作流用同一个
manifest-list digest 依次校验 amd64 与 arm64，第二次创建容器时被 Docker 拒绝。
修复通过新的 Pull Request 合并，并推进到 `rc.2`；不会移动、覆盖或复用任何 `rc.1` 标签和制品。

## rc.2 状态

`v1.3.0-rc.2` 同样保留为不可变的部分发布记录：Framework、Admin 以及修复后的多架构前端镜像已经发布；
Admin Web 包、前端 GitHub Release 与 Root Release 未发布。镜像身份和双架构容器验证均已通过，随后 npm 拒绝发布预发布
版本，因为命令没有显式指定 dist-tag。修复通过新的 Pull Request 合并，预发布版本固定使用 `next`、正式版本固定使用
`latest`，并推进到 `rc.3`；不会移动、覆盖或复用任何 `rc.2` 标签和制品。

## rc.3 状态

`v1.3.0-rc.3` 继续保留为不可变的部分发布记录：Framework、Admin、多架构前端镜像以及
`@mss-boot-io/admin-web@1.3.0-rc.3` 已从同一个精确提交发布。安全恢复执行证明远端包完整性与镜像摘要
完全一致，既有包和镜像均未被覆盖；但发布后校验使用了 `npm view <package> dist-tags --json`，GitHub
Packages 对该字段查询持续返回空结果，因此前端 GitHub Release 与 Root Release 未发布。`rc.4` 改用 npm
CLI 官方的只读 `npm dist-tag ls <package>` 命令，并进行有界重试与精确版本比对；不会移动、覆盖或复用任何
`rc.3` 标签、包版本或镜像。

## rc.4 状态

`v1.3.0-rc.4` 的 Framework、Admin 与前端 Release 已从同一精确提交公开；Root 标签存在，
但 Root Release 未完成。保留的 Thin Host 验收发现 Core 与 Business 迁移全局排序会让 Core
示例清理迁移删除已生成的 Supplier 菜单，发布测试还会因 workspace 依赖下载改写 `go.work.sum`。
修复通过新的 Pull Request 推进到 `rc.5`，不会移动、覆盖或复用 RC4 的任何标签、包或镜像。

## 后端引用

Thin Host 的 `go.mod` 必须同时精确固定 Admin 与 Framework：

```go
require (
    github.com/mss-boot-io/mss-boot-admin/admin v1.3.0-rc.4
    github.com/mss-boot-io/mss-boot-admin/mss-boot v1.3.0-rc.4
)
```

正式引用测试使用 `GOWORK=off`，且不允许 `replace` 指向 Foundation 本地目录：

```shell
GOWORK=off go mod tidy
GOWORK=off go test ./...
GOWORK=off go vet ./...
GOWORK=off go build ./cmd/server
```

## 前端引用

预览包发布到 GitHub Packages，不发布到 npmjs。GitHub npm Registry 即使读取公开包也要求认证。
生成项目的 `web/.npmrc` 只保留以下环境变量占位符：

```ini
@mss-boot-io:registry=https://npm.pkg.github.com
//npm.pkg.github.com/:_authToken=${NODE_AUTH_TOKEN}
```

本地首次使用时，为 GitHub CLI 增加 `read:packages` 权限，然后只把令牌注入当前安装进程：

```shell
gh auth refresh -h github.com -s read:packages
cd web
NODE_AUTH_TOKEN="$(gh auth token)" corepack pnpm@10.34.5 install --frozen-lockfile
corepack pnpm@10.34.5 run lint
corepack pnpm@10.34.5 run test
corepack pnpm@10.34.5 run build
```

不得把令牌展开后写入项目 `.npmrc`、lockfile、日志或报告。生成的 GitHub Actions 工作流使用
短期 `GITHUB_TOKEN`，并声明 `packages: read`。

GitHub 会把首次发布的 npm 包创建为私有包。发布流程先验证版本、源提交、完整性与仓库绑定，
再由发布管理者把可见性一次性切换为 Public。Root 发布门禁会再次强制校验公开可见性；
该校验通过前不会发布 Root 预览版。

## 生成完整 Thin Host

从与预览版一致的 Foundation checkout 执行：

```shell
go run ./cmd/mss new app preview-admin \
  --module github.com/example/preview-admin \
  --repository example/preview-admin \
  --destination /absolute/path/preview-admin \
  --write --format json
```

生成结果只包含组合胶水与业务代码。随后可放入 AdminModule 规格并运行
`mss module generate`；不得复制 `admin/`、`mss-boot/` 或完整 `web/antd-v6/src`。

## 预览验收边界

完整引用测试必须证明：真实 Go Module 可解析、GitHub Packages 中的真实 npm 包可安装、
前后端测试和构建通过、Supplier 业务模块可用，并在仓库外 Thin Host 上完成登录、菜单、CRUD、
拒绝访问、刷新与控制台健康检查。测试项目应保留并继续运行，供人工检查；预览版发现问题后
发布新的 RC，不移动或覆盖既有标签和包版本。

发布资格决策只绑定 `.mss/release-qualification.json` 选中的
`complete-admin-distribution-thin-host` Feature。Agent CI 在每个相关 PR 中运行阶段证据、
资格决策、readiness attestation 与工作流合同测试；活动版本、Feature 或精确提交绑定发生漂移时，
预发布流程必须在创建任何标签或制品前失败。
