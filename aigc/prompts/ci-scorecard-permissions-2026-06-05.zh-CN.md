# 2026-06-05 mss-boot-admin CI 失败复盘

## 现象

`OpenSSF Scorecard` 在发布结果阶段失败，原因是 workflow 顶层声明了写权限，Scorecard webapp 校验拒绝。`govulncheck` 曾出现一次 Go proxy HTTP/2 `INTERNAL_ERROR`，后续同类检查通过，判断为外部临时故障。

## 处理

- 将 workflow 顶层权限改为 `permissions: read-all`。
- 只在 `scorecard` job 上声明 `id-token: write` 和 `security-events: write`。

## 验证

- `go run github.com/rhysd/actionlint/cmd/actionlint@latest`
- `git diff --check`

## 后续

如果 govulncheck 下载依赖的临时失败反复出现，再统一给 Go module download 或 govulncheck 加 retry；当前先通过 rerun 处理一次性网络抖动。
