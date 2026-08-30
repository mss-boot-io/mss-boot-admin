# Component-scoped CI pipelines

- Status: Accepted
- Date: 2026-08-04
- Amended: 2026-08-30
- Scope: `mss-boot-admin` monorepo validation and publishing

## Context

The monorepo contains four independently testable products:

1. the reusable `mss-boot/` Go framework module;
2. the `admin/` Go application;
3. the `web/antd-v6/` frontend;
4. the `docs/` Dumi site.

The previous root `CI` workflow executed root tests, framework tests, framework race tests, framework vet, module tidiness, backend compilation, workspace vendoring, and image publishing in one serial job. Agent infrastructure validation also installed Node and rebuilt the documentation site even though `docs.yml` already owned documentation validation and deployment. Feature branches additionally ran both `push` and `pull_request` copies of the same workflows.

This made component ownership unclear and made the wall-clock duration equal to the sum of unrelated checks.

## Decision

Validation is split into component-owned workflows:

| Component | Workflow | Required behavior |
| --- | --- | --- |
| Admin backend | `.github/workflows/ci.yml` | Every pull request emits the required `admin-ci` aggregate; Admin/shared PRs run one ordinary test, while merged `main` or manual runs retain coverage, race, vet/tidy, compatibility, and binary audit. |
| Framework | `.github/workflows/mss-boot-ci.yml` | Framework PRs run one ordinary `GOWORK=off` test; merged `main` or manual runs retain coverage, race, vet, and module-tidiness audit. |
| Frontend | `.github/workflows/frontend-v6-ci.yml` | Biome, TypeScript, Vitest, production build, delivery smoke, and browser evidence run after merge or by explicit dispatch; the same suite is authoritative locally through `mss verify --all`. |
| Documentation | `.github/workflows/docs.yml` | Dumi builds run after merge, by explicit dispatch, or for a Docs tag; PR qualification is local. |
| Container image | `.github/workflows/container.yml` | `main`, explicit preview/manual calls, and release tags build images; pull requests do not repeat the local container and delivery checks. |

The reusable `.github/workflows/component-scope.yml` classifier compares exact
base and head commits and emits one of `admin`, `framework`, `web`, `docs`, or
`shared`. A change receives a component scope only when every changed path is
under that component's owned top-level directory. Empty comparisons, root
files, shared tooling, workflow files, and mixed-component changes use
`shared`, which is the fail-safe full-validation route.

Additional rules:

- A workflow that owns a required branch-protection check must start for every pull request. Component filtering may happen inside its jobs or on non-PR events, but a required check must never disappear because of trigger-level path filters.
- Required Admin, vulnerability, and CodeQL checks keep their stable context names. Outside their owned scope they run a small successful sentinel instead of installing unrelated toolchains or executing unrelated scans.
- Go vulnerability scans select only the Admin or Framework module for pure changes, no Go module for pure frontend or Docs changes, and every Go module for shared changes.
- CodeQL runs Go analysis for Admin and Framework, JavaScript/TypeScript analysis for frontend, and every language for shared changes. Matrix entries outside the selected scope become lightweight sentinels so required contexts remain present.
- Feature branches do not run duplicate `push` workflows. Pull requests retain only fast required contexts and ordinary Go smoke tests; complete qualification is local, while `push` remains a post-merge audit for `main` and release tags.
- Superseded pull-request runs are cancelled through workflow concurrency groups.
- Redis-backed Go jobs use a native `redis:7-alpine` service instead of building a third-party Docker action.
- Agent infrastructure CI no longer watches or rebuilds documentation content. Shared Agent and release contracts continue to use their own root paths.
- Inactive workflows under `mss-boot/.github/workflows/` are removed because GitHub only loads workflows from the repository-root `.github/workflows/` directory. Root workflows now own framework CI, CodeQL, Scorecard, and nested-module release behavior.
- The legacy Swagger deployment is path-scoped to merged `main`, version-pins the generator, and does not block pull requests.
- The repository does not commit `vendor/`. The container workflow must therefore run `go work vendor` before every Docker build because the Dockerfile copies the generated workspace vendor directory and compiles with vendoring enabled.
- Container changes are verified locally before merge and audited on merged `main`; registry authentication and package writes occur only outside pull requests.

## Compatibility

The workflow file `.github/workflows/ci.yml` keeps the workflow name `CI` and exposes a final job named `admin-ci`. The `main` branch rule requires that exact context, so the workflow is intentionally unfiltered for pull requests. A pull request runs only the ordinary Admin test behind that aggregate; race, coverage, vet/tidy, workspace compatibility, and standalone binary work move to the complete local verifier. The Agent, frontend, real Thin Host, container, and multi-database workflows run on merged `main` or explicit dispatch instead of blocking every pull request. Main-branch pushes remain path-scoped for post-merge audit. Release tags are owned only by their publication workflows; the self-bound local `mss verify --all --release-evidence --expect-commit <sha>` report owns broad release quality, while the successful candidate preview builds and verifies only the exact publication artifacts reused by the tag workflows.

The frontend workflow similarly keeps `Frontend CI / build` as an aggregate over quality and compilation. New framework and container checks can be made required after one successful pull-request run establishes their exact check names.

## Expected effect

- Every pull request keeps stable guard, vulnerability, CodeQL, and `admin-ci` contexts without starting unrelated broad matrices.
- Admin/shared and Framework pull requests retain one ordinary Go test as a fast server-side smoke check.
- Agent, frontend, Docs, Swagger, external Thin Host, container, credential migration, theme database, and v0.7 upgrade matrices no longer delay merge.
- `mss verify --all` is the complete pre-merge quality suite; the release-evidence form binds the same suite to the frozen merged-main commit.
- Merged-main and manual workflows remain available as asynchronous audit and diagnostics, but their completion is not a prerequisite for starting artifact staging.
- Candidate preview and tag workflows retain only source identity, immutable artifact, public dependency, OIDC/provenance, and reconciliation boundaries that cannot be proven solely by local tests.

## Validation

A workflow change is complete when:

- `admin-ci` is created and succeeds for every pull request;
- guard, govulncheck, and both CodeQL contexts keep their required stable names;
- Admin/shared PRs run one ordinary Admin test, and Framework PRs run one ordinary independent Framework test;
- pure `admin/`, `mss-boot/`, `web/`, and `docs/` fixtures select their single expected scope while mixed and root fixtures select `shared`;
- every broad component workflow has no `pull_request` trigger and remains reachable from merged `main` plus explicit dispatch;
- `mss verify --all` covers the broad quality matrix locally, including the real repository-external Thin Host;
- the first irreversible Framework publication requires one exact artifact preview, while Admin/npm do not repeat the lookup;
- workflow-governance tests, YAML/action validation, and changed-impact verification pass.
