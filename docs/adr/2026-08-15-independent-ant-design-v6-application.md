# Build Ant Design 6 as an independently released Admin application

- Status: Accepted
- Date: 2026-08-15
- Owners: Admin Platform, Frontend, Release Engineering
- Feature contract: `.mss/features/admin-antd-v6-application.yaml`

## Context

The repository currently treats `web/antd` as its only frontend. Project contracts,
development orchestration, verification, generated module paths, CI, release tags,
container images, and deployment workflows all encode that assumption. The application
also contains business behavior that must survive a framework migration alongside
legacy request, style, responsive, service-worker, and browser-token patterns that must
not survive it.

Ant Design Pro's latest formal source release is v6.0.2 at commit
`2b453c67b535b76f5f95d6542397a4b987b61de2`. This repository deliberately upgrades its
component baseline to an exactly pinned Ant Design 6.6.0. ProComponents v3 is still a
beta line, so the resolved version is pinned and its use is isolated behind shared
application adapters where practical.

## Decision

Create `web/antd-v6` as a second, independent application. It is neither an in-place
upgrade of `web/antd` nor a subpath build of the old application.

The two applications have separate:

- package manifests and lockfiles;
- Node and pnpm toolchain declarations;
- source, tests, generated output, build directories, and Nginx configuration;
- CI checks and dependency audits;
- immutable release artifacts and build identity files;
- container image repositories;
- tag namespaces (`web/antd/v*` and `web/antd-v6/v*`);
- deployment workflow inputs, environment configuration, and rollback history.

The initial v6 architecture uses React 19, Umi Max with utoopack and route prefetch,
Ant Design 6 CSS variables and semantic DOM, a pinned ProComponents v3 beta, React Query
v5, dayjs, Tailwind for layout utilities, CSS Modules for local static rules, antd-style
for token-aware rules, Biome, Vitest, and Playwright.

The release bundle gate treats transfer cost and total application capability as
different signals. The gzip budgets are 32 KiB for the entry, 240 KiB for any single
asynchronous chunk, and 960 KiB for the complete lazy-loaded JavaScript corpus. The
total-corpus budget was recalibrated from the 900 KiB scaffold threshold after the
qualified authority and operations slices reached 936.67 KiB while the entry remained
4.16 KiB and the largest asynchronous chunk remained 199.60 KiB. A four-way operations
route split was rejected because it increased the same corpus to 1018.24 KiB without
reducing either user-facing limit enough to justify the duplication.

Business migration is capability-equivalent rather than source- or pixel-equivalent.
Each vertical slice includes its route registry entry, backend-authorized menu mapping,
typed API behavior, loading/empty/error/permission/conflict states, responsive behavior,
Chinese and English copy, tests, and release evidence.

The backend menu can control visibility, order, labels, icons, and permissions, but the
client loads components only from its compiled route registry. Removed runtime developer
tools, official demo pages, AI Assistant, analytics, and PWA are excluded.

Production session hardening is part of the accepted target: same-origin HttpOnly cookie,
CSRF protection, and non-query WebSocket ticket authentication. Backend support must be
introduced compatibly so the independently released legacy frontend continues to work
through the overlap window.

## Generator and contract boundary

The existing frontend remains the canonical legacy generator target until an explicit
cutover decision. A versioned v6 generator profile writes only `web/antd-v6` and uses
Supplier as its first golden module. A command must select its target; the generator does
not blind-write both trees.

Production Supplier routes acquire the currently published database through a bounded
request lease. Authentication, authorization, and generated application operations share
that request-pinned handle, so configuration reload can drain the previous pool without
leaving the generated service bound to a closed database. Generated module output remains
unchanged; the lease belongs to server composition and is regression-tested across a
database replacement.

The Project contract identifies both applications, while `repositoryLayout.frontend`
continues to point at `web/antd` for backward compatibility. Each application records
its own path, role, toolchain, development port, release tag template, and image name.

## Release and rollback

V6 releases originate only from tags matching `web/antd-v6/v{version}` and publish a
distinct image and artifact set. V5 releases continue to use `web/antd/v{version}`.
Neither workflow can publish or overwrite the other application's artifacts.

Both workflows retain the repository rule that a releasable commit must already be
merged into `main`, exactly checked out, and clean. Tags and published artifacts are
immutable.

Rollback is application-specific: the operator redeploys the preceding immutable v6
image without rebuilding or redeploying v5. Backend changes made for v6 must remain
backward compatible, and additive persistent state is not rolled back merely because a
frontend image is reverted.

## Consequences

The repository temporarily carries two complete frontend validation and release paths.
This increases CI, contract, generator, and API compatibility work, but it prevents a
framework migration from destroying the proven production rollback path.

ProComponents beta upgrades require explicit review and regression evidence. Umi's
resolved dependency graph must be checked for duplicate React and unintended legacy Ant
Design runtime code rather than judged only from top-level package versions.

The bundle thresholds are qualification contracts, not permanent allowances. Any change
that exceeds the entry or single-chunk limit must be optimized before merge. A total
corpus increase requires measured route-transfer evidence and an explicit ADR update;
raising an environment override alone is not release evidence.

The change is complete only when the feature contract's required evidence passes for the
v6 application without weakening the independent v5 checks or publication controls.
