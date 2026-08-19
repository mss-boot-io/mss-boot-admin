# Component-scoped CI pipelines

- Status: Accepted
- Date: 2026-08-04
- Amended: 2026-08-19
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
| Admin backend | `.github/workflows/ci.yml` | Admin tests and compilation run in parallel; every pull request emits the branch-protection-required `admin-ci` aggregate check. |
| Framework | `.github/workflows/mss-boot-ci.yml` | Independent `GOWORK=off` tests, race tests, vet, and module-tidiness checks. |
| Frontend | `.github/workflows/frontend-v6-ci.yml` | Biome, TypeScript, Vitest, production build, delivery smoke, and browser evidence validate the sole V6 application. |
| Documentation | `.github/workflows/docs.yml` | Dumi build on documentation changes and Cloudflare deployment only after a successful `main` build. |
| Container image | `.github/workflows/container.yml` | Pull requests build the vendored image without pushing; `main`, tags, and manual dispatch publish with the GitHub Actions BuildKit cache. |

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
- Feature branches do not run duplicate `push` workflows. Pull requests validate feature work; `push` remains for `main` and release tags.
- Superseded pull-request runs are cancelled through workflow concurrency groups.
- Redis-backed Go jobs use a native `redis:7-alpine` service instead of building a third-party Docker action.
- Agent infrastructure CI no longer watches or rebuilds documentation content. Shared Agent and release contracts continue to use their own root paths.
- Inactive workflows under `mss-boot/.github/workflows/` are removed because GitHub only loads workflows from the repository-root `.github/workflows/` directory. Root workflows now own framework CI, CodeQL, Scorecard, and nested-module release behavior.
- The legacy Swagger deployment is path-scoped and version-pins the generator.
- The repository does not commit `vendor/`. The container workflow must therefore run `go work vendor` before every Docker build because the Dockerfile copies the generated workspace vendor directory and compiles with vendoring enabled.
- Container changes are verified on pull requests with `push: false`; registry authentication and package writes occur only outside pull requests.

## Compatibility

The workflow file `.github/workflows/ci.yml` keeps the workflow name `CI` and exposes a final job named `admin-ci`. The `main` branch rule requires that exact context, so the workflow is intentionally unfiltered for pull requests. Push and tag events remain path-scoped where no pull-request merge gate depends on them.

The frontend workflow similarly keeps `Frontend CI / build` as an aggregate over quality and compilation. New framework and container checks can be made required after one successful pull-request run establishes their exact check names.

## Expected effect

- Root admin tests and compilation run concurrently.
- Framework test, race, and static/module checks run concurrently.
- Frontend and docs jobs no longer wait behind Go validation.
- Documentation-only and frontend-only pull requests still create the required `admin-ci`, `govulncheck`, and CodeQL contexts, but unrelated heavy jobs are skipped through successful sentinels.
- Admin-only changes run Admin validation and the Admin Go vulnerability scan without starting Framework or frontend suites.
- Framework-only changes run Framework validation, its Go vulnerability scan, and the Admin compatibility contract without starting full Admin or frontend suites.
- Frontend-only changes run frontend validation and JavaScript/TypeScript CodeQL without Go test or vulnerability work.
- Docs-only changes run the Docs build and dependency audit without backend, Framework, frontend, Go vulnerability, or CodeQL analysis work.
- Main-branch image publishing no longer extends the required `admin-ci` duration.
- Dockerfile, Go dependency, and container-workflow changes fail in the pull request if workspace vendoring or the image context is invalid, instead of failing for the first time after merge.

## Validation

A workflow change is complete when a pull request demonstrates:

- `admin-ci` is created and succeeds for every pull request;
- `mss-boot CI / mss-boot` succeeds;
- `Frontend CI / build` succeeds when frontend paths change;
- `Docs / build` succeeds when docs paths change;
- pure `admin/`, `mss-boot/`, `web/`, and `docs/` fixtures select their single expected scope while mixed and root fixtures select `shared`;
- `Container Image / build` succeeds for image-affecting changes without pushing an image;
- Agent, security, PR guard, and docs-drift workflows remain green;
- no workflow is triggered twice for the same feature-branch commit solely because both `push` and `pull_request` matched.
