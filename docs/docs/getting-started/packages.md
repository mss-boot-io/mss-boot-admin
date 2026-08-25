---
title: v1.3.3 包与导入边界
order: 2
description: Complete Admin Distribution 的 Go Module、npm 包、组合边界与公共解析验证
keywords: [v1.3.3 go module npm admin web import thin host]
---

# v1.3.3 包与导入边界

Thin Host 只引用公开制品，不复制 Foundation 代码。三个包使用同一个协调版本，但保持
各自发布身份。

## 普通应用只导入 Admin

```sh
go get github.com/mss-boot-io/mss-boot-admin/admin@v1.3.3
```

`admin` 是完整后端组合入口，并传递解析同版本 Framework。普通 Thin Host 不需要再把
`mss-boot` 写成直接依赖；生成器会固定经过发布验证的完整依赖图。

只有开发不依赖 Admin 业务面的通用基础设施扩展时，才直接导入 Framework：

```sh
go get github.com/mss-boot-io/mss-boot-admin/mss-boot@v1.3.3
```

不要为了“版本看起来一致”同时手工添加两个直接依赖；以代码真实导入边界和
`go mod tidy` 结果为准。

验证公共解析时关闭 workspace：

```sh
GOWORK=off go mod download
GOWORK=off go test ./...
GOWORK=off go build ./cmd/server
```

PowerShell 要在当前进程中临时关闭 workspace，并在完成后恢复原值：

```powershell
$previousGowork = $env:GOWORK
try {
  $env:GOWORK = 'off'
  go mod download
  go test ./...
  go build ./cmd/server
} finally {
  if ($null -eq $previousGowork) {
    Remove-Item Env:GOWORK -ErrorAction SilentlyContinue
  } else {
    $env:GOWORK = $previousGowork
  }
}
```

下游 `go.mod` 不应提交指向 Foundation 源码的 `replace`。

## Admin Web

```sh
corepack pnpm@10.34.5 add --save-exact @mss-boot-io/admin-web@1.3.3
```

Thin Host 必须提交由 pnpm 10.34.5 生成的冻结锁。只使用包声明的导出：

- `@mss-boot-io/admin-web` 与 `/runtime`：完整运行时；
- `/business`：业务菜单和路由注册；
- `/preset`：受支持的 Umi 预设；
- `/styles`：公共样式入口；
- `/testing`：公开测试辅助。

不要导入 `src/*`、复制核心页面、改写别名指向 Foundation，或创建第二个 SPA。

## 组合规则

```text
业务后端模块 ──编译期注册──> Admin ──依赖──> mss-boot
业务前端路由 ──显式注册──> Admin Web 完整应用
```

- 后端模块通过 `admin/business` 注册迁移、受保护路由与就绪检查；
- 前端业务路由必须与菜单投影一起注册，并位于最终 403/404 回退之前；
- 浏览器权限只改善体验，后端授权始终权威；
- 业务仓库只拥有业务模块和组合胶水。

## 版本完整性

`.mss/project.yaml`、`.mss/lock.yaml`、`go.mod`、`web/package.json` 与冻结锁必须
一致指向 v1.3.3。不要混用不同补丁版本，也不要用分支、PR 提交或本地替换代替公共包。

`mss doctor --strict` 检查本地合同，`mss verify --all` 执行完整验证。
