# Changelog

All notable, verifiable changes to the consolidated `mss-boot-admin` foundation
are documented here. The project uses semantic versioning and component-scoped
tag namespaces.

## [Unreleased]

No unreleased changes are recorded.

## [v1.3.6] - 2026-08-27

Status: **release candidate / not yet published**. v1.3.6 is the unused
identity selected to complete the package-first distribution after the
immutable partial v1.3.5 train. The current stable and rollback baseline
remains v1.3.2 until every v1.3.6 component, package, image, tool archive, and
Release has been published and publicly reconciled from one exact merged-main
commit. This entry does not authorize installation or mixed-version adoption.

### Added

- Qualify checksummed `mss` and `mss-mcp` archives for Linux, macOS, and Windows
  on amd64 and arm64, including `BUILD-INFO`, `LICENSE`, source provenance, and
  checksum-verifying shell and PowerShell installers.
- Embed the version-matched Blueprint so an adopter can create, set up, run,
  verify, and three-way upgrade a Thin Host from an empty directory while
  consuming exact public Admin, Framework, and Admin Web packages instead of a
  Foundation clone.
- Add the governed presentation-configuration aggregate and management console.
  Its documents are sparse data over compiled capabilities; they cannot add
  code, routes, transport definitions, permissions, or runtime models.
- Add the `mss_boot_presentation_profiles` mutable aggregate table and the
  append-only `mss_boot_presentation_revisions` history table, with exact
  identity, aggregate-version, revision, digest, and idempotency constraints.

### Changed

- Make presentation configuration dormant for business pages until a trusted
  capability is compiled and registered. Existing pages therefore retain their
  compiled v1.3.2-compatible presentation after upgrade.
- Require presentation deployments to run the Admin migration first, one
  `server -a` route/permission synchronization second, and the long-running
  server last. The migrations are additive and do not rewrite existing business
  rows.
- Remove the legacy `migrate --password` and `migrate -p` bootstrap interfaces.
  Interactive setup uses hidden input, while automation may inject
  `MSS_ADMIN_INITIAL_PASSWORD` only into the one initialization process.
- Coordinate Framework, Admin, Admin Web, Root tools, the backend and frontend
  images, and the official npmjs package under one exact v1.3.6 identity. Docs
  remains a later independent publication and does not block the package train.

### Fixed

- Make a manual Root Release dispatch a candidate-only preview and make an
  exact formal Root tag push publish automatically, eliminating the second
  publish dispatch, manually selected run IDs, evidence URL, and protected
  Root-promotion handoff.
- Put all expensive tests, browser checks, dependency audits, package assembly,
  and multi-platform image qualification in the single Root preview. Formal
  tags perform only identity checks, necessary publication, and public-state
  reconciliation.
- Start Root assets, the Root image, and official npm publication independently
  from one Root tag, without cross-workflow waits or mutable image aliases.
- Serialize Root Release and official npm publication across versions, reuse an
  exact already-public result on rerun, and never move either `latest` pointer
  backward when an older tag workflow is resumed.
- Decouple Docs from the package train: Root assets, the Root image, tools, and
  official npmjs publication complete from formal tags, while Docs follows later
  from its own versioned tag.

### Security

- Enforce the presentation boundary in the backend: strict data-only schema and
  semantic validation, compiled capability hashes, strong ETags, hashed
  idempotency keys, immutable revisions, and authorization applied after every
  presentation layer.
- Keep `presentation:read`, `presentation:draft-write`, `presentation:publish`,
  and `presentation:rollback` independent. The migration grants no presentation
  access to existing ordinary roles; frontend visibility remains advisory to
  backend RBAC.
- Redact profile documents, localized values, raw subject identifiers, and raw
  idempotency keys from general audit bodies while retaining bounded transition,
  actor, aggregate, revision, outcome, and reason metadata.
- Record six time-bounded high/critical findings from the Umi package-owned
  build and inactive-plugin graph as non-runtime acceptances. Their exact
  package resolutions and 2026-11-08 expiry remain machine governed, and the
  release bundle gate fails if Vite, node-fetch, Immer, or path-to-regexp enters
  browser runtime output.

### Migration and rollback

- Back up and rehearse disaster recovery of the v1.3.2 database, configuration,
  Thin Host lock, Admin binary, and Admin Web artifact before deployment. Record
  the database backup time and how readable post-backup audit and business data
  will be exported and preserved before any full-database restore. Do not combine
  a v1.3.6 backend with a v1.3.2 frontend or vice versa.
- For a normal emergency binary rollback, first enable startup-only
  `presentation.recoveryMode`, verify that effective reads use compiled defaults,
  then redeploy only the matching v1.3.2 Admin, frontend, configuration, and
  lock. Keep the live database and its forward-compatible schema and data in
  place; preserve the additive presentation tables, immutable history,
  permission metadata, audit evidence, and business writes without a destructive
  down migration.
- Restore a full database backup only for disaster recovery. A restore discards
  every write after the backup time; before it starts, export and preserve
  post-backup audit and business data outside the target database whenever the
  current storage remains readable.
- Keep `mss-shop` outside this candidate claim. It becomes a v1.3.6 development
  exemplar only after the complete public distribution ledger and an external
  single-tenant package-consumer acceptance both pass.

### Release governance

- Record v1.3.5 as an immutable partial train after root promotion created the
  exact root tag but failed its post-push metadata check. The naturally
  triggered Root candidate and container runs were cancelled before Root
  assets, the Root Release, Docs, or the public npmjs package were published.
  No later repair may delete, move, recreate, or reuse a v1.3.5 identity. A
  complete train requires a fresh merged-main commit and an unused version;
  v1.3.6 is that candidate identity, not a continuation of v1.3.5.
- Permanently reject v1.3.5 qualification and publication independently of the
  general publication-ready switch, and require a no-bypass server ruleset to
  block late creation of every exact v1.3.5 release-tag namespace.
- Restrict release previews and every component, Root, npm, or Docs tag path to
  `lwnmengjing`, reject reruns triggered by any other account, remove manual
  environment approvals from the tag-driven path, and retain `SullivanPrime`
  as the independent pull-request reviewer.
- Consolidate all release-tag creation under one lwnmengjing-only ruleset and
  retire the Root promotion workflow, write-enabled DeployKey, environment
  secret, protected environment, and root-only creation ruleset.
- Replace incomplete v1.3.5 quick-start and deep-link commands with explicit
  v1.3.2 stable and v1.3.5 immutable-partial availability pages. Source Docs
  changes do not claim that the public site has moved from `v1.3.2+docs.1`.

## [v1.3.5] - 2026-08-26

