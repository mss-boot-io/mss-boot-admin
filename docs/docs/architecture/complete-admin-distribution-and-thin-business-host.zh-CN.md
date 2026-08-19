---
title: 完整 Admin 发行包与轻量业务宿主
order: 2
nav:
  title: 架构
  order: 2
description: mss-boot-admin 将完整管理系统作为统一发行包，由独立业务仓库通过编译期扩展生成单一后端和单一前端的已实现架构与使用合同
keywords: [complete admin distribution thin business host go module npm umi codex agent]
---

# 完整 Admin 发行包与轻量业务宿主

## 1. 文档状态

- 设计状态：已接受；`v1.3.0` 实现与本地资格验证已完成，待 PR 合并和远端 CI
- 设计日期：2026-08-19
- 设计基线：`main@9a256229774bb255dfe8a618613522fd70538195`
- 实现分支：`agent/complete-admin-distribution-plan`
- 目标发行：`Admin Distribution v1.3.0`
- 目标仓库：`mss-boot-io/mss-boot-admin`
- 配套实施提示词：`docs/aigc/prompts/complete-admin-distribution-implementation-2026-08-19.zh-CN.md`
- 机器实施契约：`.mss/features/complete-admin-distribution-thin-host.yaml`
- 架构决策记录：`docs/adr/2026-08-19-complete-admin-distribution-and-thin-business-host.md`

本文定义 `mss-boot-admin` 在下游业务代码隔离、完整产品交付、版本升级和 AI Agent 开发方面的长期目标架构。

本文是架构实施的事实来源之一。代码实现、`.mss/` 机器契约、测试和后续正式 ADR 如与本文发生冲突，应按照仓库 `AGENTS.md` 规定的事实源优先级处理，并同步更新本文，避免文档与实现长期分叉。

## 2. 背景与问题

`mss-boot-admin` 既需要作为开源项目持续迭代，也需要被真实开发者用于构建订单、DevOps、云存储、AI、运营和其他业务系统。

如果业务开发者直接 Fork 或复制完整仓库并在其中修改核心代码，会逐步产生以下问题：

1. 开源仓库升级与业务修改混在同一批文件中，升级冲突不断增加。
2. 用户、权限、Session、菜单、配置、通知和前端 Shell 等核心能力会被每个业务仓库复制并分别演化。
3. 开源版本修复安全问题后，很难稳定同步到已经深度修改的业务系统。
4. Coding Agent 无法清晰判断哪些文件属于基础设施，哪些文件属于业务实现。
5. Blueprint 三方合并需要管理越来越多核心源文件，升级成本最终接近重新合并一个 Fork。
6. 为解决源码隔离而直接采用微服务和微应用，会把一个代码所有权问题转化为分布式系统、独立部署、跨应用认证和版本治理问题。

当前仓库已经把这些基础收口为正式实现：

- `admin/` 是独立 Go Module；
- `mss-boot/` 是领域中立的框架 Module；
- `web/antd-v6/` 是唯一正式 Admin 前端；
- `AdminModule` 规格可以生成后端、前端、迁移、权限、菜单和测试；
- `mss new app`、Blueprint 和三方升级机制已经存在；
- Supplier 已经通过统一 `business.Module` 合同显式组合，并作为前后端生成与外部消费黄金样例；
- `admin/app` 提供可导入的完整应用生命周期；
- `web/antd-v6` 同时是官方应用和可打包的 `@mss-boot-io/admin-web`；
- `management-system` Blueprint 默认生成 Thin Host，不再复制完整 Foundation 源码；
- `mss upgrade admin`、发行策略和外部消费者流水线共同约束协调版本。

## 3. 核心目标

目标不是把 Admin 拆得更细，而是建立一条单一、稳定、低维护成本的开发路径：

> `mss-boot-admin` 作为完整管理系统发行包统一开发和发布；真实业务代码位于独立仓库，通过后端 Go Module 和前端单一 npm 包在编译期接入，最终仍然构建和部署为一个后端、一个前端和一个逻辑应用。

