# mss-boot-admin-antd release upsert 工作流记录

## 背景

- 记录时间：2026-06-09
- 影响文件：`.github/workflows/release.yml`
- 触发场景：推送 `v*.*.*` tag 后构建前端产物、发布 GitHub Release、推送 GHCR 镜像。

## 问题

前端 release workflow 使用 `softprops/action-gh-release` 直接创建 release。若 tag workflow 被重跑，或 release 已经存在，可能出现和后端相同的 `already_exists` 失败，导致公开流水线红灯。

## 修复

- 改为 `gh release view` 判断 release 是否已存在。
- 已存在时执行 `gh release edit`，更新标题、notes、draft、prerelease 状态。
- 不存在时执行 `gh release create --verify-tag --notes-file`，保留自定义镜像拉取说明。
- release step 设置 `GH_REPO=${{ github.repository }}`，避免 `gh` 依赖当前 git remote 推断仓库。
- 统一在 release 存在后执行 `gh release upload --clobber` 上传 `dist.tar.gz` 与 `dist-local.tar.gz`。
- release notes 中的镜像拉取命令使用 `github.ref_name` 作为 tag。

## 验证

- workflow YAML 通过 `git diff --check`。
- 后续新 tag 或重复触发 tag workflow 时，GitHub Release 产物应可被覆盖更新，不再因同 tag release 已存在而红灯。
