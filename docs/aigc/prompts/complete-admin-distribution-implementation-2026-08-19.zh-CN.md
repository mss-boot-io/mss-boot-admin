# Codex 实施提示词：完整 Admin 发行包与轻量业务宿主

时间：2026-08-19

目标仓库：`mss-boot-io/mss-boot-admin`

设计事实源：

```text
docs/docs/architecture/complete-admin-distribution-and-thin-business-host.zh-CN.md
```

本提示词编写时检查的 `main` 基线为：

```text
9a256229774bb255dfe8a618613522fd70538195
```

开始实施时必须重新获取远程最新 `main`，不得假设上述 SHA 仍是最新提交。

---

## 一、任务性质

你现在负责在 `mss-boot-io/mss-boot-admin` 中完整实施“完整 Admin 发行包 + 轻量业务宿主”架构。

这不是调研、只写 ADR、接口草图或一次性 PoC。请从最新代码开始，在同一个功能分支上持续实施、提交、推送、验证和修复，直到满足本文全部验收标准，然后创建 Pull Request。

不要在完成某个小阶段后停下来询问是否继续。遇到不涉及破坏性数据、凭据或正式发布的设计细节，应根据本提示词、设计文档、仓库机器契约和现有代码自行作出最小、可验证的决定，并把决策记录到 ADR 或长期文档中。

本任务不包括记录谁 import、采用者登记、遥测或任何用户数据采集。

---

## 二、仓库和分支操作

首先检查环境：

```bash
git status --short
git remote -v
git fetch origin --prune
git rev-parse origin/main
```

阅读当前分支和工作区状态。

如果工作区干净，从最新 `origin/main` 创建：

```bash
git switch main
git pull --ff-only origin main
git switch -c codex/complete-admin-distribution
```

如果工作区存在不属于本任务的修改：

- 不得删除、覆盖、reset 或 stash 用户修改；
- 从最新 `origin/main` 创建独立 worktree；
- 在独立 worktree 中创建 `codex/complete-admin-distribution`。

整个任务使用同一个分支：

```text
codex/complete-admin-distribution
```

禁止：

- 直接推送 `main`；
- 对已推送历史进行 rebase；
- force push；
- 使用 destructive reset 隐藏中间修复；
- 创建、移动或覆盖正式 Tag；
- 发布 npm 包、GitHub Release 或生产镜像；
- 修改用户无关的本地工作内容。

每完成一个可独立理解的纵向阶段就提交并推送，避免长时间只把工作留在本地。

建议提交序列：

```text
docs(architecture): record complete Admin distribution implementation contract
refactor(admin): expose complete application composition API
feat(admin): support external compiled business modules
feat(web): package complete Admin frontend for downstream hosts
feat(mss): generate thin downstream application hosts
feat(mss): coordinate complete Admin distribution upgrades
test(compat): qualify external downstream consumption
docs: document thin-host development and upgrade workflow
fix: resolve validation and CI findings
```

第一个可以编译的后端纵向切片完成后创建 Draft PR，后续持续在同一 PR 上推送。全部验收完成后再标记为 Ready for review。

---

## 三、开始前必须阅读

按顺序阅读：

```text
AGENTS.md
admin/AGENTS.md
web/antd-v6/AGENTS.md
.mss/project.yaml
.mss/capabilities.yaml
.mss/commands.yaml
.mss/release-policy.yaml
.mss/blueprints/management-system.yaml
MONOREPO.md
docs/docs/architecture/complete-admin-distribution-and-thin-business-host.zh-CN.md
```

继续检查相关实现：

```text
admin/main.go
admin/cmd/
admin/cmd/server/
admin/center/
admin/router/
admin/middleware/
admin/modules/
admin/modules/runtime/
admin/modules/all/
admin/modules/supplier/
admin/go.mod

web/antd-v6/package.json
web/antd-v6/config/
web/antd-v6/src/
web/antd-v6/scripts/
web/antd-v6/pnpm-lock.yaml

cmd/mss/
internal/mss/app/new.go
internal/mss/app/upgrade.go
internal/mss/blueprint/
internal/mss/generator/
internal/mss/project/
internal/mss/verify/
templates/module/

.github/workflows/
tools/release/
```