最终交付关系如下：

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

一个后端二进制 + 一个前端 dist
=
一个权限体系、一个登录协议、一个部署单元
```

该架构优先服务：

1. 仓库作者自己的真实业务开发；
2. Codex、Claude Code 和其他 Coding Agent；
3. 希望使用完整 Admin 能力、但不愿长期维护 Fork 的开发者。

不以满足所有前端框架、所有部署模型或构建通用插件市场为目标。

## 4. 非目标

本阶段明确不实现：

- 将用户、角色、菜单、配置、通知等 Admin 核心功能拆成独立服务；
- 为业务源码隔离引入微服务；
- Qiankun、Module Federation、iframe 或远程前端 Entry；
- 第二套长期维护的 Admin SPA；
- 多个独立版本的 Admin Runtime、Shell、Auth、Layout npm 包；
- Go 动态插件、`.so`、运行时下载或执行业务代码；
- 让下游自由裁剪核心用户、权限、Session 或菜单能力；
- 恢复运行时动态模型、虚拟 CRUD 或浏览器代码生成；
- import 使用者记录、采用者登记、遥测、运行时 call-home 或用户数据收集；
- 为未来可能存在、但当前没有实际需求的扩展点设计庞大生命周期框架。

业务本身确实需要独立扩缩容的 AI 推理、Worker、视频处理或任务执行服务，可以作为外部业务服务存在，但不属于本次“业务代码与 Admin 核心源码隔离”的解决方案。

## 5. 架构决策摘要

### 5.1 完整 Admin 统一发行

当前 Admin 的全部基础功能继续作为一个完整产品共同开发、测试和发布：

- 用户、角色、部门、岗位；
- 菜单、API 和 Casbin 权限；
- 登录、Session、OAuth、CSRF 和 WebSocket；
- 配置、审计、通知、任务、存储；
- 监控、统计和当前其他正式功能；
- 完整前端布局、登录页、核心页面和公共运行时。

允许增加 `admin/app`、`admin/business` 等少量公共入口，它们只用于暴露完整应用启动和业务扩展能力，不代表拆分产品。

### 5.2 业务仓库保持轻量

真实业务项目只保存：

- 很薄的 Go 启动入口；
- 业务后端模块；
- 业务前端页面；
- 业务规格、迁移和测试；
- 项目配置、部署和 CI；
- `.mss/` 项目锁和升级基线。

业务仓库不复制并长期维护 Admin 核心源码或 `mss-boot` 源码。

### 5.3 编译期组合

- 后端：Go 编译期组合完整 Admin 和业务模块；
- 前端：Umi 构建期组合完整 Admin 和业务页面；
- 输出：一个后端二进制、一个前端 `dist`；
- 运行时：不存在第二个 Admin 应用或远程业务代码加载器。

### 5.4 单一发行版本

后端 Go Module、前端 npm 包和根发行版在产品层面使用同一个 `Admin Distribution` 版本。技术制品可以使用不同 Tag 命名空间，但版本核心必须一致。

## 6. 术语

### 6.1 Admin Distribution

完整的 `mss-boot-admin` 产品发行，包含：

- 完整 Admin Go Module；
- 完整 Admin Web npm 包；
- 对应的 `mss-boot` 兼容版本；
- 机器契约、生成器和升级规则；
- 对应测试和发布证据。

### 6.2 Thin Business Host

轻量业务宿主。它是一个独立业务仓库，通过版本化依赖引入完整 Admin，并声明和实现自己的业务模块。

Thin Host 不是第二套 Admin，也不是传统意义上的深度 Fork。

### 6.3 Business Module

业务仓库中的编译期扩展单元。一个完整模块通常包含：

- AdminModule 规格；
- 数据模型和迁移；
- Service 和 API；
- 权限和菜单；
- 前端页面、路由和国际化；
- 测试和必要文档。

## 7. 目标拓扑

```text
mss-boot-io/mss-boot-admin
├── mss-boot/                    # 领域中立 Go 框架
├── admin/                       # 完整 Admin Go Module 与参考入口
├── web/antd-v6/                 # 唯一完整 Admin Web 源码与 npm 包
├── cmd/mss/                     # 创建、生成、验证和升级 CLI
├── internal/mss/                # CLI 实现
├── templates/                   # 轻量宿主和业务模块模板
├── .mss/                        # Foundation 机器契约
└── docs/
         │
         │ Admin Distribution vX.Y.Z
         ▼
