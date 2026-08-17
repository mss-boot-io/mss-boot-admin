---
title: Ant Design V6 architecture and cutover
---

# Ant Design V6 architecture and cutover

The Admin product has one supported browser application: `web/antd-v6`. The former
Ant Design 5 source, generator, commands, workflows, artifacts, and backend browser
compatibility are retired. Git history and immutable releases remain the historical
record; they are not build or rollback inputs.

## Frozen application baseline

- Ant Design Pro scaffold: v6.0.2, commit
  `2b453c67b535b76f5f95d6542397a4b987b61de2`.
- Ant Design: exactly `6.6.0`.
- React 19, Umi Max, React Query 5, and the reviewed exact ProComponents 3 beta.
- Node 24 and pnpm 10.34.5 through Corepack.
- Biome, TypeScript, Vitest, Playwright, Utoopack, and route prefetch.

Exact versions and provenance live in `web/antd-v6/package.json`, its lockfile, and
`.mss/project.yaml`. Dependency upgrades are isolated reviewed changes; `latest` or a
floating range is never a release coordinate.

## Architecture ownership

| Concern | Owner |
| --- | --- |
| HTTP envelope, CSRF, uploads, files, and normalized errors | `src/shared/api` |
| Server state, invalidation, retries, and stale data | React Query |
| Verified identity and startup-critical client state | Umi initial state |
| Browser sessions, refresh, redirect, and OAuth attempt state | `src/shared/auth` |
| Brand and component semantics | Ant Design CSS variables and tokens |
| Layout utilities | Tailwind |
| Local static rules | CSS Modules |
| Complex token-aware styles | antd-style |
| Business behavior | `src/modules/<domain>` |
| Deterministic generated modules | `src/generated` |

Database menu metadata is intersected with the compiled route registry. It cannot
select or import an executable component string. Frontend access checks improve the
experience; the backend remains authoritative.

## V6-only browser protocol

- Login, refresh, and logout use `/admin/api/user/session/*`.
- The Admin JWT stays in the HttpOnly `mss_admin_session` cookie.
- State-changing browser requests require exact trusted Origin and signed
  double-submit CSRF.
- Visible tabs schedule a coordinated refresh before session expiry.
- OAuth uses BrowserSession-only provider configuration, attempt-bound return state,
  and callback responses that never expose Admin or provider tokens.
- `/ws/connect` accepts only a short-lived one-time ticket through
  `Sec-WebSocket-Protocol`; URL tokens are rejected.
- Theme, role authorization, menu authorization, and option mutations require strong
  `If-Match` preconditions. Missing state returns 428 and stale state returns 412.

Personal access tokens and standard `Authorization: Bearer` remain supported for
documented non-browser automation. They are not a browser fallback.

## Data migration boundary

The cutover migration removes only exact obsolete OAuth configuration names and the
obsolete application theme `pwa` row. It preserves users, OAuth bindings, audit
history, business data, BrowserSession configuration, supported theme overrides, and
unknown downstream configuration. The migration is forward-only, transactional, and
idempotent.

## Development and verification

```shell
go run ./cmd/mss setup
go run ./cmd/mss dev
go run ./cmd/mss verify --changed

corepack pnpm@10.34.5 --dir web/antd-v6 lint
corepack pnpm@10.34.5 --dir web/antd-v6 test:ci
corepack pnpm@10.34.5 --dir web/antd-v6 build:release
corepack pnpm@10.34.5 --dir web/antd-v6 test:e2e
```

Functional convergence is completed before bundle refinement. Release qualification
then runs backend security and migration tests, frontend unit and browser suites,
static-delivery smoke checks, dependency inspection, locale synchronization, console
warning checks, and build budgets together.

## Release and rollback

The V6 application is independently published from `web/antd-v6/v{version}` as
`mss-boot-admin-antd-v6`. Release tooling fails closed unless the exact clean source
commit is already merged into `origin/main`.

Rollback redeploys the preceding qualified V6 frontend and backend pair. It never
rebuilds historical source, moves an immutable tag, reverses business data, or restores
the retired browser protocol.

The governing contracts are:

- `.mss/features/admin-antd-v6-application.yaml`;
- `.mss/features/admin-antd-v6-cutover-retirement.yaml`;
- `docs/adr/2026-08-17-ant-design-v6-default-cutover.md`.
