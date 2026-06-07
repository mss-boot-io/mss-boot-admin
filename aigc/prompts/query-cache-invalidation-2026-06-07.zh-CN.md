# Query Cache Invalidation

日期：2026-06-07

## 背景

外部 issue `#105` 反馈：启用 `cache.queryCache` 后，更新数据后列表显示已更新，
再次打开编辑/详情仍可能读到旧数据。

## 决策

- admin 启动时必须把 mss-boot 的 query cache 初始化回调传给 `Cache.Init`。
- 初始化 query cache 后，必须绑定 `response/actions/gorm.CleanCacheFromTag`。
- mss-boot GORM update/delete actions 会调用 `CleanCacheFromTag(tableName)`；
  admin 侧需要把表名转换为 mss-boot cache 实际使用的 tag：`gorm.cache:<table>`。

## 本轮变更

- `config/config.go`
  - 接入 `cache.queryCache` 的 GORM plugin 初始化。
  - 注册基于 table tag 的缓存清理函数。
- `config/query_cache_test.go`
  - 覆盖 query cache 初始化。
  - 覆盖 `gorm.cache:` tag 前缀清理。

## 验证

```text
go test ./config
go test ./config ./middleware ./apis ./models
```

## 后续

- 若 `#105` 仍可复现，需要用户补充版本、缓存配置、更新接口、读取接口和复现步骤。
- 创建动作当前不在 mss-boot GORM action 里触发 tag 清理；如果后续出现“创建后列表缓存未刷新”，
  需要在 mss-boot 框架层补 create 后的同类清理。