acme/example-business-admin
├── cmd/server/main.go           # 很薄的组合入口
├── internal/modules/            # 业务后端模块
├── web/src/business/            # 业务前端页面
├── .mss/modules/                # AdminModule 规格
├── .mss/features/               # Feature 规格
├── config/
├── deployments/
└── CI / Dockerfile / Makefile
```

## 8. 后端设计

### 8.1 完整应用可导入入口

同一个 `admin` Go Module 已在 `admin/app` 暴露可导入的完整应用 API，官方入口与下游入口共用它：

```text
admin/app/
```

下游使用方式示意：

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

公开入口已经确定为 `app.New`、`app.ExecuteContext`、
`app.WithBusinessModules` 和可测试的 `Application.ExecuteArgsContext`，并满足：

1. 库代码不调用 `os.Exit`；
2. 错误返回给调用者；
3. 接受 `context.Context`；
4. 保留 `server`、`migrate` 和现有参数；
5. 官方参考程序和外部宿主共用唯一启动实现；
6. 内置 Admin 功能默认全部启用；
7. 测试可以隔离构造，避免全局状态污染；
8. 配置、数据库、队列和关闭生命周期仍由完整 Admin 统一拥有。

### 8.2 最小业务模块接口

业务代码只获得一个小而稳定的公共扩展面。实际接口为：

```go
type Module interface {
    Name() string
    Register(*Registry) error
}
```

`Module.Register` 提交一个包含 `Descriptor`、`Migrations`、`Readiness` 和
`Routes` 的完整 `business.Registration`。组合完成后 Registry 冻结；迁移执行器按
Application 克隆，避免外部宿主和重复测试污染进程级状态。

`Registry` 首版只暴露当前业务生成链路实际需要的能力：

- 模块身份；
- 数据库迁移与授权迁移；
- 权限描述；
- 菜单描述；
- 认证和权限中间件之后的业务 API 路由；
- 当前生成模块确实使用的领域事件接入。

约束：

- 显式组合优先，避免依赖不可见的全局 `init()` 副作用；
- 注册顺序稳定；
- 重复模块名失败；
- 注册错误必须返回；
- 业务模块不能替换核心认证、Session、权限、安全中间件或配置所有权；
- 路由在迁移 readiness 通过后才对外可用；
- 公共接口不得暴露 `internal` 包；
- 不把 Admin 领域能力移动到 `mss-boot/`。

### 8.3 Supplier 作为黄金样例

Supplier 已从服务器专用挂载迁移到统一业务模块接口：

- Supplier 不再被 `server.go` 特殊识别；
- Supplier 使用与外部业务模块相同的注册机制；
- 请求级数据库租约、迁移 readiness、权限正反向测试、CRUD、导出和事件行为保持不变；
- Supplier 继续作为生成器、兼容性和 E2E Fixture；
- Supplier 不形成第二套长期维护的 Admin 产品。

### 8.4 Go Module 发布

`admin/` 是嵌套 Go Module，正式发布使用：

```text
admin/vX.Y.Z
```

下游正常依赖：

```go
require github.com/mss-boot-io/mss-boot-admin/admin vX.Y.Z
```

仓库内部测试可以使用临时 `replace` 指向当前 checkout，但发布模板和下游基线不得依赖本地路径。

## 9. 前端设计

### 9.1 唯一完整 npm 包

`web/antd-v6` 继续是：

1. 唯一正式前端源码；
2. 官方参考应用；
3. 下游可消费的完整 Admin Web 包。

正式包名：

```text
@mss-boot-io/admin-web
```

不得拆成多个独立版本包。允许通过同一包的 `exports` 提供不同入口，例如：

```text
@mss-boot-io/admin-web/preset
@mss-boot-io/admin-web/runtime
@mss-boot-io/admin-web/business
@mss-boot-io/admin-web/testing
```

这些入口共享同一个 `package.json`、版本、源码、锁文件和发布周期。

### 9.2 Umi 构建期集成

完整包提供 Umi Preset、Plugin 和配置工厂。Umi 实际读取 Thin Host 的
`web/config/config.ts`；`web/mss-admin.config.ts` 只是对该文件的兼容转发：

```ts
import { defineBusinessAdmin } from '@mss-boot-io/admin-web/business';
import businessRoutes from './business-routes.generated';

