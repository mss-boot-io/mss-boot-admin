# Handwritten business frontend

Keep non-generated business components and tests here. Generated module pages,
routes, contracts, and locale registries live under `src/generated/` and
`src/pages/generated/`.

Add handwritten Umi routes to `routes.config.ts` and register their server-path
projections in `route-registrations.ts`. Both files are explicit business-owned
frontend extension points; managed facades compose them after generated entries.
They only drive route and menu visibility. They do not create Admin Menu or
Casbin rows and do not authorize backend requests; the handwritten backend
module must own forward authorization migrations, readiness, enforcement, and
positive/negative permission tests.

Keep handwritten messages in `locales/zh-CN.ts` and `locales/en-US.ts`, updating
both catalogs for every user-facing addition. The managed locale facades compose
Admin core messages first, generated module messages second, and these custom
messages last. Do not edit `src/locales/` or `src/generated/locales/` directly.
