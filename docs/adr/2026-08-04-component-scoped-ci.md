# Component-scoped CI pipelines

- Status: Accepted
- Date: 2026-08-04
- Scope: `mss-boot-admin` monorepo validation and publishing

## Context

The monorepo contains four independently testable products:

1. the reusable `mss-boot/` Go framework module;
2. the root Go admin application;
3. the `web/antd-v6/` frontend;
4. the `docs/` Dumi site.

The previous root `CI` workflow executed root tests, framework tests, framework race tests, framework vet, module tidiness, backend compilation, workspace vendoring, and image publishing in one serial job. Agent infrastructure validation also installed Node and rebuilt the documentation site even though `docs.yml` already owned documentation validation and deployment. Feature branches additionally ran both `push` and `pull_request` copies of the same workflows.

This made component ownership unclear and made the wall-clock duration equal to the sum of unrelated checks.

## Decision

Validation is split into component-owned workflows:

| Component | Workflow | Required behavior |
| --- | --- | --- |
| Admin backend | `.github/workflows/ci.yml` | Root tests and root build run in parallel; the historical `CI / build` aggregate check remains available for branch protection. |
| Framework | `.github/workflows/mss-boot-ci.yml` | Independent `GOWORK=off` tests, race tests, vet, and module-tidiness checks. |
| Frontend | `.github/workflows/frontend-v6-ci.yml` | Biome, TypeScript, Vitest, production build, delivery smoke, and browser evidence validate the sole V6 application. |
| Documentation | `.github/workflows/docs.yml` | Dumi build on documentation changes and Cloudflare deployment only after a successful `main` build. |
| Container image | `.github/workflows/container.yml` | Pull requests build the vendored image without pushing; `main`, tags, and manual dispatch publish with the GitHub Actions BuildKit cache. |

Additional rules:

- Feature branches do not run duplicate `push` workflows. Pull requests validate feature work; `push` remains for `main` and release tags.
- Superseded pull-request runs are cancelled through workflow concurrency groups.
- Redis-backed Go jobs use a native `redis:7-alpine` service instead of building a third-party Docker action.
- Agent infrastructure CI no longer installs Node or builds docs. It still validates Agent contracts that happen to live under `docs/`.
- Inactive workflows under `mss-boot/.github/workflows/` are removed because GitHub only loads workflows from the repository-root `.github/workflows/` directory. Root workflows now own framework CI, CodeQL, Scorecard, and nested-module release behavior.
- The legacy Swagger deployment is path-scoped and version-pins the generator.
- The repository does not commit `vendor/`. The container workflow must therefore run `go work vendor` before every Docker build because the Dockerfile copies the generated workspace vendor directory and compiles with vendoring enabled.
- Container changes are verified on pull requests with `push: false`; registry authentication and package writes occur only outside pull requests.

## Compatibility

The workflow file `.github/workflows/ci.yml` keeps the workflow name `CI` and exposes a final job named `build`. Existing branch rules that require `CI / build` therefore continue to represent both root tests and root compilation.

The frontend workflow similarly keeps `Frontend CI / build` as an aggregate over quality and compilation. New framework and container checks can be made required after one successful pull-request run establishes their exact check names.

## Expected effect

- Root admin tests and compilation run concurrently.
- Framework test, race, and static/module checks run concurrently.
- Frontend and docs jobs no longer wait behind Go validation.
- A documentation-only pull request does not start the root or framework workflow.
- A frontend-only pull request does not start the root, framework, or docs workflow.
- A framework change still triggers root admin compatibility validation, but the two workflows execute independently and concurrently.
- Main-branch image publishing no longer extends the required `CI / build` duration.
- Dockerfile, Go dependency, and container-workflow changes fail in the pull request if workspace vendoring or the image context is invalid, instead of failing for the first time after merge.

## Validation

A workflow change is complete when a pull request demonstrates:

- `CI / build` succeeds;
- `mss-boot CI / mss-boot` succeeds;
- `Frontend CI / build` succeeds when frontend paths change;
- `Docs / build` succeeds when docs paths change;
- `Container Image / build` succeeds for image-affecting changes without pushing an image;
- Agent, security, PR guard, and docs-drift workflows remain green;
- no workflow is triggered twice for the same feature-branch commit solely because both `push` and `pull_request` matched.
