# Docker action runtime 升级记忆

- 日期：2026-06-05
- 背景：mss-boot-admin 主干 CI 暴露 Docker 官方 actions 的 Node.js 20 runtime 注解；mss-boot-admin-antd 的 release workflow 使用同一批旧 Docker actions，需要提前同步升级，避免 tag 发布时出现相同提示或未来失败。
- 处理：将 `docker/login-action` 升级到 `v4`，`docker/setup-buildx-action` 升级到 `v4`，`docker/metadata-action` 升级到 `v6`，`docker/build-push-action` 升级到 `v7`。
- 发布约束：前端 Cloudflare alpha/beta 发布仍保持手动 `workflow_dispatch`；本次只改 Docker release workflow 的 action runtime，不触发 Cloudflare 发布。
- 验收：PR checks 验证 CI/CodeQL/Scorecard/Mirror；真正 Docker release 仍由 tag 发布流程在需要时触发。

