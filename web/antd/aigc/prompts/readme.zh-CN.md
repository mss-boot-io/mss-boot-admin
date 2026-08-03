# AIGC 提示词与评估文档索引（mss-boot-admin-antd）

## 使用说明

- 本目录用于存放本仓库的提示词、评估、改造计划与交接文档。
- 文档建议遵循开源协作语气：中立、可讨论、可验证。
- 建议在文档中声明“评估时点、适用范围、关键假设”，减少跨分支歧义。
- 建议优先附上可复现步骤（检索命令、构建命令、回归清单）。

## 当前文档

1. `frontend-multi-tenant-cache-evaluation.zh-CN.md`

   - 前端多租户实现与缓存/状态维度评估。

2. `frontend-tenant-removal-impact-plan.zh-CN.md`

   - 去多租户影响分析、分阶段任务与验收标准。

3. `frontend-tenant-removal-handoff-summary.zh-CN.md`

   - 面向实施团队的一页交接摘要与检查清单。

4. `package-metadata-node22-2026-06-09.zh-CN.md`

   - Node 22、pnpm 9、package metadata 与 Cloudflare 发布约束记忆。

5. `security-dependency-convergence-2026-06-09.zh-CN.md`

   - 前端安全依赖收敛、暂缓项与验证命令记录。

## 维护建议

- 当路由、服务、typings、i18n 发生结构性变化时，建议同步更新以上文档。
- 评估文档更新时，建议保留“上次结论”和“本次变更点”以便社区协作复核。
