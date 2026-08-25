# mss-boot framework agent instructions

## Scope

This file applies to `mss-boot/` and inherits the repository-wide contract from the root `AGENTS.md`.

`mss-boot` is the reusable, domain-neutral Go framework module embedded in the monorepo. It must remain usable independently from the admin reference application.

## Architecture boundary

Allowed framework responsibilities include:

- server lifecycle and listeners;
- configuration sources and adapters;
- logging, tracing, cache, queue, lock, storage, and transport primitives;
- generic response/controller helpers;
- migration and version primitives;
- reusable operation, condition, idempotency, retry, and reconciliation helpers.

Do not add the following to this module:

- admin business entities or menus;
- generated business modules;
- React or product-specific concepts;
- imports from the root `mss-boot-admin` application;
- assumptions about one application's database tables or roles.

The dependency direction is always:

```text
mss-boot-admin → mss-boot
```

Never reverse it.

## Compatibility

- Treat exported Go APIs, configuration keys, persistence behavior, and interfaces as public compatibility surfaces.
- For a breaking change, provide migration guidance, tests, release impact, and a rollback path.
- Prefer additive options over signature changes.
- Keep optional integrations degradable; an optional provider failure should not terminate unrelated application capabilities.

## Canonical validation

From the repository root:

```shell
make test-framework
go run ./cmd/mss verify --changed
```

From `mss-boot/`, verify independent module behavior:

```shell
GOWORK=off go mod download
GOWORK=off go test ./...
GOWORK=off go vet ./...
```

Run `govulncheck` when dependency, transport, authentication, storage, configuration, or security behavior changes.

## Agent infrastructure relationship

The project contract, module generator, Skills, MCP adapter, and evaluations live outside `mss-boot/`. Framework primitives may support those tools, but tool-specific orchestration belongs under `internal/mss/`, `cmd/mss/`, `.agents/`, or `.mss/`.

Archived conversational prompts are intentionally absent from the Framework tree. Use compiling code, `.mss/` contracts, tests, ADRs, and current documentation as the durable engineering context.
