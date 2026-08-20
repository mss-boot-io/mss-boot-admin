# ADR: Complete Admin distribution and thin business hosts

- Status: Accepted; v1.3.0-rc.6 preview qualification in progress
- Date: 2026-08-19
- Owners: Admin, frontend, agent infrastructure, release engineering
- Feature contract: `.mss/features/complete-admin-distribution-thin-host.yaml`
- Detailed architecture: `docs/docs/architecture/complete-admin-distribution-and-thin-business-host.zh-CN.md`

## Context

The Foundation currently creates downstream repositories by copying most tracked source files. A real
business project therefore owns a large mirror of the Admin backend, V6 frontend, framework, tooling,
documentation, and release automation. Core security fixes and product upgrades must later be merged
through the same files as business customizations.

The problem is source ownership, not a need for independently operated services. Splitting login,
authorization, menus, configuration, or the application shell would add network, deployment, identity,
and version failure modes without reducing the number of Admin products that must be maintained.

## Decision

`mss-boot-admin` remains one complete product and one coordinated `Admin Distribution` version.

The backend exposes a context-aware application composition API from the existing `admin` Go module.
The official executable and external hosts call the same implementation. A small public business module
interface permits ordered registration of the migrations, permissions, menu metadata, authorized routes,
and events already required by generated Admin modules. It does not permit replacing authentication,
Session, CSRF, authorization, configuration ownership, or core middleware.

`web/antd-v6` remains the only maintained frontend source. It is packaged as one complete npm package
with stable exports and a build CLI. External business pages join the same Umi route tree before the
fallback routes and compile into the same `dist`. The package does not become a collection of
independently versioned runtime, shell, auth, layout, or contract packages.

The preview package is published as `@mss-boot-io/admin-web` in GitHub Packages and linked to this
repository. A generated Thin Host commits only the scoped registry mapping and an environment-backed
`NODE_AUTH_TOKEN` placeholder. Local consumers use a classic GitHub token with `read:packages`; GitHub
Actions uses its short-lived `GITHUB_TOKEN` with `packages: read`. No expanded token is written into a
generated repository, lockfile, report, or package artifact. GitHub creates the first npm package as
private; the release manager changes it to public in the package settings after verifying the exact
version, source commit, integrity, and repository binding. The Root release gate then requires public
visibility before it can publish.

`management-system` becomes the single recommended thin-host Blueprint. It generates application glue,
business source directories, machine contracts, deployment files, and CI while depending on the
versioned Admin backend and frontend artifacts. It does not copy Admin core sources, `mss-boot`, the
complete V6 source tree, Foundation documentation, or Foundation release workflows.

One distribution upgrade coordinates the backend Go module version, frontend npm version, generated
business glue, lock, and manifest. Existing read-only planning, explicit apply confirmation, three-way
ownership, conflict failure, unknown-file preservation, and commit-last baseline rules remain in force.

## Stable boundaries

The intended public contracts are:

- the Admin Distribution semantic version;
- the context-aware complete Admin application entrypoint;
- the minimal compile-time business module registration interface;
- the AdminModule schema and target project layout contract;
- one complete Admin Web package, its documented exports, and its build CLI;
- the thin-host project, lock, manifest, generation, verification, and upgrade semantics.

Internal Admin packages may continue to evolve behind these boundaries. Downstream business code must
not import Foundation `internal` packages or package-private V6 source paths.

## Security and privacy

The browser retains the existing HttpOnly Session, signed CSRF, request, refresh, and WebSocket ticket
protocol. Backend authorization remains authoritative. Business routes are published only after database
and authorization migrations pass readiness and are mounted behind the complete Admin middleware chain.

Package and generated-host allowlists exclude credentials, logs, reports, caches, temporary output, and
local absolute paths. Creation, installation, execution, verification, and upgrade add no telemetry,
import records, adopter registry, analytics, runtime call-home, or user data collection.

## Compatibility and migration

The official Admin commands, configuration paths, environment variables, container entrypoints, Swagger,
and version injection remain compatible. Supplier moves from a server-specific mount to the same public
module contract used by external hosts, preserving request-scoped database access, migrations,
authorization, CRUD, export, events, and tests.

