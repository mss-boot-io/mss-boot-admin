# Docker action runtime 升级记忆

- 日期：2026-06-05
- 背景：mss-boot-admin 主干 CI 已全绿，但 Docker image build/push 阶段提示 `docker/login-action@v3`、`docker/setup-qemu-action@v3`、`docker/setup-buildx-action@v3`、`docker/metadata-action@v5`、`docker/build-push-action@v5` 仍使用 Node.js 20 runtime。
- 处理：升级 Docker 官方 actions 到 node24 major：login/setup-qemu/setup-buildx 使用 `v4`，metadata 使用 `v6`，build-push 使用 `v7`。
- 约束：保持现有镜像构建、tag、push 逻辑不变，只升级 action runtime。
- 验收：PR checks 与 main CI 验证 Docker image build/push 成功，确认不再出现 Docker actions 的 Node20 runtime 注解。