export default defineBusinessAdmin({
  businessRoutes,
  routeRegistrations: './src/generated/routes.ts',
  useUtoopack: true,
});
```

集成必须自动提供：

- 完整 Admin 基础路由；
- Session、CSRF 和 Request；
- React Query；
- Theme 和设计系统；
- 国际化；
- WebSocket；
- Layout、登录和核心页面；
- 403、404 和全局错误边界。

业务路由与菜单注册必须成对注入、插入最终 fallback 之前，并在同一次 Umi 构建中编译。
路由校验按模式 fail closed：完全重复、核心静态路径、核心动态参数路径、核心 wildcard
和互相重叠的业务模式都在启动或构建前失败，不能依赖路由声明顺序掩盖冲突。

Thin Host 开发构建明确禁用已退役的 MFSU，并在 `useUtoopack: true` 时使用
Utoopack；发行构建继续走受控的 release 配置和 runtime/bundle/API 门槛。

### 9.3 单 Runtime 约束

外部消费场景必须验证只有一份：

```text
react
react-dom
antd
@ant-design/pro-components
@tanstack/react-query
Umi runtime
```

依赖布局需要根据真实 pnpm 安装树和构建产物设计，不能只依据顶层 `package.json` 推断。
发布合同通过 `mssAdminDistribution` 固化 `pnpm@10.34.5` 以及 React、React DOM、
Ant Design、ProComponents、React Query 和 Axios 的六项 override。Vitest 4 的构建期
peer 由 Thin Host 与发行包共同精确锁定到 Vite `8.2.1`，不进入浏览器 Runtime。

同一合同还必须把发布包的顶层 `dependencies` 完整、互斥地划分为 `runtime` 和
`tooling`。安全审计按每条 pnpm 依赖路径的顶层来源判定边界：任何高危或严重问题只要
可由 `runtime` 到达就无条件阻断；仅由 CLI、编译器或测试工具到达的问题，必须同时满足
精确 advisory、精确受影响版本、到期日期和原因记录。`buildOnlyDependencies` 固化这些
旧解析版本，产物统计只拒绝对应版本进入浏览器图，因此不会误伤同名的安全新版 Runtime。

### 9.4 单一前端产物

最终下游只运行：

```text
一个开发服务器
一个 Umi 应用
一个路由树
一个 dist
```

不存在远程 Entry、微应用容器或另一套 Session。

### 9.5 下游命令

优先由同一个 npm 包提供统一命令：

```text
mss-admin-web dev
mss-admin-web lint
mss-admin-web test
mss-admin-web build
```

下游 `package.json` 只声明完整 Admin Web 包和业务真正需要的额外依赖，不复制 Foundation 的全部依赖管理逻辑。

## 10. 轻量业务宿主结构

默认 `mss new app` 目标结构：

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
│       └── <module>/
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

下游不得包含完整的：

```text
admin/models
admin/service
admin/router
admin/middleware
admin/center
web/antd-v6/src/shared
Admin 核心页面源码
mss-boot 源码
Foundation 文档和发布流水线
```

## 11. Blueprint 与生成器

### 11.1 Blueprint 从完整复制转为薄宿主

`management-system` 已成为唯一推荐的新应用 Blueprint，并通过确定性模板生成轻量宿主。

Blueprint 负责：

- `main.go` 和应用配置入口；
- Go/npm 依赖声明；
- Dockerfile、Makefile 和业务 CI；
- `.mss/project.yaml`、Lock 和 Manifest；
- 业务目录和生成注册文件；
- 少量项目级胶水。

Blueprint 不再负责复制：

- Admin 核心后端实现；
- 完整 Admin 前端实现；
- `mss-boot` 源码；
- Foundation 自身发布和文档工程。

### 11.2 应用模板

确定性应用模板位于：

```text
templates/application/
```

模板必须满足：

- dry-run；
- 路径限制；
- 输出稳定；
- 二次执行幂等；
- 不覆盖未知业务文件；
- 不泄漏本地绝对路径；
- 标明生成器版本；
- 使用目标项目真实 Module 和仓库身份。

### 11.3 AdminModule 支持目标布局

生成器必须从目标 `.mss/project.yaml` 解析：

- 目标 Go Module；
- 业务模块目录；
- 前端目录；
- 生成目录；
- 业务路由文件。

不得继续硬编码 Foundation 仓库路径。

同一份 AdminModule 规格在 Foundation 参考应用和外部 Thin Host 中都应生成正确路径，并保持后端、前端、迁移、权限、菜单、测试和文档同属一个交付单元。

## 12. 机器契约

`.mss/project.yaml` 和 `.mss/lock.yaml` 已表达统一 Admin Distribution，例如：

```yaml
spec:
  distribution:
    name: mss-boot-admin
    version: v1.3.0
    backend:
      module: github.com/mss-boot-io/mss-boot-admin/admin
      version: v1.3.0
    frontend:
      package: "@mss-boot-io/admin-web"
      version: 1.3.0