先搜索已有的应用创建、模块注册、Supplier 组合、路由生成、Blueprint、升级、Frontend V6 发布和外部兼容测试能力。不要在已有能力旁边建立第二套逻辑。

事实源优先级遵守 `AGENTS.md`：编译代码和迁移优先，其次 `.mss/` 契约、测试、当前 ADR 和长期文档。

---

## 四、最终目标

`mss-boot-admin` 必须保持一套完整产品。

真实业务项目位于独立仓库，通过版本化依赖引用完整 Admin，在编译期追加自己的业务代码，最终仍只生成：

```text
一个后端二进制
一个前端 dist
一套登录和 Session
一套权限和菜单
一个逻辑应用
```

目标关系：

```text
完整 mss-boot-admin 后端
+
业务后端模块
=
一个后端二进制

完整 mss-boot-admin 前端
+
业务前端页面
=
一个前端 dist
```

优先服务仓库作者自己的真实项目和 Codex 等 Coding Agent，不为所有技术栈和所有组合方式建立通用生态。

---

## 五、不可协商的架构原则

### 5.1 Admin 保持完整

当前 Admin 的全部基础功能继续作为一个产品共同开发、测试和发布，包括但不限于：

```text
用户、角色、部门、岗位
菜单、API、Casbin 权限
登录、Session、OAuth、CSRF、WebSocket
配置、审计、通知、任务、存储
监控、统计及当前其他正式功能
完整前端布局、登录页、核心页面和公共运行时
```

允许增加少量公共入口：

```text
admin/app
admin/business
```

它们只用于完整应用启动和业务扩展，不代表拆分核心功能。

不得设计让下游选择禁用用户、替换权限系统、移除菜单或切换另一套 Session。

### 5.2 只有一套前端

正式前端源代码仍只有：

```text
web/antd-v6
```

只发布一个完整前端包，建议：

```text
@mss-boot-io/admin-web
```

允许同一 npm 包使用 `exports` 暴露不同入口，但不得拆成独立版本的：

```text
admin-runtime
admin-shell
admin-auth
admin-layout
admin-contracts
admin-components
```

### 5.3 编译期扩展

- 业务后端通过 Go 编译期组合；
- 业务前端通过 Umi 构建期组合；
- 不加载远程业务代码；
- 不运行第二套 Admin 应用。

禁止引入：

```text
Go plugin .so
运行时下载或执行业务代码
Qiankun
Module Federation
iframe 微应用
远程前端 Entry
为源码隔离而拆微服务
```

### 5.4 一个产品版本

后端 Go Module、前端 npm 包和根发行版产品层面属于一个 `Admin Distribution` 版本。

技术 Tag 可以是：

```text
vX.Y.Z
mss-boot/vX.Y.Z
admin/vX.Y.Z
web/antd-v6/vX.Y.Z
```

版本核心必须一致，不允许各自独立演进。

### 5.5 无遥测和 import 记录

不得添加：

```text
import 记录
安装统计
OAuth 采用者登记
运行时 call-home
匿名遥测
项目、仓库、设备或用户数据采集
更新检查顺带上报
```

---

## 六、目标下游仓库

`mss new app` 最终应生成轻量业务宿主，而不是复制完整 Foundation。

建议结构：

```text
example-business-admin/
├── AGENTS.md
├── go.mod
├── go.sum
├── cmd/
│   └── server/
│       └── main.go
├── internal/
│   └── modules/
│       ├── all/
│       │   └── generated.go
│       └── supplier/
├── web/
│   ├── package.json
│   ├── pnpm-lock.yaml
│   ├── tsconfig.json
│   ├── mss-admin.config.ts
│   ├── config/
│   │   ├── config.ts
│   │   └── business-routes.generated.ts
│   └── src/
│       ├── app.tsx
│       ├── access.ts
│       ├── locales/
│       ├── generated/
│       └── business/
├── .mss/
│   ├── project.yaml
│   ├── lock.yaml
│   ├── blueprint-manifest.json
│   ├── modules/
│   ├── features/
│   └── evals/
├── config/
├── Dockerfile
├── Makefile
└── .github/workflows/
```

