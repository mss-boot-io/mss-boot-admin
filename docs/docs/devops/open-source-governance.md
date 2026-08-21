---
title: Open Source Governance
order: 10
nav:
  order: 5
  title: "devops"
description: mss-boot-io open source governance, quality gates, release policy, and AI memory rules
keywords: [open source, governance, quality gates, release, AI memory]
---

# Open Source Governance

`mss-boot-io` ships an Agent-native Complete Admin Distribution through a
governance-first open source workflow. The goal is to keep its product,
machine-readable contracts, generated code, releases, and upgrades readable by
contributors, maintainers, downstream teams, and AI agents.

## Quality Gates

- Pull requests must include tests, documentation impact, security impact, and
  release impact.
- Go repositories run CI, CodeQL, govulncheck, Dependabot, and OpenSSF
  Scorecard.
- Frontend and Docs are independently publishable components of this repository;
  component-scoped CI keeps their heavy checks isolated while shared contract
  changes run the broader gates.
- API, config, deployment, or workflow changes should update documentation or AI
  memory.

## Release Policy

- Alpha/dev is for fast integration.
- Beta is public preview and must pass browser and delivery smoke testing before
  V6 artifact publication.
- Production uses tagged backend releases and verified frontend deployments.

## Security

Do not report vulnerabilities as public issues. Follow the security policy of
the affected repository and use private GitHub Security Advisories when enabled.

## AI Memory

Long-lived product and release decisions belong in `docs/docs/` or `docs/adr/`;
machine-executable facts belong in `.mss/`; historical prompts remain in
`docs/aigc/prompts/`.

## GitHub-First Community Flow

External platforms can introduce the project, but they are not the decision
system. Bugs, deployment failures, architecture tradeoffs, documentation gaps,
and security concerns must be routed back to GitHub Issues, Discussions, docs,
or the security policy path.

Use Discussions for review and roadmap tradeoffs. Use Issues for actionable
work with clear reproduction or acceptance criteria. Use docs for stable
answers, FAQ entries, and release/process policy.

## Repository About And Topics

Repository descriptions and topics should match the current product:

- Agent-native management systems, the complete Go Admin, React 19 + Ant Design
  6, RBAC, deterministic full-stack generation, Thin Host Blueprints, migration,
  verification, and upgrades are current public signals.
- Runtime virtual models, virtual CRUD, and browser-facing code generation have
  been removed and must not be presented as current capabilities. The separate
  development-time `cmd/mss` generator remains a supported, deterministic
  repository workflow.
- Homepage links should point to the docs site unless a repository has a more
  specific stable entry point.

## Reviewer And CODEOWNERS Boundary

Invite reviewers through Discussions and issues first. Add `CODEOWNERS` only for
real maintainers or confirmed community reviewers. Placeholder owners reduce
trust and create false expectations during PR review.