```

最终字段名以 Schema 风格为准，但必须支持：

- 一个产品版本驱动后端和前端；
- `mss context` 显示版本和来源；
- `mss doctor` 检查版本一致性和项目边界；
- `mss verify` 检查 Thin Host 结构和生成漂移；
- 升级工具读取当前基线和目标发行版；
- 不允许静默产生不兼容的前后端版本组合。

## 13. 统一版本与发布

产品版本：

```text
Admin Distribution vX.Y.Z
```

技术 Tag 可以是：

```text
vX.Y.Z
mss-boot/vX.Y.Z
admin/vX.Y.Z
web/antd-v6/vX.Y.Z
```

版本核心必须一致。发布检查应拒绝：

```text
root v1.3.0
admin v1.2.0
frontend 1.4.0
```

正式发布按同一 merged-main SHA 严格执行：Framework → Admin → Frontend → Root。
每一步验证前置制品的 Tag、GitHub Release、解析版本与源提交，Root 最后核对全部协调制品；
Docs 保持独立发布单元，不阻塞 Admin Distribution。流程分别产生 Go Module、完整 npm 包、
镜像和证明材料，但对下游只暴露一个协调发行版本。

## 14. 升级设计

用户操作应表达升级完整发行包，例如：

```bash
# 默认只生成只读计划；目标 Foundation checkout 必须声明同一个 Distribution 版本
mss upgrade admin v1.4.0 --foundation /path/to/mss-boot-admin-v1.4.0 --format json