后端依赖：

```go
require github.com/mss-boot-io/mss-boot-admin/admin vX.Y.Z
```

前端依赖：

```json
{
  "dependencies": {
    "@mss-boot-io/admin-web": "X.Y.Z"
  }
}
```

下游不得复制并长期维护：

```text
admin/models
admin/service
admin/router
admin/middleware
admin/center
完整 Admin 页面
完整 Admin shared 目录
mss-boot 源码
Foundation 发布和文档工程
```

---

## 七、后端实施

### 7.1 可导入的完整应用入口

当前 `admin/main.go` 直接执行 CLI。将完整启动过程抽取为同一 `admin` Module 内可导入、可测试、返回错误的公共 API。

推荐位置：

```text
admin/app/
```

目标使用示例：

```go
package main

import (
    "context"
    "log"

    adminapp "github.com/mss-boot-io/mss-boot-admin/admin/app"
    "github.com/acme/example-business-admin/internal/modules/all"
)

func main() {
    err := adminapp.ExecuteContext(
        context.Background(),
        adminapp.WithBusinessModules(all.Modules()...),
    )
    if err != nil {
        log.Fatal(err)
    }
}
```

实际名称可以结合现有 Cobra 和生命周期调整，但必须满足：

- 库代码不调用 `os.Exit`；
- 返回可诊断错误；
- 接受 `context.Context`；
- 保留 `server`、`migrate` 和现有 CLI 参数；
- 保留环境变量和配置兼容性；
- 官方参考应用和外部宿主共享唯一启动实现；
- 当前全部 Admin 核心功能默认启用；
- 多个测试构造不因不可重置全局状态相互污染；
- 配置、数据库、缓存、队列、任务和关闭生命周期仍由完整 Admin 统一拥有。

不要复制第二份启动流程。

### 7.2 最小 BusinessModule 接口

只开放业务代码真正需要的扩展面。

接口可以类似：

```go
type Module interface {
    Name() string
    Register(*Registry) error
}
```

`Registry` 首版至少支持当前 AdminModule 生成链路需要的：

```text
模块身份
数据库迁移
授权迁移
权限描述
菜单描述
经过完整 Admin 中间件保护的业务 API 路由
当前生成模块实际使用的领域事件
```

要求：

- 显式组合优先，不依赖不可见的全局 `init()` 才能工作；
- 注册顺序稳定；
- 重复模块名明确报错；
- 注册错误返回给调用者；
- 业务模块不能替换核心认证、权限、Session 或安全中间件；
- 业务路由必须执行后端授权；
- 迁移 readiness 通过后才挂载路由；
- 业务模块不依赖 Foundation 根 Module 或 `internal/mss/...`；
- 公共 API 不暴露 Go `internal` 包；
- 不把 Admin 领域能力放入 `mss-boot/`。

不要为了未来假设创建庞大生命周期框架。任务、事件处理器、健康检查等只有当前实现或生成模块确实需要时才纳入稳定接口。

### 7.3 Supplier 迁移

当前 Supplier 有专用服务器组合和路由挂载代码。将它迁移为通用业务模块的黄金样例。

迁移后必须满足：

- Supplier 不再由 `server.go` 或专用流程特殊识别；
- Supplier 与外部业务模块使用相同公共接口；
- 保留请求级数据库租约；
- 保留迁移 readiness；
- 保留权限正向和拒绝测试；
- 保留 CRUD、导出和领域事件；
- 保留当前前端能力和测试。

Supplier 只作为参考应用模块和兼容测试 Fixture，不建立第二套 Admin 产品。

### 7.4 官方参考应用兼容

以下命令继续工作：

```bash
cd admin
go run . --help
go run . server
go run . migrate
```

现有容器、Swagger、配置相对路径、版本注入和开发入口不能因可导入能力而失效。

---

## 八、前端实施

### 8.1 完整前端单包

`web/antd-v6` 同时作为：

```text
唯一正式前端源码
官方参考应用
下游可消费的完整 Admin Web 包
```

