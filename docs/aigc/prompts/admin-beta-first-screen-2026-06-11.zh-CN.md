# admin beta 首屏白屏与慢启动修复记忆

## 背景

- 时间：2026-06-11。
- 仓库：`mss-boot-admin-antd`。
- 环境：`admin-beta.mss-boot-io.top` Cloudflare beta 前端，对应 `admin-api-beta.mss-boot-io.top`。
- 现象：访问 beta 根路径时页面白屏，控制台有 `single-spa minified message #1`；首屏非常慢或空白。

## 根因

- standalone Cloudflare 前端默认启用了 `qiankun: { master: {} }`，导致构建产物挂载到 `#root-master`，并引入 single-spa/qiankun master 行为。
- 未登录首屏在 `getInitialState` 中等待语言、应用配置、用户配置 API 后才跳登录页；beta API 慢或异常时会拖住首屏。
- 已有 token 或 `autoLogin` 的浏览器会等待用户信息/刷新 token；当 `admin-api-beta` 522/超时时，登录页也可能因为刷新态返回空 Fragment。

## 已完成修复

- PR `mss-boot-io/mss-boot-admin-antd#112`：
  - 移除默认 qiankun master 配置，standalone 构建回到 `#root`。
  - 未登录用户立即进入 `/user/login?redirect=...`，公共配置改为登录页异步补拉。
  - 清理登录页嵌套 `<a>` DOM。
- PR `mss-boot-io/mss-boot-admin-antd#113`：
  - 用户信息、语言、应用配置、用户配置初始化增加超时兜底。
  - token 失效或后端不可达时清理本地登录态并进入登录页。
  - 登录页不再因为 autoLogin 刷新 token 处于 loading 而渲染空 Fragment。
- 两个 PR checks 均通过后 squash merge 到 main，并分别触发 Cloudflare beta 发布。

## 验证记录

- 本地验证：
  - `pnpm install --frozen-lockfile`
  - `pnpm tsc`
  - `pnpm lint:js`
  - `pnpm test -- --runInBand`
  - `pnpm build:beta`
  - `pnpm build:local`
- 产物检查：
  - HTML 使用 `<div id="root"></div>`。
  - 主 bundle 不包含 `root-master`、`single-spa`、`registerApplication`。
- 真实 beta 冒烟：
  - `https://admin-beta.mss-boot-io.top/?codex_smoke=20260611b` 成功加载新包 `umi.1c7e8791.js`。
  - 页面进入 `/user/login?redirect=%2Fworkplace`。
  - 登录页可见，`#root` 正常挂载，`#root-master` 不存在。

## 仍需跟进

- 2026-06-11 从本机访问 `admin-api-beta.mss-boot-io.top/admin/api/app-configs/profile` 仍 20 秒超时，登录接口此前出现 Cloudflare 522。
- 2026-06-11 用户确认 alpha 与 beta 后端在同一个 devops 集群；后续排查应以 alpha 所在 devops 集群为准，不再使用 `~/.kube/baas.yaml` 判断 beta。
- 本机现有 `~/.kube/devops.yml`、`~/.kube/new-devops.yaml`、`~/.kube/baas-dev.yml` 缺少 context，且 client certificate/key 内容无法被 OpenSSL 解析；临时补 context 后仍无法正常访问 devops API。
- `devops.inichain.com`、`admin-api-alpha.mss-boot-io.top`、`admin-api-beta.mss-boot-io.top` 当前 DNS 均解析到 `198.18.0.x` 网段；TCP 可连，但 devops API TLS 握手超时，alpha API TLS 阶段异常，beta API 由 Cloudflare 返回 522。
- 这说明 beta API 源站连通性仍是独立问题；待 devops 集群 kubeconfig/VPN/网络入口可用后，优先检查 `mss-boot-dev` 与 `mss-boot-beta` namespace 的 Pod、Service、Ingress、Cloudflare DNS/回源链路。
