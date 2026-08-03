# 2026-06-05 组织 CI 稳定性复盘

## 背景

多个仓库在开源治理工作流上线后出现 Actions 大面积失败，影响外部访问体验。失败主要集中在 Dependabot PR、Scorecard、Gitee mirror、docs Cloudflare deploy。

## 根因分类

- `mss-boot`: Dependabot 升级 `github.com/casbin/gorm-adapter/v3` 后拉入 Casbin v3 接口，与项目当前 Casbin v2 使用方式不兼容。
- `.github` reusable workflows: PR Guard 和 Docs Drift 对 Dependabot PR 过严，导致依赖升级 PR 缺少人工说明时失败。
- mirror workflow: 使用 shallow checkout 做镜像推送，Gitee 拒绝 shallow update；delete 事件还可能触发远端分支删除风险。
- Scorecard: `publish_results: true` 时不能在 workflow 顶层配置写权限，写权限需要收窄到 Scorecard job。
- `mss-boot-docs`: Cloudflare Wrangler 4 需要 Node 22，但 deploy job 使用 Node 20。
- `mss-boot-admin`: 一次 govulncheck 失败来自 Go proxy 临时 HTTP/2 错误，不是代码问题。
- `mss-boot-admin-antd`: Dependabot PR 触发 Cloudflare alpha 部署，但 Dependabot 无法访问仓库 secret。

## 本轮修复原则

- 真实代码/依赖不兼容直接修代码或锁依赖，不做路径兼容。
- Dependabot 的治理门禁降噪，但不取消核心 CI、CodeQL、构建和安全扫描。
- 前端不高频发布 Cloudflare；Dependabot PR 不触发 Cloudflare alpha 部署，外部 beta/prod 仍按既定流程发布。
- workflow 变更必须伴随 `aigc` 记忆，避免多 git workspace 中上下文丢失。

## 后续建议

- 先合入组织级 reusable workflow 修复，再 rerun 当前 open Dependabot PR 的 Guard/Docs Drift。
- 合入各仓库 Scorecard/mirror/docs deploy 修复后，rerun main 上失败的 Scorecard、mirror、deploy。
- 等新流水线稳定后，再把 required checks 和 branch protection 打开，避免把不稳定门禁硬加到开源协作入口。