# 解决完计划中的全部冲突后，显式确认应用
mss upgrade admin v1.4.0 --foundation /path/to/mss-boot-admin-v1.4.0 --apply --yes
```

目标 Foundation Blueprint 的 `spec.distribution.version` 和发行策略版本必须
与命令请求的版本完全一致，否则计划失败且不写文件。计划同时列出旧/新 Distribution、
Go Admin Module 和 Admin Web package 的变化、需要重新生成的 AdminModule、
受管宿主文件变化、冲突、保留文件及验证命令。内部仍使用同一个 Blueprint
三方升级引擎；旧的 `mss upgrade plan/apply --foundation ...` 入口继续兼容。

命令名称可根据现有 CLI 调整，但必须一次协调：

- Go Admin Module 版本；
- Admin Web npm 版本；
- `.mss/lock.yaml`；
- 必须重新生成的业务胶水；
- Blueprint 管理的薄宿主文件；
- 兼容性检查和验证命令。

升级继续遵守现有安全语义：

1. `plan` 默认只读；
2. `apply` 需要明确确认；
3. 三方合并只管理 Foundation 管理的薄宿主文件；
4. 未知业务文件保持不变；
5. 业务和 Foundation 同时修改时产生冲突；
6. 所有操作成功后才更新基线；
7. 不猜测性重写未生成的业务逻辑。

## 15. AI Agent 开发闭环

标准流程：

```text
用户描述业务需求
  ↓
Agent 读取 AGENTS.md、project.yaml 和 capabilities
  ↓
生成或更新 Feature / AdminModule 规格
  ↓
mss 校验并生成业务后端、前端、迁移、权限和测试
  ↓
Agent 只在业务编辑区实现非模板逻辑
  ↓
mss verify 执行变更感知验证
  ↓
提交业务仓库 PR
  ↓
