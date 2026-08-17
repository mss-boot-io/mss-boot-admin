# Changelog

All notable changes to the independently released Ant Design 6 application are
recorded here. Retired frontend history remains available through Git history
and the root changelog, not as a current release surface.

## Unreleased

- Become the repository's sole Admin frontend without changing the independent
  `web/antd-v6` directory, tag, image, lockfile, or release identity. Canonical
  setup, local development, compose delivery, active documentation, generation,
  and root distribution now select V6; rollback selects a preceding V6 pair.
- Establish the React 19 and Ant Design 6.6.0 application foundation.
- Add exact dependency, bundle, same-origin delivery, CI, and release contracts.
- Use mandatory credential-free browser login, refresh, and OAuth callback endpoints;
  the backend has no stateless or retired-client browser compatibility mode.
- Add signed-CSRF request integration and a single-use WebSocket subprotocol ticket client.
- Add the typed backend identity contract, fail-closed compiled menu intersection,
  single React Query cache, and layered Ant Design 6 token theme runtime.
- Add canonical theme media type, ETag/If-Match, and authoritative 412 handling.
- Add one application/personal theme editor with field inheritance, whole-scope
  reset, RBAC read-only state, explicit conflict decisions, and responsive locale parity.
- Add monotonic cross-tab theme events, verified-subject personal session binding,
  Web-Locks-protected 24-hour snapshots, and public-only first-paint hints.
- Alias transitional Moment consumers to Day.js and reject Moment in the release bundle.
- Compile for the explicit Chromium/Edge 120+, Firefox 121+, and Safari 17.4+
  baseline, reject legacy `core-js`, and use public per-icon package subpaths so
  Utoopack does not retain unsupported compatibility or icon-barrel code.
- Add responsive account center and settings views with exact self-profile fields,
  synchronized locale catalogs, notification preferences, and provider-gated OAuth.
- Keep identity email read-only and make profile empty-value persistence explicit on
  the backend instead of copying the legacy ambiguous update behavior.
- Add PAT list/create/rotate/revoke flows whose one-time raw secret never enters the
  shared React Query cache or browser persistence.
- Add personal password rotation and OAuth disconnect with durable recent
  reauthentication, proof-failure limits, atomic session/PAT revocation, and
  protection against removing the final verified login method.
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
- Add bounded Option management with summary/detail projections, server-owned record and
  item identity, complete prior-resource snapshots, tenant-namespaced cache invalidation,
  strong `If-Match` conflicts, built-in identity protection, usage-aware deletion, and
  separate read/create/update/delete permissions. Render opaque metadata as inert text
  through native Table/Form and Ant Design 6.6 Listy components.
- Keep Supplier as the deterministic V6 generator golden. The sole target emits
  the `web/antd-v6` route, locale, strict contract, typed transport, React Query,
  responsive CRUD, Vitest, and HttpOnly/CSRF-aware Playwright artifacts.
- Bind Max 4.7 development through its supported `PORT=8001` environment contract
  so local tools and Playwright cannot silently select another port.
- Mount model-consuming runtime bridges inside Umi's dataflow provider, namespace generated
  forms, expose stable localized action names, disable virtualization for bounded enum choices,
  and make generated browser fixtures safe for concurrent and repeated soft-delete runs. The
  Supplier HttpOnly/CSRF CRUD flow now passes Chromium desktop and mobile together.
- Reserve the independent `web/antd-v6/v{version}` tag and
  `mss-boot-admin-antd-v6` image namespaces.

Business parity is retained while this Unreleased change removes the retired
frontend and browser compatibility protocol. Publication still requires the
exact merged-main commit, the concentrated qualification suite, and immutable
artifact promotion. Rollback selects only the preceding qualified V6 pair.