Status: **published components / immutable partial train**. Framework
`mss-boot/v1.3.5`, Admin `admin/v1.3.5`, and Admin Web
`web/antd-v6/v1.3.5` were published from
`396f60615cdfa589353b16ef9d3531e249e65432`. The protected promotion then
created the exact annotated Root tag `v1.3.5`, but its post-push check compared
the GitHub tag message without Git's canonical trailing newline and failed.
The natural Root candidate and container runs were cancelled before they
published Root assets, the Root Release, Docs `docs/v1.3.5`, or the public
npmjs package. The Root tag is already public and Go Proxy indexed, so every
v1.3.5 identity remains immutable. The governance recovery under Unreleased is
not publication authority for this stopped train.

This coordinated patch was intended to complete the package-first distribution
from one new merged-main commit after v1.3.4 stopped as an immutable
component-partial train. Its reviewed implementation remains attached to the
exact v1.3.5 source commit, and it did not move or overwrite any v1.3.4 tag,
Release, package, or artifact.

### Fixed

- Preserve the exact Admin Web package tarball URL in generated Thin Host
  lockfiles, and keep the downstream compatibility registry fixture aligned
  with that metadata contract so frozen installs never infer a registry path.
- Provision a masked, job-scoped browser administrator password before Release
  Readiness executes qualifying phase commands, so the canonical browser suite
  retains its required isolated credential without storing a reusable secret.

### Changed

- Coordinate the Framework, Admin, Admin Web, Root tools, Docs, and official
  npmjs package under the single v1.3.5 distribution identity.
- Jointly requalify the consolidated Framework Go, Admin Web, Docs, and pinned
  GitHub Actions dependency sets instead of releasing their equivalent closed
  Dependabot proposals as separate trains.
- Keep package-first installation and upgrades as the adopter path: install the
  versioned tools, generate a Thin Host in an empty directory, and consume exact
  Go and npm packages without cloning the Foundation repository.
- Requalify the full external-consumer, release-governance, documentation,
  browser, upgrade, and public-reconciliation gates before any v1.3.5 object is
  published.

## [v1.3.4] - 2026-08-25

Status: **published components / immutable partial train**. Framework
`mss-boot/v1.3.4`, Admin `admin/v1.3.4`, and Admin Web
`web/antd-v6/v1.3.4` were published from
`d9b210d6672800f84f6403496a3ae871fb2aea9f`. Root pre-publication then found
that generated Thin Host frozen lockfiles discarded the exact Admin Web
tarball URL and inferred an invalid registry path. Root `v1.3.4`, Root tools,
Docs `docs/v1.3.4`, and npmjs `@mss-boot-io/admin-web@1.3.4` were not
published. These identities remain immutable; the repair uses v1.3.5.

This coordinated patch repaired the package-first release path after v1.3.3
stopped as an immutable component-partial train. It used a new merged-main
commit and never moved or overwrote any v1.3.3 identity.

### Fixed

- Match cmd/go's nested-module archive semantics by inheriting the repository
  root `LICENSE` when Admin or Framework has no module-local license, while
  preserving a module-local license and failing closed on uncommitted root
  license drift.
- Record the canonical public Admin Module Sum in generated Thin Host `go.sum`
  and mirror the inherited-license behavior in standalone file proxies, so an
  empty external consumer resolves the same bytes as the public Go proxy.
- Bind Framework and Admin public download probes to the exact candidate Module
  Sum and GoModSum in addition to version, source commit, and replace-free
  resolution.

### Changed

- Move the current package-first tools, Go Module, npm, Thin Host, upgrade,
  operations, security, and contributor guidance to v1.3.4.
- Archive v1.3.3 as a component-partial release: Framework, Admin, and Admin Web
  were published from `c00591f2a3edd0bec29bb1023bca8a230648107a`, while Root
  tools, the Root Release, Docs, and npmjs were not published.
- Require the repaired distribution to restart qualification from a new exact
  merged-main commit and rerun every affected release and public-consumer gate.

## [v1.3.3] - 2026-08-25

Status: **published components / immutable partial train**. Framework
`mss-boot/v1.3.3`, Admin `admin/v1.3.3`, and Admin Web
`web/antd-v6/v1.3.3` were published from
`c00591f2a3edd0bec29bb1023bca8a230648107a`. The public Thin Host gate then
found that the generated Admin checksum omitted cmd/go's inherited repository
`LICENSE`, so Root `v1.3.3`, Root tools, Docs `docs/v1.3.3`, and npmjs
`@mss-boot-io/admin-web@1.3.3` were not published. These identities remain
immutable. The first repair used v1.3.4, which also stopped as a component-
partial train; the coordinated complete repair uses v1.3.5.

This patch defines the package-first Admin Distribution: a user installs the
versioned tools, creates a Thin Host in an empty directory, and imports the
coordinated Go and npm packages without cloning the Foundation repository.
Publication remains governed by the immutable component tags and GitHub Release
attached to the exact merged-main commit.

### Added

- Publish checksummed `mss` and `mss-mcp` tool archives for Linux, macOS, and
  Windows on amd64 and arm64, with non-privileged shell and PowerShell installers,
  complete build provenance, exact archive contents, and immutable checksums.
- Embed the version-matched application Blueprint and managed templates in the
  released `mss` command so `mss new app` and `mss upgrade admin v1.3.3` do not
  require a Foundation checkout.
- Generate a complete Thin Host baseline with lockfiles, local-development
  contract, repository README, Agent Skills, validation, and CI entrypoints.
- Add explicit Thin Host registries and bilingual locale catalogs that merge
  generated modules, routes, and messages with ordered handwritten backend
  modules, page routes, authorization metadata, and zh-CN/en-US messages;
  custom code stays outside generated files and survives three-way upgrades.
- Fail closed when any handwritten or generated business route duplicates an
  Admin-owned UI or server path, and require handwritten protected handlers to
  enforce explicit backend permissions instead of relying on session auth or UI
  visibility.
- Distribute an exact, validated Thin Host Skill set for basic ownership, module
  and supported-field generation, coarse backend permissions, debugging, review,
  and coordinated upgrades; unsupported relation, workflow, import, and row-scope
  generation now fails closed instead of being advertised.
- Add an installed-consumer qualification that starts from an empty temporary
  directory and forbids Git checkout, local Go replacement, local npm tarballs,
  and hand-written lockfiles.
- Prompt for the first local administrator password through hidden terminal input
  in `mss setup`, while retaining a one-use environment contract for
  non-interactive automation and keeping the secret out of arguments and reports.
- Let `mss spec init --kind feature --module <name>` target a primary AdminModule
  whose name differs from the user-visible Feature, with deterministic write and
  no-overwrite behavior.
