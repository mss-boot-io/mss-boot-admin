---
title: 使用指南
order: 2
nav:
  title: 使用指南
  order: 2
description: Thin Host 日常开发、配置、验证、部署与故障排查入口
---

# 使用指南

本区不重复安装或建项目步骤；新项目统一从
[v1.3.4 快速开始](/getting-started)进入。

## 日常路径

| 任务 | 入口 |
| --- | --- |
| 检查工具链和仓库合同 | `mss doctor --strict` |
| 安装冻结依赖 | `mss setup` |
| 启动、查看和停止服务 | [本地调试](/admin/local-debug) |
| 配置数据库、会话、CORS 与资源 | [配置指南](/admin/configuration-guide) |
| 运行变更或完整验证 | [集成验证](/admin/integration-test-guide) |
| 构建容器和记录摘要 | [容器部署](/admin/docker) |
| 检查安全上线条件 | [安全基线](/admin/security-baseline) |
| 处理常见失败 | [FAQ](/guide/faq) |

业务模块应先修改 `.mss/modules/<name>.yaml`，再用 `mss spec validate` 和
`mss module generate` 产生确定性变更。不要手改生成区。