将其改造为可以通过 `pnpm pack` 被仓库外业务宿主安装和构建的完整包。

建议包名：

```text
@mss-boot-io/admin-web
```

要求：

- 去除阻止打包的 `private: true`；
- 配置明确的 `files` 白名单；
- 配置稳定 `exports`；
- 包含完整运行所需源码、配置、样式、公共资源、补丁和构建支持；
- 排除 `node_modules`、缓存、报告、日志、凭据和临时输出；
- `pnpm pack` 产物能在仓库外安装和构建；
- 官方参考应用继续从同一份源代码构建；
- 不创建第二份核心前端源码或第二个长期维护 SPA。

### 8.2 同一包内的 Umi 集成

优先实现 Umi Preset、Plugin、配置工厂或等价方案。

外部宿主配置应足够薄，例如：

```ts
import { defineBusinessAdmin } from '@mss-boot-io/admin-web/business';
import businessRoutes from './business-routes.generated';

export default defineBusinessAdmin({
  businessRoutes,
  routeRegistrations: './src/generated/routes.ts',
  useUtoopack: true,
});
```

或者：

```ts
export default {
  presets: [require.resolve('@mss-boot-io/admin-web/preset')],
  mssAdmin: {
    title: 'Example Business Admin',
    businessRoutes: './config/business-routes.generated.ts',
  },
};
```

根据当前 Umi Max API 选择最稳定实现，但必须做到：

- 完整 Admin 基础路由自动存在；
- 业务路由插入 403/404 fallback 之前；
- 完全重复、核心动态参数、核心 wildcard 和互相重叠的业务路由模式在构建前 fail closed；
- 完整 Session、CSRF、Request、React Query、Theme、国际化、WebSocket 和 Layout 自动存在；
- 业务宿主不复制 `src/shared` 和核心页面；
- 业务页面在同一次 Umi 构建中编译；
- 不运行第二个前端开发服务器；
- 不加载远程 Entry；
- 最终只有一个 `dist`；
- 业务页面可以使用同一包公开的组件、类型和 Hook；
- 业务代码不能依赖包内未导出的相对路径；
- 公共 exports 有契约测试。
- Thin Host 开发构建禁用 MFSU 并显式使用 Utoopack；发行构建继续经过受控的 runtime、bundle 和 release API 门槛。

### 8.3 统一前端命令

尽量由同一个 npm 包暴露：

```text
mss-admin-web dev
mss-admin-web lint
mss-admin-web test
mss-admin-web build
```

下游脚本示意：

```json
{
  "scripts": {
    "dev": "mss-admin-web dev",
    "lint": "mss-admin-web lint",
    "test": "mss-admin-web test",
    "build": "mss-admin-web build"
  }
}
```

下游可以增加业务依赖，但不应复制完整 Admin 的核心依赖和构建配置。

### 8.4 单 Runtime 验证

必须通过真实安装树和构建产物验证只有一份：

```text
react
react-dom
antd
@ant-design/pro-components
@tanstack/react-query
@umijs/max / Umi runtime
```

合理设计 `dependencies`、`peerDependencies` 和 CLI 解析，不得只凭顶层清单宣称没有重复依赖。

### 8.5 保留前端质量门槛

不能降低：

```text
Node 24
pnpm 10.34.5
React 19
Ant Design 6
Biome
TypeScript
Vitest
Playwright
Vite 8.2.1（仅作为 Vitest 4 的精确构建期 peer）
bundle budget
release API check
runtime bundle check
delivery smoke
当前安全 Session 协议
当前默认主题行为
```

业务页面仍需考虑 loading、empty、error、403、404、conflict、桌面/移动、zh-CN/en-US、键盘/焦点和后端授权。

---

## 九、Blueprint 和生成器

### 9.1 `management-system` 改为 Thin Host

当前 Blueprint 会复制 Foundation 大量文件。将 `management-system` 升级为唯一推荐的轻量业务宿主 Blueprint。

不要长期同时维护：

```text
完整复制 Blueprint
轻量宿主 Blueprint
```

两条默认产品路径。

历史 Manifest 可以被识别和迁移，但不能继续作为另一套活跃架构。

### 9.2 增加应用模板

