---
title: 发布与升级
order: 1
nav:
  title: 发布
  order: 3
description: mss-boot-admin 版本状态、升级、兼容性与回滚合同
keywords: [release upgrade rollback compatibility mss-boot-admin]
---

# 发布与升级

这里保存长期有效的版本合同。Git tag、GitHub Release、嵌套 Go 模块的外部解析结果和对应提交上的验证报告共同构成发布证据；分支名、`Unreleased`、`planned`、`preview` 或本地 `go.work` 替换都不代表稳定版本。

## v1.0.0 候选

当前状态：**发布准备中（preview），尚未发布 stable**。

这是合并仓库后的首个稳定 1.0 版本边界。未发布 v0.8.0 候选版的制品、摘要和验证记录不得复用；所有证据必须绑定精确的 v1.0.0 发布提交。

- [发布合同](/releases/v1-0-0)
- [从 v0.7.x 升级](/releases/v1-0-0-upgrade)
- [兼容性矩阵](/releases/v1-0-0-compatibility)
- [回滚与恢复](/releases/v1-0-0-rollback)

稳定发布必须遵循以下最短顺序：

1. 在已评审的同一发布提交上完成全部验证；
2. 发布 `mss-boot/v1.0.0`；
3. 在仓库外、关闭 workspace 替换后验证嵌套模块可解析；
4. 再发布根标签 `v1.0.0`；
5. 对已发布制品执行禁用缓存的安装与运行冒烟。

任何一步失败都保持 preview，不得为了补齐版本号而提前发布后续标签。
