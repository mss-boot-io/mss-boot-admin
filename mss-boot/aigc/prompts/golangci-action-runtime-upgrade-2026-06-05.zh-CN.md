# golangci-lint action runtime 升级记忆

- 日期：2026-06-05
- 背景：`actions/setup-go` 等 action 升级到 v6 后，mss-boot 主干 CI 仍提示 `golangci/golangci-lint-action@v8` 使用 Node.js 20 runtime。
- 处理：将 `golangci/golangci-lint-action` 升级到 `v9`，该版本 action metadata 使用 `node24`。
- 约束：lint job 仍保持 advisory 模式，历史 lint backlog 不阻断主干；真正必须通过的是 `go test`。
- 后续：历史 lint annotations 可交给 GitHub Issues / Copilot Code Review / 后续 PR 批量消化，不应让流水线红灯化。

