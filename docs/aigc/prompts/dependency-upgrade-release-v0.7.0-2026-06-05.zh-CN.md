# Dependency Upgrade And v0.7.0 Release

日期：2026-06-05

## 范围

本次处理前后端项目的 Dependabot/deps PR，修复依赖升级暴露的流水线问题，并发布
`v0.7.0`。

## 前端 mss-boot-admin-antd

- 合并 npm 与 GitHub Actions 依赖升级。
- 将前端 GitHub Actions Node 基线从 18 调整到 22，用于兼容
  `cross-env@10` 和 `@cloudflare/kv-asset-handler@0.5.x`。
- 保持 Cloudflare alpha/beta/prod 发布不随 tag 自动触发。
- 发布 `v0.7.0`，GitHub Release 产物包含 `dist.tar.gz` 和
  `dist-local.tar.gz`。

## 后端 mss-boot-admin

- 合并 Go module 与 GitHub Actions 依赖升级，包括 AWS SDK、Kubernetes
  client-go、Consul、Kafka、OAuth2 等依赖。
- AWS SDK 相关 PR 在 `go.mod` / `go.sum` 冲突后，按最新 main 重新执行
  `go get` 与 `go mod tidy`，再交给 GitHub Actions 验证。
- 发布 `v0.7.0`，后端 release workflow 成功下载前端最新
  `dist-local.tar.gz` 并打包到后端发布资产。

## 验证

- 前端 main：CI、CodeQL、OpenSSF Scorecard、GitHub Actions Mirror 均通过。
- 后端 main：CI、CodeQL、govulncheck、Swagger、OpenSSF Scorecard、
  GitHub Actions Mirror 均通过。
- 前端 `v0.7.0` tag：Release、Release Draft、Copilot Setup Steps 均通过。
- 后端 `v0.7.0` tag：Release、CI、Release Draft、Copilot Setup Steps 均通过。

## 经验

- 前端 Cloudflare 发布仍需人工触发；tag release 只负责 GitHub Release 与镜像。
- 后端 release 依赖前端 latest release，后续发布仍应先发前端，再发后端。
- 可以优先使用 Dependabot update branch、PR checks 与 GitHub Actions 验证；
  只有锁文件冲突无法自动解决时，再本地重建分支并强推到对应 Dependabot 分支。
