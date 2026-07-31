# AIGC 目录说明

本目录用于统一存放与 AI 生成相关的提示词、模板与规范文档。

## 目录约定

- 提示词与约束文档统一放在：`aigc/prompts/`
- 后续如有模板，可按需放在：`aigc/templates/`（可选）

## 强制落盘规则

- 任何新生成的提示词和文档，必须写入 `aigc/` 下的子目录（优先 `aigc/prompts/`）。
- 严禁将此类文件直接写入项目根目录。
- 若目标路径不合规，必须自动重定向到 `aigc/prompts/`。

## 命名规范

- 文件名使用小写英文和连字符：`xxx-yyy.md`
- 中文版本建议使用后缀：`*.zh-CN.md`
- 保留历史时使用日期后缀：`YYYY-MM-DD`

## 参考文件

- 详细约束见：`aigc/prompts/prompt-constraints.zh-CN.md`
- Query cache tag invalidation：`aigc/prompts/query-cache-tag-invalidation-2026-06-07.zh-CN.md`
- README 本地验证与 Action/Controller 术语：`aigc/prompts/readme-test-lint-action-glossary-2026-06-09.zh-CN.md`
