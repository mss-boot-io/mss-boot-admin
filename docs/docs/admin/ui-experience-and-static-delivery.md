---
title: V6 UI experience and static delivery
---

# V6 UI experience and static delivery

`web/antd-v6` is the sole Admin frontend. Product completeness comes before bundle
refinement: retained V6 flows must first be truthful, localized, responsive, accessible,
and backend-authorized; release qualification then enforces transfer and delivery gates.

## Experience contract

- One responsive business component serves desktop and mobile unless the interaction
  model genuinely differs.
- Loading, confirmed empty, retryable error, forbidden, not-found, and stale/conflict
  states remain distinct.
- Every visible action completes a supported backend workflow; placeholder CRUD controls
  are not rendered.
- Chinese and English catalogs remain synchronized and shared page states have no
  hard-coded language fallback.
- Relation fields show authoritative names while sending server-owned identifiers.
- Menu metadata is intersected with the compiled route registry and cannot import a
  component string.
- UI permission guards supplement, but never replace, backend positive and negative
  authorization.
- Ant Design 6 customization uses CSS variables, public tokens, and semantic slots; code
  does not select internal `.ant-*` or `.ant-pro-*` DOM structure.
- PWA, service-worker API caching, analytics, and demo-only modules are absent.

## Build and size gates

`web/antd-v6/scripts/check-bundle-budget.mjs` verifies gzip transfer limits after the
production build. Budget changes require measured route-transfer evidence and a reviewed
architecture decision; invalid or non-positive configuration fails closed.

```shell
corepack pnpm@10.34.5 --dir web/antd-v6 lint
corepack pnpm@10.34.5 --dir web/antd-v6 test:ci
corepack pnpm@10.34.5 --dir web/antd-v6 build:release
```

Functional work should not repeatedly optimize bundle shape. The concentrated pre-PR
batch verifies the final build, dependency tree, duplicate React/Ant Design runtime,
entry and lazy chunks, and complete lazy-loaded corpus.

## Nginx delivery contract

`web/antd-v6/nginx.conf`, its Dockerfile, and
`web/antd-v6/scripts/smoke-nginx-delivery.sh` enforce:

- immutable caching for content-hashed assets;
- no long-term cache for HTML and `release.json`;
- gzip compression for eligible static assets;
- a non-cacheable 404 for missing hashed chunks;
- SPA fallback for real navigation routes;
- same-origin proxying of `/admin/` HTTP and WebSocket traffic;
- `/healthz` identity for `mss-boot-admin-antd-v6`.

Release artifacts are built only from an exact clean commit already merged into
`origin/main`. Rollback redeploys the preceding qualified V6 digest; it does not rebuild
historical source or restore a retired frontend.

The executable acceptance contract is
`.mss/features/admin-ui-experience-quality.yaml`.
