# Component-scoped CI pipelines

- Status: Accepted
- Date: 2026-08-04
- Scope: `mss-boot-admin` monorepo validation and publishing

## Context

The monorepo contains four independently testable products:

1. the reusable `mss-boot/` Go framework module;
2. the root Go admin application;
3. the `web/antd/` frontend;
4. the `docs/` Dumi site.

The previous root `CI` workflow executed root tests, framework tests, framework race tests, framework vet, module tidiness, backend compilation, workspace vendoring, and image publishing in one serial job. Agent infrastructure validation also installed Node and rebuilt the documentation site even though `docs.yml` already owned documentation validation and deployment. Feature branches additionally ran both `push` and `pull_request` copies of the same workflows.

This made component ownership unclear and made the wall-clock duration equal to the sum of unrelated checks.

## Decision

Validation is split into component-owned workflows:

| Component | Workflow | Required behavior |
| --- | --- | --- |
| Admin backend | `.github/workflows/ci.yml` | Root tests and root build run in parallel; the historical `CI / build` aggregate check remains available for branch protection. |
| Framework | `.github/workflows/mss-boot-ci.yml` | Independent `GOWORK=off` tests, race tests, vet, and module-tidiness checks. |
| Frontend | `.github/workflows/frontend-ci.yml` | ESLint, TypeScript, Jest, and production-equivalent local build, isolated from Go jobs. |
| Documentation | `.github/workflows/docs.yml` | Dumi build on documentation changes and Cloudflare deployment only after a successful `main` build. |
| Container image | `.github/workflows/container.yml` | Image publishing is independent from validation and uses the GitHub Actions build cache. |

Additional rules:

- Feature branches do not run duplicate `push` workflows. Pull requests validate feature work; `push` remains for `main` and release tags.
- Superseded pull-request runs are cancelled through workflow concurrency groups.
- Redis-backed Go jobs use a native `redis:7-alpine` service instead of building a third-party Docker action.
- Agent infrastructure CI no longer installs Node or builds docs. It still validates Agent contracts that happen to live under `docs/`.
- The inactive nested workflow at `mss-boot/.github/workflows/ci.yml` is removed because GitHub only loads workflows from the repository-root `.github/workflows/` directory.
- The legacy Swagger deployment is path-scoped and version-pins the generator.

## Compatibility

The workflow file `.github/workflows/ci.yml` keeps the workflow name `CI` and exposes a final job named `build`. Existing branch rules that require `CI / build` therefore continue to represent both root tests and root compilation.

New component checks can be made required after one successful pull-request run establishes their exact check names.

## Expected effect

- Root admin tests and compilation run concurrently.
- Framework test, race, and static/module checks run concurrently.
- Frontend and docs jobs no longer wait behind Go validation.
- A documentation-only pull request does not start the root or framework workflow.
- A frontend-only pull request does not start the root, framework, or docs workflow.
- A framework change still triggers root admin compatibility validation, but the two workflows execute independently and concurrently.
- Main-branch image publishing no longer extends the required `CI / build` duration.

## Validation

A workflow change is complete when a pull request demonstrates:

- `CI / build` succeeds;
- `mss-boot CI / mss-boot` succeeds;
- `Frontend CI / frontend` succeeds when frontend paths change;
- `Docs / build` succeeds when docs paths change;
- Agent, security, PR guard, and docs-drift workflows remain green;
- no workflow is triggered twice for the same feature-branch commit solely because both `push` and `pull_request` matched.
