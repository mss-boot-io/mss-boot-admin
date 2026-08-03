# 2026-06-05 AGENTS.md 落盘记忆

## 背景

`mss-boot` 只有 `.github/copilot-instructions.md`，没有根目录 `AGENTS.md`。对 Codex、Claude、本地 agent 和外部贡献者来说，根目录 agent 指南是低成本、高可见度的协作入口。

## 处理

- 新增 `AGENTS.md`，说明仓库角色、AI 工作规则、验证命令、PR 说明要求、发布兼容约束。
- 保持 repository-local 记忆继续写入 `aigc/prompts/`。

## 验证

- `git diff --check`

## 后续

其他核心仓库可以按同样结构补齐或对齐 `AGENTS.md`，但要保留各自仓库的验证命令和发布约束。