建议增加：

```text
templates/application/
```

模板生成：

```text
go.mod
cmd/server/main.go
internal/modules/all/generated.go
web/package.json
web/tsconfig.json
web/config/config.ts
web/mss-admin.config.ts
web/src/app.tsx
web/src/access.ts
web/src/locales/zh-CN.ts
web/src/locales/en-US.ts
Dockerfile
Makefile
业务 CI
.mss/project.yaml
.mss/lock.yaml
AGENTS.md
```

要求：

- 输出稳定；
- 默认 dry-run；
- 路径约束；
- 不写出目标目录；
- 二次执行幂等；
- 不覆盖未知业务文件；
- 标明生成器版本；
- 使用目标真实 Go Module；
- 不包含个人绝对路径或 Foundation checkout 路径；
- 不在 Go 代码中散落拼接大量模板字符串。

### 9.3 扩展项目契约

在 `.mss/project.yaml`、`.mss/lock.yaml` 和 Schema 中加入统一 Admin Distribution 描述。

语义示例：

```yaml
spec:
  distribution:
    name: mss-boot-admin
    version: vX.Y.Z
    backend:
      module: github.com/mss-boot-io/mss-boot-admin/admin
      version: vX.Y.Z
    frontend:
      package: "@mss-boot-io/admin-web"
      version: X.Y.Z
```

字段名根据现有 Schema 风格确定，但必须保证：

- 一个产品版本驱动后端和前端；
- `mss context` 显示发行信息；
- `mss doctor` 检查版本一致性；
- `mss verify` 检查 Thin Host 结构；
- 升级读取当前和目标发行版；
- 不允许无意产生不兼容前后端组合。

### 9.4 AdminModule 支持下游布局

当前 `.mss/modules/*.yaml` 继续作为业务模块机器契约。

生成器不得硬编码：

```text
github.com/mss-boot-io/mss-boot-admin/admin/...
admin/modules/...
web/antd-v6/...
```

必须从目标项目契约解析：

```text
目标 Go Module
业务模块目录
前端目录
生成目录
业务路由文件
```

需要覆盖：

```text
模型
DTO
Service
API
迁移
授权迁移
模块注册
测试
前端类型
API Client
React Query
页面
路由
国际化
必要文档
```

保持：

```text
generated 与 custom 文件分离
两次生成无漂移
Spec 修改后的 sync 可预测
未声明删除不破坏手写代码
```

---

## 十、统一升级

### 10.1 升级完整发行版

新增或调整命令，使下游表达：

```bash
mss upgrade admin vX.Y.Z
```

命令名称可按当前 CLI 风格调整，但用户操作必须是升级一个完整 Admin Distribution，而不是分别升级内部包。

计划输出必须包含：

```text
旧发行版本
新发行版本
Go Admin Module 版本变化
Admin Web Package 版本变化
需要重新生成的模块
需要更新的宿主文件
冲突
保留的业务文件
验证命令
```

### 10.2 保持安全升级语义

继续遵守：

```text
plan 默认只读
apply 要求明确确认
三方合并 Foundation 管理文件
未知业务文件不受管理
业务定制不被静默覆盖
冲突时失败
所有操作成功后才更新基线
```

以后三方升级只管理薄宿主胶水，不管理完整 Admin 核心源码。

### 10.3 不自动猜测业务逻辑

发行升级可以：

```text
更新 go.mod
更新 package.json
更新 lock
重新生成确定性代码
更新 Blueprint 管理文件
```

不得无依据修改非生成业务逻辑。需要 Agent 处理的兼容问题输出结构化冲突和明确待办。

---

## 十一、版本和发布支持

### 11.1 Admin Go Module Tag

为嵌套 `admin/` Module 增加正式 Tag 模板：

```text
admin/vX.Y.Z
```

更新：

```text
.mss/release-policy.yaml
release schema
release policy checker
release source verification
release workflow
文档和测试
```

本任务不得创建真实 Tag。

### 11.2 完整前端包资格

为 `@mss-boot-io/admin-web` 增加：

```text
版本注入
pnpm pack
文件清单检查
完整性校验
SBOM/来源信息
发布工作流支持
```

