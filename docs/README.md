# mss-boot Admin Docs

[![Docs](https://github.com/mss-boot-io/mss-boot-admin/actions/workflows/docs.yml/badge.svg)](https://github.com/mss-boot-io/mss-boot-admin/actions/workflows/docs.yml)
[![CodeQL](https://github.com/mss-boot-io/mss-boot-admin/actions/workflows/codeql.yml/badge.svg)](https://github.com/mss-boot-io/mss-boot-admin/actions/workflows/codeql.yml)
[![OpenSSF Scorecard](https://github.com/mss-boot-io/mss-boot-admin/actions/workflows/scorecard.yml/badge.svg)](https://github.com/mss-boot-io/mss-boot-admin/actions/workflows/scorecard.yml)
[![License](https://img.shields.io/github/license/mss-boot-io/mss-boot-admin.svg?style=flat-square)](https://github.com/mss-boot-io/mss-boot-admin/blob/main/LICENSE)

`docs/` 是 [`mss-boot-io/mss-boot-admin`](https://github.com/mss-boot-io/mss-boot-admin)
单仓库中的文档发布单元，基于 [dumi](https://d.umijs.org) 构建。它不是独立源码仓库；产品实现、
`.mss/` 机器契约、测试与本文档在同一个 Pull Request 中保持同步。

公开站点：[https://docs.mss-boot-io.top](https://docs.mss-boot-io.top)

## 当前发布口径

- 唯一活动目标：`v1.3.0` 稳定版候选。
- 当前稳定版：`v1.2.3`；只有 `v1.3.0` 完成公开发布和发布后对账后才会切换。
- `v1.3.0-rc.1` 至 `v1.3.0-rc.6` 是不可变的预览证据，不移动、不覆盖、不复用。
- Docs 可以用 `docs/vX.Y.Z` 独立发布，但只能来自已合并到 `main` 的精确提交。

## 文档范围

- Complete Admin Distribution：可导入 Admin Go Module、完整 Admin Web npm 包与统一版本合同。
- Thin Host：只保存组合胶水和业务代码的下游管理系统。
- Agent-native 开发：`.mss/` 规格、`mss` CLI、确定性生成、验证、评测和升级。
- 运维与安全：数据库迁移、API 注册表同步、权限、会话、部署、恢复和排障。
- 发布：安装、兼容、升级、回滚、不可变标签与公开制品对账。

推荐从以下页面开始：

1. [Admin 产品概览](./docs/admin/index.md)
2. [当前功能总览](./docs/admin/current-capabilities.md)
3. [完整 Admin Distribution 与 Thin Host](./docs/architecture/complete-admin-distribution-and-thin-business-host.zh-CN.md)
4. [v1.3.0 发布、升级与回滚合同](./docs/releases/v1-3-0.md)
5. [Agent 开发入口](./docs/agent/index.md)

## 本地预览

从仓库根目录执行：

```bash
make docs-install
make docs-build
```

只调试文档时可以进入本目录：

```bash
corepack pnpm@9.15.9 install --frozen-lockfile
corepack pnpm@9.15.9 start
```

## 验证

```bash
# 文档构建与便携路径检查
corepack pnpm@9.15.9 --dir docs build

# 由仓库合同计算本次变更所需的验证
go run ./cmd/mss verify --changed
```

发布准备 PR 还需要使用 Codex 内置浏览器检查首页、Admin、发布页、导航、深色主题和窄屏布局。
构建成功只证明静态产物可生成，不代替视觉检查或公开站点发布证据。

## 贡献

请阅读 [CONTRIBUTING.md](./CONTRIBUTING.md) 和仓库根目录的
[`AGENTS.md`](../AGENTS.md)。长期事实放在 `docs/docs/`；架构决策放在 `docs/adr/`；机器可执行
事实放在 `.mss/`。历史提示词是审计材料，不自动成为当前产品要求。

## License

MIT
