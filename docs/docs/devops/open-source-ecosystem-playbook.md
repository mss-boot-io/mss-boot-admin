---
title: Open Source Ecosystem Playbook
order: 11
nav:
  order: 5
  title: "devops"
description: AI-friendly open source ecosystem playbook for mss-boot-io
keywords: [open source, community, AI, contributor, governance]
---

# Open Source Ecosystem Playbook

This playbook turns the `mss-boot-io` open source direction into repeatable
operations. The target is a personal-maintainer-friendly ecosystem that can move
toward 10/10 without relying on expensive community operations.

## North Star

`mss-boot-io` should be positioned as an AI-governed Go microservice admin
framework:

- agent-readable: agents can understand modules, contracts, permissions,
  tests, and release environments;
- governance-first: PRs, releases, security, and documentation leave an audit
  trail;
- production-maintainable: beta/prod releases are smoke-tested and rollbackable;
- docs-as-memory: durable decisions are recorded in `mss-boot-docs` and each
  repository's `aigc` directory.

## Score Ladder

| Score | Standard | Practical signal |
| --- | --- | --- |
| 7.5 | Trust baseline | README, license, security policy, CI, CodeQL, Dependabot, PR template, and release notes exist. |
| 8.5 | Contributor-ready | A newcomer can pick a `good first issue`, run tests, and open a useful PR in 30 minutes. |
| 9.2 | AI-operated | Weekly digest, issue triage, docs drift checks, PR guard, and AI memory are stable across core repositories. |
| 10 | Ecosystem-grade | Releases include compatibility matrix, SBOM/provenance, smoke evidence, rollback notes, and a visible contributor loop. |

## Contributor Journey

1. Discover the project from the organization README or docs site.
2. Pick an issue labeled `good first issue` or `help wanted`.
3. Read the issue's scope, non-goals, validation commands, and expected output.
4. Run the repository-specific setup:
   - Go libraries: `go test ./...`
   - Admin backend: `make test`
   - Admin frontend: `pnpm install --frozen-lockfile && pnpm test -- --runInBand`
   - Docs: `pnpm install --frozen-lockfile && pnpm build`
5. Open a PR with tests, docs, security, and release impact filled in.
6. Let CI, CodeQL, govulncheck, Scorecard, PR Guard, and Docs Drift validate the change.

## Issue Factory

Each AI-generated or maintainer-created issue should include:

- background: why the change matters;
- scope: files or modules that may be changed;
- non-goals: what should not be changed;
- validation: exact commands to run;
- acceptance criteria: how maintainers decide it is done.

Good first issues should prefer docs, tests, examples, small UI fixes, and
clearly bounded CI improvements. They should not include auth redesign,
database migration, production deployment, or cross-repository refactors.

## AI Operations Loop

| Cadence | Automation | Maintainer action |
| --- | --- | --- |
| Every PR | PR Guard and Docs Drift | Review only real design/security/release decisions. |
| Daily | Dependabot and security checks | Merge low-risk patches after CI; batch risky upgrades. |
| Weekly | Weekly Digest | Pick 1 to 3 issues, update labels, publish short notes if useful. |
| Monthly | Release readiness review | Confirm compatibility, changelog, smoke evidence, and rollback notes. |
| Outreach follow-up | External community check | Route external feedback back to GitHub Issues, Discussions, docs, or security policy. |

AI may draft issue text, summarize CI failures, propose labels, update memory,
and prepare release notes. AI should not auto-merge code, publish Cloudflare
frontend releases, close user issues aggressively, or disclose security details
publicly.

## GitHub-First AI And CI Delegation

Prefer GitHub-native automation before adding external SaaS, self-hosted bots,
or maintainer-only scripts. The default split is:

| Work | GitHub-owned path | Maintainer boundary |
| --- | --- | --- |
| Pull request hygiene | PR Guard, Docs Drift, CodeQL, govulncheck, Scorecard, required checks | Decide architecture, security risk, release timing, and merge approval. |
| Dependency updates | Dependabot PRs and grouped GitHub Actions upgrades | Batch risky upgrades; keep low-risk action/runtime updates green and quiet. |
| Security alerts | CodeQL code scanning and Copilot Autofix for public repositories | Review generated fixes before merge; never disclose private vulnerability context publicly. |
| Review assistance | Copilot code review when enabled for the organization | Treat comments as advisory; maintainer remains accountable for acceptance. |
| Issue drafting | GitHub Copilot issue creation/update and issue forms | Maintainer approves scope, labels, priority, and whether Copilot may be assigned. |
| Implementation delegation | Copilot coding agent on bounded issues when enabled, backed by repository instructions and `copilot-setup-steps.yml` | Use only for low-to-medium-risk tasks with clear tests and non-goals. |

Capabilities that are already safe to rely on in public CI should be encoded in
workflow files. Capabilities that depend on organization policy, Copilot plans,
GitHub Actions minutes, or security settings must be tracked as pending external
configuration before use.

Repository-level setup:

- keep `.github/copilot-instructions.md` and `AGENTS.md` in sync so GitHub
  Copilot and local agents share the same operating boundaries;
- use `.github/workflows/copilot-setup-steps.yml` to preinstall Go or Node
  dependencies for Copilot coding agent sessions;
- group low-risk Dependabot updates where possible and keep explicit open PR
  limits to avoid overwhelming a personal maintainer;
- keep frontend Cloudflare deployments manual, and use concurrency locks for
  public deployment workflows.

Pending maintainer GitHub settings:

- enable or review Copilot automatic code review at the organization or
  repository level after confirming Actions minutes and Copilot plan impact;
- configure branch rulesets so stable checks become required checks after the
  current green CI baseline has held for several days;
- enable private vulnerability reporting and review whether CodeQL Copilot
  Autofix should be used for public alerts;
- decide whether `mss-boot-admin` should inherit organization issue forms or
  keep backend-specific local issue templates.

Useful GitHub references:

- [Copilot code review](https://docs.github.com/en/copilot/concepts/agents/code-review)
- [Copilot coding agent](https://docs.github.com/en/copilot/concepts/coding-agent/coding-agent)
- [Copilot coding agent setup steps](https://docs.github.com/en/copilot/how-tos/use-copilot-agents/coding-agent/customize-the-agent-environment)
- [Copilot issue creation and updates](https://docs.github.com/en/copilot/how-tos/use-copilot-for-common-tasks/use-copilot-to-create-or-update-issues)
- [Copilot Autofix for code scanning](https://docs.github.com/en/code-security/concepts/code-scanning/copilot-autofix-for-code-scanning)

## Release Trust

Public beta and prod should be boring:

- backend beta can move quickly, but prod uses tagged versions;
- frontend Cloudflare alpha/beta deployments are manual and should happen only
  after local CI and smoke readiness;
- docs deploy can remain automatic because it is the public source of truth;
- each release should link commit, image tag or digest, frontend build, API
  compatibility, smoke result, and rollback path.

## Security And Supply Chain

Minimum target:

- `SECURITY.md` in each core repository;
- CodeQL, govulncheck, Dependabot, and OpenSSF Scorecard enabled;
- private vulnerability reporting enabled in GitHub settings when available;
- no secrets in issue text, docs, examples, or workflow logs;
- release artifacts should move toward SBOM and provenance.

## Maintainer Cost Control

For a personal maintainer, the default should be low-noise:

- avoid automatic frontend preview deploys for every PR;
- keep CI required, but make docs/community checks explainable;
- group Dependabot updates where possible;
- rely on AI for triage drafts and summaries, not final judgment;
- keep public communication in GitHub Issues, Discussions, and docs before
  adding extra social channels.

## Next Moves

- Keep all core repository main branches green for the latest runs.
- Merge low-risk CI reliability fixes before enabling stricter required checks.
- Convert the existing good-first-issue factory into a monthly issue batch.
- Add a release compatibility matrix for `mss-boot`, `mss-boot-admin`, and
  `mss-boot-admin-antd`.
- Add SBOM/provenance as a release hardening milestone after the CI baseline is
  stable.
- Keep repository About text and topics aligned with the governance-first
  direction before each outreach round.
- Convert external comments into GitHub follow-up links within 48 hours when
  they are actionable.