PR 中使用本地 Tarball 验证，不执行正式发布。

如果 Registry 凭据不存在：

- 完成包和工作流；
- 缺少凭据时失败关闭；
- 不添加秘密；
- 不绕过发布治理；
- 不把 Git URL 当成长期包管理替代品。

### 11.3 单一版本门槛

发布检查必须拒绝后端、前端和根版本核心不一致。

---

## 十二、外部消费者兼容测试

这是关键完成门槛。不能只在 Monorepo Workspace 中验证。

### 12.1 准备本地发行物

前端：

```bash
cd web/antd-v6
corepack pnpm@10.34.5 pack
```

后端外部测试可以通过临时 `replace` 指向当前 checkout。该 `replace` 仅用于测试，不得写入正常下游模板或发布基线。

### 12.2 使用真实 CLI 生成 Thin Host

用真实 `mss new app` 在临时目录生成项目。

必须确认没有复制：

```text
Admin 核心后端源码
mss-boot 源码
完整 web/antd-v6 源码
```

### 12.3 生成 Supplier

在临时项目中使用 AdminModule Spec 生成 Supplier：

```text
后端业务模块
迁移
权限
菜单
前端页面
路由
测试
```

### 12.4 外部后端验证

在临时项目执行：

```bash
GOWORK=off go mod tidy
GOWORK=off go test ./...
GOWORK=off go vet ./...
GOWORK=off go build -o bin/example-admin ./cmd/server
./bin/example-admin --help
```

确认：

- 不依赖 Foundation `go.work`；
- 不依赖未导出的 `internal` 包；
- 不依赖本地绝对路径；
- 一个二进制包含完整 Admin 和 Supplier；
- CLI、迁移和应用组合可用。

### 12.5 外部前端验证

安装本地 Tarball 后执行：

```bash
corepack pnpm@10.34.5 install --frozen-lockfile
corepack pnpm@10.34.5 run lint
corepack pnpm@10.34.5 run test
corepack pnpm@10.34.5 run build
```

确认：

- 完整 Admin 页面存在；
- Supplier 页面存在；
- 只有一个 Umi 应用；
- 只有一个 `dist`；
- 没有远程微应用；
- 核心和业务共用单 Runtime；
- 业务路由位于 fallback 之前；
- Release bundle gate 仍然有效。

### 12.6 E2E

至少一个外部宿主 E2E 覆盖：

```text
登录
加载完整 Admin Shell
进入 Supplier 菜单
Supplier 列表
创建或编辑 Supplier
权限拒绝路径
页面刷新后路由有效
控制台无关键错误或弃用警告
```

使用 SQLite、测试账号和本地依赖，不使用生产凭据。

### 12.7 幂等和边界

验证：

```text
同一 Blueprint 连续生成两次无漂移
同一 AdminModule 连续 sync 两次无漂移
生成器不能写出目标目录
未知业务文件被保留
upgrade plan 不修改文件
冲突时失败
npm 包文件清单不含敏感或本地文件
```

---

## 十三、CI

新增或调整 CI，使以下成为正式门槛：

```text
Admin 自身 GOWORK=off 测试
Admin 与本地 mss-boot Workspace 兼容测试
完整前端自身测试
完整前端 pack 测试
Thin Host 后端消费测试
Thin Host 前端消费测试
生成器幂等测试
升级 plan/apply 测试
外部宿主构建
关键 E2E
```

不得削弱当前 Required Checks。

合理使用 path filter 和缓存。修改以下区域时必须触发相应消费者测试：

```text
admin/**
web/antd-v6/**
cmd/mss/**
internal/mss/**
templates/**
.mss/**
release tooling
相关 workflow
```

固定 Action 版本或 SHA，并遵守仓库安全规则。

---

## 十四、文档和机器契约

设计文档已存在：

```text
docs/docs/architecture/complete-admin-distribution-and-thin-business-host.zh-CN.md
```

实施过程中应更新它，使其与最终代码一致；不要创建内容重复的另一份长期架构文档。

如仓库治理需要正式决策记录，可新增或更新 ADR，例如：

