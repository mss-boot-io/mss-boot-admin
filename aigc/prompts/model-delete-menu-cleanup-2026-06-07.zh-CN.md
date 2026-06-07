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
- 同步删除这些菜单/API 路径对应的 Casbin policy，并重新加载 policy。
- 不处理无关菜单，避免误删用户手动维护的其他菜单。

## 本轮变更

- `apis/model.go`
  - 为模型 controller 增加 `WithAfterDelete(deleteGeneratedModelMenus)`。
  - 根据通用 Delete action 写入的 `ids` 找到已删除模型。
  - 递归软删除 `/virtual/<path>` 菜单树。
  - 删除关联 Casbin policy。
- `apis/model_test.go`
  - 覆盖模型删除后生成菜单树被软删除。
  - 覆盖关联 policy 被清理。
  - 覆盖无关菜单和 policy 不受影响。
  - 覆盖仍有 active 模型使用相同 path 时不清理菜单和 policy。

## 验证

```text
go test ./apis
```

## 后续

- `#75` 仍包含多个未拆分问题；本轮只处理其中“模型删除后菜单残留”这个可确认子问题。
- 其他子问题仍需要基于当前 main 或最新 release 补充复现步骤、接口响应和前端截图。
