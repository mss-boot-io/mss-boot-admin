# Ant Design 6 frontend agent instructions

## Scope

This file applies to `web/antd-v6/` and inherits the repository contract. This
directory is both the official Admin application and the sole complete
`@mss-boot-io/admin-web` package. Both surfaces use this same source tree. It
must not read source, build output, dependencies, local storage, service
workers, or release identity from any retired frontend artifact.

## Frozen toolchain

- Node.js 24.
- pnpm 10.34.5 through Corepack. pnpm 11 is intentionally deferred until its
  SQLite store is reliable on the supported Node 24 and WSL/CI matrix.
- React 19, Ant Design 6.6.1, Umi Max 4.7.7, React Query 5, and a pinned
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
corepack pnpm@10.34.5 pack
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
- `package/` contains small public adapters only; it must not become a second
  application source tree. `bin/mss-admin-web.cjs` owns downstream commands.
- Public package imports use only declared `exports`. Do not expose `./src/*`,
  redirect a Thin Host's `@` alias, or require consumers to copy core pages,
  patches, configuration, or `src/shared`.
- Core source uses `@mss-admin-core`; `@mss-admin-business/routes` is the only
  generated menu-registration bridge. Business routes and registrations must
  be injected together before the 403/404 fallbacks.

React Query owns server state. Umi initial state contains only verified identity
and startup-critical client state. Do not duplicate server resources in a Umi
model, component-local cache, and query cache.

Backend authorization remains authoritative. Client route access is a user
experience guard and must have backend positive and negative tests.

Every package change must pass a real `pnpm pack` allowlist check and an
external Thin Host install/lint/test/build. Runtime singleton claims require
the installed pnpm graph and production bundle evidence, not manifest review.

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
