# mss-boot v0.7.2 And Admin Dependency Follow-up

日期：2026-06-05

## 范围

本次处理 `mss-boot` 仓库中 deps 类型 PR，发布 `mss-boot v0.7.2`，随后让
`mss-boot-admin` 升级到该版本，并修复升级过程中暴露的 CI 稳定性问题。

## mss-boot

- 合并 Dependabot/deps PR：
  - `golang.org/x/crypto` 升级到 `v0.52.0`。
  - `github.com/hashicorp/consul/api` 升级到 `v1.34.3`。
  - `github.com/aws/aws-sdk-go-v2/config` 升级到 `v1.32.23`。
  - `github.com/aws/aws-sdk-go-v2/service/s3` 升级到 `v1.103.2`。
  - `go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc`
    升级到 `v0.69.0`。
  - `softprops/action-gh-release` 升级到 `v3`。
- AWS S3 与 otelgrpc PR 在合并其它依赖后出现 `go.mod` / `go.sum` 冲突，
  处理方式是基于最新 `origin/main` 重建 Dependabot 分支，重新执行
  `go get` 与 `go mod tidy`，再强推回对应 Dependabot 分支交给 PR 流水线验证。
- `main` 全部检查通过后，打 tag 并发布 `v0.7.2`。

## mss-boot-admin

- 新建 `codex/update-mss-boot-v0.7.2-20260605` 分支，将
  `github.com/mss-boot-io/mss-boot` 从 `v0.7.1` 升级到 `v0.7.2`。
- `go mod tidy` 同步带入间接依赖升级，包括 grpc、OIDC、Consul、pgx、
  quic-go、OpenTelemetry 等。
- PR CI 暴露出 `pkg/gist_test.go` 会访问真实 GitHub Gist API，遇到
  GitHub API rate limit 时会导致流水线失败。
- 修复方式：
  - 在 `pkg/gist.go` 中抽出 `gistClone(ctx, client, id, dir)`，让测试可以注入
    mock GitHub client。
  - 移除 `log.Fatal`，改为按错误返回，避免库函数在异常时直接终止进程。
  - 在 `pkg/gist_test.go` 中使用 `httptest.NewServer` 模拟 Gist API，不再依赖
    外网 GitHub API 或公开 gist id。
- PR 验证通过后合并到 `main`。

## 验收口径

- `mss-boot`：Dependabot/deps PR 关闭且 `main` 全绿，`v0.7.2` release workflow
  通过。
- `mss-boot-admin`：升级 PR 合并到 `main`，CI、CodeQL、govulncheck、Swagger、
  OpenSSF Scorecard、GitHub Actions Mirror 等主干检查通过。
- 两个仓库均不保留打开状态的 Dependabot/deps PR。

## 经验

- deps PR 可以优先交给 Dependabot branch 和 GitHub Actions 自验证；当锁文件冲突
  无法自动解决时，再本地基于最新主干重建分支。
- 测试不应依赖 GitHub 等外部实时 API。开源项目的 CI 要优先使用本地 mock、
  `httptest` 或录制响应，避免受 rate limit、网络、外部服务状态影响。
- 依赖库函数不应使用 `log.Fatal` 终止进程；应返回错误交给调用方处理。
