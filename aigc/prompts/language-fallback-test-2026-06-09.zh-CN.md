# language middleware fallback 测试记忆

## 背景

- 时间：2026-06-09
- 仓库：`mss-boot-admin`
- 关联 issue：`mss-boot-io/mss-boot-admin#346`
- 目标：补齐语言中间件 fallback 行为的独立单测，让贡献者可以在不依赖数据库、Redis、外部服务的情况下验证用户可见的语言选择逻辑。

## 实施

- 新增 `middleware/language_test.go`。
- 覆盖空 `Accept-Language`、受支持语言、带权重后缀的语言、区域语言归一化、不支持语言 fallback、query 参数 fallback、`Content-Language` fallback。
- 同时断言 `GetLanguage(c)` 和响应 `Content-Language` 头，确保 middleware 写入 context 与响应头的一致性。

## 验证

- `go test ./middleware`
- `go test ./...`

## 约束

- 不修改产品行为。
- 不改权限、路由、数据库迁移、种子数据、缓存或生产配置。
- 此类社区 issue 适合保持小 PR、强验证、清晰说明，便于外部贡献者 review。