- Make Thin Host verification work in an unborn Git repository, inspect untracked
  text safely, and fail when any AdminModule specification has stale generated
  backend, frontend, migration, permission, test, or documentation projections.

### Changed

- Route immutable root-tag creation through the protected promotion workflow and
  its single dedicated SSH deploy key after exact `pre-root` authority and all
  component Releases; every exact-tag root run rechecks that attestation before
  candidate work.
- Refresh the coordinated Framework, Admin Web, Thin Host, Docs, and pinned
  GitHub Actions dependency sets, including Ant Design 6.6.1, Umi Max 4.7.7,
  Vite 8.2.2, and Vitest 4.1.11, without introducing a second runtime graph.
- Recalibrate the complete frontend gzip JavaScript budget from 900 KiB to
  905 KiB against measured 899.96 KiB and 900.47 KiB before/after release builds;
  entry and largest-chunk limits remain unchanged.
- Treat Linux procfs `ESRCH` during an already verified stop sequence as a
  completed process exit while continuing to reject PID reuse and unverifiable
  identities; keep the frontend lockfile deduplicated under the release pnpm.
- Keep the root command module free of `replace` and `exclude` directives so
  standard `go install .../cmd/mss@v1.3.3` and `mss-mcp` work outside the
  Foundation checkout.
- Make the package-first quick start the only adopter path; clone-based commands
  are confined to an explicitly labelled Foundation contributor workflow.
- Rewrite the active documentation around the coordinated v1.3.3 Admin,
  Framework, and Admin Web imports, one Thin Host lifecycle, and one upgrade path.
- Require upgrade guidance to verify target-version tools and the Blueprint
  manifest, back up state, review and apply a three-way plan, run full validation,
  and prove a second no-op plan; document MCP stdio configuration and empty-root
  boundaries explicitly.
- Compute generated module-document links from each target layout so both the
  Foundation projection and Thin Host output point to their in-repository source
  specification.
- Keep `--foundation` only as an explicit local override for Foundation
  contributors; it is no longer required by normal creation or upgrade commands.
- Calculate both candidate Go Module archives before any component tag, require
  Admin to record the exact Framework sums, and require the generated Thin Host
  to record the exact final Admin sums so a source-tree change cannot ship a
  stale `go.sum` baseline.

### Removed

- Remove the internal `mss-pr` coverage commenter from the public tool surface and
  keep its small CI responsibility inside the workflow that owns it.
- Remove archived AIGC prompts, one-off handoffs, superseded roadmaps, duplicate
  quick starts, and stale release plans from the active source and Docs tree.

### Security

- Verify every tool archive checksum, exact root file set, version, commit, and
  RFC3339 build timestamp before installation, without using `sudo` or modifying
  shell profiles.
- Scope first-administrator credentials to one migration execution context so
  concurrent database initialization cannot exchange identities, and remove the
  one-use secret from dependency, verification, and long-running development
  processes with centralized output redaction and private reports.
- Bind development readiness and stop operations to the exact operating-system
  process start identity, serialize lifecycle changes across CLI processes, and
  refuse to signal a reused or unverifiable PID even with a force request.
- Generate a non-root, multi-architecture Thin Host image from exact base-image
  digests with CA certificates, timezone data, a real health check, and a build
  context that excludes Git metadata, secrets, local configuration, databases,
  logs, reports, and dependency caches.

## [v1.3.2] - 2026-08-23

