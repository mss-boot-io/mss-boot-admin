# Contributing to mss-boot-admin

感谢你参与 mss-boot-admin。当前仓库维护 Admin Distribution 的源代码、测试、文档、
生成器，以及 v1.3.5、v1.3.6 不可变部分发布后的 v1.3.7 候选恢复合同。

如果你只是创建业务系统，请不要按本文 clone Foundation，也不要添加本地
`replace`。v1.3.5 已永久停止；v1.3.6 已永久停止；两者都是不完整且不可续的发布列车，
不能用于创建或升级应用。
已有采用者以 [v1.3.2 稳定记录](docs/docs/releases/archive/v1-3-2.md)为准；
v1.3.7 已选为 release candidate，但还不是稳定或可采用版本。候选制品可能分阶段公开，
在稳定提升和最终对账完成前以远端发布台账为权威；新应用等待完整公共对账。请先阅读
[采用状态](docs/docs/getting-started/index.md)。本文只适用于修改 Foundation 本身的贡献者。

## 提交问题

Bug 和功能建议统一提交到
[GitHub Issues](https://github.com/mss-boot-io/mss-boot-admin/issues)。请提供：

- 可复现步骤、预期结果和实际结果；
- 操作系统以及 Go、Node.js、pnpm 版本；
- 已脱敏的日志、错误输出或截图；
- 受影响的组件：根工具、`mss-boot/`、`admin/`、`web/antd-v6/` 或 `docs/`。

不要提交令牌、密码、私钥、生产 DSN、内部地址或未脱敏的请求内容。

## 仓库开发准备

贡献者可以 fork 并 clone 本仓库。所有命令从仓库根目录执行，除非命令段明确
切换了目录。

```shell
git clone https://github.com/YOUR_USERNAME/mss-boot-admin.git
cd mss-boot-admin
git remote add upstream https://github.com/mss-boot-io/mss-boot-admin.git

go run ./cmd/mss context
go run ./cmd/mss doctor
go run ./cmd/mss setup
```

冻结工具链为 Go 1.26.6 和 Node.js 24。Admin Web 使用 pnpm 10.34.5；Docs
使用 `docs/package.json` 固定的 pnpm 9.15.9。仓库内开发使用 `go.work` 协调
根工具、Framework 和 Admin；发布模块和外部消费者仍必须在 `GOWORK=off`
下独立成立。不得向任何发布模块提交本地 `replace`。

修改前请依次阅读：

1. 根目录 `AGENTS.md`；
2. `.mss/project.yaml`、`.mss/capabilities.yaml`、`.mss/commands.yaml`；
3. 目标目录最近的 `AGENTS.md`；
4. 与目标能力对应的结构化 spec、迁移、测试和文档。

从最新 `main` 建立一个主题分支。不要直接推送 `main`，也不要通过重写历史
隐藏修复提交。

```shell
git fetch upstream
git switch main
git merge --ff-only upstream/main
git switch -c feature/short-description
```

## 修改边界

- `mss-boot/` 只放可复用、领域无关的 Framework 能力。
- `admin/` 是完整可导入 Admin 后端；新业务能力优先放在
  `admin/modules/<name>/`。
- `web/antd-v6/` 同时是参考前端和唯一完整的
  `@mss-boot-io/admin-web` 包，不得建立第二套 SPA。
- `docs/` 是当前仓库内的文档站，不是独立 `mss-boot-docs` checkout。
- `.mss/` 保存机器可执行事实；长期说明放在 `docs/docs/`，架构决策放在
  `docs/adr/`。
- 生成文件必须从 spec 或模板重新生成，不得手改生成区域。

中大型变更先更新结构化 spec，再实现最小完整切片。涉及持久化、权限或工作流
时，必须同时覆盖迁移、后端强制授权、前端状态、正反向测试和升级路径。

## 常用开发命令

### 根工具与 Go

```shell
go run ./cmd/mss context
go run ./cmd/mss verify --changed

# Framework 独立边界
(cd mss-boot && GOWORK=off go test ./...)

# 根、Framework 和 Admin 的广泛验证
make test-all
make build
```

只格式化本次修改的 Go 文件，例如：

```shell
gofmt -w path/to/changed_file.go
```

### Admin 后端本地运行

迁移、API 注册同步和服务必须读取相同的 `STAGE` 与 DSN：

```shell
cd admin
STAGE=local go run . migrate
STAGE=local go run . server -a
STAGE=local go run . server
```

数据库或权限变更还必须运行匹配的集成测试；服务启动成功不等于迁移、授权和
业务合同已通过。

### Admin Web

```shell
cd web/antd-v6
corepack pnpm@10.34.5 install --frozen-lockfile
corepack pnpm@10.34.5 run deps:check
corepack pnpm@10.34.5 run lint
corepack pnpm@10.34.5 run test:ci
corepack pnpm@10.34.5 run build:release
```

本地开发使用：

```shell
corepack pnpm@10.34.5 --dir web/antd-v6 run start:dev
```

UI 变更必须核验 loading、empty、error、403/404、桌面与窄屏状态，并保持
浏览器控制台无错误和弃用警告。前端隐藏按钮不能替代后端授权。

### 文档

```shell
make docs-install
make docs-build
```

用户路径、命令、版本或发布合同发生变化时，同步更新当前文档和机器可读合同。
历史发布记录可以保留在 `docs/docs/releases/archive/`，但不得重新成为当前快速
开始或默认导航。

## 验证要求

先运行最小相关测试，再按影响扩展：

| 变更 | 最低验证 |
| --- | --- |
| Go 实现 | focused `go test`，然后受影响模块测试 |
| `mss-boot/` | `GOWORK=off go test ./...` |
| 根共享合同 | `make test-all`、`make build` |
| 前端 | dependency check、lint、TypeScript、focused test、release build |
| 文档 | Docs build 和链接/版本合同 |
| 迁移 | 新数据库迁移和旧版本升级测试 |
| 权限 | 后端允许与拒绝测试 |
| 生成器 | schema、golden、路径约束和两次运行幂等测试 |

最后运行：

```shell
go run ./cmd/mss verify --changed
```

只报告实际执行过的检查。Docker、数据库、浏览器或外部发布面未验证时，在 PR
中明确说明原因和剩余风险。

## 提交与 Pull Request

使用 Conventional Commits，例如：

```text
feat(module): add supplier approval workflow
fix(upgrade): preserve downstream-owned files
docs(release): record v1.3.6 immutable partial state
```

只暂存本次变更，提交前检查差异和敏感信息：

```shell
git status --short
git diff --check
git diff --cached
git commit -m "type(scope): summary"
git push origin HEAD
```

Pull Request 必须以 `main` 为目标，并说明：

- 目标与实际范围；
- 重要文件和架构选择；
- 实际运行的命令及结果；
- 迁移、兼容性和安全影响；
- UI 截图或浏览器证据（如适用）；
- 跳过的检查和具体原因。

完成审核和所需 CI 后再合并。所有可发布内容都必须先进入 `main`；不得从主题
分支、PR head、detached commit 或本地提交创建公开 tag、包、镜像或 Release。
若冻结后发现缺陷，应提交后续 PR，并从新的 merged-main commit 重新资格验证。

## 获取帮助

- 文档：https://docs.mss-boot-io.top
- Issues：https://github.com/mss-boot-io/mss-boot-admin/issues
- Discussions：https://github.com/mss-boot-io/mss-boot-admin/discussions

提交代码即表示你同意相关贡献按本仓库的 MIT License 发布，并遵守
`CODE_OF_CONDUCT.md`。
