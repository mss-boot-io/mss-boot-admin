# config env template fallback 测试记忆

## 背景

- 时间：2026-06-09
- 仓库：`mss-boot`
- 关联 issue：`mss-boot-io/mss-boot#350`
- 目标：为配置模板中的环境变量 fallback 行为补充小范围、可复现、无外部依赖的测试。

## 实施

- 在 `pkg/config/config_test.go` 中补充表驱动测试。
- 覆盖缺失变量渲染为空字符串、精确命中、小写 key fallback 到大写环境变量、大写 key fallback 到小写环境变量。
- 补充 `getValueFromEnv` 的 map 结果断言，保证非 `.Env.*` 解析键不会污染环境变量映射。

## 验证

- `go test ./pkg/config`
- `go test ./...`

## 约束

- 不修改配置解析语义。
- 不引入网络、数据库、Redis、对象存储或外部凭据依赖。
- 该改动属于社区 good-first-issue 的测试覆盖收敛，应保持 PR 小而清晰。
