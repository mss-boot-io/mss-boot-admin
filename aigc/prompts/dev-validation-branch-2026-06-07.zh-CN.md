# Dev Validation Branch

日期：2026-06-07

## 背景

本分支 `codex/admin-dev-validation-20260607` 用于 `mss-boot-dev` 后端候选镜像验证，
组合以下修复：

- `mss-boot-admin#371`：query cache 初始化和 table tag 清理。
- `mss-boot#380`：query cache miss 后写入 tag set、create 后清理 table tag。
- `mss-boot-admin#372`：删除模型后清理生成菜单树和 Casbin policy。

## 决策

- 本分支只用于 dev 环境验证，不作为正式合并 PR。
- 为生成 GitHub Container Registry 候选镜像，本分支临时在 `ci.yml` 加入 `codex/**`
  push 触发。
- 正式合并仍以 `#371`、`#372`、`mss-boot#380` 各自 PR 的 review 和 CI 结果为准。

## 验证目标

- 在 `mss-boot-dev` 部署本分支 SHA 镜像。
- 确认 `cache.queryCache: true` 时 create/update/delete 后 get/list 不返回旧缓存。
- 确认模型生成菜单删除后，`/virtual/<path>` 菜单树和相关 policy 被清理。
- 使用本地前端连接 dev 后端做菜单残留冒烟测试，不发布 Cloudflare beta。
