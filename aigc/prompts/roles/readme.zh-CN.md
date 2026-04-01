# 角色提示词索引（mss-boot-admin）

## 目录说明

- 本目录存放 `mss-boot-admin` 相关角色提示词。
- 当前包含后端实现角色和产品协调角色。
- 文件命名遵循小写 kebab-case，中文文档使用 `.zh-CN.md` 后缀。

## 当前文件

1. `backend-developer-role.zh-CN.md`
   - 后端开发角色定义。
   - 负责接口、DTO、模型、服务、权限和联调支持。

2. `leader-role.zh-CN.md`
   - leader 角色定义。
   - 负责功能梳理、优先级、协同顺序、阶段目标和验收口径。

## 使用建议

- OpenCode 默认入口是 `leader-role.zh-CN.md`；用户优先与 leader 沟通。
- 当任务已经进入后台实现、接口设计、权限逻辑或数据模型阶段，由 leader 调用后端开发角色。
- 当实现过程中发现问题已超出业务层边界，应由 leader 再同步架构设计师。
