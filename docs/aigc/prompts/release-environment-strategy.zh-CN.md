# mss-boot-io 发布环境与前后端发布策略

## 环境定位

| 环境 | Kubernetes namespace | 前端域名 | API 域名 | 定位 |
| --- | --- | --- | --- | --- |
| alpha / dev | devops 集群内 `mss-boot-dev` | `admin-alpha.mss-boot-io.top` | `admin-api-alpha.mss-boot-io.top` | 内部开发、联调、本地前端对接 |
| beta | devops 集群内 `mss-boot-beta` | `admin-beta.mss-boot-io.top` | `admin-api-beta.mss-boot-io.top` | 对外展示、公开预发布、必须冒烟通过 |
| prod | `mss-boot-prod` | `admin.mss-boot-io.top` | 待正式部署确认，推荐 `admin-api.mss-boot-io.top` | 正式生产环境，后端使用 tag 版本 |

说明：

- alpha/dev 与 beta 后端均部署在同一个 devops 集群，不能再用 `~/.kube/baas.yaml` 推断 beta 后端状态。
- `mss-boot-dev` 后续归类为开发/alpha 环境，不再承载对外展示 beta 职责。
- 本地启动的前端默认对接 alpha 后端域名，用于日常开发联调。
- `admin-beta.mss-boot-io.top` 是 Cloudflare 上的对外展示前端，必须对接 `admin-api-beta.mss-boot-io.top`。
- `admin.mss-boot-io.top` 是生产前端，对应集群内新部署的 `mss-boot-prod` 后端。

## 数据与 Redis 策略

- `mss-boot-beta` 的配置可复制 `mss-boot-dev` 的形态，但数据库和 Redis 只复用 dev 的实例，不复用 dev 的业务库/数据空间。
- beta 必须使用独立数据库、独立 Redis DB index 或独立 key prefix；实际选择在部署时确认并写入发布记录。
- beta 和 dev 即使复用实例，也应使用独立账号、独立 Secret、最小权限和明确命名，避免误连或权限扩散。
- prod 必须独立部署数据库、Redis、Secret、对象存储、OAuth 回调和外部集成配置，不复用 dev/beta 资源。

## 后端发布策略

- 后端可以较高频发布到 alpha/dev 和 beta，用于快速验证与展示前准备。
- 后端发布仍必须保留可追溯信息：分支、commit SHA、镜像 tag/digest、GitHub Actions run、目标 namespace 和回滚镜像。
- beta 后端可通过 `codex/**` 分支发布镜像并更新 `mss-boot-beta`，但应优先使用 commit SHA tag 或 digest，而不是可变分支状态。
- prod 后端只部署打 tag 的版本；tag 应对应不可变镜像，并记录 changelog、配置差异和回滚方案。

## 前端发布策略

- 前端不能高频发布到 Cloudflare；Cloudflare 发布次数有限，必须等到功能可以对外公开后再发布。
- 前端发布流程保持不变：先本地开发、构建、联调和浏览器验证，通过后再发布 Cloudflare。
- beta 前端发布前必须确认：
  - 后端 beta API 已部署并健康；
  - 本地前端已对接目标 API 完成验证；
  - 对外展示所需功能完整，不发布半成品；
  - 登录、菜单、权限、主要 CRUD、配置页和关键业务链路冒烟通过；
  - 无阻断级缺陷；
  - Release/Leader 明确确认可以对外展示。
- prod 前端发布前必须确认后端 tag 版本、数据库迁移、生产配置和回滚方案已就绪，并完成生产冒烟。

## 发布门禁

### alpha / dev

- 用于快速集成和开发联调，允许后端频繁更新。
- 本地前端默认对接 alpha 后端。
- 可以接受短期不稳定，但不能污染 beta/prod 配置、数据或密钥。

### beta

- 是公开预发布/展示环境，安全等级高于 dev。
- 后端可较频繁更新，但每次更新后至少执行健康检查和基础 API 冒烟。
- 前端只有在功能完整且通过冒烟后才能发布 Cloudflare。
- beta 页面必须展示“发布前完整功能”，不能把半成品暴露为对外展示版本。
- 每次 beta 前端发布都要记录 Cloudflare deployment、后端镜像、API 域名、冒烟结论和已知问题。

### prod

- 新部署 `mss-boot-prod`。
- 后端必须使用 tag 版本，不使用临时分支镜像。
- 发布前必须确认独立生产资源、备份、迁移、回滚和安全配置。
- 发布后必须执行线上冒烟，并保留回滚窗口。

## 待实现时确认

- alpha API 的最终域名是否采用 `admin-api-alpha.mss-boot-io.top`。
- prod API 的最终域名是否采用 `admin-api.mss-boot-io.top`。
- beta Redis 隔离采用独立 DB index、key prefix，还是两者同时使用。
- beta 新数据库命名规则和数据库账号命名规则。
- Cloudflare 发布次数限制和发布额度策略。
- prod 后端 tag 命名规则，例如 `vX.Y.Z` 或其他 release tag 约定。
