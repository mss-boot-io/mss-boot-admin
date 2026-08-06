# Remove runtime developer tools and schedule monitoring as a built-in system job

- Status: Accepted
- Date: 2026-08-06
- Owners: Admin Platform, Operations
- Feature contract: `.mss/features/admin-runtime-tools-removal-monitoring.yaml`

## Context

Before this decision, the Admin product exposed a `Development Tools` menu containing runtime model and field definitions,
virtual CRUD, and a repository-template code generator. Those features predate the current Agent-native
foundation. They create database schemas at runtime, mix product traffic with repository operations, and have
no compatible upgrade boundary for generated business behavior.

The repository now has a separate, deterministic development-time generation path under `cmd/mss`,
`internal/mss`, module specifications, and checked-in templates. Keeping both paths makes the product direction
ambiguous and makes it possible for new applications to choose the unsafe runtime path accidentally.

Monitoring had a separate correctness problem. `GET /admin/api/monitor` performed a blocking one-second CPU
measurement and repeated host inspection for every caller. Trend points existed only in React state, so the first
page load had one point and navigation discarded the complete history. Ant Design Charts used its light defaults
on a transparent canvas, leaving axes and grid lines effectively invisible on the real-dark card background.

## Decision

### Retire the runtime product surface

Remove the following from active source and product contracts:

- the `/develop`, `/model`, `/field`, `/virtual/*`, and `/generator` frontend surfaces;
- model, field, virtual CRUD, and template-generation Admin APIs;
- runtime model/field DTOs, services, API client types, locale keys, permissions, and menu seeds;
- `mss-boot/virtual` and the Admin center's virtual-model interface;
- active documentation that claims the runtime features remain available.

The repository-level `mss module generate`, application blueprint generator, deterministic templates, upgrade
manifests, and Agent skills remain. They run during development, produce reviewable code, and are not an Admin
runtime product capability.

### Preserve data while removing discoverability

A forward migration removes exact retired menu roots, their stored descendants, retired API metadata, and the
matching typed Casbin policy tuples. It does not infer arbitrary business API paths from broad prefixes.

Existing `mss_boot_models`, `mss_boot_fields`, and user-created virtual data tables are left inert. Automatically
dropping those tables would destroy data without knowing whether an operator has exported or migrated it. Fresh
installations no longer create or seed runtime model metadata. Operators may remove retained tables in a separate,
explicit data-retirement procedure after backup and verification.

If a downstream application attached an unrelated direct child to `/develop`, the migration reparents that child
instead of deleting it. A relative child route is rewritten to its former effective URL under `/develop`; matching
Casbin rules are copied to the rewritten path without removing the old rule or duplicating an existing target rule.
For database-managed languages, the migration copies the complete ancestor-aware locale namespace only when the
destination key is absent. For example, a root child maps `menu.develop.<child>` to `menu.<child>`, while a nested
child maps `menu.<ancestor>.develop.<child>` to `menu.<ancestor>.<child>`. Source definitions and definitions that
cannot be attributed to a surviving menu are retained because the legacy data has no reliable provenance marker.

### Sanitize the historical built-in OAuth credential

An earlier built-in application profile contained a GitHub OAuth client credential and repository-capable scopes.
Editing that historical migration only protects fresh installations: upgraded databases and reachable Git history
remain separate exposure paths. A new forward migration therefore identifies only the exact historical built-in
credential tuple by SHA-256 fingerprint, clears its client ID and secret, and replaces its scopes with `read:user`
and `user:email`. It does not contain the historical plaintext and does not overwrite a profile whose credential was
already rotated or customized.

Database cleanup cannot revoke a provider credential or erase reachable Git history. Provider-side rotation or
revocation and a secret-scanner/platform audit remain release gates even after the forward migration succeeds.

### Schedule monitoring on the built-in system-job lane

Reuse `mss-boot/core/server/task` for two distinct scheduling lanes. The task server itself remains the framework
`Runnable`; monitor collection is a `cron.Job`, not a separate lifecycle component.

