# mss-boot-admin Agent Contract

## Mission

This repository is the source of truth for an agent-native management-system foundation. It contains the Go backend, the reusable `mss-boot` framework module, the Ant Design frontend, documentation, project specifications, deterministic generators, validation tooling, and agent workflows.

A successful change must be understandable by humans, executable by coding agents, reproducible in CI, and upgradeable after downstream applications adopt it.

## Instruction scope

- This file applies to the entire repository.
- A closer `AGENTS.md` may add directory-specific rules; it must not weaken security, compatibility, or validation requirements from this file.
- Do not depend on personal absolute paths, workstation-specific tools, or hidden conversation context.
- Treat `.mss/` as the machine-readable project contract and this file as the human-readable top-level contract.

## Repository map

| Path | Responsibility |
| --- | --- |
| `/` | Go admin backend and current reference application |
| `mss-boot/` | Reusable Go framework module; keep domain-neutral |
| `web/antd/` | React + Ant Design frontend |
| `docs/` | Product, architecture, operations, and contributor documentation |
| `.mss/` | Machine-readable project, capability, command, schema, module, eval, and lock contracts |
| `.agents/skills/` | Reusable agent workflows; skills call the `mss` CLI instead of duplicating implementation logic |
| `cmd/mss/` | Agent-facing deterministic CLI entrypoint |
| `internal/mss/` | CLI implementation packages: project, doctor, generator, inspector, verifier, upgrader, eval |
| `modules/` | New vertical business modules and generated module registry |
| `templates/` | Deterministic application and module templates |
| `tools/` | Codemods and contract tooling |
| `compose/` | Local integration dependencies |

## Source-of-truth order

When information conflicts, use this order:

1. Compiling code and database migrations.
2. `.mss/` machine-readable contracts.
3. Tests and generated validation reports.
4. Current architecture decisions and user documentation.
5. This file and directory-specific `AGENTS.md` files.
6. Historical prompts or archived handoff notes.

Historical prompt files are evidence, not active requirements, unless a current specification explicitly references them.

## Standard workflow

1. Read the nearest applicable `AGENTS.md` files.
2. Read `.mss/project.yaml`, `.mss/capabilities.yaml`, and `.mss/commands.yaml` when present.
3. Inspect existing capabilities before creating a parallel implementation.
4. For medium or large changes, create or update a structured spec before implementation.
5. Prefer deterministic CLI or generator operations for repetitive code.
6. Implement the smallest coherent change.
7. Commit completed implementation or specifications before broad testing when repository state might be lost.
8. Run the smallest relevant validation first, then broader checks based on change impact.
9. Report commands, results, skipped checks, migrations, security impact, compatibility impact, and remaining risk.

Do not rewrite pushed history to hide intermediate fixes. Follow-up repair commits are preferred over force-push, rebase, or destructive reset.

## Canonical commands

Use the repository wrappers instead of inventing workstation-specific command sequences:

```shell
# Agent and environment context
go run ./cmd/mss context
go run ./cmd/mss doctor

# Setup and validation
go run ./cmd/mss setup
go run ./cmd/mss verify --changed
go run ./cmd/mss verify --all

# Existing direct targets remain valid during migration
make deps-all
make test-all
make web-install web-lint web-test web-build
make docs-install docs-build
```

If `cmd/mss` is not yet available on an older branch, fall back to the Make targets documented in `.mss/commands.yaml`.

## Architecture rules

### Go framework boundary

- `mss-boot/` contains reusable, domain-neutral infrastructure.
- Do not add admin-specific entities, menus, pages, or business workflows to `mss-boot/`.
- Framework changes require independent tests with `GOWORK=off` where relevant.
- The root application may depend on `mss-boot`; the framework must not depend on the root application.

### New business modules

- New business capabilities should use vertical modules under `modules/<name>/` once the module infrastructure is available.
- Existing horizontal directories such as `apis/`, `dto/`, `models/`, and `service/` remain supported for compatibility.
- Do not perform a broad mechanical migration of legacy modules unless a dedicated migration spec exists.
- A complete module change includes backend behavior, migration, permission, menu/route, frontend, tests, and documentation as applicable.

### Dynamic model legacy boundary

The existing runtime dynamic-model or virtual-model capability is legacy. Do not use it as the default implementation for new production business modules. Development-time deterministic generation is the preferred path.

### Contracts before generated code

- Structured specifications are edited by humans or agents.
- Generated files must identify their source specification and generator version.
- Do not hand-edit generated regions when the change can be expressed in the source spec or template.
- Generators must support dry-run, idempotency, path confinement, and stable output ordering.

## Backend rules

- Stack: Go, Gin, GORM, Casbin, Cobra, Swagger/OpenAPI.
- Reuse existing authentication, authorization, response, configuration, cache, queue, locking, and storage abstractions.
- API authorization must be enforced on the backend; hiding a frontend control is not authorization.
- State-changing operations must not use GET.
- Use parameterized database operations and explicit transactions where multiple writes form one logical operation.
- Add forward-compatible migrations for persistent model changes.
- Avoid external side effects in GORM hooks when an explicit service or reconciliation boundary is possible.
- Do not terminate the whole process for an optional external integration failure; return or surface a diagnosable state.

## Frontend rules

- Stack: React, TypeScript, Ant Design, Umi, pnpm.
- Use repository-relative paths only.
- Keep API client types aligned with the generated OpenAPI contract.
- Permission checks in the UI improve experience but never replace backend authorization.
- New pages must include loading, empty, error, and permission-denied states where applicable.
- Keep Chinese and English locale keys synchronized for user-facing additions.
- Run focused tests and TypeScript checks for changed modules.

## Documentation rules

- Long-lived architecture and product guidance belongs under `docs/docs/`.
- Architecture decisions belong under `docs/adr/` when introduced.
- Machine-executable facts belong under `.mss/`, not only in prose.
- Update examples, commands, repository names, and paths when structure changes.
- Do not publish production credentials, private endpoints, personal paths, or unredacted sensitive logs.

## Security rules

- Never commit passwords, tokens, private keys, production DSNs, kubeconfigs, or cloud credentials.
- Setup, tests, and evals must run without production access.
- Agent tools must not write outside the repository root.
- Shell execution must use validated arguments; do not concatenate untrusted text into a command line.
- MCP and generator write operations default to dry-run and return a changed-file list.
- Audit output must redact secrets and sensitive request bodies.
- API keys are displayed once and stored as hashes when application features introduce them.

## Validation expectations

Choose checks from the change impact; do not claim checks that were not run.

| Change | Minimum validation |
| --- | --- |
| Go implementation | focused `go test`, then affected module tests |
| `mss-boot/` | `cd mss-boot && GOWORK=off go test ./...` |
| root backend or shared contract | `make test-all` and `make build` |
| frontend | `pnpm lint:js`, `pnpm tsc`, focused Jest, relevant build |
| docs | `pnpm --dir docs build` |
| migration | fresh database migration and upgrade-path test |
| permission | positive and negative authorization tests |
| generator | schema tests, golden tests, path-confinement tests, two-run idempotency test |
| generated contract | drift check after regeneration |
| workflow | syntax validation plus a real GitHub Actions run when possible |

Use `mss verify --changed` once available. Full verification is required before release but not before every checkpoint commit.

## Delivery summary

A final handoff for a non-trivial change must state:

- Goal and implemented scope.
- Important files and architectural decisions.
- Commits created.
- Commands actually run and their results.
- Migrations and compatibility impact.
- Security impact.
- Known limitations or skipped checks with concrete reasons.
- The next executable step, not a vague recommendation.
