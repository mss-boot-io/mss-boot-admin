# Ant Design 6 frontend agent instructions

## Scope

This file applies to `web/antd-v6/` and inherits the repository contract. This
directory is an independently built and released application. It must not read
runtime source, build output, dependencies, local storage, service workers, or
release identity from `web/antd/`.

## Frozen toolchain

- Node.js 24.
- pnpm 10.34.5 through Corepack. pnpm 11 is intentionally deferred until its
  SQLite store is reliable on the supported Node 24 and WSL/CI matrix.
- React 19, Ant Design 6.6.0, Umi Max, React Query 5, and a pinned
  ProComponents 3 beta.
- Biome, TypeScript, Vitest, and Playwright.

Do not add a floating dependency range. Update `package.json`, `pnpm-lock.yaml`,
the dependency contract test, and the upstream provenance in one reviewed
change.

## Canonical commands

```shell
corepack pnpm@10.34.5 install --frozen-lockfile
corepack pnpm@10.34.5 run deps:check
corepack pnpm@10.34.5 run lint
corepack pnpm@10.34.5 run test:ci
corepack pnpm@10.34.5 run build:release
corepack pnpm@10.34.5 run delivery:smoke
```

From the repository root use `make web-v6-install web-v6-lint web-v6-test
web-v6-build` or `go run ./cmd/mss verify --changed`.

## Architecture boundaries

- `src/app/` owns startup and the application shell.
- `src/modules/<domain>/` owns one business capability vertically.
- `src/shared/api/` owns transport and generated-client adapters.
- `src/shared/auth/` owns browser session behavior, never authorization policy.
- `src/shared/design-system/` owns public tokens, semantic styles, page states,
  and responsive primitives.
- `src/generated/` is deterministic output and must not be hand edited.

React Query owns server state. Umi initial state contains only verified identity
and startup-critical client state. Do not duplicate server resources in a Umi
model, component-local cache, and query cache.

Backend authorization remains authoritative. Client route access is a user
experience guard and must have backend positive and negative tests.

## Ant Design 6 rules

- Use ConfigProvider CSS variables and public semantic tokens.
- Customize components through tokens or documented `classNames` and `styles`.
- Do not select undocumented `.ant-*`, `.ant-pro-*`, CSS-in-JS hash, or DOM
  hierarchy classes.
- Use `App.useApp()` for message, modal, and notification context.
- Use current v6 API names and keep the browser console free of deprecation
  warnings.
- Import icons from public `@ant-design/icons/<IconName>` subpaths. The package
  barrel currently prevents complete Utoopack tree-shaking and is rejected by
  the dependency contract.
- Tailwind owns layout utilities, CSS Modules own local static rules, and
  antd-style owns complex token-aware rules.
- Prefer one responsive component over parallel desktop and mobile business
  implementations.

## Completion rules

Every retained page considers loading, confirmed empty, retryable error, 403,
404, conflict, desktop/mobile, zh-CN/en-US, keyboard/focus, generated contract
drift, and backend authorization. PWA, analytics, AI demos, and removed runtime
developer tools are outside this application.
