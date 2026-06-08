# README 测试前置条件与 RBAC 术语表记忆

- 日期：2026-06-09
- 仓库：mss-boot-admin
- 对应 issue：#344、#345

## 本轮处理

- README / README.zh-CN 的准备工作中，将后端 Go 版本说明对齐到 `go.mod` 和 CI：Go 1.26+。
- 英文 README 新增 `Local Test Prerequisites`，说明 `make test`、`make deps`、Redis 7、CI 服务准备方式和敏感信息边界。
- 中文 README 新增 `本地测试前置条件`，覆盖同一验证口径。
- README / README.zh-CN 新增 RBAC 术语表，解释 User、Role、Menu、API、Permission path、Casbin rule、Access type、Data scope、Default role。

## 边界

- 本轮只改文档与 AIGC 记忆，不修改权限模型、菜单、路由、迁移、seed data 或生产配置。
- 测试说明不包含真实生产 DSN、token、Kubernetes 集群或私有端点。

## 验证

- `make test`
- `git diff --check`
- 静态 README 检查：确认 Go 1.26、Redis 7、`make test`、RBAC 关键术语均存在。
