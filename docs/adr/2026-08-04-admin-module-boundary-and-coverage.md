# Admin module boundary and coverage contracts

- Status: Accepted
- Date: 2026-08-04
- Scope: repository Go modules, compatibility verification, and coverage policy

## Context

The repository root previously served two unrelated purposes:

1. the deployable Admin application and its domain packages;
2. the Agent/Foundation command-line and project automation packages.

That arrangement made the root coverage result misleading. A single 27.3% value mixed the relatively well-tested Agent packages with the much less tested Admin runtime. A package-level audit established the actual pre-split baselines:

| Component | Pre-split statement coverage |
| --- | ---: |
| Admin application | 16.2% |
| Agent/Foundation tooling | 45.2% |
| mss-boot framework | 26.5% |

The root module also made it difficult to verify whether the deployable Admin application remained compatible with an upgraded `mss-boot`, because root tests implicitly resolved both codebases through one workspace.

## Decision

The repository contains three explicit Go modules:

```text
/
├── go.mod       # Agent/Foundation tooling
├── admin/go.mod # deployable Admin application
└── mss-boot/go.mod
```

`go.work` composes the three modules for repository development. The Admin module keeps an explicit sibling replacement for `mss-boot` until a stable nested framework tag is published and consumed independently.

Dependency direction is:

```text
Agent/Foundation ── machine contracts only ──> repository layout
Admin application ───────────────────────────> mss-boot
mss-boot ────────────────────────────────────> no Admin packages
```

The root Agent module and `mss-boot` must not import `admin/...`. Admin business packages must not move into the framework to avoid the module boundary.

## Admin compatibility contract

Every Admin or framework pull request validates all of the following:

- Admin tests with `GOWORK=off` and the committed module metadata;
- Admin tests in the repository workspace against the current local framework;
- Admin race tests and `go vet`;
- `go mod tidy` with no Admin module drift;
- Admin binary build and `--help` startup smoke test;
- framework database ownership and lifecycle contracts exercised from an Admin compatibility package;
- generated Blueprint workspace compilation for root, Admin, and framework modules;
- Docker build from a freshly generated workspace vendor directory.

This is a source-compatibility contract for the currently developed framework. Public HTTP behavior and database schemas are unchanged by the module move.

## Coverage policy

Coverage is measured independently for Agent, Admin, and framework modules. Admin and framework checks use atomic coverprofiles and a repository-owned policy file. The policy can express:

- a component-wide statement floor;
- exact critical-package floors;
- subtree floors through `/...` patterns.

A high-coverage utility package cannot compensate for a regression in a named security, lifecycle, configuration, or request package.

The first enforced floors are set only after targeted tests raise the audited baselines. Floors may increase in normal pull requests but may not be reduced without an explicit architecture decision explaining the removed or untestable behavior.

## Repository paths

- Deployable backend: `admin/`
- Reusable framework: `mss-boot/`
- Agent/Foundation tooling: root `cmd/mss*` and `internal/mss/`
- Generated business modules: `admin/modules/<name>/`
- Business specifications: `.mss/modules/` and `.mss/features/`

Specifications remain repository-level contracts; generated Go implementations belong to the Admin module.

## Build and release

Admin commands execute from `admin/` so historical relative runtime paths such as `config/` remain stable. Build outputs use a path such as `bin/mss-boot-admin`; the name `admin` is no longer usable as a root binary output because it is now a directory.

Container, release, Swagger, development, and downstream Blueprint workflows must all build or execute the Admin module explicitly.

## Migration

Internal Go imports move from:

```text
github.com/mss-boot-io/mss-boot-admin/<admin-package>
```

to:

```text
github.com/mss-boot-io/mss-boot-admin/admin/<admin-package>
```

Agent and framework import paths remain unchanged. Git history is preserved through file moves rather than copied source.

## Rollback

Rollback must revert the complete module-boundary change set, including:

- filesystem moves;
- `go.mod`, `go.sum`, and `go.work` files;
- imports and generated output paths;
- CI, Docker, release, Swagger, and development commands;
- Blueprint manifests and compatibility tests;
- coverage policy and reports.

Partially restoring root Admin paths would leave workspace resolution and downstream generation inconsistent.
