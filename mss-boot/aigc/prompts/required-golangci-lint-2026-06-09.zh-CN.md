# golangci-lint 新增问题门禁记忆

- 日期：2026-06-09
- 仓库：mss-boot
- 关联 Issue：#352

## 背景

早期开源治理时，`golangci-lint` 因历史 backlog 先以 advisory 方式运行，workflow 使用 `continue-on-error: true`，避免未清理完的 lint 债阻塞贡献。

## 当前证据

- 最新 main CI run `27155421788` 的 `lint` job 显示 success，但这是 `continue-on-error` 掩盖后的 job 结果。
- 去掉 advisory 后，PR #383 暴露当前全量 lint 仍有 131 个历史问题。
- main 分支保护已把 `lint` 作为 required check。

## 决策

- PR 场景使用 `golangci/golangci-lint-action@v9` 的 `only-new-issues: true`，阻断新增 lint 债。
- main push 场景继续保留全量 baseline advisory，避免历史 131 个问题导致 main push 全红。
- #352 不关闭，继续作为全量清理 backlog 的长期小任务。
- PR 正文应包含 tests/docs/security/release impact，并说明不会关闭 #352。

## 验证

- 本地执行 `git diff --check`。
- 本地执行 `go test ./...`。
- GitHub PR CI 需要重新验证 `test` 与 `lint` 均为 success；其中 `lint` 应只阻断本 PR 新增问题。
