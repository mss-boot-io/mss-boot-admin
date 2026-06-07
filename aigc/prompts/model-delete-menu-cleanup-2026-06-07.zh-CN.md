# Model Delete Menu Cleanup

日期：2026-06-07

## 背景

外部 issue `#75` 反馈：模型列表删除后，首页 MenuBar 中仍然保留对应菜单，
点击后会报错，需要手动删除菜单。

当前模型生成数据时会创建 `/virtual/<model.path>` 菜单树及其 API/组件子节点；
但模型删除走通用 GORM Delete action，没有同步清理生成菜单和 Casbin policy。

## 决策

- 模型删除后清理该模型生成的菜单树，而不是要求用户手动删除菜单。
- 清理范围只覆盖模型生成的 `/virtual/<model.path>` 根菜单及其子节点。
- 如果仍有其他 active 模型使用相同 `path`，不清理该菜单树，避免误删共享 path 的
  仍在用菜单。
- 只清理 `generated_data=true` 的模型对应菜单，避免模型未生成数据时误删用户手工菜单。
- 根菜单必须是 `MenuAccessType`，避免相同 path 的非菜单节点被当作生成菜单树根。
- 菜单软删除和 Casbin policy 删除必须在同一 DB transaction 中完成。
- 同步精确删除这些菜单/API 路径对应的 Casbin policy；匹配条件使用
  `ptype + v1(type) + v2(path) + v3(method)`，避免仅按 path 误删复用 policy。
- 事务提交后重新加载 policy；`LoadPolicy` 失败只记录 warning，不把已完成的数据清理
  上报为接口 500。
- 不处理无关菜单，避免误删用户手动维护的其他菜单。

## 本轮变更

- `apis/model.go`
  - 为模型 controller 增加 `WithAfterDelete(deleteGeneratedModelMenus)`。
  - 根据通用 Delete action 写入的 `ids` 找到已删除模型。
  - 递归软删除 `/virtual/<path>` 菜单树。
  - 删除关联 Casbin policy，并保护同 path active 模型和未生成数据的手工菜单。
- `apis/model_test.go`
  - 覆盖模型删除后生成菜单树被软删除。
  - 覆盖关联 policy 被清理。
  - 覆盖无关菜单和 policy 不受影响。
  - 覆盖仍有 active 模型使用相同 path 时不清理菜单和 policy。
  - 覆盖 policy 删除失败时菜单软删除回滚。
  - 覆盖 `LoadPolicy` 失败时不阻断已完成清理。
  - 覆盖同 path 但 type/method 不同的 policy 不被误删。
  - 覆盖 `generated_data=false` 时手工菜单和 policy 不被误删。

## 验证

```text
go test ./apis
go test ./apis ./models ./middleware
```

## 后续

- `#75` 仍包含多个未拆分问题；本轮只处理其中“模型删除后菜单残留”这个可确认子问题。
- 其他子问题仍需要基于当前 main 或最新 release 补充复现步骤、接口响应和前端截图。