Status: **published / current stable**. Root, `mss-boot/v1.3.2`,
`admin/v1.3.2`, `web/antd-v6/v1.3.2`, `docs/v1.3.2`, and
`@mss-boot-io/admin-web@1.3.2` were qualified and published from exact merged-main
commit `635fbb03a82976941e527d8ac1000fec0624abac`. Public checksums, npm provenance,
multi-architecture image digests, Docs identity, and immutable Release assets are
indexed in [issue #519](https://github.com/mss-boot-io/mss-boot-admin/issues/519).
The public npm `latest` tag resolves to `1.3.2`; npm Trusted Publishing is bound
to repository `mss-boot-io/mss-boot-admin`, workflow `npm-release.yml`, and
environment `release-v6`. The one-time bootstrap npm token plus GitHub
`NPM_TOKEN` secret have been removed.

The v1.3.1 train is immutable component-partial history. Framework
`mss-boot/v1.3.1` was published from
`4830fee162326788732e476a04d24f47b8fd570a`. The `admin/v1.3.1` tag points to
the same commit, but its workflow stopped before creating an Admin Release when
the committed Framework Module sum differed from the canonical public module.
No v1.3.1 root, Admin Web, Docs, or npmjs release completed. The repair used a
new v1.3.2 patch and never moved or reused those identities.

The v1.3.0 train is immutable component-partial history. Framework
`mss-boot/v1.3.0` was published from
`76530526e436eb95652df1dd06e831a90ee73125`, while the Admin workflow stopped
after creating `admin/v1.3.0` because the independent module metadata lacked the
published Framework checksums. No v1.3.0 root, Admin Web, Docs, or npmjs release
completed. Those identities were not moved or reused by the v1.3.2 release.

The complete v1.3.0-rc.6 preview train was published successfully from
`0ef09fb3caa1b2d424c540da23d01219135ebcfa`. Its Framework and Admin modules,
Admin Web package, portable assets and image, root image and candidate assets
remain immutable prerelease evidence. They did not replace v1.2.3 and are not
moved or overwritten by the completed v1.3.2 release.

### Added

- Publish one coordinated Complete Admin Distribution while retaining independent
  root, Framework, Admin, Admin Web, and Docs release identities.
- Add the importable `admin/app` composition root and explicit `admin/business`
  module boundary without package-initialization route or migration side effects.
- Add a compact Thin Host Blueprint that pins exact backend and frontend
  dependencies, preserves owned business files, and upgrades managed glue through
  a conflict-aware three-way plan.
- Publish the complete React 19 and Ant Design 6 application as
  `@mss-boot-io/admin-web` with package-owned Umi integration and external-consumer
  install, lint, test, build, bundle, and browser qualification.
- Publish one qualified Admin Web tarball to both GitHub Packages and npmjs;
  npmjs is the credential-free default for Thin Hosts, while the protected
  official-npm mirror step runs only after every coordinated Release is public.
- Execute composed Admin migrations in explicit Core then Business phases while
  retaining a collision-checked combined runner for source compatibility.
- Add an idempotent forward migration that recognizes the RC4 Supplier collision,
  clears only its stale authorization ledger marker, and lets the business phase
  rebuild menus, API inventory, policies, revisions, and generated default roles.
- Require generated browser qualification to prove the Supplier path exists in the
  authorized menu and to enter it through the visible sidebar before CRUD checks.

### Changed

- Promote the successfully qualified RC6 contract to the stable v1.3.2 target
  while retaining the exact merged-main, staged publication, external resolution,
  immutable artifact, rollback, and public-reconciliation gates.
- Calculate the exact v1.3.2 Framework module and `go.mod` checksums from every
  tracked file in the final tree, compare them with Admin metadata at checkpoint,
  pre-Framework, Framework release, and Admin release gates, and then resolve the
  published dependency without a workspace or local replacement.
- Harden external Admin route registration, package command resolution, and API
  registry synchronization so missing or malformed distribution extensions fail
  closed during qualification instead of surfacing after publication.
- Preserve the command-construction signatures exposed by the immutable
  `admin/v1.3.0` tag as deprecated fail-closed sentinels; they return no
  executable Admin tree, while all real runtime execution remains confined to
  the guarded `admin/app` entrypoints.
- Keep prerelease root images from advancing the mutable `latest` tag; only a
  stable SemVer root tag may publish `latest`, and fail publication unless the
  stable version tag and `latest` converge on the exact published manifest digest.
- Give the Admin Web image an explicit frontend OCI description instead of
  inheriting the root repository's Go-backend description.
- Preserve configured generated parent-menu icons, make the Supplier menu path
  visibly reachable, and mark decorative navigation icons as hidden from assistive
  technology without removing their accessible menu labels.
- Reset generated create and update editor state from current inputs whenever a
  modal reopens, preventing cancelled or stale values from leaking into the next
  operation.
- Scope generated enum-selection checks to the exact listbox named by each
  combobox's `aria-controls`, so duplicate labels in separate fields remain
  semantic and strict-mode safe.
- Bound the downstream Blueprint evaluation to a compact 30-64 file Thin Host
  envelope, rejecting both incomplete hosts and regressions that copy Foundation
  core sources.
- Run the complete Agent evaluation during feature freeze so this contract fails
  before any Framework, Admin, frontend, or root tag is created.
- Execute every required pre-Framework and pre-root Feature command before a
  publication-authority attestation can be issued; checkpoint remains plan-only.
- Download Admin module dependencies with `GOWORK=off` and assert the release
  checkout remains unchanged before Blueprint evaluations.
- Keep `mss spec init --kind module` synchronized with the complete Ant Design 6
  AdminModule contract, including export, browser marker, UI, permission, and
  deterministic module-specific migration identities.
- Fail the Docs build when a root-relative Markdown link has no corresponding
  portable static target, including the immutable RC4 and RC5 audit routes.
- Add an explicit module-scoped marker for compatibility migrations that query a
  generated module's ledger IDs without falsely claiming those IDs in generator
  collision detection.
- Replace stale beta/support, contributor, release-phase, configuration, and
  repository-description guidance with the executable v1.3.2 distribution
  contract, including mandatory API registry synchronization.

## [v1.2.3] - 2026-08-19

Status: **published / previous stable**. The synchronized root, Framework, and V6
frontend releases completed from `260d546851c58f7293b30e76b47d40d8e89f52fe`.

### Changed

- Cross-compile target Go binaries on the native BuildKit build platform using
  explicit `TARGETOS` and `TARGETARCH`, preventing arm64 Go compilation from
  running under QEMU.
- Raise the bounded root image publication job limit from 40 to 90 minutes and
  add a workflow contract test that prevents either safeguard from regressing.

## [v1.2.2] - 2026-08-19

Status: **component-partial / immutable**. Framework and the sole V6 frontend
were published from `29a99f2bdb9a2c516459529918795404c153df2e`. Root candidate
qualification passed and the root tag was created, but the multi-architecture
image publish timed out after the arm64 QEMU build; no root image or GitHub
Release was completed. The next complete coordinated train was v1.2.3.

### Changed

- Normalize Umi/Dumi literal dynamic-route HTML placeholders out of Admin and
  documentation release output,
  fail closed on unexpected placeholder content, and set deterministic static
  directory/file modes to 755/644 so restrictive build umasks cannot cause Nginx 403.
- Added one cross-platform path validator for directories, ZIPs, and TARs. It
  rejects forbidden/control characters, traversal, trailing spaces or periods,
  Windows device names, case-insensitive collisions, unsafe links, and overlong
  components before artifact or Release publication.
- Made both frontend workflows transport only a checksummed `dist-v6.tar.gz`,
  and made root assembly verify/extract it before validating all six final ZIPs.
  Workflow tests inventory every remaining raw directory upload and require a
  preceding portability guard.
- Extended production delivery smoke to a concrete dynamic deep link after the
  placeholder removal, in addition to hashed-asset, cache, and missing-file checks.
- Isolated Playwright qualification on port 18001 with non-persistent compiler
  cache state so a running port 8001 developer process cannot supply cookies
  from a different backend signing key or hold the E2E compiler lock.

## [v1.2.1] - 2026-08-19

Status: **component-partial / immutable**. Framework and the sole V6 frontend were
published from `80d2d20f1b44105e18706cfa0deb7f8512966f92`. The root tag and runtime
image were created, but root package assembly and GitHub Release publication did
not complete because the root workflow still uploaded raw V6 output. v1.2.2
repaired that artifact path but later stopped at root image publication; the next
complete coordinated train was v1.2.3.

### Changed

- Made V6 publication upload only the portable `dist-v6.tar.gz`, build identity,
  and checksum manifest instead of traversing the raw `dist` tree. A workflow
  contract test prevents the non-portable directory upload from returning.
- Made the V6 code-layer theme default dark while retaining application and
  personal overrides with their existing precedence.
- Made `web/antd-v6` the sole Admin browser application and removed the retired
  frontend source, generator projection, dependency automation, CI/release/deploy
  paths, active documentation, and rollback image. Root distribution and rollback
  now use only qualified V6 frontend/backend pairs.
- Made `antd-v6` the only module-generation target. The Supplier golden produces
  V6 routes, locale catalogs, strict response contracts, React Query CRUD, and an
  HttpOnly/CSRF-aware Playwright flow; CI checks the one generated projection for drift.
- Removed token-returning browser login/refresh, bearer OAuth callback mode,
  query-token WebSocket authentication, the historical `jwt` cookie, unversioned
  theme projections, and missing-revision mutation fallbacks. Standard REST Bearer
  and PAT authentication remain supported for documented non-browser automation.
- Made the V6 browser session and durable server-side session check mandatory.
  The backend no longer exposes switches that can restore stateless browser JWT
  behavior; production still configures Secure/SameSite cookies, trusted origins,
  strong keys, shared session state, and one-time WebSocket ticket lifetime.
- Added forward-only Language, Option, account-reauthentication, example-Supplier
  retirement, and retired-V5 configuration migrations for fresh and upgraded
  V6 deployments.
- Made the one-shot `STAGE=local go run . server -a` route synchronization an
  explicit setup and upgrade contract. A healthy process with an empty API registry
  is not ready for menu API binding.
- Preserved the source-compatible Framework hardening in the synchronized v1.2.1
  forward repair for transactional
  delete controls, fixed public write-error mapping, constant-time password
  verification, and non-reversible SecretRef fingerprints.
- Added a repository-wide LF checkout contract and explicit pnpm pins for the
  pnpm 10.34.5 V6 application and pnpm 9.15.9 documentation site so WSL release
  checks do not depend on a Windows Git or global package-manager setting.
- Made feature-freeze release readiness install the repository-pinned Playwright
  Chromium and operating-system dependencies before executing V6 browser evidence,
  so clean GitHub runners no longer fail before E2E execution begins.

## [v1.1.0] - 2026-08-11

Status: **published / historical**. This section is retained as immutable release
history. The next complete coordinated train was v1.2.3; the current stable train
is v1.3.2.

### Changed

- Enabled the protected v1.1.0 publication path after the scoped readiness runner, exact-run
  attestation, required-reviewer `release` environment, and immutable release-tag rulesets were
  installed and verified. Publication still requires exact-SHA pre-framework and pre-root authority.
- Added the D2 canonical-email development checkpoint for the bundled Admin:
  a full-ID forward migration preflights existing active identities without
  disclosure, performs compare-and-swap canonical backfill, and installs an
  active/non-empty unique identity using SQLite/PostgreSQL partial expression
  indexes or a MySQL nullable stored `VARBINARY` key. Real MySQL/PostgreSQL
  integration tests exist, but must be rerun with both DSNs and zero skip from
  the selected feature-freeze SHA; current development runs are not release evidence.
  Migration completion and Admin startup now share an exact redacted schema verifier,
  and the server mounts business routes only after that readiness gate passes.
- Added the D2 downstream-snapshot identity consumer checkpoint at `151a91c`:
  CLI upgrade status, MCP, and doctor now read one strict SnapshotStatus carrying the
  independent Foundation, Blueprint, generator, and downstream identities plus atomic
  lock/manifest digests. The source checkout is recognized only by its exact legacy
  development sentinel; malformed, orphaned, or source-to-generated transitional state
  cannot fall back to a false source classification, and upgrade planning no longer
  treats the nested Admin module or project generation baseline as a runtime identity.
  The checked-in compatibility workflow has a static contract test, but no real GitHub
  Actions run was performed for this development checkpoint. Feature freeze still
  requires an exact-SHA run that proves all four identities and digests, a real Blueprint
  0.1-to-0.2 customized upgrade, and an empty second upgrade.
- Added the D2 strict runtime-configuration checkpoint: exact-key YAML/JSON
  decoding, explicit Redis deployment modes, typed SecretRefs, immutable
  snapshots, and side-effect-free plans. Provider construction, health, and
  real Redis deployment conformance remain feature-freeze work.
- Added the D3 domain-neutral Runtime v2 resource graph at `d90b4c7`, with its
  deterministic close-generation evidence repaired at `c830b5f` and its public
  provider error tree repaired at `c57ffc8`, and
  deterministic side-effect-free graph preflight, topological startup, required
  readiness before dependent start, reverse rollback/close, graph-owned Run
  cancellation, concurrent idempotent close with retry after a bounded failure,
  and redacted lifecycle errors that preserve `errors.Is` classification without
  exposing provider objects or text through recursive unwrap/`errors.As`. Hermetic checkpoint
  evidence covers the state machine and owned handles; real provider health,
  Admin readiness-before-listen composition, and goroutine/file-descriptor leak
  bounds remain feature-freeze gates.
- Added the D3 additive named Redis resource at `86c0e8a`. One normalized Redis
  profile owns exactly one delayed standalone, Sentinel, or cluster go-redis client;
  stable resource-and-scope-prefixed capabilities lend structured caller-bounded
  leases without exposing the client or `Close`. Start/Ready/Health and commands
  honor caller deadlines, missing keys use a provider-neutral sentinel, and one
  tracked close generation invokes the context-free provider close exactly once.
  Twenty-two fully anchored top-level tests pass twenty uncached race-detected runs,
  including standalone miniredis and stalled-socket deadlines. The aggregate remains
  Planned: Sentinel control-plane ACL is anonymous, cluster multi-key operations are
  non-atomic with partial counts, and real Sentinel/cluster/TLS, Admin composition,
  Challenge injection, and leak conformance remain open.
- Added the D3 Framework Challenge checkpoint at `1faa9ef`. The public additive
  `runtime/challenge` package accepts one named Redis Scope, exposes explicit
  Begin/Commit/Abort/Verify outcomes without a raw client or `Close`, and uses an
  internal opaque same-slot bridge limited to fixed repository scripts. Rate-operation
  replay is idempotent at the limit boundary, and every syntactically valid Verify path
  performs one fixed read plus one fixed completion script. The deprecated D0 exported
  surface remains source-compatible and receives the same replay and redaction repairs.
  Twenty-two newly introduced top-level tests passed fully anchored, uncached `count=1`
  race evidence with `GOWORK=off`. Admin composition and real Redis Cluster/failover
  remain pending, so no capability is promoted to Stable.
- Composed the D3 Challenge runtime into Admin at `3e9ca94`. Startup now builds the
  named Redis resource `main` and Scope `challenge.email`, completes Start/Ready before
  publication and later route/listener assembly, and gives Config sole bounded-close
  ownership. Optional invalid or unavailable configuration keeps the application up but
  makes Challenge issuance and consumption return fixed 503 without falling back to the
  legacy global Redis path; required failure blocks setup. FakeCaptcha uses
  BeginIssue, SMTP delivery, and Commit/Abort, while login, registration, and password
  reset consume VerifyOutcome. Thirteen selected Admin top-level tests passed fully
  anchored, uncached `count=1` race evidence with `GOWORK=off`. Runtime configuration
  remains a startup snapshot, so changes require restart; browser and frozen-SHA
  provider/lifecycle gates remain pending and no capability is promoted to Stable.
- Added the D5 scoped runtime-cache checkpoint at `88f40c3`. The additive
  `mss-boot/runtime/cache` package declares database authority, Scope namespace,
  TTL, payload bound, provider-bypass, and loader reconstruction; provides
  singleflight plus generation invalidation; preserves not-found and RowsAffected;
  and bypasses shared state for active GORM transactions. QueryCache is an explicit
  opt-in loader adapter rather than a transparent plugin: callers own payload codecs
  and stable non-sensitive query identities, while cross-process outage recovery
  remains coupled to EventBus/database-revision reconciliation. Eight exact new tests
  passed uncached `count=1` race evidence with `GOWORK=off`; Planned/Beta status and
  the feature-freeze rerun remain unchanged.
- Added the D5 revision EventBus and Admin authorization-reconciliation checkpoint.
  Commit `04e8e0c` introduces an additive typed `mss-boot/runtime/eventbus` with
  process-local current-subscriber Memory fan-out, Redis Scope polling of the latest
  revision, panic isolation, degraded health, caller-bounded lifecycle, and a
  domain-neutral authoritative reconciler. Commit `160e2df` makes canonical Casbin
  mutation and its global `ConfigRevision` one transaction, publishes only after the
  commit, reloads authoritative policy in the subscriber, and registers a Memory
  runtime plus periodic reconciliation with the Admin server manager. Publication
  failure never rolls back an already committed policy, while missed, duplicate,
  out-of-order, panicking, disconnected, and commit-before-publish cases remain
  repairable without WorkQueue acknowledgement semantics. Exact uncached `count=1`
  race evidence covers seven Framework and eight Admin top-level tests; Framework
  runs with `GOWORK=off`, while Admin uses the current workspace until the unpublished
  v1.1.0 Framework dependency can be tagged and updated. EventBus is Beta and the
  aggregate Runtime v2 capability remains Planned pending the frozen-SHA rerun, real
  Redis multi-replica/failover evidence, and remaining runtime gates.
- Added the D5 provider-evidence validation checkpoint at `668dfe3`. The root CLI now
  strictly loads a repository-confined `ProviderMaturityReport`, validates pinned
  version/commit/fixture identities and internally consistent result counts, emits a
  deterministic normalized report, and makes required zero-run, skip, failure, partial,
  cached-only, or empty selections fail. Optional rows remain visible and non-blocking.
  The command only validates a supplied artifact: it starts no provider, creates no real
  provider report, and does not promote ObjectStore, RustFS, or any other provider.
- Added the cumulative D3 Supplier generator checkpoint. Commit `5a60ad6` projects the
  canonical AdminModule into an explicit lossless-ID SQLite/MySQL/PostgreSQL forward
  migration, model, validated DTOs, CRUD/query/export service, typed post-commit events,
  authorized HTTP operations, OpenAPI annotations, and exact generated tests under
  `admin/modules/supplier`. Commit `d92458c` adds the independent authorization migration
  `20260811120000`, persists the parent/menu plus hidden permission COMPONENT/API metadata,
  seeds exact admin/procurement/finance Casbin policy and role/global revisions atomically,
  and binds the AdminAuthorizer to canonical identity, HTTP method, full Gin path, declared
  root bypass, and ownership mode `none`. Route composition still fails closed without an
  explicit authorizer. The current dry-run honestly reports `phase=backend-checkpoint`,
  template `1.1.0-backend.3`, `complete=false`, 19 unchanged managed files, and 19 deferred
  frontend/documentation/E2E projections. Real MySQL 8.4 and PostgreSQL 17 migration tests
  passed at the earlier development checkpoint with zero skip, but must rerun on the selected
  feature-freeze SHA. Typed client, frontend/UI/browser E2E, generated module docs, and the
  customization-preserving upgrade rehearsal remain D5 work.
- Switched the next train to development-first v1.1.0: internal waves remain
  untagged, publication is disabled by checked-in policy, acceptance evidence is
  phase-scoped, and complete release qualification starts only after feature freeze.
- Reconciled the root and nested-framework changelogs, release FeatureSpec, and
  release documentation with the public `v1.0.0` evidence.
- Reclassified the aggregate cache/lock/queue adapter capability from stable to
  legacy and added provider-specific evidence guidance.
- Reduced the Admin Storage AppConfig and UI surface to the upload-admission
  `storage:maxSize` and `storage:allowedTypes` fields. Provider selection and
  SecretRef-backed credentials are reserved for the immutable startup profile.
- Replaced the object-storage provider map with an exact Local-or-S3 startup
  configuration, immutable normalized profiles, explicit credential modes, and
  typed environment SecretRefs. Admin startup configuration must migrate to this
  strict contract and neither provider is promoted. The nested framework retains
  a deprecated v1.0 source bridge for the removed storage symbols, but it rejects
  implicit credential fallback and is not the authoritative runtime path.
- Kept the framework `AdapterQueue` surface source-compatible and added the
  additive `ManagedAdapterQueue` lifecycle contract. Kafka configuration and
  registration now use caller contexts and return errors; Admin owns the managed
  adapter and registers its blocking `Start` with the server `Runnable` manager.
- Made migration registration lossless and fail-fast: full decimal identifiers are
  ordered without integer truncation, duplicates fail before any schema access,
  the Admin migrator propagates context/errors, and explicit v1.0.0 aliases prevent
  historical 10/13-digit marker rows from rerunning. The generated migration
  template now registers the complete filename identifier.

### Fixed

- Stopped the Account Settings profile form from resubmitting the immutable
  authentication email on every unrelated profile update.
- Moved the legacy Kafka adapter's Sarama offset mark after JSON decoding and
  synchronous handler success, propagated the consumer-session context into the
  message, and stopped canceled or closed consumer loops without marking unfinished
  work. D1 additionally removes registration/configuration Exit/Fatal paths, owns
  one producer and one consumer group per unique topic/group, observes consumer
  errors, rejects new work during close, and provides cancellable start plus
  idempotent, deadline-bounded, retryable close. The adapter remains
  Legacy/Blocked: hermetic evidence does not prove broker commit, manual commit,
  retry/backoff, dead-letter, rebalance, idempotency, outage, or real-broker behavior.
- Removed the Admin ghost storage initializer and per-upload S3 constructor.
  One composition-root owner now installs a leased framework Handle and pinned
  filesystem before registering Application delivery, then closes them with bounded,
  idempotent drain semantics. S3 config
  bootstrap owns a separate client and closes response bodies on every read path. Only a
  missing stage object is optional; read/malformed-overlay failures fail closed, and HTTP
  requires the explicit `s3_tls_allow_insecure_http=true` opt-in.

### Security

- Bounded both authenticated upload routes before multipart parsing with a
  100 MiB configuration ceiling, a fixed 64 KiB multipart envelope budget,
  max-plus-one stream validation, stable 413/422 responses, and deterministic
  multipart spill cleanup. Local and S3 keys are now opaque UUIDs; Local writes
  use an `os.Root`-confined create-only path and remove canceled or partial
  files. Local and S3 remain Legacy/Blocked until the D4 object metadata,
  provider conformance, authorization, and Delivery gates close.
- Added the provisional D0 Redis challenge state machine for email login,
  registration, and password recovery. It uses cryptographic fixed-width codes,
  purpose/subject HMAC keys, versioned peppered verifiers, delivery
  Begin/Commit/Abort CAS, pending leases, cooldown and rolling quotas, bounded
  caller/global issuance, bounded context-aware SMTP delivery, bounded attempts,
  and exactly-once successful consumption.
- Removed the Admin application's fallback to the legacy email-only
  verification-code adapter, made Redis/secret failure explicit, made issuance
  responses account-independent, enforced email/register switches at issuance
  and consumption, added fresh Redis-plus-SMTP readiness and an explicit
  trusted-proxy policy, removed codes from email headers, and removed the
  misleading phone-code login tab until the separately planned phone challenge
  capability exists.
- Canonicalized bounded ASCII email identities consistently, failed closed on
  ambiguous lookup, and made registration plus first-time GitHub/Lark OAuth
  provisioning create new users atomically without provider-email account merge.
  Bounded opaque usernames protect the legacy `varchar(20)` field, while only
  the named email constraint becomes a fixed identity conflict. Admin create/update
  now return redacted fixed 422/409 responses and unrelated database errors retain
  the generic safe fallback. Self-service email mutation remains disabled. The
  fail-closed schema-readiness development checkpoint is complete, but the
  capability stays Planned until both readiness and real-database suites pass
  again on the exact feature-freeze SHA.
- Stopped projecting historical storage provider or credential AppConfig rows and
  reject every removed storage key with a stable 422 before any mutation. Secret
  read/write capabilities cannot restore this retired configuration surface.
- Made absent, invalid, unresolved, closing, or unavailable object storage return
  a fixed 503 from both upload routes with zero implicit Local write. Local is
  installed only for an explicit development static mapping; S3 upload stops before
  Put until the D4 ObjectStore and Delivery contracts pass.

### Documentation

- Added machine-validated contracts for the internal safety wave, Storage Runtime v2,
  the v1.1 Generator/Blueprint golden slice, and the development-first v1.1.0 release
  train, plus the owned-resource architecture decision and provider maturity matrix.
- Added the Challenge checkpoint operator note covering SecretRef setup, failure
  semantics, focused evidence, rollback, and the remaining real-Cluster gate.
- Added the Kafka Mark-after-success checkpoint note and corrected the queue
  tutorial so legacy Kafka/NSQ adapters are not presented as production-ready.
- Added the Upload admission checkpoint note with byte-unit configuration,
  handler-level admission plus route-registration evidence, rollback guidance,
  and the remaining provider and delivery blockers.
- Added the D1 Object Provider/Owner checkpoint with exact profile, owner,
  AppConfig, 503, development Local delivery, and shutdown evidence. S3 Put,
  Delivery, and pinned RustFS conformance remain deferred to D4.
- Extended the Kafka checkpoint with exact managed-interface, owner, configuration,
  Casbin registration, Admin Runnable, error-observation, and bounded-close evidence.
  D1 is complete and the next development wave is D2 contract substrate; Kafka
  remains Legacy/Blocked pending its dedicated real-broker and delivery suites.
- Added the D2 Canonical Email Identity checkpoint note with the dialect-specific
  migration, privacy and API boundaries, exact SQLite/model/Controller evidence,
  schemahealth/migrate/server composition evidence, forward-only recovery guidance,
  and the still-open exact-freeze readiness plus MySQL/PostgreSQL zero-skip reruns.
- Added the D2 Downstream Snapshot Identity checkpoint note with the shared
  SnapshotStatus consumer contract, strict source/generated classification, fully
  anchored local evidence commands, and the still-open exact-SHA GitHub Actions,
  Blueprint 0.1-to-0.2, second-empty-upgrade, and release-built external artifact gates.
- Added the D3 Resource Lifecycle checkpoint note and a fully anchored evidence
  command requiring every top-level `runtime/resource` test for twenty uncached
  race-detected runs, without promoting the aggregate Storage Runtime capability.
- Added the D3 Named Redis Resource checkpoint note and a fully anchored evidence
  command requiring all twenty-two `runtime/redisresource` top-level tests, while
  keeping the capability Planned and listing every deferred Provider/composition gate.
- Added the D3 Challenge Runtime checkpoint note with fully anchored `count=1` race
  evidence for only the newly introduced public API, opaque bridge, Redis Scope adapter,
  replay-safe rate script, equal valid-Verify I/O, and legacy compatibility tests.
- Added the D5 Provider Evidence Validator checkpoint note and machine acceptance for
  exactly the six new validator tests and three new CLI tests introduced at `668dfe3`.
  The feature-freeze provider report remains a separate, not-yet-generated artifact.
- Added the D5 Scoped Runtime Cache checkpoint note with its explicit policy and
  caller-owned codec/QueryIdentity boundary, all eight fully anchored development
  tests, transaction isolation, and the still-open EventBus/revision plus frozen-SHA gates.
- Added the D3 Supplier Backend checkpoint note with its exact generated surface,
  machine-readable `complete=false` boundary, anchored generator/spec/Admin evidence,
  three-dialect development matrix, forward-only recovery, and explicit D4/D5 deferrals.

## [v1.0.0] - 2026-08-09

Status: **published / stable**. Root `v1.0.0` and the prerequisite
`mss-boot/v1.0.0` Release both resolve to
`ee800262c035c5f4242aca1841d077554481d2c4`. The public artifacts and exact-main
approval are recorded in GitHub issue `#471`. The optional standalone
`web/antd/v1.0.0` tag was not published.

This is the consolidated foundation's first stable 1.0 boundary. It superseded
the unpublished v0.8.0 release candidate; no v0.8.0 archive, checksum, image
digest, workflow run, or smoke result was accepted as release evidence for
v1.0.0. Every required artifact and proof was regenerated from the exact
v1.0.0 commit.

Release, upgrade, rollback, and compatibility contracts are maintained under
[`docs/docs/releases/`](docs/docs/releases/).

### Added

- Consolidated the Admin application, nested `mss-boot` framework module,
  React/Ant Design frontend, documentation, machine-readable `.mss` contracts,
  deterministic generators, Skills, MCP adapter, and evaluations into one
  foundation repository.
- Added the Agent-facing `mss` CLI for context, environment setup, verification,
  specification validation, deterministic module/application generation, and
  three-way downstream upgrade planning.
- Added backend-owned CPU and memory sampling with bounded recent history. The
  Admin task server runs monitoring and session cleanup as immutable system
  jobs, separate from user-managed Task records.
- Added database-backed configuration revisions, strong ETags, conditional
  writes, revision-bound public profile caches, and owner-isolated personal
  configuration snapshots.
- Added layered theme settings with the precedence `code defaults < application
  settings < personal settings`. This capability remains **preview** until its
  external MySQL/PostgreSQL and browser acceptance gates are complete.
- Added positive and negative authorization coverage for Admin routes, static
  frontend routes, dynamic menus, application secrets, uploads, and role
  authorization.

### Changed

- Split the former root Admin Go module into:
  - `github.com/mss-boot-io/mss-boot-admin/admin` for the deployable reference
    application;
  - `github.com/mss-boot-io/mss-boot-admin/mss-boot` for the reusable framework;
  - `github.com/mss-boot-io/mss-boot-admin` for Agent/foundation tooling.
- The Admin module requires `github.com/mss-boot-io/mss-boot-admin/mss-boot
  v1.0.0`. The nested module must therefore be published and externally
  resolved before the root `v1.0.0` tag is created.
- Authentication now resolves the current user, role, enabled state, and root
  state from authoritative storage. Role/root snapshots embedded in older JWTs
  are not trusted.
- The historical root behavior remains: an enabled root identity bypasses
  ordinary Casbin policy checks. Root/default roles and root users are protected
  from destructive generic CRUD operations.
- Role authorization is a versioned whole-resource update. Reads return a
  revision and strong ETag; bundled clients send `If-Match`; stale writes return
  `412 Precondition Failed` without partial persistence.
- Personal access tokens are owner-scoped, stored only as versioned digests,
  shown in raw form once, and atomically rotated or revoked. PATs cannot invoke
  interactive account-security operations.
- OAuth authorization and callback state are server generated, single use, and
  bound to provider, intent, browser, and—when applicable—user/session identity.
  Provider access and refresh tokens are not serialized to browser storage or
  persisted in the user binding model.
- OAuth-created or historically OAuth-bound accounts are fail-closed for local
  password login until an explicit password reset restores local credentials.
- Application and personal configuration reads use the database as authority.
  Cache keys include all query dimensions and database revisions; Redis failure
  degrades to authoritative database reads rather than stale acceptance.
- System configuration remains an opaque, root-only resource. Application
  credential fields require separate `app-config:secret-read` and
  `app-config:secret-write` capabilities for non-root users.
- Generic storage upload now requires explicit `storage:upload` authorization
  for non-root users and PATs.
- Dynamic menus remain supported and are refreshed with current identity and
  permission state; frontend visibility continues to be advisory to backend
  authorization.

### Breaking API and behavior changes

- `GET /admin/api/user-auth-token/generate` no longer creates a token and
  returns `405 Method Not Allowed`. Use `POST /admin/api/user-auth-tokens`.
- JWT refresh is state-changing and uses `POST
  /admin/api/user/refresh-token`; legacy GET refresh is not supported.
- OAuth callback completion uses `POST /admin/api/user/:provider/callback`
  with `code` and `state` in the JSON body. The legacy GET callback and browser
  token binding endpoint return `405`.
- Retired `/admin/api/template/*`, runtime model/field, and virtual CRUD routes
  are no longer registered.
- Older PATs without the minimum signed identity and persisted digest contract
  may stop authenticating and must be reissued.
- Public registration and first-time OAuth account creation are disabled unless
  `security:registerEnabled` is explicitly enabled and exactly one enabled,
  non-root default role exists.
- The least-privilege default-role migration intentionally stops on ambiguous
  historical root/default-role data instead of silently retaining a privilege
  escalation path.

### Removed

- Removed Admin runtime dynamic models, model fields, virtual CRUD, browser
  template/code generation, their routes, menu entries, policies, and reusable
  runtime framework packages.
- Preserved inert historical metadata and user-created data tables during
  automatic upgrade. Their removal or export is an explicit operator action,
  not part of the release migration.
- Removed OAuth `integration` intent and the short-lived provider credential
  handle formerly used by the browser generator flow.

### Security

- Production startup rejects the public development authentication secret and
  requires a unique random `auth.key` of at least 32 bytes.
- User/role disablement and role changes are re-evaluated from authoritative
  storage instead of trusting stale claims. Session termination and password
  changes revoke active server-side sessions, while PAT revocation and rotation
  invalidate their bearer immediately. Production deployments must enable
  `auth.sessionEnabled`; without it, an already-issued browser JWT can remain
  valid until expiry after a password change.
- Historical built-in OAuth credentials are sanitized only when their exact
  fingerprint matches. Provider-side rotation/revocation and repository secret
  scanning remain mandatory release gates.
- Audit logging redacts passwords, PATs, OAuth credentials, theme values,
  multipart file contents, and case-insensitive token query parameters.
- Audit and alert-history resources are read-only; generic mutation routes are
  not exposed.

### Migration

- A database backup, restore rehearsal, configuration backup, active-writer
  drain, and the preflight checks in the
  [v1.0.0 upgrade guide](docs/docs/releases/archive/v1-0-0-upgrade.md) are required.
- Run the Admin migration command before starting v1.0.0 application writers.
  The release adds or advances session/menu metadata, PAT digests, OAuth local
  password state and identity keys, permission metadata, retired-tool cleanup,
  configuration revisions, and least-privilege role data.
- Migrations are forward and idempotent, but not all effects are reversible.
  In particular, cleared PAT plaintext and sanitized credentials are not
  reconstructed by a code rollback.
- Use forward-fix by default. Restore the complete pre-upgrade database and
  configuration snapshot only when a proven compatible previous runtime is
  required; never partially edit migration version rows.

### Compatibility and release order

1. Publish `mss-boot/v1.0.0` from the reviewed release commit.
2. From outside this repository with `GOWORK=off`, resolve and test
   `github.com/mss-boot-io/mss-boot-admin/mss-boot@v1.0.0`.
3. Re-run the root release gates on the exact commit.
4. Publish root `v1.0.0`; publish a standalone `web/antd/v1.0.0` only after its
   independent production/local artifact contract passes.

`planned`, `preview`, a release branch, or an `Unreleased` changelog entry must
never be presented as a stable tag.

## [v0.7.0] - 2026-06-05

`v0.7.0` is the preceding root release baseline. Historical details and
artifacts remain available from the GitHub Releases page. Older untagged
development snapshots formerly described as `v1.0.0` were not consolidated
repository releases and provide no tag, artifact, or validation evidence for
the stable v1.0.0 release documented above.