Historical full-copy Blueprint manifests are recognized as a legacy ownership model. They are not
silently reclassified as thin hosts. Transition requires a reviewed conflict-aware plan, preserves unknown
business files, and records a thin baseline only after all file operations and validation succeed.

## Release and rollback

Technical artifacts may use root, `mss-boot/`, `admin/`, and `web/antd-v6/` tag namespaces, but their
exact semantic version, including a prerelease suffix, must match for a coordinated distribution. The
current public consumption rehearsal uses `v1.3.0-rc.6`; it is marked as a prerelease and does not replace
the current `v1.2.3` stable release. Publication remains restricted to the exact merged-main commit and
the protected Framework -> Admin -> Frontend -> Root release train.

The immutable `v1.3.0-rc.1` train remains partial evidence. Framework, Admin, and the multi-architecture
frontend image published, but a workflow verifier defect stopped the frontend package and GitHub Release;
the root tag and Release were never created. The repair advances the coordinated version instead of moving
or reusing any published tag or artifact.

The immutable `v1.3.0-rc.2` train also remains partial evidence. Framework, Admin, and the corrected
multi-architecture frontend image published and passed identity checks, but npm rejected the prerelease
package because the publish command omitted an explicit dist-tag. The repair assigns `next` to prereleases
and `latest` to stable releases, adds a distribution-tag assertion, and advances the coordinated version.

The immutable `v1.3.0-rc.3` train remains partial evidence as well. Framework, Admin, the corrected
multi-architecture frontend image, and `@mss-boot-io/admin-web@1.3.0-rc.3` published from the same exact
commit. Package integrity reconciliation succeeded on a clean rerun, but the post-publication verifier
used `npm view <package> dist-tags --json`, which returns an empty result against GitHub Packages. The
frontend GitHub Release and root Release were therefore not created. The repair uses the npm CLI's
supported read-only `npm dist-tag ls <package>` interface with bounded verification and advances the
coordinated version without moving or overwriting any rc.3 tag, package, or image.

The immutable `v1.3.0-rc.4` Framework, Admin, and frontend releases published from
`3dddd01c4d3b70be13fb9ff53438505805ea6087`; the root tag exists but its GitHub Release does not. The
retained Thin Host exposed a migration-order defect: globally sorting module migrations let the core
example cleanup run after the composed Supplier authorization seed and remove its menu. Root candidate
evaluation also rejected the checkout after workspace-mode dependency download changed `go.work.sum`.
RC5 executes Core then Business phases, forward-repairs the stale Supplier authorization ledger, proves
sidebar navigation in the external browser gate, and keeps dependency setup from mutating the release
checkout. RC4 refs and artifacts remain unchanged.

Before publication, rollback is a normal revert of the feature commits. After downstream adoption,
rollback pins the previous Admin Distribution and restores the previous thin-host lock and manifest.
Published tags remain immutable.

## Rejected alternatives

- **Long-lived Foundation forks:** preserve duplicated ownership and make security upgrades progressively harder.
- **Admin microservices:** convert source isolation into distributed identity, deployment, and availability problems.
- **Qiankun, Module Federation, iframe, or remote entry:** introduce a second frontend runtime and cross-application Session and routing boundaries.
- **Multiple independently versioned Admin npm packages:** permit unsupported shell, auth, layout, and contract combinations.
- **Go plugins or downloaded business code:** weaken portability and introduce runtime code-loading risk.
- **Runtime dynamic models or browser generation:** conflict with the repository's removed developer-tools boundary.

## Consequences

Downstream repositories become substantially smaller and upgrades operate on thin glue rather than copied
product sources. In exchange, the complete Admin application entrypoint, business module API, frontend
exports, and coordinated version become compatibility surfaces that require external-consumer gates.

The implementation has passed the repository-external Supplier host gate: GOWORK=off backend validation,
tarball frontend validation, single-runtime analysis, deterministic generation and upgrade tests, and the
required browser E2E described by the Feature contract. Publication remains separately gated on PR merge,
the exact merged-main commit, remote qualification, and coordinated immutable artifacts.

Because the npm package deliberately carries package-owned build tools for thin hosts, its distribution
metadata partitions every published dependency into runtime or tooling roots. Runtime security findings
remain unconditionally blocking; exact vulnerable tooling resolutions require expiring acceptance and are
rejected if those versions appear in the browser bundle graph.