需要升级时，升级一个 Admin Distribution 版本
```

建议在 Thin Host 中明确编辑边界：

```text
Agent 可以编辑：
- internal/modules/** 中非 generated 文件
- web/src/business/**
- .mss/modules/**
- .mss/features/**
- 项目配置和部署目录

Agent 不应编辑：
- Go Module Cache 中的 Admin 源码
- node_modules 中的 Admin Web 源码
- generated 文件
- 核心认证和权限实现
```

## 16. 安全和兼容性

### 16.1 安全边界

- 浏览器继续使用当前 HttpOnly Session、CSRF 和 WebSocket 安全协议；
- 业务页面不得获得 Admin Bearer Token；
- 后端权限始终是权威来源；
- 业务路由必须经过统一认证和权限中间件；
- npm 包不得包含本地日志、Token、凭据、报告或缓存；
- 生成器不得写出目标目录；
- 本架构不增加遥测或联网登记。

### 16.2 兼容性边界

稳定契约优先包括：

- Admin Distribution 版本；
- 完整应用启动入口；
- 最小 BusinessModule 接口；
- AdminModule Schema；
- 前端公开 exports；
- 业务路由和页面接入点；
- `.mss` 项目、Lock 和升级语义。

内部实现可以持续演进。面向 AI Agent 的下游不要求无限期维持所有内部包路径不变，但每次不兼容调整必须有结构化升级计划、生成器或明确冲突输出。

## 17. 实施阶段

截至 `agent/complete-admin-distribution-plan`，阶段一至阶段六的代码、机器契约、模板、
升级入口、发行工作流和长期文档均已落地。仓库外 Supplier Thin Host 已通过生成、二次
幂等、`GOWORK=off` 后端、Tarball 安装、lint、test、单一生产 `dist`/Runtime 和浏览器
E2E。标准本地 Admin 另外通过 Codex 内置浏览器完成登录、工作台、菜单绑定 API、
Supplier、硬刷新、403、深色主题和控制台健康检查。

下一条可执行步骤是把当前分支通过 Pull Request 合并到 `main`，等待远端必需检查全部
成功；本功能分支不创建 Tag、GitHub Release、镜像或 npm 公共包。

### 阶段一：设计与契约

- 落盘本设计；
- 建立 Feature Contract 或 ADR；
- 更新 `.mss` 目标能力和验收；
- 定义外部消费者测试。

### 阶段二：后端闭环

- 增加可导入完整 Admin API；
- 增加最小 BusinessModule API；
- 官方参考入口复用同一实现；
- Supplier 迁移到统一注册；
- 仓库外 `GOWORK=off` 消费测试通过。

### 阶段三：前端闭环

- `web/antd-v6` 形成单一完整 npm 包；
- 提供同包 Umi 集成和命令；
- 业务路由构建期注入；
- 本地 Tarball 在仓库外安装和构建；
- 验证单 React/AntD/Query Runtime。

### 阶段四：Thin Host Blueprint

- 增加应用模板；
- `mss new app` 默认输出轻量宿主；
- AdminModule 支持下游布局；
- 两次生成无漂移。

### 阶段五：统一升级

- 引入统一发行版本；
- 同步升级 Go 和 npm 依赖；
- 三方合并只管理薄宿主；
- 冲突、保留和回滚测试通过。

### 阶段六：CI、发布和文档

- 增加 `admin/vX.Y.Z` 支持；
- 增加完整 npm 包的 pack/publish 资格检查；
- 外部消费者成为 CI 门槛；
- 更新使用、升级和 Agent 开发文档。

## 18. 验收标准

任务完成必须同时满足：

1. Admin 仍是一套完整产品，没有拆分基础功能。
2. 外部 Go 项目可以通过版本化 `admin` Module 引用完整后端。
3. 外部项目可以显式注册编译期业务模块。
4. Supplier 不再由服务器专用代码硬编码挂载。
5. 官方参考 Admin 入口继续完整运行。
6. `web/antd-v6` 仍是唯一正式前端源码。
7. 完整前端可以作为一个 npm Tarball 在仓库外安装。
8. 外部前端只产生一个 Umi 应用和一个 `dist`。
9. 完整 Admin 和业务页面共用一份 React、AntD 和 Query Runtime。
10. `mss new app` 默认生成 Thin Host，不复制核心源码。
11. AdminModule 能根据目标布局生成完整业务前后端代码。
12. 一个发行版本可以同步升级后端和前端。
13. Blueprint 升级只管理薄宿主并保留业务代码。
14. 外部后端在 `GOWORK=off` 下测试和构建通过。
15. 外部前端通过本地 `pnpm pack` Tarball 构建通过。
16. 至少一个外部业务模块 E2E 覆盖登录、菜单、CRUD 和权限拒绝。
17. 生成器和升级通过幂等、路径限制和冲突测试。
18. 当前后端、前端、发布和安全回归门槛不降低。
19. CI 正式验证外部消费者，而不只验证 Monorepo Workspace。
20. 不存在遥测、import 记录或用户数据采集。

## 19. 主要风险与控制

### 19.1 前端包化导致依赖重复

控制方式：使用真实 Tarball、仓库外安装树和构建 Runtime 分析作为门槛，不依赖理论推断。

### 19.2 后端全局状态影响外部测试

控制方式：提取可返回错误的应用组合入口，提供测试隔离和显式模块注册，逐步限制不可重置全局状态。

### 19.3 Blueprint 改造破坏已有下游

控制方式：识别旧 Manifest 版本，提供迁移计划和冲突输出，不静默把旧完整复制项目改成 Thin Host。

### 19.4 版本制品数量增加

Go Module、npm 包和镜像是不同技术制品，但使用统一版本、统一资格检查和协调发布，避免演变为多套产品。

### 19.5 公共扩展 API 过度设计

只暴露 Supplier 和当前生成器真正需要的最小能力。新增扩展点必须由真实业务需求和测试驱动。

## 20. 最终决策

`mss-boot-admin` 后续采用以下唯一推荐架构：

```text
完整 Admin 统一开发与发布
+
独立业务仓库
+
后端 Go 编译期业务模块
+
前端 Umi 构建期业务页面
+
Thin Host Blueprint
+
统一 Admin Distribution 升级
```

微服务和微应用不用于解决源码隔离问题。业务项目不再长期维护完整 Admin Fork。架构的成功标准不是抽象数量，而是作者和 Coding Agent 能够使用一条确定性路径创建、开发、验证和升级真实业务系统，同时始终只维护一套 Admin 产品。
