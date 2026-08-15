# Changelog

All notable changes to the independently released Ant Design 6 application are
recorded here. This changelog does not describe releases of `web/antd`.

## Unreleased

- Establish the React 19 and Ant Design 6.6.0 application foundation.
- Add exact dependency, bundle, same-origin delivery, CI, and release contracts.
- Use dedicated credential-free browser login, refresh, and OAuth callback endpoints.
- Add signed-CSRF request integration and a single-use WebSocket subprotocol ticket client.
- Add the typed backend identity contract, fail-closed compiled menu intersection,
  single React Query cache, and layered Ant Design 6 token theme runtime.
- Add canonical theme media type, ETag/If-Match, and authoritative 412 handling.
- Add one application/personal theme editor with field inheritance, whole-scope
  reset, RBAC read-only state, explicit conflict decisions, and responsive locale parity.
- Add monotonic cross-tab theme events, verified-subject personal session binding,
  Web-Locks-protected 24-hour snapshots, and public-only first-paint hints.
- Alias transitional Moment consumers to Day.js and reject Moment in the release bundle.
- Add responsive account center and settings views with exact self-profile fields,
  synchronized locale catalogs, notification preferences, and provider-gated OAuth.
- Keep identity email read-only and make profile empty-value persistence explicit on
  the backend instead of copying the legacy ambiguous update behavior.
- Add PAT list/create/rotate/revoke flows whose one-time raw secret never enters the
  shared React Query cache or browser persistence.
- Keep unsafe legacy password reset and last-login-method OAuth disconnect actions
  unavailable pending a recent-reauthenticated backend contract.
- Add authoritative authorization freshness on explicit/cross-tab events, 403,
  network recovery, and throttled focus/visibility changes, with exact domain-query
  eviction and a retryable fail-closed state when identity or menu cannot be verified.
- Push a non-sensitive global authorization revision after successful policy reload,
  with non-blocking server fan-out, secure ticketed WebSocket reconnect, heartbeat,
  cross-tab revision deduplication, and authoritative HTTP revalidation.
- Replace the workplace foundation placeholder with a responsive, localized operations
  view and protected monitor query. Validate bounded server history, follow server refresh
  cadence, honor `Retry-After`, stop polling on 401/403, retain last-good data, and show
  distinct warm-up, stale, error, permission, and empty-history states.
- Render CPU and memory trends with accessible token-aware native SVG rather than adding
  a chart runtime, and migrate remaining V6 Alerts and Statistics away from deprecated APIs.
- Add a fail-closed root-only online-session inventory with strict list/detail contracts,
  bounded foreground polling, explicit last-good/error/empty/permission states, responsive
  detail, and audited row-bound session and per-user revocation.
- Enforce a 100-row server page limit and omit the legacy arbitrary user-ID revoke control.
  Use Ant Design 6 Table after the bundle gate demonstrated that ProTable's unused schema
  features would exceed the application's total JavaScript budget.
- Add bounded language management with summary/detail projections, server-owned IDs,
  canonical BCP 47 names, definition limits, optimistic revision conflicts, and separate
  read/create/update/delete permissions. Use native Table/Form and Ant Design 6.6 Listy;
  load runtime translations as an optional enhancement limited to the complete shipped
  zh-CN and en-US catalogs.
- Reserve the independent `web/antd-v6/v{version}` tag and
  `mss-boot-admin-antd-v6` image namespaces.

The application remains blocked from production release until the FeatureSpec
business parity, generator, E2E, and full qualification evidence is complete.
