# 数据表 FAQ 与虚拟模型入口收口

## 背景

- 记录时间：2026-06-09
- 来源：GitHub Discussion `mss-boot-admin#99`，用户询问如何生成 data table。
- 相关文档：`docs/guide/faq.md`、`docs/vm/index.md`

## 决策

- 新项目推荐使用 Go model + migration 新增数据表。
- 虚拟模型是历史存量/兼容能力，不再作为新功能主线。
- `/vm` 文档入口不再保持“敬请期待”，改为 legacy capability 说明，并链接到现有降级文档。

## 验证

- 文档构建应通过 `pnpm build`。
- 后续如果外部继续询问 data table / virtual model，应优先链接 FAQ 和 legacy capability deprecation。
