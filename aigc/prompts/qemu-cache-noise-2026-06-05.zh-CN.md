# QEMU cache 噪声治理记忆

- 日期：2026-06-05
- 背景：Docker actions 升级到 node24 major 后，mss-boot-admin 主干 CI 已成功且不再出现 Node20 runtime 注解，但 `docker/setup-qemu-action@v4` 在并发场景下可能提示无法保存 `tonistiigi/binfmt` cache。
- 处理：在主干 Docker build/push 路径中设置 `cache-image: false`，避免 GitHub Actions cache key 并发占用造成的非阻断 warning。
- 取舍：可能牺牲少量 QEMU 初始化速度，但换取更安静稳定的开源流水线体验。
- 验收：PR 与 main CI 均需确认 image build/push 成功，且不再出现 QEMU cache save warning。

