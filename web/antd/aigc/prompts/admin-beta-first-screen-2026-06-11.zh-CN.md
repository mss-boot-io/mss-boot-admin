# admin beta 首屏白屏与慢启动修复记忆

## 背景

- 时间：2026-06-11。
- 环境：`admin-beta.mss-boot-io.top` Cloudflare beta 前端，对应 `admin-api-beta.mss-boot-io.top`。
- 现象：访问 beta 根路径时页面白屏，控制台有 `single-spa minified message #1`；首屏等待时间很长。

## 根因

- `config/config.ts` 默认启用了 `qiankun: { master: {} }`，导致 standalone Cloudflare 前端以 qiankun master 模式构建。
- 线上 HTML 挂载点为 `#root-master`，而普通 standalone 应用应挂载 `#root`，因此 beta/prod 这类独立前端不应默认启用 qiankun master。
- 未登录首屏的 `getInitialState` 会先串行请求语言、应用配置、用户配置，再跳转登录页；当 beta API 慢或异常时，未登录用户会长时间看到空白壳。
- 已有 token 或 `autoLogin` 的浏览器会继续等待用户信息/刷新 token；当 beta API 522/超时时，登录页也可能因为刷新态返回空 Fragment，真实域名仍表现为空白。

## 修复策略

- 移除默认 qiankun master 配置，让 standalone 构建回到 `#root`。
- 未登录用户在 `getInitialState` 中立即进入 `/user/login?redirect=...`，不再阻塞等待语言、应用配置、用户配置 API。
- 已有 token 的用户信息请求增加超时兜底；超时或失败时清理本地登录态并进入登录页。
- 登录页异步补拉公共应用配置，用于 logo、站点名、第三方登录开关等展示，不阻塞首屏可见内容。
- 登录页不再因为 `autoLogin` 刷新 token 请求处于 loading 状态而渲染空 Fragment，后端不可达时也应先展示可见表单。
- 清理登录页嵌套 `<a>` DOM，减少测试和浏览器控制台噪音。

## 验证记录

- `pnpm install --frozen-lockfile`
- `pnpm tsc`
- `pnpm lint:js`
- `pnpm test -- --runInBand`
- `pnpm build:beta`
- `pnpm build:local`
- 本地 beta 静态冒烟：`/` 能进入 `/user/login?redirect=%2Fworkplace`，登录表单可见，HTML 使用 `#root`，不存在 `#root-master`。
- 构建产物扫描：主 bundle 不再包含 `single-spa` 和 `registerApplication`，`root-master` 为 false。

## 仍需跟进

- 2026-06-11 本机直连 `admin-api-beta.mss-boot-io.top` 返回 Cloudflare 522 或超时。
- 同时 `kubectl --kubeconfig ~/.kube/baas.yaml` 访问集群 API 出现 TLS handshake timeout。
- 因此 beta API 源站连通性是独立问题；待集群 API 可达后，优先检查 `mss-boot-beta` namespace 的 Pod、Service、Ingress、Cloudflare DNS/源站回源链路。
