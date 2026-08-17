# Retire Ant Design 5 and make V6 the only Admin browser application

- Status: Accepted
- Date: 2026-08-17
- Owners: Admin Platform, Frontend, Release Engineering, Security
- Feature contract: `.mss/features/admin-antd-v6-cutover-retirement.yaml`
- Supersedes: `2026-08-15-independent-ant-design-v6-application.md`

## Context

The independently built `web/antd-v6` application reached the retained product
capability baseline on merged `main`. Keeping the Ant Design 5 source, generator,
commands, workflows, static artifacts, and browser-only backend compatibility would
leave two owners for one product and preserve weaker token, WebSocket, OAuth, theme,
and revision behavior.

The supported non-browser API contract is distinct from the removed browser
compatibility. Personal access tokens and standard `Authorization: Bearer` requests
remain available for documented automation; browser JavaScript never receives an
Admin token.

## Decision

`web/antd-v6` is the only Admin browser application. The repository removes the V5
source tree, generator projection, dependency automation, CI, release and deployment
workflows, rollback build path, active inventories, and active documentation.

The V6 application keeps an independent delivery identity:

- directory `web/antd-v6`;
- tag namespace `web/antd-v6/v{version}`;
- image `mss-boot-admin-antd-v6`;
- Node 24 and pnpm 10.34.5;
- immutable checksums, SBOM, provenance, and previous-V6 rollback history.

The backend exposes one browser protocol:

- HttpOnly `mss_admin_session` plus signed double-submit CSRF;
- `/admin/api/user/session/login`,
  `/admin/api/user/session/refresh-token`, and
  `/admin/api/user/auth-cookie/clear`;
- BrowserSession-only OAuth configuration and callback responses;
- one-time WebSocket tickets on `/ws/connect` through
  `Sec-WebSocket-Protocol`;
- canonical versioned theme resources and mandatory strong `If-Match` for
  revisioned mutations.

Token-returning browser login and refresh, bearer OAuth callback mode, generic OAuth
binding endpoints, the historical `jwt` cookie, query-token WebSocket authentication,
unversioned theme projections, and missing-revision fallbacks are removed. A forward,
idempotent migration deletes only the exact obsolete OAuth and theme configuration
keys; bindings, users, audit history, unknown settings, and business data are preserved.

## Security and compatibility

The cutover is intentionally breaking for old browser assets. A frontend/backend
version mismatch fails visibly instead of silently falling back to a weaker contract.
Production still requires an exact HTTPS application origin, Secure cookies, strong
authentication keys, shared session state, and provider-specific BrowserSession OAuth
credentials.

Historical tags, changelogs, releases, and Git history remain immutable evidence. They
are not active build or rollback inputs.

## Rollout and rollback

The removal change is merged through a pull request to `main`. Release qualification
runs from the exact clean merged commit and validates migration, backend security,
frontend contracts, browser journeys, static delivery, and artifact provenance as one
batch.

Rollback redeploys the preceding qualified V6 frontend and backend pair. Additive
database revisions and business data remain in place. V5 source, artifacts, routes, or
protocol compatibility are not restored as a rollback mechanism.

## Consequences

The repository has one frontend toolchain, one generator target, one active browser
contract, and one release surface. Downstream applications must upgrade their browser
client and backend together at this compatibility boundary. The ongoing maintenance
cost is lower, while strict failures make stale deployments diagnosable rather than
implicitly insecure.
