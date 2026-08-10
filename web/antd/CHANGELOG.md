# Changelog

All notable, verifiable frontend changes are documented here. Standalone
frontend releases use the `web/antd/vX.Y.Z` tag namespace; the root foundation
release may also embed a separately validated frontend artifact.

## [Unreleased] - web/antd/v1.1.0 development

Status: **development / unpublished**. Neither the branch nor this entry is a
stable standalone frontend release. Internal waves remain untagged; production
and local builds, bundle budget, delivery smoke, and browser acceptance run as
part of the frozen `v1.1.0` qualification before any standalone tag is created.
Root `v1.0.0` was published on 2026-08-09 with its validated frontend artifact,
but no standalone `web/antd/v1.0.0` tag or Release exists. That abandoned
standalone candidate is rolled into `web/antd/v1.1.0`; the root artifact and
standalone release identities must not be conflated.

This development train targets the consolidated frontend's first separately
qualified standalone release at `v1.1.0`. The unpublished v0.8.0 and v1.0.0
candidates do not provide reusable bundle, checksum, image, endpoint-scan, or
browser evidence; all release artifacts and results must be generated from the
exact feature-frozen `web/antd/v1.1.0` commit.

### Added

- Added layered application and personal theme editors backed by one
  scope-aware component. Runtime precedence is `code defaults < application
  settings < personal settings`; sparse overrides preserve inheritance.
- Added revision-aware theme resources, conflict presentation for stale
  `If-Match` writes, cross-tab synchronization, identity-isolated browser
  snapshots, and explicit reset/inherit actions. This capability remains
  preview until the release matrix in the compatibility contract passes.
- Added permission-denied boundaries for static routes, dynamic menu routes,
  page actions, application secrets, uploads, and root-only system
  configuration.
- Added reusable monitor trend components that consume backend timestamps and
  bounded recent history, with loading, empty, stale, and error states.
- Added searchable authorized navigation and a permission-aware workplace with
  meaningful empty states instead of fabricated business data.

### Changed

- The root v1.1.0 development integration refreshes email-Challenge readiness on every
  authentication-route mount with a bounded request. Login, registration, and
  password-reset forms fail closed after dependency errors or timeouts instead
  of trusting stale bootstrap state.
- Email authentication routes expose explicit loading and unavailable states in
  both supported locales. Registration additionally requires the backend
  `registerEnabled` capability before its form is rendered.
- Dynamic menus remain the navigation source of truth and are refreshed when
  the current identity or permission revision changes. Frontend checks improve
  experience but do not replace backend authorization.
- Authentication bootstrap, avatar loading, locale resources, option data, and
  navigation use bounded caches and request deduplication while preserving
  owner/identity isolation.
- Account avatars use a circular presentation and terminate loading when the
  image or profile request fails.
- Desktop and mobile list pages use consistent loading, empty, error,
  permission-denied, and destructive-action confirmation behavior.
- CPU and memory charts derive axes, labels, lines, fills, and tooltip surfaces
  from Ant Design theme tokens so light and real-dark themes remain readable.
- Application configuration credential fields are omitted or read-only unless
  the current principal has the dedicated secret capability; SystemConfig is
  root-only.
- The Storage settings panel now reads and writes only the byte-based upload
  limit and MIME/wildcard allowlist. Provider endpoints, buckets, and credentials
  are no longer browser-managed; Provider and SecretRef belong to the startup
  profile boundary.
- PAT creation and rotation show the raw token once and never repopulate it from
  list responses or browser persistence.
- OAuth callback data is removed from the URL and submitted to the backend in a
  POST body; provider tokens never enter frontend storage.

### Breaking integration changes

- The bundled frontend expects the v1.0.0 Admin authorization and current-user
  contracts. Deploy backend migrations and the v1.0.0 backend before enabling
  versioned theme writes.
- OAuth callback completion uses `POST
  /admin/api/user/:provider/callback`; legacy GET callbacks are not supported.
- PAT creation uses `POST /admin/api/user-auth-tokens`; the historical GET
  generator endpoint is not supported.
- Role authorization and versioned theme writes use ETag/`If-Match`; stale
  drafts return `412` and require explicit user reconciliation.
- Direct navigation to removed runtime development-tool routes resolves through
  the normal not-found boundary.

### Removed

- Removed the unsupported phone-login tab and phone-captcha client path from the
  root v1.1.0 bundled authentication surface after release qualification.
- Removed runtime model/field administration, virtual CRUD, browser template
  generation, related routes, menu entries, locale keys, service clients, and
  generated API types.
- Removed placeholder rows, fake trend points, and optimistic success states
  that could disguise a permission or backend failure.

### Toolchain and delivery

- Node.js `>=22 <25` and pnpm `9.15.9` are the supported build toolchain.
- `build:local` intentionally targets a backend on the developer workstation
  and must not be mislabeled as a portable production artifact.
- A stable generic artifact must use its reviewed production or runtime
  configuration, contain no development/alpha/beta API endpoint, pass the
  bundle budget, and pass the Nginx delivery smoke test.
- Frontend-only rollback restores the preceding static artifact and cache
  headers. It does not roll back backend migrations or configuration writes.

### Upgrade

Use the consolidated [v1.0.0 upgrade guide](../../docs/docs/releases/v1-0-0-upgrade.md)
and [compatibility matrix](../../docs/docs/releases/v1-0-0-compatibility.md).
Do not deploy the new frontend ahead of the required backend migration and API
contract, and do not call a preview build stable.

## Historical development snapshot - 2026-04-06

The former changelog labeled this snapshot `v1.0.0`, but no corresponding
consolidated-repository frontend tag established that version. It remains
historical implementation context and is not standalone-release evidence. Only
the separately validated frontend artifact embedded by root `v1.0.0` was
published; `web/antd/v1.0.0` was not.
