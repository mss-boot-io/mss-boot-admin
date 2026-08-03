---
title: 应用 Blueprint 与升级
order: 5
nav:
  title: Agent 开发
  order: 2
description: 从 foundation 创建独立管理系统，并通过三方比较持续升级
keywords: [blueprint application generator foundation upgrade three-way]
---

# 应用 Blueprint 与升级

`mss-boot-admin` 同时是：

- Agent 基础设施源码；
- 可运行参考应用；
- 管理系统 Blueprint；
- 下游升级规则源。

新业务系统不应长期依赖手工 Fork 后各自分裂，而应通过版本化 Blueprint 创建并保存升级基线。

## 创建新应用

### 先 dry-run

```shell
go run ./cmd/mss new app customer-admin \
  --display-name 'Customer Administration' \
  --module github.com/acme/customer-admin \
  --repository acme/customer-admin \
  --format json
```

默认目标：

```text
.mss/output/customer-admin
```

也可以显式指定：

```shell
--destination /workspace/customer-admin
```

外部目标路径只适用于 `new app`，并且仍然需要检查冲突。

### 审查计划

重点检查：

- Application name、display name；
- Go module；
- Git repository；
- Blueprint 名称和版本；
- Foundation commit；
- 文件数和总大小；
- `create`、`unchanged`、`conflict`；
- 排除的运行态、构建态和历史 Prompt；
- 二进制文件是否保持原样。

### 写入

```shell
go run ./cmd/mss new app customer-admin \
  --display-name 'Customer Administration' \
  --module github.com/acme/customer-admin \
  --repository acme/customer-admin \
  --destination /workspace/customer-admin \
  --write \
  --git-init \
  --format json
```

命令不会自动创建初始 Git Commit。建议先验证再提交 Foundation baseline。

## Blueprint 文件选择

Blueprint 定义：

```text
.mss/blueprints/management-system.yaml
```

原则：

- 只读取 Git 已跟踪文件；
- 不读取工作区未提交临时文件；
- 不复制 `.git`；
- 不复制 `node_modules`、`vendor`、build output；
- 不复制 PID、日志、缓存和报告；
- 不复制历史一次性 Prompt；
- 不执行任意模板脚本；
- 文本文件执行受控替换；
- 二进制文件按字节复制；
- 每个输出记录 SHA-256 和文件权限。

## 生成的升级基线

下游项目包含：

```text
.mss/lock.yaml
.mss/blueprint-manifest.json
```

Lock 记录：

- Foundation repository；
- 版本和 commit；
- Blueprint 名称和版本；
- 合同版本；
- Generator 版本；
- 后续升级记录。

Manifest 记录每个 Foundation-managed 文件的：

```text
path
SHA-256
mode
size
```

这份 Manifest 是未来三方升级中的 `base`，不能删除或随意重写。

## 三方升级模型

```text
旧 Foundation Manifest Hash
       +
当前下游文件
       +
新 Foundation 期望文件
       ↓
create / update / delete / preserve / conflict
```

含义：

| Action | 条件 |
| --- | --- |
| `create` | 新 Foundation 增加文件，下游没有冲突文件 |
| `update` | Foundation 修改文件，下游仍等于旧基线 |
| `delete` | Foundation 删除文件，下游仍等于旧基线 |
| `preserve` | 下游修改文件，但 Foundation 没有修改它 |
| `unchanged` | 当前内容已是目标内容或双方都没改 |
| `conflict` | 下游和 Foundation 同时修改，或目标文件发生碰撞 |

下游自己创建、没有进入旧 Manifest 的业务文件不属于 Foundation 管理范围，会保留。

## 查看当前基线

```shell
go run ./cmd/mss upgrade status --format json
```

## 规划升级

建议使用目标 Foundation checkout 中的新 CLI：

```shell
cd /workspace/new-mss-foundation

go run ./cmd/mss \
  --root /workspace/customer-admin \
  upgrade plan \
  --foundation /workspace/new-mss-foundation \
  --format json
```

原因是旧下游 CLI 可能不理解新 Blueprint 或升级规则。

## 处理冲突

在任何 `conflict` 存在时：

- `upgrade apply` 拒绝写入；
- Manifest 不更新；
- 不覆盖下游业务逻辑；
- 不删除已定制文件。

冲突应在单独 Commit 中人工或 Agent 解决，然后重新运行 plan。

## 应用升级

```shell
go run ./cmd/mss \
  --root /workspace/customer-admin \
  upgrade apply \
  --foundation /workspace/new-mss-foundation \
  --yes \
  --format json
```

执行顺序：

1. 应用 create/update/delete；
2. 保留 preserve；
3. 任意文件操作失败则停止；
4. 所有文件成功后，最后写入新 Manifest。

因此部分失败不会伪装成已安装新版本。

## 升级后验证

```shell
cd /workspace/customer-admin

go run ./cmd/mss doctor --format json
go run ./cmd/mss skills validate --format json
go run ./cmd/mss verify --changed
go run ./cmd/mss eval run --all
```

同时检查：

- 数据库迁移；
- 权限和菜单；
- API 兼容性；
- 前端生成客户端；
- 自定义扩展边界；
- FeatureSpec 验收；
- 回滚 Commit。

源码升级不会自动执行生产数据库迁移。
