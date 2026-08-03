# mss-boot release upsert 工作流记录

## 背景

- 记录时间：2026-06-09
- 影响文件：`.github/workflows/release.yml`
- 触发场景：推送 `v*.*.*` tag 自动发布 GitHub Release。

## 问题

`v0.7.3` 的 Release 实际已经创建成功，但 workflow 最后因重复创建同 tag release 报红：

```text
Validation Failed: {"resource":"Release","code":"already_exists","field":"tag_name"}
```

这会让公开 Actions 页面看起来像版本发布失败，影响开源项目可信度。

## 修复

- 不再直接使用 `softprops/action-gh-release` 创建 release。
- 改为 `gh release view` 判断 release 是否已存在。
- 已存在时执行 `gh release edit`，确保标题、draft、prerelease 状态正确。
- 不存在时执行 `gh release create --verify-tag --generate-notes`。
- release step 设置 `GH_REPO=${{ github.repository }}`，避免未 checkout 时 `gh` 无法定位仓库或误用其它 remote。

## 验证

- workflow YAML 通过 `git diff --check`。
- 后续新 tag 或重复触发 tag workflow 时，release 创建应具备幂等性。