```text
docs/adr/2026-08-19-complete-admin-distribution-and-thin-business-host.md
```

ADR 必须说明：

```text
背景
完整 Admin 边界
Thin Host 边界
为什么不用微服务/微应用
为什么只发布一个前端包
统一版本
业务模块注册
Blueprint 和升级语义
安全和兼容性
发布与回滚
被否决方案
```

同步更新：

```text
README.md
README.zh-CN.md
MONOREPO.md
AGENTS.md
admin/AGENTS.md
web/antd-v6/AGENTS.md
.mss/project.yaml
.mss/capabilities.yaml
.mss/commands.yaml
.mss/lock.yaml
Blueprint 和相关 Schema
AdminModule 示例
相关 Skills
用户文档
```

机器执行的版本、路径、命令和约束必须进入 `.mss/`，不能只写在 prose 文档中。

---

## 十五、明确禁止事项

本任务不得：

```text
拆分 Admin 微服务
引入 Qiankun 或 Module Federation
创建第二套前端应用
创建多个独立 Admin npm 核心包
把核心 Admin 源码复制到业务项目
让业务项目维护 mss-boot 源码
实现 Go 动态插件
恢复运行时动态模型或浏览器代码生成
引入遥测或 import 记录
获取用户、仓库或设备数据
降低 HttpOnly Session、CSRF 或 WebSocket 安全标准
允许前端权限替代后端权限
将 Admin 业务代码放入 mss-boot
跳过迁移 readiness
降低现有测试、bundle 或发布门槛
提交凭据、Token、DSN 或私有地址
创建正式 Tag、Release 或生产包
```

不要为了显得通用而增加当前没有实际使用者的抽象。

---

## 十六、实施顺序

### 阶段 1：契约和 ADR

完成：

```text
Feature Contract 或 ADR
机器契约目标
公共 API 决策
外部消费者验收设计
```

提交并推送，不要停止。

### 阶段 2：后端纵向闭环

完成：

```text
可导入完整 Admin API
最小 BusinessModule API
官方 main 复用
Supplier 通用注册
外部 GOWORK=off 编译测试
```

提交、推送、修复测试。

### 阶段 3：前端纵向闭环

完成：

```text
单一完整 npm 包
Umi 集成和 CLI
业务路由注入
外部 Tarball 构建
单 Runtime 检查
```

提交、推送、修复测试。

### 阶段 4：Thin Host Blueprint

完成：

```text
应用模板
mss new app 轻量输出
AdminModule 下游路径生成
project/lock/schema 更新
```

提交、推送、验证幂等。

### 阶段 5：统一升级

完成：

```text
统一发行版本
后端和前端同步升级
薄宿主三方合并
冲突和保留语义
```

提交、推送、运行升级测试。

### 阶段 6：CI、发布支持和文档

完成：

```text
admin 子模块 Tag 支持
前端 pack/publish 资格
外部消费者 Workflow
长期文档和 Skills
```

提交、推送。

### 阶段 7：完整验证和修复

执行全部相关验证。发现问题直接修复、提交和推送，不要只把失败写入报告。

---

## 十七、必须实际运行的验证

先聚焦测试，再完整测试。

### 根目录

```bash
go run ./cmd/mss context --format json
go run ./cmd/mss doctor --strict --format json
go run ./cmd/mss verify --changed
```

完成前：

```bash
go run ./cmd/mss verify --all
make deps-all
make test-all
```

### Admin

从 `admin/` 执行：

```bash
GOWORK=off go mod tidy
git diff --exit-code -- go.mod go.sum
GOWORK=off go test ./...
GOWORK=off go test -race ./...
GOWORK=off go vet ./...
GOWORK=off go build -o ../bin/mss-boot-admin .
../bin/mss-boot-admin --help
```

再运行 Workspace 模式兼容测试。

### mss-boot

只有修改 `mss-boot/` 时执行：

```bash
cd mss-boot
GOWORK=off go test ./...
GOWORK=off go vet ./...
```

原则上本任务不应为了 Admin 扩展向 `mss-boot` 添加领域能力。

### 前端

从 `web/antd-v6/` 执行：

