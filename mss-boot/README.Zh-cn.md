# mss-boot

[![CI](https://github.com/mss-boot-io/mss-boot-admin/actions/workflows/ci.yml/badge.svg)](https://github.com/mss-boot-io/mss-boot-admin/actions/workflows/ci.yml)
[![CodeQL](https://github.com/mss-boot-io/mss-boot-admin/actions/workflows/codeql.yml/badge.svg)](https://github.com/mss-boot-io/mss-boot-admin/actions/workflows/codeql.yml)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](./LICENSE)

[English](./README.md) | 简体中文

`mss-boot` 是 [`mss-boot-admin`](https://github.com/mss-boot-io/mss-boot-admin)
单仓中的可复用 Go 框架模块，提供统一管理的 HTTP、gRPC、定时任务运行时，以及配置、
持久化、存储、安全和请求控制器适配能力。

该目录仍保持独立 Go module，并通过 `GOWORK=off` 验证，避免仓库工作区掩盖依赖缺失。

## 当前状态

- Module 路径：`github.com/mss-boot-io/mss-boot-admin/mss-boot`
- 源码目录：`mss-boot/`
- Go 要求：Go 1.26 或更高版本
- 稳定性：v1 之前；导出 API 属于兼容面，但正确性修复可能收紧过去含糊的行为

开发版本可使用 `main`；生产环境应替换为已发布的子模块标签或不可变提交：

```bash
go get github.com/mss-boot-io/mss-boot-admin/mss-boot@main
```

## 核心能力

- 基于 Context 的 HTTP、gRPC、cron 生命周期管理
- HTTP 运维路由、超时、TLS、指标和优雅关闭
- gRPC 中间件、OpenTelemetry、Prometheus、TLS 和有界优雅关闭
- GORM、MongoDB/MGM、Kubernetes 控制器 Action
- 本地文件、数据库、对象存储、ConfigMap、Consul、AWS AppConfig 等配置源
- 缓存、队列、对象存储、迁移、安全、语言和响应工具

可选集成在未配置时必须返回错误或跳过；框架包不能直接终止宿主进程。

## 运行时契约

被管理组件实现：

```go
type Runnable interface {
    fmt.Stringer
    Start(context.Context) error
}
```

`Start` 必须阻塞，直到组件停止、Context 被取消，或发生不可恢复的运行时错误；返回前必须
释放自己持有的资源。Manager 并发运行组件，首个异常退出会取消同组组件，并在配置的时间内
等待优雅关闭完成。

为兼容现有应用，Manager 默认处理 `SIGINT` 和 `SIGTERM`。自行管理进程信号的应用应使用
`server.WithoutSignalHandling()`，并传入由 `signal.NotifyContext` 创建的 Context。

生命周期、绑定、鉴权、查询和兼容性细节见
[运行时与请求契约](./docs/architecture/runtime-and-request-contracts.md)。

## 快速开始

完整示例位于 [`examples/basic-http/main.go`](./examples/basic-http/main.go)，并会随框架测试一起编译。

```go
package main

import (
    "context"
    "log/slog"
    "net/http"
    "os"
    "os/signal"
    "syscall"

    "github.com/mss-boot-io/mss-boot-admin/mss-boot/core/server"
    "github.com/mss-boot-io/mss-boot-admin/mss-boot/core/server/listener"
)

func main() {
    ctx, stop := signal.NotifyContext(
        context.Background(),
        os.Interrupt,
        syscall.SIGTERM,
    )
    defer stop()

    mux := http.NewServeMux()
    mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
        _, _ = w.Write([]byte("ok\n"))
    })

    manager := server.New(server.WithoutSignalHandling())
    manager.Add(listener.New(
        listener.WithAddr(":8080"),
        listener.WithHandler(mux),
    ))

    if err := manager.Start(ctx); err != nil {
        slog.Error("server stopped unexpectedly", "error", err)
        os.Exit(1)
    }
}
```

## 请求与控制器保证

- 先绑定 URI 和 Query/Form，再按媒体类型最多读取一种 Body，最后只校验一次完整 DTO。
- `GET`、`HEAD`、`OPTIONS` 不读取 JSON、XML、YAML Body。
- GORM、MGM、Kubernetes 和自定义 Action 的 `WithAuth(true)` 均默认拒绝：全局兼容鉴权
  Handler 缺失时返回服务配置错误，不能悄悄变成匿名接口。
- GORM Search 必定应用请求条件和分页；Count 使用同样过滤条件但不带分页。
- Search 反射遇到不支持的 DTO 字段时安全忽略，不再 panic。

## 验证

在仓库根目录执行：

```bash
make deps-framework
make test-framework
```

独立模块、竞态和静态检查：

```bash
cd mss-boot
GOWORK=off go test -race -shuffle=on -count=1 ./...
GOWORK=off go vet ./...
GOWORK=off go mod tidy
git diff --exit-code -- go.mod go.sum
```

依赖可选数据库、对象存储、Kubernetes 集群或云凭证的测试，在未配置时必须主动跳过。
在 CI 真正执行覆盖率阈值前，README 不声明未经证明的覆盖率数字。

## 兼容与发布

框架仍处于 v1 之前。任何改变可观察行为的变更都必须包含测试、文档、安全影响，以及迁移或
回滚说明。子模块发布标签应采用 `mss-boot/vX.Y.Z`。

更多信息见 [CHANGELOG.md](./CHANGELOG.md)、[CONTRIBUTING.md](./CONTRIBUTING.md)
和 [SECURITY.md](./SECURITY.md)。

## License

[MIT](./LICENSE)
