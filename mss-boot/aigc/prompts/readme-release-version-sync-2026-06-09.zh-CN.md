# README release version sync

## 背景

- 记录时间：2026-06-09
- 影响文件：`README.md`
- 当前公开 release：`v0.7.3`

## 问题

`mss-boot` GitHub Release 已发布到 `v0.7.3`，但 README 首页仍显示 `Latest Version: v0.7.1`，安装命令也使用 `v0.7.1`。这会让外部读者误判项目维护状态。

## 修复

- 将 README 最新版本更新为 `v0.7.3`。
- 将 highlights 标题改为 `v0.7.x Highlights`，保留历史 v0.7.1 checklist。
- 将 `go get github.com/mss-boot-io/mss-boot@v0.7.1` 更新为 `@v0.7.3`。

