# __MSS_APP_DISPLAY_NAME__

This repository is a Thin Host for `__MSS_DISTRIBUTION_NAME__`
`__MSS_DISTRIBUTION_VERSION__`. It imports the complete Admin backend and
frontend packages and owns only business modules, configuration, tests, and
composition glue.

## Start development

Install the matching `mss` tool from the versioned release bundle, then run:

```shell
mss doctor --strict
mss setup
mss dev --detach
```

`mss setup` downloads the exact locked packages and idempotently initializes
the local SQLite database. On an interactive terminal, the first migration
prompts for the initial administrator password with hidden input. In
non-interactive automation, inject `MSS_ADMIN_INITIAL_PASSWORD` from the CI
secret store for that setup process only. Setup explicitly removes the value
from dependency-install subprocesses and injects it only into the migration
command; it is never placed in command arguments, reports, or generated files.
Use 8-128 characters with at least one letter and one number.
After the first migration succeeds, repeated setup runs do not require it.

The backend listens on `http://127.0.0.1:8080` and Admin Web on
`http://127.0.0.1:8001`. Inspect detached services with `mss dev status`, read
logs with `mss dev logs <service>`, and stop them with `mss dev stop`.
Sign in to Admin Web as `admin` with the password supplied during the first
setup. There is no default password.

Before every pull request, run:

```shell
mss verify --all
```

## Add business capabilities

Write an `AdminModule` specification under `.mss/modules/`, review the dry-run,
then apply the deterministic generator:

```shell
mss module generate .mss/modules/example.yaml
mss module generate .mss/modules/example.yaml --write
mss verify --module example
```

Custom backend files belong under `internal/modules/<name>/`. Register a
handwritten module explicitly in `internal/modules/custom/modules.go`; the
managed `internal/modules/registry.go` facade composes generated modules first.
Custom frontend files belong under `web/src/business/`. Add handwritten Umi
routes to `web/src/business/routes.config.ts` and the matching frontend-only
server-path projections to `web/src/business/route-registrations.ts`. These
registrations drive menu visibility; they do not write Admin Menu or Casbin rows
and never authorize a backend request. The managed facades compose both
registries, and the final Admin Web registry rejects duplicates across core,
generated, and handwritten UI or server paths.

Keep handwritten user-facing messages synchronized in
`web/src/business/locales/zh-CN.ts` and
`web/src/business/locales/en-US.ts`. Managed locale facades merge Admin core,
generated module, and handwritten messages in that order, so business pages can
add both languages without editing generated locale registries.

The protected backend group authenticates sessions but does not infer business
permissions. Every handwritten module must add a forward migration that creates
or validates permission metadata and default role policies, verify those records
in readiness, and enforce the exact permission in each handler through the
injected principal and request database. Cover both allowed and denied callers.
Do not copy Foundation Admin, Framework, or Admin Web source into this repository.

## Upgrade the Admin Distribution

Back up the repository and database, then install the `mss` tool whose version
matches the requested Distribution. Confirm both binaries report that version
and that this generated repository still contains
`.mss/blueprint-manifest.json` before planning:

```shell
mss --version
mss-mcp --version
mss upgrade status --format json
mss upgrade admin __MSS_DISTRIBUTION_VERSION__ --format json
mss upgrade admin __MSS_DISTRIBUTION_VERSION__ --apply --yes --format json
mss upgrade status
mss doctor --strict
mss verify --all
mss upgrade admin __MSS_DISTRIBUTION_VERSION__ --format json
```

The first command is read-only. Review every managed change and conflict before
the confirmed apply; the final plan must contain no create, update, delete, or
conflict operations. It may continue to report a customized default registry as
`preserve`, which is read-only and must leave its bytes unchanged. Unknown
business-owned files remain outside the managed Blueprint baseline. The three
default handwritten registry files and two bilingual locale catalogs start in
the baseline so new hosts compile; their explicit edits are preserved while the
corresponding Blueprint defaults remain unchanged. A hand-assembled repository
or one missing its manifest cannot
use three-way upgrade: generate a clean target baseline in a new directory and
migrate business-owned specifications and files instead of fabricating a
manifest.

## Configuration and security

Development uses the Distribution's embedded, redacted local defaults. Keep
production credentials and environment-specific overlays outside source control. Backend
authorization is authoritative; frontend permission checks are an experience
layer only.

For deployment, run the image's idempotent `migrate` command as an init job
before starting `server`. A fresh database receives
`MSS_ADMIN_INITIAL_PASSWORD` only from the deployment secret store for that
job; do not keep it in the long-running server environment. A failed migration
must block rollout.

Generated by `mss` __MSS_GENERATOR_VERSION__ from the management-system Thin
Host Blueprint.
