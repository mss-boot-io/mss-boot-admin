# Frontend Node 22 Dependency Baseline

日期：2026-06-05

## 背景

前端依赖升级进入 Node 20+ / Node 22+ 运行时区间：

- `cross-env@10` 要求 Node 20+。
- `@cloudflare/kv-asset-handler@0.5.x` 在 `workers-site` 中要求 Node 22+。
- GitHub Actions 组升级后，部分 action 运行时也开始面向 Node 24。

## 已落地

- 将前端 CI、release、Cloudflare reusable workflow、Copilot setup workflow
  的 `actions/setup-node` 版本从 Node 18 调整为 Node 22。
- Cloudflare alpha/beta/prod 发布触发规则保持不变，不因为该基线调整自动发布。
- 前端 release 仍只负责 GitHub Release 和镜像构建，不触发 Cloudflare 发布。

## 约束

- `admin-beta` Cloudflare 环境仍必须在本地开发、CI 通过、功能可对外展示后再人工发布。
- tag 发布前端版本时不得自动触发 Cloudflare prod 发布。
- 后续新增 Node 版本相关依赖时，需要优先让 GitHub Actions 基线与依赖引擎要求一致。
