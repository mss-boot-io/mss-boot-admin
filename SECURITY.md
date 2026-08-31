# Security Policy

## Reporting a vulnerability

Do not open a public issue for suspected vulnerabilities.

Use GitHub Security Advisories for this repository when private vulnerability
reporting is enabled. If that setting is not available yet, contact the
maintainer privately and include the affected commit/tag, impact, reproduction
steps, and any proof of concept.

The organization still needs a final public security contact. Until then,
private GitHub advisories are the preferred intake path.

## Supported versions

The active `main` branch and the current stable Complete Admin Distribution are
supported by default. v1.3.2 remains the current stable and rollback baseline.
v1.3.7 is a release candidate and becomes the supported stable Distribution
only after its coordinated Framework, Admin, frontend, Root, tools, images,
and official npmjs package have been published and publicly reconciled from one
exact merged-main commit, stable aliases converge, and the current-stable policy
is updated. Docs is an asynchronous website publication and never gates stable
support or adopter availability. v1.3.5 and v1.3.6 are immutable
partial trains and never become complete supported Distributions. v1.3.6
published component and Root identities but lacks the Root image, official
npmjs package, and Docs; no stored npm token may be introduced to complete it.
During release
preparation, `.mss/release-policy.yaml` remains the authority for the supported
stable, candidate, stopped, and rollback identities.
Preview and release-candidate refs remain immutable evidence but do not receive
the stable support commitment. Older stable versions are handled case by case.

## v1.3.7 candidate security boundary

- Presentation profiles are untrusted, strictly data-only overlays over exact
  compiled capabilities. They cannot define code, HTML, SQL, transport, routes,
  permissions, runtime models, or executable components.
- `presentation:read`, `presentation:draft-write`, `presentation:publish`, and
  `presentation:rollback` are independent backend permissions. Existing
  ordinary roles receive none of them automatically, and UI visibility is not
  an authorization decision.
- Strong ETags, capability-hash validation, hashed idempotency keys, immutable
  revisions, and redacted audit metadata protect publication and rollback.
  Startup-only `presentation.recoveryMode` bypasses stored layers without
  deleting their evidence.
- Six high/critical advisories inherited through the Umi build or inactive
  plugin graph have time-bounded non-runtime acceptances through 2026-11-08:
  two Vite, one node-fetch, two Immer, and one path-to-regexp finding. Exact
  resolutions are machine checked; no accepted package may enter the browser
  runtime bundle.
- Fresh initialization no longer accepts `migrate --password` or `migrate -p`.
  A hidden interactive prompt or one-use `MSS_ADMIN_INITIAL_PASSWORD` injection
  is confined to the initialization process and must not be persisted or logged.

## Response expectations

- Acknowledge valid private reports within 7 days when possible.
- Triage severity, affected versions, and exploitability before public
  disclosure.
- Prepare a fix, release note, image tag, and upgrade guidance before disclosing
  details.