```bash
corepack pnpm@10.34.5 install --frozen-lockfile
corepack pnpm@10.34.5 run deps:check
corepack pnpm@10.34.5 run lint
corepack pnpm@10.34.5 run test:ci
corepack pnpm@10.34.5 run build:release
corepack pnpm@10.34.5 run delivery:smoke
corepack pnpm@10.34.5 pack
```

### 文档

```bash
corepack pnpm@9.15.9 --dir docs install --frozen-lockfile
corepack pnpm@9.15.9 --dir docs build
```

### 外部消费者

运行新增的完整消费者验证脚本或 Workflow 对应命令，并保留脱敏结果。

CI 中的外部浏览器回归继续由 Playwright 串行执行并生成脱敏报告。本地人工 UI
验收直接使用 Codex 内置浏览器连接标准开发服务，至少检查登录、Admin Shell、菜单与
API 绑定、Supplier 页面、硬刷新、403 和控制台健康；不要用本地 Playwright 结果替代
这份可见验收，也不要因此削弱 CI 的自动化门槛。

不要声称执行了未实际运行的命令。

---

## 十八、完成验收标准

只有同时满足以下条件，任务才完成：

1. `admin` 仍是一套完整产品，没有拆分基础功能。
2. 外部 Go 项目可通过版本化 `admin` Module 引用完整后端。
3. 外部项目可显式注册自己的编译期业务模块。
4. Supplier 不再通过服务器专用硬编码挂载。
5. 官方 Admin 参考应用继续完整运行。
6. `web/antd-v6` 仍是唯一正式前端源码。
7. 完整前端可作为一个 npm Tarball 被仓库外项目安装。
8. 外部前端只构建一个 Umi 应用和一个 `dist`。
9. 核心页面与业务页面共用一份 React、AntD 和 Query Runtime。
10. `mss new app` 默认生成 Thin Host，不复制 Admin 核心源码。
11. AdminModule 生成器能根据目标项目布局生成完整业务前后端代码。
12. 一个发行版本可以同步升级后端和前端。
13. Blueprint 升级只管理薄宿主并保留业务代码。
14. 外部 Go 消费者在 `GOWORK=off` 下测试和构建通过。
15. 外部前端通过本地 `pnpm pack` Tarball 构建通过。
16. 至少一个完整外部业务模块 E2E 通过。
17. 生成器和升级通过幂等、路径约束和冲突测试。
18. 当前 Admin 后端和 V6 前端回归测试通过。
19. CI 未被削弱，外部消费成为持续验证门槛。
20. 不存在遥测、import 记录或用户数据采集。
21. 没有创建正式 Tag、Release 或生产包。
22. 所有代码、契约、生成输出、文档和 Workflow 已提交并推送。
23. PR 已创建，描述完整，CI 状态明确。
24. 不存在关键 TODO、空实现或未接入正式路径的功能。

---

## 十九、Pull Request

PR 标题建议：

```text
feat: deliver complete Admin distribution with thin business hosts
```

PR 描述必须包含：

```text
背景和问题
最终架构
后端完整发行方式
前端单包方式
业务模块组合方式
Blueprint 变化
升级变化
版本和发布变化
外部消费者证据
兼容性影响
安全影响
数据库迁移影响
实际运行的命令和结果
未运行检查及具体原因
提交列表
回滚方式
```

附上：

```text
最新 base SHA
最终 head SHA
分支名
PR URL
CI 状态或 URL
外部消费者目录树
pnpm pack 文件清单摘要
GOWORK=off 验证摘要
单 Runtime 验证摘要
E2E 结果
```

---

## 二十、最终汇报格式

完成后汇报：

```text
1. 最终结论
2. 架构实现摘要
3. 关键目录和公共 API
4. Thin Host 的实际使用示例
5. Blueprint 和升级行为
6. 提交列表
7. 已推送分支
8. PR 地址
9. 实际运行的测试及结果
10. CI 状态
11. 兼容性和迁移影响
12. 安全影响
13. 已知限制
14. 下一条可执行操作
```

不要以“建议以后继续实现”结束。请持续推进，直到架构、消费者验证、CI、文档和 PR 全部落地。