System jobs are registered with `task.WithSystemSchedule` in immutable, process-local storage. The always-on
`monitor-sampler` and `session-cleanup` jobs load when the service starts, even when `task.enable` is false.
User-managed jobs use the optional persistent Task storage; their internal reconciliation schedule is added only
when `task.enable` is true. A persistent task cannot shadow a reserved system-job key, and Task CRUD cannot update
or remove a system job.

System-job registration and the `task.enable` capability are evaluated before `task.Server.Start` loads its
startup schedule snapshot. Configuration hot reload does not rebuild that snapshot or change the user-schedule
capability; changing either requires a process restart. User Task CRUD may still reconcile user schedules after
startup, but it cannot mutate the separate system-job store.

Register both `monitor-sampler` and `session-cleanup` on the built-in lane. Neither job creates rows in
`mss_boot_tasks` or `mss_boot_task_runs`/`TaskRun`; the absence of those rows is therefore expected, not a scheduler
failure. The task server owns cron startup, cancellation, and shutdown for both lanes.

The monitor system job will:

- sample CPU and memory every five seconds;
- retain 120 chronological points in an in-memory ring buffer, approximately ten minutes;
- calculate CPU utilization from adjacent CPU time counters rather than a blocking request-time interval;
- bound each collection attempt with a timeout;
- preserve the last good snapshot after a transient failure and mark it stale;
- stop scheduling through the task server when the server context is cancelled.

The authorized monitor API copies the immutable latest snapshot and requested recent history. Existing response
fields remain compatible; collection time, interval, stale state, instance identity, and history are additive.
Samples are instance-local and intentionally disappear on restart. Persistent or cross-replica trends remain the
responsibility of external observability.

### Make charts theme-aware

The welcome and monitor views consume server timestamps instead of fabricating browser-local points. A shared chart
component derives canvas, axes, grid, line, area, and tooltip colors from the active Ant Design tokens and switches
the G2 base theme with the application theme. The views distinguish initial loading, empty history, stale data, and
request failure, and pause polling in hidden tabs.

## Consequences

### Positive

- The Admin product has one clear extension path: checked-in, reviewable, deterministic modules.
- Runtime routes can no longer create schemas or push generated repositories.
- Opening multiple dashboards no longer multiplies blocking host measurements.
- A newly opened page immediately receives recent points collected before the page existed.
- CPU and memory trends remain readable in light and dark themes.
- Disabling user-managed tasks cannot silently disable monitoring or session cleanup.

### Breaking

- Downstream imports of `mss-boot/virtual` stop compiling.
- Calls to retired Admin APIs and direct navigation to retired pages return not found.
- Existing automation that relied on the browser code generator must migrate to `mss module generate` or another
  reviewed development-time workflow.

### Operational

- Each Admin replica exposes its own history and instance identity.
- A restart resets the in-memory window.
- `task.enable` controls persistent user tasks only; built-in system jobs always accompany the service.
- `monitor-sampler` necessarily runs once per replica. `session-cleanup` is idempotent but also runs once per replica;
  large clusters should add an external lease if simultaneous cleanup creates material database pressure.
- Existing default/func user tasks remain process-local and do not guarantee cluster-wide single execution; use the
  Kubernetes provider or an external coordinator when that guarantee is required.
- Operators must use service logs and monitor timestamps for built-in-job health because built-ins intentionally have
  no Task/TaskRun rows or user-facing CRUD records.
- Retained legacy tables consume their existing storage until an operator explicitly removes them.
- The credential-sanitizing migration removes the known value from matching built-in database profiles, but an
  operator must still revoke or rotate it at the provider because it remains reachable in historical commits.

## Rollout and rollback

Before rollout, back up the database and inventory any downstream `mss-boot/virtual` imports or retired endpoint
calls. Apply the forward migration, restart all Admin instances, verify that the retired menu tree is absent, and
wait for at least three sample intervals before accepting the trend charts. Also verify that session cleanup and
monitoring still run with user tasks disabled and that no built-in job appears in Task/TaskRun tables. Before
production promotion, rotate or revoke the historical OAuth credential at the provider and confirm through the
repository secret scanner and provider audit that the old value can no longer authenticate.

Rollback requires deploying the preceding application version and restoring menu and policy metadata from the
backup if the runtime tools must be temporarily re-enabled. Retained metadata and virtual data tables make data
export possible during that rollback. Monitoring samples need no database rollback.
