# README badge refresh

## 背景

- 记录时间：2026-06-09
- 影响文件：`README.md`

## 问题

README 顶部 CI badge 使用旧 workflow URL，License badge 仍指向 `mashape/apistatus`，会降低外部读者对项目维护状态的信任。

## 修复

- CI badge 改为 `actions/workflows/ci.yml`。
- License badge 改为 `mss-boot-io/mss-boot-admin` 仓库 license，并链接到 `LICENSE`。

